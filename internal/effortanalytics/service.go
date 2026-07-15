package effortanalytics

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// BestEffortsStore is the persistence dependency (satisfied by *Repo).
type BestEffortsStore interface {
	Replace(ctx context.Context, workoutID uuid.UUID, achievedAt time.Time, recs []Record) error
	Curve(ctx context.Context, from, to time.Time, metric Metric, sport string) ([]CurvePoint, error)
	DurabilityBests(ctx context.Context, from, to time.Time) ([]TierBest, error)
}

// Service computes best efforts from streams and serves the aggregated curve.
// The stream-ingest entrypoint lives in the activity-streams capability, which
// persists the raw arrays and then delegates the mean-maximal computation here
// via ComputeAndReplace (persist-activity-streams).
type Service struct {
	store BestEffortsStore
}

func NewService(store BestEffortsStore) *Service {
	return &Service{store: store}
}

// ComputeAndReplace recomputes the mean-maximal ladder for the already-resolved
// workout from the power/speed series and replaces its best-effort rows. Empty
// input is a no-op (existing rows untouched). Returns the record count. Exposed
// so the activity-streams capability can reuse the computation without a second
// workout fetch (persist-activity-streams).
func (s *Service) ComputeAndReplace(ctx context.Context, w *workouts.Workout, power, speed []float64) (int, error) {
	if len(power) == 0 && len(speed) == 0 {
		return 0, nil
	}
	var recs []Record
	recs = append(recs, meanMaximal(power, MetricPower)...)
	recs = append(recs, meanMaximal(speed, MetricSpeed)...)
	recs = append(recs, tieredEfforts(power)...)
	if err := s.store.Replace(ctx, w.ID, w.StartedAt, recs); err != nil {
		return 0, err
	}
	return len(recs), nil
}

// tieredEfforts computes the kJ-tiered mean-maximal power best efforts: for each
// durability tier, the best rolling-window average at each durability duration
// whose window STARTS at or after the point where cumulative work (∫power dt, at
// 1 Hz a running sum) reaches the tier. Rows are produced only for tiers the ride
// actually reaches, and only when the ride has a full window after the tier.
// Power only; an empty series yields nothing. Values rounded at the boundary.
func tieredEfforts(power []float64) []Record {
	n := len(power)
	if n == 0 {
		return nil
	}
	prefix := make([]float64, n+1) // prefix[i] = work (J) done before sample i
	for i, v := range power {
		prefix[i+1] = prefix[i] + v
	}

	var out []Record
	for _, tierKJ := range DurabilityTiers {
		threshold := float64(tierKJ) * 1000
		// prefix is non-decreasing (power ≥ 0), so the first index at/after the
		// tier is a binary search. minStart == n+1 means the ride never reached it.
		minStart := sort.Search(n+1, func(i int) bool { return prefix[i] >= threshold })
		if minStart > n {
			continue // tier never reached
		}
		for _, d := range DurabilityDurations {
			best := math.Inf(-1)
			for i := minStart; i+d <= n; i++ {
				mean := (prefix[i+d] - prefix[i]) / float64(d)
				if mean > best {
					best = mean
				}
			}
			if math.IsInf(best, -1) || best < 0 {
				continue // no full window after the tier
			}
			out = append(out, Record{
				Metric:    MetricPower,
				DurationS: d,
				Value:     numfmt.Round1(best),
				KJTier:    tierKJ,
			})
		}
	}
	return out
}

// CurveFor returns the windowed mean-maximal curve for the metric, scoped to
// p.Sport (empty defaults to bike — the power metric's home).
func (s *Service) CurveFor(ctx context.Context, p CurveParams) ([]CurvePoint, error) {
	fromDay := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, p.Loc)
	toDay := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc)
	upper := toDay.Add(24 * time.Hour).Add(-time.Nanosecond)
	sport := p.Sport
	if sport == "" {
		sport = string(workouts.SportBike)
	}
	return s.store.Curve(ctx, fromDay, upper, p.Metric, sport)
}

