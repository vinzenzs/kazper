## Context

`openspec/specs/nutrition-chat/spec.md` (247 lines) is the chat capability's authoritative spec under a name two rebrands out of date, with `Purpose: TBD`. OpenSpec deltas operate on requirements within a capability; renaming a capability folder is out-of-band mechanics, established by the `coach-recommendations` → `coach-memory` rename (folder retired by hand at archive).

## Goals / Non-Goals

**Goals:** the spec folder matches the system's language (`coach-chat`); a real Purpose header.

**Non-Goals:** changing any requirement, touching code or routes, rewriting historical notes that used the old name.

## Decisions

### D1 — Rename at archive time via `git mv`, seeded by a minimal delta
The change's delta creates `specs/coach-chat/` with a naming requirement; at archive, the mechanics are: `git mv openspec/specs/nutrition-chat/spec.md openspec/specs/coach-chat/spec.md` (preserving history), merge the seeded requirement away (it's scaffolding, not a behavior), retitle the header, and write the Purpose paragraph. The old folder is retired by hand — the documented limitation that a REMOVED-all delta cannot empty a spec.

### D2 — Purpose text
One paragraph: the coach chat backend — session-persisted SSE streaming chat over the coach agent, tool rounds against the same REST surface, write-confirm pauses for mutating tools, typed error/persistence SSE codes. (Final wording at apply, grounded in the actual requirements.)

### D3 — Reference sweep is narrow
Only current-state descriptions update (e.g. a README capability list if one names `nutrition-chat`). Archive summaries, continuity Notes, roadmap rows are historical records and stay verbatim.

## Risks / Trade-offs

- **A concurrent change touching `nutrition-chat` deltas mid-flight** would need its delta folder renamed too — today's queue has none targeting it; apply/archive this quickly or re-check.

## Migration Plan

Docs-only; revert = `git mv` back.

## Open Questions

- None.
