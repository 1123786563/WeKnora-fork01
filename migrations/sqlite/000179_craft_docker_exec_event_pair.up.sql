ALTER TABLE craft_charge_start_journal ADD COLUMN exec_event_started_at_ns INTEGER;
ALTER TABLE craft_charge_start_journal ADD COLUMN exec_event_finished_at_ns INTEGER;
ALTER TABLE craft_charge_start_journal ADD COLUMN duration_source TEXT;

CREATE TRIGGER ck_craft_charge_start_exec_event_pair_insert
BEFORE INSERT ON craft_charge_start_journal
WHEN NOT (
    (NEW.exec_event_started_at_ns IS NULL AND NEW.exec_event_finished_at_ns IS NULL AND NEW.duration_source IS NULL)
    OR
    (NEW.exec_event_started_at_ns IS NOT NULL AND NEW.exec_event_started_at_ns > 0
        AND NEW.exec_event_finished_at_ns IS NOT NULL AND NEW.exec_event_finished_at_ns > NEW.exec_event_started_at_ns
        AND NEW.duration_source IS NOT NULL AND NEW.duration_source = 'docker_exec_events')
)
BEGIN
    SELECT RAISE(ABORT, 'invalid craft charge start exec event pair');
END;

CREATE TRIGGER ck_craft_charge_start_exec_event_pair_update
BEFORE UPDATE OF exec_event_started_at_ns, exec_event_finished_at_ns, duration_source ON craft_charge_start_journal
WHEN NOT (
    (NEW.exec_event_started_at_ns IS NULL AND NEW.exec_event_finished_at_ns IS NULL AND NEW.duration_source IS NULL)
    OR
    (NEW.exec_event_started_at_ns IS NOT NULL AND NEW.exec_event_started_at_ns > 0
        AND NEW.exec_event_finished_at_ns IS NOT NULL AND NEW.exec_event_finished_at_ns > NEW.exec_event_started_at_ns
        AND NEW.duration_source IS NOT NULL AND NEW.duration_source = 'docker_exec_events')
)
BEGIN
    SELECT RAISE(ABORT, 'invalid craft charge start exec event pair');
END;
