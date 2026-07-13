# athlete-config Specification (delta)

## ADDED Requirements

### Requirement: Threshold changes append dated history snapshots

The system SHALL maintain an `athlete_config_history` table of full-row
physiology snapshots, one row per `effective_from` DATE (primary key),
carrying the same 16 nullable physiology columns as `athlete_config` (same
types and `> 0` CHECKs) plus `created_at`/`updated_at`. Whenever
`PUT /athlete-config` results in a stored state that differs from the prior
stored state on any physiology field (pointer-aware comparison across all 16
fields; timestamps excluded), the system SHALL record the new full state as
the snapshot for the current date — inserting it, or replacing an existing
same-date snapshot. A PUT that changes nothing SHALL append and modify
nothing. If a same-date replacement would make the snapshot identical to the
latest snapshot dated strictly earlier, the same-date row SHALL be removed
instead, so that no two consecutive history rows are physiology-identical.
The singleton upsert and the history maintenance SHALL be applied atomically
(a history failure fails the PUT; the two can never diverge). The singleton
remains the authoritative current read: `GET /athlete-config` and
`PUT /athlete-config` responses and error paths are unchanged by this
requirement.

#### Scenario: A changed PUT appends a snapshot

- **WHEN** the stored config has `ftp_watts: 240`
- **AND** the client calls `PUT /athlete-config` with a body whose `ftp_watts` is `255` (other fields unchanged)
- **THEN** the PUT succeeds exactly as before (200, singleton replaced)
- **AND** a new `athlete_config_history` row exists with today's `effective_from` and the full new state including `ftp_watts: 255`

#### Scenario: A no-change PUT appends nothing

- **WHEN** the daily Garmin sync re-issues `PUT /athlete-config` with a body producing a stored state identical to the current one on every physiology field
- **THEN** the PUT succeeds as before
- **AND** no history row is inserted, replaced, or deleted

#### Scenario: A second change on the same date replaces that date's snapshot

- **WHEN** a PUT earlier today changed `ftp_watts` to `255` (snapshot for today exists)
- **AND** a later PUT today changes `ftp_watts` to `260`
- **THEN** today's history row is replaced in place with the `ftp_watts: 260` state
- **AND** exactly one history row carries today's `effective_from`

#### Scenario: A same-day revert removes the day's snapshot

- **WHEN** the latest snapshot dated before today records `ftp_watts: 240`
- **AND** a PUT today changed `ftp_watts` to `255`, and a later PUT today restores the exact prior state (`ftp_watts: 240`, all other fields equal)
- **THEN** today's history row is removed
- **AND** the history is identical to before the first PUT (no two consecutive rows are physiology-identical)

#### Scenario: The first-ever PUT seeds the first snapshot

- **WHEN** no `athlete_config` row and no history rows exist
- **AND** the client calls `PUT /athlete-config` with `{"ftp_watts": 240}`
- **THEN** the singleton row is created as before
- **AND** one history row exists with today's `effective_from` and `ftp_watts: 240`

### Requirement: History read endpoint returns dated snapshots

The system SHALL expose `GET /athlete-config/history` returning the history
rows ascending by `effective_from` as
`{"history":[{"effective_from":"YYYY-MM-DD", <physiology fields>, "created_at":..., "updated_at":...}, ...]}`,
with null fields omitted (`omitempty`) and float fields rounded via
`numfmt.Round1` at the response boundary only — the same presentation rules
as `GET /athlete-config`. Optional `from` and `to` query parameters SHALL
bound `effective_from` inclusively. A malformed date parameter SHALL return
`400 Bad Request` with `{"error":"date_invalid","field":"from"|"to"}`; a
`from` later than `to` SHALL return `400 Bad Request` with
`{"error":"range_invalid"}`. There is no range cap and no pagination (history
grows only when physiology changes). An empty result — including before any
config has ever been written — SHALL return `200 OK` with `{"history":[]}`.

#### Scenario: History lists snapshots ascending

- **WHEN** history contains snapshots effective `1970-01-01` (`ftp_watts: 240`) and `2026-05-02` (`ftp_watts: 255`)
- **AND** the client calls `GET /athlete-config/history`
- **THEN** the response is `200 OK` with `history` listing both snapshots in that order
- **AND** each entry carries its `effective_from` and the full populated physiology state (nulls omitted)

