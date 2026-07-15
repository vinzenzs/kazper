## Context

Ingest stores one best-effort row per (workout, metric, duration, kj_tier) for every provided series — sport-blind by design, and correctly so (a run's power ladder is true data about that run). The bug is at read: `Repo.Curve` (power-curve/cp-model/cp-model-history/power-profile input) and the durability query filter on `metric` + completed + window but not `w.sport`, silently mixing running power into bike analytics and bike speed into run/swim pace analytics. Found via a live CP fit whose points traced to a running workout.

## Goals / Non-Goals

**Goals:** every windowed aggregate attributes efforts to the sport it's answering for; history is correct immediately with zero data movement; a garbage fit can never again present without a warning.

**Non-Goals:** deleting or rewriting stored rows; multisport per-segment attribution (no segment-level streams); running-power analytics (separate future concern); changing per-workout endpoints.

## Decisions

### D1 — Fix at read, not at ingest
Ingest-side gating (skip power rows for runs) would fix the future but strand history behind a recompute sweep, and would delete true information. A `w.sport` predicate in the two windowed queries fixes past and future in one commit and keeps the door open for running-power analytics later.

### D2 — Metric→sport binding
`power` windows bind to `sport = 'bike'`; `speed` windows bind to the caller's requested sport (`run`/`swim` — the existing power-curve `sport` param semantics, now actually enforced). Durability/cp-model/profile are power-only and hard-bind to bike. Multisport workouts match no sport-scoped window (their streams span legs indistinguishably); the accepted cost is a tri race's bike leg not feeding the bike curve — honest until segment-level streams exist.

### D3 — `poor_fit` warning at r² < 0.5, warning not gate
The contaminated fit passed both existing gates (points + span) — quality is an independent failure axis. Below 0.5 the line explains less than half the variance and CP/W′ are numerology; still returned (auditability posture, matching the sweat-rate `implausible_result` precedent) but flagged. History anchors carry the same flag per anchor. Not a null-gate: a mediocre fit on clean data is information; hiding it would just move the question.

### D4 — Regression fixture is the bug's shape
Tests seed a completed run with a power series (450–540 W spikes, ~300 W sustained — the real signature) beside modest bike efforts and assert every affected endpoint ignores the run: curve contributions, CP fit points, profile anchors, durability tiers. Plus the mirror: a bike's speed rows never enter a run pace curve.

## Risks / Trade-offs

- **Windows that were only populated by mis-attributed data go empty/gated** — correct behavior surfacing as regression-looking output (CP may return `insufficient_points` until real bike efforts exist in window). Expected and better than the lie.
- **Multisport exclusion** shrinks coverage for triathlon race days — accepted (D2), revisit with segment streams.

## Migration Plan

None. Rollback = revert the predicates (and re-inherit the bug).

## Open Questions

- Should `power_curve` accept `sport=bike` speed reads (bike pace curve)? Today's param semantics say metric-per-sport; left as-is.
