## Why

**Correctness bug found in production data (2026-07-15).** The windowed best-effort queries (`Repo.Curve` and the durability query) join `workouts` for status/date but never filter sport — so a run posting Garmin running power contaminates every cycling power analytic: a live `cp_model` read returned CP 56.9 W at r² 0.052 with 3 of 4 fit points sourced from a running workout, and `power_profile` anchored 5s/1m/5m on the same run. The symmetric axis is latent but equally real: bikes store `speed` best-efforts, so run/swim pace curves are contaminated by bike speeds. Until fixed, `power_curve`, `cp_model`, `cp_model_history`, `power_profile`, and `durability` are unreliable; the configured FTP is the better number.

## What Changes

- **Read-side sport scoping** on every windowed best-effort query: the `power` metric SHALL only aggregate best efforts from `sport = 'bike'` workouts; the `speed` metric SHALL only aggregate from the requested run/swim sport. Multisport workouts are excluded from sport-scoped windows (segment attribution doesn't exist at stream level — accepted loss, noted in responses' honesty story). No migration, no data rewrite — stored rows are fine; attribution was the bug.
- Per-workout reads (W′bal, quadrant, intervals, execution metrics) are unaffected and untouched; ingest keeps storing running power (it's real data — it just doesn't belong on the bike curve).
- **Fit-quality warning**: `cp_model` (and history anchors) gain `warning: "poor_fit"` when `r_squared < 0.5` — the contaminated fit passed the point-count/span gates, proving quality needs its own advisory flag. Non-breaking additive field.
- Regression tests seeding a running-power workout beside bike efforts across all five affected endpoints.

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `effort-analytics`: 2 ADDED requirements — windowed sport scoping (cross-cutting over the curve/CP/profile/durability reads) and the CP fit-quality warning.

## Impact

- **Code:** sport predicates in `internal/effortanalytics/repo.go` (both windowed queries + the profile's reuse), warning field in `cpmodel.go`; `task swag` (response field).
- **Data:** none — reads become correct immediately, including history.
- **Out of scope:** per-segment multisport attribution, running-power analytics as their own surface (a future capability if Stryd-style training ever matters), retroactive cleanup (nothing stored is wrong).
