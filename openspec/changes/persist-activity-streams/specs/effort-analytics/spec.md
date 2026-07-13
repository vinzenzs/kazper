# effort-analytics Specification (delta)

## MODIFIED Requirements

### Requirement: Per-activity best-effort records are computed from ingested streams and stored

The system SHALL expose `POST /api/v1/workouts/{id}/streams` accepting a workout's per-sample
time series (at least a **power** series in watts and/or a **speed** series in m/s, with sample
timestamps or a fixed cadence; an optional **heart_rate** series in bpm may accompany them for
the `activity-streams` capability). For the referenced completed workout, the system SHALL compute
the **mean-maximal** value of each provided power/speed metric at a fixed set of standard durations
(e.g. 5s, 15s, 30s, 1m, 5m, 10m, 20m, 30m, 60m) — the best rolling-window average of that length
anywhere in the activity — and SHALL store one best-effort record per (workout, metric, duration)
in a dedicated table keyed so a re-post **replaces** that workout's records rather than duplicating
them. A duration longer than the activity SHALL yield no record for that duration. The raw
streams ARE persisted by the `activity-streams` capability, and the system SHALL support
re-deriving a workout's best-effort records from those stored streams via its recompute path,
producing the same records the original ingest would (heart-rate series feed no best-effort
record — the mean-maximal ladder remains power/speed only). Power (W) and pace/speed values
live only in this capability's shapes and SHALL feed no nutrition, hydration, or energy total.
An unknown workout id SHALL return `404`; a workout with no usable series SHALL be accepted with
no records written.

#### Scenario: Posting a power stream computes and stores best efforts

- **WHEN** `POST /api/v1/workouts/{id}/streams` receives a power series for a completed workout
- **THEN** the system stores that workout's best mean power at each standard duration up to the
  activity length

#### Scenario: Re-posting replaces, does not duplicate

- **WHEN** the same workout's streams are posted a second time
- **THEN** its best-effort records are replaced, not duplicated

#### Scenario: Durations longer than the activity are skipped

- **WHEN** the activity is shorter than a standard duration
- **THEN** no best-effort record is written for that duration

#### Scenario: A workout without a usable series writes nothing

- **WHEN** the posted streams contain no power or speed series
- **THEN** the request is accepted and no best-effort records are written

#### Scenario: Recompute from stored streams reproduces the ladder

- **WHEN** a workout's best-effort records are re-derived from its persisted streams via the
  `activity-streams` recompute path
- **THEN** the resulting (workout, metric, duration) records replace the prior set and match
  what ingesting the same series would produce

#### Scenario: A heart-rate series yields no best-effort record

- **WHEN** the posted streams include a `heart_rate` series
- **THEN** no best-effort record with a heart-rate metric is written (the ladder stays
  power/speed only)
