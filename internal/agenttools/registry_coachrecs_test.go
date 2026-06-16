package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The coach-recommendations domain contributes exactly these four MCP tools.
func TestCoachRecs_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"log_coach_recommendation":    TierWriteAuto,
		"list_coach_recommendations":  TierRead,
		"get_coach_recommendation":    TierRead,
		"delete_coach_recommendation": TierWriteAuto,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType for MCP schema reflection", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// log_coach_recommendation → POST /coach/recommendations; emits date/scope/
// recommendation, includes reason when supplied, and drops idempotency_key from
// the body.
func TestCoachRecs_Log(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["log_coach_recommendation"].Build(json.RawMessage(
		`{"date":"2026-06-17","scope":"fueling","recommendation":"220g carbs","reason":"long ride","idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/coach/recommendations", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "2026-06-17", body["date"])
	assert.Equal(t, "fueling", body["scope"])
	assert.Equal(t, "220g carbs", body["recommendation"])
	assert.Equal(t, "long ride", body["reason"])
	_, hasKey := body["idempotency_key"]
	assert.False(t, hasKey, "idempotency_key must not ride in the body")
}

// list_coach_recommendations → GET with from/to and optional tz/scope.
func TestCoachRecs_List(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["list_coach_recommendations"].Build(json.RawMessage(
		`{"from":"2026-06-15","to":"2026-06-18","scope":"recovery"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", call.Method)
	assert.Equal(t, "/coach/recommendations", call.Path)
	assert.Equal(t, "2026-06-15", call.Query.Get("from"))
	assert.Equal(t, "2026-06-18", call.Query.Get("to"))
	assert.Equal(t, "recovery", call.Query.Get("scope"))
}

// get/delete → GET/DELETE /coach/recommendations/{id}.
func TestCoachRecs_GetAndDelete(t *testing.T) {
	specs := ByName(MCPRegistry())
	get, err := specs["get_coach_recommendation"].Build(json.RawMessage(`{"id":"abc"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", get.Method)
	assert.Equal(t, "/coach/recommendations/abc", get.Path)

	del, err := specs["delete_coach_recommendation"].Build(json.RawMessage(`{"id":"abc","idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", del.Method)
	assert.Equal(t, "/coach/recommendations/abc", del.Path)
}
