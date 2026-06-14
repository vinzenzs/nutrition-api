-- Per-slot duration overrides: a JSONB list of {intent, duration} entries (at
-- most one per intent) that supersede a referenced template's step durations
-- when a planned workout's effective program is resolved. Each duration uses the
-- workout-templates Duration shape restricted to the bounded kinds
-- (time / distance). Null = no overrides. Sibling of target_overrides (034).
ALTER TABLE plan_slots ADD COLUMN duration_overrides JSONB NULL;
