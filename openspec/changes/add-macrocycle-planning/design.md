## Context

The repo already models the two lower levels of the periodization hierarchy:

- **`training-phases`** (`internal/trainingphases/`) — the mesocycle unit: a named date
  range tagged with a type (`base`/`build`/`peak`/`recovery`/`race_week`/`off_season`/
  `other`), optionally pointing at a goal template that drives nutrition adherence between
  per-date overrides and the singleton default. A phase already carries optional
  `methodology` Markdown and is surfaced in `/context/training` via
  `phasesRepo.PhaseFor(date)`.
- **`training-plan`** (`internal/trainingplan/`) — the microcycle structure: plan → weeks →
  slots, where a week carries an optional `phase_id`, materializing into dated workouts. It
  already FK-references `races(id) ON DELETE SET NULL` — the precedent this design copies.

What's absent is the **macrocycle**: the season-long container that orders phases into one
goal-anchored yearly progression. Today phases are independent, overlap-tolerant date
ranges with no notion of belonging to a season, no progression sequence, and no per-period
load target. This change adds that top level as a thin new capability and four additive
fields on the phase, following the established one-package-per-capability shape, the
tri-state-clear PATCH convention, and the unified `agenttools` MCP registry.

## Goals / Non-Goals

**Goals:**
- A season container (`macrocycle`) that **groups existing phases** — phases stay the one
  source of truth for date ranges; the macrocycle never duplicates them.
- Optional **race anchor** (the A-race the season peaks for) and optional **methodology**
  prose at the season level.
- Per-period **progression targets** (`target_weekly_tss`, `target_weekly_hours`) so the
  yearly load ramp is explicit and visible in one read.
- A **nested progression read** (`GET /macrocycles/{id}` returns the season with its ordered
  member phases) — the "plan my year" view.
- **`/context/training` awareness** — the coach learns the current season, its race anchor
  with `days_to_race`, and where today sits in the progression.
- REST + MCP parity (five new tools); migrations append-only; `task swag` regenerated.

**Non-Goals:**
- **No adherence effect.** A macrocycle does NOT enter the goals resolver chain. Phases
  (via their `default_template_id`) remain the only periodization input to adherence. A
  macrocycle is planning/visualization + coach-context only.
- **No plan-materialize effect.** Macrocycles do not drive `training-plan` expansion.
- **No enforcement of progression targets.** `target_weekly_tss`/`target_weekly_hours` are
  planning hints, never compared against actuals or validated against load. (A future
  "macro adherence" change could read them back; out of scope here.)
- **No multi-race ladder.** A macrocycle anchors to at most one race (the A-race). B/C-race
  priorities are a deferred extension.
- **No ordinal uniqueness / no overlap ban.** Consistent with phases' permissive model,
  ordinals are ordering hints (ties allowed) and seasons may overlap; the covering-season
  pick is deterministic via most-recently-updated (mirrors `PhaseFor`).
- **No mobile-companion surface.** Deferred to a separate Flutter change.

## Decisions

### 1. New `macrocycle` capability; membership lives on the phase (not a join table)

A new `internal/macrocycle/` package owns the `macrocycles` table and CRUD. Membership is
expressed by **four nullable columns added to `training_phases`** —
`macrocycle_id`, `macrocycle_ordinal`, `target_weekly_tss`, `target_weekly_hours` — set on
the existing phase write path. `GET /macrocycles/{id}` aggregates `WHERE macrocycle_id = $1
ORDER BY macrocycle_ordinal NULLS LAST, start_date`.

**Why over a `macrocycle_phases` join table:** a phase belongs to at most one season, so a
single FK column is the natural cardinality — a join table would model a many-to-many that
doesn't exist and force a second write path. Keeping the date range solely on the phase
preserves the "phases are the source of truth for dates" invariant; the macrocycle stores
only the season envelope (`start_date`/`end_date`) for the covering-date query and as a
sanity boundary, never the per-period dates.

**Why the membership write is on the phase, not the macrocycle:** the phase write path
already has the `default_template_id` tri-state-clear plumbing to copy verbatim, and it
keeps "edit a period" as one call against one resource. The macrocycle read is the
aggregator.

### 2. `macrocycle_id` is `ON DELETE SET NULL`; `race_id` likewise

Deleting a season SHALL **orphan, not delete, its phases** — they revert to standalone
phases and keep driving adherence exactly as before. This mirrors `training_plans.race_id`
and avoids a destructive cascade that would silently drop a quarter of training history.
`races.id` deletion likewise nulls the anchor rather than deleting the season.

**Alternative considered:** `ON DELETE RESTRICT` on `macrocycle_id` (can't delete a season
with members). Rejected — too rigid for a single-user planning tool; the user should be
able to drop a season scaffold without first unlinking every phase.

### 3. Progression targets on the phase, nullable, unenforced

`target_weekly_tss NUMERIC NULL` and `target_weekly_hours NUMERIC NULL` live on the phase
because the period *is* the phase (the chosen "group existing phases" model). They are the
deliberate load staircase (base low → build rising → peak high → taper down). Stored at
full precision, rounded at the response boundary per the `numfmt.Round1` convention,
validated only as "non-negative when present" (`target_invalid`), never compared to actual
TSS/duration. This matches the `training-focus` precedent: declared intent, not measured.

