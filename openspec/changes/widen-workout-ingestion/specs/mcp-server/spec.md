## ADDED Requirements

### Requirement: Workouts tools expose ingestion metrics and session grouping

The `log_workout` and `patch_workout` MCP tools SHALL accept optional `distance_m` (metres, > 0), `avg_power_w` (watts, positive integer), `temperature_c` (°C, -40..60), `sweat_loss_ml` (millilitres, > 0), and `session_group` (free-text group key, non-empty, ≤ 255 chars) arguments. When set, the wrapper forwards them to the underlying REST endpoint verbatim; when omitted, the wrapper omits them from the request body (matching the existing pattern for nullable workout fields). The `list_workouts` tool SHALL accept an optional `session_group` argument forwarded as the `session_group` query parameter (the `from`/`to` window stays required). The `get_workout` and `list_workouts` tools surface the five fields on response bodies whenever the underlying rows have them set. No new tools are introduced.

#### Scenario: log_workout forwards the ingestion metrics when supplied

- **WHEN** the agent calls `log_workout` with `{"source":"manual","sport":"bike","started_at":"…","ended_at":"…","distance_m":80500,"avg_power_w":182,"temperature_c":27.5,"sweat_loss_ml":2400,"session_group":"brick-2026-06-13"}`
- **THEN** the wrapper issues `POST /workouts` with all five fields included in the JSON body
- **AND** the REST response body — including the five fields — is forwarded verbatim to the tool result

#### Scenario: log_workout omits the fields when not supplied

- **WHEN** the agent calls `log_workout` without any of the five new arguments
- **THEN** the wrapper issues `POST /workouts` with a body that does NOT contain those keys (matching the existing optional-field pattern)

#### Scenario: patch_workout supports setting, clearing, and leaving unchanged

- **WHEN** the agent calls `patch_workout` with `{"workout_id":"…","sweat_loss_ml":1850,"temperature_c":31}`
- **THEN** the PATCH body sets both fields on the row

- **WHEN** the agent calls `patch_workout` with `{"workout_id":"…","session_group":null}`
- **THEN** the PATCH body carries explicit JSON `null` for `session_group` (clearing the grouping on the backend)
- **AND** the other ingestion fields are unchanged (absent from body)

- **WHEN** the agent calls `patch_workout` with `{"workout_id":"…","notes":"hot one"}` (none of the five fields)
- **THEN** the PATCH body omits all five and the backend leaves them unchanged

#### Scenario: list_workouts forwards the session_group filter

- **WHEN** the agent calls `list_workouts` with `{"from":"2026-06-13T00:00:00Z","to":"2026-06-14T00:00:00Z","session_group":"garmin:9876543"}`
- **THEN** the wrapper issues `GET /workouts?from=…&to=…&session_group=garmin%3A9876543`
- **AND** the response — the legs of that session in `started_at` order — is forwarded verbatim

#### Scenario: Tool descriptions name the units and the brick use-case

- **WHEN** the agent reads the `log_workout` tool description
- **THEN** the description states the units explicitly: `distance_m` metres, `avg_power_w` watts, `temperature_c` °C, `sweat_loss_ml` millilitres
- **AND** explains `session_group` as "set the same key on every leg of a brick/multisport session (e.g. the Garmin parent activity id) so the legs can be fetched together"
- **AND** notes all five are nullable — omit what the source did not measure

- **WHEN** the agent reads the `patch_workout` tool description
- **THEN** the description explains the tri-state on the five fields: absent = leave unchanged, value = set, JSON null = clear

#### Scenario: Validation errors from the endpoint are forwarded verbatim

- **WHEN** the REST endpoint returns `400 {"error":"temperature_c_invalid","range":{"min":-40,"max":60}}` (or any of `distance_m_invalid`, `avg_power_w_invalid`, `sweat_loss_ml_invalid`, `session_group_invalid`)
- **THEN** the wrapper forwards the response body verbatim
- **AND** the tool result has `isError = true`

#### Scenario: workout_fueling_summary surfaces sweat/heat context when set

- **WHEN** the agent calls `workout_fueling_summary` on a workout that has `sweat_loss_ml = 2400` and `temperature_c = 27.5`
- **THEN** the response body — forwarded verbatim from `GET /workouts/{id}/fueling` — includes both fields at the top level alongside `rpe`, `gi_distress_score`, and the pre/intra/post window breakdowns
- **AND** the agent reads "estimated sweat loss vs fluid actually taken, in the day's heat" from a single tool call
