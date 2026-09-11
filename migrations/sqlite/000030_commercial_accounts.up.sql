CREATE TABLE commercial_accounts (
    tenant_id INTEGER PRIMARY KEY,
    customer_id TEXT NOT NULL UNIQUE,
    version INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE commercial_grants (
    tenant_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    capability TEXT NOT NULL,
    granted_by TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (tenant_id, user_id, capability)
);
