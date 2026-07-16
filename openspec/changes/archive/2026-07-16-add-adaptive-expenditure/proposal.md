## Why

Top pick of the nutrition-side gap analysis (2026-07-16, vs MacroFactor/Fuelin/EatMyRide): Kazper's kcal goals rest on an assumed expenditure nobody ever measured. MacroFactor's defining feature reverse-derives *actual* TDEE from the two series Kazper already stores — logged intake and the smoothed body-weight trend — via energy balance, beating static formulas by a wide margin (their validation: ~±9%). Every input exists (meals kcal/day; the body-weight capability already serves a noise-suppressed rolling trend); what's missing is the balance arithmetic and its honesty gates.

## What Changes

- `GET /api/v1/nutrition/expenditure?from=&to=&tz=` — estimates daily energy expenditure over the window from energy balance: `expenditure ≈ mean(logged intake) − (Δ trend-weight × 7700 kcal/kg ÷ days)`, using the body-weight capability's existing smoothed trend as the mass signal. Returns `expenditure_kcal_per_day`, the trend endpoints it used (start/end trend weight + dates), `days_logged` / `days_unlogged`, and the per-day intake series.
- **Honesty gates:** a day counts as logged only if it has ≥ 1 meal (an unlogged day is excluded and counted, never treated as zero intake); fewer than **14 logged days** → `reason: "insufficient_logged_days"`; fewer than **5 weigh-ins** spanning the window → `insufficient_weigh_ins`. Both degrade to `200` with a null estimate.
- **Advisory and uncoupled** (the cp-model posture): the endpoint reads no goals and writes nothing — the coach composes it with the goals read ("your goals assume 2,700; expenditure is running ~2,950") and any adjustment goes through the existing deliberate goals PUT.
- New `energy_expenditure` MCP tool (read tier, one GET, verbatim).
- Compute-on-read, no migration; new `internal/expenditure/` package (multi-repo aggregator over meals + bodyweight, the `workoutfueling` pattern).

## Capabilities

### New Capabilities

- `adaptive-expenditure`: the energy-balance TDEE estimate — inputs, gates, window semantics, and the MCP tool.

### Modified Capabilities

_None._

## Impact

- **Code:** `internal/expenditure/` (pure balance math + handler over narrow meals/bodyweight read interfaces wired in `httpserver.Run()`); MCP registry + golden additive; `task swag`.
- **Out of scope (deferred):** automatic goal adjustment (deliberate writes stand), diet-phase machinery (cut/bulk rate targets — the coach conversation covers it), a dashboard panel (the nutrition dashboard lens is a standing deferred tier), expenditure *history* (call with different windows).
