## Context

The sports-nutrition literature (Jeukendrup 2014, Burke 2017, ISSN 2017 position stand) converges on a small set of CHO/hr ratios scaled by duration and intensity. Endurance athletes carry these in their heads; agents trying to answer "what should I eat for tomorrow's ride" reconstruct them session by session from training data, and the math goes off the rails on edge cases (45-vs-60-min boundary, GI-distress-limited tolerance on the run). One stateless endpoint with the literature baked in fixes that.

The endpoint sits in `internal/raceprep/` as a sibling to `plan_carb_load` — both are stateless computation tools that take parameters in and return derived numbers. Neither writes; both leave persistence (`workout_fuel_entries` for actuals, `daily_goal_overrides` for the targets) to the caller. `plan_carb_load(apply: true)` is the deliberate exception (auto-write carb-load targets) — that's a different shape problem and the layering principle holds.

Most design weight here is in (a) the literature table itself — what CHO/hr at what duration/intensity, what sport modifiers, (b) the two input modes (workout_id vs explicit), (c) where the body-weight resolver lives now that three callers want it.

## Goals / Non-Goals

**Goals:**

- One read endpoint + one MCP tool returning a pre/intra/post fueling recommendation.
- Two input modes: `workout_id` (pull from row) and explicit (`sport` + `duration_min` + `intensity_zone`).
- Body weight resolved via the existing 4-tier rule (explicit > rolling 7d > last-before-date > 400).
- Numeric values grounded in published ratios, with `rationale` strings the agent can pass back to the user verbatim.
- `intra_workout.applicable: false` for sessions where the literature says "don't bother" (e.g., < 45 min CHO, strength sessions).
- Reuse the protein-distribution MPS threshold (0.3 g/kg) for the post-workout protein recommendation — single source of truth.

**Non-Goals:**

- Caffeine, sodium personalization beyond a documented midpoint, environmental modifiers, sweat-rate-derived targets, brick sessions, hypertrophy-specific protein peri-protocols, persistence, auto-write into `workout_fuel_entries`, phase-aware modifiers, race-day-only protocols (those are `plan_carb_load`).

## Decisions

### 1. Sport enum: `bike` / `run` / `swim` / `row` / `strength` / `other`

Matches the existing `workouts.Sport` enum exactly so `workout_id` mode maps without translation. Each maps to a profile:

- **bike, row**: highest in-session CHO tolerance (~90 g/hr ceiling), highest fluid range.
- **swim**: literature for in-session intake is thin; default to "intra not applicable for sessions ≤ 120 min" because gel access during a swim set is unrealistic; post-workout standard.
- **run**: lower CHO ceiling (~60 g/hr) due to GI distress; fluid ~400–600 ml/hr (carry constraint).
- **strength**: `intra_workout.applicable: false`. Pre + post protein only.
- **other**: bike profile as the default (most generic endurance pattern).

### 2. Intensity zones: 1–5 with literature bucketing

```
Zone 1 — recovery / very easy aerobic     (< 65% HRmax)
Zone 2 — easy aerobic / endurance         (65–75%)
Zone 3 — tempo / moderately hard          (75–85%)
Zone 4 — threshold / hard                 (85–92%)
Zone 5 — VO2max / interval                (> 92%)
```

Zones 1–2 ("aerobic"): lower CHO/hr (fat oxidation share is higher).
Zones 3–4 ("intensive endurance"): mid to high CHO.
Zone 5: short by nature, usually < 45 min total — `intra_workout.applicable` is duration-driven and usually false here.

**Workout-mode zone derivation:**
If the row has `tss` and we know `duration_min`, compute intensity factor `IF = sqrt(tss / (duration_min/60) / 100)` and map:
- IF < 0.65 → Zone 1
- 0.65–0.75 → Zone 2
- 0.75–0.85 → Zone 3
- 0.85–0.92 → Zone 4
- > 0.92 → Zone 5

If `tss` is absent: default to **Zone 2** with a note in `rationale` ("intensity defaulted to Z2 because the workout has no TSS"). Agent can re-call with an explicit `intensity_zone` if it has the context.

**Alternatives considered:**

- *Use `avg_hr` + the user's threshold HR.* Rejected — threshold HR isn't in the API today; adding it would couple this change to a separate athlete-profile schema.
- *Use RPE.* Same problem — no field for it. Defer.

