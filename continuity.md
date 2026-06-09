## Continuity

_Forward plan for OpenSpec changes. Tracks **what's next**, **what's in flight**, and **what's queued**._
_Companion to `openspec/priorities.md` (tier/triage framing) — this file is the operational queue._
_Last refreshed: 2026-06-09 by the `continuity` skill (re-verified — queue reprioritized: `add-recommend-workout-fuel` + `add-rolling-window-summaries` promoted to Up next; `add-meal-from-photo` + `add-flutter-companion-app` demoted to Backlog)._

## In progress

This repo currently commits implementation work directly to `main` (single-user
project, no feature-branch flow yet). The Branch column reflects that — switch
to per-change branches when the cadence makes it worth it.

_Nothing in flight — pick from Up next._

## Up next

Ordered queue — top is next to pick up.

1. **add-recommend-workout-fuel** _(new 2026-06-09)_ — `plan_carb_load` answers "how should I eat in the 1–4 days *before* my race." `daily_summary` + the phase template answer "what's my macro target for *today's training block*." `workout_fueling_summary` answers "what *did* I take during this ride." None of them answer the *forward-looking* question for everyday training: "what should I eat before/during/after tomorrow's 90-min Z2 ride?" Closes T2 #10.

2. **add-rolling-window-summaries** — `GET /summary/rolling?anchor_date=…&window_days=N`. Multi-day averages for the metrics that are actually multi-day phenomena: protein for MPS (~1.6–2.2 g/kg/day across a week), Energy Availability (5–14 day Loucks bands), 72-hour carb-load window, weekly sodium baseline. One bad day is noise; the rolling view is the signal. _Implemented + committed (8612f56) but not yet archived — pending `/opsx:archive add-rolling-window-summaries` and the §11.3 manual e2e._

## Backlog

Planned changes not yet prioritized.

- **add-meal-from-photo** — Backend-mediated Claude Vision integration so the Flutter app can log a meal from a photo. Mirrors the `off-integration` pattern (one API key, server-side; clients stay simple). _Why now: independent backend feature; the Flutter app's #2 killer interaction; touches different surfaces from the recent fueling + aggregator work._

- **add-flutter-companion-app** — Three-screen Flutter app (barcode / photo / hydration widget) as a focused supplement to the agent. _Caveat: predates the endurance-training pivot — see Meta #3 in `openspec/priorities.md`. Worth a short explore session on whether the three killer interactions still match today's most-pressing use._

## Notes

- **T1 list fully delivered**: #1, #1A, #1B, #2, #3, #4, #5 all shipped by 2026-06-09. Tier-2 work is the active surface.
- **T2 closures so far (all 2026-06-09)**: #6B (`daily_context` aggregator), #7 (`protein_distribution`). #1B (`rolling_summary`) still pending archive even though implementation has landed (committed 8612f56).
- **Pending-archive choreography**: `add-rolling-window-summaries` is coded + tasks-ticked + committed but the openspec dir hasn't moved to `archive/` yet. Quick `/opsx:archive add-rolling-window-summaries` would close it out.
- **Uncommitted archive moves**: the recent `add-protein-distribution`, `add-daily-context-aggregator`, and `add-training-phases-and-templates` archives are all still uncommitted in the working tree (their `mv`s plus the synced main specs). A consolidated cleanup commit would close them out in one pass.
- **Decisions pending** (do not queue yet): T2 #6F (`coach_recommendation` persistence) — tests the synthesis principle, deliberate discussion first.
- **Remaining priorities-flagged work**: T2 #6A (sleep/HRV log), T2 #6C (sweat-rate test workflow), T2 #6D (GI distress / RPE on workout fueling), T2 #6E (retroactive freeform→product correction), T2 #8 (caffeine), T2 #9 (supplement log). T2 #6C is the cheapest now that workouts + weight + workout-fuel all exist; T2 #6A is the natural pair-with-weight "morning metrics" expansion.

---
_To update: ask Claude "update continuity", "queue X next", or "start work on X"._
_For tier/triage and "why does this matter" framing, see [`openspec/priorities.md`](openspec/priorities.md)._
_For historical record of implemented changes, see [`roadmap.md`](roadmap.md)._
