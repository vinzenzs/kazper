## MODIFIED Requirements

### Requirement: Garmin imports reconcile against open planned workouts

The system SHALL reconcile completed activities and planned workouts in **both
directions**, matching on exact sport and **local calendar day within a ±1-day
tolerance**, preferring an exact same-day candidate. A match SHALL act only when
there is **exactly one** candidate after same-day preference (if any candidate
falls on the exact day, only same-day candidates are considered); zero or
more-than-one SHALL never auto-link.

**Forward (at ingest).** When a completed activity is ingested via
`POST /workouts` or `POST /workouts/bulk` with `source='garmin'` and its
`external_id` is not already stored, the system SHALL match exactly one **open
planned workout** — `status='planned'`, `external_id IS NULL`, the same sport,
within ±1 local day of the activity's start (same day preferred). On exactly one
match the system SHALL **fulfill** that planned workout in place: set its
`external_id`, `source`, and actual metrics from the activity, flip `status` to
`completed`, and retain its `template_id` and `plan_slot_id`; no new row is
created. On no match the system SHALL insert a standalone completed row. On more
than one candidate the system SHALL insert a standalone completed row and mark it
as needing a link rather than guess. The match SHALL run only on first sight; a
subsequent re-sync of the same activity follows the existing `external_id` UPSERT
path.

**Reverse (at materialize).** When a plan slot is materialized and no workout row
yet exists for that `plan_slot_id`, the system SHALL match exactly one
**adoptable completed activity** — `status='completed'`, `plan_slot_id IS NULL`,
`external_id IS NOT NULL`, the slot's sport, within ±1 local day of the slot's
planned date (same day preferred). On exactly one match the system SHALL **adopt**
that activity: set its `plan_slot_id` and `template_id` from the slot, clear
`needs_link`, and keep its `status='completed'` and actual metrics; no planned
row is created. On no match the system SHALL create the planned row as usual. On
more than one candidate the system SHALL create the planned row and leave the
completed activities standalone. Once a slot has a workout row, re-materialize
follows the existing `plan_slot_id`-keyed, `status='planned'`-guarded path and
SHALL NOT re-adopt or duplicate.

#### Scenario: A completed import fulfills the matching planned workout

- **WHEN** a `garmin` activity is ingested for a sport and local day on which
  exactly one open planned workout exists
- **THEN** that planned workout is updated to `status='completed'` with the
  activity's `external_id`, `source`, and actual metrics
- **AND** its `template_id` and `plan_slot_id` are retained
- **AND** no second row is created

#### Scenario: No matching planned workout creates a standalone row

- **WHEN** a `garmin` activity is ingested and no open planned workout matches its
  sport within ±1 local day
- **THEN** a standalone completed workout is created (the prior behavior)

#### Scenario: Ambiguous match is flagged, not guessed

- **WHEN** a `garmin` activity matches more than one open planned workout of the
  same sport (after same-day preference)
- **THEN** a standalone completed workout is created and marked as needing a link
- **AND** no planned workout is auto-fulfilled

#### Scenario: Re-sync of a fulfilled activity is idempotent

- **WHEN** the daily sync re-sends an activity whose `external_id` is already
  stored (on a fulfilled planned row)
- **THEN** ingestion follows the existing `external_id` UPSERT path and updates
  that row in place
- **AND** reconciliation does not run again

#### Scenario: Matching uses local calendar day and exact sport

- **WHEN** an activity starts late in the local evening
- **THEN** it is matched against planned workouts on that local date (not the UTC
  date)
- **AND** only planned workouts of the same sport are considered

#### Scenario: A cross-day-by-one activity reconciles via the tolerance

- **WHEN** a `garmin` run is ingested whose local day is one day off the only
  open planned run of the same sport, and there is no same-day candidate
- **THEN** that adjacent-day planned workout is fulfilled (no standalone row)

#### Scenario: Same-day candidate is preferred over an adjacent-day one

- **WHEN** both a same-day and an adjacent-day open planned workout of the sport
  exist for an ingested activity
- **THEN** the same-day planned workout is fulfilled and the adjacent-day one is
  untouched (the match is not treated as ambiguous)

#### Scenario: Materialize adopts an already-imported activity (reverse)

- **WHEN** a plan slot is materialized and exactly one completed `garmin` activity
  of the slot's sport, with no `plan_slot_id`, exists within ±1 local day of the
  slot's date
- **THEN** that activity is adopted — its `plan_slot_id` and `template_id` are set
  from the slot, `needs_link` is cleared, and it stays `completed`
- **AND** no new planned row is created for the slot

#### Scenario: Reverse declines on more than one candidate

- **WHEN** a slot is materialized and more than one adoptable completed activity
  matches its sport within ±1 local day (after same-day preference)
- **THEN** the planned row is created normally and the completed activities are
  left standalone (resolved via explicit fulfill)

#### Scenario: Re-materialize does not re-adopt or duplicate

- **WHEN** a slot whose activity was already adopted (a completed row carrying its
  `plan_slot_id`) is re-materialized
- **THEN** the slot-keyed, `status='planned'`-guarded path skips it and no second
  row is created
