## ADDED Requirements

### Requirement: The daily context carries today's wellness entry

The `/api/v1/context/daily` payload SHALL include a `wellness` object holding today's wellness
entry verbatim (the reported scores and note), positioned as a sibling of the objective recovery
data so subjective and device-measured state read side by side. When no entry exists for today
the `wellness` field SHALL be omitted entirely — never an empty object and never an error — and
the rest of the payload SHALL be unaffected. Historical entries SHALL NOT be included (window
reads stay on the wellness endpoints).

#### Scenario: A logged day surfaces in the daily context

- **WHEN** today has a wellness entry with `fatigue = 3` and a note
- **THEN** `/context/daily` includes `wellness` carrying exactly those fields

#### Scenario: An unlogged day omits the field

- **WHEN** today has no wellness entry
- **THEN** the `/context/daily` payload has no `wellness` key and is otherwise unchanged
