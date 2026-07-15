# Meals, products & daily summary

## Logging

Meals log in two modes: **product-referenced** (a known product + quantity in grams — nutrients
derive from the product's per-100 g values) or **freeform** (you state the nutrients — the
"~600 kcal pasta at a friend's place" case). Products come from barcode scans via Open Food
Facts, Cookidoo recipe imports, or manual creation. Meals can link to the workout they fueled.

A freeform guess can later be **corrected to a product** in place — nutrients re-derive, but the
entry keeps its identity, timestamp, notes, and workout link, and the day's summary follows
automatically.

## Goals & overrides

A singleton goal set (kcal, macros) plus per-date **overrides** for special days (race day,
recovery day). The effective goal for a date = override if present, else the base goal.

## The daily summary

Per day (or date range): totals per nutrient against the effective goal with a status per
tracked value. Totals are computed at full precision and rounded once at the boundary — border
judgments ("49.7 g is under 50") are made on the real number, not the rounded one.

!!! note "What the daily summary is *not*"
    It deliberately excludes hydration ml and in-session workout fuel — those live in their own
    summaries with their own units. See [Units & honesty rules](../conventions.md).
