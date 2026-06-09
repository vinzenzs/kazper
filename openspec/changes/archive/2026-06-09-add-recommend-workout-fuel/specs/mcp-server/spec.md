## ADDED Requirements

### Requirement: recommend_workout_fuel tool wraps the recommend-workout-fuel endpoint

The MCP server SHALL expose one tool `recommend_workout_fuel` that invokes `GET /race-prep/recommend-workout-fuel` with `Authorization: Bearer <AGENT_API_TOKEN>`, forwards the response body via `toToolResult`, and does NOT send an `Idempotency-Key` header (the endpoint is read-only). Inputs mirror the REST query parameters; outputs are passed through verbatim.

#### Scenario: Workout-mode call forwards only workout_id

- **WHEN** the agent calls `recommend_workout_fuel` with `{"workout_id":"<uuid>"}`
- **THEN** the wrapper issues `GET /race-prep/recommend-workout-fuel?workout_id=<uuid>`
- **AND** does NOT include `sport`, `duration_min`, `intensity_zone`, or `body_weight_kg` query parameters
- **AND** does NOT include an `Idempotency-Key` header

#### Scenario: Explicit-mode call forwards the triplet

- **WHEN** the agent calls `recommend_workout_fuel` with `{"sport":"bike","duration_min":90,"intensity_zone":3}`
- **THEN** the wrapper issues `GET /race-prep/recommend-workout-fuel?sport=bike&duration_min=90&intensity_zone=3`
- **AND** does NOT include `workout_id`

#### Scenario: Optional body_weight_kg is forwarded when supplied

- **WHEN** the agent calls `recommend_workout_fuel` with `body_weight_kg` set alongside either mode
- **THEN** the wrapper appends `body_weight_kg=<value>` to the query string

#### Scenario: Tool description names the two modes, the literature ratios, and the linked endpoints

- **WHEN** the agent reads the `recommend_workout_fuel` tool description
- **THEN** the description states the two input modes (workout_id for an existing row; explicit sport+duration+intensity for planned sessions) and that exactly one must be used
- **AND** lists the headline literature ratios: pre 1–2 g/kg by zone, intra 30/60/90 g CHO/hr by duration bucket (and that run caps at 60), post 1.0 g/kg CHO + 0.3 g/kg protein
- **AND** notes that the post-workout protein recommendation reuses the `0.3 g/kg` MPS threshold from `protein_distribution` so the two endpoints stay consistent
- **AND** points at `plan_carb_load` for race-week / > 90 min pre-loading and at `log_workout_fuel` for committing the recommendation as a real entry
- **AND** notes that the endpoint is read-only (no idempotency-key)

#### Scenario: REST 4xx errors are forwarded as isError

- **WHEN** the REST backend returns `400 input_conflict` (both modes supplied) or `400 weight_data_missing` (no resolvable body weight)
- **THEN** the tool result has `isError: true`
- **AND** the response body is the verbatim REST error payload

#### Scenario: Response body passes through verbatim

- **WHEN** the REST backend returns a `200 OK` response with the documented pre/intra/post shape
- **THEN** the tool result content is the same JSON body byte-for-byte
- **AND** the wrapper does NOT inject any additional advisory or warning content — the response's `notes[]` and per-section `rationale` fields already carry the literature context
