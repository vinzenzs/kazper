# Interval detection & quadrant analysis

Two per-ride views over the stored power stream.

## Detected intervals

For unstructured rides (no template, no lap button), Kazper finds the work intervals
deterministically and parameter-free:

1. Smooth power with a 30-second rolling mean.
2. Derive the ride's own work/rest threshold from its power distribution (Otsu's method — the
   ride defines its own contrast, so tempo intervals in an easy ride still register).
3. Assemble efforts: sub-threshold dips ≤ 30 s don't split an effort; efforts under 60 s are
   discarded (that's best-effort territory, not intervals).

You get each interval's duration, average/max power, and kJ, the rests between them, the derived
threshold (so every boundary is auditable), and a summary. A genuinely steady ride returns
`no_distinct_efforts` — "that ride had no interval structure" is a real answer.

## Quadrant analysis

*How* you produce power: each pedaling sample becomes a point of **pedal force vs pedal speed**:

$$ CPV = \frac{\text{cadence} \cdot \text{crank} \cdot 2\pi}{60}, \qquad AEPF = \frac{P}{CPV} $$

split into four quadrants around your CP at a reference cadence (both supplied explicitly;
crank length defaults to 172.5 mm):

| | high force | low force |
|---|---|---|
| **high velocity** | Q I — sprinting, big-gear power | Q IV — fast spinning |
| **low velocity** | Q II — grinding, climbing, MTB-style | Q III — easy riding |

Coasting and dropouts are excluded (reported as `excluded_s`), not diluted into the shares. Use
it to check race demands vs training: if your goal event lives in Q II and your training in
Q III, that's a specific, fixable mismatch.

!!! note
    Quadrant analysis needs a **cadence stream**, which syncs began storing when the feature
    shipped — older rides report `cadence_stream_missing` until re-synced.
