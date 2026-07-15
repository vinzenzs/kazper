## Why

A still-open triage item: supplements (creatine, iron, vitamin D, magnesium, caffeine tablets outside sessions) have no home — they aren't meals (no meaningful macros), aren't workout-fuel (not in-session), and freeform coach-memory isn't queryable by date. A thin dated log makes "did the iron protocol hold through the block?" answerable.

## What Changes

- New `supplement_entries` table (migration, next free slot): `logged_at` timestamp, required free-text `name`, optional paired `dose` + `dose_unit` (both or neither), optional `note`.
- New `internal/supplements/` capability package: `POST /api/v1/supplements` (Idempotency-Key supported), `GET /supplements?from=&to=` (ascending window, 92-day cap), `GET /supplements/{id}`, `DELETE /supplements/{id}` — no PATCH; corrections are delete + re-log (the coach-memory precedent).
- MCP tools: `log_supplement` (write) + `list_supplements` (read).
- `/context/daily` folds in today's entries (omitted when none) beside wellness — the same snapshot posture.
- `supplement_entries` classified **export-included** in dataexport.

## Capabilities

### New Capabilities

- `supplements`: the dated supplement log — entry shape, write/read/delete, window read, MCP tools.

### Modified Capabilities

- `daily-context`: 1 ADDED requirement — today's supplements in `/context/daily`.

## Impact

- **Code:** new capability package per the template; migration (**coordinate the slot** — several proposed changes carry migrations); dataexport classification; daily-context touch; MCP registry + golden additive; `task swag`.
- **Out of scope:** a supplement catalog/products link, scheduled-protocol tracking with compliance ("take daily, alert on miss"), interaction with nutrition summaries (supplements deliberately feed no macro/kcal totals — unit isolation).
