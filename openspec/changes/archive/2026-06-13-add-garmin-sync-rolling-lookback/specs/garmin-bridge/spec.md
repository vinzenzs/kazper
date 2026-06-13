## MODIFIED Requirements

### Requirement: Headless daily sync maps Garmin data to the REST API

The bridge SHALL expose `POST /sync` that reads the stored token from the backend (`GET /garmin/token`), obtains a fresh access token without any interactive step, fetches Garmin data, and writes it to the existing nutrition REST API under `GARMIN_API_TOKEN`.

`POST /sync` SHALL select the days it syncs as follows:
- **With an explicit `date`** in the request body: the bridge syncs exactly that one day (no lookback).
- **With no `date`**: the bridge syncs a bounded **rolling window** of recent days — today and the previous `SYNC_LOOKBACK_DAYS` days (a configurable non-negative integer, default `2`, yielding a 3-day window). The window is ordered oldest-to-newest and bounded entirely by `SYNC_LOOKBACK_DAYS`; `SYNC_LOOKBACK_DAYS = 0` reduces the window to today only.

The rolling window exists because a day's completed activities and Garmin's recomputed training-load/VO2max/race-predictor metrics are not available at the moment a same-day early-morning sync runs; re-pulling recent days lets those late-arriving signals land. Every day in the window SHALL be synced through the same per-day fetch-and-map path used for a single explicit date, so all mappings, per-capability failure tolerance, and idempotent upserts apply unchanged per day. Each day in the window SHALL be synced independently and tolerant of a single failing day — one day raising an error SHALL NOT abort the remaining days — and the response SHALL report a per-day result for the window.

The mapping SHALL be: sleep/HRV/RHR/stress → `/recovery-metrics`; VO2max/training-load → `/fitness-metrics`; sweat loss → `/hydration-balance`; whole-day energy/activity totals → `/daily-summary`; weigh-ins → `/weight`; activities → `/workouts` (`source = "garmin"`), where each activity additionally carries the scalar performance and HR-zone fields plus nested `splits`/`sets` detail when Garmin provides them; gear inventory → `/gear` (upsert by Garmin gear id); personal records → `/personal-records` (upsert by Garmin PR id); the athlete's physiology configuration (FTP, thresholds, max HR, lactate-threshold HR, HR-zone and optional power-zone boundaries) → `PUT /athlete-config` as a non-date-keyed singleton refresh (in-place overwrite, Garmin source-of-truth); device inventory → `/devices`; blood pressure / all-day HR / all-day stress → `/health-vitals`; and earned badges / ad-hoc challenges → `/achievements`. Gear, personal records, devices, and achievements are slowly-changing inventory refreshed via idempotent upsert on each sync, not date-keyed snapshots; the device, health-vitals, and achievement targets are reference/coaching context only and feed no nutrition computation. Each per-capability fetch is guarded so its failure does not abort the day. Sync SHALL require no MFA or human interaction.

#### Scenario: Dateless sync syncs a rolling window of recent days

- **WHEN** `POST /sync` runs with no `date` and a valid stored token, with `SYNC_LOOKBACK_DAYS = 2`
- **THEN** the bridge syncs three days — today, yesterday, and the day before — each through the same per-day fetch-and-map path
- **AND** the late-arriving signals for recent days (each day's completed activities and Garmin's recomputed training-load/VO2max/race-predictor metrics) are picked up on the days they become available
- **AND** the response reports a per-day result for each day in the window

#### Scenario: Explicit date syncs exactly that day

- **WHEN** `POST /sync` runs with an explicit `date` in the body
- **THEN** the bridge syncs only that single day and applies no lookback window
- **AND** the targeted-day and CronJob first-run-backfill behaviors are preserved unchanged

#### Scenario: One failing day does not sink the window

- **WHEN** a dateless `POST /sync` runs over a multi-day window and one day's fetch or write raises an error
- **THEN** the bridge records that day as failed and continues syncing the remaining days in the window
- **AND** the response reports the per-day outcome, marking the failed day without aborting the others

#### Scenario: Daily sync writes a day's data

- **WHEN** `POST /sync` runs with a valid stored token
- **THEN** the bridge refreshes its access token without prompting for MFA
- **AND** posts the day's recovery, fitness, hydration-balance, daily-summary,
  weight, and activity data to their respective endpoints under the garmin
  identity
- **AND** each activity item carries the available scalar/zone/split/set detail
- **AND** upserts the current gear and personal-record inventory to `/gear` and
  `/personal-records`
- **AND** refreshes the athlete physiology config via `PUT /athlete-config` when
  Garmin provides it
- **AND** additionally upserts the day's device inventory, health-vitals snapshot,
  and earned achievements when Garmin provides them

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date (whether targeted directly or revisited by the rolling window on a later run)
- **THEN** the date-keyed metrics (including `/daily-summary` and `/health-vitals`) are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** each activity's nested splits and sets are replaced (not duplicated) on the second run
- **AND** gear and personal records are upserted by their Garmin external id, and
  the athlete config is re-written in place via the singleton `PUT`
- **AND** devices and achievements are deduped by `external_id`, and the
  health-vitals snapshot is upserted by `date` (no duplicates on the second run)

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing
