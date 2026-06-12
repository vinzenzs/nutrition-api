-- Per-slot effort target overrides: a JSONB list of {intent, target} entries
-- (at most one per intent) that supersede a referenced template's step targets
-- when a planned workout's effective program is resolved. Null = no overrides.
ALTER TABLE plan_slots ADD COLUMN target_overrides JSONB NULL;
