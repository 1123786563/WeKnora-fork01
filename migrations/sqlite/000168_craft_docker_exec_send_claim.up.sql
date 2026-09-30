ALTER TABLE craft_charge_start_journal ADD COLUMN provider TEXT;
ALTER TABLE craft_charge_start_journal ADD COLUMN container_id TEXT;
ALTER TABLE craft_charge_start_journal ADD COLUMN exec_id TEXT;
ALTER TABLE craft_charge_start_journal ADD COLUMN send_claimed_at DATETIME;

CREATE UNIQUE INDEX uq_craft_charge_start_docker_exec_receipt
    ON craft_charge_start_journal (provider, container_id, exec_id)
    WHERE provider IS NOT NULL;

CREATE TRIGGER ck_craft_charge_start_docker_receipt_insert
BEFORE INSERT ON craft_charge_start_journal
WHEN NOT (
    (NEW.provider IS NULL AND NEW.container_id IS NULL AND NEW.exec_id IS NULL AND NEW.send_claimed_at IS NULL)
    OR
    (NEW.provider IS NOT NULL AND NEW.provider = 'docker'
        AND NEW.container_id IS NOT NULL AND length(trim(NEW.container_id)) > 0
        AND NEW.exec_id IS NOT NULL AND length(trim(NEW.exec_id)) > 0)
)
BEGIN
    SELECT RAISE(ABORT, 'invalid craft charge start Docker receipt');
END;

CREATE TRIGGER ck_craft_charge_start_docker_receipt_update
BEFORE UPDATE OF provider, container_id, exec_id, send_claimed_at ON craft_charge_start_journal
WHEN NOT (
    (NEW.provider IS NULL AND NEW.container_id IS NULL AND NEW.exec_id IS NULL AND NEW.send_claimed_at IS NULL)
    OR
    (NEW.provider IS NOT NULL AND NEW.provider = 'docker'
        AND NEW.container_id IS NOT NULL AND length(trim(NEW.container_id)) > 0
        AND NEW.exec_id IS NOT NULL AND length(trim(NEW.exec_id)) > 0)
)
BEGIN
    SELECT RAISE(ABORT, 'invalid craft charge start Docker receipt');
END;
