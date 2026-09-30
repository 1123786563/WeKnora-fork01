ALTER TABLE craft_charge_start_journal
    ADD COLUMN exec_event_started_at_ns BIGINT,
    ADD COLUMN exec_event_finished_at_ns BIGINT,
    ADD COLUMN duration_source VARCHAR(32);

ALTER TABLE craft_charge_start_journal
    ADD CONSTRAINT ck_craft_charge_start_exec_event_pair
    CHECK (
        (exec_event_started_at_ns IS NULL AND exec_event_finished_at_ns IS NULL AND duration_source IS NULL)
        OR
        (exec_event_started_at_ns IS NOT NULL AND exec_event_started_at_ns > 0
            AND exec_event_finished_at_ns IS NOT NULL AND exec_event_finished_at_ns > exec_event_started_at_ns
            AND duration_source IS NOT NULL AND duration_source = 'docker_exec_events')
    );