#### Scenario: from/to bound the range inclusively

- **WHEN** history contains snapshots effective `1970-01-01`, `2026-05-02`, and `2026-06-10`
- **AND** the client calls `GET /athlete-config/history?from=2026-05-02&to=2026-06-10`
- **THEN** the response contains exactly the `2026-05-02` and `2026-06-10` snapshots

#### Scenario: Invalid parameters are rejected with structured errors

- **WHEN** the client calls `GET /athlete-config/history?from=not-a-date`
- **THEN** the response is `400 Bad Request` with `{"error":"date_invalid","field":"from"}`
- **AND** a call with `from=2026-06-10&to=2026-05-02` returns `400 Bad Request` with `{"error":"range_invalid"}`

#### Scenario: Empty history returns an empty list

- **WHEN** no config has ever been written (no history rows)
- **AND** the client calls `GET /athlete-config/history`
- **THEN** the response is `200 OK` with `{"history":[]}`

#### Scenario: Pace floats round at the response boundary

- **WHEN** a stored snapshot has `threshold_pace_sec_per_km` = `258.04999`
- **THEN** the history entry returns `"threshold_pace_sec_per_km": 258.0`
- **AND** the stored column is unchanged

### Requirement: History is seeded from the existing config at migration time

The migration that creates `athlete_config_history` SHALL insert one baseline
snapshot copied from the current `athlete_config` row (when one exists) with
the sentinel `effective_from = 1970-01-01`, meaning "the oldest known state —
assumed for all earlier dates"; the seed row's `created_at` records when
tracking actually began. On a database with no config row, nothing is seeded
and the first PUT creates the first snapshot. Once a config exists, history
is therefore never empty.

#### Scenario: Migration seeds one baseline snapshot

- **WHEN** the migration runs against a database whose `athlete_config` row has `ftp_watts: 240`
- **THEN** `athlete_config_history` contains exactly one row
- **AND** that row has `effective_from = 1970-01-01` and the full current physiology state including `ftp_watts: 240`

#### Scenario: Fresh database seeds nothing

- **WHEN** the migration runs against a database with no `athlete_config` row
- **THEN** `athlete_config_history` is created empty
- **AND** the first subsequent `PUT /athlete-config` creates the first snapshot

### Requirement: Thresholds are resolvable as of a date

The athlete-config service SHALL provide a service-level as-of lookup
(`ConfigAsOf(date)`) returning the physiology state — the full snapshot plus
its `effective_from` — from the latest history row with
`effective_from <= date`, or a nil result (distinct from an error) when
history is empty. Combined with the epoch-dated seed, the lookup is total for
any plausible date once a config exists. This lookup is a provider contract
for future consumers (per-sport TSS derivation, step-compliance zone
resolution, race pacing): **no existing consumer is rewired to it in this
change** — every current consumer keeps reading the singleton's current
values, and each rewiring is an explicit per-consumer follow-up.

#### Scenario: As-of resolves the state effective on a date

- **WHEN** history contains snapshots effective `1970-01-01` (`ftp_watts: 240`) and `2026-05-02` (`ftp_watts: 255`)
- **AND** the service resolves `ConfigAsOf(2026-04-15)`
- **THEN** the result is the `1970-01-01` snapshot (`ftp_watts: 240`)
- **AND** `ConfigAsOf(2026-05-02)` and `ConfigAsOf(2026-06-01)` both resolve to the `2026-05-02` snapshot (`ftp_watts: 255`)

#### Scenario: Empty history resolves to nil, not an error

- **WHEN** no config has ever been written
- **AND** the service resolves `ConfigAsOf` for any date
- **THEN** the result is nil with no error (mirroring the singleton's absent-row signal)

#### Scenario: Existing consumers still read current values

- **WHEN** history contains an older snapshot with a different `ftp_watts` than the current singleton
- **AND** a bike workout qualifying for `intensity_factor` derivation is created
- **THEN** the derivation uses the singleton's current `ftp_watts`, exactly as before this change
