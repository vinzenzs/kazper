package workoutstats

import (
	"context"
	"time"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// WorkoutsRepo is the read dependency: the completed-workout window the totals
// are composed from. Satisfied by *workouts.Repo.
type WorkoutsRepo interface {
	List(ctx context.Context, from, to time.Time, sessionGroup, status *string) ([]*workouts.Workout, error)
}

// Service composes workout volume totals over a date window.
type Service struct {
	repo WorkoutsRepo
}

func NewService(repo WorkoutsRepo) *Service {
	return &Service{repo: repo}
}

const statusCompleted = "completed"

// SummaryFor returns the per-day series + window total for [From, To] (inclusive
// calendar dates in Loc). Only completed workouts count; planned are excluded.
// Every calendar day in the range gets a bucket (zero-filled) so the caller can
// render a complete heatmap grid.
func (s *Service) SummaryFor(ctx context.Context, p Params) (*Summary, error) {
	fromDay := startOfDay(p.From, p.Loc)
	toDay := startOfDay(p.To, p.Loc)
	// The repo filters on started_at in [lower, upper]; upper is the last instant
	// of the To day so a late-evening workout is included.
	upper := toDay.Add(24 * time.Hour).Add(-time.Nanosecond)

	status := statusCompleted
	ws, err := s.repo.List(ctx, fromDay, upper, nil, &status)
	if err != nil {
		return nil, err
	}

	// Seed a bucket for every day in the range so gaps render as zero.
	byDate := map[string]*Bucket{}
	var days []Bucket
	for d := fromDay; !d.After(toDay); d = d.Add(24 * time.Hour) {
		key := d.Format("2006-01-02")
		days = append(days, Bucket{Date: key, BySport: map[string]int{}})
	}
	for i := range days {
		byDate[days[i].Date] = &days[i]
	}

	total := Bucket{BySport: map[string]int{}}
	for _, w := range ws {
		key := w.StartedAt.In(p.Loc).Format("2006-01-02")
		b := byDate[key]
		if b == nil {
			// Defensive: a workout outside the seeded range (tz edge) — skip rather
			// than panic. The repo bounds make this unlikely.
			continue
		}
		accumulate(b, w)
		accumulate(&total, w)
	}

	round(&total)
	for i := range days {
		round(&days[i])
	}

	return &Summary{
		From:  fromDay.Format("2006-01-02"),
		To:    toDay.Format("2006-01-02"),
		TZ:    p.Loc.String(),
		Days:  days,
		Total: total,
	}, nil
}

// accumulate adds one workout's contribution to a bucket. Duration is elapsed
// (ended − started) minutes; nullable metrics are summed present-only.
func accumulate(b *Bucket, w *workouts.Workout) {
	b.Count++
	b.TotalDurationMin += w.EndedAt.Sub(w.StartedAt).Minutes()
	if w.DistanceM != nil {
		b.TotalDistanceM += *w.DistanceM
	}
	if w.ElevationGainM != nil {
		b.TotalElevationGainM += *w.ElevationGainM
	}
	if w.KcalBurned != nil {
		b.TotalKcal += *w.KcalBurned
	}
	b.BySport[string(w.Sport)]++
}

// round applies the 1dp response-boundary rounding to every numeric total.
func round(b *Bucket) {
	b.TotalDurationMin = numfmt.Round1(b.TotalDurationMin)
	b.TotalDistanceM = numfmt.Round1(b.TotalDistanceM)
	b.TotalElevationGainM = numfmt.Round1(b.TotalElevationGainM)
	b.TotalKcal = numfmt.Round1(b.TotalKcal)
}

func startOfDay(t time.Time, loc *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}
