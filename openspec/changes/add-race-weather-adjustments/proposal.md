## Why

The high-stakes half of the weather arc: race day. The race-fueling spec explicitly reserved weather as "adjustments the agent layers on top" — this change gives that layer numbers. And the heat model's constants need their promised evidence pass ("refine on evidence", user direction): the history already stores per-workout temperature/humidity beside EF/decoupling, so the degradation-vs-heat relationship is measurable today.

## What Changes

- **Race-day heat on the pacing plan**: `GET /races/{id}/pacing-plan` gains an optional `weather=true` mode — geocode the race's `location` (cached on read, `location_ungeocodable` degradation), forecast the race window when within range (~16 days; else `forecast_out_of_range`), compute the race heat load + acclimatization (same shipped model), and annotate **each leg's band with its heat-adjusted counterpart** (original bands always retained — the adjustment is a second opinion, not a replacement).
- **Race-day fluids**: the race fueling plan's fluid/sodium derivation gains the same optional weather mode — sweat-rate-derived fluid scaled by the race heat load (multiplier echoed), flagged default otherwise.
- **Heat analytics over history**: `GET /workouts/heat-analytics?from=&to=&tz=` — outdoor completed workouts bucketed by session heat index (<20 / 20–25 / 25–30 / >30 °C): per bucket, session count, mean EF, mean decoupling, mean pace-or-power vs the athlete's same-window baseline; plus Spearman correlations (EF vs heat index, decoupling vs heat index, minimum-pairs gate — the wellness-correlation shape). This is the evidence stream for refining the v1 adjustment constants.
- New MCP tool `heat_analytics` (read); the pacing/fueling weather modes ride the existing `plan_race_pacing` / race-fueling tools as an optional arg.
- Compute-on-read, no migration.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `race-pacing-plan`: 1 ADDED requirement — the optional weather-adjusted band annotations.
- `race-fueling-plan`: 1 ADDED requirement — the optional heat-scaled fluid derivation.
- `heat-adjustment`: 1 ADDED requirement — the heat-analytics read and its MCP tool.

## Impact

- **Code:** race geocode resolution (weather client), heat annotations in `internal/racepacing` + fluid scaling in the race-fueling path, `internal/heat/` analytics aggregation; MCP golden (arg additions + new tool); `task swag`.
- **Out of scope (deferred):** automatic constant refitting from the analytics (a human reads the evidence and proposes new constants — a future change), multi-day stage races, historical race-day weather retrospectives (archive endpoint exists; add on demand).
