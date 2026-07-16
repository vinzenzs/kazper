package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The expenditure read maps to exactly one GET; tz is forwarded only when set
// (the REST server owns the DEFAULT_USER_TZ fallback).
func TestExpenditure_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	spec, ok := specs["energy_expenditure"]
	require.True(t, ok, "tool energy_expenditure missing from MCPRegistry")
	assert.Equal(t, TierRead, spec.Tier)

	call, err := spec.Build(json.RawMessage(`{"from":"2026-03-01","to":"2026-03-28"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/nutrition/expenditure", call.Path)
	assert.Equal(t, "2026-03-01", call.Query.Get("from"))
	assert.Equal(t, "2026-03-28", call.Query.Get("to"))
	assert.False(t, call.Query.Has("tz"))
	assert.Nil(t, call.Body)

	withTZ, err := spec.Build(mustMarshal(t, EnergyExpenditureArgs{
		From: "2026-03-01",
		To:   "2026-03-28",
		TZ:   "Europe/Berlin",
	}))
	require.NoError(t, err)
	assert.Equal(t, "Europe/Berlin", withTZ.Query.Get("tz"))
}
