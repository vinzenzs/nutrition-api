## Context

`fetch_day` stores `raw["training_status"] = api.get_training_status(date)` — the verbatim `/metrics-service/metrics/trainingstatus/aggregated/{date}` response. `map_fitness` (mapping.py:157) and `_training_status_label` (mapping.py:204) extract acute/chronic load and the status label from it.

A live sync + raw capture (`/tmp/training_status.json`, account device `3628010293`) shows the real shape:

```
training_status:
  mostRecentVO2Max: {...}
  mostRecentTrainingLoadBalance:
    metricsTrainingLoadBalanceDTOMap.<deviceId>: { monthlyLoad*, ...targets... }   # MONTHLY only — no acute/chronic, no acwrPercent
  mostRecentTrainingStatus:
    latestTrainingStatusData.<deviceId>:
      trainingStatus: 7                        # int code, not a string
      trainingStatusFeedbackPhrase: "PRODUCTIVE_7"
      acuteTrainingLoadDTO:
        dailyTrainingLoadAcute: 391            # ← acute_load
        dailyTrainingLoadChronic: 218          # ← chronic_load
        dailyAcuteChronicWorkloadRatio: 1.7    # Garmin's own ACWR
        acwrPercent: 80                        # a ratio %, NOT a load
```

Current mapper paths (`training_status.acuteTrainingLoad`, `training_status.chronicTrainingLoad`, the `acwrPercent` fallback, and `training_status.latestTrainingStatusData`) match **none** of this → both loads and the label resolve to `None`. The fixture `garmin_day.json` is hand-authored to the buggy paths, so tests pass falsely. Simulated against the real payload: current → `acute=None, chronic=None`; corrected → `acute=391, chronic=218` (server derives ACWR 1.8).

## Goals / Non-Goals

**Goals:**
- Populate `acute_load`/`chronic_load` from the real device-keyed `acuteTrainingLoadDTO` so the server's ACWR derivation works.
- Populate the `training_status` label from the corrected path, robust to Garmin's int status code.
- Make the fixture + tests reflect Garmin's actual response shape.

**Non-Goals:**
- `athlete_config`/FTP mapping (W/kg blocker) — separate investigation.
- `vo2max_running` absence on the sampled day.
- Storing Garmin's own `dailyAcuteChronicWorkloadRatio` — the server already derives ACWR from the two loads; no new field.
- Any backend/REST/MCP change.

## Decisions

- **Resolve the device entry once, share it.** Add a helper that returns the chosen `latestTrainingStatusData` device DTO: iterate `mostRecentTrainingStatus.latestTrainingStatusData` values, pick the one carrying an `acuteTrainingLoadDTO` (fall back to the first entry). Both the load fields and the label read from it — same device-keyed iteration `_training_status_label` already does, just at the corrected nesting. Single device today; the iterate-values approach tolerates multiple without hardcoding a key.
- **Load fields:** `acute_load ← acuteTrainingLoadDTO.dailyTrainingLoadAcute`, `chronic_load ← .dailyTrainingLoadChronic`, via `_as_float`. Drop the `acwrPercent` fallback entirely (wrong metric).
- **Label from the feedback phrase.** Garmin's `trainingStatus` is an int code; the human word lives in `trainingStatusFeedbackPhrase` as `LABEL_<n>` (e.g. `PRODUCTIVE_7`). Derive: strip the trailing `_<digits>` and lowercase → `productive`. Precedence: (1) a string `trainingStatus` on the device entry (defensive, older shapes), (2) the feedback-phrase prefix, (3) a top-level string `trainingStatus` (legacy fallback, retained). An int `trainingStatus` with no phrase yields no label (omitted), preserving the "don't coerce a bare code" intent.
- **Fixture from real capture.** Replace the `training_status` block in `garmin_day.json` with the recorded shape (`mostRecentTrainingStatus.latestTrainingStatusData.<dev>` with `acuteTrainingLoadDTO` + `trainingStatusFeedbackPhrase`). Keep the existing expected values where possible (acute 412 / chronic 388.5, label `productive`) so the rest of the fitness test is stable — only the nesting moves.

## Risks / Trade-offs

- [Feedback-phrase format varies by status (`PRODUCTIVE_7`, `MAINTAINING_1`, `DETRAINING`, `STRAINED`)] → The strip-trailing-`_<digits>`-then-lowercase rule handles both suffixed and bare forms; unknown words pass through verbatim (lowercased), consistent with the existing "not enum-gated" philosophy.
- [Multiple recording devices could disagree] → Pick the entry with an `acuteTrainingLoadDTO`; document the first-match rule. Acceptable for a single-athlete account.
- [Only one real payload sampled] → The capture is a genuine account response; fixture is built from it. If a future status surfaces a shape without `trainingStatusFeedbackPhrase`, the label is simply omitted (no crash).
- [Existing `test_training_status_ignores_non_string_codes` semantics] → Updated: a bare int code with no phrase still yields no label; the test is rewritten to the new path/shape rather than removed.
