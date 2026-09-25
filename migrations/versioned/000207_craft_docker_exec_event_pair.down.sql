ALTER TABLE craft_charge_start_journal
    DROP CONSTRAINT IF EXISTS ck_craft_charge_start_exec_event_pair;
ALTER TABLE craft_charge_start_journal
    DROP COLUMN IF EXISTS duration_source,
    DROP COLUMN IF EXISTS exec_event_finished_at_ns,
    DROP COLUMN IF EXISTS exec_event_started_at_ns;
