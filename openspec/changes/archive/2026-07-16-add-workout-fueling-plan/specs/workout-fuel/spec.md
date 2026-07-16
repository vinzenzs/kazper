## ADDED Requirements

### Requirement: A planned workout's fueling plan is computed on read

The system SHALL expose `GET /api/v1/workouts/{id}/fueling-plan?carbs_per_hr=` for a **planned**
workout, computing: estimated work `kJ = planned_tss / 100 × effective FTP × 3.6` with energy
expenditure per the kJ≈kcal convention; estimated carbohydrate burn as kcal × a CHO fraction
selected by planned IF (`< 0.60 → 45 %`, `0.60–0.75 → 55 %`, `0.75–0.85 → 70 %`,
`> 0.85 → 80 %`; IF derived as `sqrt(planned_tss/100 ÷ hours)` when not present) ÷ 4 kcal/g;
and an intake prescription from the duration ladder (`< 60 min → 0`,
`60–150 min → 30–60 g/hr`, `> 150 min → 60–90 g/hr`), its upper bound clamped by the OPTIONAL
`carbs_per_hr` capacity parameter (validated > 0 and ≤ 130 → `400 carbs_per_hr_invalid`). The
response SHALL carry the echoed inputs (planned TSS, duration, IF, FTP, fraction),
`estimated_kj`, `estimated_carb_burn_g`, `per_hour_g` and `session_total_g` ranges, and
`projected_deficit_g` (burn − maximum prescribed intake), gram values to 1 decimal at the
boundary. Degradations: a non-planned workout → `409 workout_not_planned`; no planned TSS and
no duration → `200` with `reason: "plan_data_missing"`; duration without TSS → intake guidance
only with `reason: "tss_missing"`; missing effective FTP → intake guidance only with
`reason: "ftp_missing"`. Compute-on-read; persists nothing; carb values SHALL feed no daily
nutrition total.

#### Scenario: A long planned ride gets burn and prescription

- **WHEN** a planned 3-hour ride carries 180 planned TSS and effective FTP is 280 W
- **THEN** the response reports `estimated_kj ≈ 1814`, a CHO fraction from IF ≈ 0.77, the
  60–90 g/hr prescription over 3 hours, and the projected deficit

#### Scenario: Capacity clamps the prescription

- **WHEN** `carbs_per_hr=70` is supplied for a session whose ladder allows up to 90
- **THEN** `per_hour_g` tops out at 70 and the totals follow

#### Scenario: A short session prescribes nothing

- **WHEN** the planned workout is 45 minutes
- **THEN** the prescription is 0 g/hr regardless of intensity, with burn still estimated

#### Scenario: Missing FTP degrades to guidance

- **WHEN** no effective FTP is available
- **THEN** the duration-ladder prescription returns with `reason: "ftp_missing"` and no burn
  estimate

#### Scenario: A completed workout is refused

- **WHEN** the referenced workout is completed
- **THEN** the response is `409` with `workout_not_planned`

### Requirement: The fueling plan is readable over MCP

The system SHALL expose a `workout_fueling_plan` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/fueling-plan` and forwarding the body verbatim (`workout_id`
required, `carbs_per_hr` optional). The description SHALL state the division of labor — races
carry authored `race-fueling-plan`s, this computes training-day prescriptions — and that the
athlete's gut capacity comes from rehearsal experience, not from this endpoint.

#### Scenario: Agent plans tomorrow's fueling in one call

- **WHEN** the agent invokes `workout_fueling_plan` for tomorrow's planned ride
- **THEN** the tool issues one GET and returns burn, prescription, and deficit verbatim
