package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The macrocycle domain contributes exactly these five MCP tools.
func TestMacrocycle_SurfaceAndTiers(t *testing.T) {
	specs := ByName(MCPRegistry())
	wantTier := map[string]Tier{
		"create_macrocycle": TierWriteAuto,
		"list_macrocycles":  TierRead,
		"get_macrocycle":    TierRead,
		"update_macrocycle": TierWriteAuto,
		"delete_macrocycle": TierWriteConfirm,
	}
	for name, tier := range wantTier {
		s, ok := specs[name]
		require.Truef(t, ok, "missing MCP tool %s", name)
		assert.Equalf(t, tier, s.Tier, "tier wrong for %s", name)
		assert.NotNilf(t, s.SchemaType, "tool %s must carry a SchemaType", name)
		assert.Truef(t, s.MCPExposed, "tool %s should be MCP-exposed", name)
	}
}

// create_macrocycle → POST /macrocycles; forwards fields and drops idempotency_key.
func TestMacrocycle_Create(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["create_macrocycle"].Build(json.RawMessage(
		`{"name":"2026 season","start_date":"2026-01-05","end_date":"2026-09-27","race_id":"r1","methodology":"why","notes":"n","idempotency_key":"k1"}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/macrocycles", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "2026 season", body["name"])
	assert.Equal(t, "2026-01-05", body["start_date"])
	assert.Equal(t, "r1", body["race_id"])
	assert.Equal(t, "why", body["methodology"])
	_, hasKey := body["idempotency_key"]
	assert.False(t, hasKey, "idempotency_key must not ride in the body")
}

func TestMacrocycle_ListAndGet(t *testing.T) {
	specs := ByName(MCPRegistry())

	listCall, err := specs["list_macrocycles"].Build(json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", listCall.Method)
	assert.Equal(t, "/macrocycles", listCall.Path)

	getCall, err := specs["get_macrocycle"].Build(json.RawMessage(`{"macrocycle_id":"m1"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", getCall.Method)
	assert.Equal(t, "/macrocycles/m1", getCall.Path)
}

// update_macrocycle → PATCH /macrocycles/{id}; tri-state race_id ("" clears).
func TestMacrocycle_Update(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["update_macrocycle"].Build(json.RawMessage(
		`{"macrocycle_id":"m1","end_date":"2026-10-04","race_id":""}`))
	require.NoError(t, err)
	assert.Equal(t, "PATCH", call.Method)
	assert.Equal(t, "/macrocycles/m1", call.Path)
	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "2026-10-04", body["end_date"])
	raceID, ok := body["race_id"]
	require.True(t, ok, "empty-string race_id must be forwarded to clear the anchor")
	assert.Equal(t, "", raceID)
	_, hasName := body["name"]
	assert.False(t, hasName, "omitted name must not appear in the patch body")
}

func TestMacrocycle_Delete(t *testing.T) {
	specs := ByName(MCPRegistry())
	call, err := specs["delete_macrocycle"].Build(json.RawMessage(`{"macrocycle_id":"m1","idempotency_key":"k"}`))
	require.NoError(t, err)
	assert.Equal(t, "DELETE", call.Method)
	assert.Equal(t, "/macrocycles/m1", call.Path)
}

// The phase write tools carry the four macrocycle/progression fields.
func TestPhaseTools_CarryMacrocycleFields(t *testing.T) {
	specs := ByName(MCPRegistry())

	createCall, err := specs["create_phase"].Build(json.RawMessage(
		`{"name":"build","type":"build","start_date":"2026-03-01","end_date":"2026-03-28","macrocycle_id":"m1","macrocycle_ordinal":2,"target_weekly_tss":620,"target_weekly_hours":11.5}`))
	require.NoError(t, err)
	var cBody map[string]any
	require.NoError(t, json.Unmarshal(createCall.Body, &cBody))
	assert.Equal(t, "m1", cBody["macrocycle_id"])
	assert.EqualValues(t, 2, cBody["macrocycle_ordinal"])
	assert.EqualValues(t, 620, cBody["target_weekly_tss"])
	assert.EqualValues(t, 11.5, cBody["target_weekly_hours"])

	// update_phase: empty-string macrocycle_id is forwarded to clear the link.
	updateCall, err := specs["update_phase"].Build(json.RawMessage(
		`{"phase_id":"p1","macrocycle_id":"","target_weekly_tss":700}`))
	require.NoError(t, err)
	var uBody map[string]any
	require.NoError(t, json.Unmarshal(updateCall.Body, &uBody))
	mid, ok := uBody["macrocycle_id"]
	require.True(t, ok, "empty-string macrocycle_id must be forwarded to clear the link")
	assert.Equal(t, "", mid)
	assert.EqualValues(t, 700, uBody["target_weekly_tss"])
}
