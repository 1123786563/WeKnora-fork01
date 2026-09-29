DROP TRIGGER IF EXISTS trg_craft_web_build_receipt_no_delete ON craft_web_build_receipts;
DROP TRIGGER IF EXISTS trg_craft_web_build_receipt_no_update ON craft_web_build_receipts;
DROP FUNCTION IF EXISTS craft_web_build_receipt_no_mutation_fn();
DROP TABLE IF EXISTS craft_web_build_receipts;
