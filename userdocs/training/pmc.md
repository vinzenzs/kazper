# PMC — fitness, fatigue & form

The Performance Management Chart (Coggan/TrainingPeaks model) turns your daily TSS series into
three curves:

| Value | Name | Meaning | Time constant |
|---|---|---|---|
| **CTL** | Chronic Training Load — *fitness* | the slow average of your load | 42 days |
| **ATL** | Acute Training Load — *fatigue* | the fast average | 7 days |
| **TSB** | Training Stress Balance — *form* | **yesterday's** CTL − ATL | — |

Each is an exponentially-weighted moving average over per-day summed TSS of **completed**
workouts (planned ones never count):

$$ CTL_t = CTL_{t-1} + \frac{TSS_t - CTL_{t-1}}{42}, \qquad ATL_t \text{ likewise with } 7 $$

TSB uses *yesterday's* values — form is what you brought into today, not what today's session
does to you.

## Reading it

- **TSB well negative** → carrying fatigue (productive in a build, risky before racing).
- **TSB near / above zero** → fresh; classic race-day targets sit slightly positive.
- **CTL trend** is the season story; single days matter little.

## Kazper specifics

- **Window-independent warm-up:** the EWMA is seeded from your *earliest workout ever*, so the
  values for a given date don't change when you widen the chart. `seed_date` is reported.
- **Ramp rate** = CTL today − CTL 7 days ago, per day; weeks (Monday-start) exceeding a ramp of
  **8 CTL/week** are flagged as `ramp_alerts` — the classic overreaching ceiling.
- **Missing TSS is surfaced, never imputed:** per-day `missing_tss_count` plus a window-level
  list of the workouts lacking TSS. If those counts are non-trivial, fix the
  [TSS sources](tss.md) before trusting the curves.
- **Per-sport view:** `sport=bike|run|swim` filters the whole computation to that discipline
  (its own seed, ramp alerts, missing counts). The combined curve is *not* the sum of the
  filtered ones — EWMAs don't add.
