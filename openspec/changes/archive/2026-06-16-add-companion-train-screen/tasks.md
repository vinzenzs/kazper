## 1. Navigation: the fifth screen

- [x] 1.1 Add a `Train` destination to the bottom navigation (Today · Train · Camera · Recent · Chat) in `apps/companion/lib/ui`; wire routing/state for the new tab.
- [x] 1.2 Decide and set nav order (open question in design); add the empty `ui/train/train_page.dart` shell.

## 2. Data layer: training reads (stale-while-revalidate)

- [x] 2.1 Add domain models for the day's session(s) + resolved program + session fuel in `apps/companion/lib/domain`.
- [x] 2.2 Add repository reads in `apps/companion/lib/data`: today's/upcoming sessions (`GET /context/training`), the session's resolved steps (`GET /workouts/{id}/program`), and the per-session fueling recommendation (`recommend-workout-fuel` / `workout-fuel`).
- [x] 2.3 Cache via the existing SWR pattern + DAOs (`data/db/dao`); show stale, revalidate, no offline banner.

## 3. Band 1 UI: today's session + its fuel (the hero)

- [x] 3.1 Build the session card: sport, duration, planned time, resolved target (e.g. `230–268W (Z4)`); render multisport sessions as ordered per-segment targets.
- [x] 3.2 Build the fuel block: pre / during / post fueling the session demands.
- [x] 3.3 Build the rest-day state (no session → minimal recovery-fueling view, not blank).

## 4. Writes (read-only v1)

- [x] 4.1 v1 is **read-only** (user decision): the Train screen initiates no writes. The log-fuel outbox write is a documented fast-follow — capture it in design's open questions, build nothing here.
- [x] 4.2 Guardrail holds trivially: the screen has zero write affordances (no scheduling/status/edit, and no fueling write in v1).

## 5. Tests & guardrail

- [x] 5.1 Widget/integration tests: session card renders header + resolved target + fuel; multisport segments; rest-day state. Provider test with a fake `Repository` (no Drift/Dio), mirroring `test/state/`.
- [x] 5.2 Guardrail test: the Train screen exposes no write affordance (no button/control that mutates state).
- [x] 5.3 Run the companion app test suite (`flutter test`) + `dart analyze`.

## 6. Follow-on phases (tracked, not built here)

- [x] 6.1 Note Band 2 (causal "training moved your targets" delta) as a separate proposal — needs a backend read breaking out the training contribution to the day's goal.
- [x] 6.2 Note Band 3 (weekly load + adherence arc + EA flag + race countdown) as a separate proposal, sequenced after `plan-adherence-analytics`.
