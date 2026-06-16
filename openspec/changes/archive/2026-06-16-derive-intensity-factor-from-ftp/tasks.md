## 1. Wire the athlete-config dependency into workouts

- [x] 1.1 Add a nilable `athleteConfigRepo *athleteconfig.Repo` field to `workouts.Service` and a `SetAthleteConfigRepo(*athleteconfig.Repo)` optional setter (mirroring `trainingplan.SetAthleteConfigRepo`); guard against an import cycle (`athleteconfig` must not import `workouts`).
- [x] 1.2 In `internal/httpserver/server.go`, call `workoutsSvc.SetAthleteConfigRepo(athleteConfigRepo)` after `athleteConfigRepo` is constructed, with a comment referencing the same cross-injection convention as `trainingPlanSvc.SetAthleteConfigRepo`.

## 2. Implement the derivation

- [x] 2.1 Add a private helper on `workouts.Service` (e.g. `deriveIntensityFactor`) that returns the IF to store given sport, `normalized_power_w`, caller-supplied `intensity_factor`, and the fetched config: returns the supplied IF verbatim when non-nil; otherwise, when sport==`bike` && np>0 && cfg.FtpWatts>0, returns `Round2(np/ftp)`; otherwise returns the original (nil) value.
- [x] 2.2 Fetch the athlete-config singleton only when the pre-gate (bike && np>0 && IF not supplied) passes and the repo is non-nil; on repo error or nil config, fall through to no-derivation (no error surfaced on the write path).
- [x] 2.3 Use a 2dp rounding consistent with the `NUMERIC(4,2)` column and the existing `service.go` IF-precision note; reuse/add the appropriate `numfmt` helper.
- [x] 2.4 Call the helper from BOTH the create build path and the update (full-replace) build path in `service.go` so a re-synced bike workout with NP but no IF fills in.

## 3. Tests

- [x] 3.1 Bike workout with FTP set + NP + no supplied IF → stored IF == `Round2(np/ftp)`.
- [x] 3.2 Caller-supplied IF is stored verbatim and never overridden (derivation skipped).
- [x] 3.3 Non-bike sport (run) with NP + FTP → IF stays NULL.
- [x] 3.4 FTP unset → IF stays NULL, create succeeds without error.
- [x] 3.5 NP missing/zero → IF stays NULL.
- [x] 3.6 Update (full-replace) fills a previously-NULL IF on a bike workout.
- [x] 3.7 Nil athlete-config repo (not wired) → write succeeds, no derivation, no panic.

## 4. Docs & verification

- [x] 4.1 Run `task swag` if any handler annotation/struct changed — verified NOT needed: `handlers.go`, the `Workout` response type, and the handler request struct were untouched (`intensity_factor` already documented); `docs/` shows no drift.
- [x] 4.2 Run `go test -count=1 ./internal/workouts/...` and `task vet`; fix any drift. (Full suite green in 170s, vet exit 0.)
- [ ] 4.3 Manual/MCP smoke (deferred to user — needs a live server): set `ftp_watts`, log a bike workout with `normalized_power_w` and no `intensity_factor`, confirm `GET /workouts/{id}` returns the derived IF; confirm a supplied IF is preserved.
