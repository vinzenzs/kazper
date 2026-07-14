## Context

`activity-streams` (migration 056) persists contiguous 1 Hz power/speed/HR arrays per workout and already computes execution metrics (NP/VI/EF/decoupling) from them; `GET /workouts/{id}/streams` serves the arrays with bucket-mean `downsample` (bounds [10, 5000], echoed). `add-critical-power-model` serves windowed CP (W) and W′ (kJ) estimates with fit quality — advisory, never written to config. Its design left one open question for this change: where W′bal gets its parameters.

W′ balance models the anaerobic work capacity as a battery: riding above CP drains it at `P − CP` joules per second; riding below CP recharges it toward W′. The differential form (Froncioni–Clarke–Skiba) is a simple recursion over the 1 Hz series; the original Skiba integral form additionally fits a recovery time constant τ from the session's sub-CP power.

## Goals / Non-Goals

**Goals:**
- Per-workout W′bal series + summary from the stored power stream, computed on read, nothing persisted.
- Parameters explicit and auditable — the response echoes exactly what it was given.
- The summary (not the series) is what the coach agent consumes over MCP.

**Non-Goals:**
- The integral Skiba model / fitted τ (needs validation data the single-athlete setup can't provide; differential is what Intervals.icu ships).
- Run/swim W′bal (critical speed isn't modeled yet — deferred with it).
- Persisting W′bal summaries or aggregating across workouts (revisit if the coach reads it routinely enough that recompute cost matters).
- CP/W′ as `athlete-config` columns (would imply confirmed physiology + migration; the advisory posture stays until routine use argues otherwise).

## Decisions

### D1 — Parameters are explicit query args, resolved by the caller (settles cp-model's open question)
`cp_watts` and `w_prime_kj` are required query parameters. The coach agent composes `cp_model` → `w_prime_balance`; the dashboard reuses the cp-model fit it already fetches. The endpoint validates positivity and echoes both back in the response.

- **Why not auto-resolve from the cp-model fit at request time?** That buries a second computation, imposes a window opinion (trailing 90 days? season?) the endpoint has no business holding, and makes results non-reproducible as the window slides.
- **Why not config columns?** A migration plus a write surface for values the athlete hasn't confirmed — and the CP model was deliberately advisory-only. Explicit params keep W′bal a pure function of (stream, params): same inputs, same answer, trivially testable.

### D2 — Differential (Froncioni–Clarke–Skiba) model at 1 Hz
Per sample: above CP, `bal -= (P − CP)·dt`; below CP, `bal += (W′ − bal)·(CP − P)/W′·dt`, starting from `bal = W′`. O(n), closed-form per step, hand-computable fixtures. The integral model's fitted τ adds a parameter the data can't validate; the differential form is the widely deployed default.

### D3 — Balance is not clamped at zero
A negative minimum is diagnostic, not an error: it means the ride demonstrated more anaerobic work than the supplied W′ allows — i.e. the parameters are stale or the fit underestimated. Clamping would hide exactly the signal that should send the athlete back to `cp_model`. `max_depletion_pct` may accordingly exceed 100.

### D4 — Response shape: summary + optionally-downsampled series
`{params: {cp_watts, w_prime_kj}, duration_s, summary: {min_w_prime_kj, min_at_s, end_w_prime_kj, max_depletion_pct, time_below_25_pct_s}, series: [...]}`. The series reuses the streams-GET `downsample` convention verbatim (bucket-mean, bounds [10, 5000], echoed; full resolution when omitted) — the exact minimum always lives in the summary, so bucket-mean smoothing loses nothing that matters. `summary_only=true` omits the series entirely. kJ values `Round1`, `max_depletion_pct` `Round1`, at the boundary only.

### D5 — Gate on a stored power stream, not on sport
Requirement is data, not taxonomy: any workout with a stored power series computes (a power-meter run works; a bike ride synced before migration 056 doesn't). Missing power while other streams exist is its own sentinel `404 power_stream_missing` — distinct from `streams_not_found` (nothing stored) per the 1:1 sentinel convention. Param failures: `400 cp_invalid` / `w_prime_invalid` (missing, non-numeric, or ≤ 0).

### D6 — MCP tool always requests `summary_only=true`
`w_prime_balance` (read tier) builds its one GET with `summary_only=true` hardcoded — the agent gets params echo + summary, never the series, honoring the `persist-activity-streams` precedent that raw series are chart data, not reasoning inputs. Tool args: `workout_id`, `cp_watts`, `w_prime_kj`; description points at `cp_model` as the parameter source.

### D7 — Dashboard strip on the workout-detail route
`/workouts/:id` gains a W′bal strip (visx area/line over time, min marked) for workouts whose streams include power: parameters come from the cp-model fetch the stats page already uses (trailing-90-day window), via a shared hook. When the cp-model is null or power is absent the strip renders nothing — absence, not an error state, since W′bal is supplementary detail on this page.

## Risks / Trade-offs

- **Wrong params → confidently wrong series** — mitigated by the params echo, the unclamped negative floor (D3) making staleness visible, and the advisory framing in the tool description.
- **Full-resolution series is large** (5 h ride ≈ 18k points) — same trade-off the streams GET already accepts; `downsample` and `summary_only` are the pressure valves, and the MCP surface never carries it.
- **Differential model recovers slightly faster than τ-fitted integral in long sub-CP stretches** — known property; accepted for v1 (the summary metrics the coach acts on are dominated by depletion, not recovery tails).

## Migration Plan

Additive only: one endpoint, one MCP tool, one dashboard strip. No migration, no config. Rollback = revert route/tool registration.

## Open Questions

- Is `time_below_25_pct_s` the right "red zone" threshold, or should the summary carry a small fixed set (10/25/50%)? (v1: 25% only; constants-over-config precedent.)
- Windowed aggregation ("how often did I empty the battery this block?") — deliberately deferred; would need persisted summaries first.
