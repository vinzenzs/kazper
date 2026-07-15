# Planned vs actual CTL

Your season plan (macrocycle) declares per-phase **weekly TSS targets**. This view simulates the
CTL curve those targets *imply* and lays it beside your measured CTL — answering "am I building
toward the race as planned, or two weeks behind the ramp?"

## How the target curve is built

- Each day inside a phase gets `target_weekly_tss / 7` as synthetic daily load.
- The same 42-day EWMA as the real PMC folds it forward — the two curves are mathematically
  comparable.
- The simulation is **seeded from your measured CTL on the macrocycle's start date** — the plan
  ramps a real athlete, not a blank one.
- Days between phases, or in a phase with no declared target, simulate at **0** and are flagged
  `target_declared: false` — an undeclared block *means* decay for the declared plan; the chart
  mutes those spans.

## What you get

Per day: `target_ctl`, your `actual_ctl` (up to today), and the `delta`. Plus a summary:

| Field | Question it answers |
|---|---|
| `current_delta` | how far off-plan am I right now? |
| `delta_trend_14d` | converging or diverging over the last two weeks? |
| `projected_end_ctl_planned` | where the plan lands if followed throughout |
| `projected_end_ctl_current` | where I land following the plan *from here* |

A macrocycle whose phases declare no targets returns `trajectory: null` with
`reason: "targets_missing"` — no synthetic season is invented.

!!! note
    Weekly targets flattened to daily sevenths under-specify hard/easy structure — fine for CTL
    (42-day horizon) which is why ATL/TSB targets are deliberately not simulated.
