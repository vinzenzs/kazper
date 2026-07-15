## Why

**Production data-corruption bug (user-reported 2026-07-15).** The bridge's daily sync builds an athlete-config body purely from Garmin data (`map_athlete_config`) and **`PUT`s it to the full-replace `/athlete-config` singleton** ("Garmin source-of-truth" by design). Consequences, all observed live: user-confirmed values are silently overwritten every sync (confirmed MaxHR 196 clobbered by Garmin's `maxHeartRateUsed`), fields Garmin doesn't expose (swim CSS, functional threshold HR) are **cleared** nightly, `threshold_history` is polluted with a snapshot per clobber, and workouts synced while thresholds were cleared derived no TSS. This violates the line every recent change held — "threshold writes stay deliberate" — which this grandfathered daily PUT predates.

## What Changes

- **The bridge stops writing `/athlete-config` entirely.** Garmin detections are demoted from writes to advisory data: sync `PUT`s the mapped values to a new `PUT /api/v1/athlete-config/garmin-detected` — a latest-detection singleton (values + `detected_at`), accepted **only from the garmin identity**.
- **`PUT /athlete-config` rejects the garmin identity** (403, matching the existing identity-guard vocabulary) — the config singleton and `threshold_history` become purely deliberate, human/coach-confirmed records.
- **Both values are kept, and the coach sets which one computations use** (per user direction 2026-07-15): a per-field **source selector** (`manual` default | `garmin`) over the detectable fields (`ftp_watts`, `lactate_threshold_hr`, `max_hr`, `threshold_pace_sec_per_km`, `hr_zones`, `power_zones`), set via `PUT /athlete-config/sources` (non-garmin identities) and mirrored as a `set_threshold_sources` MCP write tool. An **effective config** resolves per field — the detection value where the source is `garmin` and a detection exists, the manual value otherwise — and computational consumers (TSS derivation, zone resolution, race pacing, compliance) read the effective view through one adapter at the wiring trunk; raw endpoints stay raw.
- `GET /athlete-config/garmin-detected` and `GET /athlete-config/effective` (mobile/agent/web); `/context/training` carries detection + sources + effective beside the config block so the coach sees drift and what's live in one read, and flips a source or applies a deliberate update from there.
- Migration (next free slot): `garmin_detected_thresholds` singleton table + a `garmin_sourced_fields` column on `athlete_config` (default empty = all-manual, preserving today's semantics; the config PUT full-replace does not touch it). Detection table classified **export-excluded** (latest-only, re-derived by the next sync); the sources column exports with the config row.
- Recovery runbook (operator tasks): deploy both sides → re-PUT the confirmed config (MaxHR 196; FTP 278 confirmed deliberately) → `POST /workouts/recompute-tss` to backfill the NULL-TSS workouts. Polluted `threshold_history` rows stay (retroactive edits remain a non-goal); history is trustworthy from the fix forward.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `garmin-bridge`: 1 MODIFIED requirement — the sync mapping/scenarios route physiology to the detection endpoint instead of the config PUT.
- `athlete-config`: 4 ADDED requirements — the garmin-detected singleton (write/read), the config PUT's garmin-identity rejection, the per-field source selector, and the effective-config resolution consumed by computations.
- `coach-context`: 1 ADDED requirement — the training context carries detection + sources + effective beside the config block.
- `mcp-server`: 1 ADDED requirement — the `set_threshold_sources` write tool.

## Impact

- **Code:** `internal/athleteconfig/` detection store + sources + effective resolution + endpoints + identity guard; **one effective-config adapter at the `httpserver.Run()` wiring trunk** so TSS derivation / zone resolution / race pacing / compliance consume the effective view without per-package changes; migration; context fold; bridge `sync.py` endpoint swap + tests; dataexport classification; MCP registry + golden; `task swag`.
- **Deploy sequencing:** backend first, then bridge (old bridge + new backend: its config PUT gets 403 and the sync's guarded per-capability accounting records the failure without aborting — acceptable for one cycle). All-manual default means behavior is unchanged until the coach flips a source.
- **Out of scope:** retroactive `threshold_history` cleanup, auto-apply rules (flipping a source is the coach's explicit, durable policy — not value-by-value automation), detection history (latest-only), historying effective drift while a field is garmin-sourced (threshold_history stays the record of deliberate confirmations).
