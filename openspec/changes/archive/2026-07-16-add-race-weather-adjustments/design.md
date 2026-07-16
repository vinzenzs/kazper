## Context

`add-heat-adjusted-training` ships the weather client, heat-load model, and acclimatization. Races carry a free-text `location` and a date; the pacing plan computes per-leg bands from effective thresholds; the fueling plan derives fluids from measured sweat rate (flagged default otherwise). History holds outdoor completed workouts with temperature/humidity beside EF/decoupling/pace — untouched evidence.

## Goals / Non-Goals

**Goals:** race-day numbers where the stakes justify them, opt-in and side-by-side (never replacing the cool-weather plan); an evidence read that turns "refine on evidence" from intention into data.

**Non-Goals:** auto-refitting constants (humans read evidence; a later change updates the table); storing weather on races; stage races; making weather mode the default (a race two weeks out has no reliable forecast — opt-in keeps the default plan deterministic).

## Decisions

### D1 — Opt-in `weather=true`, annotations beside originals
The deterministic cool-weather plan stays byte-identical without the flag (existing consumers unchanged, spec promise kept). With it, each leg gains `heat_adjusted: {band, if, tss}` beside the original, plus a race-level `heat: {load_c, acclimatization, source}` block. Both visible → the coach can present "plan A cool / plan B hot" honestly.

### D2 — Race location via geocoding, resolved at read
The race's `location` text geocodes through the shipped client (cached); no coordinates are stored on races (no migration, text stays the authored truth). Empty/unmatchable location → `location_ungeocodable` degradation with the unadjusted plan intact. Race beyond forecast range → `forecast_out_of_range` likewise — callers always get the base plan.

### D3 — Fluid scaling is a bounded multiplier on measured sweat rate
Heat load bands map to a fluid multiplier (bounded ~1.0–1.5×) applied to the sweat-rate-derived ml/hr; the multiplier and its band are echoed. Without a measured sweat rate the existing flagged-default path scales the same way, flag intact — heat never manufactures precision the base didn't have.

### D4 — Analytics: buckets + correlations, the wellness-correlation shape
Fixed heat-index buckets (<20/20–25/25–30/>30 °C) over outdoor completed workouts in the window; per bucket the mean EF, decoupling, and output-vs-baseline; Spearman rho per metric with an n ≥ 10-pairs gate (`insufficient_pairs`). Null-environment workouts count with an `assumed_outdoor` tally so the caveat is visible. This is deliberately descriptive, not a model fit — the human refines the adjustment table from it.

## Risks / Trade-offs

- **Forecast volatility inside 16 days** — mitigated by opt-in + the forecast timestamp echoed; race-week re-reads are the coach's rhythm anyway.
- **Geocoding a vague location** ("local crit") mismatches — the resolved place name is echoed; fix by editing the race's location text.
- **Analytics confounding** (hot days are also long days) — stated in the tool description; buckets report duration means alongside so the reader sees it.

## Migration Plan

None. Rollback = revert the flag handling + endpoint/tool.

## Open Questions

- Constant-refit cadence: revisit the adjustment table once heat-analytics has a season of buckets (explicitly a future, human-driven change).