// CPModelFor fits the critical-power model over the window's power best-efforts.
// It reuses the windowed per-duration MAX (the power-curve projection), keeps the
// in-band points, and either fits or reports a gate reason with a null model.
// Compute-on-read; reads no athlete-config; persists nothing. From/To/TZ are set
// by the caller (they carry the resolved display strings).
func (s *Service) CPModelFor(ctx context.Context, p CurveParams) (*CPModelResult, error) {
	p.Metric = MetricPower
	p.Sport = string(workouts.SportBike)
	curve, err := s.CurveFor(ctx, p)
	if err != nil {
		return nil, err
	}
	pts := selectInBand(curve)
	res := &CPModelResult{Points: pts}
	if reason := gateCP(pts); reason != "" {
		res.Reason = reason
		return res, nil
	}
	m := fitCPModel(pts)
	res.Model = &m
	// Quality is an independent axis from the gates: return the fit
	// (auditability) but flag one whose line explains <50% of the variance.
	if m.RSquared < cpPoorFitR2 {
		res.Warning = WarningPoorFit
	}
	return res, nil
}

// Default + bounds for the CP-history trailing window (days).
const (
	cpHistoryWindowDefault = 90
	cpHistoryWindowMin     = 30
	cpHistoryWindowMax     = 365
)

// CPModelHistoryFor fits the CP2 model at each Monday anchor in [From, To], each
// over its trailing windowDays, reusing the shipped fit + gates. A window that
// can't support a fit yields a null model + the gate reason (the anchor is kept,
// so the trend gaps rather than zeroes). Compute-on-read; the caller sets
// From/To/TZ; windowDays is validated by the handler.
func (s *Service) CPModelHistoryFor(ctx context.Context, p CurveParams, windowDays int) (*CPModelHistoryResult, error) {
	res := &CPModelHistoryResult{WindowDays: windowDays, Anchors: []CPHistoryAnchor{}}

	from := dateOnlyUTC(p.From)
	to := dateOnlyUTC(p.To)
	// First Monday on or after `from`.
	anchor := from
	for anchor.Weekday() != time.Monday {
		anchor = anchor.AddDate(0, 0, 1)
	}
	for ; !anchor.After(to); anchor = anchor.AddDate(0, 0, 7) {
		winFrom := anchor.AddDate(0, 0, -(windowDays - 1))
		fit, err := s.CPModelFor(ctx, CurveParams{From: winFrom, To: anchor, Loc: p.Loc})
		if err != nil {
			return nil, err
		}
		res.Anchors = append(res.Anchors, CPHistoryAnchor{
			Date:    anchor.Format("2006-01-02"),
			Model:   fit.Model,
			Reason:  fit.Reason,
			Warning: fit.Warning,
		})
	}
	return res, nil
}

// dateOnlyUTC strips time-of-day, keeping the calendar date at UTC midnight for a
// clean weekday/day-iteration sequence.
func dateOnlyUTC(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DurabilityFor assembles the fresh-vs-tier fade table over the window: for each
// durability duration, the fresh (tier-0) best and each tier's best with
// fade_pct = (fresh − tier)/fresh × 100. Tiers absent in the window are omitted;
// a window with only fresh rows carries reason "no_tiered_data". Compute-on-read
// over stored rows; the caller sets From/To/TZ. fade rounded at the boundary.
func (s *Service) DurabilityFor(ctx context.Context, p CurveParams) (*DurabilityResult, error) {
	fromDay := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, p.Loc)
	toDay := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc)
	upper := toDay.Add(24 * time.Hour).Add(-time.Nanosecond)

	bests, err := s.store.DurabilityBests(ctx, fromDay, upper)
	if err != nil {
		return nil, err
	}

	// Index by duration → tier → best.
	byDur := map[int]map[int]TierBest{}
	anyTiered := false
	for _, b := range bests {
		if byDur[b.DurationS] == nil {
			byDur[b.DurationS] = map[int]TierBest{}
		}
		byDur[b.DurationS][b.KJTier] = b
		if b.KJTier > 0 {
			anyTiered = true
		}
	}

	res := &DurabilityResult{Durations: []DurabilityDuration{}}
	for _, d := range DurabilityDurations {
		tiers := byDur[d]
		if len(tiers) == 0 {
			continue // no data at this duration in the window
		}
		col := DurabilityDuration{DurationS: d, Tiers: []DurabilityTierPoint{}}
		fresh, hasFresh := tiers[0]
		if hasFresh {
			col.Fresh = &DurabilityPoint{Watts: fresh.Watts, WorkoutID: fresh.WorkoutID, Date: fresh.Date}
		}
		for _, tierKJ := range DurabilityTiers {
			tb, ok := tiers[tierKJ]
			if !ok {
				continue
			}
			tp := DurabilityTierPoint{
				KJTier: tierKJ, Watts: tb.Watts, WorkoutID: tb.WorkoutID, Date: tb.Date,
			}
			if hasFresh && fresh.Watts > 0 {
				tp.FadePct = numfmt.Round1((fresh.Watts - tb.Watts) / fresh.Watts * 100)
			}
			col.Tiers = append(col.Tiers, tp)
		}
		res.Durations = append(res.Durations, col)
	}

	if !anyTiered {
		res.Reason = "no_tiered_data"
	}
	return res, nil
}

