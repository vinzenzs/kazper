## Context

`PUT /athlete-config` is deliberately full-replace, transactionally snapshotting `threshold_history` on change (`add-threshold-history`). The bridge's sync treats Garmin as config source-of-truth: `map_athlete_config` (FTP, LTHR, max HR from `maxHeartRateUsed`, threshold pace, Coggan-derived power zones; CSS and functional threshold HR unmappable) → `PUT /athlete-config` every sync. Two writers with opposite philosophies share one full-replace singleton; the automated one runs nightly and always wins — clobbering confirmed values (MaxHR 196), clearing Garmin-unknown fields, polluting threshold history, and NULLing TSS derived while thresholds were gone.

User direction (2026-07-15): **store both values and let the coach set which one to use** — not merely "coach copies Garmin's number when it looks right."

## Goals / Non-Goals

**Goals:**
- The config singleton and threshold history become exclusively deliberate records; the bridge can never again write them.
- Garmin detections stored and visible daily, side by side with configured values.
- A durable, coach-settable per-field policy choosing which value computations consume — trust-Garmin is an explicit decision made once, not a nightly ambush.
- All-manual default: shipping this changes no computed number until a source is flipped.

**Non-Goals:**
- Auto-apply heuristics ("adopt if within 3%") — the selector *is* the automation policy, set deliberately.
- Detection history (latest-only singleton) and historying effective drift while a field is garmin-sourced (`threshold_history` stays the record of deliberate confirmations; revisit if a garmin-sourced season needs auditability).
- Retroactive `threshold_history` cleanup (edits stay a non-goal; the pollution is bounded and dated).

## Decisions

### D1 — Split the write paths: bridge → detection singleton; config PUT rejects garmin
`garmin_detected_thresholds`: one row (fixed PK), the detectable physiology fields, `detected_at`, timestamps. `PUT /athlete-config/garmin-detected` full-replaces it, garmin identity only; `GET` open to the others. `PUT /athlete-config` gets the inverse guard (garmin → 403, the established identity-guard pattern). Old-bridge/new-backend transition: one cycle of visible, non-aborting `athlete_config: failed (403)` in the sync summary.

### D2 — Per-field source selector, stored on the config row, set via its own endpoint
`garmin_sourced_fields TEXT[]` on `athlete_config` (default `{}` = all manual), whitelisted tokens: `ftp_watts`, `lactate_threshold_hr`, `max_hr`, `threshold_pace_sec_per_km`, and the zone groups `hr_zones` / `power_zones` (zones flip as sets — mixed-source zone ladders are incoherent). `PUT /athlete-config/sources` (non-garmin identities; full-replace of the list; unknown token → `400 source_field_invalid`) mutates ONLY this policy column — and the config PUT's full-replace explicitly does not touch it (policy survives value confirmations). Mirrored as the `set_threshold_sources` MCP write tool; `GET /athlete-config` echoes `sources`.

- **Why on the config row, not the detection table?** It's policy about the config's consumption, owned by the human side; the detection row is garmin-owned and fully replaced by every sync.

### D3 — Effective config resolved centrally, consumed via one wiring adapter
`EffectiveConfig()`: per field, the detection value where the field's source is `garmin` **and** the detection carries it (absent/null detection field → manual fallback, never a hole); manual otherwise. Computational consumers — per-sport TSS derivation, `trainingplan.EffectiveProgram` zone resolution, race pacing, step compliance — already consume athlete-config through narrow injected interfaces, so a single effective-reading adapter at the `httpserver.Run()` wiring trunk switches them all without per-package edits. Raw endpoints stay raw (`GET /athlete-config` returns what was confirmed + `sources`; `GET /athlete-config/effective` returns the resolved view with per-field `source` annotations).

- **Why not make the plain GET return effective values?** PUT-then-GET must echo what was written (existing contract + tests), and hiding which layer a number came from is this bug's original sin.

### D4 — `ConfigAsOf`/threshold-history semantics unchanged
History snapshots remain tied to deliberate PUTs. A garmin-sourced field's day-to-day drift is *not* historied — accepted (non-goal) and honest: history answers "what did the athlete confirm, when"; the effective view answers "what is live now". Write-time consumers (per-sport TSS) snapshot the effective value into their own rows (`tss_source` already records provenance there).

### D5 — Drift and policy surface in `/context/training`
The bundle gains `garmin_detected` (values + `detected_at`), `threshold_sources` (the active list), and an `effective` block — configured, detected, chosen, all in one read. Null-safe throughout; nothing else in the bundle moves.

### D6 — Recovery is operational, not code
After both deploys: re-PUT the confirmed config (restores MaxHR 196; deliberately confirms FTP 278), optionally flip `ftp_watts` to `garmin` if that's the standing preference, then `POST /workouts/recompute-tss` to backfill the NULL-TSS workouts. All existing endpoints; listed as operator tasks.

## Risks / Trade-offs

- **A garmin-sourced field reintroduces silent nightly drift into computations** — now opt-in, per field, visible in context and annotated in the effective GET. That's the user's explicit trade to make.
- **Recompute interactions:** flipping a source changes effective thresholds without a config PUT, so the auto-nothing happens — the coach should run `recompute-tss` after a flip when it matters (task note + tool description).
- **Selector granularity is field-level, zones as groups** — coarser than per-zone but coherent; revisit only on demand.
- **One sync cycle of 403s during rollout** — bounded, visible, non-aborting.

## Migration Plan

Migration on the next free slot (**verify the on-disk head**): the detection table + the `garmin_sourced_fields` column (default `{}`). Additive; down drops both. Deploy backend → bridge → D6 runbook. Rollback = revert routes/guard/adapter (all-manual default means the adapter is identity until a source is flipped).

## Open Questions

- Should flipping a source auto-trigger `recompute-tss`? (v1: no — the flip is policy, recompute is a visible, deliberate action; the MCP tool description points at it.)
- Companion-app surfacing of detected-vs-configured (data's in the bundle; UI is a separate workstream).
