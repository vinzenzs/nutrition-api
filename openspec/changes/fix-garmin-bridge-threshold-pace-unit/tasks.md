## 1. Conversion fix

- [x] 1.1 Rewrite `_speed_to_pace_per_km`/`_speed_to_pace_per_100m` as `_pace_per_km`/`_pace_per_100m` taking `sec_per_m`: `pace = sec_per_m * 1000` (km) / `* 100` (100m); keep the plausibility-band guard; update docstrings to state the s/m unit
- [x] 1.2 Update the call sites in `map_athlete_config` (field values unchanged — still `lactateThresholdSpeed`/`lactateThresholdSwimSpeed`)

## 2. Fixture + tests

- [x] 2.1 Set the fixture `user_profile.userData.lactateThresholdSpeed`/`lactateThresholdSwimSpeed` to realistic s/m values (e.g. `0.25` → 250 s/km, `1.5` → 150 s/100m)
- [x] 2.2 Update `test_athlete_config_mapping` pace assertions to the corrected math (250.0 / 150.0)
- [x] 2.3 Repurpose `test_athlete_config_drops_implausible_threshold_pace` to a value genuinely out-of-band under the correct unit (the old `0.269` is now valid → 269.4)

## 3. Verification

- [x] 3.1 Run the bridge test suite (`apps/garmin-bridge` pytest) — confirm green
- [x] 3.2 End-to-end: start the bridge, re-sync 2026-06-14, confirm `GET /athlete-config` now carries a sensible `threshold_pace_sec_per_km` (≈269 s/km for this account)
