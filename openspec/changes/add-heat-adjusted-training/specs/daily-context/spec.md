## ADDED Requirements

### Requirement: The daily context carries today's and tomorrow's heat picture

The `/api/v1/context/daily` payload SHALL include a `heat` block — for today's and tomorrow's
planned outdoor sessions: `heat_load_c`, the acclimatization level, and the suggested
adjustment percentage, with the resolved location name — so the morning check-in surfaces
weather-driven session changes without an extra call (the trigger for the coach's confirmed
update flow). Sessions that are indoor, absent, or uncomputable (location unconfigured,
weather unavailable) SHALL be omitted from the block; a fully empty block SHALL be omitted
entirely, never an error.

#### Scenario: A hot tomorrow surfaces in the check-in

- **WHEN** tomorrow's outdoor threshold session computes a heat load of 33 °C
- **THEN** `/context/daily` carries tomorrow's load, level, and suggested adjustment beside
  the resolved location name

#### Scenario: Nothing heat-relevant omits the block

- **WHEN** today and tomorrow hold only indoor or no planned sessions
- **THEN** the payload has no `heat` key and is otherwise unchanged
