## Context

`internal/chat/service.go` persists conversation turns at six sites. One (the pre-stream user-turn persist, `stream`) treats failure as terminal and emits the typed SSE `persistence_error`. The other five discard the error (`_ =`):

| Site | What's being persisted | Consequence of a silent failure today |
|---|---|---|
| `runLoop` final answer | the terminal assistant turn | answer streamed but lost from history; next request replays without it |
| `runLoop` pause path | the awaiting-confirmation assistant `tool_use` turn | `proposal` is emitted with no stored anchor — confirm finds `nothing_to_confirm` (409) for a card the user is looking at |
| `runLoop` tool round | assistant `tool_use` + `tool_result` pair (atomic append) | executed side effects vanish from history; the model re-answers without knowing what it did |
| `implicitlyRejectPending` | the synthetic declined `tool_result` | the request proceeds and appends the new user turn after an unanswered `tool_use` — stored history is malformed mid-conversation, `sanitizeHistory` only repairs leading/trailing turns, so **every subsequent request** sends an invalid message sequence upstream |
| `streamConfirm` execute | the `tool_result` turn for dispatched confirm writes | writes executed but unrecorded; a confirm retry re-enters `confirmExecute` and re-dispatches them |

`SetTitleIfEmpty` is also discarded — but titling is cosmetic and legitimately best-effort.

Constraint: SSE has already streamed text by the time most of these fire; there is no transactional "un-stream". The stream contract has exactly five event types (`text`/`tool`/`proposal`/`done`/`error`) and `persistence_error` is an existing error code — both clients already render it.

## Goals / Non-Goals

**Goals:**
- No persist failure is ever silent: terminal `persistence_error` for turn persists, logged best-effort for titling.
- A `done` event implies the conversation state it describes is durably stored; a `proposal` event implies its anchor turn is stored and resumable.
- Failure-injection test coverage for every site.

**Non-Goals:**
- No retry/queue/outbox for failed persists — Postgres is local to the deployment; if it's down, failing loudly is correct for a single-user system.
- No transactional coupling of tool dispatch and persist (side effects are loopback HTTP calls; they can't join a DB transaction).
- No change to the SSE event vocabulary, the confirm protocol, or `D5` dropped-stream recovery semantics.
- No fixing of the pre-existing confirm-retry re-execution window (see Risks) — this change makes it visible, not impossible.

## Decisions

1. **Uniform policy: turn-persist failure ⇒ emit `persistence_error`, end stream, emit nothing after it.**
   One rule for all five sites, matching the user-turn site that already exists — over per-site bespoke handling (e.g. "warn but still `done`" for the final answer). A `done` that lies about durability is exactly the silent-loss bug in politer clothing; the client keeps whatever text already streamed either way, and the error tells the user the reply didn't save. Alternative considered: a new non-terminal `warning` event — rejected, it grows the SSE vocabulary for one edge and both clients would need new handling.

2. **Pause path: persist first, then `proposal` + `done`.**
   Reorder so the awaiting-confirmation anchor is stored before anything is surfaced. If the persist fails, the user never sees a proposal card that the confirm endpoint would 409 on. This is a pure reorder — same turn content, same events on success.

3. **`implicitlyRejectPending` returns `([]StoredTurn, error)` and the caller aborts on error.**
   The alternative — proceed in-memory — permanently corrupts stored history (malformed mid-history `tool_use` with no reply), which is strictly worse than failing one request. `stream` maps the error to the same `persistence_error` SSE before the user turn is appended, so nothing new is stored on the failure path.

4. **`streamConfirm` execute-mode persist failure ends the stream after dispatch.**
   The writes have already executed; we cannot undo them. Emit `persistence_error` and stop — do not continue the loop on top of state that isn't stored (the continuation's own persist would strand further turns). The stored trailing turn remains the awaiting-confirmation anchor, so a retry re-enters `confirmExecute` — see Risks.

5. **Titling: log, don't fail.** `SetTitleIfEmpty` failure is logged at `WARN` with the session id. A stream that delivered a full answer should not error over a list-view label.

6. **Logging via `*slog.Logger` on `Service`, defaulting to `slog.Default()`.**
   Every persist failure (including the terminal ones) logs `ERROR` with `session_id` and the site, so the operator sees *why* clients are getting `persistence_error`. Injected (a `SetLogger` or `New` param following the existing `Set*` wiring style) so tests can assert against it; `slog.Default()` keeps `httpserver` wiring minimal.

## Risks / Trade-offs

- [Confirm retry after an execute-persist failure re-dispatches the approved writes] → Pre-existing exposure (identical to today's dropped-stream-before-persist case, D5 case *a*); this change only converts it from invisible to visible. Write tools reached via the loopback dispatcher carry the same idempotency behavior as any REST client using them; a true exactly-once fix (persist a dispatch-intent record first) is deliberately out of scope and can be a follow-up if it ever bites in practice.
- [Final-answer site: user sees a full reply followed by an error] → Correct but mildly confusing UX; the error message should say the *reply couldn't be saved* (client already displays typed error messages), not imply the reply is wrong.
- [A transient Postgres blip now fails streams that previously "succeeded"] → Those successes were losing data; failing is the honest behavior. Retry is one message away.
- [Behavior change under test fakes] → Existing loop tests that fake `AppendTurns` as always-succeeding are unaffected; new failure-injection cases are additive.

## Migration Plan

Single deploy, no data or schema migration, no client coordination needed (`persistence_error` is already handled by both clients). Rollback = revert the commit.

## Open Questions

- None blocking. The exactly-once confirm-execute follow-up (dispatch-intent record) is noted in Risks and deliberately deferred.
