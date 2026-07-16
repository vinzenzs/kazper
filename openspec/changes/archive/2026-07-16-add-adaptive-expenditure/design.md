## Context

Meals store per-entry kcal (product-derived or freeform); the body-weight capability stores weigh-ins and already serves a rolling-average **trend** designed to suppress the 1–2 kg daily noise (hydration, glycogen) that would otherwise dominate any balance computation. Goals hold an assumed kcal target with no feedback loop. Energy balance closes it: over a window, `intake − expenditure = Δ stored energy ≈ Δ trend-mass × 7700 kcal/kg`.

## Goals / Non-Goals

**Goals:** a data-derived expenditure number with visible inputs and hard gates; reuse the existing weight-trend smoothing (one smoothing truth in the system); advisory-only.

**Non-Goals:** MacroFactor's full program layer (weekly auto-adjusted targets, diet phases) — the coach *is* that layer here; per-macro partitioning of the estimate; body-composition modeling (7700 kcal/kg is the standard mixed-tissue constant, documented as such); imputing unlogged days.

## Decisions

### D1 — Balance over the window's trend endpoints, not day-by-day regression
`expenditure = mean(intake over logged days) − (trend(end) − trend(start)) × 7700 / window_days`. Using the smoothed trend at the window's ends (rather than regressing daily weights) delegates noise handling to the body-weight capability's existing trend — one smoothing implementation, already specced with its `sample_count` honesty. Day-level regression adds fragility without accuracy at these window lengths.

### D2 — "Logged day" = at least one meal; unlogged days excluded and counted
Kazper can't distinguish fasting from not-logging; treating an empty day as zero intake would wreck the estimate silently. A day with ≥ 1 meal counts; others are excluded from the intake mean and reported (`days_unlogged`). The known bias — partially-logged days drag the estimate down — is stated in the endpoint description; the gate (D3) bounds it.

### D3 — Gates: ≥ 14 logged days, ≥ 5 weigh-ins spanning the window
Below ~2 logged weeks the intake mean is noise; without weigh-ins near both ends the trend delta is fiction (the trend values used are echoed with their dates so staleness is visible). Both gates degrade to `200` + `reason` (the CP-model posture). Default window: the caller's choice; 21–28 days is the honest sweet spot and the tool description says so. Range cap 92 days (nutrition tier).

### D4 — Uncoupled from goals
The response carries no goal comparison — the coach reads goals separately and owns the "your target assumes X, reality is Y" framing, exactly like CP-vs-configured-FTP. Applying a change is the existing deliberate goals/override PUT.

### D5 — Response
`{expenditure_kcal_per_day, window: {from, to, days}, trend: {start_kg, start_date, end_kg, end_date, delta_kg}, intake: {mean_kcal_logged_days, days_logged, days_unlogged}, reason?}` — kcal/day and kg `Round1`, boundary-only. Sentinels: shared range vocabulary; gates per D3.

## Risks / Trade-offs

- **Under-logging bias** (snacks unlogged → expenditure underestimated) — inherent to the method everywhere, mitigated by the logged-day rule + visible counts; the coach knows the athlete's logging discipline.
- **Water-mass swings** (carb-load week, race taper) distort short windows — mitigated by the trend smoothing + the 14-day floor; the tool description warns against reading it across a deliberate glycogen manipulation.
- **7700 kcal/kg is a simplification** — documented; the error is second-order at these deltas.

## Migration Plan

None. Rollback = revert route/tool.

## Open Questions

- Weekly "expenditure history" series (MacroFactor's chart) — deferred; call the endpoint per window until the shape proves wanted.
