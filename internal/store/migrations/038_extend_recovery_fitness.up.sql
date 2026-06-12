-- extend-recovery-fitness (change C): widen the recovery and fitness daily
-- snapshots with the remaining cheap-to-fetch Garmin signals. Recovery gains
-- blood-oxygen (SpO2 avg/lowest), overnight respiration (avg/lowest), and the
-- per-stage sleep breakdown (deep/light/REM/awake seconds — read from the sleep
-- DTO the bridge already fetches). Fitness gains an endurance score, hill score,
-- fitness age, and a free-text training_status label that complements the
-- numeric acute/chronic load. All columns nullable with CHECKs mirroring the
-- existing snapshot conventions; no back-fill — every existing row reads back
-- NULL, the meaningful "not measured" state.

ALTER TABLE recovery_metrics
    ADD COLUMN spo2_avg            INTEGER       CHECK (spo2_avg IS NULL OR (spo2_avg BETWEEN 0 AND 100)),
    ADD COLUMN spo2_lowest         INTEGER       CHECK (spo2_lowest IS NULL OR (spo2_lowest BETWEEN 0 AND 100)),
    ADD COLUMN respiration_avg     NUMERIC(4, 1) CHECK (respiration_avg IS NULL OR respiration_avg > 0),
    ADD COLUMN respiration_lowest  NUMERIC(4, 1) CHECK (respiration_lowest IS NULL OR respiration_lowest > 0),
    ADD COLUMN deep_sleep_seconds  INTEGER       CHECK (deep_sleep_seconds IS NULL OR deep_sleep_seconds >= 0),
    ADD COLUMN light_sleep_seconds INTEGER       CHECK (light_sleep_seconds IS NULL OR light_sleep_seconds >= 0),
    ADD COLUMN rem_sleep_seconds   INTEGER       CHECK (rem_sleep_seconds IS NULL OR rem_sleep_seconds >= 0),
    ADD COLUMN awake_seconds       INTEGER       CHECK (awake_seconds IS NULL OR awake_seconds >= 0);

ALTER TABLE fitness_metrics
    ADD COLUMN endurance_score INTEGER       CHECK (endurance_score IS NULL OR endurance_score > 0),
    ADD COLUMN hill_score      INTEGER       CHECK (hill_score IS NULL OR hill_score > 0),
    ADD COLUMN fitness_age     NUMERIC(4, 1) CHECK (fitness_age IS NULL OR fitness_age > 0),
    ADD COLUMN training_status TEXT          CHECK (training_status IS NULL OR length(training_status) BETWEEN 1 AND 64);
