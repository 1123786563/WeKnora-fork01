DROP TRIGGER IF EXISTS trg_craft_web_build_receipt_no_update;
CREATE TRIGGER trg_craft_web_build_receipt_no_update
BEFORE UPDATE ON craft_web_build_receipts
BEGIN
    SELECT RAISE(ABORT, 'craft web build receipt is immutable');
END;
