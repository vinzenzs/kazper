# TSS — Training Stress Score

TSS quantifies one workout's training load: **100 points ≈ one hour at threshold**. It is the
input currency of the [PMC](pmc.md).

## How Kazper derives it

Every completed workout gets TSS from the best available source, in strict precedence:

| Priority | Source | Formula | Needs |
|---|---|---|---|
| 1 | **explicit** | Garmin's own value, or one you set manually | — |
| 2 | **power** | \( \text{TSS} = h \cdot IF^2 \cdot 100 \), with \( IF = NP / FTP \) | FTP |
| 3 | **pace** | run rTSS: \( h \cdot IF^2 \cdot 100 \) with \( IF = \frac{\text{threshold pace}}{\text{actual pace}} \); swim sTSS uses \( IF^3 \) vs CSS | threshold pace / CSS |
| 4 | **hr** | HR-based estimate from \( \overline{HR} / LTHR \) (any sport) | LTHR |
| — | none | left NULL — never guessed | |

Every stored TSS carries its **`tss_source`** (`garmin`, `manual`, `power`, `pace`, `hr`) so you
always know how trustworthy a number is. Guard rails: only completed workouts derive; an
implausible \( IF > 2.5 \) skips derivation; missing thresholds fail *open* (NULL, counted as
missing downstream).

## Editing and recomputing

- Setting TSS by hand marks it `manual`; clearing it clears both value and source.
- Measured values (`garmin`, `manual`) are **immutable** to recomputation.
- After changing thresholds, `recompute-tss` re-derives all NULL/computed rows against the new
  config — measured rows stay untouched. The response tells you how many changed and by which
  source.

!!! warning "Threshold quality is TSS quality"
    A stale FTP skews every power-TSS and therefore your whole PMC. Cross-check with the
    [critical power model](../power/critical-power.md), and recompute after confirming changes.
