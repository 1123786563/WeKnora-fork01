-- I05: attempt-namespaced completion receipts (extends 000096).
ALTER TABLE semantic_completion_receipts ADD COLUMN attempt INTEGER;
ALTER TABLE semantic_completion_receipts ADD COLUMN operation_id VARCHAR(128);

CREATE TABLE IF NOT EXISTS semantic_task_operations (
    tenant_id BIGINT NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    attempt BIGINT NOT NULL,
    operation_id VARCHAR(128) NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL DEFAULT 'apply',
    state VARCHAR(32) NOT NULL DEFAULT 'pending',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, kb_id, document_id, attempt, operation_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS semantic_task_operations_idem_idx
    ON semantic_task_operations (idempotency_key);

CREATE TABLE IF NOT EXISTS semantic_attempt_counters (
    tenant_id BIGINT NOT NULL,
    kb_id VARCHAR(255) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    attempt BIGINT NOT NULL,
    pending BIGINT NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, kb_id, document_id, attempt)
);
