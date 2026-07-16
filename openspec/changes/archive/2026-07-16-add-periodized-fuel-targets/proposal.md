## Why

Fuelin's core idea — *fuel for the work required* — from the nutrition gap analysis (2026-07-16): carb needs should track the training day, not a flat goal. Kazper is uniquely positioned to do this without Fuelin's manual plan-sync: tomorrow's planned session (TSS, duration) is already in the training plan, per-date goal **overrides** already exist as the application mechanism, and body weight supplies the g/kg denominator. What's missing is the classifier and the suggestion surface.

## What Changes

- `GET /api/v1/nutrition/fuel-plan?from=&to=&tz=` (default: today + 6 days) — per day: the planned sessions (id, sport, planned TSS/duration), a derived **day tier** from total planned load — `rest` (no session), `easy` (< 60 planned TSS), `moderate` (60–150), `heavy` (> 150 or any session ≥ 150 min) — and the suggested carb target: tier × g/kg constant (**3 / 5 / 7 / 9 g/kg**, the sports-nutrition ladder, constants v1) × the latest body-weight trend value, reported as `suggested_carbs_g` beside that date's currently-effective goal carbs and the delta.
- **Suggestions, never writes** (the threshold-selector lesson applied to nutrition): applying a day's suggestion is the existing per-date goal-override PUT, proposed by the coach and confirmed in chat. Days with no plan data degrade to `rest`-tier suggestions flagged `plan_missing: true`.
- `/context/daily` gains a compact `fuel_plan` block — today's and tomorrow's tier + suggested carbs — beside the goals block, so every morning check-in sees the classification without an extra call.
- New `fuel_plan` MCP tool (read tier, one GET, verbatim).
- Compute-on-read, no migration; new `internal/fuelplan/` package (aggregator over planned workouts + bodyweight + goals, the `workoutfueling` pattern).

## Capabilities

### New Capabilities

- `fuel-periodization`: the day classifier, g/kg ladder, suggestion semantics, and the MCP tool.

### Modified Capabilities

- `daily-context`: 1 ADDED requirement — the `fuel_plan` block in `/context/daily`.

## Impact

- **Code:** `internal/fuelplan/` (pure classifier + handler over narrow planned-workouts / bodyweight-trend / goals read interfaces); context fold; MCP + golden additive; `task swag`.
- **Out of scope (deferred):** auto-writing overrides (deliberate writes stand), kcal/protein periodization (carbs are the periodized macro; protein stays flat by design), Fuelin-style meal-level traffic lights (day-level is the honest granularity for v1), tier-threshold tuning (constants precedent).
