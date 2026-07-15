# Wellness diary & correlation

Objective recovery data says how your body *measured*; the wellness diary records how you
*felt*. Both matter, and they disagree in informative ways.

## The diary

One entry per day, collected conversationally (the coach asks; a morning check-in takes ten
seconds), all fields optional but at least one required:

| Field | Scale | Direction |
|---|---|---|
| `fatigue` | 1–5 | 1 = none → 5 = severe |
| `soreness` | 1–5 | 1 = none → 5 = severe |
| `stress` | 1–5 | 1 = none → 5 = severe |
| `mood` | 1–5 | 1 = low → 5 = high |
| `motivation` | 1–5 | 1 = low → 5 = high |
| `note` | text | anything ("left achilles tight") |

Partial entries are first-class — "just log soreness 4" is a normal day. Re-logging a day
replaces it. Today's entry appears in the coach's daily context right next to the Garmin
recovery block, so *"TSB says fresh, legs say otherwise"* is one glance.

## Correlation

Once the diary has data, Kazper can compute the **Spearman rank correlation** between each
wellness field and a PMC metric (TSB by default, or CTL / ramp rate) over a window — "does your
reported fatigue actually track your form?" Honesty gates apply: a field with fewer than
14 same-day pairs reports `insufficient_pairs` instead of a confident number, and results are
explicitly associations, not causes.

Use it to calibrate: if soreness correlates strongly with ramp rate, the coach can treat your
soreness reports as an early ramp alarm; if mood tracks nothing, mood is context, not signal.
