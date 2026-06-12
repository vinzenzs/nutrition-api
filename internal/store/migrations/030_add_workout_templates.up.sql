-- Reusable, structured workout-template library (per add-workout-templates).
-- A template is a sport + named, ordered steps (warmup / intervals with target
-- zones / cooldown). Steps live as a validated JSONB array — always read with
-- the template, never queried individually — so no child table. The service
-- layer enforces the step structure; the DB only guarantees a non-empty array.
-- sport reuses the workouts vocabulary so a template's sport and a workout's
-- sport share one set of values. First-party authored: no external_id/source.
CREATE TABLE workout_templates (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sport                  TEXT NOT NULL CHECK (sport IN ('run', 'bike', 'swim', 'strength', 'other')),
    name                   TEXT NOT NULL,
    description            TEXT NULL,
    estimated_duration_sec INTEGER NULL CHECK (estimated_duration_sec IS NULL OR estimated_duration_sec > 0),
    steps                  JSONB NOT NULL CHECK (jsonb_typeof(steps) = 'array' AND jsonb_array_length(steps) > 0),
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Supports the GET /workout-templates?sport= list filter.
CREATE INDEX idx_workout_templates_sport ON workout_templates (sport);
