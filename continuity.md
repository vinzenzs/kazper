## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-16 by the `continuity` skill (`plan-adherence-analytics` + `add-companion-train-screen` shipped & archived; `openspec/changes/` is empty — nothing in flight or queued; `main` is 4 ahead of `origin/main`)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight; `openspec/changes/` is empty._

## Up next

Ordered queue — top is next to pick up.

_(empty — no open proposals. Next change must be proposed first via `/opsx:propose`.)_

## Backlog

Planned changes not yet prioritized.

- _No open proposals._ Future-but-unproposed candidate seams if needed: multisport "Phase 4" niceties (per-segment duration in the template view) and the still-open priorities-flagged items below.

## Notes

- **The workout-target arc is COMPLETE — 5/5 shipped + archived** (surfaced 2026-06-15 from the Garmin step-editor screenshots):
  - `resolve-zone-targets` (archived 2026-06-15) — the engine: `EffectiveProgram` resolves zone-reference targets to absolute `power_w`/`hr_bpm` from the athlete-config singleton (bike-gated power per D7, HR cross-sport, missing-config passthrough, `origin` provenance). **`athlete-config` is no longer capture-only.**
  - `add-swim-pace-targets` (archived 2026-06-15) — distinct `swim_pace` kind, sec/100m, swim-restricted, bridge converts `100/sec_per_100m`.
  - `add-secondary-target` (archived 2026-06-16) — bike-only Primary+Secondary target (different metric family); resolver resolves a zone-kind secondary; bridge emits `secondaryTarget*`.
  - `add-cadence-target` (archived 2026-06-16) — cross-sport `cadence` kind (bike rpm / run spm), Garmin target type id 3 (`cadence.zone`), registered as a metric family for secondary-target.
  - `add-multisport-structured-workouts` (archived 2026-06-16) — **Phase 1**: new `multisport-workouts` capability (per-sport segments + transitions, `multisport_templates` table, migration `045`), garmin-bridge multi-segment compile, compile-and-schedule action.
  - `multisport-phase-2` (archived 2026-06-16) — **plan integration**: a slot references a single-sport OR multisport template (XOR), materialize emits a `sport='multisport'` planned workout, and `EffectiveProgram` returns per-segment programs resolved **by each segment's sport** (the per-segment resolution Phase 1's D7 deferred) — pushed through the same effective-program→bridge path. Migration `046`. Verified live via API + MCP (XOR validation, materialize idempotency, bike `power_zone→power_w`/run `hr_zone→hr_bpm`/swim passthrough).
  - `multisport-phase-3` (archived 2026-06-16) — **read-time polish**: a multisport template's response carries a derived `estimated_duration_sec` (summed segment durations; null when not fully time-bounded), and the `/context/training` recent-load `by_sport` summary decomposes a `multisport` workout into its segment sports (brick credits swim/bike/run) with graceful fallback. No migration, no new tools.
  - The data back-fill of ~45 zone-ref templates remains a separate coaching task that works independently.
- **`derive-intensity-factor-from-ftp` (archived 2026-06-16) — the last athlete-config consumption.** Bike workouts now derive `intensity_factor = normalized_power_w / ftp_watts` when the watch didn't supply one. With this, the previously-deferred IF-from-FTP consumption is **done** — `athlete-config` no longer has an unconsumed zone/FTP field.
- **`reverse-direction-workout-reconciliation` (archived 2026-06-16) — closes the reconcile loop.** Auto-reconciliation was forward-only (at activity ingest); now **materialize** also adopts a matching unlinked completed activity (so an activity imported before its plan no longer orphans into a duplicate), and a **±1-day tolerance** (same-day preferred) covers cross-day-by-one slippage in both directions. No migration, no new tools; `fulfill`/`unfulfill` stay the manual escape hatch for the >1-candidate/>1-day cases.
- **`plan-adherence-analytics` (archived 2026-06-16) — reads back how well the plan was followed.** New `GET /workouts/adherence` window read: completed/missed/upcoming/unplanned counts, `adherence_rate` over due sessions only (null when none due), planned-vs-actual duration & TSS, `by_sport`; optional `plan_id` scoping via a `plan_slots`/`plan_weeks` join. Pure read (no migration), mirrored as the `workout_adherence` MCP tool. Open follow-up (deferred per spec): a `missed_sessions` list so the coach can name them, and a per-week trend series.
- **`add-companion-train-screen` (archived 2026-06-16) — Flutter companion.** Read-only "Train" screen (fuel-the-training lens) in `apps/companion`; modifies the `mobile-companion` spec. Separate Flutter workstream, committed by its owner (`7e9edb0` + archive `618cc55`).
- **The chat→coach unification arc is COMPLETE — 4/4 shipped + archived.** `expand-chat-to-coach`, `add-coach-context-endpoints`, `unify-mcp-tool-registry`, and `rebrand-to-kazper` (archived 2026-06-14).
- **The "mirror everything" Garmin arc: COMPLETE — archived** (`add-garmin-{workout-detail,daily-energy,gear-and-prs,athlete-config,misc-mirror,history-backfill,sync-rolling-lookback}` + `garmin-workout-library-mgmt`, plus `extend-recovery-fitness`). Migrations 036–041 landed; multisport Phase 1 added `045_add_multisport_templates` and **`multisport-phase-2` added `046_add_multisport_plan_integration`, so head is now `046` on disk.** Re-verify the head before any future `task migrate:new`.
- **Recent garmin-bridge fixes archived (2026-06-15):** `fix-garmin-bridge-{athlete-config-mapping,threshold-pace-unit,training-status-mapping}`, `drop-phantom-swim-threshold-mapping`, `schedule-adhoc-yoga-mobility`, `surface-athlete-readiness-context`.
- **The PRIOR Garmin + Option B training-plan arc is COMPLETE and archived** — auth, read-import, login, workout-templates → training-plan → garmin-scheduling → plan-slot-targets → workout-reconciliation, plus `fix-chat-tool-status-chips`.
- **Drift to clean up (carried):**
  - **`main` is 4 ahead of `origin/main`** — the `reverse-direction-workout-reconciliation` docs sync, both `plan-adherence-analytics` commits, and the `add-companion-train-screen` feat+archive are local-only. Push when ready. (Only `continuity.md` is uncommitted in the working tree.)
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main`) and `feat/add-recommend-workout-fuel` (leftover) — both safe to delete when convenient (this skill never prunes branches).
- **Open follow-ups (not proposed):** manual on-device smoke for `fix-chat-tool-status-chips` + `expand-chat-to-coach` phase 4 (4.6); a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (503 `chat_unavailable` until then); the derived sweat-rate (ml/hr) endpoint (T2 #6C); plan-adherence `missed_sessions` list + per-week trend series (deferred in the `plan-adherence-analytics` spec). _(Plan-adherence analytics, add-companion-train-screen, reverse-direction reconciliation, IF-from-FTP, and multisport Phase 3 all shipped — see Notes.)_
- **Still-open priorities-flagged work** (in `openspec/priorities.md`): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence), #9 (supplement log).
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block; prefer ADDED requirements for additive intent. **A delta authored before a sibling lands will silently drop the sibling's language on a blind replace** — the whole workout-target arc hit this on the shared garmin-bridge requirement, so each archive *merged* cadence/secondary/swim_pace/multisport rather than replacing. OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. The `openspec instructions … --json` command prints a progress line before the JSON — strip with `sed -n '/^{/,$p'` before parsing.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
