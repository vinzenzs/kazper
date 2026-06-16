## Why

When the coach reasons "today's carb target is 220g because of tomorrow's long ride," that rationale is ephemeral — it lives only in the chat turn and is reconstructed from scratch next session. There's no way for the coach to look back at "what did I advise last week, and did it hold?" Recommendations are the one synthesis output worth keeping: they're dated, scoped, and referenced repeatedly. Priorities #6F flags this as a **deliberate** test of the project's "agent does synthesis, API records primitives" principle — so this change is scoped narrowly to honor it: the API gains a **thin append-and-read log** for recommendations the agent has *already* synthesized. The server never generates, ranks, or interprets a recommendation; it stores the text the agent wrote and hands it back, exactly like `meals` stores an intake the user logged.

## What Changes

- Add a new **`coach-recommendations`** capability — a small CRUD primitive over a `coach_recommendations` table (a dated log), in its own `internal/coachrecs/` package mirroring the standard capability shape.
- Endpoints (all authenticated):
  - **`POST /coach/recommendations`** — record one recommendation: `date` (the day it applies to), `scope` (a small validated set: `fueling` | `training` | `recovery` | `race` | `general`), `recommendation` (the synthesized advice text, required), and optional `reason` (the rationale). Returns the stored row. Honors the idempotency middleware on POST.
  - **`GET /coach/recommendations`** — list over a `[from, to]` local-date window (the standard `tz` handling), with an optional `scope` filter, newest-first.
  - **`GET /coach/recommendations/{id}`** — fetch one.
  - **`DELETE /coach/recommendations/{id}`** — remove a superseded/incorrect entry (corrections are deletes + re-log, matching the log-primitive shape; no PATCH).
- Mirror each endpoint 1:1 as MCP tools (`log_coach_recommendation`, `list_coach_recommendations`, `get_coach_recommendation`, `delete_coach_recommendation`) so the coach can write a recommendation as it makes it and ground on prior ones next session. Write tools auto-derive an idempotency key; the MCP expected-tools list gains four entries and the announced-schema golden is regenerated.
- One additive migration creating `coach_recommendations` (verify the on-disk head first — `046` currently, so likely `047`).

## Capabilities

### New Capabilities
- `coach-recommendations`: a dated, scoped append-and-read log of coach-authored recommendations (recommendation text + optional reason), with windowed list, single get, and delete. A storage primitive only — no server-side synthesis, ranking, or interpretation.

### Modified Capabilities
<!-- none — the four MCP tools are captured as a requirement in the new coach-recommendations spec (the MCP-mirror pattern recent changes follow), and the mcp-server integration expected-tools list + announced-schema golden are updated as implementation tasks, not a spec-behavior change. -->

## Impact

- **Code**: new `internal/coachrecs/` (`types.go`/`repo.go`/`service.go`/`handlers.go` + tests); route registration in `internal/httpserver/server.go`; new tool group in `internal/agenttools/` (e.g. `registry_coachrecs.go`); `internal/mcpserver/mcp_integration_test.go` expected-tools list (+4) and the announced-schema golden.
- **Migration**: one additive `NNN_add_coach_recommendations` pair (no change to existing tables).
- **Docs**: `task swag` for the new endpoints + shapes.
- **Principle / scope**: deliberately a primitive, not synthesis — the design records *why* this stays consistent with "agent synthesizes, API records." No coupling to goals/overrides (a recommendation does not mutate a nutrition target; the agent still writes goal overrides separately if it wants the number enforced).
