# Tasks: add-coach-context-endpoints

## 1. Backend aggregate package

- [x] 1.1 New `internal/coachcontext/` package: `types.go` (TrainingContext, RecoveryContext, lite shapes), `service.go` (wide-constructor Service composing workouts/fitnessmetrics/recoverymetrics/trainingphases repos via errgroup, no partial bundle), `handlers.go` (`GET /context/training`, `GET /context/recovery`, swag annotations).
- [x] 1.2 Clamp lookback/lookahead/days to sane bounds; round numerics via `numfmt`; ACWR derived (acute/chronic) only when both present.
- [x] 1.3 Wire the service + handlers in `internal/httpserver/server.go`.
- [x] 1.4 Tests: training bundle (phase + fitness + acwr + recent/upcoming), recovery bundle (latest + trend), quiet-history → null/empty, clamp behavior.

## 2. MCP dual-surface (D11)

- [x] 2.1 `internal/mcpserver/tools_coachcontext.go`: `get_training_context` / `get_recovery_context` MCP tools (one loopback call each); register in `server.go`.
- [x] 2.2 Add both names to `AnnouncedToolNames`; integration test still green.

## 3. Cross-cutting

- [x] 3.1 `task swag` (new handler response structs).
- [x] 3.2 `task vet`; `go test ./internal/coachcontext/... ./internal/mcpserver/...` green.
