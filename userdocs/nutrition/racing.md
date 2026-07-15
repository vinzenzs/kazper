# Race prep, fueling & pacing

Three race-facing calculators, all advisory, all built on your stored data.

## Carb-load math (race prep)

Stateless arithmetic for the final days: given your weight and a target loading rate
(g carbs/kg/day), it lays out per-day carb totals and how they compare to your normal goals —
the "yes, 600 g of carbs is the assignment, here's why" numbers.

## Race fueling plan

Per-race, per-leg fueling targets: carbs/hr, sodium/hr, fluids — the race-day protocol distilled
from what training rehearsals (workout-fuel logs) showed you can absorb. Races carry an optional
**A/B/C priority** (A = full taper + peak, B = mini-taper, C = train through) that the coach
weighs when advising around a race week.

## Race pacing plan

Per-leg intensity bands derived from your configured thresholds:

| Leg | Band source | Duration logic |
|---|---|---|
| Bike | % of FTP | shorter legs → higher %FTP band (e.g. < 45 min → 90–100 %, ≥ 3 h → 68–78 %) |
| Run | × threshold pace | multipliers widen with duration |
| Swim | × CSS | likewise |

Each leg reports its band, an implied IF, and an **estimated TSS** (swim uses the cubic sTSS
convention), plus race totals and a `tss_complete` flag when any leg couldn't be computed.
Per-leg **overrides** let you pin a target (survives leg edits); a missing threshold degrades
that leg to "targets omitted, reason given" — the plan never invents a band you have no
threshold for.

## The countdown page

The public "road to race" site shows your active macrocycle's A-race and days remaining —
non-PII by construction (name, date, countdown; nothing else leaves the system).
