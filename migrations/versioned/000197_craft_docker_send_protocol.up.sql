ALTER TABLE craft_charge_start_journal
    ADD COLUMN protocol VARCHAR(32);

ALTER TABLE craft_charge_start_journal
    ADD CONSTRAINT ck_craft_charge_start_protocol
    CHECK (protocol IS NULL OR protocol = 'docker_coordinator');
