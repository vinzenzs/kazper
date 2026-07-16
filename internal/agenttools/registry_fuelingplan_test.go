package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkoutFuelingPlan_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["workout_fueling_plan"]
	require.True(t, ok, "tool workout_fueling_plan missing from MCPRegistry")
	assert.Equal(t, TierRead, spec.Tier)

	// Required id only → a bare GET; the capacity clamp is opt-in, so an agent
	// that doesn't know the athlete's gut must not send one.
	call, err := spec.Build(json.RawMessage(`{"workout_id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/w1/fueling-plan", call.Path)
	assert.False(t, call.Query.Has("carbs_per_hr"))
	assert.Nil(t, call.Body)

	capacity := 70.0
	withCap, err := spec.Build(mustMarshal(t, WorkoutFuelingPlanArgs{
		WorkoutID:  "w1",
		CarbsPerHr: &capacity,
	}))
	require.NoError(t, err)
	assert.Equal(t, "70", withCap.Query.Get("carbs_per_hr"))

	// Path params are escaped, like every sibling workout tool.
	escaped, err := spec.Build(json.RawMessage(`{"workout_id":"a b/c"}`))
	require.NoError(t, err)
	assert.Equal(t, "/workouts/a%20b%2Fc/fueling-plan", escaped.Path)
}
