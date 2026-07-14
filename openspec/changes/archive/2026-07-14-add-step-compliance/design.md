# Design — per-step workout compliance scoring

## Context

Everything the read needs already exists:

- **Templates** (`internal/workouttemplates`): ordered `Step` nodes — single
  steps (`intent`, exactly one `Duration`, a `Target`, optional bike
  `SecondaryTarget`) and one-level `repeat` groups (`count >= 2`, single steps
  only). Target kinds: `none`, `hr_zone`, `power_zone`, `pace` (sec/km),
  `swim_pace` (sec/100m), `hr_bpm`, `power_w`, `cadence`, `rpe`.
- **Effective program** (`trainingplan.Service.EffectiveProgram(ctx, workoutID)`):
  template steps + slot `target_overrides`/`duration_overrides` + the
  athlete-config zone→absolute pass (`hr_zone`→`hr_bpm` everywhere,
  `power_zone`→`power_w` bike-only, `Origin` provenance, best-effort
  passthrough when config is absent). The training-plan spec mandates this as
  *the single representation downstream consumers build from* — compliance must
  compare against it, not raw template steps.
- **Splits** (`workouts.Split`, single-get only): per-lap `SplitIndex`,
  `DistanceM`, `DurationS`, `AvgHR`, `AvgPowerW`, `AvgSpeedMPS`,
  `ElevationGainM`. No per-lap cadence, no per-lap RPE.
- **Linkage**: `Workout.TemplateID` (nullable) on planned-then-completed rows;
  `MultisportTemplateID` for bricks; `Status` ∈ planned/completed.
- **Precedents**: `plan-adherence-analytics` (pull rows, aggregate in a pure
  function, compute-on-read, MCP mirror + goldengen), `workoutfueling`
  (separate aggregation-only package registering under `/workouts/:id/...` to
  avoid import cycles), `numfmt.Round1` at the response boundary,
  `http-error-shape` (every error is `{"error":"<code>", ...}`).

## Goals / Non-Goals

**Goals:**

- One read that answers "was the session executed as written?" —
  TrainingPeaks-style per-step compliance: target vs actual, duration planned
  vs actual, in-band/under/over, an overall 0–100 score.
- Reuse the existing target-resolution arc wholesale; no second resolver.
- Honest degradation: when laps don't align to steps, say so explicitly instead
  of guessing.
- Coach-groundable: mirrored 1:1 as an MCP tool.

**Non-Goals:**

- Multisport workouts (per-leg splits vs per-segment programs — own change).
- Heuristic/tolerant lap matching (time-window alignment, merging manual laps).
  V1 serves the structured-execution flow; everything else is `unavailable`.
- Persisting scores or any write — pure read, recomputed every call.
- Dashboard surfacing — the SPA workout detail route already shows splits; a
  compliance panel is a follow-up SPA change once the API shape settles.
- Strength/yoga/mobility scoring (they carry `Sets`, not `Splits`; their steps
  rarely carry scorable targets) — they fall out naturally as `splits_missing`.

## Decisions

### D1: New leaf package `internal/workoutcompliance`, route on the workouts path

`GET /workouts/{id}/compliance` is workout-anchored, so the route lives under
`/workouts/:id/` — but the logic needs `workouts` (row + splits),
`workouttemplates` (step model), and `trainingplan` (effective program), and
`trainingplan` already imports `workouts`, so the aggregator cannot live in
either without a cycle. Exactly the `workoutfueling` situation, same answer: a
leaf package with service + handlers (no repo of its own), registering
`rg.GET("/workouts/:id/compliance", ...)`, wired in `httpserver.Run()`. It
consumes two narrow injected interfaces (mirroring `garmincontrol`'s
`planService`):

```go
type workoutsRepo interface {
    GetByID(ctx context.Context, id uuid.UUID) (*workouts.Workout, error) // loads Splits
}
type programProvider interface {
    EffectiveProgram(ctx context.Context, workoutID uuid.UUID) (*trainingplan.Program, error)
}
```

### D2: Target resolution is reused, never reimplemented

The compared-against program is `EffectiveProgram`'s output: slot overrides
applied, `hr_zone`/`power_zone` rewritten to `hr_bpm`/`power_w` ranges with
`Origin` set, everything else passed through. Consequences accepted as-is:

- A zone target that could not resolve (no athlete config, unset boundary,
  power zone on a non-bike sport) arrives still zone-shaped → that step's
  target is **unscorable** (`reason: "zone_unresolved"`), not an error.
