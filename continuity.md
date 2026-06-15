## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-15 by the `continuity` skill (`resolve-zone-targets` shipped + archived — athlete-config is now consumed; `rebrand-to-kazper` shipped, closing the chat→coach arc 4/4)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight; tree clean on `main` (only `continuity.md` + the 4 untracked proposal dirs)._

## Up next

Ordered queue — top is next to pick up.

_(empty — prioritize from the backlog below. The 4 workout-target proposals are all written but unapplied; `add-swim-pace-targets` is the natural next pick as the deferred sibling of the just-shipped `resolve-zone-targets`.)_

## Backlog

Planned changes not yet prioritized. **All four are proposed (artifacts written) but not yet applied.**

- **Workout target-model extensions** (surfaced 2026-06-15 while scoping `resolve-zone-targets`; Garmin's step editor supports these, our `Target` model doesn't):
  - `add-swim-pace-targets` — swim pace as `sec_per_100m`. `Target` only carries `sec_per_km` and the bridge's pace conversion assumes `/km`, so a "Race Pace" swim can't carry any pace target — a third of race day has no prescribable intensity. `athlete_config` already stores `threshold_swim_pace_sec_per_100m`. **Proposed 2026-06-15** — distinct `swim_pace` kind, swim-restricted, bridge converts `100/sec_per_100m`. _The deferred sibling of the now-shipped `resolve-zone-targets`._
  - `add-cadence-target` — new cross-sport `Target.kind` for cadence (Garmin "Bike Cadence" rpm / run "Cadence" spm). Without it, cadence drills/high-cadence intervals are impossible and `add-secondary-target` has nothing to pair power with. **Proposed 2026-06-15** — reuses `Low/High`, bike/run only, Garmin target type id 3; registers a `cadence` metric family for `add-secondary-target` (gated).
  - `add-secondary-target` — structural, **bike-only**: Garmin bike steps carry Primary + Secondary targets (e.g. Power Zone *and* cadence/HR band); our `Step.Target` is a single slot. Adds a second (bike-scoped) slot. **Proposed 2026-06-15** — different-metric-family pair validation, bridge emits `secondaryTarget*`. _Resolver coupling was gated on `resolve-zone-targets` — that gate has now shipped, so this is unblocked._
- **`add-multisport-structured-workouts`** — a single pushed Garmin workout with multiple sport segments + transitions (swim→T1→bike→T2→run). Today bricks/triathlons are separate single-sport rows linked by `session_group`, and the garmin-bridge builder emits one segment only. **Proposed 2026-06-15.** Scoped as **Phase 1**: new `multisport-workouts` capability (inline per-sport segments + transitions, new table), garmin-bridge multi-segment compile, compile-and-schedule action. Per-segment sport is where the now-shipped `resolve-zone-targets` bike-gate (D7) and the `add-secondary-target` bike-only rules finally apply per segment (gated tasks). **Phase 2 deferred** (separate later proposal): plan-slot/materialize integration + `multisport`/`transition` in `workouts.Sport`/`Program`.

## Notes

- **`resolve-zone-targets` SHIPPED + archived (2026-06-15).** `EffectiveProgram` now resolves zone-reference targets to absolute `power_w`/`hr_bpm` ranges from the athlete-config singleton (bike-gated power per D7, HR cross-sport, missing-config passthrough, `origin` provenance label). **`athlete-config` is no longer capture-only** — its zone boundaries are consumed as the single source of truth for workout target resolution. This is the gate the 3 remaining workout-target proposals referenced; the data back-fill of ~45 zone-ref templates is a separate coaching task that works independently.
- **The chat→coach unification arc is COMPLETE — 4/4 shipped + archived.** `expand-chat-to-coach` (unified coach: shared `internal/agenttools` registry + tiered pause/resume write-confirm + companion proposal card), `add-coach-context-endpoints` (`/context/training` + `/context/recovery` grounding reads), `unify-mcp-tool-registry` (the MCP server's full tool surface generated from `agenttools` via one generic dispatcher; only `log_meal_from_photo` stays bespoke for multipart), and **`rebrand-to-kazper`** (archived 2026-06-14 — renamed the coach persona to Kazper).
- **The "mirror everything" Garmin arc: COMPLETE — archived** (`add-garmin-{workout-detail,daily-energy,gear-and-prs,athlete-config,misc-mirror,history-backfill,sync-rolling-lookback}` + `garmin-workout-library-mgmt`, plus `extend-recovery-fitness`). Migrations 036–041 landed; head is `041` on disk. Re-verify the head before any future `task migrate:new`.
- **Recent garmin-bridge fixes archived (2026-06-15):** `fix-garmin-bridge-{athlete-config-mapping,threshold-pace-unit,training-status-mapping}`, `schedule-adhoc-yoga-mobility`, `surface-athlete-readiness-context`.
- **The PRIOR Garmin + Option B training-plan arc is COMPLETE and archived** — auth, read-import, login, workout-templates → training-plan → garmin-scheduling → plan-slot-targets → workout-reconciliation, plus `fix-chat-tool-status-chips`.
- **Drift to clean up (carried):**
  - **`main` is well ahead of `origin/main` and unpushed** — the whole prior Garmin + Option B + chat-sessions + chat→coach arc, plus today's `resolve-zone-targets`, is local-only. Push when ready.
  - **`roadmap.md` is stale** — `resolve-zone-targets`, the 2026-06-15 garmin-bridge fixes, the chat→coach arc, and the earlier Garmin arc aren't reflected; run the `roadmap` skill to refresh.
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main`) and `feat/add-recommend-workout-fuel` (leftover) — both safe to delete when convenient (this skill never prunes branches).
- **Open follow-ups (not proposed):** manual on-device smoke for `fix-chat-tool-status-chips` + `expand-chat-to-coach` phase 4 (4.6); reverse-direction workout reconciliation + ±1-day tolerance + plan-adherence analytics; a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (503 `chat_unavailable` until then); the derived sweat-rate (ml/hr) endpoint (T2 #6C); cadence/secondary-target/IF-from-FTP consumptions still deferred per the athlete-config spec.
- **Still-open priorities-flagged work** (in `openspec/priorities.md`): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence), #9 (supplement log).
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block; prefer ADDED requirements for additive intent. OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. The `openspec instructions … --json` command prints a progress line before the JSON — strip with `sed -n '/^{/,$p'` before parsing.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
