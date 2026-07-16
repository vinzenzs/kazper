## Why

EatMyRide's core, from the nutrition gap analysis (2026-07-16): riders guess in-ride fueling, but carb burn is estimable *before* the session from planned load, and the intake plan follows arithmetically. Kazper has the inputs — planned TSS/duration on the workout, effective FTP (post threshold-split), and the race-fueling-plan precedent for the race case — but no training-day equivalent. This closes the rehearse-side of the loop: plan the session's fueling, log what was actually taken (workout-fuel), and let the coach compare.

## What Changes

- `GET /api/v1/workouts/{id}/fueling-plan?carbs_per_hr=` for a **planned** workout — estimates the session's work and substrate demand, then prescribes intake:
  - **Work:** `kJ ≈ planned_tss / 100 × effective FTP × 3.6`; energy expenditure via the standard kJ≈kcal convention.
  - **Carb burn:** kcal × a CHO fraction from planned intensity (`IF < 0.60 → 45 %`, `0.60–0.75 → 55 %`, `0.75–0.85 → 70 %`, `> 0.85 → 80 %` — constants v1) ÷ 4 kcal/g.
  - **Intake prescription:** duration-gated guidance — `< 60 min → 0`, `60–150 min → 30–60 g/hr`, `> 150 min → 60–90 g/hr` — clamped by the optional `carbs_per_hr` capacity param (the athlete's tested gut tolerance; explicit-params convention), with per-hour targets and session totals, plus the projected glycogen deficit (burn − intake).
- Honest degradation: not a planned workout → `409 workout_not_planned`; missing planned TSS/duration → `200` with `reason: "plan_data_missing"`; missing effective FTP → burn omitted, duration-based intake guidance still returned (`reason: "ftp_missing"`).
- New `workout_fueling_plan` MCP tool (read tier, one GET, verbatim) — the coach's "here's tomorrow's fueling" call.
- Compute-on-read, no migration; lands in `internal/workoutfueling/` (which already composes workouts + fuel; FTP arrives via the effective-config adapter).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `workout-fuel`: 2 ADDED requirements — the fueling-plan read and the MCP tool.
- `athlete-config`: 1 MODIFIED requirement — the consumption gate gains the workout-fueling-plan's `ftp_watts` read (fail-open).

## Impact

- **Code:** `internal/workoutfueling/` pure burn/prescription math + handler (effective-config adapter for FTP); MCP + golden additive; `task swag`.
- **Out of scope (deferred):** product-slotted schedules ("gel at 1:30" needs serving-size modeling on products), post-session plan-vs-actual compliance scoring, carb-capacity trending from rehearsal logs, EatMyRide-style metabolic profiling from VO₂max, completed-workout retrospective plans.
