## Context

The dashboard (`apps/web/`) renders five panels (Header, FormGauge, LoadTrend,
RecoverySnapshot, two WorkoutLists) from three queries: `/api/v1/context/training`,
`/api/v1/context/recovery`, and `/api/v1/fitness-metrics` (the 42-day acute/chronic history).
The training payload already carries a full `FitnessSnapshot` (VO₂max, race predictors,
training status, endurance/hill score, fitness age), `watts_per_kg`, and `athlete_config`
(FTP, thresholds, HR + power 5-zone tables); the recovery payload's `latest` carries sleep
score, average stress, and body battery. The panels render none of those.

This change is a Tier A expansion: render the already-fetched fields. It touches only the
frontend. The display contract is owned by the `coach-dashboard` capability.

## Goals / Non-Goals

**Goals:**
- Surface every fitness/performance/threshold/zone field the context payloads already carry.
- Show the HR and power zones visually (a banded strip), not just as numbers.
- Keep the null-tolerant, glance-able panel idiom the v1 panels established.

**Non-Goals:**
- **No new data sources.** No new endpoints/fetches; Tier B (weight/race/PR boards) and Tier C
  (fueling / energy-availability / nutrition) stay deferred.
- **No backend change.** Data contract, auth, serving untouched.
- **No density-driven cuts.** The page may grow taller; that is the accepted trade.

## Decisions

### D1 — New panels for new concepts; extend Recovery in place

VO₂max/status/scores, race predictions, and power/thresholds are distinct glance-units, so each
gets its own panel (`FitnessPanel`, `RacePredictions`, `PowerThresholds`) reusing the existing
`Panel` + `Stat` primitives. The recovery extras (sleep score, stress, body battery) are the
same concept as today's recovery snapshot, so they extend `RecoverySnapshot` (4 → 7 stats)
rather than spawning a panel.

### D2 — Zones as a banded strip, not a number table

The user wants to *see* the zones, and visx is already a dependency. A `ZoneStrip` renders Z1–Z5
as proportional colored segments with each boundary labeled — readable at a glance, worth the
vertical space. Two strips: HR (`hr_zone_*_max`, ≤bpm) and power (`power_zone_*_max`, ≤W). Each
zone boundary is independently nullable; the strip renders whatever boundaries are present and
shows the panel's empty state when none are.

### D3 — Accept a taller page

Tier A roughly doubles on-screen metrics. Rather than gate panels for density, the layout grows
vertically (explicit user decision: surface everything). Panels slot into the existing
responsive grid; on a monitor it reads as one tall board, on a laptop it scrolls.

### D4 — New format helpers for time-shaped fields

`lib/format.ts` has `num`, `duration` (minutes→"Xh Ym"), `sleep` (seconds→"Xh Ym"), date
helpers, and `titleCase`. None fit race times or paces, so add:
- `raceTime(seconds)` → `mm:ss` under an hour, `h:mm:ss` over (e.g. `18:32`, `1:24:05`).
- `pace(secPerKm)` → `m:ss /km` (e.g. `4:05 /km`).
- `pace100(secPer100m)` → `m:ss /100m` (e.g. `1:38 /100m`).
- `training_status` → a badge label + color class (productive / maintaining / detraining /
  overreaching / recovery / unproductive / peaking …), with an unknown-status fallback.

### D5 — Fill out the `AthleteConfig` TS type

The dashboard's `AthleteConfig` currently models only `ftp_watts` plus an index signature.
Extend it to mirror `internal/athleteconfig/types.go` (threshold_hr, lactate_threshold_hr,
max_hr, threshold_pace_sec_per_km, threshold_swim_pace_sec_per_100m, hr_zone_1..5_max,
power_zone_1..5_max). No backend change — these already serialize into `/context/training`.

### D6 — training_status badge placement

Render the `training_status` badge in the Header (next to phase/season) so the headline state
is visible without scrolling, and also as a stat in the Fitness panel for completeness. Header
is the at-a-glance home; the panel is the detail home.

## Risks / Trade-offs

- **Visual noise.** More panels risk a busy board. Mitigation: reuse the one `Panel`/`Stat`
  idiom and consistent spacing so density reads as a dashboard, not a dump.
- **Zone-strip with sparse data.** Missing inner boundaries make proportional widths ambiguous.
  Mitigation: render present bands only, label boundaries, fall back to the empty state when no
  zones exist; never infer a missing boundary.
- **`training_status` vocabulary drift.** Garmin's status strings may include values not in the
  color map. Mitigation: an explicit fallback badge style for unknown statuses (no crash).
- **Committed `dist` drift.** Same as the parent change; `task web:build` + the `docs/`-style
  CI diff discipline covers it.

## Migration Plan

Frontend-only. Implement panels + helpers + type, wire into `App.tsx`, add render tests, run
`task web:build` to refresh the committed `dist`. No server, migration, or deploy steps.

## Open Questions

- Zone strip: proportional widths (band size ∝ its range) vs. equal-width segments with boundary
  labels? Leaning proportional, but equal-width is more legible when ranges are very uneven.
- Body battery: show charged and drained separately, or a single net figure? Leaning both, as
  two small stats, since they tell different stories.
