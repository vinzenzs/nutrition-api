## Why

The athlete-config mapper derives `threshold_swim_pace_sec_per_100m` from `userData.lactateThresholdSwimSpeed` — a field Garmin never returns. Verified live: this account's `userData` has only the running `lactateThresholdSpeed` (+ `lactateThresholdHeartRate`); there is no swim-threshold key, and the garminconnect client exposes no swim/CSS endpoint (Garmin's swim-threshold concept is Critical Swim Speed, unavailable here). The key originated in the old synthetic fixture, so the mapping can never fire and the only "evidence" it works is fabricated test data — the same false-confidence pattern the recent bridge fixes removed.

## What Changes

- Remove the `threshold_swim_pace_sec_per_100m` mapping (and the now-orphaned `_pace_per_100m` helper) from `mapping.py`.
- Remove the fabricated `lactateThresholdSwimSpeed` key from the test fixture and the swim-pace assertion from `test_athlete_config_mapping`.
- Update the `garmin-bridge` spec to drop the swim-pace claim and document swim threshold (CSS) as having no reachable Garmin source — mirroring how power zones were treated before they became FTP-derivable.
- **Drive-by consistency fix:** the same requirement still says "Power-zone boundaries SHALL be omitted while no Garmin endpoint exposes them," which contradicts the now-current FTP-derivation scenario; correct that sentence to match (power zones derived from FTP).
- **Not removed:** the backend `athlete_config.threshold_swim_pace_sec_per_100m` field stays in the schema — only the bridge's phantom mapping goes, leaving room for a real source (e.g. CSS) later.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `garmin-bridge`: stop mapping a non-existent swim-threshold field; document swim threshold as no-reachable-source; align the power-zone wording with the FTP-derivation behavior.

## Impact

- **Code:** `apps/garmin-bridge/garmin_bridge/mapping.py` (remove swim mapping + `_pace_per_100m`).
- **Tests/fixtures:** `apps/garmin-bridge/tests/fixtures/garmin_day.json`, `apps/garmin-bridge/tests/test_mapping.py`.
- **No backend / REST / MCP change.** No behavior change in practice (the field never populated); this removes dead code and misleading tests.
