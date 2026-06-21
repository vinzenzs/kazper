## 1. Migration

- [x] 1.1 `task migrate:new NAME=add_macrocycles`; verify `048` is still the head before claiming `049`.
- [x] 1.2 In `049_*.up.sql`: `CREATE TABLE macrocycles (id UUID PK, name TEXT NOT NULL, start_date DATE NOT NULL, end_date DATE NOT NULL, race_id UUID NULL REFERENCES races(id) ON DELETE SET NULL, methodology TEXT NULL, notes TEXT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), updated_at TIMESTAMPTZ NOT NULL DEFAULT now(), CHECK (start_date <= end_date));`.
- [x] 1.3 In the same `up.sql`: `ALTER TABLE training_phases ADD COLUMN macrocycle_id UUID NULL REFERENCES macrocycles(id) ON DELETE SET NULL`, `ADD COLUMN macrocycle_ordinal INT NULL`, `ADD COLUMN target_weekly_tss NUMERIC NULL CHECK (target_weekly_tss IS NULL OR target_weekly_tss >= 0)`, `ADD COLUMN target_weekly_hours NUMERIC NULL CHECK (target_weekly_hours IS NULL OR target_weekly_hours >= 0)`; add `CREATE INDEX training_phases_macrocycle_id_idx ON training_phases (macrocycle_id);`.
- [x] 1.4 In `049_*.down.sql`: drop the four `training_phases` columns (and the index), then `DROP TABLE macrocycles;`.

## 2. macrocycle package — types & repo

- [x] 2.1 Create `internal/macrocycle/types.go`: `Macrocycle` struct (mirrors the row + `RaceName *string omitempty` convenience + `Phases []*MemberPhase omitempty` for the nested read), `MemberPhase` lite (id, name, type, start/end, `macrocycle_ordinal`, `target_weekly_tss`, `target_weekly_hours`), `CreateInput`, `PatchInput` (with `RaceID *string` + `ClearRaceID bool` tri-state).
- [x] 2.2 Create `internal/macrocycle/repo.go` against `store.Querier`: `Create`, `List` (ORDER BY `start_date DESC`), `GetByID` (joins member phases ORDER BY `macrocycle_ordinal NULLS LAST, start_date`), `Patch`, `Delete`, and `MacrocycleFor(date)` (most-recently-updated covering — `ORDER BY updated_at DESC LIMIT 1`), plus `RaceNameFor`/join for the convenience field.
- [x] 2.3 Add an `ErrMacrocycleNotFound` sentinel (and reuse the races repo / a lookup for `race_not_found`).

## 3. macrocycle package — service & handlers

- [x] 3.1 Create `internal/macrocycle/service.go`: validation → sentinels mapping 1:1 to error codes — `date_range_invalid`, `macrocycle_name_invalid`, `macrocycle_name_too_long` (max 128), `race_not_found` (race_id given but absent), `patch_empty`. Cross-inject a race-existence checker (the `races` repo) like phases cross-inject templates.
- [x] 3.2 Create `internal/macrocycle/handlers.go` with swag annotations and `Register(rg *gin.RouterGroup)`: `POST /macrocycles`, `GET /macrocycles`, `GET /macrocycles/{id}`, `PATCH /macrocycles/{id}` (convert `race_id:""` → `ClearRaceID`), `DELETE /macrocycles/{id}`. Apply `numfmt.Round1` to target fields at the response boundary.
- [x] 3.3 Wire the package in `internal/httpserver/server.go`: instantiate repo/service (cross-inject the races repo for FK validation), register routes, and hand a macrocycle repo to `coachcontext.NewService(...)`.

## 4. training-phases — thread the four new fields

