## Why

ACWR is never populated. The server derives ACWR from `acute_load ÷ chronic_load` on the fitness snapshot, but the bridge never sends those loads — verified against a live sync of `2026-06-14`: `fitness: posted`, yet the stored snapshot has no `acute_load`/`chronic_load` and `/context/training` returns `acwr: null`. The training-status label is silently dropped the same way. Both are mapped from `get_training_status`, and the mapper reads paths that do not exist in Garmin's real payload. A synthetic test fixture matches the buggy paths, so the test suite is green while production is broken.

## What Changes

- Fix `acute_load` / `chronic_load` mapping in `garmin_bridge/mapping.py` to read the **real** Garmin shape: `training_status.mostRecentTrainingStatus.latestTrainingStatusData.<deviceId>.acuteTrainingLoadDTO.{dailyTrainingLoadAcute, dailyTrainingLoadChronic}` (device-keyed map, iterate values). Currently it reads non-existent top-level `acuteTrainingLoad`/`chronicTrainingLoad` keys with a fallback to `acwrPercent` (wrong subtree, wrong metric — a ratio %, not a load).
- Fix the training-status **label**: read the device entry under `mostRecentTrainingStatus.latestTrainingStatusData` (currently read one level too high, at `training_status.latestTrainingStatusData`), and derive the label from `trainingStatusFeedbackPhrase` (e.g. `PRODUCTIVE_7` → `productive`) since Garmin's `trainingStatus` is an int code, not a string. Retain the top-level string fallback for older shapes.
- Replace the synthetic `training_status` block in `tests/fixtures/garmin_day.json` with the real recorded shape, and update `test_mapping.py` accordingly so the tests assert against what Garmin actually returns.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `garmin-bridge`: correct the source paths the fitness-snapshot enrichment uses for acute/chronic training load and the training-status label (both from `get_training_status`).

## Impact

- **Code:** `apps/garmin-bridge/garmin_bridge/mapping.py` (`map_fitness` load fields + `_training_status_label`).
- **Tests/fixtures:** `apps/garmin-bridge/tests/fixtures/garmin_day.json`, `apps/garmin-bridge/tests/test_mapping.py`.
- **Downstream (no code change):** once loads land, the server's existing `acwr()` derivation and the `/context/training` `acwr` field populate automatically; the `training_status` label appears on `/fitness-metrics`. No backend, REST, or MCP change.
- **Out of scope:** `athlete_config: skipped (no data)` (FTP/zones, blocks W/kg) — a separate suspected mapping gap to be investigated on its own; the missing `vo2max_running` on the same day is not addressed here.
