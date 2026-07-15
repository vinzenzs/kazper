## ADDED Requirements

### Requirement: The training context carries detection, source policy, and effective values beside the config

The `/api/v1/context/training` bundle SHALL include, beside the existing athlete-config block:
`garmin_detected` (the latest detected values with `detected_at`; null when none),
`threshold_sources` (the active garmin-sourced field list; empty when all-manual), and an
`effective` block (the resolved values with per-field `source` annotations) — so the coach reads
configured, detected, and live-in-computations in one call, names drift, and flips a source or
proposes a deliberate update from there. Absent pieces SHALL serialize as null/empty, never as
errors, and the rest of the bundle SHALL be unaffected.

#### Scenario: Drift and policy are visible in one read

- **WHEN** the config holds `ftp_watts: 278`, the detection holds 285, and `ftp_watts` is
  garmin-sourced
- **THEN** the bundle shows the configured 278, the detected 285 with `detected_at`,
  `threshold_sources: ["ftp_watts"]`, and an effective `ftp_watts: 285` annotated
  `source: "garmin"`

#### Scenario: No detection degrades to null

- **WHEN** no detection has been recorded and the policy is empty
- **THEN** `garmin_detected` is null, `threshold_sources` is empty, the effective block equals
  the config, and the bundle is otherwise unchanged
