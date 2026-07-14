# Tasks — add A/B/C race priority

## 1. Migration

- [x] 1.1 Verify the current migration head: list `internal/store/migrations/` and take the next free sequential slot (head was `054_sync_run_summary_partial` at proposal time, but in-flight siblings — `add-race-pacing-plan`, `persist-activity-streams`, … — also claim slots; do not assume `055`).
- [x] 1.2 `task migrate:new NAME=add_race_priority`; up: `ALTER TABLE races ADD COLUMN priority TEXT CHECK (priority IN ('A','B','C'));` with a header comment carrying the nullable/no-default/no-backfill rationale (mirror `048_add_workout_training_focus`); down: `ALTER TABLE races DROP COLUMN IF EXISTS priority;`.

## 2. Types & repo (`internal/races/`)

- [x] 2.1 `types.go`: add a typed `Priority string` enum (`PriorityA`/`PriorityB`/`PriorityC`) with a `valid()` switch mirroring `Discipline`, and `Priority *Priority \`json:"priority,omitempty"\`` on `Race`.
- [x] 2.2 `repo.go`: add `priority` to `raceCols`, `InsertRace` (column + value), and `scanRace`.
- [x] 2.3 `repo.go`: extend `UpdateRaceParams` with `Priority *string` + `ClearPriority bool` and grow the `UpdateRace` `CASE WHEN` update so set writes the value, clear writes NULL, and neither leaves the column unchanged (the tri-state plumbing behind D3).
- [x] 2.4 `repo.go`: extend `ListRaces` with an optional priority filter (nil = no WHERE clause, current behavior).

## 3. Service & handlers (`internal/races/`)

- [x] 3.1 `service.go`: add `ErrPriorityInvalid = errors.New("race_priority_invalid")`; add `Priority *string` to `CreateInput` and `Priority *string` + `ClearPriority bool` to `UpdateInput`; validate non-nil values against the closed set in `Create` and `Update`; thread set/clear into `UpdateRaceParams`; add the priority filter to `List`.
- [x] 3.2 `handlers.go`: add `Priority *string` to `createRequest` and `patchRequest`; in `patch`, convert the empty-string sentinel (`*req.Priority == ""`) into `ClearPriority: true` (nil pointer = unchanged), matching the workouts `training_focus` handler pattern.
- [x] 3.3 `handlers.go`: in `list`, read the optional `priority` query param, validate it (invalid → `400 race_priority_invalid`), and pass it to the service; map `ErrPriorityInvalid` in `respondServiceError`.
- [x] 3.4 Update swag annotations: `priority` on the create/patch bodies (documenting the empty-string clear), the `priority` query param on `GET /races`, and `race_priority_invalid` added to the `@Failure 400` code lists on create, patch, and list.

## 4. MCP (`internal/agenttools/` + golden)

- [x] 4.1 `registry_races.go`: add `Priority *string` to `CreateRaceArgs` and `UpdateRaceArgs` (jsonschema description spelling out `A|B|C`, and on update the empty-string clear), forward it in both `Build` bodies; add an optional `Priority *string` filter to `ListRacesArgs` and forward it as the `priority` query param.
- [x] 4.2 Regenerate the announced-schema golden: `go test -tags=goldengen -run TestCaptureAnnouncedSchemas ./internal/mcpserver/` (registry-derived post `unify-mcp-tool-registry` — there is no hand-maintained expected-tools list to bump). Verify the diff to `internal/mcpserver/testdata/announced_schemas.json` touches only `create_race`, `update_race`, and `list_races`.
- [x] 4.3 Run the MCP integration test (`-tags=integration`) and the schema golden test to confirm the announced surface is green.

## 5. Tests (`internal/races/`)

- [x] 5.1 Create: POST with `"priority":"A"` persists and echoes it; POST with `"priority":"D"` (and lowercase `"a"`) returns `400 race_priority_invalid` and persists nothing; POST without priority yields a response with no `priority` key.
- [x] 5.2 PATCH tri-state: `{"priority":"B"}` sets (other fields preserved); `{"priority":""}` clears (subsequent GET omits the key); a PATCH omitting `priority` leaves it unchanged; invalid value returns `400 race_priority_invalid` without changing the row.
- [x] 5.3 List filter: with `A`/`C`/untriaged races stored, `GET /races?priority=A` returns only the A-race; omitting the param returns all three; `GET /races?priority=X` returns `400 race_priority_invalid`.
- [x] 5.4 Advisory stance: a race anchored by a macrocycle can be PATCHed to `"priority":"C"` with `200 OK` (no coupling error) — assert in the races or macrocycle integration tests, whichever wires both repos more cheaply.

## 6. Docs & verification

- [x] 6.1 `task swag` to regenerate `docs/` (confirm `priority` appears on the race schema and the list query param).
- [x] 6.2 `task vet` and `go test -count=1 ./internal/races/...` (plus `./internal/agenttools/...` and the mcpserver tests from 4.3); rerun any testcontainers parallel-boot flakes isolated with `-p 1`.
