-- Description: Lago billing migration T08 (#80) — drop the benefits
-- projection and the credit batch registry (both are rebuildable
-- projections; dropping them never touches authority-side state).
DO $$ BEGIN RAISE NOTICE '[Migration 000180] Dropping commercial_tenant_benefits and commercial_credit_batches'; END $$;

DROP TABLE IF EXISTS commercial_credit_batches;
DROP TABLE IF EXISTS commercial_tenant_benefits;
