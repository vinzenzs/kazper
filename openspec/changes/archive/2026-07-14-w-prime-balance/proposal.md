## Why

`add-critical-power-model` (archived 2026-07-14) produces the CP/W′ parameters; this is the payoff feature they parameterize — flagged in both the GoldenCheetah and Intervals.icu gap analyses. W′ balance (Skiba) turns a stored 1 Hz power stream into the ride's anaerobic-battery story: how deep W′ was drained during each hard effort and how much was left at the end. Today the coach can see *that* an interval session was hard (TSS, step compliance) but not *how close to empty* the athlete got — the difference between "finished the 5th rep comfortably" and "finished with 2 kJ in the tank", which is exactly the signal for progressing or backing off interval prescriptions.

## What Changes

- `GET /api/v1/workouts/{id}/w-prime-balance?cp_watts=&w_prime_kj=` — computes the W′bal time series over the workout's stored power stream using the differential (Froncioni–Clarke–Skiba) model, returning a summary (minimum balance + when, end balance, max depletion %, time spent below 25%) and the series (with the existing `downsample` convention); `summary_only=true` omits the series.
- **CP/W′ are explicit request parameters** — resolved by the caller (typically from the `cp_model` endpoint), never auto-fitted or read from config. Compute-on-read, persists nothing.
- New `w_prime_balance` MCP tool (read tier, one GET) that always requests `summary_only=true` — the series stays chart data, honoring the "raw streams are not reasoning inputs" MCP precedent.
- Coach-dashboard workout-detail page gains a W′bal strip for rides with a stored power stream, parameterized from the cp-model fit it already fetches for `/stats`; hidden when either is unavailable.
- No migration, no new package (lands in `internal/activitystreams/`, which owns per-workout stream reads and execution metrics).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `activity-streams`: 2 ADDED requirements — the per-workout W′bal computation endpoint, and the `w_prime_balance` MCP tool.
- `coach-dashboard`: 1 ADDED requirement — the workout-detail W′bal strip.

## Impact

- **Code:** `internal/activitystreams/` gains a pure `wprimebal.go` (differential model + summary) + handler reusing the stored-stream read; `apps/web` workout-detail view gains the strip + hook/types.
- **API/MCP:** one new GET; one new read MCP tool (registry-derived, golden regen additive). `task swag` required.
- **Dependencies/systems:** none new; no migration (head stays `059`); dataexport untouched.
- **Out of scope (deferred):** the integral Skiba model with fitted τ, run/swim W′bal, persisting per-workout W′bal summaries, CP/W′ columns on `athlete-config`, and any windowed cross-workout W′bal aggregation.
