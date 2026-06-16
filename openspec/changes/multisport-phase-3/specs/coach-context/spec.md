## ADDED Requirements

### Requirement: The training-context load summary decomposes multisport workouts by segment sport

The `GET /context/training` recent-load summary SHALL count a `multisport`
workout once per **non-transition segment sport** of its referenced multisport
template in the `by_sport` breakdown — so a swim→bike→run brick contributes one
each to `swim`, `bike`, and `run` rather than a single `multisport` entry. The
segment sports SHALL be resolved from the workout's `multisport_template_id`. The decomposition SHALL apply only to the `by_sport`
map; the summary's `count`, `total_duration_min`, and `total_kcal` SHALL still
treat the brick as a single session (one count, one window, one burn). When the
multisport template cannot be resolved (the repo is unavailable, the template was
deleted, or the load fails), the workout SHALL fall back to a single `multisport`
entry in `by_sport`, and the aggregate read SHALL NOT error. Non-multisport
workouts SHALL be counted under their own sport exactly as before.

#### Scenario: A brick credits each of its segment sports

- **WHEN** the recent-load window contains a `multisport` workout whose template
  has swim, transition, bike, transition, and run segments
- **THEN** `by_sport` shows `swim`, `bike`, and `run` each incremented by one (and
  no `transition` or `multisport` entry from that workout)
- **AND** `count` increases by exactly one for that workout

#### Scenario: An unresolvable multisport workout falls back to the multisport bucket

- **WHEN** a `multisport` workout is in the window but its template cannot be
  resolved
- **THEN** `by_sport` shows a single `multisport` entry for it and the response is
  returned without error

#### Scenario: Single-sport workouts are unaffected

- **WHEN** the window contains only single-sport workouts
- **THEN** `by_sport` counts each under its own sport exactly as before
