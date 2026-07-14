package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// athlete_config_update → PUT /athlete-config, full-replace body. It is tiered
// write-confirm (the chat surface pauses; the MCP dispatcher runs it directly).
// PUT carries no Idempotency-Key — the dispatcher keys only POST/PATCH/DELETE,
// so there is nothing on HTTPCall to assert beyond the verb + path + body.
func TestBuild_AthleteConfigUpdate(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["athlete_config_update"]
	require.True(t, ok, "athlete_config_update must be registered on the MCP surface")
	assert.Equal(t, TierWriteConfirm, spec.Tier)
	assert.False(t, spec.OmitIdempotencyKey, "PUT is skipped method-side, not via OmitIdempotencyKey")

	in := json.RawMessage(`{"ftp_watts":270,"threshold_hr":168,"threshold_pace_sec_per_km":222.5,"hr_zone_1_max":120}`)
	call, err := spec.Build(in)
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/athlete-config", call.Path)
	assert.Empty(t, call.Query)
	assert.JSONEq(t,
		`{"ftp_watts":270,"threshold_hr":168,"threshold_pace_sec_per_km":222.5,"hr_zone_1_max":120}`,
		string(call.Body))
}

// An empty athlete_config_update clears the whole config: an empty JSON object
// body (full-replace clear-on-omit).
func TestBuild_AthleteConfigUpdate_EmptyClearsAll(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["athlete_config_update"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/athlete-config", call.Path)
	assert.JSONEq(t, `{}`, string(call.Body))
}
