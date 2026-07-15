# Zones & training focus

## Zone boundaries

Your config carries HR-zone and power-zone maxima (power zones follow the Coggan %-of-FTP
model when derived from FTP). Completed Garmin activities arrive with **time-in-zone** already
measured (`secs_in_zone_1..5`), which feeds the
[intensity distribution](../training/intensity-distribution.md).

## Zone-referenced targets

Structured workout templates can prescribe steps by **zone reference** ("Z2", "power zone 4")
instead of absolute numbers. When a plan materializes to your Garmin calendar, Kazper resolves
those references against your current config — power zones for bike steps, HR cross-sport, pace
from threshold pace / CSS — so the watch shows real targets. The same resolution feeds
[step compliance](../plans/adherence-compliance.md) when scoring how you executed.

## Training focus

Each workout can carry a declared **training focus** — one of the 7-band German
*Trainingsbereiche* model (`recovery`, `basic_endurance_1`, `basic_endurance_2`, `development`,
`competition_specific`, `peak`, `strength_endurance`). It is *declared intent*, never derived
from the data — the intensity distribution report shows sessions-by-focus beside the measured
zone shares precisely so intent and execution can disagree visibly.
