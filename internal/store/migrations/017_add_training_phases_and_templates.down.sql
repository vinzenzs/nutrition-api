-- training_phases has a FK to goal_templates, so it must drop first.
DROP TABLE IF EXISTS training_phases;
DROP TABLE IF EXISTS goal_templates;
