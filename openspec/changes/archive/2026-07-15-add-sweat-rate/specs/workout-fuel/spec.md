## ADDED Requirements

### Requirement: A workout's sweat rate is derivable from explicit weights and logged fluid

The system SHALL expose
`GET /api/v1/workouts/{id}/sweat-rate?pre_weight_kg=&post_weight_kg=&fluid_ml_override=`
computing the standard field test over a **completed** workout: fluid intake as the sum of the
workout-linked hydration entries' ml and workout-fuel entries' `quantity_ml` (itemized in the
response; a supplied `fluid_ml_override` ≥ 0 replaces the derived sum and is echoed),
`sweat_loss_ml = (pre − post) × 1000 + fluid_ml`, and `sweat_rate_ml_per_hr` over the workout's
elapsed duration. `pre_weight_kg`/`post_weight_kg` are REQUIRED positive numbers
(`400 pre_weight_invalid` / `post_weight_invalid`; `400 fluid_override_invalid` on a negative
override). A planned workout SHALL return `409 workout_not_completed`; an unknown id
`404 not_found`. A negative loss or a rate above 5000 ml/hr SHALL still return the computed
values with `warning: "implausible_result"`. Values SHALL round to 1 decimal at the boundary;
the computation SHALL persist nothing and feed no daily hydration or nutrition total.

#### Scenario: A field test computes loss and rate

- **WHEN** a 2-hour completed ride carries 1000 ml of linked fluid and
  `pre_weight_kg=71.0&post_weight_kg=69.8` is supplied
- **THEN** the response reports `sweat_loss_ml = 2200`, `sweat_rate_ml_per_hr = 1100`, and the
  fluid itemization

#### Scenario: An override replaces derived fluid

- **WHEN** `fluid_ml_override=1500` is supplied
- **THEN** the computation uses 1500 ml, and the response shows both the override and the
  derived itemization it replaced

#### Scenario: Weight gain warns instead of refusing

- **WHEN** `post_weight_kg` exceeds `pre_weight_kg` enough to make the loss negative
- **THEN** the computed values return with `warning: "implausible_result"`

#### Scenario: A planned workout is rejected

- **WHEN** the referenced workout is not completed
- **THEN** the response is `409` with `workout_not_completed`

### Requirement: The sweat rate is readable over MCP

The system SHALL expose a `sweat_rate` MCP tool (read tier) issuing a single
`GET /api/v1/workouts/{id}/sweat-rate` and forwarding the body verbatim (`workout_id`,
`pre_weight_kg`, `post_weight_kg` required; `fluid_ml_override` optional); the description SHALL
frame the result as a field-test calculation whose quality follows the supplied weights.

#### Scenario: Agent computes a session's sweat rate in one call

- **WHEN** the agent invokes `sweat_rate` with the workout id and both weights
- **THEN** one GET is issued and the loss, rate, and itemization return verbatim
