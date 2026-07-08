package workouts

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
)

// AdherenceRow is the minimal per-workout projection plan-adherence needs. It
// is read by Repo.AdherenceCandidates and classified by computeAdherence.
type AdherenceRow struct {
	ID         uuid.UUID
	Status     Status
	Sport      Sport
	PlanSlotID *uuid.UUID
	StartedAt  time.Time
	EndedAt    time.Time
	TSS        *float64

	// Plan-week provenance — populated only when AdherenceCandidates is called
	// with a plan_id (the join is present). Nil in the unscoped read, which is
	// what tells computeAdherence to bucket the weekly trend by calendar week
	// instead of by plan week.
	PlanWeekOrdinal *int
	PlanPhase       *string
	PlanStartDate   *time.Time
}

// BySportCount is one sport's completed/missed tally in the by_sport breakdown.
type BySportCount struct {
	Completed int `json:"completed"`
	Missed    int `json:"missed"`
}

// missedSessionsCap bounds the missed_sessions list. Plan-scoped a window is
// naturally bounded, but an unscoped YTD window can list many misses — beyond
// the cap the tail is dropped and MissedSessionsTruncated is set so the list
// never reads as complete.
const missedSessionsCap = 50

// MissedSession is one compact entry in the missed_sessions list — enough to
// name the session, not to fully describe it (no title/focus by design). Date
// is the session's local date in the resolved timezone.
type MissedSession struct {
	ID                 uuid.UUID `json:"id"`
	Date               string    `json:"date"`
	Sport              Sport     `json:"sport"`
	PlannedDurationMin float64   `json:"planned_duration_min"`
	PlannedTSS         *float64  `json:"planned_tss"`
}

// WeeklyBucket is one week of the adherence trend. Ordinal and Phase are set
// only in plan-week mode (plan_id supplied); in calendar mode they are null and
// WeekStart is the Monday of the week. AdherenceRate/duration are null on the
// same "nothing due / no present values" rules as the top-level summary.
type WeeklyBucket struct {
	WeekStart string  `json:"week_start"`
	Ordinal   *int    `json:"ordinal"`
	Phase     *string `json:"phase"`

	Completed int `json:"completed"`
	Missed    int `json:"missed"`

	AdherenceRate        *float64 `json:"adherence_rate"`
	PlannedDurationMin   *float64 `json:"planned_duration_min"`
	CompletedDurationMin *float64 `json:"completed_duration_min"`
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

	// MissedSessions names the overdue-unfulfilled sessions (oldest first),
	// capped at missedSessionsCap; MissedSessionsTruncated is set when the tail
	// was dropped. Weekly is the per-week trend (plan-week or calendar buckets).
	MissedSessions          []MissedSession `json:"missed_sessions"`
	MissedSessionsTruncated bool            `json:"missed_sessions_truncated"`
	Weekly                  []WeeklyBucket  `json:"weekly"`
}

// weekAcc accumulates one weekly bucket while classifying rows. Metadata
// (weekStart/ordinal/phase) is captured from the first row that lands in the
// bucket; every same-key row shares it.
type weekAcc struct {
	weekStart    string
	ordinal      *int
	phase        *string
	completed    int
	missed       int
	plannedDur   float64
	plannedAny   bool
	completedDur float64
	completedAny bool
}

// bucketFor returns the bucket key for a row plus the metadata to seed a fresh
// bucket. Plan-week mode (ordinal present) keys by ordinal and derives
// week_start from the plan's start_date; calendar mode keys by the Monday of the
// row's local week.
func bucketFor(r AdherenceRow, loc *time.Location) (key string, meta weekAcc) {
	if r.PlanWeekOrdinal != nil {
		ord := *r.PlanWeekOrdinal
		ws := ""
		if r.PlanStartDate != nil {
			ws = r.PlanStartDate.AddDate(0, 0, (ord-1)*7).Format("2006-01-02")
		}
		return fmt.Sprintf("ord:%d", ord), weekAcc{weekStart: ws, ordinal: &ord, phase: r.PlanPhase}
	}
	d := r.StartedAt.In(loc)
	// Monday-of-week: Go's Weekday has Sunday=0, so days-since-Monday is (wd+6)%7.
	offset := (int(d.Weekday()) + 6) % 7
	monday := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -offset)
	ws := monday.Format("2006-01-02")
	return ws, weekAcc{weekStart: ws}
}

