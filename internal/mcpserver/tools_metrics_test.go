package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogRecoveryMetrics_ForwardsBodyAndDerivesKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"date":"2026-06-09"}`)
	_ = handleLogRecoveryMetrics(context.Background(), c, LogRecoveryMetricsArgs{
		Date: "2026-06-09", SleepSeconds: ptrInt(27000), RestingHR: ptrInt(48), HRVMs: ptrFloat(61),
	})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/recovery-metrics", rec.path)
	assert.NotEmpty(t, rec.idemKey, "write tool derives an idempotency key")
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.body, &body))
	assert.Equal(t, "2026-06-09", body["date"])
	assert.EqualValues(t, 27000, body["sleep_seconds"])
	assert.EqualValues(t, 48, body["resting_hr"])
	// Omitted metrics are absent (not null).
	_, hasStress := body["stress_avg"]
	assert.False(t, hasStress)
}

func TestListRecoveryMetrics_BuildsWindowNoKey(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"recovery_metrics":[]}`)
	_ = handleListRecoveryMetrics(context.Background(), c, ListRecoveryMetricsArgs{From: "2026-06-01", To: "2026-06-30"})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, "/recovery-metrics", rec.path)
	values, err := url.ParseQuery(rec.rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01", values.Get("from"))
	assert.Equal(t, "2026-06-30", values.Get("to"))
	assert.Empty(t, rec.idemKey)
}

func TestGetAndDeleteRecoveryMetrics_AddressByDate(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"date":"2026-06-09"}`)
	_ = handleGetRecoveryMetrics(context.Background(), c, GetRecoveryMetricsArgs{Date: "2026-06-09"})
	require.Len(t, *recs, 1)
	assert.Equal(t, "/recovery-metrics/2026-06-09", (*recs)[0].path)

	c2, recs2 := newWorkoutRecorder(t, 204, ``)
	r := handleDeleteRecoveryMetrics(context.Background(), c2, DeleteRecoveryMetricsArgs{Date: "2026-06-09"})
	require.Len(t, *recs2, 1)
	assert.Equal(t, http.MethodDelete, (*recs2)[0].method)
	assert.Equal(t, "/recovery-metrics/2026-06-09", (*recs2)[0].path)
	assert.False(t, r.IsError)
}

func TestLogFitnessMetrics_ForwardsBody(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"date":"2026-06-09"}`)
	_ = handleLogFitnessMetrics(context.Background(), c, LogFitnessMetricsArgs{
		Date: "2026-06-09", VO2MaxRunning: ptrFloat(54), RacePredictor5kSeconds: ptrInt(1230),
	})
	require.Len(t, *recs, 1)
	rec := (*recs)[0]
	assert.Equal(t, "/fitness-metrics", rec.path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.body, &body))
	assert.InDelta(t, 54.0, body["vo2max_running"], 0.001)
	assert.EqualValues(t, 1230, body["race_predictor_5k_seconds"])
	_, hasCycling := body["vo2max_cycling"]
	assert.False(t, hasCycling)
}

func TestListFitnessMetrics_BuildsWindow(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"fitness_metrics":[]}`)
	_ = handleListFitnessMetrics(context.Background(), c, ListFitnessMetricsArgs{From: "2026-06-01", To: "2026-06-30"})
	require.Len(t, *recs, 1)
	values, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "2026-06-01", values.Get("from"))
}

func TestLogWeight_ForwardsBiometrics(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"id":"w1"}`)
	_ = handleLogWeight(context.Background(), c, LogWeightArgs{
		WeightKg: 72.5, LoggedAt: "2026-06-09T07:00:00Z",
		MuscleMassKg: ptrFloat(58.4), BMI: ptrFloat(22.4),
	})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &body))
	assert.InDelta(t, 58.4, body["muscle_mass_kg"], 0.001)
	assert.InDelta(t, 22.4, body["bmi"], 0.001)
	_, hasWater := body["body_water_pct"]
	assert.False(t, hasWater, "omitted biometric absent from body")
}

func TestLogWorkout_ForwardsStatus(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 201, `{"id":"w1"}`)
	_ = handleLogWorkout(context.Background(), c, LogWorkoutArgs{
		Source: "garmin", Sport: "bike", Status: "planned",
		StartedAt: "2026-09-01T08:00:00Z", EndedAt: "2026-09-01T10:00:00Z",
	})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &body))
	assert.Equal(t, "planned", body["status"])
}

func TestListWorkouts_ForwardsStatusFilter(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"workouts":[]}`)
	st := "planned"
	_ = handleListWorkouts(context.Background(), c, ListWorkoutsArgs{
		From: "2026-06-01T00:00:00Z", To: "2026-06-30T00:00:00Z", Status: &st,
	})
	require.Len(t, *recs, 1)
	values, err := url.ParseQuery((*recs)[0].rawQuery)
	require.NoError(t, err)
	assert.Equal(t, "planned", values.Get("status"))
}

func TestPatchWorkout_ForwardsStatus(t *testing.T) {
	c, recs := newWorkoutRecorder(t, 200, `{"id":"w1"}`)
	st := "completed"
	_ = handlePatchWorkout(context.Background(), c, PatchWorkoutArgs{ID: "w1", Status: &st})
	require.Len(t, *recs, 1)
	var body map[string]any
	require.NoError(t, json.Unmarshal((*recs)[0].body, &body))
	assert.Equal(t, "completed", body["status"])
}

func ptrInt(v int) *int           { return &v }
func ptrFloat(v float64) *float64 { return &v }
