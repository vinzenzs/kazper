package workouts_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The stream-derived execution metrics (variability_index, efficiency_factor,
// decoupling_pct) are written ONLY by the activity-streams ingest/recompute
// path (persist-activity-streams). The workouts surface must echo them on read
// when set, omit them when NULL, and never accept them from a POST/PATCH body.

func TestExecutionMetrics_GetEchoesWhenSet(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", `{
        "external_id":"garmin:em1","source":"garmin","sport":"bike",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"
    }`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())

	// Simulate the activity-streams path writing the metrics onto the workout.
	require.NoError(t, f.repo.SetExecutionMetrics(context.Background(), w.ID, f64(1.08), f64(1.333), f64(4.2)))

	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	require.NotNil(t, got.VariabilityIndex)
	assert.InDelta(t, 1.08, *got.VariabilityIndex, 0.001)
	require.NotNil(t, got.EfficiencyFactor)
	assert.InDelta(t, 1.333, *got.EfficiencyFactor, 0.001)
	require.NotNil(t, got.DecouplingPct)
	assert.InDelta(t, 4.2, *got.DecouplingPct, 0.001)
}

func TestExecutionMetrics_OmittedWhenNull(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", `{
        "external_id":"garmin:em2","source":"garmin","sport":"bike",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"
    }`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())

	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, got.VariabilityIndex)
	assert.Nil(t, got.EfficiencyFactor)
	assert.Nil(t, got.DecouplingPct)

	// omitempty: the keys are absent from the JSON entirely.
	body := rec.Body.String()
	assert.NotContains(t, body, "variability_index")
	assert.NotContains(t, body, "efficiency_factor")
	assert.NotContains(t, body, "decoupling_pct")
}

func TestExecutionMetrics_PostBodyCannotWriteThem(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", `{
        "external_id":"garmin:em3","source":"garmin","sport":"bike",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z",
        "variability_index":1.5,"efficiency_factor":2.2,"decoupling_pct":9.9
    }`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())

	// The body values are ignored — the workout has no stream-derived metrics.
	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, got.VariabilityIndex)
	assert.Nil(t, got.EfficiencyFactor)
	assert.Nil(t, got.DecouplingPct)
}

func TestExecutionMetrics_PatchCannotWriteThem(t *testing.T) {
	f := setup(t)
	rec := doReq(t, f.r, http.MethodPost, "/workouts", `{
        "external_id":"garmin:em4","source":"garmin","sport":"bike",
        "started_at":"2026-06-07T08:00:00Z","ended_at":"2026-06-07T09:00:00Z"
    }`)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	w := decodeWorkout(t, rec.Body.Bytes())

	// PATCH's mutable allowlist excludes the stream-derived metrics entirely, so
	// even attempting to set one is rejected with field_immutable (not silently
	// ignored) — a strictly stronger guarantee.
	prec := doReq(t, f.r, http.MethodPatch, "/workouts/"+w.ID.String(),
		`{"variability_index":1.5}`)
	require.Equal(t, http.StatusBadRequest, prec.Code, prec.Body.String())
	assert.Contains(t, prec.Body.String(), "field_immutable")

	got, code := getWorkout(t, f, w.ID)
	require.Equal(t, http.StatusOK, code)
	assert.Nil(t, got.VariabilityIndex)
	assert.Nil(t, got.EfficiencyFactor)
	assert.Nil(t, got.DecouplingPct)
}
