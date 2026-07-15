# Units, rounding & honesty rules

Understanding these four house rules explains a lot of Kazper's behavior.

## Unit isolation

Different measurement families never share a struct or a total. Hydration is **ml**; nutrition
is **kcal + g + mg**; workout fuel carries **carbs g / sodium mg / caffeine mg / optional ml**;
power is **W**, pace **sec/km** (swim **sec/100 m**). The daily nutrition summary deliberately
has no `total_ml`, and the hydration summary has no kcal. This prevents the classic footgun of
"add everything up" producing a number with no meaning.

## Rounding at the boundary

Values are stored and computed at **full precision** and rounded only when serialized — most to
1 decimal, ratios like IF to 2, EF to 3. That keeps status math (am I over/under goal?) honest
at borderline values: you'll never be "under" on a number that only looks under because it was
rounded before comparison.

## Missing means missing

- A workout with no TSS contributes nothing to the PMC and is **counted** (`missing_tss_count`)
  rather than treated as zero load.
- Heart-rate dropouts are excluded from HR means; if fewer than 80 % of samples are valid, the
  HR-derived metrics are NULL rather than pretending.
- Analytics that can't be computed return `200` with a machine-readable reason
  (`insufficient_points`, `no_tiered_data`, `weight_missing`, `no_distinct_efforts`…) — an
  empty result is an answer, not an error.

## Advisory vs configured

Data-derived estimates — critical power, the power profile, W′ balance, sweat rate — are
**advisory**: they inform a conversation, they never write your configuration. Your configured
thresholds (FTP, LTHR, max HR, threshold paces) are deliberate records with their own history;
changing them is always an explicit, confirmed act — after which derived TSS can be recomputed
against the new values on request.
