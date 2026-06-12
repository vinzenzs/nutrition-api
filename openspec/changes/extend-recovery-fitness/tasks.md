## 1. Migration

- [ ] 1.1 Verify the migration head on disk (`internal/store/migrations/`) before `task migrate:new` — the arc assigns B=`036`, A=`037`, so expect `038`, but an out-of-band slot collision has happened before; confirm the highest existing number first, then `task migrate:new NAME=extend_recovery_fitness`
- [ ] 1.2 `038_extend_recovery_fitness.up.sql`: `ALTER TABLE recovery_metrics ADD COLUMN` the 8 recovery fields (`spo2_avg`, `spo2_lowest`, `respiration_avg`, `respiration_lowest`, `deep_sleep_seconds`, `light_sleep_seconds`, `rem_sleep_seconds`, `awake_seconds`), all nullable with CHECKs mirroring existing column conventions (percentages 0–100, respiration `> 0`, seconds `>= 0`)
- [ ] 1.3 Same up-migration: `ALTER TABLE fitness_metrics ADD COLUMN` the 4 fitness fields (`endurance_score`, `hill_score`, `fitness_age`, `training_status` TEXT with a length CHECK), all nullable
- [ ] 1.4 `.down.sql`: drop the 4 fitness columns, then the 8 recovery columns (no data transform; existing rows read back unchanged)

## 2. Types & repo (internal/recoverymetrics)

- [ ] 2.1 `types.go`: add the 8 nullable pointer fields to `Snapshot` (omitempty) — `Spo2Avg`, `Spo2Lowest`, `RespirationAvg`, `RespirationLowest`, `DeepSleepSeconds`, `LightSleepSeconds`, `RemSleepSeconds`, `AwakeSeconds`
- [ ] 2.2 `repo.go`: extend the INSERT/UPSERT and the SELECT (single + list) to carry the new columns

## 3. Types & repo (internal/fitnessmetrics)

- [ ] 3.1 `types.go`: add the 4 nullable pointer fields to `Snapshot` (omitempty) — `EnduranceScore`, `HillScore`, `FitnessAge`, `TrainingStatus *string`
- [ ] 3.2 `repo.go`: extend the INSERT/UPSERT and the SELECT (single + list) to carry the new columns

## 4. Service & handlers (both packages)

- [ ] 4.1 `recoverymetrics/service.go`: validate the new fields (SpO2 0–100, respiration `> 0`, sleep-stage seconds `>= 0`) with sentinel errors mapping 1:1 to the documented API codes
- [ ] 4.2 `fitnessmetrics/service.go`: validate the new fields (`endurance_score`/`hill_score`/`fitness_age` positive; `training_status` trimmed, reject empty/oversized, NOT enum-gated) with sentinel errors
- [ ] 4.3 `handlers.go` (both): apply `numfmt.Round1` to every new float (`respiration_*`, `fitness_age`) at the response boundary; integers and `training_status` text pass through unrounded
- [ ] 4.4 Update swag annotations on the affected request/response structs

## 5. Wiring & MCP

- [ ] 5.1 Verify `internal/httpserver/server.go` wiring needs no change (same packages, same routes); adjust only if a repo/service construction signature changed
- [ ] 5.2 Confirm the existing recovery/fitness read tools forward the enriched body verbatim (no new tool); review `mcp_integration_test` expected-tools list (expect unchanged)

## 6. garmin-bridge (apps/garmin-bridge)

- [ ] 6.1 `garmin_client.py`: add guarded per-day fetches `get_spo2_data`, `get_respiration_data`, `get_endurance_score`, `get_hill_score`, `get_fitnessage_data` wired into `fetch_day`, each via the existing `safe()` pattern (sleep DTO and `get_training_status` are already fetched)
- [ ] 6.2 `mapping.py` `map_recovery`: extract `spo2_avg`/`spo2_lowest` (from `get_spo2_data`), `respiration_avg`/`respiration_lowest` (from `get_respiration_data`), and `deep_sleep_seconds`/`light_sleep_seconds`/`rem_sleep_seconds`/`awake_seconds` (from the already-dug sleep DTO); defensive extraction (absent → omitted)
- [ ] 6.3 `mapping.py` `map_fitness`: extract `endurance_score`, `hill_score`, `fitness_age`, and the `training_status` label (from the already-fetched training-status payload); defensive extraction
- [ ] 6.4 `sync.py`: confirm the recovery/fitness posts carry the new fields unchanged (no new endpoint, no per-field round-trip)

## 7. Tests & docs

- [ ] 7.1 Expand `apps/garmin-bridge/tests/fixtures/garmin_day.json` with SpO2, respiration, sleep-stage, endurance, hill, fitness-age, and training-status payloads
- [ ] 7.2 `test_mapping.py`: assert the new recovery/fitness field mapping, including absent-field omission and the training-status verbatim pass-through
- [ ] 7.3 `internal/recoverymetrics` + `internal/fitnessmetrics` integration tests: POST round-trips the new fields, NULL omitted on absent, out-of-range rejected with the right code, `training_status` non-enum acceptance + empty rejection
- [ ] 7.4 `task swag`, `task vet`, `task test` (or scoped `go test -count=1 ./internal/recoverymetrics/... ./internal/fitnessmetrics/...` + bridge `pytest`) all green
