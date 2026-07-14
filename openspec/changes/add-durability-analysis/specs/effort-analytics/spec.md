## MODIFIED Requirements

### Requirement: Per-activity best-effort records are computed from ingested streams and stored

The system SHALL expose `POST /api/v1/workouts/{id}/streams` accepting a workout's per-sample
time series (at least a **power** series in watts and/or a **speed** series in m/s, with sample
timestamps or a fixed cadence; an optional **heart_rate** series in bpm may accompany them for
the `activity-streams` capability). For the referenced completed workout, the system SHALL compute
the **mean-maximal** value of each provided power/speed metric at a fixed set of standard durations
(e.g. 5s, 15s, 30s, 1m, 5m, 10m, 20m, 30m, 60m) — the best rolling-window average of that length
anywhere in the activity — and SHALL store one best-effort record per (workout, metric, duration,
kJ tier) in a dedicated table keyed so a re-post **replaces** that workout's records rather than
duplicating them. The **fresh** ladder is stored at kJ tier `0`. For **power** series the system
SHALL additionally compute kJ-tiered best efforts at durations 1m/5m/20m for accumulated-work
tiers 500/1000/1500/2000 kJ — the best rolling-window average whose window **starts at or after**
the point where cumulative work reaches the tier — storing rows only for tiers the activity
reaches. A duration longer than the activity SHALL yield no record for that duration. The raw
streams ARE persisted by the `activity-streams` capability, and the system SHALL support
re-deriving a workout's best-effort records (fresh and tiered) from those stored streams via its
recompute path, producing the same records the original ingest would (heart-rate series feed no
best-effort record — the mean-maximal ladder remains power/speed only; speed series feed no
tiered record). Power (W) and pace/speed values live only in this capability's shapes and SHALL
feed no nutrition, hydration, or energy total. An unknown workout id SHALL return `404`; a
workout with no usable series SHALL be accepted with no records written.

#### Scenario: Posting a power stream computes and stores best efforts

- **WHEN** `POST /api/v1/workouts/{id}/streams` receives a power series for a completed workout
- **THEN** the system stores that workout's best mean power at each standard duration up to the
  activity length

#### Scenario: A long ride stores kJ-tiered best efforts

- **WHEN** the posted power series accumulates 1600 kJ over the activity
- **THEN** tiered records are stored at 1m/5m/20m for tiers 500/1000/1500 (windows starting after
  the tier's cumulative work) and no tier-2000 record is written

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
- **THEN** the resulting (workout, metric, duration, kJ tier) records replace the prior set and
  match what ingesting the same series would produce

#### Scenario: A heart-rate series yields no best-effort record

- **WHEN** the posted streams include a `heart_rate` series
- **THEN** no best-effort record with a heart-rate metric is written (the ladder stays
  power/speed only)

## ADDED Requirements

### Requirement: Windowed durability compares fresh and kJ-tiered best efforts

The system SHALL expose `GET /api/v1/workouts/durability?from=&to=&tz=` returning, for each
tiered duration (1m/5m/20m), the window's fresh (tier-0) best power and each kJ tier's best
power with `fade_pct = (fresh − tier) / fresh × 100` (1 decimal, boundary-only rounding), each
entry carrying its contributing workout id and date. Tiers with no record in the window SHALL be
omitted; a window with fresh data but no tiered rows SHALL return the fresh values with
`reason: "no_tiered_data"`. The endpoint SHALL read stored best-effort rows only (no stream
scans), persist nothing, and use the power-curve range/`tz` error contract.

#### Scenario: Fade is reported per duration and tier

- **WHEN** the window holds fresh and tier-1000 20-minute records
- **THEN** the response carries the 20m fresh watts and a tier-1000 entry with its watts,
  `fade_pct`, and contributing workout

#### Scenario: A window without tiered data degrades with a reason

- **WHEN** no kJ-tiered records exist in the window (e.g. history not yet recomputed)
- **THEN** the response is `200` with the fresh values and `reason: "no_tiered_data"`

### Requirement: Durability is readable over MCP

The system SHALL expose a `durability` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/durability` and forwarding the body verbatim, with `from`/`to` and optional
`tz` args; its description SHALL note that historical windows require the stream recompute
backfill to carry tiered data.

#### Scenario: Agent reads the fade table in one call

- **WHEN** the agent invokes `durability` with a window
- **THEN** the tool issues one GET and returns the fresh/tiered fade table verbatim
