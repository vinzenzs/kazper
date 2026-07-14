## 1. Service plumbing

- [x] 1.1 Add a `*slog.Logger` to `chat.Service` (default `slog.Default()`, injectable following the existing `Set*` wiring style) and a small helper that logs a persist failure with `session_id` + site
- [x] 1.2 Change `implicitlyRejectPending` to return `([]StoredTurn, error)`; caller in `stream` maps the error to `sse.error("persistence_error", …)` and returns before persisting the new user turn

## 2. Terminal persist handling per site

- [x] 2.1 `runLoop` terminal-answer site: on `AppendTurns` error, log + emit `persistence_error`, return without `sse.done`
- [x] 2.2 `runLoop` pause path: persist the awaiting-confirmation turn first; on error, log + emit `persistence_error` and return without `sse.proposal`/`sse.done`; success path emits `proposal` + `done` as before
- [x] 2.3 `runLoop` tool-round site: on `AppendTurns` error, log + emit `persistence_error`, return (no further rounds)
- [x] 2.4 `streamConfirm` execute site: on `AppendTurns` error, log + emit `persistence_error`, return without continuing the loop
- [x] 2.5 `stream` titling: log `SetTitleIfEmpty` failure at WARN, keep the stream unaffected

## 3. Tests (failure injection on the fake SessionStore)

- [x] 3.1 Extend the loop tests' fake `SessionStore` with per-call/per-site `AppendTurns` failure injection
- [x] 3.2 Test: terminal-answer persist failure → `error(persistence_error)` emitted, no `done`
- [x] 3.3 Test: pause-path persist failure → no `proposal`, no `done`, `error(persistence_error)`; and a follow-up `prepareConfirm` returns `nothing_to_confirm`
- [x] 3.4 Test: tool-round persist failure → loop stops after the failing round, `error(persistence_error)`, previously stored turns intact
- [x] 3.5 Test: implicit-reject persist failure → `error(persistence_error)`, stored history unchanged (no new user turn appended)
- [x] 3.6 Test: confirm-execute persist failure → `error(persistence_error)`, no continuation streamed
- [x] 3.7 Test: `SetTitleIfEmpty` failure → stream completes through `done`
- [x] 3.8 Confirm existing happy-path loop/confirm tests are untouched and green

## 4. Verification & wrap-up

- [x] 4.1 `go test -count=1 ./internal/chat/...` and `task vet` green; confirm no handler request/response structs changed (no `task swag` needed — SSE payloads are not in the OpenAPI surface)
- [x] 4.2 Update task states and propose the `fix(chat): …` commit
