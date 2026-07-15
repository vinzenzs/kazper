# Intensity distribution

How your training time splits across heart-rate zones — the polarized/pyramidal check.

## What's computed

Over a window of completed workouts (from Garmin's measured `secs_in_zone_*`):

- **Per-zone shares** (Z1–Z5, % of zone time), collapsed into three bands:
  **low** (Z1–Z2), **moderate** (Z3), **high** (Z4–Z5).
- A **classification** from fixed thresholds (low ≥ 75 % and high share > moderate →
  *polarized*; low ≥ 75 % otherwise → *pyramidal*; moderate-dominant → *threshold*; anything
  else → *mixed*; not enough data → null). The bands are always returned so the label is
  auditable.
- A **weekly trend** (Monday-start, only weeks that contain a completed workout).
- A **sessions-by-training-focus** axis — declared intent beside measured execution — plus an
  `unclassified_focus_count`.

## Honesty rules

Workouts with no zone data at all (strength, pool sessions without HR) are **excluded from the
shares but counted** (`missing_zone_data_count`) so they can't dilute the split. Zero zone time
omits the share rather than reporting 0 %.

## Reading it

Endurance orthodoxy wants most time low: polarized (~80 % low, meaningful high, little middle)
or pyramidal (low > moderate > high). A *threshold*-classified block — living in Z3 — is the
classic "no easy days, no hard days" pattern worth a coach conversation.
