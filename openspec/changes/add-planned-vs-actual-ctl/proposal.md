## Why

Deliberately left out of the PMC v1 and carried on the backlog since: the macrocycle declares intent (per-phase `target_weekly_tss`) and the PMC measures reality (CTL from completed TSS), but nothing holds them against each other. "Am I building toward the A-race as planned, or two weeks behind the ramp?" is currently the coach eyeballing two endpoints. Now that `performance-management`, macrocycle phase targets, and per-sport-honest TSS all coexist, the comparison is pure arithmetic over data Kazper already has.

## What Changes

- `GET /api/v1/performance/pmc/target-trajectory?macrocycle_id=&tz=` — simulates the target CTL curve implied by the macrocycle's phase `target_weekly_tss` values (daily TSS = weekly/7 through each phase, same Coggan 42-day EWMA, seeded from actual CTL at the macrocycle start) and returns it beside the actual CTL series to date, with per-day `delta` and a summary (current delta, trend over the last 14 days, projected CTL at macrocycle end on plan vs on current trajectory).
- `macrocycle_id` optional — defaults to the active macrocycle (the one containing today, latest-`start_date` tie-break: the `public-race-feed` resolution rule).
- Honest degradation: no active/known macrocycle → `404 macrocycle_not_found`; a macrocycle whose phases carry no `target_weekly_tss` → `200` with `trajectory: null`, `reason: "targets_missing"`; phase gaps simulate at 0 target TSS (a declared rest gap is a real plan).
- New `pmc_target_trajectory` MCP tool (read tier, one GET, verbatim).
- The `/stats` PMC panel gains a target-CTL overlay line + on/behind-plan readout when a macrocycle with targets is active.
- Compute-on-read, no migration; lands in `internal/pmc/`.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `performance-management`: 2 ADDED requirements — the target-trajectory endpoint and the MCP tool.
- `coach-dashboard`: 1 ADDED requirement — the PMC panel target overlay.

## Impact

- **Code:** `internal/pmc/` gains the target simulation (pure, reuses the existing EWMA) + handler; a narrow macrocycle/phases read interface wired in `httpserver.Run()`; `apps/web` PMC panel overlay + types.
- **API/MCP:** one GET, one read tool, golden regen additive, `task swag`.
- **Out of scope:** hours-based targets (`target_weekly_hours` — TSS is the load currency here), writing anything back to phases, ATL/TSB target simulation, mid-macrocycle re-planning suggestions ("increase week 6 to catch up" stays the coach's judgment).
