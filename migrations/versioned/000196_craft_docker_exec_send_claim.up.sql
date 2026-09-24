ALTER TABLE craft_charge_start_journal
    ADD COLUMN provider VARCHAR(32),
    ADD COLUMN container_id VARCHAR(256),
    ADD COLUMN exec_id VARCHAR(256),
    ADD COLUMN send_claimed_at TIMESTAMP;

ALTER TABLE craft_charge_start_journal
    ADD CONSTRAINT ck_craft_charge_start_docker_receipt
    CHECK (
        (provider IS NULL AND container_id IS NULL AND exec_id IS NULL AND send_claimed_at IS NULL)
        OR
        (provider IS NOT NULL AND provider = 'docker'
            AND container_id IS NOT NULL AND btrim(container_id) <> ''
            AND exec_id IS NOT NULL AND btrim(exec_id) <> '')
    );

CREATE UNIQUE INDEX uq_craft_charge_start_docker_exec_receipt
    ON craft_charge_start_journal (provider, container_id, exec_id)
    WHERE provider IS NOT NULL;
