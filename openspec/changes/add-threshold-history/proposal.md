# Add threshold history to athlete-config

## Why

The `athlete_config` singleton only knows the athlete's *current* physiology —
every `PUT` full-replaces it, so "FTP went 240 → 255 → 270 this season" is
unrecoverable, even though threshold progression is itself a first-class
coaching signal (TrainingPeaks dates every threshold change) and future
consumers (per-sport TSS, zone resolution, race pacing) will eventually want
the thresholds that were true *on a workout's date*, not today's.

## What Changes

- **New append-only history table** `athlete_config_history`: whenever
  `PUT /athlete-config` changes any physiology field, the resulting full config
  state is snapshotted with an `effective_from` date. No-change PUTs (notably
  the daily Garmin sync re-PUT) append **nothing**; a second change on the same
  date replaces that date's snapshot. The singleton stays the authoritative
  "current" read — `GET`/`PUT /athlete-config` behavior is completely
  unchanged (no breaking change).
- **New read endpoint** `GET /athlete-config/history?from=&to=` returning the
  dated snapshots ascending, so the coach agent can answer "how has my FTP
  developed" from data instead of memory.
- **Service-level as-of resolution**: `ConfigAsOf(date)` returns the config
  state effective on a given date (latest snapshot with
  `effective_from <= date`). Provided but **not wired into any existing
  consumer** in this change — `add-per-sport-tss` (write-time snapshot
  semantics), `add-step-compliance` (zone resolution), and
  `add-race-pacing-plan` (compute-on-read against current thresholds) keep
  their current-value semantics; rewiring each is an explicit per-consumer
  follow-up.
- **Migration-time seed**: the migration inserts one baseline snapshot from
  the existing singleton (sentinel `effective_from = 1970-01-01`), so history
  is never empty once a config exists and `ConfigAsOf` is total for any date.
- **MCP tool mirroring 1:1**: `athlete_config_history_get` (read) →
  `GET /athlete-config/history`, via the shared `agenttools` registry;
  announced surface is registry-derived, schema golden regenerated with
  `-tags=goldengen`.

## Capabilities

### New Capabilities

None. Threshold history is versioning *of* the athlete-config state — same
fields, same table family, same package — not a new domain concern; splitting
it out would give two capabilities one source of truth.

### Modified Capabilities

- `athlete-config`: gains four **ADDED** requirements (history snapshot on
  change, history read endpoint, migration seed, as-of resolution). The
  existing requirements — including the consumption-gate requirement that both
  in-flight siblings `add-per-sport-tss` and `add-race-pacing-plan` MODIFY —
  are deliberately **not touched**, to avoid a triple-delta conflict on that
  block.
- `mcp-server`: one ADDED requirement for the history read tool.

## Impact

- **Specs**: ADDED-only deltas on `athlete-config` and `mcp-server`. No
  MODIFIED blocks anywhere; `fitness-metrics` (VO2max etc.) and
  `http-error-shape` are untouched.
- **Code**: `internal/athleteconfig/` only — history repo methods + snapshot
  type, a write hook in `Service.Put`, `Service.ConfigAsOf`, one new handler
  on the existing `Register`. No `httpserver/server.go` wiring change beyond
  what already exists (route registers on the same group).
- **One migration pair** creating `athlete_config_history` + seed insert.
  Head is `054_sync_run_summary_partial` at writing, but sibling in-flight
  changes also claim slots — **verify the highest existing number first**.
- **MCP**: one new registry entry beside the existing `athlete_config_get`;
  regenerate the announced-schema golden (`go test -tags=goldengen ...`) — the
  expected-tools surface is registry-derived post `unify-mcp-tool-registry`
  (do not hand-edit a tools list).
- **Garmin bridge**: no change. The daily sync already re-issues
  `PUT /athlete-config` with Garmin-detected FTP/LTHR/threshold-pace, so
  detected threshold changes flow into history automatically through the PUT
  hook; the no-change dedup keeps the daily re-PUT silent.
- **Docs**: `task swag` after the handler; README MCP table row.

### Out of scope (explicit non-goals)

- Rewiring `add-per-sport-tss`, `add-step-compliance`, or
  `add-race-pacing-plan` (or the existing IF derivation / zone-target
  resolution) to use `ConfigAsOf` — their current-value / write-time-snapshot
  semantics are already reasonable; each rewiring is its own follow-up.
- Retroactive edit/correction of history rows (no PUT/PATCH/DELETE on
  history in v1).
- A dedicated Garmin auto-ingest path for detected thresholds — the manual/
  bridge `PUT` remains the only writer.
