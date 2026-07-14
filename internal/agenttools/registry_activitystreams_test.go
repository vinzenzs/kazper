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
