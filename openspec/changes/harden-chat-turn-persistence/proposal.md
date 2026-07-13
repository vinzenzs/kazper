## Why

The chat loop discards persistence errors at five sites in `internal/chat/service.go` (`AppendTurns` at the final-answer, pause-for-confirmation, mid-loop tool-round, implicit-reject, and confirm-execute persists; `SetTitleIfEmpty` after the user turn). A failed persist after streaming silently loses the assistant turn — the next request replays truncated history and the coach "forgets" its own answer; worse, a failed pause-path persist emits a proposal whose anchor turn was never stored, and a failed implicit-reject persist leaves the stored history malformed (a dangling `tool_use` mid-conversation that the upstream API rejects on every subsequent request). The initial user-turn persist is already correctly terminal (`persistence_error` SSE) — the other sites are inconsistent with it. Flagged as the top code-level fix in the 2026-07-13 gap analysis.

## What Changes

- **Every turn-persist failure becomes terminal for the stream**: on `AppendTurns` error the loop emits the existing typed SSE `error` event (`persistence_error`) and ends the stream **without** emitting `done` (or `proposal`, for the pause path) — the stream never signals success for state that isn't durably stored. This extends the exact policy the user-turn persist already follows to all five sites.
- **The pause path persists before proposing**: the awaiting-confirmation assistant turn is stored first; only then are `proposal` + `done(awaiting_confirmation)` emitted, so a surfaced proposal is always resumable by the confirm endpoint.
- **`implicitlyRejectPending` failure aborts the request** (signature gains an error) instead of proceeding to append a user message after an unanswered `tool_use` — preventing permanently malformed stored history.
- **Session titling stays best-effort** (`SetTitleIfEmpty` — cosmetic, never worth failing a stream) but the error is now **logged** instead of discarded; all persist failures are logged with session id via `log/slog`.
- No API-shape change: `persistence_error` is an existing typed SSE error code; no new endpoint, no migration, no MCP change, no `task swag` impact (SSE payloads aren't in the OpenAPI surface).

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `nutrition-chat`: ADDED requirement — turn-persistence failures are surfaced as terminal `persistence_error` events (never silent), the proposal event is only emitted after its anchor turn is durably stored, and titling remains best-effort-with-logging. Existing requirements are untouched (they mandate *that* turns persist; this adds *what happens when persisting fails*).

## Impact

- **Code**: `internal/chat/service.go` — `runLoop` (3 sites), `stream` (title logging), `implicitlyRejectPending` (error return), `streamConfirm` (1 site); a `*slog.Logger` on `Service` (defaulting to `slog.Default()`).
- **Tests**: `internal/chat` loop tests gain failure-injection cases on the existing fake `SessionStore` (per-site `AppendTurns` failures → `error` event asserted, no `done`/`proposal`; title failure → stream unaffected).
- **Clients**: the companion app and dashboard already render the typed SSE `error` event; they now see `persistence_error` in cases that previously looked like success. The confirm-execute site's failure mode (writes dispatched, result not stored → retry may re-execute) becomes *visible* instead of silent; its retry semantics are unchanged from today's dropped-stream case and noted in design.
- **Not affected**: REST/MCP surface, migrations, `docs/`.
