## Why

The athlete-config mapper treats Garmin's `lactateThresholdSpeed` as metres-per-second and converts pace as `1000 / value`. That assumption is wrong: the field is **seconds-per-metre** (a pace). Confirmed against live data — the account's value `0.26944369` gives a nonsensical 3711 s/km (61:53/km) under m/s, but its reciprocal `3.711 m/s` (= `value × 1000` = 269.4 s/km = 4:29/km) is a textbook threshold pace, correctly faster than the athlete's real easy runs (2.2 m/s ≈ 7:20/km). The previous change papered over this by dropping the implausible result, so `threshold_pace_sec_per_km` is never populated. This change converts correctly so it is.

## What Changes

- Fix the conversion in `mapping.py`: `threshold_pace_sec_per_km = lactateThresholdSpeed × 1000` and `threshold_swim_pace_sec_per_100m = lactateThresholdSwimSpeed × 100` (interpret the field as seconds-per-metre, not m/s). Rename the helpers/params off the misleading "speed_mps" naming.
- Keep a plausibility guard as a safety net (drop genuinely out-of-band results), now that legitimate values pass.
- Update the fixture's `lactateThresholdSpeed`/`lactateThresholdSwimSpeed` to realistic s/m values and update `test_mapping.py` (including repurposing the implausible-pace test, since the old `0.269` input is now a *valid* 269.4 s/km).
- **Note:** the running threshold is directly confirmed (s/m). The swim field (`lactateThresholdSwimSpeed`) is absent for this account, so its s/m interpretation is applied by analogy with the same field family.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `garmin-bridge`: correct the unit interpretation for threshold pace in the athlete-physiology-config mapping (seconds-per-metre → pace).

## Impact

- **Code:** `apps/garmin-bridge/garmin_bridge/mapping.py` (the two pace helpers + their call sites).
- **Tests/fixtures:** `apps/garmin-bridge/tests/fixtures/garmin_day.json`, `apps/garmin-bridge/tests/test_mapping.py`.
- **Downstream:** `athlete_config.threshold_pace_sec_per_km` (and swim, when present) now populates on each sync. No backend / REST / MCP change.
