# Add per-sport computed TSS with provenance

## Why

Stored `tss` today is whatever Garmin supplies, which is effectively power-based — swim and run sessions without power land with NULL TSS, so any future load analytics (CTL/ATL/TSB) over the workouts table would be dishonest. All the inputs for TrainingPeaks-style per-sport fallbacks (distance/duration, avg HR, and the athlete-config thresholds: FTP, run threshold pace, CSS, LTHR) already exist — nothing computes them into a TSS.

## What Changes

- **`tss_source` provenance column on `workouts`** (`garmin | manual | power | pace | hr`, nullable, server-managed). Migration back-fills it for existing rows that already carry a `tss` (`source='garmin'` → `garmin`, else `manual`); rows with NULL `tss` keep NULL.
- **Ingest-time TSS derivation for completed workouts** (`POST /workouts`, each `POST /workouts/bulk` item, and the `external_id` upsert-update path), with a fixed precedence: explicit caller-supplied TSS > power TSS (bike, from IF) > pace TSS (rTSS for runs, sTSS for swims) > hrTSS (any sport with `avg_hr`) > none. Missing thresholds fall through to the next method; when nothing applies, `tss` stays NULL with no error (fail-open, matching the existing `intensity_factor` derivation).
- **`PATCH /workouts/{id}` interplay**: patching `tss` to a value marks `tss_source='manual'`; patching `tss` to `null` clears both. PATCH never derives.
- **Backfill via a one-off recompute endpoint** `POST /workouts/recompute-tss` (not a migration-time backfill — see design D4): recomputes rows whose `tss` is NULL or whose `tss_source` is computed (`power|pace|hr`); never touches `garmin`/`manual` rows. Re-runnable after threshold changes.
- **New MCP tool `recompute_workout_tss`** forwarding that endpoint 1:1; existing workout tools return the wider body (with `tss_source`) verbatim.
- No **BREAKING** changes: `tss` keeps its shape and validation; `tss_source` is additive with omitempty.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `workouts` — ADDED requirements: `tss_source` provenance column; ingest-time per-sport TSS derivation with precedence; recompute-tss backfill endpoint.
- `athlete-config` — MODIFIED requirement: the consumption gate ("capture-only") widens — `threshold_pace_sec_per_km`, `threshold_swim_pace_sec_per_100m`, `lactate_threshold_hr`/`threshold_hr`, and (transitively via `intensity_factor`) `ftp_watts` are now consumed by the workouts capability's TSS derivation. **No new config fields** — every threshold this change needs already exists on the singleton.

## Impact

- **Migration**: one new pair (next free slot after `054_sync_run_summary_partial` — verify head before creating, per convention): add `tss_source TEXT NULL` with a CHECK enum, a `(tss IS NULL) = (tss_source IS NULL)` pairing CHECK, and the provenance back-fill UPDATE.
- **Code**: `internal/workouts/` — `types.go` (field), `repo.go` (column in scan/insert/update + recompute candidates query), `service.go` (deriveTSS + precedence, wired next to `deriveIntensityFactor`; recompute service method), `handlers.go` (new `POST /workouts/recompute-tss` handler + swag annotations). `internal/httpserver/server.go`: no new wiring (the athlete-config repo is already cross-injected into the workouts service).
- **MCP**: `internal/agenttools/registry_workouts.go` — new `recompute_workout_tss` tool spec; announced-tools list derives from the registry automatically; regenerate the schema golden (`-tags=goldengen`).
- **Docs**: `task swag` after handler/type changes (checked-in `docs/` would otherwise drift).
- **Tests**: testcontainers integration tests per handler convention (`internal/workouts/*_test.go`), unit tests for each formula and the precedence chain, MCP golden/integration.
- **Not touched**: planned workouts (derivation gates on `status='completed'`), adherence `planned_tss` semantics, energy/summary math, race-prep — no consumer changes in this change.
