## 1. Fetch the real endpoints

- [x] 1.1 In `garmin_client.fetch_day`, add `raw["cycling_ftp"] = safe("cycling_ftp", lambda: api.get_cycling_ftp())`
- [x] 1.2 In `fetch_day`, add `raw["heart_rate_zones"] = safe("heart_rate_zones", lambda: api.connectapi("/biometric-service/heartRateZones"))` (raw call — no garminconnect helper)

## 2. Mapping fix

- [x] 2.1 Rewrite `map_athlete_config`: `ftp_watts` ← `cycling_ftp.functionalThresholdPower`; `lactate_threshold_hr`/`threshold_pace_sec_per_km`/`threshold_swim_pace_sec_per_100m` ← `user_profile.userData.{lactateThresholdHeartRate,lactateThresholdSpeed,lactateThresholdSwimSpeed}`; drop the `userprofile_settings.userData`, `ftpAutoDetected`, `functionalThresholdHeartRate`, and `maxHeartRate`-from-userData paths
- [x] 2.2 Add `_hr_zone_maxima(entry)` deriving `hr_zone_1..5_max` from the DEFAULT-sport heart-rate-zones entry (zone N max = zone N+1 floor for N 1..4; zone 5 max = `maxHeartRateUsed`) and set `max_hr` ← `maxHeartRateUsed`; select the entry with `sport == "DEFAULT"`, else the first
- [x] 2.3 Remove the power-zone read (and the now-unused `_zone_maxima` if no other caller); leave `threshold_hr` unmapped (no distinct Garmin source)

## 3. Fixture + tests

- [x] 3.1 Rebuild the athlete-config slice of `tests/fixtures/garmin_day.json` from the real captures: real `user_profile.userData` (lactate threshold HR/speed, `ftpAutoDetected: true`), preferences-only `userprofile_settings`, plus `cycling_ftp` and a per-sport `heart_rate_zones` block
- [x] 3.2 Update `test_athlete_config_mapping` to assert `ftp_watts`, `lactate_threshold_hr`, threshold paces, `max_hr`, and the floor-derived `hr_zone_*` from the real shape
- [x] 3.3 Replace `test_athlete_config_power_zones_absent` with a power-zones-omitted case and update `test_athlete_config_empty_yields_none`; add a case asserting preferences-only settings and the `ftpAutoDetected` flag are not used as sources

## 4. Verification

- [x] 4.1 Run the bridge test suite (`apps/garmin-bridge` pytest) — confirm green
- [x] 4.2 End-to-end: start the bridge, sync a recent day, confirm `athlete_config: updated` in the summary and `GET /context/training` returns a non-null `watts_per_kg` (FTP ÷ latest bodyweight)
- [x] 4.3 Delete the `/tmp/*.json` biometric captures once the fixture is built
