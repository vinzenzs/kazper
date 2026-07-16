## ADDED Requirements

### Requirement: Daily fuel targets are suggested from planned training load

The system SHALL expose `GET /api/v1/nutrition/fuel-plan?from=&to=&tz=` (window defaulting to
today plus six days, capped at 14 days) returning, per day: the planned sessions (workout id,
sport, planned TSS, planned duration), a derived `tier` — `rest` (no planned session), `easy`
(total planned TSS < 60), `moderate` (60–150), `heavy` (> 150, or any single session of 150
minutes or more) — the tier's carb prescription in g/kg (**3 / 5 / 7 / 9** respectively),
`suggested_carbs_g` (g/kg × the body-weight trend's latest value, echoed with its date, 1
decimal), the date's currently-effective goal carbs, and `delta_g`. Days beyond the last
materialized plan data SHALL be flagged `plan_missing: true`; absent weight data SHALL degrade
to tiers-without-gram-targets with `reason: "weight_missing"`. The endpoint SHALL be
compute-on-read, persist nothing, and SHALL NOT write goals or overrides — applying a
suggestion is the existing per-date override flow.

#### Scenario: A heavy day suggests the heavy prescription

- **WHEN** tomorrow carries a planned 180-TSS ride and the weight trend reads 70.0 kg
- **THEN** tomorrow's entry reports `tier: "heavy"`, `suggested_carbs_g: 630`, the effective
  goal carbs, and the delta

#### Scenario: A long low-intensity session is heavy regardless of TSS

- **WHEN** a day holds one planned 160-minute session with 90 planned TSS
- **THEN** the day classifies `heavy`

#### Scenario: Beyond the plan is flagged, not disguised as rest

- **WHEN** the window extends past the last materialized plan week
- **THEN** those days carry `tier: "rest"` with `plan_missing: true`

#### Scenario: Missing weight degrades to tiers only

- **WHEN** no body-weight data exists
- **THEN** tiers and g/kg values are returned with `reason: "weight_missing"` and no
  `suggested_carbs_g`

### Requirement: The fuel plan is readable over MCP

The system SHALL expose a `fuel_plan` MCP tool (read tier) issuing a single
`GET /api/v1/nutrition/fuel-plan` and forwarding the body verbatim (optional `from`/`to`/`tz`).
The description SHALL state that suggestions are applied via the existing goal-override flow
after athlete confirmation, and that this endpoint periodizes carbs within the standing kcal
target (expenditure estimation is a separate concern).

#### Scenario: Agent reads the week's fuel plan in one call

- **WHEN** the agent invokes `fuel_plan` with no arguments
- **THEN** the tool issues one GET and returns seven classified days verbatim