// computeAdherence classifies each row once and folds it into both the window
// total and a per-week bucket, so the top-line and the weekly trend are computed
// in a single pass and can never disagree. now is the wall clock; a planned
// session whose started_at is before now is "missed", at/after "upcoming".
// Planned volume sums completed+missed windows; completed volume sums completed
// windows only. loc resolves local dates for the missed list and calendar-week
// bucketing. Numbers are rounded at this boundary.
func computeAdherence(rows []AdherenceRow, now time.Time, loc *time.Location) AdherenceSummary {
	s := AdherenceSummary{
		BySport:        map[string]BySportCount{},
		MissedSessions: []MissedSession{},
		Weekly:         []WeeklyBucket{},
	}

	var plannedDur, completedDur float64
	var plannedDurAny, completedDurAny bool
	var plannedTSS, completedTSS float64
	var plannedTSSAny, completedTSSAny bool

	buckets := map[string]*weekAcc{}
	getBucket := func(r AdherenceRow) *weekAcc {
		key, meta := bucketFor(r, loc)
		b := buckets[key]
		if b == nil {
			m := meta
			b = &m
			buckets[key] = b
		}
		return b
	}

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
		b := getBucket(r)
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
			b.completed++
			b.plannedDur += durMin
			b.completedDur += durMin
			b.plannedAny = true
			b.completedAny = true
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
			b.missed++
			b.plannedDur += durMin
			b.plannedAny = true
			s.MissedSessions = append(s.MissedSessions, MissedSession{
				ID:                 r.ID,
				Date:               r.StartedAt.In(loc).Format("2006-01-02"),
				Sport:              r.Sport,
				PlannedDurationMin: numfmt.Round1(durMin),
				PlannedTSS:         roundPtr(r.TSS),
			})
		case r.Status == StatusPlanned:
			// started_at >= now — not yet due, excluded from the rate. The bucket
			// still exists (an all-future week is a real, null-rate week).
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

	// Missed list: rows arrive started_at-ascending, so it is already oldest
	// first; truncate the tail (newest) past the cap and flag it.
	sort.SliceStable(s.MissedSessions, func(i, j int) bool {
		return s.MissedSessions[i].Date < s.MissedSessions[j].Date
	})
	if len(s.MissedSessions) > missedSessionsCap {
		s.MissedSessionsTruncated = true
		s.MissedSessions = s.MissedSessions[:missedSessionsCap]
	}

	// Weekly trend: finalize each bucket and sort by week_start (monotonic with
	// ordinal in plan mode, chronological in calendar mode).
	for _, b := range buckets {
		wb := WeeklyBucket{
			WeekStart: b.weekStart,
			Ordinal:   b.ordinal,
			Phase:     b.phase,
			Completed: b.completed,
			Missed:    b.missed,
		}
		if due := b.completed + b.missed; due > 0 {
			rate := numfmt.Round1(float64(b.completed) / float64(due))
			wb.AdherenceRate = &rate
		}
		if b.plannedAny {
			v := numfmt.Round1(b.plannedDur)
			wb.PlannedDurationMin = &v
		}
		if b.completedAny {
			v := numfmt.Round1(b.completedDur)
			wb.CompletedDurationMin = &v
		}
		s.Weekly = append(s.Weekly, wb)
	}
	sort.SliceStable(s.Weekly, func(i, j int) bool {
		return s.Weekly[i].WeekStart < s.Weekly[j].WeekStart
	})

	return s
}

// roundPtr rounds a nullable nutrient/metric to 1dp, preserving nil.
func roundPtr(v *float64) *float64 {
	if v == nil {
		return nil
	}
	r := numfmt.Round1(*v)
	return &r
}

// Adherence loads the windowed candidate workouts (optionally scoped to planID)
// and computes the adherence summary against the current time. loc resolves the
// missed list's local dates and calendar-week bucketing; the classification
// "now" is the same instant regardless of loc. from/to are the half-open window
// the handler built from inclusive local dates.
func (s *Service) Adherence(ctx context.Context, from, to time.Time, planID *uuid.UUID, loc *time.Location) (*AdherenceSummary, error) {
	rows, err := s.repo.AdherenceCandidates(ctx, from, to, planID)
	if err != nil {
		return nil, err
	}
	if loc == nil {
		loc = s.loc
	}
	sum := computeAdherence(rows, time.Now().In(loc), loc)
	return &sum, nil
}

// DefaultLocation exposes the service's configured timezone so the handler can
// resolve a window's local dates when the request supplies no tz override.
func (s *Service) DefaultLocation() *time.Location { return s.loc }
