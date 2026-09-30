DROP INDEX IF EXISTS uq_craft_charge_start_docker_exec_receipt;
ALTER TABLE craft_charge_start_journal
    DROP CONSTRAINT IF EXISTS ck_craft_charge_start_docker_receipt;
ALTER TABLE craft_charge_start_journal
    DROP COLUMN IF EXISTS send_claimed_at,
    DROP COLUMN IF EXISTS exec_id,
    DROP COLUMN IF EXISTS container_id,
    DROP COLUMN IF EXISTS provider;
