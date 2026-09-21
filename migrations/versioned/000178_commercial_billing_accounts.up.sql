-- Description: Lago T06 (#78) — rebuildable Billing Projection of the
-- Tenant -> Lago Customer mapping. One row per space (tenant_id PRIMARY
-- KEY); external_customer_id is the DETERMINISTIC identity derived purely
-- from the tenant id (weknora-tenant-<id>, internal/commercial.
-- ExternalCustomerID) and UNIQUE, so no two spaces can ever share a
-- customer identity. provider_customer_ref is the seam-internal receipt
-- identity and NEVER crosses the Billing API. state is projection-local
-- (pending = ensure attempted, outcome indeterminate or failed;
-- linked = receipt/snapshot confirmed); the row is REBUILDABLE — the
-- authority, not this table, is the truth. Migration-number allocation:
-- #78 owns 000178 versioned (000099 sqlite); #79 takes 000179/000100.
DO $$ BEGIN RAISE NOTICE '[Migration 000178] Creating commercial_billing_accounts'; END $$;

CREATE TABLE IF NOT EXISTS commercial_billing_accounts (
    tenant_id             BIGINT      NOT NULL,
    external_customer_id  TEXT        NOT NULL,
    provider_customer_ref TEXT        NOT NULL DEFAULT '',
    state                 TEXT        NOT NULL DEFAULT 'pending',
    ensured_at            TIMESTAMPTZ,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id),
    CONSTRAINT uq_commercial_billing_accounts_external_id UNIQUE (external_customer_id)
);

COMMENT ON TABLE commercial_billing_accounts IS 'Rebuildable Billing Projection: one space to exactly one Lago Customer (T06, #78); the authority is the truth, this row is a cache';
COMMENT ON COLUMN commercial_billing_accounts.external_customer_id IS 'deterministic identity weknora-tenant-<tenant id> — immutable across rename and owner transfer';
COMMENT ON COLUMN commercial_billing_accounts.provider_customer_ref IS 'seam-internal receipt identity (CommandReceipt.ExternalID); never crosses the Billing API';
COMMENT ON COLUMN commercial_billing_accounts.state IS 'projection-local state: pending (ensure attempted, unconfirmed) | linked (receipt/snapshot confirmed)';
