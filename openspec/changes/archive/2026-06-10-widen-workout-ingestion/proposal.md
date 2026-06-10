# widen-workout-ingestion

## Why

Garmin Connect measures most of what the fueling tools currently guess at — per-activity distance, average power, ambient temperature, estimated sweat loss — and records triathlon/brick sessions as linked multi-leg activities. The `/workouts` ingestion shape accepts none of it, so the importer must drop these fields at the door: sweat-loss data that would personalize fluid recommendations is lost, heat context for fueling is lost, and a brick session (the signature triathlon workout, scheduled weekly in the current 18-week plan) lands as two unrelated rows with no way to ask "carbs/hr on the bike leg vs off the bike." With race day 2026-07-24 ~6 weeks out and the Peak phase (the fueling-rehearsal window) running now, every un-synced session is rehearsal data permanently degraded.

This change widens the `workouts` row with five nullable, source-agnostic columns so any writer (Garmin today, Apple Health/Strava tomorrow, manual entry always) can carry them: `distance_m`, `avg_power_w`, `temperature_c`, `sweat_loss_ml`, and `session_group` (a free-text key linking the legs of a brick/multisport session). It deliberately does NOT change any computation — `recommend_workout_fuel` personalization from sweat loss/temperature is a follow-up once the data exists.

## What Changes

- **New migration** (`019_widen_workout_ingestion`, verify head before committing) adding five nullable columns to `workouts`:
  - `distance_m NUMERIC(10,1) NULL CHECK (distance_m IS NULL OR distance_m > 0)`
  - `avg_power_w INTEGER NULL CHECK (avg_power_w IS NULL OR avg_power_w > 0)`
  - `temperature_c NUMERIC(4,1) NULL CHECK (temperature_c IS NULL OR (temperature_c BETWEEN -40 AND 60))`
  - `sweat_loss_ml NUMERIC(10,1) NULL CHECK (sweat_loss_ml IS NULL OR sweat_loss_ml > 0)`
  - `session_group TEXT NULL` + a partial index on `(session_group) WHERE session_group IS NOT NULL`
- **`internal/workouts/types.go`** — `Workout` gains the five pointer fields with `omitempty` JSON tags (matches the KcalBurned/AvgHR/TSS nullable pattern).
- **`internal/workouts/repo.go`** — Upsert/Patch/selectCols carry the new columns; List gains an optional `session_group` filter.
- **`internal/workouts/handlers.go`** — `POST /workouts`, `POST /workouts/bulk`, and `PATCH /workouts/:id` accept the new fields with range validation (`distance_m_invalid`, `avg_power_w_invalid`, `temperature_c_invalid`, `sweat_loss_ml_invalid`, `session_group_invalid`); `GET /workouts` accepts `?session_group=` to fetch the legs of one session together.
- **`workout_fueling` aggregation** — the workout summary block of `GET /workouts/{id}/fueling` echoes `sweat_loss_ml` and `temperature_c` so the agent reads sweat/heat context alongside the fluid+sodium totals it is evaluating. (`distance_m`/`avg_power_w`/`session_group` are NOT echoed there — not fueling-evaluation inputs.)
- **MCP tools** — `log_workout`, `patch_workout` schemas gain the five optional fields with descriptions (units explicit: metres, watts, °C, ml); `list_workouts` gains the `session_group` filter; `get_workout`/`list_workouts` responses carry the fields automatically (verbatim body forwarding). No new tools.
- **swag** regenerates `docs/` for the changed request/response shapes.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `workouts`: the table/row shape requirement gains five nullable ingestion fields with validation rules; POST/bulk/PATCH accept them; GET echoes them; the list endpoint gains a `session_group` filter requirement; the `GET /workouts/{id}/fueling` top-level echo requirement (which lives in this spec) gains `sweat_loss_ml` and `temperature_c`.
- `mcp-server`: `log_workout`, `patch_workout`, `list_workouts` tool schemas + descriptions updated. No new tools (expected-tools list in the integration test unchanged).

## Impact

- **Schema**: one append-only migration pair; no back-fill (existing rows get NULL — "not measured" is meaningful, mirrors the rpe/gi precedent).
- **Code**: `internal/workouts/` (types, repo, service validation, handlers), `internal/workoutfueling/` (two echo fields on the workout block), `internal/mcpserver/tools_workouts.go`.
- **Tests**: repo upsert/patch round-trips per field, DB CHECK enforcement, handler validation error codes, bulk path carries the fields, `session_group` list filter, fueling-block echo, MCP forwarding (explicit values forwarded verbatim / absent values omitted).
- **Docs**: README workouts subsection (one paragraph + the brick-session `session_group` example), `task swag` regen.
- **Out-of-repo coordination** (not implemented by this change, noted for the companion work): `garmin.py` gains a push/sync path mapping Garmin activities onto the widened shape — distance, avg power, temperature, estimated sweat loss, and `session_group` from multisport parent activity IDs.

### Out of scope (explicit non-goals)

- **Using the new data in computations.** `recommend_workout_fuel` keeps its current fluid/sodium tables; personalizing from `sweat_loss_ml`/`temperature_c` is a deliberate follow-up change once a few weeks of real data exist.
- **A derived sweat-rate (ml/hr) endpoint** — composition over this data plus the weight log; future change (priorities 6C).
- **Planned/scheduled workouts** (a `status` field or a planned-workouts table) — separate change; different lifecycle semantics.
- **A race entity / per-leg race-day fueling plan** — separate change.
- **Normalized power / IF / cadence / elevation** — TSS (already stored) is the intensity signal; the importer derives it from IF where available. Avg power is stored for work (kJ) arithmetic; the rest is performance analysis, which the workouts spec explicitly excludes.
- **A `brick` sport enum value** — `session_group` linking real per-leg rows is strictly more expressive than a merged pseudo-sport.
