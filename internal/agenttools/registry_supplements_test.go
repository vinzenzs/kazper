package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild_LogSupplement(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["log_supplement"]
	require.True(t, ok, "log_supplement must be registered")
	assert.True(t, spec.Tier.IsWrite())

	call, err := spec.Build(json.RawMessage(`{"name":"creatine","logged_at":"2026-07-14T08:00:00Z","dose":5,"dose_unit":"g"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/supplements", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "creatine", body["name"])
	assert.EqualValues(t, 5, body["dose"])
	assert.Equal(t, "g", body["dose_unit"])
}

func TestBuild_ListSupplements(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["list_supplements"]
	require.True(t, ok)
	assert.Equal(t, TierRead, spec.Tier)
	call, err := spec.Build(json.RawMessage(`{"from":"2026-06-01T00:00:00Z","to":"2026-07-01T00:00:00Z"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/supplements", call.Path)
	assert.Equal(t, "2026-06-01T00:00:00Z", call.Query.Get("from"))
}
