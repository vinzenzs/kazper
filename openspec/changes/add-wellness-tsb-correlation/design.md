## Context

`wellness-diary` (shipped 2026-07-14) stores per-date 1–5 scores; `performance-management` serves the daily PMC series. Correlating them was an explicit deferral in the diary's design ("correlation happens in the coach's reasoning, not in SQL, for v1") — this graduates it to SQL-adjacent arithmetic while keeping interpretation with the coach.

## Goals / Non-Goals

**Goals:** one honest number per wellness field per window; hard minimum-N gating so early sparse data can't produce confident noise.

**Non-Goals:** lagged/causal analysis, Garmin-vitals correlation (v2), multi-metric matrices in one call (one `metric` per request keeps the response readable), dashboard charting.

## Decisions

### D1 — Spearman rank correlation
Wellness scores are ordinal 1–5; Pearson assumes interval scales. Spearman over (score, metric) day-pairs, standard tie-ranking. Pure function, hand-computable fixtures.

### D2 — Pairing is same-day, by the athlete's calendar date
A wellness entry pairs with the PMC row of the same local date (the PMC's tz bucketing). Days lacking either side simply don't pair — no interpolation, no carry-forward.

### D3 — Minimum 14 pairs per field, else `insufficient_pairs`
Below n=14, rho on a 5-level ordinal is noise. Per-field gating (soreness may have 30 entries while motivation has 5); gated fields report `{n, reason}` — visible progress toward usefulness rather than absence.

### D4 — One `metric` per request: `tsb` (default) | `ctl` | `ramp_rate`
TSB is the hypothesis people actually hold ("form should feel like something"); ctl/ramp cover the load-side questions. One metric per call keeps the shape flat; the agent can issue three calls when it wants the matrix.

### D5 — Composition via a narrow interface
`internal/wellness` consumes a `PMCSeries(from, to, tz)` interface implemented by the pmc service, injected at wiring (the FK-validation cross-injection precedent) — no package cycle, no duplicate EWMA.

### D6 — Response
`{metric, from, to, fields: {fatigue: {n, rho?|reason}, soreness: …, stress: …, mood: …, motivation: …}}`, rho `Round2` at the boundary. PMC error vocabulary for range/tz; `400 metric_invalid` otherwise.

## Risks / Trade-offs

- **Confounding is rampant** (illness, life stress) — the endpoint reports association only; the tool description says so explicitly.
- **rho on 5-level data saturates** — accepted; direction + rough magnitude is the coaching signal, not the third decimal.

## Migration Plan

Additive; no migration. Rollback = revert registrations.

## Open Questions

- Lag-k variant ("does today's ramp predict tomorrow's fatigue?") — v2 with the Garmin-vitals extension.
