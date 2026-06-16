## 1. Migration

- [x] 1.1 Verify the on-disk migration head (currently `046`) and scaffold the next pair with `task migrate:new NAME=add_coach_recommendations`.
- [x] 1.2 Write the `.up.sql`: `CREATE TABLE coach_recommendations (id UUID PK DEFAULT gen_random_uuid(), date DATE NOT NULL, scope TEXT NOT NULL CHECK (scope IN ('fueling','training','recovery','race','general')), recommendation TEXT NOT NULL CHECK (length(recommendation) > 0), reason TEXT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now())` + an index on `(date)`. Write the matching `.down.sql` (drop index + table).

## 2. Capability package (`internal/coachrecs/`)

- [x] 2.1 `types.go`: `Recommendation` struct mirroring the row with JSON tags (`date` as `YYYY-MM-DD`); the validated `Scope` set + `ValidScope`.
- [x] 2.2 `repo.go`: `Create`, `ListWindow(from, to, scope *string)`, `GetByID`, `Delete` against `store.Querier`; `ErrNotFound` sentinel; list ordered `date DESC, created_at DESC`.
- [x] 2.3 `service.go`: validation (`recommendation` non-empty → `ErrRecommendationRequired`; `scope` in set → `ErrScopeInvalid`) with sentinel errors mapping 1:1 to API codes; thin pass-through to repo for reads/delete.
- [x] 2.4 `handlers.go`: `POST /coach/recommendations`, `GET /coach/recommendations` (from/to/tz + optional scope; inclusive local-date window), `GET /coach/recommendations/{id}`, `DELETE /coach/recommendations/{id}`, with swag annotations and a `Register(rg *gin.RouterGroup)`.
- [x] 2.5 Per-handler integration tests against testcontainers Postgres: create happy-path; `recommendation_required`; `scope_invalid`; `date_invalid`; window + newest-first ordering; scope filter; out-of-window exclusion; get/delete + 404s; idempotency replay on POST.

## 3. Wiring

- [x] 3.1 Instantiate repo+service and register handlers in `internal/httpserver/server.go` (within the authed group, alongside the other capabilities).

## 4. MCP tools

- [x] 4.1 Add a `registry_coachrecs.go` tool group in `internal/agenttools/`: `log_coach_recommendation` (TierWriteAuto), `list_coach_recommendations` (TierRead), `get_coach_recommendation` (TierRead), `delete_coach_recommendation` (TierWriteAuto) — each one HTTP call, args mirroring the endpoints; register the group.
- [x] 4.2 Bump the `mcp_integration_test.go` expected-tools surface (+4, registry-derived — confirm the assertion picks them up) and regenerate the announced-schema golden via `goldengen`.

## 5. Docs & verification

- [x] 5.1 Run `task swag` to regenerate `docs/` for the new endpoints + `Recommendation` shape.
- [x] 5.2 Run `task test` and `task vet`; confirm the new package passes and the MCP integration + golden tests pass with the four new tools. (Full `go test ./...` green — zero failures; `coachrecs`/`agenttools`/`httpserver`/`mcpserver` all ok; `go vet` clean; integration-tagged MCP test green.)
