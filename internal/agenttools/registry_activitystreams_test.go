package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// w_prime_balance → GET /workouts/{id}/w-prime-balance with cp/w′ as query params
// and summary_only=true hardcoded (the agent never gets the raw series).
func TestBuild_WPrimeBalance(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["w_prime_balance"]
	require.True(t, ok, "w_prime_balance must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"workout_id":"w1","cp_watts":262,"w_prime_kj":21.5}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/w1/w-prime-balance", call.Path)
	assert.Equal(t, "262", call.Query.Get("cp_watts"))
	assert.Equal(t, "21.5", call.Query.Get("w_prime_kj"))
	assert.Equal(t, "true", call.Query.Get("summary_only"))
	assert.Empty(t, call.Body)
}

// detect_intervals → GET /workouts/{id}/intervals (read tier, full body).
func TestBuild_DetectIntervals(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["detect_intervals"]
	require.True(t, ok, "detect_intervals must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"workout_id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/w1/intervals", call.Path)
	assert.Empty(t, call.Body)
}

// quadrant_analysis → GET /workouts/{id}/quadrant with cp/cadence + hardcoded
// summary_only=true (read tier; the scatter stays chart-only).
func TestBuild_QuadrantAnalysis(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["quadrant_analysis"]
	require.True(t, ok, "quadrant_analysis must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"workout_id":"w1","cp_watts":262,"cadence_rpm":90}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/w1/quadrant", call.Path)
	assert.Equal(t, "262", call.Query.Get("cp_watts"))
	assert.Equal(t, "90", call.Query.Get("cadence_rpm"))
	assert.Equal(t, "true", call.Query.Get("summary_only"))
	assert.Empty(t, call.Query.Get("crank_mm")) // omitted → default applied server-side
}

// recompute_workout_streams → POST /workouts/{id}/streams/recompute (write tier).
func TestBuild_RecomputeWorkoutStreams(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["recompute_workout_streams"]
	require.True(t, ok)
	assert.True(t, spec.Tier.IsWrite())

	call, err := spec.Build(json.RawMessage(`{"workout_id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/workouts/w1/streams/recompute", call.Path)
}

func TestStrideAnalysis_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["stride_analysis"]
	require.True(t, ok, "tool stride_analysis missing from MCPRegistry")
	assert.Equal(t, TierRead, spec.Tier)

	// summary_only is ALWAYS applied: the scatter is chart data, never
	// reasoning data (the quadrant/W′bal convention).
	call, err := spec.Build(json.RawMessage(`{"workout_id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/w1/stride", call.Path)
	assert.Equal(t, "true", call.Query.Get("summary_only"))
	assert.False(t, call.Query.Has("min_speed_mps"), "the cutoff is opt-in")
	assert.Nil(t, call.Body)

	min := 1.8
	withMin, err := spec.Build(mustMarshal(t, StrideAnalysisArgs{WorkoutID: "w1", MinSpeedMps: &min}))
	require.NoError(t, err)
	assert.Equal(t, "1.8", withMin.Query.Get("min_speed_mps"))
	assert.Equal(t, "true", withMin.Query.Get("summary_only"), "still summary-only")

	escaped, err := spec.Build(json.RawMessage(`{"workout_id":"a b/c"}`))
	require.NoError(t, err)
	assert.Equal(t, "/workouts/a%20b%2Fc/stride", escaped.Path)
}
