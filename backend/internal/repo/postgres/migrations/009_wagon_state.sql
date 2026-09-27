ALTER TABLE sessions ADD COLUMN wagon_state JSONB;
ALTER TABLE situations ADD COLUMN seat_anchor TEXT;
ALTER TABLE situations ADD COLUMN physical_requirement JSONB;
ALTER TABLE situations ADD COLUMN physical_action_done BOOLEAN NOT NULL DEFAULT false;
