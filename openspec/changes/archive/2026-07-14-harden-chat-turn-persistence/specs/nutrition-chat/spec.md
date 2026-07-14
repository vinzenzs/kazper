## ADDED Requirements

### Requirement: Turn-persistence failures are terminal and never silent

The chat loop SHALL treat any failure to persist a conversation turn (`AppendTurns`) as terminal for the stream: it SHALL emit the typed SSE `error` event with code `persistence_error` and end the stream, emitting no further events — in particular no `done`. A `done` event therefore guarantees that every turn it summarizes has been durably stored. This applies uniformly to the pre-stream user-turn persist (existing behavior), the terminal-assistant-turn persist, the mid-loop assistant + `tool_result` pair persist, the implicit-reject declined-`tool_result` persist, and the confirm-execute `tool_result` persist. Every such failure SHALL be logged with the session id. Session titling (`SetTitleIfEmpty`) is exempt: a titling failure SHALL be logged and SHALL NOT affect the stream.

#### Scenario: Final assistant turn fails to persist

- **WHEN** the loop's terminal assistant turn fails to persist after its text has streamed
- **THEN** the stream emits `error` with code `persistence_error` and no `done` event
- **AND** the failure is logged with the session id

#### Scenario: Mid-loop tool-round persist failure aborts the loop

- **WHEN** the atomic append of an assistant `tool_use` turn and its `tool_result` reply fails after the tools were dispatched
- **THEN** the stream emits `error` with code `persistence_error` and the loop does not run further rounds
- **AND** previously persisted turns of the session remain intact, so a retry resumes from the last stored state

#### Scenario: Implicit-reject persist failure aborts before the new message is stored

- **WHEN** a `/chat` message arrives while the session awaits confirmation and persisting the synthetic declined `tool_result` turn fails
- **THEN** the stream emits `error` with code `persistence_error` before the new user message is persisted
- **AND** the stored history is left unchanged (still ending on the awaiting-confirmation turn), never with a user turn following an unanswered `tool_use`

#### Scenario: Titling failure is logged but harmless

- **WHEN** naming an untitled session from its opening message fails
- **THEN** the stream proceeds normally through `done`
- **AND** the failure is logged

### Requirement: A proposal is only surfaced after its anchor turn is stored

When a turn contains `write-confirm` tool calls, the loop SHALL persist the awaiting-confirmation assistant turn **before** emitting the `proposal` and `done` events. If that persist fails, the stream SHALL emit `error` with code `persistence_error` and SHALL NOT emit a `proposal` — so every proposal the user can see is guaranteed to be resumable by the confirmation endpoint.

#### Scenario: Pause-path persist failure suppresses the proposal

- **WHEN** the assistant turn carrying pending `write-confirm` calls fails to persist
- **THEN** the stream emits `error` with code `persistence_error` and neither `proposal` nor `done` is emitted
- **AND** a subsequent confirm request for the session returns `409 nothing_to_confirm` consistently with the stored state

#### Scenario: Successful pause is unchanged

- **WHEN** the assistant turn carrying pending `write-confirm` calls persists successfully
- **THEN** the stream emits `proposal` followed by `done` with stop reason `awaiting_confirmation`, exactly as before

### Requirement: Confirm-execute persist failure ends the resume stream visibly

When a confirmed resume has dispatched the trailing turn's tool calls and the append of their `tool_result` turn fails, the stream SHALL emit `error` with code `persistence_error` and end without continuing the loop. The stored trailing turn remains the awaiting-confirmation anchor; a retried confirmation therefore re-enters execute mode, and this exposure (identical to a stream dropped before the persist) is accepted and documented rather than silent.

#### Scenario: Executed writes with a failed result persist surface an error

- **WHEN** a confirmation request dispatches approved writes and the `tool_result` turn fails to persist
- **THEN** the stream emits `error` with code `persistence_error` and no continuation turn is streamed
- **AND** the failure is logged with the session id
