# Tasks

## 1. Schema & stream plumbing

- [x] 1.1 Verify the on-disk migration head (currently `060`; sibling proposals also carry migrations — take the next free slot), then `task migrate:new NAME=widen_stream_types_cadence`: widen the `stream_type` CHECK to include `'cadence'`; down deletes cadence rows and narrows it back
- [x] 1.2 Ingest accepts optional `cadence` (rpm; gap-zeros, all-non-positive dropped); retrieval serves `streams.cadence`; assert cadence feeds no best-effort/execution-metric/energy path
- [x] 1.3 Integration: four-stream post + retrieval, cadence-only-absent omission, replace-on-repost, cascade

## 2. Bridge

- [x] 2.1 `_extract_streams` pulls `directBikeCadence` defensively (gaps→0, all-non-positive dropped, unexpected shape → no array) into the existing stream POST
- [x] 2.2 Bridge tests: extraction fixture, missing descriptor, malformed shape (pytest suite green)

## 3. Quadrant endpoint

- [x] 3.1 `internal/activitystreams/quadrant.go`: CPV/AEPF per paired positive sample, reference-point classification, shares/pedaling/excluded, ≤1000-point systematic scatter; pure, unit-tested (hand-computed fixture, coasting exclusion, crank sensitivity, boundary rounding)
- [x] 3.2 `GET /workouts/{id}/quadrant` handler: param matrix (`cp_invalid`/`cadence_invalid`/`crank_invalid` + default 172.5), `summary_only`, sentinels incl. `cadence_stream_missing`; nothing persisted
- [x] 3.3 Integration tests: happy path, summary_only, param 400 matrix, all four sentinels, read-only
- [ ] 3.4 `task swag`

## 4. MCP

- [x] 4.1 `quadrant_analysis` read tool (`summary_only=true` hardcoded; description points at `cp_model`)
- [x] 4.2 Golden regen (additive) + registry/integration green

## 5. Dashboard

- [x] 5.1 `useQuadrant` hook + types; detail-page scatter (reference lines + shares legend), 90 rpm pivot, absent states
- [x] 5.2 Web tests — `tsc` + vitest green

## 6. Verification & rollout

- [x] 6.1 `task vet` + full Go suite green (`-p 1` rerun on boot-contention flakes)
- [ ] 6.2 Deploy sequencing: backend before bridge (old-bridge/new-backend safe; the reverse gets cadence rejected by the CHECK); confirm the `directBikeCadence` key on the first real post-deploy sync
- [ ] 6.3 Live: quadrant read on a post-deploy ride; shares eyeball-consistent with the session
