package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuelPlan_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["fuel_plan"]
	require.True(t, ok, "tool fuel_plan missing from MCPRegistry")
	assert.Equal(t, TierRead, spec.Tier)

	// No args → a bare GET; the REST server owns the today+6 default, so the
	// tool must not invent bounds of its own.
	call, err := spec.Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/nutrition/fuel-plan", call.Path)
	assert.Empty(t, call.Query)
	assert.Nil(t, call.Body)

	full, err := spec.Build(mustMarshal(t, FuelPlanArgs{
		From: "2026-07-20",
		To:   "2026-07-26",
		TZ:   "Europe/Berlin",
	}))
	require.NoError(t, err)
	assert.Equal(t, "2026-07-20", full.Query.Get("from"))
	assert.Equal(t, "2026-07-26", full.Query.Get("to"))
	assert.Equal(t, "Europe/Berlin", full.Query.Get("tz"))

	// tz alone is forwarded without fabricating a window.
	tzOnly, err := spec.Build(mustMarshal(t, FuelPlanArgs{TZ: "Europe/Berlin"}))
	require.NoError(t, err)
	assert.Equal(t, "Europe/Berlin", tzOnly.Query.Get("tz"))
	assert.False(t, tzOnly.Query.Has("from"))
	assert.False(t, tzOnly.Query.Has("to"))
}
