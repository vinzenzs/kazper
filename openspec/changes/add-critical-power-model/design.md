## Context

`effort-analytics` stores one best-effort row per (workout, metric, duration) — the mean-maximal ladder at 5s/15s/30s/1m/5m/10m/20m/30m/60m — and already serves the windowed per-duration MAX as `GET /workouts/power-curve`. The 2-parameter critical power model is a straight line over exactly that data: total work `W(t) = CP·t + W′`, so plotting best-effort *work* (power×duration) against duration and fitting least squares yields CP (slope) and W′ (intercept). Kazper has the data, the query, and the package; what's missing is the fit.

Threshold context: `athlete-config` holds the authoritative `ftp_watts` (versioned by `threshold-history`); per-sport TSS, race pacing, and zone resolution all consume it. Nothing in the system estimates a threshold *from* performance data.

## Goals / Non-Goals

**Goals:**
- A windowed, compute-on-read CP/W′ estimate with explicit fit quality and the exact points it used — auditable, never a black box.
- Honest degradation when the window can't support a fit (too few points, too narrow a span).
- Keep it advisory: a number the coach compares against the configured FTP, never a write.

**Non-Goals:**
- W′bal per-workout depletion tracking (follow-up change; consumes this model's CP/W′).
- Run/swim critical speed (same math, different metric — deferred until asked for).
- 3-parameter Morton / exponential models (CP2 is the robust default; more parameters need better data).
- Auto-updating `athlete-config` or writing `threshold-history` (threshold writes stay deliberate; the existing PUT → snapshot → recompute-tss path is the application route).
- Population percentile ranking (the "power profile" gap-analysis item — separate, trivial, later).

## Decisions

### D1 — Lands in `effort-analytics`, not a new capability
The fit is a read-side aggregation over rows this capability owns, exactly like `power-curve`. A new package would need the same repo projection injected and would fragment "analytics over best efforts" across two capabilities. Precedent: `add-intensity-distribution` extended `workout-stats` rather than minting a capability. The delta is ADDED-only, so it composes safely with the existing spec.

### D2 — Linear CP2 in work–time form over the windowed MAX points
For each ladder duration in the validity band, take the windowed best (the same per-duration MAX the power curve serves, bike/power metric) and fit ordinary least squares on `work_j = cp · t_s + w_prime_j`. CP is the slope (W), W′ the intercept (J, reported as `w_prime_kj`, `Round1`). Fit quality: `r_squared` (`Round3`) and `rmse_w` (RMSE of predicted-vs-actual *power* per point, `Round1`) — power-space residuals read naturally ("the fit misses your 5-min best by 4 W").

- **Why the work–time form?** It makes CP2 exactly linear — closed-form least squares, no iterative solver, trivially unit-testable against hand-computed fixtures.
- **Why windowed MAX per duration (one point per duration) rather than every workout's efforts?** The mean-maximal envelope is the model's subject; per-workout submaximal efforts below the envelope would drag CP down. This matches how GC's CP chart fits the season curve.

### D3 — Validity band [120s, 1800s]; gates before fitting
Only ladder durations 2m/5m/10m/20m/30m enter the fit (below 2 min the anaerobic term dominates and inflates W′; above 30 min motivation/fueling — not physiology — caps the data; 60m and sprints are excluded). Gates, checked in order, each producing a null model with a machine-readable `reason`:
- fewer than **3** distinct durations with a point in band → `insufficient_points`
- longest in-band duration < **3×** the shortest present → `span_too_narrow` (a 2m+5m-only fit extrapolates wildly)

A gated response is still `200` with `model: null`, `reason`, and whatever `points` were found — the race-pacing "degrade with a reason, don't error" posture (an empty or thin window is a data state, not a client mistake).

### D4 — No `athlete-config` read; interpretation stays with the consumer
The response carries CP/W′/fit/points and nothing about the configured FTP. The coach agent already reads `athlete_config_get`, and the dashboard already fetches the config for its thresholds panel — both can render "CP 262 W vs configured FTP 250 W" without this endpoint coupling to the config package. This also sidesteps widening the athlete-config consumption-gate spec for a display-only read. CP≈FTP framing lives in the tool/endpoint description, not in a `suggested_ftp_watts` field that would imply more precision than the model has.

### D5 — Bike/power only in v1
`sport` is not a parameter; the fit reads the `power` metric rows. Run/swim critical speed is the same linear model over distance–time, but pace thresholds already flow in from Garmin and the coaching questions to date are bike-FTP-shaped. Adding CS later is an ADDED requirement with a `sport` param, not a breaking change.

### D6 — Window contract mirrors `power-curve`
`from`/`to`/`tz` with the shared range error vocabulary (`range_required`, `date_invalid`, `range_invalid`, `range_too_large` + `max_days`, `tz_invalid`) and the same ≥1-year support. A typical call is the trailing 90 days (the coach's choice); the endpoint doesn't impose a "recency" opinion beyond the window it's given.

### D7 — MCP `cp_model`, read tier
One GET, body forwarded verbatim, registry-derived + golden-gated like `power_curve`/`pmc_series`. Tool description states the advisory CP≈FTP interpretation and points threshold *changes* at the existing athlete-config update flow.

### D8 — `/stats` panel: readout + fit visualization
CP / W′ / R² readout cards plus the in-band effort points with the fitted curve `P(t) = CP + W′/t` (visx, log-x like the power curve panel), null-model state rendering the gate reason ("not enough long efforts in this window to estimate CP"). Window selector reuses the panel conventions from PMC (90/180/365).

## Risks / Trade-offs

- **Garbage-in fits** — a window with only stale or submaximal efforts yields a confidently wrong CP → mitigation: the response always carries its points and fit quality; the dashboard shows them; the gates catch the degenerate cases. The model is advisory by design (D4).
- **W′ from MAX-envelope points can mix workouts** — the 2m point and 20m point may come from different days/forms; that's inherent to season-curve fitting (GC does the same) and is visible via each point's workout id/date.
- **Only 5 ladder durations in band** — the fixed ladder caps fit resolution. Accepted: adding fit-specific durations would require re-deriving stored best efforts; the 5-point fit is what the data supports and the gates enforce honesty.

## Migration Plan

Additive only: one endpoint, one MCP tool, one dashboard panel. No migration, no config, no change to ingest or existing responses. Rollback = revert the route/tool registration.

## Open Questions

- Should the gate thresholds (≥3 points, 3× span) be tunable, or are constants fine for one athlete? (v1: constants, mirroring the intensity-distribution 20/75 precedent.)
- W′bal follow-up: consume this endpoint's CP/W′ at request time, or require the athlete to confirm values into config first? (Decide when proposing W′bal.)
