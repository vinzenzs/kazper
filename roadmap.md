# Project Roadmap

_Generated from OpenSpec changes. Last refreshed: 2026-06-16 by the `roadmap` skill (74 implemented, 1 planned)._

_All changes authored by Vinzenz Stadtmueller. Commits link to `github.com/vinzenzs/kazper` (some are local-only until `main` is pushed)._

## Implemented

| Date | Change | Summary | Commit |
|---|---|---|---|
| 2026-06-16 | add-secondary-target | Garmin bike steps carry a Primary + Secondary target (e.g. Power Zone *and* cadence/HR) but `Step.Target` was a single slot; adds a bike-only secondary target (different metric family). | [`526daf3`](https://github.com/vinzenzs/kazper/commit/526daf3) |
| 2026-06-16 | add-cadence-target | Garmin offers a cadence step target on bike (rpm) and run (spm) for drills/intervals and as the classic secondary pairing with power; adds a cross-sport `cadence` target kind. | [`4d730d4`](https://github.com/vinzenzs/kazper/commit/4d730d4) |
| 2026-06-15 | add-swim-pace-targets | Swim pace was unexpressible — `Target` only carried sec/km but swims are prescribed in sec/100m; adds a `swim_pace` kind (swim-restricted; bridge converts `100/sec_per_100m`). | [`025d9a5`](https://github.com/vinzenzs/kazper/commit/025d9a5) |
| 2026-06-15 | surface-athlete-readiness-context | When the coach grounds on `GET /context/training`, it has no view of the athlete's physiology — surfaces `athlete_config` readiness context. | [`cf07892`](https://github.com/vinzenzs/kazper/commit/cf07892) |
| 2026-06-15 | schedule-adhoc-yoga-mobility | The vault's `/yoga schedule` still shells out to `garmin.py` because the API has no server-side path; adds `yoga`/`mobility` to the sport vocabulary + scheduling. | [`34c4b5b`](https://github.com/vinzenzs/kazper/commit/34c4b5b) |
| 2026-06-15 | resolve-zone-targets | Athlete physiology config (FTP, power/HR zones) was stored but consumed by nothing; `EffectiveProgram` now resolves zone-reference targets to absolute power_w/hr_bpm ranges. | [`0ac66fd`](https://github.com/vinzenzs/kazper/commit/0ac66fd) |
| 2026-06-15 | fix-garmin-bridge-training-status-mapping | ACWR was never populated — the server derives it from acute÷chronic load, but the bridge never sent those loads. Fixes the training-status mapping. | [`0dee9fa`](https://github.com/vinzenzs/kazper/commit/0dee9fa) |
| 2026-06-15 | fix-garmin-bridge-threshold-pace-unit | The athlete-config mapper treated Garmin's `lactateThresholdSpeed` as m/s; it's seconds-per-metre. Corrects the threshold-pace unit conversion. | [`edd5753`](https://github.com/vinzenzs/kazper/commit/edd5753) |
| 2026-06-15 | fix-garmin-bridge-athlete-config-mapping | `athlete_config` was never populated — every daily sync reported `skipped (no data)`, so FTP/zones stayed null. Fixes the bridge's athlete-config mapping. | [`108dad3`](https://github.com/vinzenzs/kazper/commit/108dad3) |
| 2026-06-15 | drop-phantom-swim-threshold-mapping | The mapper derived `threshold_swim_pace_sec_per_100m` from a `userData` field Garmin never returns; drops the phantom swim-threshold mapping. | [`8ea26a8`](https://github.com/vinzenzs/kazper/commit/8ea26a8) |
| 2026-06-14 | unify-mcp-tool-registry | `agenttools` was the single source of truth for the agent tool surface but only `internal/chat` consumed it; the MCP server now generates its whole surface from it. | [`6211099`](https://github.com/vinzenzs/kazper/commit/6211099) |
| 2026-06-14 | rebrand-to-kazper | The name `kazper` described a backend, not the product — Kazper is the endurance coach the app embodies. Rebrands user-facing identity + coach persona. | [`942fe88`](https://github.com/vinzenzs/kazper/commit/942fe88) |
| 2026-06-14 | expand-chat-to-coach | The app had two AI surfaces built apart (desktop MCP coach + companion chat); unifies them onto the shared `agenttools` registry with tiered write-confirm. | [`bad56f7`](https://github.com/vinzenzs/kazper/commit/bad56f7) |
| 2026-06-14 | add-slot-duration-override | `add-plan-slot-targets` let a slot carry per-intent target overrides; this adds the parallel per-intent duration overrides. | [`33a2922`](https://github.com/vinzenzs/kazper/commit/33a2922) |
| 2026-06-14 | add-coach-methodology | Surfaces the hand-curated training methodology that lives alongside the schedule in `Plan.md` as plan-level prose. | [`54368c5`](https://github.com/vinzenzs/kazper/commit/54368c5) |
| 2026-06-14 | add-coach-context-endpoints | The in-app coach needs to ground training/recovery advice the way `get_daily_context` grounds nutrition; adds `/context/training` + `/context/recovery`. | [`7a7132e`](https://github.com/vinzenzs/kazper/commit/7a7132e) |
| 2026-06-13 | garmin-workout-library-mgmt | Pushing the plan to the watch leaked Garmin workout objects; adds library management (list/delete) to keep the Garmin workout library clean. | [`630cebd`](https://github.com/vinzenzs/kazper/commit/630cebd) |
| 2026-06-13 | add-garmin-sync-rolling-lookback | The daily sync ran once at 05:00 for today only, missing signals not yet available then; adds a rolling lookback that revisits recent days. | [`2fd25c0`](https://github.com/vinzenzs/kazper/commit/2fd25c0) |
| 2026-06-13 | add-garmin-misc-mirror | The "mirror everything" arc's catch-all: brings remaining Garmin surface fields under the backend's storage. | [`0c7cb0e`](https://github.com/vinzenzs/kazper/commit/0c7cb0e) |
| 2026-06-13 | add-garmin-history-backfill | The bridge's daily sync used a rolling `backfillDays` window; adds an explicit history-backfill path for older data beyond that window. | [`e97e942`](https://github.com/vinzenzs/kazper/commit/e97e942) |
| 2026-06-13 | add-garmin-gear-and-prs | Mirrors two Garmin inventory datasets the backend stored nowhere: gear (shoe/bike mileage, retirement) and personal records. | [`e991134`](https://github.com/vinzenzs/kazper/commit/e991134) |
| 2026-06-13 | add-garmin-athlete-config | `add-garmin-workout-detail` imported NP and time-in-zone but left `intensity_factor` null and zones unknown; mirrors Garmin's athlete config (FTP/zones). | [`64e4629`](https://github.com/vinzenzs/kazper/commit/64e4629) |
| 2026-06-12 | fix-chat-tool-status-chips | The companion Chat screen showed a permanent row of duplicated "running" tool-status chips; fixes the status-chip rendering. | [`4c39f3c`](https://github.com/vinzenzs/kazper/commit/4c39f3c) |
| 2026-06-12 | extend-recovery-fitness | The recovery/fitness snapshot capabilities mirrored only a slice of Garmin's daily measures; extends them (sleep, HRV, RHR, stress, body battery, readiness). | [`9d27c3c`](https://github.com/vinzenzs/kazper/commit/9d27c3c) |
| 2026-06-12 | add-workout-templates | The athlete's reusable workout library — the ~40 swim/bike/run/yoga session definitions — gets its own capability. | [`e7225ad`](https://github.com/vinzenzs/kazper/commit/e7225ad) |
| 2026-06-12 | add-workout-reconciliation | Once the plan materializes planned workouts, completed Garmin imports must be matched back; adds workout reconciliation. | [`ccc2b08`](https://github.com/vinzenzs/kazper/commit/ccc2b08) |
| 2026-06-12 | add-training-plan | The 18-week triathlon plan — which workout falls on which day of which week — becomes a first-class plan → weeks → slots structure with materialize. | [`d2cc452`](https://github.com/vinzenzs/kazper/commit/d2cc452) |
| 2026-06-12 | add-plan-slot-targets | A workout template carries effort targets (pace, HR/power zone, RPE); this lets a plan slot override them per-intent. | [`4d62851`](https://github.com/vinzenzs/kazper/commit/4d62851) |
| 2026-06-12 | add-garmin-workout-detail | Garmin-synced workouts landed as a flat summary; imports far more per activity — time-in-HR-zone, elevation, and more. | [`6967118`](https://github.com/vinzenzs/kazper/commit/6967118) |
| 2026-06-12 | add-garmin-scheduling | The last plane of `garmin.py` and the only genuinely new direction: pushes structured workouts to the watch and schedules them. | [`1066e53`](https://github.com/vinzenzs/kazper/commit/1066e53) |
| 2026-06-12 | add-garmin-mcp-login | The Garmin token expires roughly yearly (and on password change); adds an MCP login/re-auth path. | [`19b7558`](https://github.com/vinzenzs/kazper/commit/19b7558) |
| 2026-06-12 | add-garmin-daily-energy | EA saw only logged-workout burn; mirrors Garmin's full daily energy expenditure so non-workout movement counts. | [`b740059`](https://github.com/vinzenzs/kazper/commit/b740059) |
| 2026-06-12 | add-garmin-bridge | The backend stores recovery/fitness/hydration/weight/workout data but had no Garmin ingest path; adds the garmin-bridge sidecar. | [`26071ec`](https://github.com/vinzenzs/kazper/commit/26071ec) |
| 2026-06-12 | add-garmin-auth-token | The backend had homes for Garmin-sourced data but no stored credential; adds encrypted Garmin auth-token storage. | [`345dc9e`](https://github.com/vinzenzs/kazper/commit/345dc9e) |
| 2026-06-12 | add-companion-session-list | The backend persists chat sessions and the app creates one per conversation, but gave no way to see them; adds the on-device session list. | [`c98f72d`](https://github.com/vinzenzs/kazper/commit/c98f72d) |
| 2026-06-12 | add-chat-sessions | `/chat` was stateless — every client carried the full transcript; adds server-side conversation sessions (list/resume). | [`1931ca4`](https://github.com/vinzenzs/kazper/commit/1931ca4) |
| 2026-06-11 | add-shopping-list | Selecting dishes is half the loop — ingredients must reach the store; adds a shopping list derived from recipe ingredients + meal-plan selections. | [`1de88cd`](https://github.com/vinzenzs/kazper/commit/1de88cd) |
| 2026-06-11 | add-recipe-ingredients | Cookidoo recipe imports threw away their ingredient lists; captures the Schema.org `Recipe` ingredients at import time. | [`78f547b`](https://github.com/vinzenzs/kazper/commit/78f547b) |
| 2026-06-11 | add-race-fueling-plan | The API had no durable concept of a race or per-leg fueling; adds races + a per-leg race fueling plan. | [`34e2f36`](https://github.com/vinzenzs/kazper/commit/34e2f36) |
| 2026-06-11 | add-meal-plan | The "what should I eat?" flow needed somewhere to put a selection that isn't yet a logged event; adds planned meals. | [`4686f10`](https://github.com/vinzenzs/kazper/commit/4686f10) |
| 2026-06-11 | add-flutter-companion-app | The system was designed for two clients but only the agent existed; adds the Flutter mobile companion app. | [`61568d9`](https://github.com/vinzenzs/kazper/commit/61568d9) |
| 2026-06-11 | add-companion-food-picker | The companion could only log a food three ways; adds a unified food picker (scan/search/recent). | [`127a935`](https://github.com/vinzenzs/kazper/commit/127a935) |
| 2026-06-11 | add-companion-chat | The mobile spec reserved a disabled fourth nav slot for a v2 chat; lands the companion chat screen on the backend chat loop. | [`c57c58e`](https://github.com/vinzenzs/kazper/commit/c57c58e) |
| 2026-06-11 | add-chat-backend | The companion needs nutrition chat, but the coaching agent ran on desktop over stdio MCP — unreachable from a phone; adds the HTTP chat backend. | [`9938907`](https://github.com/vinzenzs/kazper/commit/9938907) |
| 2026-06-10 | widen-workout-ingestion | Garmin measures what the fueling tools guess at (distance, power, temp, sweat loss) and records bricks as links; widens workout ingestion. | [`6952e61`](https://github.com/vinzenzs/kazper/commit/6952e61) |
| 2026-06-10 | add-hydration-balance-metrics | Garmin's daily hydration carries `sweatLossInML`/`activityIntakeInML` alongside the value/goal the importer pushed; adds hydration-balance metrics. | [`46978a2`](https://github.com/vinzenzs/kazper/commit/46978a2) |
| 2026-06-10 | add-garmin-daily-metrics | The `garmin.py` script computed 7-day recovery/fitness averages each run and threw them away; adds storage for Garmin daily metrics. | [`60a13e8`](https://github.com/vinzenzs/kazper/commit/60a13e8) |
| 2026-06-10 | add-deployment-pipeline | The API shipped 17+ features but lived only locally; adds the deployment pipeline. | [`8bad270`](https://github.com/vinzenzs/kazper/commit/8bad270) |
| 2026-06-09 | add-workout-rpe-and-gi | The 70.3 build mandates fueling rehearsal on long rides; the API logged what was consumed but not RPE/GI response — adds those. | [`303cd60`](https://github.com/vinzenzs/kazper/commit/303cd60) |
| 2026-06-09 | add-training-phases-and-templates | Every day looked identical to the system mid-way through a 16-week plan; adds training phases + templates so the agent knows the block. | [`8e51019`](https://github.com/vinzenzs/kazper/commit/8e51019) |
| 2026-06-09 | add-rolling-window-summaries | Most nutrition-science recommendations are multi-day averages, not single-day totals; adds rolling-window summaries. | [`e8c33b6`](https://github.com/vinzenzs/kazper/commit/e8c33b6) |
| 2026-06-09 | add-recommend-workout-fuel | Bridges "how to eat before a race" and "today's macro target" with a workout-fuel recommendation for a given session. | [`66a2085`](https://github.com/vinzenzs/kazper/commit/66a2085) |
| 2026-06-09 | add-protein-distribution | Daily protein total is necessary but not sufficient — MPS triggers per-meal (~0.3 g/kg); adds per-meal protein distribution analysis. | [`8e51019`](https://github.com/vinzenzs/kazper/commit/8e51019) |
| 2026-06-09 | add-meal-from-photo | The companion's second killer interaction is photo-of-meal for foods with no barcode; adds meal-from-photo. | [`68d0f0c`](https://github.com/vinzenzs/kazper/commit/68d0f0c) |
| 2026-06-09 | add-daily-context-aggregator | The agent made 5–7 separate MCP calls to start every conversation; adds a single daily-context aggregator. | [`8e51019`](https://github.com/vinzenzs/kazper/commit/8e51019) |
| 2026-06-08 | add-workouts-capability | The API had no concept of a workout, blocking six tier-1/2 needs; adds the core workouts capability. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-08 | add-workout-fuel | Endurance sodium targets (300–800 mg/hr) were invisible; `log_hydration` recorded ml only — adds workout-fuel intake tracking. | [`5d141a1`](https://github.com/vinzenzs/kazper/commit/5d141a1) |
| 2026-06-08 | add-weight-log | The Garmin coach read body weight from Connect, but the API could not record a weight taken any other way; adds the weight log. | [`5d141a1`](https://github.com/vinzenzs/kazper/commit/5d141a1) |
| 2026-06-08 | add-meal-workout-link | Workouts and weight-log landed as standalone tables; links meals/fuel to a workout session. | [`5d141a1`](https://github.com/vinzenzs/kazper/commit/5d141a1) |
| 2026-06-08 | add-last-logged-quantity | The companion's killer interaction is scan→log in 2 taps, where the default quantity decides speed; adds last-logged-quantity defaults. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-08 | add-hydration-tracking | The system could answer "what did I eat?" but not "what did I drink?"; adds hydration tracking (ml). | [`5d141a1`](https://github.com/vinzenzs/kazper/commit/5d141a1) |
| 2026-06-08 | add-energy-availability | EA is the single most important number for an athlete in a deficit; adds the Loucks energy-availability computation. | [`5d141a1`](https://github.com/vinzenzs/kazper/commit/5d141a1) |
| 2026-06-08 | add-date-varying-goals | A training day differs from a rest day in calories/carbs; adds per-date goal overrides (≈2200 train / 1900 rest). | [`5d141a1`](https://github.com/vinzenzs/kazper/commit/5d141a1) |
| 2026-06-08 | add-carb-load-auto-apply | `plan_carb_load` returned a schedule but the agent had to issue N `set_daily_goal_override` calls; auto-applies the carb-load plan. | [`4026c8e`](https://github.com/vinzenzs/kazper/commit/4026c8e) |
| 2026-06-07 | unify-adherence-shape | An MCP test session flagged three API-shape inconsistencies that annoy clients; unifies the adherence response shape. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | streamline-local-dev | Local dev required disconnected steps (Postgres, hand-edited `.env`, source, run); streamlines into one command. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | harden-write-paths | A real MCP-driven session surfaced two correctness bugs and a rough edge in the write paths; hardens them. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | daily-use-essentials | The MVP shipped single-ingredient logging and macro-only summaries — painful by day three; adds multi-ingredient meals + "did I hit my day?". | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | add-swagger-cobra-viper | The entrypoint mixed ad-hoc env parsing, had no CLI surface, and exposed no API contract; adds swagger + cobra + viper. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | add-race-prep-primitives | Race week has a specific carb-loading shape (8–12 g/kg, then ~2 g/kg pre-race); adds the race-prep carb-load primitives. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | add-product-management-tools | An MCP test session left two product-hygiene findings unaddressed; adds product-management tools. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | add-meal-logging-mvp | There was no backend for logging meals; adds the meal-logging MVP, writable from mobile and (later) the agent. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | add-mcp-server | The REST API stored meals but the LLM coaching agent could not reach it; adds the MCP server wrapping the REST surface. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |
| 2026-06-07 | add-cookidoo-importer | Cookidoo has no public API but embeds Schema.org `Recipe` JSON-LD; adds a Cookidoo recipe importer. | [`debcd6d`](https://github.com/vinzenzs/kazper/commit/debcd6d) |

## Planned

| Change | Summary | Proposed by | Proposed |
|---|---|---|---|
| add-multisport-structured-workouts | A triathlon/brick is one continuous session (swim→T1→bike→T2→run) that should push as a single multisport watch workout; today bricks are faked as separate single-sport rows. Phase 1 = library + multi-segment compile + schedule. | Vinzenz Stadtmueller | uncommitted |

---
_To regenerate: ask Claude "update the roadmap"._
_For the forward queue (in-progress + up-next + backlog), see [`continuity.md`](continuity.md)._
