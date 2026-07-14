package chat

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bufLogger returns a logger writing JSON to buf, for asserting persist-failure
// lines carry the session id + site.
func bufLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, nil))
}

// 3.2: a failed final-assistant-turn persist is terminal — persistence_error,
// no done, and the failure is logged with the session id.
func TestChatPersist_FinalAnswerFailureIsTerminal(t *testing.T) {
	turn := sseFrames(frameMessageStart, textBlockFrames("Here you go."), messageDelta("end_turn"), frameMessageStop)
	env := newLoopEnv(t, scriptedAnthropic(t, []string{turn}), Config{})
	var buf bytes.Buffer
	env.svc.SetLogger(bufLogger(&buf))
	env.store.failAppend = func(_ int, turns []StoredTurn) bool {
		return len(turns) == 1 && turns[0].Role == "assistant"
	}

	rec := postMsg(t, env, "hi")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, `"code":"persistence_error"`)
	assert.NotContains(t, body, "event: done")
	assert.Contains(t, body, "Here you go.", "already-streamed text is unaffected")
	assert.Contains(t, buf.String(), env.sessionID.String())
	assert.Contains(t, buf.String(), "final_answer")
}

// 3.3: a failed pause-path persist suppresses the proposal (no proposal, no
// done), and a follow-up confirm finds nothing to confirm.
func TestChatPersist_PauseFailureSuppressesProposal(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{schedTurn("c1", "2026-06-20", "ride")}), Config{})
	env.store.failAppend = func(_ int, turns []StoredTurn) bool {
		return len(turns) == 1 && turns[0].Role == "assistant"
	}

	body := fmt.Sprintf(`{"session_id":%q,"message":"schedule my ride"}`, env.sessionID.String())
	rec := postChat(t, env.engine, body)
	require.Equal(t, http.StatusOK, rec.Code)
	out := rec.Body.String()

	assert.Contains(t, out, `"code":"persistence_error"`)
	assert.NotContains(t, out, "event: proposal")
	assert.NotContains(t, out, "event: done")
	assert.EqualValues(t, 0, atomic.LoadInt32(env.scheduleCalls), "nothing dispatched")

	// The anchor was never stored, so the confirm endpoint sees nothing to confirm.
	_, code := env.svc.prepareConfirm(context.Background(), env.sessionID, nil)
	assert.Equal(t, "nothing_to_confirm", code)
}

// 3.4: a failed tool-round persist stops the loop after the failing round;
// previously stored turns stay intact.
func TestChatPersist_ToolRoundFailureStopsLoop(t *testing.T) {
	turn1 := sseFrames(frameMessageStart, toolUseFrames("t1", "get_daily_context", `{"date":"2026-06-12"}`), messageDelta("tool_use"), frameMessageStop)
	turn2 := sseFrames(frameMessageStart, textBlockFrames("done"), messageDelta("end_turn"), frameMessageStop)
	env := newLoopEnv(t, scriptedAnthropic(t, []string{turn1, turn2}), Config{})
	env.store.failAppend = func(_ int, turns []StoredTurn) bool { return len(turns) == 2 }

	rec := postMsg(t, env, "go")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, `"code":"persistence_error"`)
	assert.NotContains(t, body, "event: done")
	assert.EqualValues(t, 1, atomic.LoadInt32(env.contextCalls), "tool ran once then the loop stopped")

	turns := env.store.loaded(env.sessionID)
	require.Len(t, turns, 1, "only the user turn is durably stored")
	assert.Equal(t, "user", turns[0].Role)
}

// 3.5: a failed implicit-reject persist aborts before the new user message is
// stored; stored history is unchanged (still ending on the awaiting turn).
func TestChatPersist_ImplicitRejectFailureLeavesHistoryUnchanged(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{doneTurn("ok")}), Config{})
	env.store.seed(env.sessionID,
		StoredTurn{Role: "user", Content: []byte(`"schedule it"`)},
		pausedAssistantTurn("c1", "2026-06-20", "ride"),
	)
	before := env.store.loaded(env.sessionID)
	env.store.failAppend = func(_ int, turns []StoredTurn) bool {
		return len(turns) == 1 && turns[0].Role == "user" && hasBlockType(turns[0].Content, "tool_result")
	}

	body := fmt.Sprintf(`{"session_id":%q,"message":"never mind, dinner ideas?"}`, env.sessionID.String())
	rec := postChat(t, env.engine, body)
	require.Equal(t, http.StatusOK, rec.Code)
	out := rec.Body.String()

	assert.Contains(t, out, `"code":"persistence_error"`)
	assert.NotContains(t, out, "event: done")

	after := env.store.loaded(env.sessionID)
	require.Len(t, after, len(before), "no new turn appended")
	assert.Equal(t, "assistant", after[len(after)-1].Role, "still ends on the awaiting-confirmation turn")
	assert.Contains(t, string(after[len(after)-1].Content), "tool_use")
}

// 3.6: a failed confirm-execute persist ends the resume stream after the writes
// dispatched; no continuation is streamed and the failure is logged.
func TestChatPersist_ConfirmExecuteFailureEndsStream(t *testing.T) {
	env := newConfirmEnv(t, scriptedAnthropic(t, []string{schedTurn("c1", "2026-06-20", "ride"), doneTurn("Scheduled.")}), Config{})
	var buf bytes.Buffer
	env.svc.SetLogger(bufLogger(&buf))

	// Create the pause (must succeed).
	body := fmt.Sprintf(`{"session_id":%q,"message":"schedule it"}`, env.sessionID.String())
	require.Equal(t, http.StatusOK, postChat(t, env.engine, body).Code)
	require.EqualValues(t, 0, atomic.LoadInt32(env.scheduleCalls))

	// Fail the confirm-execute tool_result persist.
	env.store.failAppend = func(_ int, turns []StoredTurn) bool {
		return len(turns) == 1 && turns[0].Role == "user" && hasBlockType(turns[0].Content, "tool_result")
	}
	rec := postConfirm(t, env.engine, env.sessionID, `{"decisions":[{"tool_id":"c1","approve":true}]}`)
	require.Equal(t, http.StatusOK, rec.Code)
	out := rec.Body.String()

	assert.EqualValues(t, 1, atomic.LoadInt32(env.scheduleCalls), "the approved write did dispatch")
	assert.Contains(t, out, `"code":"persistence_error"`)
	assert.NotContains(t, out, "Scheduled.", "no continuation streamed")
	assert.NotContains(t, out, "event: done")
	assert.Contains(t, buf.String(), env.sessionID.String())
	assert.Contains(t, buf.String(), "confirm_execute")
}

// 3.7: a titling failure is logged but never affects the stream.
func TestChatPersist_TitlingFailureIsHarmless(t *testing.T) {
	turn := sseFrames(frameMessageStart, textBlockFrames("Hello."), messageDelta("end_turn"), frameMessageStop)
	env := newLoopEnv(t, scriptedAnthropic(t, []string{turn}), Config{})
	var buf bytes.Buffer
	env.svc.SetLogger(bufLogger(&buf))
	env.store.failTitle = true

	rec := postMsg(t, env, "hi there")
	require.Equal(t, http.StatusOK, rec.Code)
	body := rec.Body.String()

	assert.Contains(t, body, "event: done", "the stream completes through done")
	assert.Contains(t, body, "Hello.")
	assert.NotContains(t, body, "persistence_error")
	assert.Contains(t, buf.String(), "titling failed")
}
