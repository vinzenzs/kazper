package workoutfueling_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/workoutfueling"
	"github.com/vinzenzs/kazper/internal/workouts"
)

// makeTimedWorkout: a completed workout of `hours` duration starting 08:00 UTC.
func makeTimedWorkout(t *testing.T, repo *workouts.Repo, hours float64) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 6, 7, 8, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusCompleted,
		StartedAt: start,
		EndedAt:   start.Add(time.Duration(hours * float64(time.Hour))),
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

func makePlannedWorkout(t *testing.T, repo *workouts.Repo) uuid.UUID {
	t.Helper()
	start := time.Date(2026, 6, 8, 8, 0, 0, 0, time.UTC)
	w := &workouts.Workout{
		Source:    workouts.SourceManual,
		Sport:     workouts.SportBike,
		Status:    workouts.StatusPlanned,
		StartedAt: start,
		EndedAt:   start.Add(2 * time.Hour),
	}
	_, err := repo.Upsert(context.Background(), w)
	require.NoError(t, err)
	return w.ID
}

// ============================================================================

func TestSweatRate_FieldTest(t *testing.T) {
	f := setup(t)
	wid := makeTimedWorkout(t, f.workoutsRepo, 2)
	at := time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC)
	// 600 ml linked hydration + 400 ml linked workout-fuel = 1000 ml.
	insertHydration(t, f.hydRepo, at, 600, &wid)
	insertFuel(t, f.fuelRepo, at, fuelOpts{ml: ptr(400), workoutID: &wid})
	// An UNLINKED bottle must not count.
	insertHydration(t, f.hydRepo, at, 999, nil)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/sweat-rate?pre_weight_kg=71.0&post_weight_kg=69.8")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.SweatRate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	assert.Equal(t, 2.0, out.DurationHr)
	assert.Equal(t, 600.0, out.Fluid.HydrationMl)
	assert.Equal(t, 400.0, out.Fluid.WorkoutFuelMl)
	assert.Equal(t, 1000.0, out.Fluid.TotalMl)
	assert.Nil(t, out.Fluid.FluidMlOverride)
	// loss = (71.0-69.8)*1000 + 1000 = 2200; rate = 2200/2 = 1100.
	assert.Equal(t, 2200.0, out.SweatLossMl)
	assert.Equal(t, 1100.0, out.SweatRateMlPerHr)
	assert.Nil(t, out.Warning)
}

func TestSweatRate_OverrideReplacesDerived(t *testing.T) {
	f := setup(t)
	wid := makeTimedWorkout(t, f.workoutsRepo, 2)
	at := time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC)
	insertHydration(t, f.hydRepo, at, 600, &wid)
	insertFuel(t, f.fuelRepo, at, fuelOpts{ml: ptr(400), workoutID: &wid})

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/sweat-rate?pre_weight_kg=71.0&post_weight_kg=69.8&fluid_ml_override=1500")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.SweatRate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	require.NotNil(t, out.Fluid.FluidMlOverride)
	assert.Equal(t, 1500.0, *out.Fluid.FluidMlOverride)
	assert.Equal(t, 1500.0, out.Fluid.TotalMl)
	// Derived itemization still visible as the value it replaced.
	assert.Equal(t, 600.0, out.Fluid.HydrationMl)
	assert.Equal(t, 400.0, out.Fluid.WorkoutFuelMl)
	// loss = 1200 + 1500 = 2700; rate = 1350.
	assert.Equal(t, 2700.0, out.SweatLossMl)
	assert.Equal(t, 1350.0, out.SweatRateMlPerHr)
}

func TestSweatRate_WeightGainWarns(t *testing.T) {
	f := setup(t)
	wid := makeTimedWorkout(t, f.workoutsRepo, 2)

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/sweat-rate?pre_weight_kg=70.0&post_weight_kg=71.0")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.SweatRate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	// loss = -1000 (net gain, no fluid) → values still returned, warning set.
	assert.Equal(t, -1000.0, out.SweatLossMl)
	require.NotNil(t, out.Warning)
	assert.Equal(t, "implausible_result", *out.Warning)
}

func TestSweatRate_HighRateWarns(t *testing.T) {
	f := setup(t)
	wid := makeTimedWorkout(t, f.workoutsRepo, 1)

	// pre 76 post 70 over 1h → loss 6000, rate 6000 > 5000 ml/hr.
	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/sweat-rate?pre_weight_kg=76.0&post_weight_kg=70.0")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var out workoutfueling.SweatRate
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out))

	assert.Equal(t, 6000.0, out.SweatRateMlPerHr)
	require.NotNil(t, out.Warning)
	assert.Equal(t, "implausible_result", *out.Warning)
}

func TestSweatRate_PlannedRejected(t *testing.T) {
	f := setup(t)
	wid := makePlannedWorkout(t, f.workoutsRepo)
	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/sweat-rate?pre_weight_kg=71.0&post_weight_kg=69.8")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "workout_not_completed")
}

func TestSweatRate_NotFound(t *testing.T) {
	f := setup(t)
	rec := doGet(t, f.r, "/workouts/"+uuid.NewString()+"/sweat-rate?pre_weight_kg=71.0&post_weight_kg=69.8")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

func TestSweatRate_ParamErrors(t *testing.T) {
	f := setup(t)
	wid := makeTimedWorkout(t, f.workoutsRepo, 2)
	base := "/workouts/" + wid.String() + "/sweat-rate"
	cases := []struct{ query, code string }{
		{"?post_weight_kg=69.8", "pre_weight_invalid"},
		{"?pre_weight_kg=71.0", "post_weight_invalid"},
		{"?pre_weight_kg=0&post_weight_kg=69.8", "pre_weight_invalid"},
		{"?pre_weight_kg=71.0&post_weight_kg=-1", "post_weight_invalid"},
		{"?pre_weight_kg=71.0&post_weight_kg=69.8&fluid_ml_override=-5", "fluid_override_invalid"},
	}
	for _, c := range cases {
		rec := doGet(t, f.r, base+c.query)
		require.Equal(t, http.StatusBadRequest, rec.Code, c.query)
		assert.Contains(t, rec.Body.String(), c.code, c.query)
	}
}

func TestSweatRate_UnitIsolation(t *testing.T) {
	f := setup(t)
	wid := makeTimedWorkout(t, f.workoutsRepo, 2)
	at := time.Date(2026, 6, 7, 8, 30, 0, 0, time.UTC)
	insertFuel(t, f.fuelRepo, at, fuelOpts{ml: ptr(400), carbs: ptr(60), sodium: ptr(500), workoutID: &wid})

	rec := doGet(t, f.r, "/workouts/"+wid.String()+"/sweat-rate?pre_weight_kg=71.0&post_weight_kg=69.8")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	assert.NotContains(t, body, "kcal")
	assert.NotContains(t, body, "sodium")
	assert.NotContains(t, body, "carbs")
}
