## Context

`cp-model` (shipped 2026-07-14) fits CP2 over the windowed best-effort MAX with gates (`insufficient_points`, `span_too_narrow`) and stays deliberately uncoupled from athlete-config (D4). `threshold-history` records configured-FTP changes as a dated series. Nothing shows the *estimate's* movement over time.

## Goals / Non-Goals

**Goals:** a fit series over rolling windows, reusing the shipped fit verbatim; honest per-anchor degradation; the configured-vs-derived comparison composed by consumers, not the backend.

**Non-Goals:** persisting fits; auto-apply (D4); sub-weekly anchors; run/swim CS history (deferred with CS itself).

## Decisions

### D1 — Weekly Monday anchors over the range, each fitting its trailing `window_days`
Monday-start is the repo's week convention (PMC ramp, intensity trend). Each anchor runs the existing fit over `[anchor − window_days, anchor]`; `window_days` default 90 (the CP panel's habit), bounds [30, 365] → `400 window_days_invalid`. Range uses the 400-day cap + power-curve error vocabulary. Cost: one windowed-MAX query per anchor (≤ ~58 for a full-year read) — acceptable for a read this infrequent; no caching until proven needed.

### D2 — Null anchors carry their reason and stay in the series
An anchor whose window fails a gate contributes `{date, model: null, reason}` rather than being dropped — gaps are information (base season without long efforts), and charting needs the x-position to gap honestly.

### D3 — Comparison stays client-side (D4 upheld)
The dashboard overlays `/athlete-config/history` (already served) as a step line; the coach agent composes the two tools. The endpoint returns fit data only.

### D4 — Response
`{window_days, anchors: [{date, model: {cp_watts, w_prime_kj, r_squared, rmse_w} | null, reason?}]}` — same rounding as the single fit, boundary-only.

## Risks / Trade-offs

- **Adjacent anchors share most of their window** → the series is smoothed by construction; fine, the question is trend not week-to-week noise.
- **N queries per read** → bounded by the range cap; revisit with a single grouped query if it ever shows up in the request-duration histograms.

## Migration Plan

Additive; no migration. Rollback = revert registrations.

## Open Questions

- Chart W′ trend alongside CP once real data shows whether it's stable enough to be meaningful (returned already; charting deferred).
