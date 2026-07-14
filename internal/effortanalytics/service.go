package effortanalytics

import (
	"context"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/vinzenzs/kazper/internal/numfmt"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// BestEffortsStore is the persistence dependency (satisfied by *Repo).
type BestEffortsStore interface {
	Replace(ctx context.Context, workoutID uuid.UUID, achievedAt time.Time, recs []Record) error
	Curve(ctx context.Context, from, to time.Time, metric Metric) ([]CurvePoint, error)
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
	if err := s.store.Replace(ctx, w.ID, w.StartedAt, recs); err != nil {
		return 0, err
	}
	return len(recs), nil
}

// CurveFor returns the windowed mean-maximal curve for the metric.
func (s *Service) CurveFor(ctx context.Context, p CurveParams) ([]CurvePoint, error) {
	fromDay := time.Date(p.From.Year(), p.From.Month(), p.From.Day(), 0, 0, 0, 0, p.Loc)
	toDay := time.Date(p.To.Year(), p.To.Month(), p.To.Day(), 0, 0, 0, 0, p.Loc)
	upper := toDay.Add(24 * time.Hour).Add(-time.Nanosecond)
	return s.store.Curve(ctx, fromDay, upper, p.Metric)
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
