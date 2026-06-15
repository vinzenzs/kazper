## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-16 by the `continuity` skill (`add-swim-pace-targets` shipped + archived; `add-secondary-target` + `add-cadence-target` implemented on `main`, awaiting archive; only `add-multisport-structured-workouts` still unimplemented)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| add-secondary-target | `main` | 2026-06-15 | Vinzenz | **Implemented (12/12 tasks)** — awaiting `/opsx:archive`. Bike-only Primary+Secondary; resolver coupling done (resolves zone-kind secondary). |
| add-cadence-target | `main` | 2026-06-16 | Vinzenz | **Implemented (8/8 tasks)** — awaiting `/opsx:archive`. `cadence` kind (bike/run), Garmin type 3; registered as a metric family for secondary-target. |

_Both implemented directly on `main` (no feature branch). Next step is `/opsx:archive` each, then refresh `roadmap.md`._

## Up next

Ordered queue — top is next to pick up.

1. **add-multisport-structured-workouts** — `Why:` a triathlon/brick should push to the watch as one auto-advancing multisport workout (swim→T1→bike→T2→run); today every workout/template is single-sport and the bridge emits one segment, so bricks exist only as separate rows linked by `session_group`. _Why now: the last unimplemented piece of the workout-target arc, and the only place per-segment sport matters — the now-shipped `resolve-zone-targets` bike-gate and the implemented `add-secondary-target` rules apply per segment (gated tasks 5.1/5.2 are now live). Largest of the family; **Phase 1** only (library + multi-segment compile + schedule), Phase 2 plan integration deferred._

## Backlog

Planned changes not yet prioritized.

- _Empty_ — the four workout-target siblings are done or queued; multisport is the only open proposal (in Up next). Future-but-unproposed: **multisport Phase 2** (plan-slot/materialize integration + `multisport`/`transition` in `workouts.Sport`/`Program`) becomes a proposal once Phase 1 lands.

## Notes

- **The workout-target arc is nearly complete.** Five changes total, surfaced 2026-06-15 from the Garmin step-editor screenshots:
  - `resolve-zone-targets` — **SHIPPED + archived 2026-06-15** (the engine).
  - `add-swim-pace-targets` — **SHIPPED + archived 2026-06-15** (distinct `swim_pace` kind, sec/100m, swim-restricted, bridge converts `100/sec_per_100m`).
  - `add-secondary-target` — **implemented on `main`, awaiting archive** (see In progress).
  - `add-cadence-target` — **implemented on `main`, awaiting archive** (see In progress).
  - `add-multisport-structured-workouts` — proposed, unimplemented (see Up next).
- **`resolve-zone-targets` (archived 2026-06-15).** `EffectiveProgram` resolves zone-reference targets to absolute `power_w`/`hr_bpm` ranges from the athlete-config singleton (bike-gated power per D7, HR cross-sport, missing-config passthrough, `origin` provenance). **`athlete-config` is no longer capture-only.** The data back-fill of ~45 zone-ref templates is a separate coaching task that works independently.
- **The chat→coach unification arc is COMPLETE — 4/4 shipped + archived.** `expand-chat-to-coach`, `add-coach-context-endpoints`, `unify-mcp-tool-registry`, and `rebrand-to-kazper` (archived 2026-06-14).
- **The "mirror everything" Garmin arc: COMPLETE — archived** (`add-garmin-{workout-detail,daily-energy,gear-and-prs,athlete-config,misc-mirror,history-backfill,sync-rolling-lookback}` + `garmin-workout-library-mgmt`, plus `extend-recovery-fitness`). Migrations 036–041 landed; head is `041` on disk. Re-verify the head before any future `task migrate:new` — **note: `add-multisport-structured-workouts` Phase 1 adds a `multisport_templates` table, the next migration.**
- **Recent garmin-bridge fixes archived (2026-06-15):** `fix-garmin-bridge-{athlete-config-mapping,threshold-pace-unit,training-status-mapping}`, `drop-phantom-swim-threshold-mapping`, `schedule-adhoc-yoga-mobility`, `surface-athlete-readiness-context`.
- **The PRIOR Garmin + Option B training-plan arc is COMPLETE and archived** — auth, read-import, login, workout-templates → training-plan → garmin-scheduling → plan-slot-targets → workout-reconciliation, plus `fix-chat-tool-status-chips`.
- **Drift to clean up (carried):**
  - **`main` is well ahead of `origin/main` and unpushed** — the whole prior arc plus `resolve-zone-targets`, `add-swim-pace-targets`, `add-secondary-target`, and `add-cadence-target` is local-only. Push when ready.
  - **`add-multisport-structured-workouts/` is untracked** (`??` in `git status`) — the proposal dir isn't committed yet.
  - **`roadmap.md` is stale** — `resolve-zone-targets`, `add-swim-pace-targets`, the 2026-06-15 garmin-bridge fixes, the chat→coach arc, and the earlier Garmin arc aren't reflected; run the `roadmap` skill to refresh (and again after archiving secondary + cadence).
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main`) and `feat/add-recommend-workout-fuel` (leftover) — both safe to delete when convenient (this skill never prunes branches).
- **Open follow-ups (not proposed):** manual on-device smoke for `fix-chat-tool-status-chips` + `expand-chat-to-coach` phase 4 (4.6); reverse-direction workout reconciliation + ±1-day tolerance + plan-adherence analytics; a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (503 `chat_unavailable` until then); the derived sweat-rate (ml/hr) endpoint (T2 #6C); IF-from-FTP consumption still deferred per the athlete-config spec (cadence + secondary-target consumptions are now done).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence), #9 (supplement log).
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block; prefer ADDED requirements for additive intent. OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. The `openspec instructions … --json` command prints a progress line before the JSON — strip with `sed -n '/^{/,$p'` before parsing.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
