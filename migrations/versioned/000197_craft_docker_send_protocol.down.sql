ALTER TABLE craft_charge_start_journal
    DROP CONSTRAINT IF EXISTS ck_craft_charge_start_protocol;

ALTER TABLE craft_charge_start_journal
    DROP COLUMN IF EXISTS protocol;
