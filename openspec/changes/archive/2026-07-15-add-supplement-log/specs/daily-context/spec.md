## ADDED Requirements

### Requirement: The daily context carries today's supplement entries

The `/api/v1/context/daily` payload SHALL include a `supplements` array holding today's entries
verbatim (ascending `logged_at`), omitted entirely when none exist — never an empty array, never
an error — positioned beside the wellness object. Historical entries SHALL NOT be included
(window reads stay on the supplements endpoints).

#### Scenario: Today's intakes surface in the daily context

- **WHEN** two supplement entries exist for today
- **THEN** `/context/daily` carries both in a `supplements` array, ascending

#### Scenario: A day without intakes omits the field

- **WHEN** no entry exists for today
- **THEN** the payload has no `supplements` key and is otherwise unchanged
