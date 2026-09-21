-- T06 (#78): SQLite twin of 000178 — rebuildable Billing Projection of the
-- Tenant -> Lago Customer mapping. One row per space; the deterministic
-- external identity is UNIQUE across spaces; the row is a rebuildable
-- cache of the authority's truth. (#78 owns 000099 sqlite / 000178
-- versioned; #79 takes 000100/000179.)

CREATE TABLE IF NOT EXISTS commercial_billing_accounts (
    tenant_id             INTEGER  NOT NULL,
    external_customer_id  TEXT     NOT NULL,
    provider_customer_ref TEXT     NOT NULL DEFAULT '',
    state                 TEXT     NOT NULL DEFAULT 'pending',
    ensured_at            DATETIME,
    created_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id),
    CONSTRAINT uq_commercial_billing_accounts_external_id UNIQUE (external_customer_id)
);
