## ADDED Requirements

### Requirement: The fueling plan optionally scales fluids by race-day heat

`GET` of the race fueling plan SHALL accept an optional `weather=true` parameter applying the
shipped heat model to the race window (same geocoding/forecast resolution and degradation
reasons as the pacing plan's weather mode): fluid and sodium derivations gain a bounded
heat multiplier (~1.0–1.5×, band and multiplier echoed) applied to the sweat-rate-derived
rates — or to the flagged defaults, flag intact (heat never upgrades a default's confidence).
Carb targets SHALL be unchanged by weather. Without the parameter the plan SHALL be
byte-identical to today's.

#### Scenario: A hot race day scales the bottle plan

- **WHEN** the fueling plan is requested with `weather=true` and the race heat load lands in a
  high band
- **THEN** per-leg fluid/sodium rates carry the multiplier (echoed) over the sweat-rate-derived
  baseline, and carbs are untouched

#### Scenario: Defaults stay flagged under heat

- **WHEN** no measured sweat rate exists and weather mode is on
- **THEN** the scaled fluids still carry the default-derived flag
