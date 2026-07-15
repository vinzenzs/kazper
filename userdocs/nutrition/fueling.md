# Hydration & workout fuel

## Hydration

A plain ml log — water, bottles, whatever — with its own daily summary (ml only, by design).
Entries can link to a workout. Garmin's estimated per-activity sweat loss syncs into a separate
hydration-balance record for comparison.

## Workout fuel

In-session fueling is its own thing — gels, drink mix, salt tabs, caffeine — captured in the
units that matter mid-race:

| Field | Unit | Why it exists |
|---|---|---|
| `carbs_g` | g | carb-per-hour drives long-course performance |
| `sodium_mg` | mg | endurance targets typically 300–800 mg/hr |
| `potassium_mg` | mg | electrolyte completeness |
| `caffeine_mg` | mg | dosing strategy is per-mg, not per-cup |
| `quantity_ml` | ml | the fluid the fuel came in |

Every entry keeps its **name** ("SIS Beta Fuel", "flat cola") so race rehearsals record *what*
worked, not just numbers. The **workout fueling view** aggregates everything anchored to one
workout — fuel entries, linked hydration, linked meals — into per-hour rates for rehearsal
review.

## Sweat rate

The standard field test, computed on demand for a completed workout:

$$ \text{sweat loss (ml)} = (\text{kg}_{\text{pre}} - \text{kg}_{\text{post}}) \times 1000 + \text{fluid}_{\text{ml}}, \qquad \text{rate} = \frac{\text{loss}}{\text{hours}} $$

You supply pre/post weights explicitly (the daily weight log can't stand in for a
scale-before/scale-after test); fluid comes from the workout's linked hydration + fuel ml,
itemized, with an override for unlogged bottles. Implausible results (negative loss, > 5 L/hr)
are returned **with a warning** rather than hidden — the arithmetic is shown so the inputs can
be questioned.

## Supplements

A thin dated log for what's neither meal nor fuel — iron, vitamin D, magnesium, creatine. Name
required, dose + unit optional (paired), multiple entries per day, no macro fields on purpose.
Today's entries appear in the coach's daily context.
