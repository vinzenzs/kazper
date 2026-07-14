package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// training_totals → GET /workouts/summary with from/to/tz; read tier.
func TestBuild_TrainingTotals(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["training_totals"]
	require.True(t, ok)
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30","tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/summary", call.Path)
	assert.Equal(t, "2026-06-01", call.Query.Get("from"))
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))
}

// intensity_distribution → GET /workouts/intensity-distribution; read tier, tz optional.
func TestBuild_IntensityDistribution(t *testing.T) {
	specs := ByName(MCPRegistry())
	spec, ok := specs["intensity_distribution"]
	require.True(t, ok, "intensity_distribution must be registered on the MCP surface")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-05-01","to":"2026-06-30","tz":"Europe/Berlin"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/workouts/intensity-distribution", call.Path)
	assert.Equal(t, "2026-05-01", call.Query.Get("from"))
	assert.Equal(t, "2026-06-30", call.Query.Get("to"))
	assert.Equal(t, "Europe/Berlin", call.Query.Get("tz"))
	assert.Empty(t, call.Body)

	// tz optional.
	call2, err := spec.Build(json.RawMessage(`{"from":"2026-06-01","to":"2026-06-30"}`))
	require.NoError(t, err)
	assert.Empty(t, call2.Query.Get("tz"))
}
