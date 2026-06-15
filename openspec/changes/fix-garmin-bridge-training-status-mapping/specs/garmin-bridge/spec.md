## MODIFIED Requirements

### Requirement: The bridge enriches recovery and fitness snapshots with additional daily signals

The bridge SHALL attach to each synced day's recovery and fitness snapshots the additional daily signals Garmin exposes: blood-oxygen (SpO2), respiration, and the per-stage sleep breakdown on the recovery snapshot; endurance score, hill score, fitness age, the acute and chronic training load, and the training-status label on the fitness snapshot. The sleep-stage seconds, the acute/chronic training load, and the training-status label SHALL be read from payloads the bridge already fetches (the sleep DTO and `get_training_status`) with no extra Garmin call; SpO2, respiration, endurance, hill, and fitness-age SHALL each be fetched per day via an individually `safe()`-guarded call so that a failing, throttled, or account-unavailable Garmin endpoint yields absent detail for that field — never an aborted day. The acute and chronic training load SHALL be read from the device-keyed training-status payload at `mostRecentTrainingStatus.latestTrainingStatusData.<deviceId>.acuteTrainingLoadDTO` (`dailyTrainingLoadAcute` → `acute_load`, `dailyTrainingLoadChronic` → `chronic_load`); the bridge SHALL NOT use the monthly-load `mostRecentTrainingLoadBalance` subtree or the `acwrPercent` ratio for these fields. The training-status label SHALL be read from the same device entry, deriving the word from `trainingStatusFeedbackPhrase` (the alphabetic prefix, lowercased — e.g. `PRODUCTIVE_7` → `productive`) because Garmin's `trainingStatus` is a numeric code, with a fallback to a string `trainingStatus` value when present. The mapper SHALL attach whatever detail is present and omit what is absent, posting the result onto the EXISTING `/recovery-metrics` and `/fitness-metrics` targets (no new sync target endpoint). The acute:chronic workload ratio (ACWR) SHALL NOT be sent by the bridge — the backend derives it from the two loads.

#### Scenario: Acute and chronic training load are mapped from the device-keyed training-status payload

- **WHEN** the already-fetched `get_training_status` payload carries `mostRecentTrainingStatus.latestTrainingStatusData.<deviceId>.acuteTrainingLoadDTO` with `dailyTrainingLoadAcute` and `dailyTrainingLoadChronic`
- **THEN** the bridge maps them to `acute_load` and `chronic_load` on the `/fitness-metrics` body with NO additional Garmin call
- **AND** it ignores the `mostRecentTrainingLoadBalance` monthly-load subtree and the `acwrPercent` ratio
- **AND** the backend can derive ACWR from the two loads

#### Scenario: Sleep-stage and training-status come free from already-fetched payloads

- **WHEN** the day's sleep DTO carries deep / light / REM / awake stage seconds and the already-fetched training-status payload carries a device entry with a `trainingStatusFeedbackPhrase`
- **THEN** the bridge maps the four sleep-stage seconds into the `/recovery-metrics` body and the `training_status` label (the lowercased phrase prefix, e.g. `productive`) into the `/fitness-metrics` body with NO additional Garmin call
- **AND** a field absent from those payloads is simply omitted from the snapshot

#### Scenario: A numeric training-status code with no phrase yields no label

- **WHEN** the device entry carries a numeric `trainingStatus` code but no `trainingStatusFeedbackPhrase` and no string status anywhere
- **THEN** the `training_status` label is omitted from the fitness snapshot rather than coerced from the bare code

#### Scenario: SpO2, respiration, endurance, hill, and fitness-age are fetched per day and guarded

- **WHEN** the bridge syncs a day
- **THEN** it fetches SpO2, respiration, endurance score, hill score, and fitness age via the Garmin client, each through the existing `safe()` wrapper
- **AND** if one of those endpoints raises or returns nothing, the corresponding field is omitted from the recovery or fitness snapshot
- **AND** the rest of the day's snapshots and capabilities still sync (one bad detail fetch is not fatal)

#### Scenario: Enriched snapshots are posted to the existing snapshot endpoints

- **WHEN** a synced day yields SpO2 / respiration / sleep-stage detail and endurance / hill / fitness-age / acute-load / chronic-load / training-status detail
- **THEN** the recovery detail is posted on the `/recovery-metrics` upsert and the fitness detail on the `/fitness-metrics` upsert
- **AND** re-running the sync for the same date re-posts the same snapshots, and the date-keyed full-replace upsert leaves no duplicate rows
