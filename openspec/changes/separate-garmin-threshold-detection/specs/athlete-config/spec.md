## ADDED Requirements

### Requirement: Garmin-detected thresholds are stored as an advisory singleton

The system SHALL store the latest Garmin-detected physiology in a `garmin_detected_thresholds`
singleton (the detection fields mirroring the config's physiology shapes, plus `detected_at` and
audit timestamps). `PUT /api/v1/athlete-config/garmin-detected` SHALL full-replace it and SHALL
be accepted **only from the garmin identity** (any other identity → 403 with the established
identity-guard vocabulary); per the PUT rule an `Idempotency-Key` is rejected.
`GET /api/v1/athlete-config/garmin-detected` SHALL return the latest detection (`200` with a
null body-object when none exists) to the non-garmin identities. Writing a detection SHALL NOT
read or mutate `athlete_config` and SHALL NOT create `threshold_history` rows — detections are
advisory evidence; applying one goes through the deliberate `PUT /athlete-config` flow. The
table SHALL be classified export-excluded (latest-only, re-derived by the next sync).

#### Scenario: The bridge records a detection without touching the config

- **WHEN** the garmin identity PUTs detected values including `ftp_watts: 285`
- **THEN** the detection singleton holds them with `detected_at`, and `athlete_config` and
  `threshold_history` are byte-identical to before

#### Scenario: Non-garmin identities cannot write detections

- **WHEN** the mobile or agent identity PUTs `/athlete-config/garmin-detected`
- **THEN** the response is `403` and nothing is stored

#### Scenario: No detection yet reads as null, not an error

- **WHEN** `GET /athlete-config/garmin-detected` is called before any sync has written one
- **THEN** the response is `200` with a null detection

### Requirement: The athlete-config PUT rejects the garmin identity

`PUT /api/v1/athlete-config` SHALL reject requests authenticated as the garmin identity with
`403` (established identity-guard vocabulary), so the configured physiology and its
`threshold_history` remain exclusively deliberate human/coach records — an automated writer can
never overwrite a confirmed value or clear fields Garmin does not expose.

#### Scenario: A garmin-identity config write is refused

- **WHEN** the garmin identity PUTs `/athlete-config`
- **THEN** the response is `403`, the config is unchanged, and no history snapshot is written

#### Scenario: Deliberate writes are unaffected

- **WHEN** the agent identity PUTs `/athlete-config` with confirmed values
- **THEN** the existing full-replace + snapshot-on-change behavior applies unchanged

### Requirement: A per-field source selector chooses between configured and detected values

The system SHALL persist a threshold source policy as `garmin_sourced_fields` on the config row
(default empty = all manual), whitelisted to `ftp_watts`, `lactate_threshold_hr`, `max_hr`,
`threshold_pace_sec_per_km`, and the zone groups `hr_zones` / `power_zones` (zones flip as
whole sets). `PUT /api/v1/athlete-config/sources` (non-garmin identities; garmin → 403) SHALL
full-replace the list, rejecting unknown tokens with `400 source_field_invalid`; it SHALL mutate
only the policy — never the physiology values — and the config PUT's full-replace SHALL NOT
touch the policy column. `GET /athlete-config` SHALL echo the active `sources`. A
`set_threshold_sources` MCP tool (write tier, one PUT) SHALL mirror the endpoint; its
description SHALL note that flipping a source changes effective thresholds and point at
`recompute-tss` when derived values matter.

#### Scenario: The coach flips FTP to Garmin

- **WHEN** the agent PUTs `/athlete-config/sources` with `["ftp_watts"]`
- **THEN** the policy holds exactly that list, the configured `ftp_watts` value is unchanged,
  and no threshold-history snapshot is written

#### Scenario: An unknown field token is rejected

- **WHEN** the body carries `["ftp_watts", "vo2max"]`
- **THEN** the response is `400` with `source_field_invalid` and the policy is unchanged

#### Scenario: Confirming values never resets policy

- **WHEN** `PUT /athlete-config` full-replaces the physiology while `["ftp_watts"]` is sourced
- **THEN** the policy still holds `["ftp_watts"]`

### Requirement: Computations consume the effective config

The system SHALL resolve an **effective config**: per field, the latest detection's value where
the field's source is `garmin` AND the detection carries a non-null value (manual fallback
otherwise — a garmin-sourced field with no detection never yields a hole), the confirmed value
for all other fields. `GET /api/v1/athlete-config/effective` SHALL return the resolved view with
a per-field `source` annotation. Computational consumers of athlete physiology — per-sport TSS
derivation, zone-reference target resolution, race pacing, and step compliance — SHALL consume
the effective view (wired once at the server trunk), while `GET /athlete-config` continues to
return exactly the confirmed values. With an empty policy the effective view SHALL equal the
confirmed config (shipping this change alters no computed number until a source is flipped).

#### Scenario: A garmin-sourced FTP drives TSS derivation

- **WHEN** `ftp_watts` is garmin-sourced, the detection holds 285, and the config holds 278
- **THEN** the effective view reports `ftp_watts: 285` annotated `source: "garmin"` and a new
  bike workout's power TSS derives against 285

#### Scenario: Missing detection falls back to manual

- **WHEN** `max_hr` is garmin-sourced but the latest detection carries no max HR
- **THEN** the effective `max_hr` is the confirmed value, annotated `source: "manual"`

#### Scenario: All-manual policy is behavior-identical to today

- **WHEN** `garmin_sourced_fields` is empty
- **THEN** the effective view equals the confirmed config field-for-field
