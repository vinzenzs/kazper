## Context

Workouts carry sport/status/metrics but nothing distinguishing a trainer session from a road one. Garmin knows (activity types like `indoor_cycling`, `virtual_ride`, `treadmill_running`, `lap_swimming`) and the bridge already maps activity types to Kazper sports — the environment axis was simply dropped. The heat arc (heat-adjusted targets, acclimatization, temperature analytics) needs it as a first-class field.

## Goals / Non-Goals

**Goals:** one nullable enum, derived automatically for Garmin workouts, manually settable everywhere, honest null.

**Non-Goals:** template/plan-slot environment defaults (later nicety); inferring environment for pre-existing rows from weather-field absence (heuristic back-fill invites wrong labels — null is honest; re-syncs fill the recent window); any consumer logic (the following changes own that).

## Decisions

### D1 — Nullable enum, no back-fill, `training_focus` conventions throughout
`environment TEXT NULL CHECK IN ('indoor','outdoor')`; POST/bulk accept it; PATCH is tri-state via the empty-string sentinel; `omitempty` on the wire; `environment_invalid` at the boundary. Null means "not stated" and downstream heat logic must treat it as *assumed outdoor, flagged* (their design decision, restated here so the field's semantics are set once).

### D2 — Bridge derives from activity type, defensively
A small mapping table over Garmin's `activityType` keys: indoor cycling/virtual rides/treadmill/indoor rows/pool swims → `indoor`; road/gravel/mtb/open-water/outdoor runs → `outdoor`; unrecognized → omitted. Pool swims are `indoor` deliberately: the field answers "does ambient weather apply", not "is there a roof". The mapping is data, unit-tested, and an unknown key can never fail a sync (the `directPower` posture).

### D3 — Re-sync clobber accepted
The bulk upsert full-replaces; a manual override on a Garmin workout survives only until the next re-sync of that day. Same accepted caveat as `training_focus`, same escape hatch if it ever bites (COALESCE on the upsert).

## Risks / Trade-offs

- **Ambiguous types** (e.g. a generic "cycling" done on rollers) mis-derive to outdoor — correctable via PATCH; the mapping only claims what the type states.

## Migration Plan

Migration on the next free slot (verify head; `064` at proposal time). Down drops the column. No data movement.

## Open Questions

- Template-level environment default (a "trainer intervals" template implying indoor for its planned workouts) — deferred until the heat read shows how often planned-session PATCHing annoys.
