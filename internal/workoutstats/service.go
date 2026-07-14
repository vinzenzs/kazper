package workoutstats

import (
	"context"
	"sort"
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

// zoneAccum accumulates zoned seconds + counts for a group (window, sport, week).
type zoneAccum struct {
	secs    [5]int64
	counted int // completed workouts with ≥1 non-null zone
	missing int // completed workouts with all-null zones
}

// DistributionFor aggregates completed workouts in [From, To] into time-in-zone
// shares: a window total (with band collapse + classification), a by-sport
// breakdown, a Monday-start weekly trend, and a training-focus session-count
// axis. Missing zone data is counted, never imputed. Shares/bands are full
// precision internally and rounded at the boundary. (add-intensity-distribution)
func (s *Service) DistributionFor(ctx context.Context, p Params) (*Distribution, error) {
	fromDay := startOfDay(p.From, p.Loc)
	toDay := startOfDay(p.To, p.Loc)
	upper := toDay.Add(24 * time.Hour).Add(-time.Nanosecond)

	completed := statusCompleted
	ws, err := s.repo.List(ctx, fromDay, upper, nil, &completed)
	if err != nil {
		return nil, err
	}

	var total zoneAccum
	bySport := map[string]*zoneAccum{}
	weeks := map[string]*zoneAccum{}
	byFocus := map[string]int{}
	unclassified, missing := 0, 0

	get := func(m map[string]*zoneAccum, k string) *zoneAccum {
		a := m[k]
		if a == nil {
			a = &zoneAccum{}
			m[k] = a
		}
		return a
	}

	for _, w := range ws {
		// Training-focus axis counts EVERY completed workout (zone data or not).
		if w.TrainingFocus != nil {
			byFocus[string(*w.TrainingFocus)]++
		} else {
			unclassified++
		}

		weekKey := mondayOf(w.StartedAt.In(p.Loc)).Format("2006-01-02")
		wk := get(weeks, weekKey) // a bucket exists for any week with a completed workout

		z, hasZone := zoneSecs(w)
		if !hasZone {
			missing++
			total.missing++
			wk.missing++
			continue
		}
		total.counted++
		wk.counted++
		sp := get(bySport, string(w.Sport))
		sp.counted++
		for i := 0; i < 5; i++ {
			total.secs[i] += z[i]
			wk.secs[i] += z[i]
			sp.secs[i] += z[i]
		}
	}

	out := &Distribution{
		From:                   fromDay.Format("2006-01-02"),
		To:                     toDay.Format("2006-01-02"),
		TZ:                     p.Loc.String(),
		Total:                  TotalAggregate{ZoneAggregate: aggregate(&total), Bands: bands(total.secs), Classification: classify(total.secs)},
		BySport:                map[string]ZoneAggregate{},
		Weekly:                 []WeekBucket{},
		ByTrainingFocus:        byFocus,
		UnclassifiedFocusCount: unclassified,
		MissingZoneDataCount:   missing,
	}
	for sp, a := range bySport {
		out.BySport[sp] = aggregate(a)
	}
	for _, wk := range sortedKeys(weeks) {
		a := weeks[wk]
		out.Weekly = append(out.Weekly, WeekBucket{
			WeekStart:            wk,
			ZoneAggregate:        aggregate(a),
			MissingZoneDataCount: a.missing,
		})
	}
	return out, nil
}

// zoneSecs returns a workout's per-zone seconds and whether it has any zone data
// (≥1 non-null zone). A null individual zone contributes 0 (present-only).
func zoneSecs(w *workouts.Workout) ([5]int64, bool) {
	cols := [5]*int{w.SecsInZone1, w.SecsInZone2, w.SecsInZone3, w.SecsInZone4, w.SecsInZone5}
	var out [5]int64
	has := false
	for i, c := range cols {
		if c != nil {
			out[i] = int64(*c)
			has = true
		}
	}
	return out, has
}

// aggregate builds the response ZoneAggregate (5-entry ordered zone shares) from
// an accumulator; share_pct is omitted when the group accrued no zone time.
func aggregate(a *zoneAccum) ZoneAggregate {
	var total int64
	for _, s := range a.secs {
		total += s
	}
	var zones [5]ZoneShare
	for i := 0; i < 5; i++ {
		zones[i] = ZoneShare{Zone: i + 1, Secs: int(a.secs[i])}
		if total > 0 {
			pct := numfmt.Round1(float64(a.secs[i]) / float64(total) * 100)
			zones[i].SharePct = &pct
		}
	}
	return ZoneAggregate{WorkoutsCounted: a.counted, TotalZoneSecs: int(total), Zones: zones}
}

// bands collapses zone seconds into low/moderate/high band percentages (1dp).
func bands(secs [5]int64) Bands {
	var total int64
	for _, s := range secs {
		total += s
	}
	if total == 0 {
		return Bands{}
	}
	low := float64(secs[0]+secs[1]) / float64(total) * 100
	mod := float64(secs[2]) / float64(total) * 100
	high := float64(secs[3]+secs[4]) / float64(total) * 100
	return Bands{LowPct: numfmt.Round1(low), ModeratePct: numfmt.Round1(mod), HighPct: numfmt.Round1(high)}
}

// classify labels the distribution from full-precision band shares. Returns nil
// when there is no zone time. A total, deterministic partition.
func classify(secs [5]int64) *string {
	var total int64
	for _, s := range secs {
		total += s
	}
	if total == 0 {
		return nil
	}
	t := float64(total)
	low := float64(secs[0]+secs[1]) / t * 100
	mod := float64(secs[2]) / t * 100
	high := float64(secs[3]+secs[4]) / t * 100

	var label string
	switch {
	case mod >= thresholdBandPct:
		label = "threshold"
	case low >= lowBaseBandPct && high > mod:
		label = "polarized"
	case low >= lowBaseBandPct:
		label = "pyramidal"
	default:
		label = "mixed"
	}
	return &label
}

// mondayOf returns the Monday of t's local week, at local midnight.
func mondayOf(t time.Time) time.Time {
	d := startOfDay(t, t.Location())
	offset := (int(d.Weekday()) + 6) % 7 // Monday→0, Sunday→6
	return d.AddDate(0, 0, -offset)
}

func sortedKeys(m map[string]*zoneAccum) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