- Scores reflect the athlete config *at read time* — see Risks.

### D3: Matching — positional after repeat expansion, strict count, honest failure

Expand the effective program's step tree into a flat executed-step list:
each `repeat{count, steps}` contributes `count` consecutive copies of its inner
steps, in order; single steps contribute themselves. Each expanded step carries
provenance: flat `step_index` (0-based), and for repeat children
`{group_index, iteration, of}` so "interval 3 of 5" is nameable.

Then match `splits[i] ↔ expanded[i]` **iff
`len(splits) == len(expanded)`** — Garmin structured execution emits exactly
one lap per executed step (including lap_button steps, pressed by the athlete),
which is this repo's main flow since templates compile to watch workouts.

On count mismatch there is no trustworthy alignment, so the response is
**HTTP 200 with `status: "unavailable"`, `reason: "lap_count_mismatch"`,
`planned_steps: N`, `executed_laps: M`** and no per-step array. A 200 (not an
error) because the read itself succeeded and the counts are the useful answer;
errors are reserved for missing prerequisites (D6). Tolerant matching (e.g.
absorbing a trailing extra lap) is deliberately deferred — a wrong alignment
that produces confident-looking per-interval numbers is worse than "can't
score".

### D4: Per-step scoring — metric selection, band classification, tolerance

**Metric per resolved target kind** (actual taken from the matched split):

| target kind            | actual                                    | unit      |
|------------------------|-------------------------------------------|-----------|
| `power_w` (incl. resolved `power_zone`) | `AvgPowerW`              | W         |
| `hr_bpm` (incl. resolved `hr_zone`)     | `AvgHR`                  | bpm       |
| `pace`                 | `1000 / AvgSpeedMPS`                      | sec/km    |
| `swim_pace`            | `100 / AvgSpeedMPS`                       | sec/100m  |
| `cadence`, `rpe`, `none`, unresolved zones | — unscorable (no per-lap actual / nothing to compare) | |

A scorable kind with a nil actual on the split (e.g. no power meter) is also
unscorable, with `reason: "actual_missing"`.

**Classification** is on the numeric band: actual within `[low, high]` →
`in_band`; below `low` → `under`; above `high` → `over`. For pace kinds the
numbers invert semantically (more sec/km = slower), which is fine because the
response always carries `metric`, `low`, `high`, `actual`, and a signed
`delta` (distance from the violated bound, 0 in band) — "20W under target" is
`delta: -20` on a `power_w` step. `deviation_pct = |delta| / nearest_bound`
accompanies it so severity is readable without knowing the unit (>5% out of
band is the documented "major deviation" line; exposed as the continuous
`deviation_pct`, not a fourth enum value).

**Target score** (0–100): `100` in band, else
`100 × max(0, 1 − deviation_pct / 0.25)` — linear falloff hitting zero at 25%
outside the band. Constants (`5%` major line, `25%` zero line) are package
constants, documented in the response? No — documented in swag/spec only, to
keep the payload lean.

**Duration score**: for `time` steps compare planned `Seconds` vs split
`DurationS`; for `distance` steps compare planned `Meters` vs split
`DistanceM`. `ratio = actual / planned`; in-band within ±10%, else the same
linear falloff on `|ratio − 1|` (zero at ±25%). `lap_button`/`open` steps have
no planned duration — duration is unscored (the athlete decides when it ends).

**Step score**: `0.7 × target + 0.3 × duration` when both scored; the one
present otherwise; step unscored (excluded from the aggregate) when neither.

**Secondary target** (bike): scored and reported as its own `secondary` block
(same shape as the primary), **informational only** — not folded into the step
score in v1, keeping the formula explainable ("hold power" is the contract;
the HR/cadence gate is context).

### D5: Overall score — planned-duration-weighted mean

`score = Σ(step_score × weight) / Σ(weight)` over scored steps, where `weight`
is the planned duration in seconds (time steps), an estimated duration for
distance steps (planned meters at the target pace midpoint when pace-targeted,
else the actual lap duration), and the actual lap duration for
`lap_button`/`open`. Weighting by time keeps a 3-second miss on a 10-minute
tempo block from being drowned out by nailing five 30-second recoveries.
Also reported: `steps_scored`, `steps_in_band`. `score` is `null` when no step
is scorable (e.g. an all-RPE template) — the response is still
`status: "scored"` with per-step rows, mirroring adherence's null-rate
semantics. All floats pass `numfmt.Round1` at the boundary.

