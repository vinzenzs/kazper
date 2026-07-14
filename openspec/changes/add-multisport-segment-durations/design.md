## Context

`multisport-phase-3` added a template-level `estimated_duration_sec` (sum of every sport segment's time-bound step durations; null unless fully time-bounded). The per-segment values are computed implicitly inside that derivation and then discarded — this change surfaces them.

## Goals / Non-Goals

**Goals:** each segment self-reports its estimated duration under exactly the template-level rule; template-level semantics unchanged.

**Non-Goals:** estimating distance-bound or open steps (that needs pace assumptions — a different feature), touching materialized workouts, any write-path change.

## Decisions

### D1 — Same rule, one level down
Per segment: sum of time-bound step durations (repeat blocks expanded), `null` when any step is distance-bound/open/lap-button. Deriving at the serialization boundary (where the template-level sum already runs) — response-only, `omitempty`-style null handling, never stored, never writable.

### D2 — Template-level value stays exactly as specced
It remains the sum over segments; a template with one unbounded segment keeps a null total while its bounded segments still report their own estimates — strictly more information, no contradiction.

## Risks / Trade-offs

- None of substance: additive response field, derived from data already read.

## Migration Plan

None. Rollback = drop the field.

## Open Questions

- None.
