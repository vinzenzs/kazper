# Project Roadmap

_Generated from OpenSpec changes. Last refreshed: 2026-06-13 by the `roadmap` skill ("mirror everything" Garmin arc complete, 8/8; queue empty)._

## Implemented

| Date | Change | Summary | Implementer(s) | Commit |
|---|---|---|---|---|
| 2026-06-13 | add-garmin-history-backfill | Bounded, paced, idempotent `POST /sync/backfill` + `garmin_backfill` MCP tool replaying the enriched sync over a historical range; houses the arc's Garmin call-budget pacing. | Vinzenz Stadtmueller | [`e97e942`](https://github.com/vinzenzs/nutrition-api/commit/e97e942) |
| 2026-06-13 | add-garmin-misc-mirror | Catch-all tail: `devices`, `health-vitals` (BP + all-day HR/stress), `achievements`, plus activity ops (gear-link, export, FIT upload, rename/delete). | Vinzenz Stadtmueller | [`0c7cb0e`](https://github.com/vinzenzs/nutrition-api/commit/0c7cb0e) |
| 2026-06-13 | garmin-workout-library-mgmt | Garmin workout-library control plane; fixes the orphan-workout leak on unschedule/re-push; push-hydration-back + activity FIT-export tools. | Vinzenz Stadtmueller | [`630cebd`](https://github.com/vinzenzs/nutrition-api/commit/630cebd) |
| 2026-06-13 | add-garmin-athlete-config | Singleton `athlete-config`: FTP / threshold HR & pace / max HR / HR-zone boundaries from the Garmin profile — makes workout-detail's power/zone data interpretable. | Vinzenz Stadtmueller | [`64e4629`](https://github.com/vinzenzs/nutrition-api/commit/64e4629) |
| 2026-06-13 | add-garmin-gear-and-prs | Two inventory-shaped capabilities: `gear` (shoe/bike mileage + retirement) and `personal-records`. | Vinzenz Stadtmueller | [`e991134`](https://github.com/vinzenzs/nutrition-api/commit/e991134) |
| 2026-06-12 | add-garmin-workout-detail | Mirror Garmin per-activity detail: time-in-HR-zone / elevation / normalized power columns, per-lap splits + strength sets as child tables, weather for sweat-rate; nested-write on `/workouts/bulk`. | Vinzenz Stadtmueller | [`6967118`](https://github.com/vinzenzs/nutrition-api/commit/6967118) |
| 2026-06-12 | add-garmin-daily-energy | New `daily-summary` capability mapping Garmin TDEE (active/resting/total kcal, steps, floors, intensity minutes) as an EA context signal — Loucks formula untouched. | Vinzenz Stadtmueller | [`b740059`](https://github.com/vinzenzs/nutrition-api/commit/b740059) |
| 2026-06-12 | extend-recovery-fitness | Additive columns on the recovery/fitness snapshots: SpO2, respiration, sleep stages, endurance/hill score, fitness age, `training_status`. | Vinzenz Stadtmueller | [`9d27c3c`](https://github.com/vinzenzs/nutrition-api/commit/9d27c3c) |
| 2026-06-12 | fix-chat-tool-status-chips | Fix the companion chat tool chips: the `tool` SSE event carries the `tool_use` id, the app coalesces by it (one chip per call, running→done) and labels by tool name. | Vinzenz Stadtmueller | [`4c39f3c`](https://github.com/vinzenzs/nutrition-api/commit/4c39f3c) |
| 2026-06-12 | add-workout-reconciliation | Merge a completed Garmin import into its matching planned workout (planned→completed in place, keeping the prescription), with `needs_link` + fulfill/unfulfill. | Vinzenz Stadtmueller | [`ccc2b08`](https://github.com/vinzenzs/nutrition-api/commit/ccc2b08) |
| 2026-06-12 | add-companion-session-list | Flutter chat session-history screen — list, resume, and start conversations against the session-backed `/chat`. | Vinzenz Stadtmueller | [`c98f72d`](https://github.com/vinzenzs/nutrition-api/commit/c98f72d) |
| 2026-06-12 | add-plan-slot-targets | Per-slot target overrides so one template progresses across the plan (e.g. a tempo run at 7:30→7:15→7:00); `GET /workouts/{id}/program` exposes resolved steps. | Vinzenz Stadtmueller | [`4d62851`](https://github.com/vinzenzs/nutrition-api/commit/4d62851) |
| 2026-06-12 | add-chat-sessions | Make `/chat` stateful: conversations persist as resumable server-side sessions instead of every client carrying the full transcript. | Vinzenz Stadtmueller | [`1931ca4`](https://github.com/vinzenzs/nutrition-api/commit/1931ca4) |
| 2026-06-12 | add-garmin-scheduling | The write-to-watch edge: compile a planned workout's steps into a structured Garmin workout and schedule it on the calendar (push/unschedule/read). | Vinzenz Stadtmueller | [`1066e53`](https://github.com/vinzenzs/nutrition-api/commit/1066e53) |
| 2026-06-12 | add-training-plan | The 18-week plan as plan→weeks→slots→template with an idempotent `materialize` into planned workouts; retires `Plan.md`. | Vinzenz Stadtmueller | [`d2cc452`](https://github.com/vinzenzs/nutrition-api/commit/d2cc452) |
| 2026-06-12 | add-workout-templates | The ~40-session workout library (`WORKOUT_DEFS`) as structured steps — intervals/zones/repeats — in JSONB. | Vinzenz Stadtmueller | [`e7225ad`](https://github.com/vinzenzs/nutrition-api/commit/e7225ad) |
| 2026-06-12 | add-garmin-mcp-login | Re-link Garmin from chat: backend login proxy + MCP `garmin_login`/`garmin_submit_mfa`, since the token expires ~yearly. | Vinzenz Stadtmueller | [`19b7558`](https://github.com/vinzenzs/nutrition-api/commit/19b7558) |
| 2026-06-12 | add-garmin-bridge | Python garmin-bridge service owning all Garmin auth/fetch, mapping results to the REST API on a schedule. | Vinzenz Stadtmueller | [`26071ec`](https://github.com/vinzenzs/nutrition-api/commit/26071ec) |
| 2026-06-12 | add-garmin-auth-token | Encrypted single-row Garmin token store + dedicated `garmin` auth identity to unblock the bridge. | Vinzenz Stadtmueller | [`345dc9e`](https://github.com/vinzenzs/nutrition-api/commit/345dc9e) |
| 2026-06-11 | add-shopping-list | The agent merges/dedupes ingredients across planned days; the API stores the resulting checklist primitive. | Vinzenz Stadtmueller | [`1de88cd`](https://github.com/vinzenzs/nutrition-api/commit/1de88cd) |
| 2026-06-11 | add-recipe-ingredients | Capture Cookidoo `recipeIngredient` arrays at import so a shopping list can be derived from recipes. | Vinzenz Stadtmueller | [`78f547b`](https://github.com/vinzenzs/nutrition-api/commit/78f547b) |
| 2026-06-11 | add-race-fueling-plan | A durable race entity + per-leg race-day fuelling plan. | Vinzenz Stadtmueller | [`34e2f36`](https://github.com/vinzenzs/nutrition-api/commit/34e2f36) |
| 2026-06-11 | add-meal-plan | A selection primitive between "recommended" and "eaten" — planned meals + the eaten→real-meal transition. | Vinzenz Stadtmueller | [`4686f10`](https://github.com/vinzenzs/nutrition-api/commit/4686f10) |
| 2026-06-11 | add-flutter-companion-app | The mobile client — offline-first barcode/photo/quick meal logging. | Vinzenz Stadtmueller | [`61568d9`](https://github.com/vinzenzs/nutrition-api/commit/61568d9) |
| 2026-06-11 | add-companion-food-picker | Scan / recent / search / quick-create food picker in the companion app. | Vinzenz Stadtmueller | [`127a935`](https://github.com/vinzenzs/nutrition-api/commit/127a935) |
| 2026-06-11 | add-companion-chat | In-app chat → pick a meal → Today card → "ate it" → consolidated shopping list. | Vinzenz Stadtmueller | [`c57c58e`](https://github.com/vinzenzs/nutrition-api/commit/c57c58e) |
| 2026-06-11 | add-chat-backend | Server-side Anthropic SSE agent loop reachable from the phone, grounded on the same DB. | Vinzenz Stadtmueller | [`9938907`](https://github.com/vinzenzs/nutrition-api/commit/9938907) |
| 2026-06-10 | widen-workout-ingestion | Accept distance/power/temperature/sweat-loss + brick session grouping on `/workouts`. | Vinzenz Stadtmueller | [`6952e61`](https://github.com/vinzenzs/nutrition-api/commit/6952e61) |
| 2026-06-10 | add-hydration-balance-metrics | Store Garmin sweat-loss + activity-intake daily water-balance signals. | Vinzenz Stadtmueller | [`46978a2`](https://github.com/vinzenzs/nutrition-api/commit/46978a2) |
| 2026-06-10 | add-garmin-daily-metrics | Give Garmin's daily recovery/fitness streams a home in the API. | Vinzenz Stadtmueller | [`60a13e8`](https://github.com/vinzenzs/nutrition-api/commit/60a13e8) |
| 2026-06-10 | add-deployment-pipeline | Ship the API off localhost — a real deployment pipeline. | Vinzenz Stadtmueller | [`8bad270`](https://github.com/vinzenzs/nutrition-api/commit/8bad270) |
| 2026-06-09 | add-workout-rpe-and-gi | `gi_distress_score` + `rpe` on workouts to iterate race-fueling rehearsal. | Vinzenz Stadtmueller | [`303cd60`](https://github.com/vinzenzs/nutrition-api/commit/303cd60) |
| 2026-06-09 | add-training-phases-and-templates | Training phases (build/peak/recovery) as the trunk + goal templates as the leaves. | Vinzenz Stadtmueller | [`8e51019`](https://github.com/vinzenzs/nutrition-api/commit/8e51019) |
| 2026-06-09 | add-rolling-window-summaries | Multi-day rolling-average summaries (most nutrition guidance is an average, not a daily total). | Vinzenz Stadtmueller | [`e8c33b6`](https://github.com/vinzenzs/nutrition-api/commit/e8c33b6) |
| 2026-06-09 | add-recommend-workout-fuel | "I have a 90-min Z2 ride tomorrow — what should I eat before, during, and after?" | Vinzenz Stadtmueller | [`66a2085`](https://github.com/vinzenzs/nutrition-api/commit/66a2085) |
| 2026-06-09 | add-protein-distribution | Per-meal protein distribution vs the ~0.3 g/kg MPS threshold, not just the daily total. | Vinzenz Stadtmueller | [`8e51019`](https://github.com/vinzenzs/nutrition-api/commit/8e51019) |
| 2026-06-09 | add-meal-from-photo | Photo-of-meal logging in the companion app. | Vinzenz Stadtmueller | [`68d0f0c`](https://github.com/vinzenzs/nutrition-api/commit/68d0f0c) |
| 2026-06-09 | add-daily-context-aggregator | One call bundling the agent's 5–7-call morning context ritual. | Vinzenz Stadtmueller | [`8e51019`](https://github.com/vinzenzs/nutrition-api/commit/8e51019) |
| 2026-06-08 | add-workouts-capability | The workouts primitive — "what was I doing yesterday between 6:00 and 7:30?" | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-08 | add-workout-fuel | Sodium / carbs-per-hour / caffeine taken during endurance work. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-weight-log | Record a weight measurement taken any way other than Garmin. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-meal-workout-link | Link meals to a workout's pre/intra/post window. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-last-logged-quantity | Pre-fill the last logged gram amount so barcode-scan logging stays 2 taps. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-08 | add-hydration-tracking | Log fluid intake (ml) — the missing "what did I drink?" half. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-energy-availability | Energy Availability composition over meals + workouts + bodyweight (Loucks bands). | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-date-varying-goals | Per-date goal overrides — training day ≠ rest day in kcal/carbs. | Vinzenz Stadtmueller | [`5d141a1`](https://github.com/vinzenzs/nutrition-api/commit/5d141a1) |
| 2026-06-08 | add-carb-load-auto-apply | Auto-apply the computed carb-load schedule into goal overrides (no manual loop). | Vinzenz Stadtmueller | [`4026c8e`](https://github.com/vinzenzs/nutrition-api/commit/4026c8e) |
| 2026-06-07 | unify-adherence-shape | Fix three API-shape inconsistencies surfaced in MCP testing. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | streamline-local-dev | Collapse local startup into one command; Makefile → Taskfile. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | harden-write-paths | Fix two write-path correctness bugs + one rough edge from MCP testing. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | daily-use-essentials | Multi-ingredient meals, B12/iron/vit-D micros, targets, and `meal_type` queries. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-swagger-cobra-viper | Cobra + Viper config/CLI structure + a Swagger/OpenAPI contract. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-race-prep-primitives | Deterministic carb-loading math the agent should never compute from scratch. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-product-management-tools | Product-hygiene tools left unaddressed by earlier MCP-testing findings. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-meal-logging-mvp | The v1 personal nutrition log — a thin Go service over Postgres + Open Food Facts. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-mcp-server | Expose the REST endpoints as MCP tools the coaching agent can call. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |
| 2026-06-07 | add-cookidoo-importer | Chrome extension importing Cookidoo (Thermomix) recipes as `source=recipe` products via JSON-LD. | Vinzenz Stadtmueller | [`debcd6d`](https://github.com/vinzenzs/nutrition-api/commit/debcd6d) |

## Planned

_None — every proposed change has been implemented and archived (the "mirror everything" Garmin arc is complete, 8/8). New work starts with `/opsx:propose`._

---
_To regenerate: ask Claude "update the roadmap"._
