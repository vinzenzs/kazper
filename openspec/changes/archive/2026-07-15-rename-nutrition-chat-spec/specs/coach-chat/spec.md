## ADDED Requirements

### Requirement: The chat capability is documented as coach-chat

The chat capability's authoritative specification SHALL live at
`openspec/specs/coach-chat/spec.md`, carrying the full requirement set currently under
`nutrition-chat` verbatim and a non-TBD Purpose paragraph describing the coach chat backend
(session-persisted SSE streaming, tool rounds, write-confirm pause, typed SSE error codes). The
retired `nutrition-chat` spec folder SHALL no longer exist after the move. This requirement is
naming scaffolding: at archive it is satisfied by the folder move and MAY be folded into the
moved spec's header rather than kept as a standalone requirement.

#### Scenario: The spec lives under the current name

- **WHEN** the change is archived and specs synced
- **THEN** `openspec/specs/coach-chat/spec.md` contains the chat requirements with a real
  Purpose, and `openspec/specs/nutrition-chat/` is gone
