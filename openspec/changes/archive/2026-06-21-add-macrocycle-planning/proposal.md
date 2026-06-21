## Why

The system can describe a single block of training — a `training-phase` is a named,
typed date range (base / build / peak / …) that may drive nutrition adherence — but it
has no concept of the *season* those blocks add up to. There is nowhere to say "this
base → build → peak → race-week → recovery sequence is one annual progression toward my
September A-race," nowhere to record the deliberate load ramp across the year, and
nothing that tells the coach where today sits in that arc. Macrocycle planning is the
missing top level: the yearly container that turns a pile of independent phases into a
single, goal-anchored progression.

## What Changes

- **New `macrocycle` capability** — a season container: a named, dated record
  (`start_date` … `end_date`) optionally anchored to a goal **race** (the A-race the
  season builds toward) and carrying optional cited **methodology** prose. Full CRUD over
  REST (`POST/GET list/GET by-id/PATCH/DELETE /macrocycles`), mirrored 1:1 by five new MCP
  tools. `GET /macrocycles/{id}` returns the season **with its ordered member phases
  nested** — the yearly-progression view (the base→build→peak staircase with each
  period's load targets).
- **Phases gain macrocycle membership + progression targets** (modifies `training-phases`):
  four new nullable columns on `training_phases` — `macrocycle_id` (FK → `macrocycles`,
  the season this period belongs to), `macrocycle_ordinal` (its position in the
  progression), and the per-period planning targets `target_weekly_tss` and
  `target_weekly_hours` (the deliberate load ramp). Linkage is set on the existing phase
  write path using the established tri-state empty-string-clears convention; the macrocycle
  read aggregates phases by `macrocycle_id`. Phases remain the single source of truth for
  date ranges — a macrocycle never duplicates them.
- **`/context/training` surfaces the current macrocycle** (modifies `coach-context`) — a
  new `macrocycle` block carrying the season covering today, its race anchor with derived
  `days_to_race`, and the current period's position in the progression ("phase 3 of 6").
  This is the only read-time behavior; macrocycles do **not** enter the goals resolver or
  plan materialization.

## Capabilities

### New Capabilities
- `macrocycle`: a season-level periodization container — a named, dated, optionally
  race-anchored record that orders existing training-phases into a yearly progression and
  carries season-level methodology. Owns macrocycle CRUD, the nested progression read, the
  five MCP tools, and the `/context/training` surfacing.

### Modified Capabilities
- `training-phases`: a phase gains four optional fields — `macrocycle_id` (season
  membership, tri-state settable on create/update, `ON DELETE SET NULL` so deleting a
  season orphans rather than deletes its phases), `macrocycle_ordinal` (position in the
  progression), and the planning targets `target_weekly_tss` / `target_weekly_hours`.
  Accepted on create/update, returned on read. No change to the adherence resolver.
- `coach-context`: `GET /context/training` gains a `macrocycle` block (current season +
  race anchor + days-to-race + current-period position). Pure read; no new behavior
  elsewhere.

## Impact

- **Schema**: new migration `049_add_macrocycles.{up,down}.sql` — creates the `macrocycles`
  table (`id, name, start_date, end_date, race_id NULL → races ON DELETE SET NULL,
  methodology TEXT NULL, notes TEXT NULL, timestamps`) and `ALTER TABLE training_phases`
  adding `macrocycle_id UUID NULL → macrocycles ON DELETE SET NULL`,
  `macrocycle_ordinal INT NULL`, `target_weekly_tss NUMERIC NULL`,
  `target_weekly_hours NUMERIC NULL`. Verify `048` is still the head before claiming `049`.
- **Code**: new `internal/macrocycle/` package (`types/repo/service/handlers` + tests)
  following the one-package-per-capability shape; cross-injected in
  `internal/httpserver/server.go` (race-FK validation against `races`, route registration,
  and a macrocycle repo handed to `coachcontext`). `internal/trainingphases/`
  (`types/service/repo/handlers`) threads the four new phase fields through
  POST/PATCH/GET. `internal/coachcontext/` (`types/service`) adds the `macrocycle` block to
  `TrainingContext` / `BuildTraining`.
- **MCP**: five new `agenttools` specs in a new `registry_macrocycle.go`
  (`create_macrocycle`, `list_macrocycles`, `get_macrocycle`, `update_macrocycle`,
  `delete_macrocycle`); the existing `create_phase` / `update_phase` specs gain the four
  new phase fields. `AnnouncedToolNames()` derives from the registry, so the MCP
  integration test's expected surface updates automatically — no hand-maintained list to
  bump (the stale "eight expected tools" comment is corrected).
- **Docs**: `task swag` regenerates `docs/` from the new/changed request/response structs.
- **Mobile companion** (`apps/companion`): out of scope — a season-overview surface is a
  separate Flutter change if/when wanted.
