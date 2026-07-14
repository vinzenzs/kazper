package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plan_race_pacing → GET /races/{id}/pacing-plan, no query/body.
func TestBuild_PlanRacePacing(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["plan_race_pacing"]
	require.True(t, ok, "plan_race_pacing must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"race_id":"11111111-1111-1111-1111-111111111111"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/races/11111111-1111-1111-1111-111111111111/pacing-plan", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)
}

// set_race_leg_pacing_override → PUT /races/{id}/pacing-plan/overrides/{ordinal}.
// Write-confirm; PUT skips the idempotency header centrally, so Build sets no
// query. Only the supplied unit family appears in the body.
func TestBuild_SetRaceLegPacingOverride(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["set_race_leg_pacing_override"]
	require.True(t, ok, "set_race_leg_pacing_override must be registered")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteConfirm, spec.Tier)

	in := json.RawMessage(`{"race_id":"r1","ordinal":3,"target_power_low_w":190,"target_power_high_w":200,"note":"holding 195"}`)
	call, err := spec.Build(in)
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/races/r1/pacing-plan/overrides/3", call.Path)
	assert.Empty(t, call.Query)
	// ordinal + race_id are path segments, not body fields; only the populated
	// unit family and note are serialized.
	assert.JSONEq(t, `{"target_power_low_w":190,"target_power_high_w":200,"note":"holding 195"}`, string(call.Body))
}

// A run-family override serializes only the sec_per_km fields.
func TestBuild_SetRaceLegPacingOverride_RunFamily(t *testing.T) {
	specs := ByName(MCPRegistry())
	in := json.RawMessage(`{"race_id":"r1","ordinal":4,"target_pace_low_sec_per_km":300,"target_pace_high_sec_per_km":320}`)
	call, err := specs["set_race_leg_pacing_override"].Build(in)
	require.NoError(t, err)
	assert.Equal(t, "/races/r1/pacing-plan/overrides/4", call.Path)
	assert.JSONEq(t, `{"target_pace_low_sec_per_km":300,"target_pace_high_sec_per_km":320}`, string(call.Body))
}

// clear_race_leg_pacing_override → DELETE /races/{id}/pacing-plan/overrides/{ordinal}.
// Write-confirm; the idempotency_key arg is schema-only (never in path/body).
func TestBuild_ClearRaceLegPacingOverride(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["clear_race_leg_pacing_override"]
	require.True(t, ok, "clear_race_leg_pacing_override must be registered")
	assert.True(t, spec.Tier.IsWrite())
	assert.Equal(t, TierWriteConfirm, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"race_id":"r1","ordinal":3}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/races/r1/pacing-plan/overrides/3", call.Path)
	assert.Empty(t, call.Query)
	assert.Empty(t, call.Body)
}