// PowerProfileFor ranks the window's power best-efforts at the four Coggan
// anchors against the reference tables for `sex`, dividing by `weightKg`. It
// reuses the windowed per-duration MAX (the power-curve projection); an anchor
// with no best-effort in the window is omitted and named in MissingAnchors. The
// phenotype is derived from the ranked anchors (nil unless all four are present).
// Compute-on-read; reads no athlete-config; persists nothing. The caller resolves
// the weight (param or stored) and sets From/To/TZ/WeightSource.
func (s *Service) PowerProfileFor(ctx context.Context, p CurveParams, weightKg float64, sex string) (*PowerProfileResult, error) {
	p.Metric = MetricPower
	p.Sport = string(workouts.SportBike)
	curve, err := s.CurveFor(ctx, p)
	if err != nil {
		return nil, err
	}
	byDuration := make(map[int]CurvePoint, len(curve))
	for _, c := range curve {
		byDuration[c.DurationS] = c
	}

	res := &PowerProfileResult{
		Sex:            sex,
		WeightKg:       weightKg,
		Anchors:        []PowerProfileAnchor{},
		MissingAnchors: []string{},
	}
	for _, a := range cogganAnchors {
		pt, ok := byDuration[a.durationS]
		if !ok {
			res.MissingAnchors = append(res.MissingAnchors, a.label)
			continue
		}
		wPerKg := pt.Value / weightKg
		category, percentile := rankAnchor(wPerKg, a.col, sex)
		res.Anchors = append(res.Anchors, PowerProfileAnchor{
			Label:      a.label,
			DurationS:  a.durationS,
			Watts:      pt.Value,
			WPerKg:     wPerKg,
			Category:   category,
			Percentile: percentile,
			WorkoutID:  pt.WorkoutID,
			Date:       pt.Date,
		})
	}
	res.Phenotype = phenotype(res.Anchors)
	return res, nil
}

// meanMaximal returns the best rolling-window mean of a 1 Hz sample series at
// each ladder duration — the mean-maximal (a.k.a. mean-max) curve for one
// activity. Samples are contiguous seconds (coasting seconds are 0), so a
// window of D seconds is D samples. A prefix sum makes each duration O(n).
// Durations longer than the series are skipped. Values are rounded at the
// boundary. An empty series yields no records.
func meanMaximal(samples []float64, metric Metric) []Record {
	n := len(samples)
	if n == 0 {
		return nil
	}
	prefix := make([]float64, n+1)
	for i, v := range samples {
		prefix[i+1] = prefix[i] + v
	}
	var out []Record
	for _, d := range Ladder {
		if d > n {
			continue
		}
		best := math.Inf(-1)
		for i := 0; i+d <= n; i++ {
			mean := (prefix[i+d] - prefix[i]) / float64(d)
			if mean > best {
				best = mean
			}
		}
		if math.IsInf(best, -1) || best < 0 {
			continue
		}
		out = append(out, Record{Metric: metric, DurationS: d, Value: numfmt.Round1(best)})
	}
	return out
}
