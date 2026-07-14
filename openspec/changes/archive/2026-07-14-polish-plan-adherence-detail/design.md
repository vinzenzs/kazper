## Context

`GET /workouts/adherence` returns `missed_sessions` (oldest-first, capped 50, `missed_sessions_truncated` flag) and a `weekly` trend that emits buckets only for weeks containing a classified session (plan-week ordinals in plan mode, Monday calendar weeks otherwise). Both behaviors were shipped with their limits explicitly deferred as open questions.

## Goals / Non-Goals

**Goals:** let the caller shape the read without changing any default; keep the one-pass `computeAdherence` fold intact.

**Non-Goals:** changing defaults, an adherence panel (enabled by, not part of, this change), zero-filling as default behavior (existing consumers' payloads must not grow unasked).

## Decisions

### D1 — `missed_limit` bounds [1, 200], default 50
An explicit request for more is a deliberate act; 200 covers a worst-case YTD read while keeping the compact-list intent. `400 missed_limit_invalid` outside bounds. The truncation flag keeps its exact meaning against the effective limit.

### D2 — `zero_fill=true` is opt-in and mode-aware
Calendar mode fills every Monday week in the window span; plan mode fills every `plan_weeks.ordinal` of the plan (with phase name/week_start as populated buckets carry). Zero buckets carry zeroed counts and null rates — a week with nothing due has no adherence rate, and zero-fill must not manufacture one. Non-boolean values → `400 zero_fill_invalid`.

### D3 — Fill at serialization, not in the fold
`computeAdherence`'s single-pass fold stays untouched; filling happens when assembling the response from the bucket map (the span is known from the window/plan). Keeps the trend-can't-disagree-with-topline property trivially intact.

## Risks / Trade-offs

- **A zero-filled YTD plan read grows the payload** (~52 buckets) — bounded and opt-in.
- **Null rate in zero weeks** may surprise chart code expecting numbers — deliberate; a zero-due week has no rate, and the spec says so.

## Migration Plan

Additive params; no migration. Rollback = revert param handling.

## Open Questions

- None — this change exists to close the two that were open.
