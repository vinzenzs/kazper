package agenttools

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocations_BuildShapes(t *testing.T) {
	specs := ByName(MCPRegistry())

	log, ok := specs["log_location_period"]
	require.True(t, ok, "tool log_location_period missing from MCPRegistry")
	// A create: the dispatcher derives the Idempotency-Key when the agent
	// doesn't supply one.
	assert.Equal(t, TierWriteAuto, log.Tier)

	call, err := log.Build(json.RawMessage(
		`{"name":"Mallorca","start_date":"2026-07-20","end_date":"2026-07-28","lat":39.57,"lon":2.65}`))
	require.NoError(t, err)
	assert.Equal(t, "POST", call.Method)
	assert.Equal(t, "/locations", call.Path)
	assert.Empty(t, call.Query)

	var body map[string]any
	require.NoError(t, json.Unmarshal(call.Body, &body))
	assert.Equal(t, "Mallorca", body["name"])
	assert.Equal(t, "2026-07-20", body["start_date"])
	assert.Equal(t, "2026-07-28", body["end_date"])
	assert.InDelta(t, 39.57, body["lat"], 0.001)
	assert.InDelta(t, 2.65, body["lon"], 0.001)
	_, hasNote := body["note"]
	assert.False(t, hasNote, "an unset note must not be sent")

	// 0,0 is a real coordinate and must survive the round trip rather than
	// being omitted as a zero value.
	zero, err := log.Build(json.RawMessage(
		`{"name":"Null Island","start_date":"2026-07-20","end_date":"2026-07-21","lat":0,"lon":0}`))
	require.NoError(t, err)
	var zeroBody map[string]any
	require.NoError(t, json.Unmarshal(zero.Body, &zeroBody))
	require.Contains(t, zeroBody, "lat")
	require.Contains(t, zeroBody, "lon")
	assert.Equal(t, float64(0), zeroBody["lat"])
	assert.Equal(t, float64(0), zeroBody["lon"])

	note := "altitude camp, 2320 m"
	lat, lon := 37.09, -3.40
	withNote, err := log.Build(mustMarshal(t, LogLocationPeriodArgs{
		Name: "Sierra Nevada", StartDate: "2026-05-01", EndDate: "2026-05-21",
		Lat: &lat, Lon: &lon, Note: &note,
	}))
	require.NoError(t, err)
	var noteBody map[string]any
	require.NoError(t, json.Unmarshal(withNote.Body, &noteBody))
	assert.Equal(t, note, noteBody["note"])

	list, ok := specs["list_location_periods"]
	require.True(t, ok, "tool list_location_periods missing from MCPRegistry")
	assert.Equal(t, TierRead, list.Tier)

	lcall, err := list.Build(json.RawMessage(`{"from":"2026-07-01","to":"2026-07-31"}`))
	require.NoError(t, err)
	assert.Equal(t, "GET", lcall.Method)
	assert.Equal(t, "/locations", lcall.Path)
	assert.Equal(t, "2026-07-01", lcall.Query.Get("from"))
	assert.Equal(t, "2026-07-31", lcall.Query.Get("to"))
	assert.Nil(t, lcall.Body)
}
