# Streams & execution metrics

The garmin-bridge posts each activity's **1 Hz sample streams** (power, speed, heart rate) and
Kazper stores them raw — gap-filled zeros included, interpretation happens at compute time.
Streams are the substrate for most power analytics; they can be re-processed any time via the
recompute endpoint (which is also how new analytics backfill history).

## Per-ride execution metrics

Derived once at ingest and stored on the workout:

### NP — Normalized Power

The power your ride *felt* like physiologically: the fourth-root of the mean fourth power of
the 30-second rolling average. Surges cost disproportionately; NP captures that where average
power hides it.

### VI — Variability Index

$$ VI = NP / \bar{P} $$

How smooth the ride was. ~1.00–1.05 is steady (TT, trainer); high VI marks stochastic racing or
group surges.

### EF — Efficiency Factor

$$ EF = NP / \overline{HR} \quad (\text{or mean speed} / \overline{HR} \text{ without power}) $$

Output per heartbeat. Tracked across similar aerobic sessions, a rising EF is aerobic progress.

### Aerobic decoupling (Pw:Hr)

Split the ride in halves; compare output-per-heartbeat between them:

$$ \text{decoupling} = \frac{r_1 - r_2}{r_1} \times 100\,\% $$

Under ~5 % on a long steady ride is the classic "aerobically coupled" marker; large positive
drift means the engine faded (fitness, heat, fueling).

**HR honesty:** zero samples (strap dropouts) are excluded from all HR means; below 80 % valid
HR coverage the HR-derived metrics are NULL rather than misleading.

## Best efforts (mean-maximal ladder)

For every ride with power (and runs/swims with speed), Kazper stores the best rolling average at
standard durations — 5 s, 15 s, 30 s, 1 m, 5 m, 10 m, 20 m, 30 m, 60 m. The windowed maximum per
duration across rides is your **power curve**, and it feeds
[critical power](critical-power.md), the [power profile](power-profile.md), and
[durability](durability.md).

!!! note "Sport scoping"
    Best-effort aggregates are strictly sport-scoped: running power (Garmin running dynamics)
    never enters the *bike* curve, and bike speeds never enter run/swim pace curves. This was a
    real bug fixed on 2026-07-15 — if an old screenshot shows a 540 W "5-minute best", that was
    a run bleeding through.
