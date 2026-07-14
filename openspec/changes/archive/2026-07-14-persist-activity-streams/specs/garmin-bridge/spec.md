# garmin-bridge Specification (delta)

## MODIFIED Requirements

### Requirement: The sync ingests per-activity streams for effort analytics

The bridge's activity sync SHALL, in addition to the scalar/zone/split/set detail it already
pulls, fetch each activity's detail streams (`get_activity_details`), extract the available
per-sample **power** (watts), **speed** (m/s), and **heart-rate** (bpm) series, and post them to
`POST /api/v1/workouts/{id}/streams` for the workout that activity maps to. The heart-rate
series SHALL follow the same extraction conventions as power and speed: located via the
`directHeartRate` metric descriptor, treated as contiguous ~1 Hz samples with missing samples
filled as `0`, and dropped entirely when the series is wholly non-positive (no HR sensor
present). The stream fetch and post SHALL be individually guarded so that a failure — or an
activity that carries no power/speed/heart-rate series (e.g. an indoor run without a power
meter) — omits that activity's streams without aborting the day, mirroring the existing
per-detail guarding. Re-running a day (daily re-pull or backfill) SHALL be idempotent:
re-posting an activity's streams replaces that workout's best-effort records and stored stream
rows rather than duplicating them.

#### Scenario: An activity with a power stream is ingested

- **WHEN** the sync processes a completed activity whose Garmin detail includes a power series
- **THEN** the bridge posts that series to `POST /api/v1/workouts/{id}/streams` for the mapped
  workout

#### Scenario: An activity's heart-rate series is included

- **WHEN** the sync processes an activity whose Garmin detail includes a usable
  `directHeartRate` series
- **THEN** the posted streams body carries a `heart_rate` array (bpm) alongside any power/speed
  series

#### Scenario: An activity without a usable stream is skipped gracefully

- **WHEN** an activity carries no power, speed, or heart-rate series (or the detail fetch fails)
- **THEN** that activity's streams are omitted and the rest of the day's sync continues

#### Scenario: Re-running a day re-posts idempotently

- **WHEN** a day is synced a second time (re-pull or backfill)
- **THEN** each activity's streams are re-posted and its best-effort records and stored stream
  rows are replaced, not duplicated
