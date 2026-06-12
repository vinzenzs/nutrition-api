-- add-workout-reconciliation (design D4): a stored marker for a completed
-- activity that matched more than one open planned workout, so the agent/app
-- can surface "this may fulfill a planned session — link it?". Set once at
-- ingestion; cleared on fulfill.
ALTER TABLE workouts ADD COLUMN needs_link BOOLEAN NOT NULL DEFAULT false;
