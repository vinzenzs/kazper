## Why

The deferred second half of the wellness diary: subjective scores are only fully useful held against objective load — "does your reported fatigue actually track TSB, or does soreness lag ramp weeks?" Deliberately parked until the diary existed; proposing now so it's ready when the entries accumulate (the endpoint's own minimum-N gate keeps it honest until then).

## What Changes

- `GET /api/v1/wellness/correlation?from=&to=&tz=&metric=` — pairs each wellness field's daily entries with that day's PMC value (`metric`: `tsb` default | `ctl` | `ramp_rate`) and returns per-field Spearman rank correlation `{n, rho}`; fields with fewer than 14 paired days return `{n, reason: "insufficient_pairs"}` with no rho. Range cap 92 days–400 days? — the PMC range rules apply (400-day cap).
- New `wellness_correlation` MCP tool (read tier, one GET, verbatim).
- Compute-on-read (wellness window + PMC series joined by date), persists nothing, no migration. No dashboard surface in v1 — this is a coach-conversation read.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `wellness-diary`: 2 ADDED requirements — the correlation endpoint and the MCP tool.

## Impact

- **Code:** `internal/wellness/` gains a pure Spearman + pairing module and a handler consuming a narrow PMC-series interface (wired in `httpserver.Run()` — wellness must not import pmc's package internals).
- **API/MCP:** one GET, one read tool, golden additive, `task swag`.
- **Out of scope:** causal/lagged analysis (rho at lag k), correlation against sleep/HRV Garmin vitals (a natural v2 once this shape settles), any dashboard panel.
