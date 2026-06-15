## Why

`athlete_config` is never populated — every daily sync reports `athlete_config: skipped (no data)`, so `PUT /athlete-config` is never issued, FTP/zones stay null, and the `watts_per_kg` we just added to `/context/training` can never compute. Verified against live payloads: `map_athlete_config` reads `userprofile_settings.userData`, but `get_userprofile_settings()` returns **only preferences** (no `userData`); the real `userData` lives under `get_user_profile()`. FTP watts is not in any fetched payload at all (`userData.ftpAutoDetected` is a boolean flag, not a value), and the configured HR zones / max-HR are in a Garmin endpoint the bridge never calls. The synthetic test fixture invents a shape Garmin never returns, so tests are green while production writes nothing.

## What Changes

- Fetch the two endpoints that actually carry the data, each `safe()`-guarded in `fetch_day`:
  - `get_cycling_ftp()` → `functionalThresholdPower` (live value: **255**) for `ftp_watts`.
  - `/biometric-service/heartRateZones` (raw `connectapi`; no garminconnect helper) → per-sport zone floors + `maxHeartRateUsed` for `max_hr` and HR-zone maxima.
- Repoint `map_athlete_config` to the **real** sources/field names:
  - `ftp_watts` ← `cycling_ftp.functionalThresholdPower`.
  - `lactate_threshold_hr` ← `user_profile.userData.lactateThresholdHeartRate`; `threshold_pace_sec_per_km` ← `lactateThresholdSpeed`; `threshold_swim_pace_sec_per_100m` ← `lactateThresholdSwimSpeed` (omitted when absent).
  - `max_hr` ← `heartRateZones[DEFAULT].maxHeartRateUsed`.
  - `hr_zone_1..5_max` ← derived from the DEFAULT-sport zone floors (zone N max = zone N+1 floor; zone 5 max = `maxHeartRateUsed`).
- Drop the dead/wrong paths (`userprofile_settings.userData`, `ftpAutoDetected`, `functionalThresholdPower` in userData, `functionalThresholdHeartRate`, zones under `userprofile_settings`).
- Rebuild the synthetic `athlete_config` slice of `tests/fixtures/garmin_day.json` from real captures (`/tmp/ftp_zones.json`, `/tmp/profile.json`) and update `test_mapping.py`.
- **Out of scope (documented):** **power zones** — `/power-service/powerZones` and `/biometric-service/powerZones` return 404/405, no reachable source found, so `power_zone_*` stays absent; and a distinct functional `threshold_hr` — Garmin exposes no value separate from lactate-threshold HR for this account.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `garmin-bridge`: correct the source endpoints/paths the athlete-physiology-config refresh reads (FTP, threshold HR/pace, max HR, HR-zone maxima), and scope power zones out until a source is found.

## Impact

- **Code:** `apps/garmin-bridge/garmin_bridge/garmin_client.py` (`fetch_day`: two new guarded fetches), `apps/garmin-bridge/garmin_bridge/mapping.py` (`map_athlete_config` + HR-zone-floor → maxima helper).
- **Tests/fixtures:** `apps/garmin-bridge/tests/fixtures/garmin_day.json`, `apps/garmin-bridge/tests/test_mapping.py`.
- **Downstream (no code change):** once `ftp_watts` lands via `PUT /athlete-config`, `/context/training.watts_per_kg` populates automatically.
- **No backend / REST / MCP change.**
