## 1. Mapping fix

- [x] 1.1 Add a helper in `mapping.py` that returns the chosen `mostRecentTrainingStatus.latestTrainingStatusData` device DTO: iterate the device-keyed map's values, prefer the entry carrying an `acuteTrainingLoadDTO`, else the first entry; return `None` when absent
- [x] 1.2 Rewrite `map_fitness` `acute_load` to read `<device>.acuteTrainingLoadDTO.dailyTrainingLoadAcute` and `chronic_load` to read `.dailyTrainingLoadChronic` (via `_as_float`); remove the `acuteTrainingLoad`/`chronicTrainingLoad` top-level paths and the `acwrPercent` fallback
- [x] 1.3 Rewrite `_training_status_label` to read the corrected device entry: prefer a string `trainingStatus`, else derive from `trainingStatusFeedbackPhrase` (strip a trailing `_<digits>`, lowercase), else fall back to a top-level string `trainingStatus`; a bare int code with no phrase → `None`

## 2. Fixture + tests

- [x] 2.1 Replace the synthetic `training_status` block in `tests/fixtures/garmin_day.json` with the real recorded shape (`mostRecentTrainingStatus.latestTrainingStatusData.<dev>` with `acuteTrainingLoadDTO.{dailyTrainingLoadAcute,dailyTrainingLoadChronic}` and `trainingStatusFeedbackPhrase`), keeping acute 412 / chronic 388.5 / label `productive` so `test_fitness_mapping` stays meaningful
- [x] 2.2 Update `test_fitness_mapping` to assert `acute_load`/`chronic_load`/`training_status` from the new shape
- [x] 2.3 Update `test_training_status_label_falls_back_to_top_level` and `test_training_status_ignores_non_string_codes` to the corrected paths; add a case deriving the label from `trainingStatusFeedbackPhrase`

## 3. Verification

- [x] 3.1 Run the bridge test suite (`apps/garmin-bridge` pytest) — confirm green
- [x] 3.2 End-to-end: start the bridge, sync a recent day, and confirm `GET /context/training?date=<day>` returns a non-null `acwr` and the fitness snapshot carries `acute_load`/`chronic_load`/`training_status`
