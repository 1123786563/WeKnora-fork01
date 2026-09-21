-- Description: Lago billing migration T08 (#80) — SQLite mirror of 000180:
-- per-tenant benefits projection + coordinator monthly credit batch registry
-- (portable types; semantics identical to the versioned PostgreSQL copy).
CREATE TABLE IF NOT EXISTS commercial_tenant_benefits (
    tenant_id                INTEGER   NOT NULL,
    external_customer_id     TEXT      NOT NULL,
    external_subscription_id TEXT      NOT NULL,
    subscription_state       TEXT      NOT NULL,
    plan_code                TEXT      NOT NULL DEFAULT '',
    plan_key                 TEXT      NOT NULL DEFAULT '',
    plan_version             INTEGER   NOT NULL DEFAULT 0,
    features_json            TEXT      NOT NULL DEFAULT '{}',
    limits_json              TEXT      NOT NULL DEFAULT '{}',
    credits_balance_micro    INTEGER   NOT NULL DEFAULT 0,
    projected_at             DATETIME  NOT NULL,
    PRIMARY KEY (tenant_id)
);

CREATE TABLE IF NOT EXISTS commercial_credit_batches (
    id            INTEGER  NOT NULL,
    tenant_id     INTEGER  NOT NULL,
    period        TEXT     NOT NULL,
    command_key   TEXT     NOT NULL,
    wallet_ref    TEXT     NOT NULL DEFAULT '',
    granted_micro INTEGER  NOT NULL,
    expires_at    DATETIME NOT NULL,
    state         TEXT     NOT NULL,
    created_at    DATETIME NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT uq_credit_batch_tenant_period UNIQUE (tenant_id, period)
);
