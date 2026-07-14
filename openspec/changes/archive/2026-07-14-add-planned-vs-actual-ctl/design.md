## Context

`internal/pmc` computes CTL/ATL/TSB as a pure EWMA (Coggan 42/7) over per-local-day completed TSS, warmed from the earliest workout so values are window-independent. `training-phases` rows optionally link to a macrocycle with `macrocycle_ordinal` and carry declared `target_weekly_tss`/`target_weekly_hours` (plan, never measured). The macrocycle itself has `[start_date, end_date]` and anchors the A-race. Nothing compares the declared ramp to the measured curve.

## Goals / Non-Goals

**Goals:**
- One endpoint answering "on plan or off, by how much, and where does this trajectory land at macrocycle end."
- Target curve derived exclusively from declared plan values — no inference of what the athlete "should" do.
- Reuse the existing EWMA verbatim so target and actual are mathematically comparable.

**Non-Goals:**
- Hours-based targets (TSS is the PMC's currency; mixing units here would need a conversion opinion).
- ATL/TSB targets (a weekly-TSS plan under-specifies daily distribution; CTL is robust to it, ATL is not).
- Re-planning/catch-up suggestions — the endpoint reports divergence, the coach decides what to do with it.
- Persisting anything.

## Decisions

### D1 — Lands in `internal/pmc`, reusing the EWMA
The target curve is the same recursion fed synthetic inputs: for each day in `[macrocycle.start_date, macrocycle.end_date]`, target daily TSS = the containing phase's `target_weekly_tss / 7` (0 for days in no phase or in a phase with a null target — but see D4). Same 42-day constant, same rounding discipline (full precision internally, `Round1` at boundary). Macrocycle + phases arrive via a narrow read interface injected in `httpserver.Run()` (the `workoutfueling` multi-repo pattern), keeping `pmc` free of package cycles.

### D2 — Seeded from actual CTL at macrocycle start
The target simulation starts from the *measured* CTL on `start_date` (the existing PMC warm-up provides it), not from zero — the plan ramps a real athlete, not a blank one. This makes the two curves directly comparable from day one and means a mid-season macrocycle gets an honest baseline. `seed_ctl` and `start_date` are echoed in the response.

### D3 — Default macrocycle = the active one, by the public-feed rule
`macrocycle_id` optional; omitted → the macrocycle whose `[start_date, end_date]` contains today, tie-broken by latest `start_date` (the established `public-race-feed` resolution). None active or an unknown id → `404 macrocycle_not_found` (a specific macrocycle is the subject of this read; unlike the feed, there is no meaningful null-object response).

### D4 — Declared-plan honesty for gaps and missing targets
A phase with `target_weekly_tss = NULL` and days between phases both simulate at 0 target TSS — CTL then decays, which is what an undeclared block *means* for the declared plan; these spans are flagged per-day (`target_declared: false`) so the UI/coach can render them differently from a deliberate taper. If **no** phase in the macrocycle declares a target, the response is `200` with `trajectory: null`, `reason: "targets_missing"` (the CP-model null posture) — simulating an all-zero season would be confidently meaningless.

### D5 — Response shape
`{macrocycle: {id, name, start_date, end_date}, seed_ctl, series: [{date, target_ctl, target_declared, actual_ctl?, delta?}], summary: {current_delta, delta_trend_14d, projected_end_ctl_planned, projected_end_ctl_current}}`. `actual_ctl`/`delta` present only up to today; future days carry the target only. `projected_end_ctl_current` extends the EWMA from today's actual CTL using the remaining *planned* daily targets — "if you follow the plan from here, where do you land" (vs `_planned`: "where you'd land had you followed it throughout"). Missing-TSS honesty is inherited from the PMC (`missing_tss_count` surfaced per the existing convention). `tz` param and error vocabulary match `GET /performance/pmc`.

### D6 — MCP `pmc_target_trajectory`, read tier
One GET, verbatim body; args `macrocycle_id` (optional) + `tz`. Description distinguishes it from `pmc_series` (measured only) and names the active-macrocycle default.

### D7 — Dashboard: overlay on the existing PMC panel
A dashed target-CTL line over the existing CTL series plus a compact "on plan / +N behind" readout; undeclared spans (D4) rendered muted. Hidden entirely when no active macrocycle or `targets_missing` — the measured PMC panel is unaffected. No new panel.

## Risks / Trade-offs

- **Weekly targets under-specify daily load** — daily = weekly/7 flattens intent (a 3-hard-day week ≠ flat). CTL's 42-day horizon makes this negligible for the trajectory; ATL/TSB targets are excluded for exactly this reason (non-goal).
- **Seed inherits PMC missing-TSS gaps** — if early history lacks TSS, `seed_ctl` is understated; mitigated by surfacing the existing missing-TSS counters rather than imputing.
- **Two projections may confuse** — mitigated by naming (`_planned` vs `_current`) and the tool description; the summary leads with `current_delta`, the number that drives decisions.

## Migration Plan

Additive: one endpoint, one tool, one overlay. No migration. Rollback = revert registrations.

## Open Questions

- Should `delta_trend_14d` be a slope, or just `delta_now − delta_14d_ago`? (v1: the simple difference — readable, sufficient for "diverging or converging".)
- Per-sport target trajectories once the per-sport PMC split lands (both deferred; they compose naturally).
