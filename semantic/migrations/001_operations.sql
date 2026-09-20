CREATE TABLE {{schema}}.operations (
    operation_id TEXT PRIMARY KEY,
    tenant_id NUMERIC(20, 0) NOT NULL,
    kb_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    revision NUMERIC(20, 0) NOT NULL,
    config_digest TEXT NOT NULL,
    idempotency_key TEXT NOT NULL,
    payload_hash TEXT NOT NULL,
    phase TEXT NOT NULL,
    stage TEXT NOT NULL,
    request_bytes BYTEA,
    lease_owner TEXT,
    lease_until TIMESTAMPTZ,
    lease_token NUMERIC(20, 0) NOT NULL DEFAULT 0 CHECK (lease_token >= 0 AND lease_token <= 18446744073709551615),
    result_generation TEXT,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CONSTRAINT operations_scope_idempotency_unique UNIQUE (tenant_id, kb_id, idempotency_key),
    CONSTRAINT operations_document_revision_config_unique UNIQUE (tenant_id, kb_id, document_id, revision, config_digest)
);

CREATE INDEX operations_claim_index
    ON {{schema}}.operations (phase, lease_until, created_at);
