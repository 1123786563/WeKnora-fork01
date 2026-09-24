DROP TRIGGER IF EXISTS ck_craft_charge_start_docker_receipt_update;
DROP TRIGGER IF EXISTS ck_craft_charge_start_docker_receipt_insert;
DROP INDEX IF EXISTS uq_craft_charge_start_docker_exec_receipt;
ALTER TABLE craft_charge_start_journal DROP COLUMN send_claimed_at;
ALTER TABLE craft_charge_start_journal DROP COLUMN exec_id;
ALTER TABLE craft_charge_start_journal DROP COLUMN container_id;
ALTER TABLE craft_charge_start_journal DROP COLUMN provider;
