## Context

`activity-streams` stores contiguous 1 Hz power arrays and already hosts per-workout stream computations (execution metrics, W′bal). Step compliance scores rides against a *planned* template; Garmin `splits` reflect lap-button presses. Neither describes an unstructured ride's actual work/rest structure. Detection must be deterministic and parameter-free to be trustworthy in a single-athlete, no-tuning-UI system.

## Goals / Non-Goals

**Goals:**
- Deterministic, explainable detection: same stream → same intervals, with the derived threshold reported so every boundary is auditable.
- Useful output for the coach conversation: count, durations, powers, rests — not a black-box "session type" label.
- Honest null result on steady rides.

**Non-Goals:**
- Run/swim (pace streams are noisier; defer until power detection proves the shape).
- Planned-vs-detected comparison (step-compliance territory), persistence, auto-labeling, or ML anything.
- Tunable parameters — constants first (the intensity-distribution 20/75 precedent); revisit only on real misdetections.

## Decisions

### D1 — Ride-relative Otsu threshold, no parameters
Smooth power with a 30 s centered rolling mean, then find the work/rest threshold by Otsu's method (maximize between-class variance over the smoothed distribution). No CP, no config, no query params — the ride defines its own contrast.

- **Why not threshold at a CP fraction (the W′bal-style explicit param)?** Detection is about *structure*, not physiology: tempo intervals in a recovery ride sit far below CP but are plainly intervals. Ride-relative detection finds them; a CP-anchored one can't without per-call tuning.
- **Why Otsu over k-means/changepoint?** Closed-form over a histogram, deterministic, no iteration/seeds, trivially unit-testable — and it degrades detectably (low between-class variance) on unimodal rides, which feeds D3.

### D2 — Span assembly constants: 30 s smooth, ≤ 30 s gap-merge, ≥ 60 s minimum effort
Samples above threshold form work spans; spans separated by ≤ 30 s of sub-threshold merge (surges/gear lulls don't split an effort); assembled spans under 60 s are discarded (sprint bursts are best-effort territory, not intervals). Constants documented in the response-adjacent swag docs; revisit on evidence.

### D3 — Bimodality gate → `no_distinct_efforts`
When Otsu's between-class variance falls below a fixed fraction of total variance (steady ride, one power mode), detection reports `200 {threshold_w: null, intervals: [], reason: "no_distinct_efforts"}` rather than manufacturing intervals from noise. An empty result is a legitimate coaching answer ("that ride was genuinely steady").

### D4 — Response carries everything needed to audit a boundary
`{threshold_w, intervals: [{n, start_s, end_s, duration_s, avg_w, max_w, kj}], rests: [{after_n, duration_s, avg_w}], summary: {count, work_total_s, mean_effort_s, mean_effort_w}}` — watts int, kJ `Round1`, boundary-only. Sentinels mirror the sibling endpoints: `workout_not_found` / `streams_not_found` / `power_stream_missing`.

### D5 — MCP tool returns the full body
`detect_intervals` (read tier) forwards verbatim with no summary-only split: unlike raw series, a detected-interval list is compact, structured reasoning input — exactly what the MCP surface is for.

### D6 — Dashboard: a table, not a chart
The workout-detail page gains a "Detected intervals" table (n, duration, avg W, rest after) next to the Garmin splits table, rendered only when detection returns ≥ 1 interval; the `no_distinct_efforts` case and missing power render nothing. Painting detection boundaries onto a power chart is deferred with the per-workout chart itself.

## Risks / Trade-offs

- **Otsu on genuinely trimodal rides** (warmup + tempo + VO₂ blocks) picks one split and may lump tiers — accepted for v1; the per-interval `avg_w` still separates them for the reader, and the threshold is visible.
- **30 s smoothing blurs very short recoveries** (<30 s micro-intervals like 30/15s merge into one effort) — accepted: the merged block's avg/duration still tells the story; micro-interval fidelity would need different constants (deferred with tunability).
- **Constants are opinions** — mitigated by reporting the threshold and keeping the algorithm pure (one function to re-run when second-guessing).

## Migration Plan

Additive: one endpoint, one tool, one table. No migration. Rollback = revert registrations.

## Open Questions

- Should rests carry their own avg power always, or only when meaningful (> 30 s)? (v1: always — it's already computed.)
- Painting detected boundaries on a future per-workout power chart (deferred with the chart).
