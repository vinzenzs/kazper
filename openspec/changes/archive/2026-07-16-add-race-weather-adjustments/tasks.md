# Tasks

## 1. Race weather modes

- [x] 1.1 Race-location geocode resolution (weather client, cached; `location_ungeocodable`) + forecast-range gate (`forecast_out_of_range`)
      _The horizon gate is checked BEFORE the geocode, so a distant race costs no lookups at all. `location_ungeocodable` covers both an empty location text and a lookup that ran and matched nothing — the two cases the athlete fixes by editing the race; a geocoder that couldn't run is `weather_unavailable`, which is ours to fix (the weather client's ok/empty distinction earning its keep)._
- [x] 1.2 Pacing plan `weather=true`: race-level heat block + per-leg `heat_adjusted` siblings (originals untouched); byte-identical-without-flag regression test
      _Each leg is adjusted on ITS OWN duration — a 5-hour bike and a 4-hour run off one race sit in different duration bands — while the race-level percentage uses the total. Pace bands are DIVIDED (backing off = more sec/km) and TSS follows the adjusted IF quadratically. A leg with no computable band (an unset threshold degraded it) gets no sibling rather than an invented one._
- [x] 1.3 Fueling plan `weather=true`: bounded fluid/sodium multiplier over sweat-rate baseline (flag propagation); carbs untouched; byte-identical-without-flag regression
      _The 1000 ml/hr absorption cap is re-applied AFTER the multiplier — heat cannot make a gut drink faster — and the `defaulted` flags are deliberately not cleared, so a scaled generic 600 ml/hr still says it's a default (asserted by comparing rationales)._
- [x] 1.4 Integration tests (fake weather backend): annotated plans, all three degradation reasons, default-flag propagation
- [x] 1.5 `task swag`

## 2. Heat analytics

- [x] 2.1 Bucketed aggregation + Spearman (reuse the tie-ranked implementation) over outdoor completed workouts; `assumed_outdoor` tally; unit tests (bucket boundaries, gate, baseline-relative output)
      _"Reuse" taken literally: the tie-aware Spearman was unexported inside `wellness`, so it was **extracted to a new `internal/stats`** (with its test) and wellness now delegates to it. A second copy would have been free to drift from the first. Bucket means are nullable — a bucket whose sessions carry no EF reports the count with a null mean, never a 0 that reads as "terrible" rather than "not measured"._
- [x] 2.2 `GET /workouts/heat-analytics` handler + integration tests (gradient fixture, thin-exposure gating, indoor exclusion, range 400s, read-only)
      _The fixture had to write EF/decoupling through `SetExecutionMetrics`: the workouts upsert deliberately refuses stream-derived metrics (persist-activity-streams), which the first draft of the test discovered the hard way._

## 3. MCP

- [x] 3.1 `weather` arg on `plan_race_pacing` + the race-fueling tool; new `heat_analytics` tool (confound caveat); golden regen + registry/integration green
      _Two golden entries are **updates** (the opt-in arg), one is additive — and a test pins that the flag is sent only when asked, so the deterministic default can't drift. The `heat_analytics` description leads with the duration confound and states the agent cannot refit the constants._

## 4. Verification

- [x] 4.1 `task vet` + full suite green
- [ ] 4.2 Live: weather-mode pacing plan for the actual A-race (in range or verify the degradation), heat-analytics over the season vs gut feel
      _(operator step — needs the real A-race and live Open-Meteo; not runnable in-session. Expect `forecast_out_of_range` until ~16 days out — that IS the correct answer, not a bug.)_
