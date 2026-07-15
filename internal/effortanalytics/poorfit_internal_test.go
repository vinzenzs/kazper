package effortanalytics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// curveStore is a fake BestEffortsStore serving a fixed curve — the poor-fit
// warning tests need crafted envelopes the real ingest can't easily produce.
type curveStore struct{ points []CurvePoint }

func (s *curveStore) Replace(context.Context, uuid.UUID, time.Time, []Record) error { return nil }
func (s *curveStore) Curve(context.Context, time.Time, time.Time, Metric, string) ([]CurvePoint, error) {
	return s.points, nil
}
func (s *curveStore) DurabilityBests(context.Context, time.Time, time.Time) ([]TierBest, error) {
	return nil, nil
}

func cpParams() CurveParams {
	return CurveParams{
		From: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC),
		Loc:  time.UTC,
	}
}

// The contamination signature that motivated the flag: hot short-duration
// points beside a modest long one make work non-monotonic in time, so the
// work–time line explains almost nothing — yet both existing gates pass. The
// fit must return (auditability) but carry the poor_fit warning.
func TestCPModelFor_PoorFitWarned(t *testing.T) {
	svc := NewService(&curveStore{points: []CurvePoint{
		{DurationS: 300, Value: 540, WorkoutID: "run-a", Date: "2026-03-05"},
		{DurationS: 600, Value: 500, WorkoutID: "run-a", Date: "2026-03-05"},
		{DurationS: 1200, Value: 460, WorkoutID: "run-b", Date: "2026-03-12"},
		{DurationS: 1800, Value: 120, WorkoutID: "bike", Date: "2026-03-20"},
	}})
	res, err := svc.CPModelFor(context.Background(), cpParams())
	require.NoError(t, err)
	require.NotNil(t, res.Model, "poor fit is returned, not gated")
	assert.Empty(t, res.Reason)
	assert.Equal(t, WarningPoorFit, res.Warning)
	assert.Less(t, res.Model.RSquared, cpPoorFitR2)
}

// A clean monotonic envelope fits well and must carry no warning.
func TestCPModelFor_SoundFitUnwarned(t *testing.T) {
	// work = 250·t + 20000 exactly → r² = 1.
	mk := func(d int) CurvePoint {
		return CurvePoint{DurationS: d, Value: 250 + 20000/float64(d), WorkoutID: "bike", Date: "2026-03-05"}
	}
	svc := NewService(&curveStore{points: []CurvePoint{mk(300), mk(600), mk(1200), mk(1800)}})
	res, err := svc.CPModelFor(context.Background(), cpParams())
	require.NoError(t, err)
	require.NotNil(t, res.Model)
	assert.Empty(t, res.Warning)
	assert.GreaterOrEqual(t, res.Model.RSquared, cpPoorFitR2)
}

// History anchors inherit the warning per anchor.
func TestCPModelHistoryFor_AnchorCarriesWarning(t *testing.T) {
	svc := NewService(&curveStore{points: []CurvePoint{
		{DurationS: 300, Value: 540, WorkoutID: "run-a", Date: "2026-03-05"},
		{DurationS: 600, Value: 500, WorkoutID: "run-a", Date: "2026-03-05"},
		{DurationS: 1200, Value: 460, WorkoutID: "run-b", Date: "2026-03-12"},
		{DurationS: 1800, Value: 120, WorkoutID: "bike", Date: "2026-03-20"},
	}})
	res, err := svc.CPModelHistoryFor(context.Background(), cpParams(), 90)
	require.NoError(t, err)
	require.NotEmpty(t, res.Anchors)
	for _, a := range res.Anchors {
		require.NotNil(t, a.Model)
		assert.Equal(t, WarningPoorFit, a.Warning)
	}
}
