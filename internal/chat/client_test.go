package chat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sseFrames joins pre-built `event:/data:` frames into one stream body.
func sseFrames(frames ...string) string { return strings.Join(frames, "") }

const (
	frameMessageStart = "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":12,\"output_tokens\":0}}}\n\n"
	frameMessageStop  = "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
)

func textBlockFrames(text string) string {
	return "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"" + text + "\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"
}

func toolUseFrames(id, name, inputJSON string) string {
	// partial_json must be a JSON-escaped string inside the data payload.
	esc := strings.ReplaceAll(inputJSON, `"`, `\"`)
	return "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"" + id + "\",\"name\":\"" + name + "\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"" + esc + "\"}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n"
}

func messageDelta(stopReason string) string {
	return "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"" + stopReason + "\"},\"usage\":{\"output_tokens\":7}}\n\n"
}

func TestParseSSE_TextTurn(t *testing.T) {
	body := sseFrames(frameMessageStart, textBlockFrames("Hello world"), messageDelta("end_turn"), frameMessageStop)
	var streamed strings.Builder
	res, err := parseSSE(strings.NewReader(body), func(d string) { streamed.WriteString(d) })
	require.NoError(t, err)
	assert.Equal(t, "Hello world", res.Text)
	assert.Equal(t, "Hello world", streamed.String())
	assert.Equal(t, "end_turn", res.StopReason)
	assert.Empty(t, res.ClientToolCalls)
	assert.Equal(t, 12, res.Usage.InputTokens)
	assert.Equal(t, 7, res.Usage.OutputTokens)
}

func TestParseSSE_ToolUseTurn(t *testing.T) {
	body := sseFrames(frameMessageStart,
		toolUseFrames("toolu_1", "get_daily_context", `{"date":"2026-06-12"}`),
		messageDelta("tool_use"), frameMessageStop)
	res, err := parseSSE(strings.NewReader(body), nil)
	require.NoError(t, err)
	assert.Equal(t, "tool_use", res.StopReason)
	require.Len(t, res.ClientToolCalls, 1)
	assert.Equal(t, "get_daily_context", res.ClientToolCalls[0].Name)
	assert.Equal(t, "toolu_1", res.ClientToolCalls[0].ID)
	assert.JSONEq(t, `{"date":"2026-06-12"}`, string(res.ClientToolCalls[0].Input))
	// The assistant content is echo-able and includes the tool_use block.
	require.Len(t, res.AssistantContent, 1)
	assert.Contains(t, string(res.AssistantContent[0]), "tool_use")
}

func TestParseSSE_ErrorEventIsProtocolError(t *testing.T) {
	body := frameMessageStart + "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"busy\"}}\n\n"
	_, err := parseSSE(strings.NewReader(body), nil)
	require.Error(t, err)
	var proto *ErrUpstreamProtocol
	assert.ErrorAs(t, err, &proto)
}

func TestParseSSE_MixedTextThenTool(t *testing.T) {
	body := sseFrames(frameMessageStart,
		textBlockFrames("Let me check. "),
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"t2\",\"name\":\"search_products\"}}\n\n"+
			"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"q\\\":\\\"lentil\\\"}\"}}\n\n"+
			"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":1}\n\n",
		messageDelta("tool_use"), frameMessageStop)
	res, err := parseSSE(strings.NewReader(body), nil)
	require.NoError(t, err)
	assert.Equal(t, "Let me check. ", res.Text)
	require.Len(t, res.ClientToolCalls, 1)
	assert.Equal(t, "search_products", res.ClientToolCalls[0].Name)
	assert.Len(t, res.AssistantContent, 2) // text block + tool_use block
}
