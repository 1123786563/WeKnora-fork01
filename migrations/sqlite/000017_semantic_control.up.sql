-- I02: business-side semantic control tables (revision CAS, outbox,
-- authorization epochs, deletion barriers, backend states, receipts).
-- Owned by the Go business system; the semantic service NEVER writes these.
CREATE TABLE IF NOT EXISTS semantic_document_revisions (
    tenant_id INTEGER NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    revision INTEGER NOT NULL DEFAULT 0,
    deleted BOOLEAN NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, kb_id, document_id)
);

CREATE TABLE IF NOT EXISTS semantic_outbox (
    tenant_id INTEGER NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    revision INTEGER NOT NULL,
    event_id VARCHAR(512) NOT NULL,
    payload_hash VARCHAR(128) NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    claimed_until DATETIME,
    confirmed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (event_id),
    CONSTRAINT uq_semantic_outbox_identity UNIQUE (tenant_id, kb_id, document_id, revision)
);

CREATE TABLE IF NOT EXISTS semantic_access_epochs (
    tenant_id INTEGER NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    epoch INTEGER NOT NULL DEFAULT 0,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, kb_id)
);

CREATE TABLE IF NOT EXISTS semantic_denials (
    tenant_id INTEGER NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    revision INTEGER NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, kb_id, document_id, revision)
);

CREATE TABLE IF NOT EXISTS semantic_backend_states (
    tenant_id INTEGER NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    desired_backend VARCHAR(32) NOT NULL DEFAULT 'native',
    active_backend VARCHAR(32) NOT NULL DEFAULT 'native',
    config_version VARCHAR(64) NOT NULL DEFAULT '',
    active_generation VARCHAR(128) NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, kb_id)
);

-- Reserved for I05's idempotent subtask completion. I05's contract adds
-- (attempt, operation_id) columns via its own migration; this table lands
-- the scope identity now so both dialects share the same baseline.
CREATE TABLE IF NOT EXISTS semantic_completion_receipts (
    tenant_id INTEGER NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    revision INTEGER NOT NULL,
    task_key VARCHAR(128) NOT NULL,
    completed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, kb_id, document_id, revision, task_key)
);
