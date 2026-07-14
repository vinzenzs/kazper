## Why

The Intervals.icu gap analysis (2026-07-14) surfaced one capability Kazper genuinely lacks: structured *subjective* wellness. The system ingests rich objective recovery signals from Garmin (HRV, sleep, RHR, readiness, body battery) but has no seam for how the athlete actually feels — fatigue, soreness, mood, motivation, stress. Per-workout RPE and freeform `coach-memory` observations don't fill it: there's no dated, queryable daily series to hold against the objective data ("TSB says fresh, legs say otherwise") or to inform today's session call. The coach agent is the natural collector — it can *ask* during a morning check-in and log the answer — which makes this a better fit for Kazper than for the form-driven platforms that inspired it.

## What Changes

- New `wellness_entries` table (migration `060`): one row per date, five optional 1–5 self-reported scores (`fatigue`, `soreness`, `stress` — 1 none → 5 severe; `mood`, `motivation` — 1 low → 5 high) plus an optional free-text `note`; at least one field must be present.
- New `internal/wellness/` capability package: `PUT /api/v1/wellness/{date}` (per-date singleton, full-replace upsert — rejects `Idempotency-Key` per the PUT rule), `GET /wellness/{date}`, `GET /wellness?from=&to=` (ascending window), `DELETE /wellness/{date}`.
- MCP tools: `log_wellness` (write; wraps the PUT) and `list_wellness` (read window).
- `/context/daily` folds in today's entry (omitted when none) next to the Garmin recovery block, so both surfaces see subjective + objective side by side.
- `wellness_entries` classified **export-included** in dataexport (user-authored, small, not re-derivable).

## Capabilities

### New Capabilities

- `wellness-diary`: the subjective daily wellness log — entry shape and score semantics, per-date upsert/read/delete, window read, and the MCP tools.

### Modified Capabilities

- `daily-context`: 1 ADDED requirement — today's wellness entry in the `/context/daily` payload.

## Impact

- **Code:** new `internal/wellness/` (types/repo/service/handlers per the capability template); migration `060` (verify head on disk first — out-of-band work has taken slots before); `internal/dataexport/inventory.go` classification; daily-context aggregation touch; MCP registry + golden (additive).
- **API/MCP:** four REST routes, two MCP tools. `task swag` required.
- **Out of scope (deferred):** a dashboard wellness panel, companion-app entry UI (the coach chat is the input surface for v1), correlation analytics against PMC/TSB, injury tracking as a structured field (freeform `note` carries it for now), and any Garmin/Whoop subjective import.
