-- A03: raw model invocations and budget reservations.
CREATE TABLE IF NOT EXISTS semantic_invocations (
    invocation_id      VARCHAR(128) PRIMARY KEY,
    parent_invocation_id VARCHAR(128),
    operation_id       VARCHAR(128) NOT NULL,
    tenant_id          INTEGER NOT NULL,
    kb_id              VARCHAR(255) NOT NULL,
    model_profile_ref  VARCHAR(255) NOT NULL,
    budget_ref         VARCHAR(128) NOT NULL,
    request_hash       VARCHAR(128) NOT NULL,
    messages           TEXT NOT NULL DEFAULT '[]',
    max_output_tokens  INTEGER NOT NULL DEFAULT 0,
    state              VARCHAR(32)  NOT NULL,
    status             VARCHAR(32)  NOT NULL DEFAULT '',
    text               TEXT,
    input_tokens       INTEGER,
    output_tokens      INTEGER,
    provider_request_id VARCHAR(255),
    error              TEXT,
    created_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at         DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS semantic_invocations_op_idx ON semantic_invocations (operation_id);
CREATE INDEX IF NOT EXISTS semantic_invocations_parent_idx ON semantic_invocations (parent_invocation_id);

CREATE TABLE IF NOT EXISTS semantic_budget_reservations (
    budget_ref    VARCHAR(128) NOT NULL,
    invocation_id VARCHAR(128) NOT NULL,
    upper_bound   INTEGER NOT NULL,
    actual_tokens INTEGER,
    state         VARCHAR(32) NOT NULL,
    created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (budget_ref, invocation_id)
);

CREATE TABLE IF NOT EXISTS semantic_budgets (
    budget_ref   VARCHAR(128) PRIMARY KEY,
    total_units  INTEGER NOT NULL,
    spent_units  INTEGER NOT NULL DEFAULT 0
);
