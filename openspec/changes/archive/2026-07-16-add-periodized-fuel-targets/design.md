## Context

Training plans materialize planned workouts carrying planned TSS/duration; goals hold flat macro targets with per-date overrides for special days; the body-weight capability serves a smoothed trend. Today the coach periodizes fuel by hand ("big day tomorrow — eat more carbs") with no computed anchor. Fuelin productized exactly this classification; Kazper has every input in-house.

## Goals / Non-Goals

**Goals:** a deterministic day classifier over *planned* load; suggestions in g/kg terms beside the effective goal so the delta is one glance; zero writes.

**Non-Goals:** auto-applied overrides; meal-level distribution (which meal carries the carbs is coach conversation); protein/fat periodization (evidence supports keeping protein flat); within-day timing (that's the workout-fueling-plan change's territory); menstrual-phase adjustments (not applicable here).

## Decisions

### D1 — Classify on planned load only, with an honest missing-plan flag
The tier reads *intent* (planned TSS/duration), not results — fueling decisions happen before the session. Days with no planned workout are `rest`; days where the plan simply doesn't extend that far are also computable only as rest, so they carry `plan_missing: true` (window end beyond the last materialized plan week) — a rest suggestion and a "no plan data" suggestion must not look alike.

### D2 — Tier thresholds and the g/kg ladder are fixed constants
`rest`/`easy`/`moderate`/`heavy` at 0 / <60 / 60–150 / >150 planned TSS (or any single session ≥ 150 min — long low-intensity days need carbs regardless of TSS), mapping to 3/5/7/9 g/kg. The sports-nutrition literature brackets these ranges; single-athlete constants-over-config precedent applies (intensity-distribution 20/75). Both the tier and the inputs that produced it are in the response, so a disagreement is auditable.

### D3 — Weight = the smoothed trend's latest value
The g/kg denominator uses the body-weight trend (same signal adaptive-expenditure consumes), echoed with its date; no weigh-in data → `reason: "weight_missing"` with tiers still classified (tiers are weight-free; only gram targets need mass).

### D4 — Suggested vs effective, side by side
Each day carries `suggested_carbs_g`, the date's currently-effective goal carbs (base or override), and `delta_g`. Reading goals here is deliberate — unlike CP-vs-FTP this endpoint's *purpose* is the comparison, and goals are nutrition-domain (no cross-philosophy coupling). Applying remains the existing override PUT; the MCP tool description points the coach at it and at confirming with the athlete first.

### D5 — Context carries today + tomorrow only
The `/context/daily` block is the check-in view ("today easy/5 g/kg; tomorrow heavy/9 — front-load tonight"); the full week stays behind the endpoint. Omitted when the window can't classify (no plan capability data at all), null-safe.

## Risks / Trade-offs

- **Planned TSS quality varies** (templates without TSS estimates) — duration fallback catches long sessions; a planned workout with neither TSS nor duration classifies `easy` and says so via the echoed inputs.
- **Suggestion fatigue** — mitigated by day-level granularity and no nagging: suggestions render in context, the coach raises them when material.
- **Double-counting risk with adaptive-expenditure** — none by design: this change periodizes *carbs within* whatever kcal target stands; expenditure informs the kcal target itself. The tool descriptions state the division.

## Migration Plan

None. Rollback = revert route/tool/context fold.

## Open Questions

- Post-heavy-day recovery tier (elevated carbs the morning after a glycogen-depleting session) — v2 candidate once real use shows whether the coach wants it computed.