### 3. Pre-workout: 1–4 g CHO/kg, scaled by zone

Literature: 1–4 g CHO/kg in the 1–4 hours before, with the lower end for shorter/lower-intensity work and the higher end for race-week / glycogen-depleting sessions.

```
Zone 1-2 (aerobic):       1.0 g/kg in [60, 120] min before
Zone 3:                   1.5 g/kg in [60, 120] min before
Zone 4:                   2.0 g/kg in [60, 180] min before
Zone 5:                   1.0 g/kg in [60, 90] min before (small + close — avoid GI load before high-intensity)
strength:                 0.5 g/kg in [30, 90] min before (lower CHO, focus on protein in the prior meal)
```

`window_minutes_before` is a `[lo, hi]` range. The response carries both the absolute carbs_g and the per-kg ratio so the agent can reason about the magnitude.

### 4. Intra-workout: applicability + per-hour rates

```
duration < 45 min OR sport=strength
    → applicable: false; all numeric fields nil.
sport=swim AND duration ≤ 120 min
    → applicable: false (no realistic in-session intake).
duration 45-90 min, Zone 1-2
    → 30 g CHO/hr, single transportable; fluid 400-600 ml/hr; sodium 300 mg/hr.
duration 45-90 min, Zone 3-4
    → 60 g CHO/hr; fluid 500-700 ml/hr; sodium 500 mg/hr.
duration 90-180 min, Zone 1-2
    → 60 g CHO/hr; fluid 500-700 ml/hr; sodium 400-500 mg/hr.
duration 90-180 min, Zone 3-4
    → 60 g CHO/hr (capped — GI tolerance), single transportable; fluid 600-800 ml/hr; sodium 600 mg/hr.
duration > 180 min, any zone
    → 90 g CHO/hr (multiple transportable, glucose:fructose 2:1); fluid 600-800 ml/hr; sodium 600-800 mg/hr.
```

`run` profile caps `carbs_g_per_hour` at 60 even in the > 180 min bucket (GI tolerance). `bike` and `row` allow 90.

For ranges that the literature reports as a band (sodium especially), the response surfaces a single midpoint number with a `rationale` note that the validated range is 300–800 mg/hr. Agent-side personalization can adjust.

**Alternatives considered:**

- *Return ranges, not midpoints, for sodium/fluid.* Considered. Rejected for v1 — a single number is what the agent passes to the user; the rationale carries the band. Reconsider if real use shows agents always want the range.
- *Scale carbs_g_per_hour smoothly by duration.* Rejected — the bucket boundaries are sharp in the literature too (45/90/180 min). Smoothing would be more wrong than honest.

### 5. Post-workout: glycogen replenishment + MPS

```
carbs_g     = 1.0 g/kg (first 60 min after — peak glycogen window).
protein_g   = 0.3 g/kg (MPS threshold from add-protein-distribution).
window_minutes_after = [0, 60]
```

Single rule across all sports/zones because the recovery science doesn't materially differentiate. Zone 5 sessions might use less because total work was lower; that's an over-estimate in the conservative direction.

Reusing the `0.3 g/kg` MPS threshold from add-protein-distribution keeps the system consistent — a user who hits the post-workout protein recommendation will also get `mps_effective: true` in their `protein_distribution` query on that day. Single literature constant, two endpoints.

### 6. Body-weight resolution: hoist the duplicated helper into `internal/bodyweight/`

Two callers already duplicate the 4-tier date-anchored resolver:
1. `internal/summary/protein.go` (just shipped) — full ownership.
2. This change's `internal/raceprep/recommend.go` — third caller.

Rule of three. Hoist a `bodyweight.ResolveAtDate(ctx, repo, date, loc, override) (kg float64, source string, err)` helper, call it from both. The `internal/energy/composition.go` resolver stays — it's the window-anchored variant and has different semantics.

### 7. Endpoint placement: `/race-prep/recommend-workout-fuel`

Same namespace as `plan_carb_load` (the existing literature-tables tool). Adjacent to `/race-prep/carb-load` and `/race-prep/carb-load/apply`. The naming matches the MCP tool name (`recommend_workout_fuel`) so the URL is predictable.

**Alternatives considered:**

