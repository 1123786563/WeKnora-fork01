-- Description: Lago billing migration T08 (#80) — drop the SQLite benefits
-- projection and credit batch registry.
DROP TABLE IF EXISTS commercial_credit_batches;
DROP TABLE IF EXISTS commercial_tenant_benefits;
