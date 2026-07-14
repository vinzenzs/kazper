# Tasks

## 1. Schema

- [x] 1.1 Verify the on-disk migration head (should be `059`), then `task migrate:new NAME=add_wellness_entries` → `060`: `wellness_entries` (`entry_date DATE PK`, five nullable `SMALLINT CHECK BETWEEN 1 AND 5`, `note TEXT`, timestamps) + down
- [x] 1.2 Classify `wellness_entries` **export-included** in `internal/dataexport/inventory.go` (drift-guard green)

## 2. Capability package

- [x] 2.1 `internal/wellness/` per the template: `types.go` (Entry, omitempty scores), `repo.go` (upsert/get/window-asc/delete against `store.Querier`), `service.go` (validation + sentinels: `wellness_empty`, `wellness_score_invalid`+field, `note_too_long`, `date_invalid`, `not_found`, range errors w/ 92-day cap)
- [x] 2.2 `handlers.go` + `Register`: `PUT/GET/DELETE /wellness/{date}`, `GET /wellness?from&to`; PUT rejects `Idempotency-Key` (`idempotency_unsupported_for_put`); swag annotations; wire in `httpserver.Run()`
- [x] 2.3 Integration tests (testcontainers): partial-entry PUT + echo, full-replace (not merge), empty-entry 400, score-range matrix with `field`, note cap, Idempotency-Key-on-PUT 400, GET/DELETE 404s, window ascending / empty-200 / full 400 matrix (invalid, reversed, >92d)
- [x] 2.4 `task swag`

## 3. Daily context

- [x] 3.1 Fold today's entry into `/context/daily` as `wellness` (verbatim, omitted when absent) beside the recovery block
- [x] 3.2 Context tests: logged day present with exact fields, unlogged day omits the key, remainder of payload unaffected

## 4. MCP

- [x] 4.1 `log_wellness` (write, wraps the PUT, no idempotency key; description: score directions + full-replace + encourage partial entries) and `list_wellness` (read window)
- [x] 4.2 Golden regen (`-tags=goldengen`, additive) + registry/integration tests green

## 5. Docs & verification

- [x] 5.1 README MCP table rows for the two tools
- [x] 5.2 `task vet` + full Go suite green (isolated `-p 1` rerun on testcontainers boot-contention flakes)
- [ ] 5.3 Live smoke: log a partial entry via MCP, read it back in `/context/daily`, replace it, delete it
