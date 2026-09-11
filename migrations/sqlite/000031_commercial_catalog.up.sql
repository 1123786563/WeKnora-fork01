CREATE TABLE commercial_plan_catalog (
    plan_key TEXT NOT NULL,
    version INTEGER NOT NULL,
    definition_json TEXT NOT NULL,
    external_id TEXT NOT NULL UNIQUE,
    state TEXT NOT NULL,
    PRIMARY KEY (plan_key, version)
);

CREATE TABLE commercial_quotes (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    subscription_version INTEGER NOT NULL,
    snapshot_json TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    used_order_id TEXT UNIQUE
);
