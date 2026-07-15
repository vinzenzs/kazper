# Power profile

Ranks your best efforts against Coggan's published power-profile tables — the answer to "is my
5-minute power actually good, and which duration is my limiter?"

## The four anchors

| Anchor | Energy system |
|---|---|
| 5 s | neuromuscular / sprint |
| 1 min | anaerobic capacity |
| 5 min | VO₂max |
| 20 min best | functional threshold proxy (used as-is, no 0.95 haircut) |

For each anchor in the window: raw **watts**, **W/kg**, the Coggan **category** band (Untrained
→ World Class), and an interpolated **percentile** (an estimate — the category is the
authoritative output). Each carries the workout it came from.

## Inputs and their honesty

- **Weight:** the `weight_kg` parameter if given, else your most recent stored body weight
  (source echoed); no weight at all → `weight_data_missing`, because W/kg is the whole point.
- **Sex** selects the table (male default).
- Anchors with no best effort in the window are listed as missing, not zeroed — sprint data is
  legitimately rare in base season.

## Phenotype

When all four anchors rank, the shape of your profile yields a rider **phenotype** — sprinter,
time-trialist, climber, or all-rounder. Uneven anchors are the interesting part: a profile that
ranks *Very Good* at 5 min but *Fair* at 5 s says train (or accept) the sprint.

!!! note
    Bike power only, and strictly bike-sourced — running power never enters these tables.
