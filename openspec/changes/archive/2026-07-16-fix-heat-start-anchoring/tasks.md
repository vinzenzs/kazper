# Tasks

## 1. Backend

- [x] 1.1 `DEFAULT_TRAINING_START` config (`HH:MM`, boot-validated, default `06:00`, interpreted in `DEFAULT_USER_TZ`)
- [x] 1.2 Window anchoring in `internal/heat/service.go`: precedence param > non-midnight `started_at` > assumed default; local-midnight sentinel detection in the user tz; `window`/`start_source`/`assumed_start` echoes
      _**Spec corrected during apply — the sentinel is UTC-or-local midnight, not local only.** The proposal blamed "materialized/scheduled planned workouts"; both halves are imprecise. `internal/trainingplan` materialize stamps a REAL hour (`defaultStartHour + slotOrdinal`, i.e. 06:00+) — those rows were never affected. The midnight rows come solely from the ad-hoc **`internal/garmincontrol` schedule path**, which does `time.Parse("2006-01-02")` → **UTC** midnight. For a Vienna athlete that reads 02:00 local, so a **local-only sentinel would have missed every real row and left the bug unfixed**. Implemented as "exactly midnight in UTC OR in the athlete's zone"; the delta spec was updated to match, with the reasoning._
      _`assumed_start` carries the applied **`HH:MM`** rather than a bool (per the spec's scenario): `start_source` already says THAT it was assumed, so the value says WHICH default applied and a wrong one is self-evident._
- [x] 1.3 `start=HH:MM` param validation (`start_invalid`); context heat block same precedence + `assumed_start`
- [ ] 1.4 ~~Verify the existing workout PATCH accepts a time-of-day update on planned `started_at` (the durable per-session fix); document in the heat tool description~~
      _**VERIFIED FALSE — the premise doesn't hold, so nothing was documented.** `PATCH /workouts/{id}` rejects `started_at` with `400 field_immutable` (it sits in the handler's immutable set; the endpoint's own docs say "delete and re-create if those are wrong"). Proved empirically against the real handler, not by reading. The proposal's "setting the true start via the existing workout PATCH is the durable per-session fix" **does not exist**, and the tool description deliberately does NOT tell the coach to try it — that instruction would have produced a 400 every time._
      _What actually exists: (a) the `start=HH:MM` param — answers the question per read, not durable; (b) delete + re-create the planned workout with a real `started_at` (POST accepts it) — durable but heavy, and it would break the Garmin schedule linkage; (c) the design's own deferred option, **write-side stamping at schedule time**, which is the real systemic fix. Left unimplemented because it is explicitly out of this change's scope. **Needs a decision — see the session summary.**_
- [x] 1.5 Tests: midnight-sentinel assumption, real-time anchoring, param override + validation, tz-correct midnight detection, context propagation; regression — a completed-workout 409 and all prior degradations unchanged
      _The integration fixture reproduces the live case end-to-end: a Vienna athlete, a warming-morning forecast (15 °C pre-dawn → 30 °C by 10:00), and a session written exactly as `garmincontrol` writes it. Asserts the pre-dawn under-read is gone (~21 °C at the assumed 06:00, not ~15 °C) and that `start=10:00` reads ~31.7 °C — the coach's early-vs-late comparison, in two calls._
- [x] 1.6 `task swag`

## 2. MCP

- [x] 2.1 `workout_heat` gains optional `start`; golden regen (input-schema touch) + registry/integration green
      _Description now tells the coach to read `start_source`/`assumed_start` before quoting a number, and to say "assuming your usual 06:00" rather than presenting an assumed hour as fact._

## 3. Verification

- [x] 3.1 `task vet` + heat suite green (full Go suite green too)
- [ ] 3.2 Live re-run of the coach's case: tomorrow's brick with no param (expect ~06:00 anchoring + `assumed_start`), then `start=10:00` (expect the ~27 °C apparent read) — matches the independent forecast pull
      _(operator step — needs the live deployment and the real forecast. **`DEFAULT_TRAINING_START` is new config**: unset falls back to `06:00`, which matches this athlete's habit, so no action is needed unless it differs.)_
