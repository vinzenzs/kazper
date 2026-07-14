package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pmc_series → GET /performance/pmc with from/to/tz; read tier, no idempotency key.
func TestBuild_PMCSeries(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["pmc_series"]
	require.True(t, ok, "pmc_series must be registered on the MCP surface")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-01-01","to":"2026-07-01","tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/performance/pmc", call.Path)
	assert.Equal(t, "2026-01-01", call.Query.Get("from"))
	assert.Equal(t, "2026-07-01", call.Query.Get("to"))
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))
	assert.Empty(t, call.Body)
}

// tz is optional — omitted means the server uses DEFAULT_USER_TZ.
func TestBuild_PMCSeries_OmitsTZ(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["pmc_series"].Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30"}`))
	require.NoError(t, err)
	assert.Empty(t, call.Query.Get("tz"))
	assert.Empty(t, call.Query.Get("sport"))
}

// sport is forwarded when set (per-sport PMC).
func TestBuild_PMCSeries_Sport(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["pmc_series"].Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30","sport":"run"}`))
	require.NoError(t, err)
	assert.Equal(t, "run", call.Query.Get("sport"))
}

// pmc_target_trajectory → GET /performance/pmc/target-trajectory; macrocycle_id
// forwarded when set, omitted otherwise (server resolves the active macrocycle).
func TestBuild_PMCTargetTrajectory(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["pmc_target_trajectory"]
	require.True(t, ok, "pmc_target_trajectory must be registered")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"macrocycle_id":"abc-123","tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/performance/pmc/target-trajectory", call.Path)
	assert.Equal(t, "abc-123", call.Query.Get("macrocycle_id"))
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))
	assert.Empty(t, call.Body)

	// Omitted macrocycle_id is not forwarded (active-macrocycle default).
	call2, err := spec.Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Empty(t, call2.Query.Get("macrocycle_id"))
}
