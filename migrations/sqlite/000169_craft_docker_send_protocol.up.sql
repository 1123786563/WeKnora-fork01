ALTER TABLE craft_charge_start_journal
    ADD COLUMN protocol TEXT CHECK (protocol IS NULL OR protocol = 'docker_coordinator');
