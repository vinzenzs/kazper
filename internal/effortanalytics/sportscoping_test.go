package effortanalytics_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/effortanalytics"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// seedSportWorkout inserts a completed workout of the given sport.
func seedSportWorkout(t *testing.T, repo *workouts.Repo, sport workouts.Sport, start time.Time) *workouts.Workout {
	t.Helper()
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     sport,
		Status:    workouts.StatusCompleted,
		StartedAt: start,
		EndedAt:   start.Add(time.Hour),
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w
}

// The production bug (fix-effort-ladder-sport-scoping): a run posting Garmin
// running power stored metric='power' best efforts that out-competed real bike
// efforts in every windowed cycling analytic (live cp_model returned CP 56.9 W
// at r² 0.052 from run-sourced points). All four windowed reads must ignore the
// hotter running-power workout entirely.
func TestSportScoping_RunningPowerExcludedFromBikeAnalytics(t *testing.T) {
	f := setup(t)
	day := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	bike := seedSportWorkout(t, f.repo, workouts.SportBike, day)
	run := seedSportWorkout(t, f.repo, workouts.SportRun, day.Add(3*time.Hour))

	// Bike: modest constant 250 W. Run: running power far hotter (500 W) — the
	// contamination signature; unscoped it would win every duration.
	_, err := f.svc.ComputeAndReplace(context.Background(), bike, constSlice(3600, 250), nil)
	require.NoError(t, err)
	_, err = f.svc.ComputeAndReplace(context.Background(), run, constSlice(3600, 500), nil)
	require.NoError(t, err)

	window := "from=2026-03-10&to=2026-03-10&tz=UTC"

	// power-curve (bike): every point from the bike workout at 250 W.
	crec := get(t, f.r, "/workouts/power-curve?"+window+"&sport=bike")
	require.Equal(t, http.StatusOK, crec.Code)
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(crec.Body.Bytes(), &curve))
	require.NotEmpty(t, curve.Points)
	for _, p := range curve.Points {
		assert.Equal(t, bike.ID.String(), p.WorkoutID, "duration %d sourced from the run", p.DurationS)
		assert.InDelta(t, 250, p.Value, 0.11)
	}

	// cp-model: all fit points bike-sourced; constant power fits exactly → no
	// poor_fit warning on clean data.
	mrec := get(t, f.r, "/workouts/cp-model?"+window)
	require.Equal(t, http.StatusOK, mrec.Code)
	var cp effortanalytics.CPModelResult
	require.NoError(t, json.Unmarshal(mrec.Body.Bytes(), &cp))
	require.NotNil(t, cp.Model)
	assert.Empty(t, cp.Warning)
	assert.InDelta(t, 250, cp.Model.CPWatts, 1)
	for _, p := range cp.Points {
		assert.Equal(t, bike.ID.String(), p.WorkoutID)
	}

	// power-profile: anchors bike-sourced at 250 W, never the run's 500.
	prec := get(t, f.r, "/workouts/power-profile?"+window+"&weight_kg=70")
	require.Equal(t, http.StatusOK, prec.Code)
	var profile effortanalytics.PowerProfileResult
	require.NoError(t, json.Unmarshal(prec.Body.Bytes(), &profile))
	require.NotEmpty(t, profile.Anchors)
	for _, a := range profile.Anchors {
		assert.Equal(t, bike.ID.String(), a.WorkoutID)
		assert.InDelta(t, 250, a.Watts, 0.11)
	}

	// durability: the run reaches deeper kJ tiers (1800 kJ at 500 W) but must
	// contribute nothing; every tier best is the bike's 250 W.
	drec := get(t, f.r, "/workouts/durability?"+window)
	require.Equal(t, http.StatusOK, drec.Code)
	var dur effortanalytics.DurabilityResult
	require.NoError(t, json.Unmarshal(drec.Body.Bytes(), &dur))
	require.NotEmpty(t, dur.Durations)
	for _, d := range dur.Durations {
		for _, tp := range d.Tiers {
			assert.Equal(t, bike.ID.String(), tp.WorkoutID, "tier %d of duration %d sourced from the run", tp.KJTier, d.DurationS)
			assert.InDelta(t, 250, tp.Watts, 0.11)
		}
	}
}

// The mirror axis: bikes store speed best efforts too, so an unscoped run pace
// curve would be dominated by (much faster) bike speeds.
func TestSportScoping_BikeSpeedExcludedFromRunCurve(t *testing.T) {
	f := setup(t)
	day := time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC)
	bike := seedSportWorkout(t, f.repo, workouts.SportBike, day)
	run := seedSportWorkout(t, f.repo, workouts.SportRun, day.Add(3*time.Hour))

	_, err := f.svc.ComputeAndReplace(context.Background(), bike, nil, constSlice(3600, 10.0))
	require.NoError(t, err)
	_, err = f.svc.ComputeAndReplace(context.Background(), run, nil, constSlice(3600, 3.0))
	require.NoError(t, err)

	rec := get(t, f.r, "/workouts/power-curve?from=2026-03-10&to=2026-03-10&sport=run&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var curve effortanalytics.Curve
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &curve))
	require.NotEmpty(t, curve.Points)
	for _, p := range curve.Points {
		assert.Equal(t, run.ID.String(), p.WorkoutID, "duration %d sourced from the bike", p.DurationS)
		assert.InDelta(t, 3.0, p.Value, 0.01)
	}
}

// A window whose only power rows came from non-bike workouts must degrade
// through the existing gates rather than fit foreign data.
func TestSportScoping_RunOnlyWindowGatesHonestly(t *testing.T) {
	f := setup(t)
	run := seedSportWorkout(t, f.repo, workouts.SportRun, time.Date(2026, 3, 10, 8, 0, 0, 0, time.UTC))
	_, err := f.svc.ComputeAndReplace(context.Background(), run, constSlice(3600, 500), nil)
	require.NoError(t, err)

	rec := get(t, f.r, "/workouts/cp-model?from=2026-03-10&to=2026-03-10&tz=UTC")
	require.Equal(t, http.StatusOK, rec.Code)
	var cp effortanalytics.CPModelResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &cp))
	assert.Nil(t, cp.Model)
	assert.Equal(t, "insufficient_points", cp.Reason)
	assert.Empty(t, cp.Points)
}
