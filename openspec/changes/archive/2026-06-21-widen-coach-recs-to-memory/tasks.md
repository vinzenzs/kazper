## 1. Migration (rename + widen)

- [x] 1.1 `task migrate:new NAME=widen_coach_recs_to_memory`; verify head first — macrocycle holds an uncommitted `049_add_macrocycles`, so this is `050`.
- [x] 1.2 `up.sql`: `ALTER TABLE coach_recommendations RENAME TO coach_memory`; `RENAME COLUMN recommendation TO text`; add `kind TEXT` (CHECK over the 5 values), `expires_at DATE NULL`, `review_at DATE NULL`, `status TEXT NOT NULL DEFAULT 'active'` (CHECK `active|archived`); back-fill `kind='recommendation'`; drop NOT NULL on `date` and `scope`; rename the index; keep the `scope` CHECK but allow NULL.
- [x] 1.3 `down.sql`: reverse the rename/columns; re-tighten `date`/`scope` NOT NULL (after asserting no NULLs); document that down is lossy for non-recommendation rows.

## 2. Package rename + kind model

- [x] 2.1 Rename `internal/coachrecs/` → `internal/coachmemory/` (types/repo/service/handlers + tests); update imports and `httpserver` wiring.
- [x] 2.2 Add the `Kind` enum (`fact|preference|constraint|observation|recommendation`) with `ValidKind`/`ParseKind` (mirror the Sport/Status pattern); rename `recommendation`→`text` on the row/struct.
- [x] 2.3 Add nullable `Scope`/`Date`, plus `ExpiresAt`/`ReviewAt`/`Status` to types + repo select/scan + insert.

## 3. Service + handlers

- [x] 3.1 Validation: `text_required`, `kind_invalid`, `scope_invalid` (when present), `date_invalid`, and conditional `date_required` when `kind=recommendation`.
- [x] 3.2 Routes rename `/coach/recommendations*` → `/coach/memory*`; 404 code `memory_not_found`.
- [x] 3.3 List: window-filter `recommendation` items; return dateless standing items regardless of window; `kind`/`scope`/`include_archived` filters; exclude archived + expired by default.
- [x] 3.4 **New** `PATCH /coach/memory/{id}` — lifecycle only (`review_at`/`expires_at`/`status`); reject content edits; preserve `created_at`; `status_invalid`/`date_invalid` mapping.
- [x] 3.5 `task swag` after handler/struct changes.

## 4. Agent tools (rename + add) — explicit writes

- [x] 4.1 Rename the four `*_coach_recommendation(s)` specs in `internal/agenttools` to the memory family (write/list/get/delete) over the new endpoints.
- [x] 4.2 Add the `update_coach_memory` tool for the PATCH/confirm path.
- [x] 4.3 Keep the write tool chat-exposed at `TierWriteConfirm` (user-confirmed); MCP-exposed for user-initiated writes. Verify no autonomous-write path.
- [x] 4.4 Bump the `mcp_integration_test` expected-tools list.

## 5. Fold memory into grounding

- [x] 5.1 `/context/daily` (daily-context): add a `memory` block — active, non-expired; standing kinds always, `recommendation` for the requested date; `needs_review` flag when `review_at <= today`.
- [x] 5.2 `/context/training` (coach-context): same `memory` block, `recommendation` scoped to the lookback window.
- [x] 5.3 Ensure the aggregators do no synthesis over memory (verbatim items).

## 6. Tests

- [x] 6.1 POST: standing fact (no date) stored; recommendation without date → `date_required`; `text_required`; `kind_invalid`.
- [x] 6.2 List: standing items returned regardless of window; recommendations window-filtered; expired/archived excluded by default; `include_archived` includes.
- [x] 6.3 PATCH: confirm pushes `review_at` and preserves `created_at`; content edit rejected; archive hides from default list; 404 on missing.
- [x] 6.4 Migration: existing recommendation rows back-fill to `kind='recommendation'`, `status='active'`, text preserved.
- [x] 6.5 Context: daily + training responses carry the `memory` block with `needs_review` flagging; archived/expired excluded.
- [x] 6.6 MCP: each memory tool issues exactly one request; expected-tools list matches.

## 7. Spec retirement + verification

- [x] 7.1 At archive: retire the `coach-recommendations` spec folder (superseded by `coach-memory`); confirm the archive tooling moves/removes it cleanly or do it by hand.
- [x] 7.2 `task test` (coachmemory + agenttools + context packages green), `task vet`.
