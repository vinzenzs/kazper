## Why

Cosmetic debt flagged when `harden-chat-turn-persistence` archived: the chat capability's spec is still slugged `nutrition-chat` — a stale name from `add-chat-backend` that predates both `expand-chat-to-coach` and `rebrand-to-kazper` (2026-06-14) — and its Purpose header still reads `TBD`. The spec system's names should describe the system that exists.

## What Changes

- **Docs/spec-mechanics only — zero code.** `openspec/specs/nutrition-chat/` → `openspec/specs/coach-chat/`, with a real Purpose paragraph replacing `TBD` (the coach chat loop: SSE streaming, tool rounds, write-confirm pause, session persistence).
- Any in-repo references to the `nutrition-chat` capability name (docs, continuity/roadmap notes are historical and stay) are updated where they describe current state.
- Capability renames aren't expressible as spec deltas (the `widen-coach-recs-to-memory` precedent: "a REMOVED-all delta can't empty a spec; folder retired by hand") — the delta here seeds the new folder, and the archive step performs the `git mv` + Purpose write by hand per tasks.

## Capabilities

### New Capabilities

- `coach-chat`: receives the existing `nutrition-chat` spec content verbatim under the correct name (the delta seeds the capability; the full 247-line content moves at archive).

### Modified Capabilities

_None (no requirement's behavior changes)._

## Impact

- `openspec/specs/` only. No code, no API, no MCP, no swag, no migration. Roadmap/continuity historical notes untouched (they describe what was true when written).