- `/workouts/{id}/recommend-fuel` — would require a separate explicit-mode endpoint. Two endpoints for one concept fragments the surface.
- `/fueling/recommend` — new namespace. Premature; race-prep already exists.

### 8. Input-mode validation: exactly one mode

```
neither workout_id nor (sport + duration_min + intensity_zone)
    → 400 input_required
both workout_id AND (sport or duration_min or intensity_zone)
    → 400 input_conflict
workout_id present but not found
    → 404 workout_not_found
explicit mode partial (e.g., sport without duration)
    → 400 sport_required / duration_min_required / intensity_zone_required
```

The handler runs this validation before service-layer logic. The OpenAPI spec documents the exclusivity in the description.

### 9. MCP tool: same param shape as REST

`RecommendWorkoutFuelArgs{WorkoutID *string; Sport *string; DurationMin *int; IntensityZone *int; BodyWeightKg *float64}` — all pointers because all are optional at the args level (mode-exclusivity is enforced at the REST endpoint). The tool description leads with:

- The two input modes (workout_id pulls from the row; explicit is for planned/future sessions).
- The exact literature ratios it returns (so the agent doesn't reconstruct them from training data).
- The reused MPS threshold for the post-workout protein recommendation.
- What's NOT covered (caffeine, environmental, sweat-rate, race-loading — point at `plan_carb_load`).

## Risks / Trade-offs

- **Literature values are a moving target.** The CHO/hr ceiling has been climbing (60 → 90 → some recent work at 120). v1 codifies the conservative consensus; if/when consensus shifts, the values are constants in one file. → Low cost to revise.
- **Single midpoint for sodium hides individual variability.** A heavy sweater needs 700+ mg/hr; a light sweater 300. The notes say so, but the agent might pass the midpoint to the user verbatim without context. → Mitigation: the rationale string explicitly says "personalize to sweat rate." T3 #6 sweat-rate workflow will fix this properly.
- **Workout-mode `tss` derivation is fragile when `tss` is absent.** Default to Z2 is a defensible neutral, but the agent might not catch the defaulted-band note. → Mitigation: `rationale` calls it out loudly; `intensity_zone` in the response is the agent's signal to either trust it or pass explicit.
- **The CHO/hr buckets have hard edges** (45/90/180 min). A 44-min Z3 ride gets `intra: not applicable`; a 46-min one gets 60 g/hr. Honest about the bucket; the rationale explains. Smoothing would be more wrong than honest.
- **No phase awareness** means a recovery-block ride gets the same recommendation as a build-block one of the same shape. Phases are a *daily-target* concept; the per-session recommendation is independent. The agent overrides if the build/recovery context matters. → Documented; revisit only if friction is real.
- **No sport-modifier for triathlon.** Brick sessions get answered per-segment. → Future proposal if it earns the surface.
- **Reusing the MPS threshold tightly couples this change to add-protein-distribution's literature constant.** If a future change re-tunes that constant (e.g., to 0.4 g/kg for cutting blocks), this endpoint inherits it. → Intentional. Single source of truth beats coordinated literature constants.

## Migration Plan

- Forward: no schema change. Code-only. Deploy lands the endpoint behind the existing `BearerAuth`.
- Rollback: revert the binary.
- No feature flag — additive, read-only.

## Open Questions

- Whether to surface `carbs_g_total` for the intra-window when `duration_min > 60` (i.e. for a 90-min ride at 60 g/hr, report `total: 90` alongside `per_hour: 60`). Tentative yes — it's the number the agent passes to the user ("plan to take 90 g during the ride"). Decide in spec/impl.
- Whether the response should include a `confidence: "high" | "medium" | "low"` field signaling when the recommendation is grounded in tightly-controlled literature vs extrapolated. Tentative no — adds complexity, and the rationale string already lets the agent surface uncertainty in natural language. Reconsider if the agent's framing is consistently overconfident.
- Whether to accept body-weight via a `body_fat_pct` → FFM derivation path like EA does. Tentative no — fueling recommendations scale with total body weight (not FFM); the FFM logic is EA-specific.
- Whether to allow `duration_min` of 0 (some users might query "what's the post-workout for a missed session"). Tentative no — return `400 duration_min_invalid` for ≤ 0. The "skipped workout" question is an agent-side narrative, not an API surface.
