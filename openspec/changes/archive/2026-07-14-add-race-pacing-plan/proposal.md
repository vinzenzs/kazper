# Add a per-leg race pacing plan (compute-on-read + persisted overrides)

## Why

The race calendar computes per-leg *fuelling* deterministically, but pacing — the
Best Bike Split / TrainingPeaks race-plan niche of "what watts on the bike, what
pace off the bike" — is still re-derived by the agent from scratch every
conversation, exactly the evidence-based arithmetic an LLM should anchor on a
deterministic primitive instead of hallucinating.

## What Changes

- **New compute-on-read pacing plan** over the existing `races` + `race_legs`
  tables: `GET /races/{id}/pacing-plan` returns, per leg, an intensity target
  derived from the athlete-config thresholds — bike legs get a power band as a
  duration-banded % of `ftp_watts` (e.g. 68–78% FTP for a ≥3 h full-distance
  bike), run legs a pace band as a duration-banded multiple of
  `threshold_pace_sec_per_km` (with a multisport "this comes after the bike"
  rationale), swim legs a pace band per 100 m relative to
  `threshold_swim_pace_sec_per_100m` (CSS). Banding by leg duration mirrors how
  the fuelling plan bands carbs by duration.
- **Per-leg IF + estimated TSS**, and a race-level `estimated_tss_total` with an
  honesty flag (`tss_complete`) — the taper-planning input for the sibling
  `add-performance-management` PMC change (synergy only, no structural
  dependency in either direction).
- **Missing thresholds degrade per leg, not per request**: legs whose threshold
  is unset return null targets plus a machine-readable `missing_thresholds`
  list (and a race-level union) in a `200` response; structured errors are
  reserved for request-shaped failures (`404 race_not_found`, override
  validation `400`s) per `http-error-shape`.
- **Persisted per-leg manual overrides** (the fuelling plan has none — this is
  the first stored artifact on the race subtree): one new table keyed by
  `(race_id, ordinal)` so overrides survive the wholesale leg-replace on
  `PATCH /races/{id}`. `PUT /races/{id}/pacing-plan/overrides/{ordinal}`
  full-replaces (rejects `Idempotency-Key` per the PUT rule),
  `DELETE …/overrides/{ordinal}` reverts to computed. Overridden legs report
  `source: "override"` in the plan.
- **Unit isolation**: power only in `_w` fields, run pace only in
  `_sec_per_km`, swim pace only in `_sec_per_100m` — never merged into a shared
  target struct. `numfmt` rounding at the response boundary (watts integer,
  paces/TSS 1dp, IF 2dp matching the workouts `intensity_factor` precedent).
- **MCP tools mirroring 1:1** via the shared `agenttools` registry:
  `plan_race_pacing` (read), `set_race_leg_pacing_override`,
  `clear_race_leg_pacing_override` (writes).

## Capabilities

### New Capabilities

- `race-pacing-plan`: deterministic compute-on-read per-leg pacing targets
  (bike %FTP power band, run threshold-pace band, swim CSS band, per-leg
  IF/estimated TSS, race TSS total) over the race calendar owned by
  `race-fueling-plan`, plus persisted per-leg manual overrides with
  compute-on-read fallback. Deliberate sibling of `race-fueling-plan`, kept a
  separate capability for the same reason fuelling is separate from hydration:
  one concern per capability, unit-isolated response shapes.

### Modified Capabilities

- `athlete-config`: the consumption-gate requirement widens — `ftp_watts`,
  `threshold_pace_sec_per_km`, and `threshold_swim_pace_sec_per_100m` are now
  also consumed by the race-pacing-plan computation. No new config fields.
- `mcp-server`: the tool surface gains the three pacing tools (the spec
  enumerates tools per REST surface).

## Impact

- **Specs**: new `race-pacing-plan` spec; MODIFIED consumption-gate requirement
  on `athlete-config`; ADDED tool requirement on `mcp-server`. The
  `race-fueling-plan` spec is untouched — it keeps owning the race/leg tables
  and CRUD.
- **New package** `internal/racepacing/` (types / pure `compute.go` /
  overrides `repo.go` / `service.go` / `handlers.go` + tests), depending on the
  `races` repo (race + legs read) and the `athleteconfig` repo (thresholds) —
  the multi-repo-dependency pattern established by `internal/workoutfueling/`.
- **One migration pair**: `race_leg_pacing_overrides` keyed
  `(race_id, ordinal)` with `ON DELETE CASCADE` to `races`. Next free slot
  after `054_sync_run_summary_partial` — verify the head before creating.
- **Routes** registered in `internal/httpserver/server.go`:
  `GET /races/{id}/pacing-plan`, `PUT`/`DELETE`
  `/races/{id}/pacing-plan/overrides/{ordinal}`.
- **MCP**: new `internal/agenttools/registry_racepacing.go`; the announced
  tool list derives from the registry automatically; regenerate the MCP schema
  golden (`-tags=goldengen`) and confirm the integration test's
  registry-derived surface.
- **Docs**: `task swag` after handlers; README MCP table + RUN_LOCAL walkthrough.
- **Coordination**: the in-flight `add-per-sport-tss` change modifies the same
  athlete-config consumption-gate requirement — whichever archives second
  reconciles the merged block (both only *widen* the consumer list; no
  conflict in substance).

### Out of scope (explicit non-goals)

- **Course/GPS modelling.** No gradient, wind, CdA, or split-by-segment math —
  explicitly *not* Best Bike Split. The band is generic duration-based
  guidance; course-specific adjustment stays with the agent.
- **HR-based pacing targets.** `threshold_hr`/zones exist but HR drifts and
  lags; v1 keys on power/pace only. Revisit if a leg has no pace/power model.
- **Environment adjustments** (heat, altitude) — agent-side, like weather in
  the fuelling plan.
- **Writing pacing into planned workouts or race results capture.**
