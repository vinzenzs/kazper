# Tasks

## 1. Race weather modes

- [ ] 1.1 Race-location geocode resolution (weather client, cached; `location_ungeocodable`) + forecast-range gate (`forecast_out_of_range`)
- [ ] 1.2 Pacing plan `weather=true`: race-level heat block + per-leg `heat_adjusted` siblings (originals untouched); byte-identical-without-flag regression test
- [ ] 1.3 Fueling plan `weather=true`: bounded fluid/sodium multiplier over sweat-rate baseline (flag propagation); carbs untouched; byte-identical-without-flag regression
- [ ] 1.4 Integration tests (fake weather backend): annotated plans, all three degradation reasons, default-flag propagation
- [ ] 1.5 `task swag`

## 2. Heat analytics

- [ ] 2.1 Bucketed aggregation + Spearman (reuse the tie-ranked implementation) over outdoor completed workouts; `assumed_outdoor` tally; unit tests (bucket boundaries, gate, baseline-relative output)
- [ ] 2.2 `GET /workouts/heat-analytics` handler + integration tests (gradient fixture, thin-exposure gating, indoor exclusion, range 400s, read-only)

## 3. MCP

- [ ] 3.1 `weather` arg on `plan_race_pacing` + the race-fueling tool; new `heat_analytics` tool (confound caveat); golden regen + registry/integration green

## 4. Verification

- [ ] 4.1 `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 4.2 Live: weather-mode pacing plan for the actual A-race (in range or verify the degradation), heat-analytics over the season vs gut feel
