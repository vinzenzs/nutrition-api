-- Curated, cited methodology prose the in-app coach (Kazper) reads when it
-- grounds advice — distinct from the operational `notes` columns. Markdown text,
-- stored verbatim (the LLM reads it raw); authored in the vault and pushed in.
-- Per-phase on training_phases (the "why this phase" narrative, surfaced through
-- /context/training's covering phase); plan-level on training_plans (Key
-- Principles, cross-cutting reference). Null = none set.
ALTER TABLE training_phases ADD COLUMN methodology TEXT NULL;
ALTER TABLE training_plans ADD COLUMN methodology TEXT NULL;
