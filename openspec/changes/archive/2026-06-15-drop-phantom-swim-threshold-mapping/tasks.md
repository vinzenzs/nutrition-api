## 1. Remove the phantom mapping

- [x] 1.1 In `mapping.py`, drop the `threshold_swim_pace_sec_per_100m` entry from `map_athlete_config`
- [x] 1.2 Remove the now-orphaned `_pace_per_100m` helper (keep `_pace_per_km`); update the `map_athlete_config` docstring to note swim threshold has no reachable source

## 2. Fixture + tests

- [x] 2.1 Remove the `lactateThresholdSwimSpeed` key from `tests/fixtures/garmin_day.json`
- [x] 2.2 Remove the `threshold_swim_pace_sec_per_100m` assertion from `test_athlete_config_mapping`; add a short assertion (or case) that the field is absent

## 3. Verification

- [x] 3.1 Run the bridge test suite (`apps/garmin-bridge` pytest) — confirm green
- [x] 3.2 `grep -r "_pace_per_100m\|threshold_swim_pace\|lactateThresholdSwimSpeed" apps/garmin-bridge` returns no stray references in bridge code/fixtures
