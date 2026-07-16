package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkoutHeat_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["workout_heat"]
	require.True(t, ok, "tool workout_heat missing from MCPRegistry")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"workout_id":"w1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/w1/heat", call.Path)
	assert.Empty(t, call.Query)
	assert.Nil(t, call.Body)

	escaped, err := spec.Build(json.RawMessage(`{"workout_id":"a b/c"}`))
	require.NoError(t, err)
	assert.Equal(t, "/workouts/a%20b%2Fc/heat", escaped.Path)
}

// The place/lat-lon alternative on the location write.
func TestLogLocationPeriod_PlaceAndCoordinateForms(t *testing.T) {
	spec := ByName(MCPRegistry())["log_location_period"]

	// place-only: no coordinates are invented client-side.
	byPlace, err := spec.Build(json.RawMessage(
		`{"place":"Mallorca","start_date":"2026-07-20","end_date":"2026-07-28"}`))
	require.NoError(t, err)
	var placeBody map[string]any
	require.NoError(t, json.Unmarshal(byPlace.Body, &placeBody))
	assert.Equal(t, "Mallorca", placeBody["place"])
	assert.NotContains(t, placeBody, "lat")
	assert.NotContains(t, placeBody, "lon")
	assert.NotContains(t, placeBody, "name")

	// explicit coordinates still round-trip, including a real 0.
	byCoords, err := spec.Build(json.RawMessage(
		`{"name":"Null Island","start_date":"2026-07-20","end_date":"2026-07-21","lat":0,"lon":0}`))
	require.NoError(t, err)
	var coordBody map[string]any
	require.NoError(t, json.Unmarshal(byCoords.Body, &coordBody))
	require.Contains(t, coordBody, "lat")
	assert.Equal(t, float64(0), coordBody["lat"])
	assert.Equal(t, float64(0), coordBody["lon"])
	assert.Equal(t, "Null Island", coordBody["name"])
	assert.NotContains(t, coordBody, "place")
}
