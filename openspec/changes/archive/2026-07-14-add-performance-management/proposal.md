# Add a Performance Management Chart (CTL / ATL / TSB)

## Why

Kazper stores per-workout TSS and mirrors Garmin's acute/chronic load, but has no
Performance Management Chart — the TrainingPeaks centerpiece that turns daily TSS
into fitness (CTL), fatigue (ATL), and form (TSB), the numbers a coach actually
plans tapers and flags overreaching with.

## What Changes

- New capability `performance-management`: a pure compute-on-read daily PMC series
  derived from stored **completed** workouts' TSS. No new tables, no migration —
  same posture as `energy-availability` and `race-prep`.
- New endpoint `GET /performance/pmc?from=&to=&tz=` returning one entry per
  calendar day `{date, tss_total, ctl, atl, tsb, ramp_rate}` plus window-level
  `ramp_alerts` (Monday-start weeks where CTL rose faster than the safe
  threshold of 8 CTL/week) and missing-TSS honesty counters.
- Classic Coggan EWMA math: CTL = 42-day exponentially weighted moving average of
  daily TSS, ATL = 7-day EWMA, TSB(d) = CTL(d−1) − ATL(d−1). The EWMA is seeded
  at zero the day before the earliest completed workout and warmed up over the
  full stored history, so values at `from` need no client-supplied warm-up window.
- New MCP tool `pmc_series` mirroring the endpoint 1:1 via the shared
  `agenttools` registry (one HTTP call, verbatim forward, read tier).
- Coach-dashboard: the `/stats` surface gains a PMC chart — CTL line, ATL line,
  TSB area/bars around a zero baseline — over a selectable window, with
  ramp-flagged weeks highlighted.
- Sibling note: the in-flight `add-per-sport-tss` change fills TSS gaps for
  non-power sports; PMC works without it (missing TSS contributes 0 and is
  counted, not hidden) and gets more accurate once it lands. No structural
  dependency in either direction.

## Capabilities

### New Capabilities

- `performance-management` — compute-on-read Coggan PMC (CTL/ATL/TSB daily
  series + ramp-rate flags) over stored workout TSS.

### Modified Capabilities

- `coach-dashboard` — the `/stats` surface gains a PMC chart requirement.
- `mcp-server` — the tool surface gains the `pmc_series` read tool (the spec
  enumerates tools per REST surface).

## Impact

- **Specs:** new `performance-management` spec; ADDED requirements on
  `coach-dashboard` and `mcp-server`.
- **Code:** new `internal/pmc/` package (types / read-only repo / service /
  handlers, per the `effortanalytics` shape); wiring in
  `internal/httpserver/server.go`; new `internal/agenttools/registry_pmc.go`
  entry + regenerated MCP golden schema; PMC chart component in
  `apps/web/src/` on the `/stats` route + rebuilt `apps/web/dist`.
- **No migration.** Reads the existing `workouts` table only (completed rows,
  `tss` column). Read-only endpoint — never consumes an `Idempotency-Key`.
- **Docs:** `task swag` regenerates `docs/` for the new endpoint.
