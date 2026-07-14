## ADDED Requirements

### Requirement: The stats CP panel renders the estimate trend against configured FTP

The dashboard's stats critical-power panel SHALL render a CP-over-time trend line from
`GET /api/v1/workouts/cp-model/history` (null anchors rendered as gaps, never zeros) with the
configured-FTP step line overlaid from the athlete-config history endpoint, composed
client-side. When the history has no fitted anchors, or either fetch fails, the trend SHALL be
absent and the panel's existing single-fit view unaffected.

#### Scenario: The trend gaps where the model was unfittable

- **WHEN** the history carries fitted anchors around a null-anchor span
- **THEN** the CP line breaks across the span rather than interpolating through it

#### Scenario: Configured FTP overlays as a step line

- **WHEN** threshold history holds FTP changes inside the charted range
- **THEN** the panel shows the configured-FTP steps beside the derived-CP trend
