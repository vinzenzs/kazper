## 1. Types + format helpers

- [x] 1.1 `api/types.ts`: extend `AthleteConfig` to the full Go shape — `threshold_hr`, `lactate_threshold_hr`, `max_hr`, `threshold_pace_sec_per_km`, `threshold_swim_pace_sec_per_100m`, `hr_zone_1..5_max`, `power_zone_1..5_max` (all nullable). Drop the catch-all index signature in favor of the explicit fields.
- [x] 1.2 `lib/format.ts`: add `raceTime(seconds)` (`mm:ss` < 1h, `h:mm:ss` ≥ 1h), `pace(secPerKm)` (`m:ss /km`), `pace100(secPer100m)` (`m:ss /100m`). All null-tolerant → `PLACEHOLDER`.
- [x] 1.3 `lib/format.ts` (or a small `lib/status.ts`): a `training_status` → `{label, colorClass}` map with an unknown-status fallback.

## 2. New panels

- [x] 2.1 `FitnessPanel`: VO₂max running + cycling, `training_status` badge, endurance score, hill score, fitness age — from the training context's `fitness` snapshot. Reuse `Panel` + `Stat`.
- [x] 2.2 `RacePredictions`: 5k / 10k / half / full via `raceTime`. Empty state when all null.
- [x] 2.3 `PowerThresholds`: FTP (`ftp_watts`), watts/kg (`watts_per_kg`), threshold HR, max HR, lactate-threshold HR, threshold pace (`pace`), threshold swim pace (`pace100`).
- [x] 2.4 `ZoneStrip` (visx): a banded Z1–Z5 strip with labeled boundaries; render present bands only, empty state when none. Used twice — HR (`hr_zone_*_max`, ≤bpm) and power (`power_zone_*_max`, ≤W) — composed in a `Zones` panel.

## 3. Extend existing panels

- [x] 3.1 `RecoverySnapshot`: add sleep score, average stress, body battery (charged / drained) alongside HRV · sleep · resting HR · readiness; widen the grid as needed.
- [x] 3.2 `Header`: render the `training_status` badge next to phase/season (the at-a-glance home).
- [x] 3.3 `WorkoutList`: show `recent_load.by_sport` as chips on the recent-workouts header.

## 4. Layout

- [x] 4.1 `App.tsx`: slot the new panels into the responsive grid (Fitness full-width; Race predictions + Power/thresholds paired; Zones full-width; Recovery extended). Accept the taller page.

## 5. Tests + verification

- [x] 5.1 Render test per new panel against fixture context payloads — populated and null-heavy (every metric absent → placeholders / empty state, no crash).
- [x] 5.2 Format-helper unit tests: `raceTime` (sub-hour vs. over-hour), `pace`, `pace100`, and the `training_status` map (known + unknown).
- [x] 5.3 `ZoneStrip` test: full zones render five bands; sparse zones render present bands only; no zones → empty state.
- [x] 5.4 `task web:build` produces a committed-clean `apps/web/dist` diff; `task vet` + `task test` green; `task build` works without a Node toolchain.
- [x] 5.5 Live smoke: with `WEB_USER`/`WEB_PASSWORD` set, load `/` and confirm the new panels render from live Garmin-fed data and degrade gracefully where fields are absent.
