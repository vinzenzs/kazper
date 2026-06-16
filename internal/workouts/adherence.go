package workouts

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// AdherenceRow is the minimal per-workout projection plan-adherence needs. It
// is read by Repo.AdherenceCandidates and classified by computeAdherence.
type AdherenceRow struct {
	Status     Status
	Sport      Sport
	PlanSlotID *uuid.UUID
	StartedAt  time.Time
	EndedAt    time.Time
	TSS        *float64
}

// BySportCount is one sport's completed/missed tally in the by_sport breakdown.
type BySportCount struct {
	Completed int `json:"completed"`
	Missed    int `json:"missed"`
}

// AdherenceSummary is the GET /workouts/adherence response. Volume fields are
// pointers so a sum over zero present values serializes as null (not 0), and
// adherence_rate is null when nothing was due in the window.
type AdherenceSummary struct {
	Completed int `json:"completed"`
	Missed    int `json:"missed"`
	Upcoming  int `json:"upcoming"`
	Unplanned int `json:"unplanned"`

	AdherenceRate *float64 `json:"adherence_rate"`

	PlannedDurationMin   *float64 `json:"planned_duration_min"`
	CompletedDurationMin *float64 `json:"completed_duration_min"`
	PlannedTSS           *float64 `json:"planned_tss"`
	CompletedTSS         *float64 `json:"completed_tss"`

	BySport map[string]BySportCount `json:"by_sport"`
}

// computeAdherence classifies each row once and aggregates the window. now is
// the wall clock in the resolved timezone; a planned session whose started_at
// is before now is "missed", at/after now is "upcoming". Planned volume sums
// completed+missed windows (what the plan asked for); completed volume sums
// completed windows only. Numbers are rounded at this boundary.
func computeAdherence(rows []AdherenceRow, now time.Time) AdherenceSummary {
	s := AdherenceSummary{BySport: map[string]BySportCount{}}

	var plannedDur, completedDur float64
	var plannedDurAny, completedDurAny bool
	var plannedTSS, completedTSS float64
	var plannedTSSAny, completedTSSAny bool

	bump := func(sport Sport, completed bool) {
		c := s.BySport[string(sport)]
		if completed {
			c.Completed++
		} else {
			c.Missed++
		}
		s.BySport[string(sport)] = c
	}

	for _, r := range rows {
		durMin := r.EndedAt.Sub(r.StartedAt).Minutes()
		switch {
		case r.Status == StatusCompleted && r.PlanSlotID != nil:
			// A planned session that was done — counts as both planned and actual
			// volume (the fulfilled row carries the actual window).
			s.Completed++
			bump(r.Sport, true)
			plannedDur += durMin
			completedDur += durMin
			plannedDurAny = true
			completedDurAny = true
			if r.TSS != nil {
				plannedTSS += *r.TSS
				completedTSS += *r.TSS
				plannedTSSAny = true
				completedTSSAny = true
			}
		case r.Status == StatusPlanned && r.StartedAt.Before(now):
			// Overdue, never fulfilled — counts only against planned volume.
			s.Missed++
			bump(r.Sport, false)
			plannedDur += durMin
			plannedDurAny = true
			if r.TSS != nil {
				plannedTSS += *r.TSS
				plannedTSSAny = true
			}
		case r.Status == StatusPlanned:
			// started_at >= now — not yet due, excluded from the rate.
			s.Upcoming++
		case r.Status == StatusCompleted && r.PlanSlotID == nil:
			// Off-plan extra work — reported but excluded from the rate.
			s.Unplanned++
		}
	}

	if due := s.Completed + s.Missed; due > 0 {
		rate := numfmt.Round1(float64(s.Completed) / float64(due))
		s.AdherenceRate = &rate
	}
	if plannedDurAny {
		v := numfmt.Round1(plannedDur)
		s.PlannedDurationMin = &v
	}
	if completedDurAny {
		v := numfmt.Round1(completedDur)
		s.CompletedDurationMin = &v
	}
	if plannedTSSAny {
		v := numfmt.Round1(plannedTSS)
		s.PlannedTSS = &v
	}
	if completedTSSAny {
		v := numfmt.Round1(completedTSS)
		s.CompletedTSS = &v
	}
	return s
}

// Adherence loads the windowed candidate workouts (optionally scoped to planID)
// and computes the adherence summary against the current time in the service's
// configured timezone. from/to are the half-open window the handler built from
// inclusive local dates.
func (s *Service) Adherence(ctx context.Context, from, to time.Time, planID *uuid.UUID) (*AdherenceSummary, error) {
	rows, err := s.repo.AdherenceCandidates(ctx, from, to, planID)
	if err != nil {
		return nil, err
	}
	sum := computeAdherence(rows, time.Now().In(s.loc))
	return &sum, nil
}

// DefaultLocation exposes the service's configured timezone so the handler can
// resolve a window's local dates when the request supplies no tz override.
func (s *Service) DefaultLocation() *time.Location { return s.loc }
