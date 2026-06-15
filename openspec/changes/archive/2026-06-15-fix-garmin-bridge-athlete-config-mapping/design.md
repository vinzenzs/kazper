## Context

`fetch_day` fetches `user_profile` (`get_user_profile`) and `userprofile_settings` (`get_userprofile_settings`); `map_athlete_config` (mapping.py:615) reads FTP/threshold/zone fields from them and `sync_day` issues `PUT /athlete-config`. Live captures show the mapper's assumptions are wrong on every field:

- `get_userprofile_settings()` → preferences only (`measurementSystem`, `timeFormat`, …); **no `userData`, no zones**.
- `get_user_profile().userData` (41 fields) holds `lactateThresholdHeartRate: 171`, `lactateThresholdSpeed: 0.269`, `ftpAutoDetected: true` (a flag), `weight`, VO2max — but **no FTP watts, no max HR, no `functionalThresholdHeartRate`, no zones**.
- `get_cycling_ftp()` → `{functionalThresholdPower: 255, sport: "CYCLING", …}`.
- `/biometric-service/heartRateZones` → a per-sport list; DEFAULT entry = floors `98/129/153/171/178` + `maxHeartRateUsed: 196`.
- Power zones: `/power-service/powerZones` 404, `/biometric-service/powerZones` 405 — no reachable source.

So `cfg` is always empty → `None` → `skipped (no data)`.

## Goals / Non-Goals

**Goals:**
- Populate `ftp_watts`, `lactate_threshold_hr`, `threshold_pace_sec_per_km`, `threshold_swim_pace_sec_per_100m`, `max_hr`, and `hr_zone_1..5_max` from the real endpoints.
- Keep the `safe()`-guarded, omit-when-absent, singleton-PUT behavior intact.

**Non-Goals:**
- `power_zone_*` — no reachable endpoint; left absent until one is found.
- A distinct functional `threshold_hr` separate from lactate-threshold HR (no source for this account).
- Backfilling historical config (singleton, source-of-truth overwrite each sync — unchanged).

## Decisions

- **Two new `safe()`-guarded fetches in `fetch_day`:** `raw["cycling_ftp"] = safe("cycling_ftp", lambda: api.get_cycling_ftp())` and `raw["heart_rate_zones"] = safe("heart_rate_zones", lambda: api.connectapi("/biometric-service/heartRateZones"))`. The HR-zones endpoint has no garminconnect helper, so a raw `connectapi` GET is used — consistent with how other raw paths are reached. Both guarded so an absent endpoint yields absent config, never an aborted day.
- **`map_athlete_config` reads the real sources:**
  - `ftp_watts` ← `_dig(raw, "cycling_ftp", "functionalThresholdPower")` (handles `get_cycling_ftp` returning a dict; if a list variant ever appears, take index 0).
  - `lactate_threshold_hr` ← `user_profile.userData.lactateThresholdHeartRate`.
  - `threshold_pace_sec_per_km` ← `lactateThresholdSpeed` (existing `_speed_to_pace_per_km`); swim from `lactateThresholdSwimSpeed`.
  - `max_hr` + HR-zone maxima from the DEFAULT-sport `heart_rate_zones` entry.
- **HR-zone floors → maxima.** Garmin returns *floors*; the schema wants per-zone *maxima* (upper bounds). Add `_hr_zone_maxima(entry)`: `hr_zone_N_max = zone(N+1)Floor` for N∈1..4, `hr_zone_5_max = maxHeartRateUsed`. Pick the entry with `sport == "DEFAULT"` (fall back to the first entry). This replaces the old `_zone_maxima(... "hr_zone")` call, which read `zoneHigh` from a non-existent settings list.
- **Power zones removed from the mapper** (the `_zone_maxima(... powerZones ..., "power_zone")` call is dropped) since the source 404/405s. `_zone_maxima` becomes unused for athlete-config; keep the helper only if another caller needs it, else remove to avoid dead code.
- **`threshold_hr` left unmapped** — no Garmin value distinct from lactate-threshold HR. Documented; the field stays absent (NULL) rather than duplicating the lactate value.
- **Fixture rebuilt from real captures.** Replace the fabricated `user_profile`/`userprofile_settings` athlete-config data with the real shapes and add `cycling_ftp` + `heart_rate_zones` blocks. Update the three `test_athlete_config_*` tests to the real fields and the floor→max derivation; drop power-zone assertions.

## Risks / Trade-offs

- [HR zones are per-sport; the schema is single-set] → Use DEFAULT (the device's general zones); document the choice. Sport-specific zones are not modeled by `athlete-config`.
- [Floor→max boundary convention (next-floor vs next-floor-1)] → Use the next zone's floor as the boundary max (matches how `zoneHigh` was treated); zone 5 uses `maxHeartRateUsed`. Coaching-grounding precision, not a hard threshold.
- [`get_cycling_ftp` return shape (dict vs list across lib versions)] → `_dig` on the dict; add an index-0 fallback if a list is observed. Captured response is a dict.
- [Only one account sampled] → Fixtures built from a genuine response; absent fields (swim speed, power zones) simply omit, never crash.
- [Garmin overwrites manual `PUT /athlete-config` edits on next sync] → Pre-existing, documented behavior; this change makes the sync actually write, which is the intent.
- [`lactateThresholdSpeed` is not reliably m/s] → Observed live: the account's value (`0.269`) yields a nonsense ~3711 s/km pace. The m/s→pace helpers now drop results outside a plausible band (run ≈90–1200 s/km, swim ≈30–600 s/100m), mirroring `_progress_pct`'s out-of-range omission, so a garbage source omits the field rather than storing a misleading pace. Confirming the true unit is a follow-up.
