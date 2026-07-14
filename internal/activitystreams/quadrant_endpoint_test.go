package activitystreams_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vinzenzs/kazper/internal/activitystreams"
)

// seedPowerCadence seeds a workout with paired power + cadence streams.
func seedPowerCadence(t *testing.T, f *fixture, durS int, watts, rpm float64) string {
	t.Helper()
	id := seedWorkout(t, f.repo).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+id+"/streams",
			fmt.Sprintf(`{"power":%s,"cadence":%s}`, arr(durS, watts), arr(durS, rpm))).Code)
	return id
}

func getQuadrant(t *testing.T, f *fixture, id, query string) (activitystreams.QuadrantResult, int) {
	t.Helper()
	rec := get(t, f.r, "/workouts/"+id+"/quadrant?"+query)
	if rec.Code != http.StatusOK {
		return activitystreams.QuadrantResult{}, rec.Code
	}
	var res activitystreams.QuadrantResult
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	return res, rec.Code
}

func TestQuadrantEndpoint_HappyPath(t *testing.T) {
	f := setup(t)
	// 300 W @ 60 rpm vs a 250 W/90 rpm reference → grinding (quadrant II).
	id := seedPowerCadence(t, f, 1200, 300, 60)

	res, code := getQuadrant(t, f, id, "cp_watts=250&cadence_rpm=90")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 250.0, res.Params.CPWatts)
	assert.Equal(t, 90.0, res.Params.CadenceRPM)
	assert.Equal(t, 172.5, res.Params.CrankMM) // defaulted
	assert.Equal(t, 1200, res.Summary.PedalingS)
	assert.InDelta(t, 100.0, res.Summary.Q2Pct, 0.1)
	assert.NotEmpty(t, res.Scatter)
	assert.LessOrEqual(t, len(res.Scatter), 1000)
}

func TestQuadrantEndpoint_SummaryOnly(t *testing.T) {
	f := setup(t)
	id := seedPowerCadence(t, f, 1200, 300, 60)
	res, code := getQuadrant(t, f, id, "cp_watts=250&cadence_rpm=90&summary_only=true")
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, res.Scatter)
	assert.Equal(t, 1200, res.Summary.PedalingS) // summary still computed
}

func TestQuadrantEndpoint_CrankParam(t *testing.T) {
	f := setup(t)
	id := seedPowerCadence(t, f, 60, 250, 90)
	res, code := getQuadrant(t, f, id, "cp_watts=250&cadence_rpm=90&crank_mm=165")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, 165.0, res.Params.CrankMM)
}

func TestQuadrantEndpoint_ParamValidation(t *testing.T) {
	f := setup(t)
	id := seedPowerCadence(t, f, 60, 250, 90)
	cases := []struct{ name, query, wantErr string }{
		{"missing cp", "cadence_rpm=90", "cp_invalid"},
		{"zero cp", "cp_watts=0&cadence_rpm=90", "cp_invalid"},
		{"missing cadence", "cp_watts=250", "cadence_invalid"},
		{"zero cadence", "cp_watts=250&cadence_rpm=0", "cadence_invalid"},
		{"crank too small", "cp_watts=250&cadence_rpm=90&crank_mm=50", "crank_invalid"},
		{"crank too large", "cp_watts=250&cadence_rpm=90&crank_mm=300", "crank_invalid"},
		{"crank nonnumeric", "cp_watts=250&cadence_rpm=90&crank_mm=abc", "crank_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := get(t, f.r, "/workouts/"+id+"/quadrant?"+tc.query)
			require.Equal(t, http.StatusBadRequest, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.wantErr, body["error"])
		})
	}
}

func TestQuadrantEndpoint_SentinelMatrix(t *testing.T) {
	f := setup(t)
	// Unknown workout.
	rec := get(t, f.r, "/workouts/"+seedWorkoutMissing()+"/quadrant?cp_watts=250&cadence_rpm=90")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "workout_not_found")

	// No streams at all.
	bare := seedWorkout(t, f.repo).String()
	rec = get(t, f.r, "/workouts/"+bare+"/quadrant?cp_watts=250&cadence_rpm=90")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "streams_not_found")

	// Streams but no power (HR only).
	hrOnly := seedWorkout(t, f.repo).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+hrOnly+"/streams", fmt.Sprintf(`{"heart_rate":%s}`, arr(600, 150))).Code)
	rec = get(t, f.r, "/workouts/"+hrOnly+"/quadrant?cp_watts=250&cadence_rpm=90")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "power_stream_missing")

	// Power but no cadence.
	powerOnly := seedWorkout(t, f.repo).String()
	require.Equal(t, http.StatusOK,
		post(t, f.r, "/workouts/"+powerOnly+"/streams", fmt.Sprintf(`{"power":%s}`, arr(600, 250))).Code)
	rec = get(t, f.r, "/workouts/"+powerOnly+"/quadrant?cp_watts=250&cadence_rpm=90")
	require.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "cadence_stream_missing")
}

func TestQuadrantEndpoint_ReadOnly(t *testing.T) {
	f := setup(t)
	id := seedPowerCadence(t, f, 600, 280, 85)
	a := get(t, f.r, "/workouts/"+id+"/quadrant?cp_watts=250&cadence_rpm=90")
	b := get(t, f.r, "/workouts/"+id+"/quadrant?cp_watts=250&cadence_rpm=90")
	assert.Equal(t, a.Body.String(), b.Body.String())
	assert.NotContains(t, a.Body.String(), "kcal")
}
