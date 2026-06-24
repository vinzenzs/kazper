## Why

The coach dashboard already *fetches* far more than it renders. `GET /api/v1/context/training`
and `GET /api/v1/context/recovery` carry a rich fitness/performance picture — VO₂max, race
predictions, training status, endurance/hill scores, fitness age, watts/kg, FTP, thresholds,
full HR + power zone tables, and recovery extras (sleep score, stress, body battery) — but the
v1 panels surface only ACWR, the acute/chronic load trend, a four-stat recovery snapshot, and
the workout lists. The rest arrives on the wire and is dropped on the floor.

This change renders that already-fetched data. It is **pure presentation**: no new endpoints,
no new fetches, no change to the data contract, auth, or serving. The fueling / energy-
availability / nutrition domain remains out of scope (that's a separate, deferred lens).

## What Changes

- **New "Fitness" panel** — VO₂max (running / cycling), a `training_status` badge, endurance
  score, hill score, fitness age.
- **New "Race predictions" panel** — 5k / 10k / half / full, formatted as race times.
- **New "Power & thresholds" panel** — FTP, watts/kg, threshold HR, max HR, lactate-threshold
  HR, threshold pace, threshold swim pace.
- **New "Zones" panel** — HR and power 5-zone tables rendered as compact horizontal banded
  strips (visx), each band labeled with its boundary; degrades to whatever boundaries exist.
- **Extended Recovery panel** — add sleep score, average stress, and body battery
  (charged / drained) to the existing HRV · sleep · resting HR · readiness stats.
- **By-sport chips** — `recent_load.by_sport` shown as chips on the recent-workouts header.
- **New format helpers** — `raceTime(seconds)` (e.g. `18:32` / `1:24:05`), `pace(sec/km)`
  (`4:05 /km`), `pace100(sec/100m)` (`1:38 /100m`), and a `training_status` → badge/color map.
- **Type fill-out** — extend the dashboard's `AthleteConfig` TS type to the full Go shape
  (thresholds + the ten zone fields). Frontend-only; the data already flows.
- **Layout** — the page grows taller to fit everything (explicit decision: surface all the
  data rather than cut for density); panels slot into the existing responsive grid.

Every added metric is nullable and SHALL degrade to a placeholder when absent, matching the
existing panel behavior.

## Capabilities

### New Capabilities
<!-- none — this expands the existing coach-dashboard display surface -->

### Modified Capabilities

- `coach-dashboard`: the training view SHALL additionally present the fitness/performance,
  race-prediction, power/threshold, and zone data already carried by the context payloads, plus
  the recovery extras — while fueling / energy-availability / nutrition panels remain out of
  scope.

## Impact

- **Frontend only** (`apps/web/`): new panel components (`FitnessPanel`, `RacePredictions`,
  `PowerThresholds`, `ZoneStrip`), an extended `RecoverySnapshot`, by-sport chips on
  `WorkoutList`, new helpers in `lib/format.ts`, a fuller `AthleteConfig` type in
  `api/types.ts`, and `App.tsx` layout wiring. New render tests per panel (null-heavy fixtures).
- **No backend change** — no new endpoints, fetches, data-contract, auth, or serving changes.
  The three existing queries (`/context/training`, `/context/recovery`, `/fitness-metrics`)
  already carry every field.
- **Build** — `task web:build` regenerates the committed `apps/web/dist`; `task build` still
  needs no Node toolchain (embedded committed dist, the `docs/` precedent).
- **Scope** — Tier A only (render what's fetched). Net-new data sources — weight/race/PR
  boards (Tier B) and the fueling/EA/nutrition lens (Tier C) — stay deferred.