- [x] 4.1 `internal/trainingphases/types.go`: add `MacrocycleID *uuid.UUID`, `MacrocycleOrdinal *int`, `TargetWeeklyTSS *float64`, `TargetWeeklyHours *float64` (all `omitempty`) to `Phase`; thread onto `CreateInput` and the patch params (`MacrocycleID *string` + `ClearMacrocycleID bool` tri-state, the other three as nullable set/clear).
- [x] 4.2 `internal/trainingphases/phases_service.go`: validate `macrocycle_not_found` (when `macrocycle_id` set but absent — cross-inject a macrocycle-existence checker) and `target_invalid` (negative target); add the sentinels.
- [x] 4.3 `internal/trainingphases/phases_repo.go`: add the four columns to `selectCols` + row scan, to the Upsert/INSERT column+value lists, and to the PATCH dynamic SET builder (honor set vs clear-to-NULL for `macrocycle_id`).
- [x] 4.4 `internal/trainingphases/phases_handlers.go`: accept the four fields on POST and PATCH (tri-state `macrocycle_id:""` → clear), map the new sentinels to `400`, round target fields on responses.
- [x] 4.5 Wire the macrocycle-existence checker into `phasesSvc` in `internal/httpserver/server.go` (order the instantiation so the macrocycle repo exists first).

## 5. coach-context — surface the current macrocycle

- [x] 5.1 `internal/coachcontext/types.go`: add `Macrocycle *MacrocycleLite` to `TrainingContext`; define `MacrocycleLite` (id, name, start/end, `race_id`/`race_name`/`race_date`/`days_to_race` omitempty, `current_phase_ordinal`, `total_periods`).
- [x] 5.2 `internal/coachcontext/service.go`: add a parallel `errgroup` leg calling `macrocycleRepo.MacrocycleFor(date)` (ErrMacrocycleNotFound → leave nil); when found, compute `days_to_race` from the race anchor, set `current_phase_ordinal` from the covering phase's `macrocycle_ordinal` (when its `macrocycle_id` matches), and `total_periods` from the member-phase count. Add the macrocycle repo to the `Service` struct + `NewService` signature.

## 6. MCP tools

- [x] 6.1 Create `internal/agenttools/registry_macrocycle.go` with five specs — `create_macrocycle`, `list_macrocycles`, `get_macrocycle`, `update_macrocycle`, `delete_macrocycle` — each one HTTPCall to `/macrocycles[...]`; writes `TierWriteAuto`, `update_macrocycle` carries tri-state `race_id`. Register via `registerMCPDomain(...)`.
- [x] 6.2 Extend `CreatePhaseArgs` / `UpdatePhaseArgs` (and their `Build` payloads) in `internal/agenttools/registry_trainingphases.go` with `macrocycle_id` (tri-state on update), `macrocycle_ordinal`, `target_weekly_tss`, `target_weekly_hours`.
- [x] 6.3 Fix the stale "eight expected tools" comment in `internal/mcpserver/mcp_integration_test.go`; confirm `AnnouncedToolNames()` now includes the five macrocycle tools (the `ElementsMatch` assertion auto-tracks them).

## 7. Tests

- [x] 7.1 `internal/macrocycle/*_test.go`: POST happy-path + each validation error (`date_range_invalid`, name invalid/too-long, `race_not_found`, unanchored season); GET-by-id nested-progression ordering (ordinal NULLS LAST → start_date) + empty `phases` `[]`; list ordering; PATCH subset + `race_id` tri-state + `patch_empty`; DELETE orphans members (phases survive with `macrocycle_id NULL`) + 404s.
- [x] 7.2 `internal/trainingphases/*_test.go`: nullable-no-backfill; POST/PATCH accept the four fields; `macrocycle_not_found` + `target_invalid` rejections; tri-state `macrocycle_id` clear; linking a phase does NOT change its adherence (`goal_source` unchanged).
- [x] 7.3 `internal/coachcontext/*_test.go`: training read includes covering macrocycle with `days_to_race` + `current_phase_ordinal`/`total_periods`; unanchored season nulls race fields; no covering season → `macrocycle: null`; overlapping seasons → most-recently-updated; covering phase outside the season → `current_phase_ordinal: null`.
- [x] 7.4 `internal/agenttools/registry_macrocycle_test.go`: each macrocycle spec builds the expected HTTPCall (method/path/body); phase specs carry the four new fields.

## 8. Docs & verification

- [x] 8.1 `task swag` to regenerate `docs/` from the new/changed request/response structs.
- [x] 8.2 `task test` (macrocycle, trainingphases, coachcontext, agenttools, mcpserver packages green) and `task vet`.
- [x] 8.3 `openspec validate add-macrocycle-planning --strict` stays green.
