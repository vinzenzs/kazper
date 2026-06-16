## Why

Auto-reconciliation today is **one-directional**: it runs only when a completed Garmin activity is ingested, matching it against an already-existing open planned workout (exact sport + local day). But the daily Garmin sync often imports a completed activity **before** that day's plan is materialized — or a planned workout is added after the fact — so the forward match finds nothing and inserts a standalone completed row. When the planned workout later materializes, nothing re-runs reconciliation: the plan slot gets a fresh `planned` row while the real, already-imported activity sits beside it as an orphan. The athlete then has two rows for one session and has to fix it by hand via the explicit `fulfill` endpoint. The same manual fix is needed when an activity lands a day off the planned date (a Sunday long run done Saturday night), which the exact-day matcher declines. This change adds the missing reverse direction (reconcile at materialize) and a ±1-day tolerance so the common timing/slippage cases reconcile automatically.

## What Changes

- **Reverse reconciliation at materialize**: when a plan slot is materialized, before inserting a fresh `planned` row the system SHALL look for exactly one **adoptable completed activity** — `status='completed'`, `plan_slot_id IS NULL`, an `external_id` present (a real import), the slot's sport, within the day tolerance — and **adopt** it: set its `plan_slot_id` and `template_id`, clear `needs_link`, and keep its completed actuals. No duplicate planned row is created. On no match, materialize creates the `planned` row exactly as today. On more than one candidate it declines (creates the planned row, leaves the completed rows standalone) rather than guess.
- **±1-day tolerance** for both directions: the forward (ingest-time) match and the new reverse (materialize-time) match SHALL consider a planned/completed pair when their local calendar days differ by **at most one day**, not only exact-day — still requiring an exact sport match and exactly one candidate. The same-day candidate SHALL be preferred when both a same-day and an adjacent-day candidate exist.
- **Idempotency preserved**: once a slot is linked to a completed row (adopted or fulfilled), re-materialize follows the existing `plan_slot_id`-keyed, `status='planned'`-guarded path and neither re-adopts nor duplicates.
- Out of scope: plan-adherence analytics (a separate read/scoring surface), reconciling across multiple plans, and any change to the explicit `fulfill`/`unfulfill` endpoints (they remain the manual escape hatch for the >1-candidate and >1-day cases).

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `workouts`: the reconciliation requirement gains a ±1-day matching tolerance (preferring same-day) and a **reverse** rule — a materialized plan slot adopts a matching unlinked completed activity instead of creating a duplicate planned row.
- `training-plan`: materialize reconciles each slot against an existing adoptable completed activity (linking it) before falling back to creating a planned row.

## Impact

- **Code**: `internal/workouts/` (a slot-reconcile path alongside `reconcileUpsert`/`UpsertPlannedFromSlot` — find-adoptable-completed within the transaction, then adopt-or-insert; extend the candidate matcher to ±1 day with same-day preference, shared by both directions), `internal/trainingplan/` (materialize calls the reverse-reconcile slot upsert), `internal/httpserver/server.go` wiring if a new dependency edge is needed.
- **Migration**: none — reuses existing columns (`plan_slot_id`, `external_id`, `needs_link`, `status`).
- **Docs**: `task swag` if any response/request shape shifts (none expected; behavior-only).
- **Coupling**: extends the `add-workout-reconciliation` forward path; orthogonal to the multisport work (completed bricks remain `session_group` single-sport rows and won't match a `multisport` slot).
