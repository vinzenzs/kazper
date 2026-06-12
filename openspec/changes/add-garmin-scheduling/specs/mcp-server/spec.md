# mcp-server — delta for add-garmin-scheduling

## ADDED Requirements

### Requirement: Garmin scheduling tools push the plan to the watch from the agent

The MCP server SHALL expose `garmin_schedule_workout` (a `workout_id` →
`POST /garmin/schedule/workout`), `garmin_unschedule_workout` (a `workout_id` →
`DELETE /garmin/schedule/workout/{id}`), `garmin_schedule_plan` (a plan scope →
`POST /garmin/schedule/plan`), and `garmin_list_scheduled` (a date range →
`GET /garmin/calendar`), each issuing exactly one HTTP call to the corresponding
backend endpoint and forwarding the response verbatim. Write tools SHALL
auto-derive an idempotency key when the caller does not supply one. When the
backend returns `503 garmin_disabled`, the tool result SHALL carry that body with
`isError=true`. The MCP integration expected-tools list SHALL include all four.

#### Scenario: garmin_schedule_workout issues one POST

- **WHEN** the agent calls `garmin_schedule_workout` with a planned workout's id
- **THEN** the MCP server issues exactly one `POST /garmin/schedule/workout`
- **AND** the tool result is the backend response verbatim

#### Scenario: garmin_schedule_plan pushes a week

- **WHEN** the agent calls `garmin_schedule_plan` with a plan-week scope
- **THEN** the MCP server issues exactly one `POST /garmin/schedule/plan`
- **AND** the tool result reports the per-workout results verbatim

#### Scenario: Disabled bridge surfaces as a tool error

- **WHEN** `GARMIN_BRIDGE_URL` is unset and the agent calls any scheduling tool
- **THEN** the tool result carries the `503 garmin_disabled` body with `isError=true`

#### Scenario: Expected-tools list includes the scheduling tools

- **WHEN** the MCP integration test enumerates registered tools
- **THEN** `garmin_schedule_workout`, `garmin_unschedule_workout`,
  `garmin_schedule_plan`, and `garmin_list_scheduled` are all present
