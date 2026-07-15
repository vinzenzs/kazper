# Tasks

## 1. Schema

- [x] 1.1 Verify the on-disk migration head, then `task migrate:new NAME=add_garmin_detected_thresholds`: the `garmin_detected_thresholds` singleton (detectable physiology fields + `detected_at` + timestamps, fixed PK) **and** `garmin_sourced_fields TEXT[] NOT NULL DEFAULT '{}'` on `athlete_config`; down drops both
- [x] 1.2 Dataexport: detection table **excluded** (latest-only, re-derived next sync); confirm the config row's new column rides the existing inclusion; drift guard green

## 2. Backend — detection, policy, effective

- [x] 2.1 `internal/athleteconfig/`: detection repo/service (full-replace PUT, garmin-identity-only guard + Idempotency-Key-on-PUT rejection; GET open to others, null when none)
- [x] 2.2 Guard `PUT /athlete-config` against the garmin identity (403, existing identity-guard vocabulary + tests)
- [x] 2.3 Sources: whitelist validation (`source_field_invalid`), `PUT /athlete-config/sources` (policy-only mutation; config PUT full-replace provably preserves it), `sources` echoed on `GET /athlete-config`
- [x] 2.4 `EffectiveConfig()` resolution (garmin-sourced + detection-present → detected; fallback manual; zones as groups) + `GET /athlete-config/effective` with per-field `source` annotations
- [x] 2.5 Effective-config adapter at `httpserver.Run()` wired into the physiology consumers (per-sport TSS derivation, `trainingplan.EffectiveProgram`, race pacing, step compliance) — all-manual policy must be behavior-identical (regression: existing suites green untouched)
- [x] 2.6 Integration tests: detection write/read + identity matrix + config-untouched assertion, sources matrix (flip/invalid/preserved-across-config-PUT/no-history-snapshot), effective resolution (sourced/fallback/all-manual-identity), TSS-derives-against-effective, `task swag`

## 3. Bridge

- [x] 3.1 `sync.py`: PUT `/athlete-config/garmin-detected` instead of `/athlete-config` (mapper unchanged); summary key stays per-capability
- [x] 3.2 Bridge tests updated (endpoint swap, per-capability failure accounting on 403); pytest green

## 4. Context & MCP

- [x] 4.1 `/context/training`: `garmin_detected` + `threshold_sources` + `effective` blocks (null-safe) + tests
- [x] 4.2 `set_threshold_sources` write tool (description: tokens, full-replace, recompute pointer); golden regen (additive) + registry/integration green

## 5. Verification & rollout (operator)

- [x] 5.1 `task vet` + full suite green (`-p 1` on boot-contention flakes)
- [ ] 5.2 Deploy backend, then bridge (one cycle of visible `athlete_config: failed (403)` in the sync summary is expected between the two)
- [ ] 5.3 **Recovery runbook:** re-PUT the confirmed config (MaxHR 196; FTP 278 deliberately confirmed), set the desired sources (e.g. flip `ftp_watts` to garmin if that's the standing preference), then `POST /workouts/recompute-tss` and confirm the NULL-TSS workouts backfilled
- [ ] 5.4 Verify the next real sync writes the detection singleton, touches neither config nor threshold history, and the training context shows all three blocks