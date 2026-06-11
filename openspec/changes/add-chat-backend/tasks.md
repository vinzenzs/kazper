# Tasks: add-chat-backend

## 1. Config & scaffolding

- [x] 1.1 Config additions in `internal/config`: `CHAT_MODEL` (default `claude-sonnet-4-6`), `CHAT_MAX_TOOL_ROUNDS` (8), `CHAT_MAX_HISTORY_MESSAGES`, `CHAT_REQUEST_TIMEOUT` (120s), `CHAT_DIETARY_PREFERENCES` (`vegetarian`); reuse `ANTHROPIC_API_KEY`
- [x] 1.2 `internal/chat` package skeleton: Anthropic streaming Messages client (typed `ErrAPIKeyMissing`, versioned `User-Agent: nutrition-chat/<version>`, anthropic-version header) following `internal/vision` conventions

## 2. Tool registry & loopback dispatch

- [x] 2.1 Tool registry: the 14 allowlisted custom tools with JSON schemas + descriptions; `web_search` server tool with Cookidoo `allowed_domains`
- [x] 2.2 Loopback dispatcher: one HTTP call per tool against own listen address, bearer forwarded, deterministic idempotency key derivation from conversation position (study `effectiveIdempotencyKey` in mcpserver)
- [x] 2.3 Resolve `get_race_fueling` shape against the existing races REST surface (design open question)
- [x] 2.4 Unit tests: registry completeness (exact allowlist, nothing more), key derivation determinism, dispatcher against a stub server

## 3. Agent loop & SSE

- [x] 3.1 Loop: stream upstream, execute tool_use blocks via dispatcher, feed tool_result back, repeat; round cap with forced tools-withheld final turn (`stop_reason: max_tool_rounds`)
- [x] 3.2 SSE writer: `text` / `tool` (name+status+summary only) / `done` (full message, stop_reason, usage) / `error` (typed codes incl. `upstream_unavailable`); flush correctness under Gin
- [x] 3.3 Request validation: reject `system` role in transcript (400), history truncation, 503 `chat_unavailable` pre-stream when key missing; per-request timeout
- [x] 3.4 System prompt template + config assembly per spec (scope, grounding mandate, 2–3 options contract, no-invented-nutriments, selection contract, per-serving conversion guidance)

## 4. Wiring & tests

- [x] 4.1 Wire `POST /chat` in `internal/httpserver` behind auth (idempotency middleware not applicable to the SSE endpoint itself — document why); swag annotation for text/event-stream
- [x] 4.2 Integration tests with a stubbed Anthropic upstream (httptest): happy path event sequence, tool dispatch through real middleware (request log + idempotency replay assertions), round-cap behavior, mid-stream upstream failure, system-role rejection
- [x] 4.3 Retry-safety test: identical resubmitted turn replays write tools without duplicates
- [x] 4.4 `task swag`; `task vet` + `task test` green; manual smoke against real Anthropic API with a "plan 3 dinners" conversation
