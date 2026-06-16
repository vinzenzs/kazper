## Context

`add-workout-reconciliation` shipped the **forward** path in `workouts.reconcileUpsert` (`internal/workouts/service.go`): on first-sight ingest of a `garmin` completed activity, `FindOpenPlanned(sport, startedAt, loc)` returns open planned candidates on the activity's **local day**; one → `Merge` (fulfill the planned row in place, keeping `plan_slot_id`/`template_id`), zero → standalone insert, many → standalone insert flagged `needs_link`. An explicit `POST /workouts/{id}/fulfill {completed_id}` merges an existing completed row into a planned one for the cases auto-reconcile declines.

The materialize path (`trainingplan.Materialize` → `workouts.UpsertPlannedFromSlot`, keyed on `plan_slot_id`, guarded by `status='planned'`) never reconciles: it always upserts a `planned` row for the slot. So when the completed activity is imported **before** the slot is materialized, the slot gets a fresh planned row and the activity stays a standalone orphan. The columns needed for the reverse link already exist (`plan_slot_id`, `template_id`, `external_id`, `needs_link`, `status`); only the matching + adopt logic is missing.

## Goals / Non-Goals

**Goals:**
- Materialize adopts an already-imported completed activity for a slot instead of creating a duplicate planned row.
- A ±1-day local-day tolerance (same-day preferred) for both the forward and reverse matchers, so common slippage reconciles automatically.
- One row per real session in both directions; idempotent re-materialize.

**Non-Goals:**
- Plan-adherence analytics / scoring (separate read surface).
- Changing `fulfill`/`unfulfill` — they stay the manual escape hatch for >1-candidate and >1-day cases.
- Reconciling across multiple plans, or any multi-day tolerance beyond ±1.
- A migration — no new columns.

## Decisions

### D1: Reverse reconcile lives in `workouts`, invoked by materialize
`workouts` owns the rows and the forward matcher, so the reverse logic belongs there too. Add a slot-upsert variant — `ReconcileSlotOrUpsertPlanned(ctx, tx, in PlannedSlotInput, loc)` — that runs inside materialize's existing transaction: (1) if a row already exists for `in.PlanSlotID`, take the current `UpsertPlannedFromSlot` path (status-guarded, idempotent); (2) else find adoptable completed activities and adopt one, or (3) fall back to inserting the planned row. `trainingplan.Materialize` calls this instead of `UpsertPlannedFromSlot`. Alternative (reconcile in a separate post-materialize pass) rejected — it would re-scan and risk racing the upsert; doing it inline keeps it atomic per slot.

### D2: A shared ±1-day matcher with same-day preference
Generalize candidate matching to a window `[date-1, date+1]` of **local** calendar days for the exact sport, used by both directions:
- Forward: `FindOpenPlanned` matches planned rows within ±1 day of the **activity's** local day.
- Reverse: `FindAdoptableCompleted` matches completed rows (`status='completed'`, `plan_slot_id IS NULL`, `external_id IS NOT NULL`) within ±1 day of the **slot's** local day.
**Same-day preference**: if any candidate falls on the exact day, only same-day candidates count toward the match; adjacent-day candidates are considered only when there is no same-day candidate. This keeps "one same-day + one adjacent-day" a clean single match (same-day wins) rather than an ambiguous decline.

### D3: Decline on ambiguity, never guess
Reverse, like forward, acts only on **exactly one** candidate (after same-day preference). Zero → create the planned row (current behavior). More than one → create the planned row and leave the completed activities standalone (and, as today, the forward path may have already flagged them `needs_link`); the user resolves via explicit `fulfill`.

### D4: Adoption keeps the activity, gains the plan links
Adopting sets the completed row's `plan_slot_id` and `template_id` (from the slot), clears `needs_link`, and leaves `status='completed'` and the activity's own name + actual metrics intact (it is the real session). This mirrors the forward end-state (one completed row carrying `plan_slot_id`/`template_id`), differing only in which row survives — there, the planned row; here, the imported activity. No second row is created.

### D5: Idempotency via the existing slot key
Re-materialize: the slot already has a row keyed by `plan_slot_id` (the adopted activity or a prior planned row), so step (1) of D1 runs the existing `status='planned'`-guarded upsert — a completed/adopted row is skipped, a still-`planned` row is updated in place. The reverse adopt path runs only on the first materialize of a slot, so it cannot re-adopt or duplicate.

## Risks / Trade-offs

- **A ±1-day window can mis-adopt a genuinely different session** (e.g. two easy runs on consecutive days, only one planned) → Mitigated by exact-sport + same-day preference + exactly-one-candidate; the window only widens the match when there is a single unambiguous completed activity nearby. The explicit `unfulfill` endpoint reverses a bad adoption.
- **Materialize now does extra reads per slot** (the adoptable-completed query) → One indexed query per slot on first materialize only (skipped once the slot has a row); negligible for a week's worth of slots.
- **Behavior change to the forward matcher** (exact-day → ±1-day) → A documented widening of an existing shipped requirement; same-day preference keeps prior same-day matches identical, so only the previously-declined cross-day-by-one case changes (now auto-reconciled instead of standalone).

## Migration Plan

Pure code change, no DB migration. Deploy ships the matcher + reverse path. Rollback = revert; existing links are untouched (the change only adds links it would otherwise have left for manual `fulfill`).

## Open Questions

- Should adoption adopt the **template's** name over the Garmin activity name for consistency with planned rows? Lean: keep the activity name (it is the real session); revisit if the coach UI wants the planned label.
- Should the tolerance be configurable per plan/sport rather than a fixed ±1? Defer until a real case needs more than ±1.
