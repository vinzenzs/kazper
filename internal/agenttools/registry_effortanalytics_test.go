package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// power_profile → GET /workouts/power-profile with weight_kg/sex forwarded when
// present (read tier).
func TestBuild_PowerProfile(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["power_profile"]
	require.True(t, ok, "power_profile must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-04-15","to":"2026-07-14","weight_kg":72.5,"sex":"male"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/power-profile", call.Path)
	assert.Equal(t, "2026-04-15", call.Query.Get("from"))
	assert.Equal(t, "2026-07-14", call.Query.Get("to"))
	assert.Equal(t, "72.5", call.Query.Get("weight_kg"))
	assert.Equal(t, "male", call.Query.Get("sex"))
	assert.Empty(t, call.Body)
}

// Omitted weight_kg/sex are not forwarded (the endpoint applies its own
// stored-weight fallback + male default).
func TestBuild_PowerProfile_OmitsDefaults(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec := specs["power_profile"]
	call, err := spec.Build(json.RawMessage(`{"from":"2026-04-15","to":"2026-07-14"}`))
	require.NoError(t, err)
	assert.Empty(t, call.Query.Get("weight_kg"))
	assert.Empty(t, call.Query.Get("sex"))
}

// cp_model_history → GET /workouts/cp-model/history; window_days forwarded when set.
func TestBuild_CPModelHistory(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["cp_model_history"]
	require.True(t, ok, "cp_model_history must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-04-15","to":"2026-07-14","window_days":60}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/cp-model/history", call.Path)
	assert.Equal(t, "60", call.Query.Get("window_days"))

	// Omitted window_days is not forwarded (server default 90 applies).
	call2, err := spec.Build(json.RawMessage(`{"from":"2026-04-15","to":"2026-07-14"}`))
	require.NoError(t, err)
	assert.Empty(t, call2.Query.Get("window_days"))
}

// durability → GET /workouts/durability with from/to/tz (read tier).
func TestBuild_Durability(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["durability"]
	require.True(t, ok, "durability must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-04-15","to":"2026-07-14","tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/durability", call.Path)
	assert.Equal(t, "2026-04-15", call.Query.Get("from"))
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))
	assert.Empty(t, call.Body)
}
