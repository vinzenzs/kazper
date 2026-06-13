# garmin-bridge Specification (delta)

## ADDED Requirements

### Requirement: The bridge refreshes the athlete physiology config each sync

The bridge SHALL, on each daily sync, fetch the athlete's Garmin physiology configuration — FTP, thresholds, max HR, and HR-zone (and any power-zone) boundaries — from the user-profile and user-settings payloads (`get_user_profile` / `get_userprofile_settings`, the source endpoints actually exposed by the Garmin client; the zone boundaries ride in the user-settings payload), map them to the `athlete-config` shape, and write them to the backend via `PUT /athlete-config`. Because this configuration is slowly-changing physiology and NOT a date-keyed snapshot, the refresh is a single in-place singleton upsert (not one row per day): the same `PUT /athlete-config` is re-issued each sync and overwrites the prior config (Garmin is source-of-truth). Each config fetch SHALL be individually guarded by the existing `safe()` pattern so a failing, throttled, or account-unavailable Garmin endpoint yields absent config for that sync — never an aborted day. The mapper SHALL attach whatever fields are present and omit what is absent.

#### Scenario: Config is fetched, mapped, and written via the singleton PUT

- **WHEN** a daily sync runs and Garmin's profile/settings payload carries FTP and threshold HR and five HR-zone boundaries
- **THEN** the bridge maps them and issues `PUT /athlete-config` with `ftp_watts`, `threshold_hr`, and `hr_zone_1_max..hr_zone_5_max`
- **AND** a field absent from Garmin's response is simply omitted from the request body

#### Scenario: Config refresh is a non-date-keyed singleton overwrite

- **WHEN** `POST /sync` is run on two different dates
- **THEN** each run re-issues `PUT /athlete-config` overwriting the single config row in place (no per-day config rows accumulate)
- **AND** the most recent Garmin values win (Garmin is source-of-truth for these fields)

#### Scenario: A failing profile or zone fetch does not abort the day

- **WHEN** `get_user_profile` or `get_heart_rate_zones` raises or returns nothing during a sync
- **THEN** the bridge omits the corresponding config fields (or skips the `PUT` when nothing was obtained)
- **AND** the rest of the day's recovery, fitness, hydration-balance, weight, and activity sync still completes

## MODIFIED Requirements

### Requirement: Headless daily sync maps Garmin data to the REST API

The bridge SHALL expose `POST /sync` (optionally for a specific date, default
today) that reads the stored token from the backend (`GET /garmin/token`),
obtains a fresh access token without any interactive step, fetches the day's
Garmin data, and writes it to the existing nutrition REST API under
`GARMIN_API_TOKEN`. The mapping SHALL be: sleep/HRV/RHR/stress →
`/recovery-metrics`; VO2max/training-load → `/fitness-metrics`; sweat loss →
`/hydration-balance`; whole-day energy/activity totals → `/daily-summary`;
weigh-ins → `/weight`; activities → `/workouts` (`source = "garmin"`), where
each activity additionally carries the scalar performance and HR-zone fields
plus nested `splits`/`sets` detail when Garmin provides them; gear inventory →
`/gear` (upsert by Garmin gear id); personal records → `/personal-records`
(upsert by Garmin PR id); and the athlete's physiology configuration (FTP,
thresholds, max HR, lactate-threshold HR, HR-zone and optional power-zone
boundaries) → `PUT /athlete-config` as a non-date-keyed singleton refresh
(in-place overwrite, Garmin source-of-truth), guarded so its fetch failure does
not abort the day. Gear and personal records are slowly-changing inventory
refreshed via idempotent upsert on each sync, not date-keyed snapshots. Sync
SHALL require no MFA or human interaction.

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

#### Scenario: Re-running a day is idempotent

- **WHEN** `POST /sync` is run twice for the same date
- **THEN** the date-keyed metrics (including `/daily-summary`) are upserted (not duplicated)
- **AND** activities are deduped by `external_id = "garmin:<activity_id>"` via the
  existing `/workouts` UPSERT (no new field or migration)
- **AND** each activity's nested splits and sets are replaced (not duplicated) on the second run
- **AND** gear and personal records are upserted by their Garmin external id
  (re-observing the same item updates it in place, no duplicate)
- **AND** the athlete config is re-written in place via the singleton `PUT`
  (no per-day config rows accumulate)

#### Scenario: Sync with no stored token fails clearly

- **WHEN** `POST /sync` runs and the backend has no stored token (`404`)
- **THEN** the bridge returns an error indicating a login is required
- **AND** writes nothing
