# Add A/B/C race priority to the race calendar

## Why

Every race in the calendar currently reads as equally important, so the coach
agent has no durable signal for the TrainingPeaks-style A/B/C triage (A = full
taper + peak, B = mini-taper, C = train through) that drives how surrounding
weeks are treated and how scheduling conflicts are reasoned about — the season's
A-race lives only implicitly in the macrocycle anchor.

## What Changes

- **`priority` on the race resource**: an optional closed enum `A|B|C`, nullable
  with no default — absence means "not triaged", honest for existing rows (the
  `training_focus` precedent). Serialized with `omitempty`, so untriaged races
  are unchanged on the wire.
- **Writable on create and PATCH**: `POST /races` accepts `priority`;
  `PATCH /races/{id}` is tri-state per the repo convention for optional enums
  (`{"priority":"A"}` sets, `{"priority":""}` clears, omitted leaves unchanged —
  the workouts `training_focus` pattern). Values outside `A|B|C` are rejected
  with `400 {"error":"race_priority_invalid"}` per `http-error-shape`.
- **Returned everywhere the race row is read**: single get, list, and the create/
  PATCH echoes all serialize the full `Race` struct, so they gain the field
  automatically. The fueling-plan response is unaffected — it carries only
  `race_id` + `race_name`, not the race metadata.
- **List filter**: `GET /races?priority=A` — the list endpoint's first query
  param; cheap for a single-user calendar and directly agent-useful ("what are
  my A-races"). Invalid values get the same `400 race_priority_invalid`.
- **Advisory only vs the macrocycle A-race anchor**: no hard coupling. A
  macrocycle may anchor a race marked `B` or `C` (or untriaged) without error;
  the LLM coach reasons over both signals. See design for rationale.
- **MCP**: `create_race` / `update_race` arg structs in the shared `agenttools`
  registry gain `priority`; the announced-schema golden is regenerated via the
  `-tags=goldengen` capture (registry-derived post `unify-mcp-tool-registry` —
  there is no hand-maintained expected-tools list to bump).
- **One migration**: `ALTER TABLE races ADD COLUMN priority TEXT CHECK (priority
  IN ('A','B','C'))` — nullable, no backfill.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `race-fueling-plan`: the persistent-race requirement's resource shape gains
  `priority?`; two ADDED requirements cover the priority semantics (validation,
  tri-state PATCH, advisory stance) and the list filter.

No `mcp-server` delta: its "Race tools mirror the race REST endpoints"
requirement does not enumerate per-tool arg fields, and the tool set is
unchanged — the new arg flows through the existing 1:1 mirroring contract.
No `macrocycle` delta: its requirements are untouched (no new validation).

## Impact

- **Specs**: `race-fueling-plan` only — one MODIFIED requirement (race shape),
  two ADDED requirements (priority semantics, list filter).
- **Code**: `internal/races/` — `types.go` (typed `Priority` enum + field),
  `repo.go` (column in insert/select/update; set/clear in the `CASE WHEN`
  update), `service.go` (validation + `ErrPriorityInvalid` sentinel +
  `ClearPriority` on `UpdateInput`), `handlers.go` (request structs, empty-string
  sentinel conversion, list query param, swag annotations + error-code lists).
- **Migration**: one pair in `internal/store/migrations/`. Head is `054` at
  writing, but several in-flight siblings (`add-race-pacing-plan`,
  `persist-activity-streams`, …) also claim slots — verify the highest existing
  number before creating.
- **MCP**: `internal/agenttools/registry_races.go` (`CreateRaceArgs`,
  `UpdateRaceArgs`); regenerate `internal/mcpserver/testdata/announced_schemas.json`
  via the goldengen capture test; MCP integration test (`-tags=integration`)
  stays green (registry-derived tool list is unchanged).
- **Docs**: `task swag` after handler changes.
- **Coordination**: the in-flight `add-race-pacing-plan` change touches the race
  surface but adds a sibling resource — its delta specs cover `race-pacing-plan`,
  `athlete-config`, and `mcp-server`, not `race-fueling-plan`, so there is no
  requirement-block collision with this change.

### Out of scope (explicit non-goals)

- Taper/peak automation or any plan-generation behavior keyed off priority.
- Hard consistency enforcement between `macrocycles.race_id` and `priority`.
- Priority on the public race feed or coach-dashboard read models.
