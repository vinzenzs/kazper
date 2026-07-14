package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// log_wellness → PUT /wellness/{date} with a partial body, no Idempotency-Key.
func TestBuild_LogWellness(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["log_wellness"]
	require.True(t, ok, "log_wellness must be registered")
	assert.True(t, spec.Tier.IsWrite())

	call, err := spec.Build(json.RawMessage(`{"date":"2026-07-14","soreness":4,"note":"heavy legs"}`))
	require.NoError(t, err)
	assert.Equal(t, "PUT", call.Method)
	assert.Equal(t, "/wellness/2026-07-14", call.Path)

	// Only the provided fields marshal (omitempty) — full-replace clears the rest.
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.EqualValues(t, 4, body["soreness"])
	assert.Equal(t, "heavy legs", body["note"])
	_, hasFatigue := body["fatigue"]
	assert.False(t, hasFatigue, "unset score must be omitted from the PUT body")
}

// wellness_correlation → GET /wellness/correlation; metric forwarded when set.
func TestBuild_WellnessCorrelation(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["wellness_correlation"]
	require.True(t, ok, "wellness_correlation must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30","metric":"ctl"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/wellness/correlation", call.Path)
	assert.Equal(t, "ctl", call.Query.Get("metric"))

	// Omitted metric is not forwarded (server default tsb applies).
	call2, err := spec.Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30"}`))
	require.NoError(t, err)
	assert.Empty(t, call2.Query.Get("metric"))
}

// list_wellness → GET /wellness with from/to query params (read tier).
func TestBuild_ListWellness(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["list_wellness"]
	require.True(t, ok, "list_wellness must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-07-14"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/wellness", call.Path)
	assert.Equal(t, "2026-06-01", call.Query.Get("from"))
	assert.Equal(t, "2026-07-14", call.Query.Get("to"))
	assert.Empty(t, call.Body)
}
