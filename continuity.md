## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-13 by the `continuity` skill (the "mirror everything" Garmin arc is **COMPLETE — 8/8 shipped + archived**; the queue is empty)._

## In progress

| Change | Branch | Started | Owner | Notes |
|---|---|---|---|---|
| _(none)_ | | | | |

_Nothing in flight; tree clean. **The "mirror everything" Garmin arc is fully shipped and archived** (8/8). `main` is far ahead of `origin/main` and unpushed._

## Up next

Ordered queue — top is next to pick up.

- _Empty — every proposed change has shipped. Next pickup comes from `openspec/priorities.md` (`/opsx:propose <slug>`), or one of the still-open follow-ups in Notes below._

## Backlog

Planned changes not yet prioritized.

- _Empty._

## Notes

- **The "mirror everything" Garmin arc: COMPLETE — 8/8 shipped + archived.** B `add-garmin-workout-detail` (`6967118`), A `add-garmin-daily-energy` (`b740059`), C `extend-recovery-fitness` (`9d27c3c`), D `add-garmin-gear-and-prs` (`e991134`), F `add-garmin-athlete-config` (`64e4629`), E `garmin-workout-library-mgmt` (`630cebd`), G `add-garmin-misc-mirror` (`0c7cb0e`), and `add-garmin-history-backfill` (`e97e942`). Garminconnect's whole surface is now mirrored into the API/MCP. Decision provenance lives in each change's archived `design.md`.
  - **Gap-closure pass (after the initial 5):** weather (humidity/wind, sweat-rate) + the reconcile-seam guarantee were folded into **B**; **F** (athlete-config) makes B's IF/zone data interpretable; the **backfill** change sweeps mid-season history older than the rolling CronJob window and houses the Garmin call-budget pacing; **G** is the honest catch-all so "mirror everything" doesn't overclaim. The low-value tail G still excludes is listed in its proposal ("Deliberately still excluded": menstrual/pregnancy, social/leaderboard, per-second streams).
  - **Migrations 036–041 all landed** in order (036 B, 037 A, 038 C, 039 D, 040 F, 041 G; E + backfill carried none). Head is `041` on disk.
  - **Worth a post-arc verification pass** (the arc shipped fast via concurrent implementation): confirm E's FIT-export body-cap raise actually shipped (the control proxy's 16 KB cap vs the ~8 MB export blob, base64-in-JSON), and that E's orphan-workout fix wired both leak points (`unschedule` and re-push/`pushOne` in `scheduling.go`). Spot-check `task test` green across the new garmin packages if not already done.
  - **A's design boundary:** the Loucks EA formula is untouched (`(intake − exercise burn)/FFM`); Garmin TDEE is surfaced only as context in `daily-summary`, never merged into `summary` Totals (unit isolation). "EA NEAT-enrichment" is an explicit follow-up, not in scope.
- **The PRIOR Garmin integration + Option B training-plan arc is COMPLETE and archived** — auth, read-import, login, workout-templates → training-plan → garmin-scheduling → plan-slot-targets → workout-reconciliation, plus `fix-chat-tool-status-chips`. This new arc deepens that foundation (depth + breadth) rather than re-plumbing it. Coaching synthesis stays the chat agent's job, not an API endpoint.
- **Drift to clean up (carried):**
  - **`main` is well ahead of `origin/main` and unpushed** — the whole prior Garmin + Option B + chat-sessions arc is local-only. Push when ready.
  - **Stale branches to prune:** `feat/add-chat-sessions` (now == `main`) and `feat/add-recommend-workout-fuel` (leftover) — both safe to delete when convenient (this skill never prunes branches).
- **Open follow-ups from prior arcs (not proposed):** manual on-device smoke for `fix-chat-tool-status-chips` (task 4.3); reverse-direction workout reconciliation + ±1-day tolerance + plan-adherence analytics; manual companion-chat e2e; a real-Anthropic `/chat` smoke once `ANTHROPIC_API_KEY` is set (503 `chat_unavailable` until then).
- **Still-open priorities-flagged work** (in `openspec/priorities.md`, independent of this arc): T2 #6E (retroactive freeform→product correction), #6F (`coach_recommendation` persistence), #9 (supplement log); the derived sweat-rate (ml/hr) endpoint completing T2 #6C — now **buildable**: `add-garmin-workout-detail` (B) has shipped, supplying its missing inputs (time-in-zone, elevation, weather).
- **Pattern notes (carried):** MODIFIED spec deltas are full-replace — copy prior scenarios into the MODIFIED block; prefer ADDED requirements for additive intent. OpenSpec requirement bodies must lead with a SHALL/MUST sentence or `validate --strict` rejects them. Migration head is `041` on disk (the arc's 036–041 all shipped). Re-verify the head before any future `task migrate:new`. The `openspec instructions … --json` command prints a progress line before the JSON — strip with `sed -n '/^{/,$p'` before parsing.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For the historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
