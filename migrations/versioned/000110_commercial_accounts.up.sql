CREATE TABLE commercial_accounts (
    tenant_id BIGINT PRIMARY KEY,
    customer_id TEXT NOT NULL UNIQUE,
    version BIGINT NOT NULL DEFAULT 1
);

CREATE TABLE commercial_grants (
    tenant_id BIGINT NOT NULL,
    user_id TEXT NOT NULL,
    capability TEXT NOT NULL,
    granted_by TEXT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, user_id, capability)
);