### D6: Prerequisite failures are structured errors, per http-error-shape

Sentinel errors in the service, mapped 1:1 in the handler:

| condition                                        | status | code                       |
|--------------------------------------------------|--------|----------------------------|
| id not a UUID                                    | 400    | `workout_id_invalid`       |
| workout missing                                  | 404    | `not_found`                |
| `status != completed`                            | 409    | `workout_not_completed`    |
| `sport == multisport` (or `MultisportTemplateID` set) | 409 | `multisport_unsupported`  |
| `TemplateID` nil                                 | 409    | `no_template_link`         |
| completed + linked but zero splits               | 409    | `splits_missing`           |

409 (not 422/400) because these are conflicts with the resource's current
state, not malformed requests — the same request succeeds once the workout is
completed/linked/backfilled. The multisport check precedes the template check
so a brick doesn't misleadingly report `no_template_link`.

### D7: Compute-on-read, nothing persisted

Same rationale as adherence and the effective program (whose spec says
resolution "SHALL be computed on read and SHALL NOT be persisted"): no
migration, no staleness, a config or template fix retroactively corrects the
score, and the input is one workout + one template — trivially cheap. A
snapshot-on-completion model was rejected: it would need a migration, a
backfill story, and an invalidation story, for no read-latency win at this
data size.

### D8: MCP tool `workout_compliance`

One registry entry (in `internal/agenttools/`, alongside `workout_adherence`),
args `{workout_id}`, `TierRead`, building exactly one
`GET /workouts/{id}/compliance`; body forwarded verbatim including the
`unavailable` shape (the agent should see *why* scoring was refused). The
announced-tools list derives from the registry (`AnnouncedToolNames`), so no
manual list bump — but the new tool needs a golden schema entry:
`go test -tags=goldengen -run TestCaptureAnnouncedSchemas ./internal/mcpserver/`.

## Risks / Trade-offs

- **[Risk] Positional matching silently trusts lap provenance** — an athlete
  who ran the pushed structured workout but added manual laps produces a
  mismatch → Mitigation: strict count equality means misalignment can't produce
  wrong per-interval numbers, only an honest `unavailable`; the response's
  `planned_steps`/`executed_laps` make the cause visible.
- **[Risk] Scores drift when athlete config changes** (FTP bump rewrites zone
  bands, historical reads re-score) → Mitigation: accepted — identical to the
  effective-program precedent; the coach reads compliance close to the session.
  If historical fidelity ever matters, a snapshot column is a clean follow-up.
- **[Risk] Avg-over-lap hides intra-lap variance** (a lap that surged and faded
  can average in-band) → Mitigation: v1 scores what splits carry; per-second
  stream analysis is out of scope and would need new ingestion.
- **[Risk] Tolerance constants (5% / 25% / ±10%) are judgment calls** →
  Mitigation: package constants exercised by table-driven unit tests; tuning is
  a one-line change that alters no shapes.
- **[Trade-off] Secondary target excluded from the score** keeps the formula
  explainable at the cost of not penalizing a blown cadence gate — revisit if
  the coach wants it folded in.
- **[Trade-off] 200-unavailable vs 409 for lap mismatch** — a 200 keeps "read
  succeeded, here's why there's no score" distinct from "you asked about the
  wrong resource state", at the cost of clients having to branch on `status`.

## Migration Plan

None. No schema change — the read composes existing rows (`workouts`,
`workout_splits`, `workout_templates`, `plan_slots`, `athlete_config`) at
request time. Deploy ships endpoint + tool; `task swag` regenerates docs;
`goldengen` regenerates the MCP schema baseline. Rollback = revert; nothing
persisted.

## Open Questions

- Should warmup/cooldown/recovery steps weigh less than `interval`/`active`
  steps in the overall score (intent weighting)? V1 says no (duration-weighted
  only); revisit after real sessions are scored.
- Distance-step duration weighting uses a pace-midpoint estimate — good enough,
  or should distance steps just weight by actual lap time unconditionally?
- Fold `secondary` into the step score with a small weight (e.g. 0.9/0.1
  primary/secondary) once there's coaching signal that the gate matters?
- Tolerant matching for the one-extra-trailing-lap case (watch auto-lap after
  the final step) — worth a narrow special case if it shows up often in real
  Garmin data.
