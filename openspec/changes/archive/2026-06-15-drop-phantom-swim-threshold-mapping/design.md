## Context

`map_athlete_config` maps `threshold_swim_pace_sec_per_100m` from `user_profile.userData.lactateThresholdSwimSpeed` via `_pace_per_100m`. Live investigation found:

- `userData` for this account contains `lactateThresholdSpeed` (running) and `lactateThresholdHeartRate` only — **no `lactateThresholdSwimSpeed`**.
- `get_lactate_threshold()` returns running speed/HR + a running power block — no swim entry.
- No `get_*swim*` / `*css*` method exists in the garminconnect client; Garmin's swim threshold is Critical Swim Speed (CSS), with no reachable endpoint.
- The `lactateThresholdSwimSpeed` key (and its value `1.5`) exist only in the test fixture, which was authored to match the mapper.

So the swim mapping is dead code validated by fabricated data. `_pace_per_100m` is used solely by this mapping.

Separately, the requirement body still asserts power zones are "omitted while no Garmin endpoint exposes them," which the most recent change superseded (power zones are now derived from FTP). The scenario was updated; the body sentence was not.

## Goals / Non-Goals

**Goals:**
- Remove the phantom swim-threshold mapping, its orphaned helper, and the fabricated fixture/test data.
- Make the spec honest: swim threshold has no reachable source; power-zone wording matches the FTP-derivation behavior.

**Non-Goals:**
- Removing the backend `threshold_swim_pace_sec_per_100m` column/field — it stays for a future source.
- Implementing CSS ingestion — no reachable endpoint today; out of scope.

## Decisions

- **Delete the swim mapping and `_pace_per_100m`.** Drop the `threshold_swim_pace_sec_per_100m` line from `map_athlete_config` and remove `_pace_per_100m` (no other caller). Keep `_pace_per_km` (still used for the running threshold).
- **Strip the fixture key + swim assertion.** Remove `lactateThresholdSwimSpeed` from `garmin_day.json` and the `assert cfg["threshold_swim_pace_sec_per_100m"] == ...` line; the running-pace assertion and the rest of `test_athlete_config_mapping` stay.
- **Spec MODIFIED, two edits in one block:** (1) drop `lactateThresholdSwimSpeed → threshold_swim_pace_sec_per_100m` from the field list and the s/m conversion sentence, and add that swim threshold (CSS) has no reachable Garmin source so the field is omitted; (2) replace the stale "power zones omitted" sentence with the FTP-derivation statement so the body and the existing power-zone scenario agree. Add a scenario asserting the swim field is never written.

## Risks / Trade-offs

- [A future Garmin device/account might expose a swim threshold] → The backend field is retained; re-adding a mapping (from a real, verified source) is a small change. We refuse to map a key not observed in real data.
- [Removing `_pace_per_100m` could surprise a future caller] → It has no current caller; the conversion is trivial to reintroduce alongside a real swim source.