### 4. Race anchor + methodology at the season level

`macrocycles.race_id` (nullable FK → `races`, `ON DELETE SET NULL`) is the A-race; when set,
`/context/training` derives `days_to_race = race_date − anchor_date`. `macrocycles.methodology`
is nullable Markdown stored verbatim (no server rendering), exactly like the phase
`methodology` field — the season-level "why this whole arc" narrative, distinct from
operational `notes`.

### 5. `/context/training` gains a `macrocycle` block computed from existing reads

`coachcontext.BuildTraining` adds one parallel `errgroup` leg: find the macrocycle covering
the anchor date (`MacrocycleFor(date)` — most-recently-updated covering, mirroring
`PhaseFor`); when found, populate a `MacrocycleLite` with the season identity, the race
anchor (`race_id`, `race_name`, `race_date`, derived `days_to_race`), and the current
period's position (`current_phase_ordinal`, `total_periods`) computed from the already-fetched
covering `Phase`'s `macrocycle_ordinal` and a count of member phases. Absent season →
`macrocycle: null`, mirroring how an absent phase serializes. This needs a `macrocycle` repo
cross-injected into `coachcontext` in `httpserver`, alongside the existing `phasesRepo`.

### 6. REST surface and MCP parity

REST: `POST /macrocycles`, `GET /macrocycles` (list all, ordered `start_date DESC` — seasons
are few; no range filter), `GET /macrocycles/{id}` (season + nested ordered member phases),
`PATCH /macrocycles/{id}` (partial, tri-state on `race_id`), `DELETE /macrocycles/{id}`
(204; members' `macrocycle_id` set null). Behind the standard auth + idempotency middleware;
no PUT, so no idempotency-on-PUT concern. MCP: five new `agenttools` specs
(`create/list/get/update/delete_macrocycle`) in `registry_macrocycle.go`, write tiers
`TierWriteAuto` like phases. The existing `create_phase` / `update_phase` specs gain the
four new phase fields (`macrocycle_id` tri-state empty-string-clears, `macrocycle_ordinal`,
`target_weekly_tss`, `target_weekly_hours`). Because `AnnouncedToolNames()` derives from the
registry, the MCP integration test's expected surface updates automatically.

## Risks / Trade-offs

- **[Two seasons overlap → which is "current"?]** → Deterministic most-recently-updated pick
  in `MacrocycleFor`, the same rule `PhaseFor` already uses; documented in the spec. A
  single-user planner rarely runs overlapping seasons, but the tiebreak is defined rather
  than left to row order.
- **[Phase `macrocycle_ordinal` can disagree with `start_date` order]** → Accepted: the
  nested read orders by `macrocycle_ordinal NULLS LAST, start_date`, so a missing/duplicated
  ordinal degrades gracefully to date order rather than erroring. Ordinal is a hint.
- **[Progression targets could drift from reality]** → Accepted by design — they are declared
  plan, not measured. Coexisting with `fitness`/`recent_load` actuals in the same context is
  a feature (plan vs actual), not a contradiction. No enforcement keeps the surface small.
- **[Adding columns to `training_phases` touches a hot table]** → All four are nullable with
  no back-fill and no default; the `ALTER`s are metadata-only, and existing phase
  read/write paths are unaffected until they opt into the new fields. Mirrors the
  `training-focus` migration shape.
- **[Scope creep toward macro-adherence]** → Explicitly deferred (Non-Goals). This change
  ships the container + the read; reading targets back against actuals is a clean follow-up.

## Migration Plan

1. `task migrate:new NAME=add_macrocycles` → scaffolds `049_*.{up,down}.sql`. Verify `048`
   is still the head before claiming `049` (out-of-band slots have happened).
2. `up.sql`: `CREATE TABLE macrocycles (...)` with `race_id` FK `ON DELETE SET NULL` and a
   `start_date <= end_date` CHECK; then `ALTER TABLE training_phases ADD COLUMN
   macrocycle_id UUID NULL REFERENCES macrocycles(id) ON DELETE SET NULL`, `ADD COLUMN
   macrocycle_ordinal INT NULL`, `ADD COLUMN target_weekly_tss NUMERIC NULL CHECK (… >= 0)`,
   `ADD COLUMN target_weekly_hours NUMERIC NULL CHECK (… >= 0)`; index
   `training_phases (macrocycle_id)` for the aggregate read.
3. `down.sql`: drop the four columns, then `DROP TABLE macrocycles`.
4. Build `internal/macrocycle/`, thread the four fields through `internal/trainingphases/`,
   extend `internal/coachcontext/`, add the MCP specs, wire `httpserver`, add tests,
   `task swag`, `task test`, `task vet`.
5. Rollback = run the down migration; dropping the columns and table is non-destructive to
   every other field (phase rows survive, having only lost their season link).

## Open Questions

- None blocking. Macro-level adherence (reading `target_weekly_tss`/`hours` back against
  actual rolling load) and a B/C-race ladder are deliberately deferred to follow-up changes.
