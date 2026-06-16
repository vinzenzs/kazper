## ADDED Requirements

### Requirement: Bike intensity_factor is derived from FTP when missing

The system SHALL derive a workout's `intensity_factor` as `normalized_power_w / ftp_watts`, rounded to 2 decimal places, and store it on workout create and update — but ONLY when ALL of the following hold:

- the workout's `sport` is `bike`,
- `normalized_power_w` is present and `> 0`,
- the `athlete_config` singleton has `ftp_watts` present and `> 0`, and
- the caller did NOT explicitly supply a non-null `intensity_factor` in the request.

When the caller supplies an `intensity_factor`, that value SHALL be stored verbatim (rounded at the response boundary only) and the derivation SHALL NOT run — a watch- or client-provided IF always wins. When any gate condition fails (non-bike sport, missing/zero `normalized_power_w`, unset/zero `ftp_watts`, or the athlete-config dependency unavailable), the workout SHALL be written through unchanged with `intensity_factor` left as the caller provided it (NULL when absent), and no error SHALL be raised. The value is computed against the FTP in effect at write time; the system SHALL NOT retroactively recompute the stored value when `ftp_watts` later changes.

#### Scenario: Bike workout with NP and no supplied IF gets a derived value

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `bike` workout with `normalized_power_w` `200` and no `intensity_factor`
- **THEN** the stored `intensity_factor` is `0.80`

#### Scenario: Caller-supplied IF is never overridden

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `bike` workout with `normalized_power_w` `200` and `intensity_factor` `0.95`
- **THEN** the stored `intensity_factor` is `0.95` (no derivation occurs)

#### Scenario: Non-bike workout is not derived

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `run` workout with `normalized_power_w` `200` and no `intensity_factor`
- **THEN** the stored `intensity_factor` remains NULL (FTP is a cycling metric; only bike sports derive)

#### Scenario: Missing FTP leaves IF NULL

- **WHEN** `athlete_config.ftp_watts` is unset (NULL)
- **AND** the client creates a `bike` workout with `normalized_power_w` `200` and no `intensity_factor`
- **THEN** the stored `intensity_factor` remains NULL and the create succeeds without error

#### Scenario: Missing normalized power leaves IF NULL

- **WHEN** `athlete_config.ftp_watts` is `250`
- **AND** the client creates a `bike` workout with no `normalized_power_w` and no `intensity_factor`
- **THEN** the stored `intensity_factor` remains NULL

#### Scenario: Update fills a previously-NULL IF

- **WHEN** a `bike` workout exists with `normalized_power_w` `200` and `intensity_factor` NULL
- **AND** `athlete_config.ftp_watts` is `250`
- **AND** the client updates (full-replace) that workout keeping `normalized_power_w` `200` and supplying no `intensity_factor`
- **THEN** the stored `intensity_factor` becomes `0.80`
