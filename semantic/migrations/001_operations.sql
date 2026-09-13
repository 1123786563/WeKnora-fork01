-- I01: persistent operations with idempotency and worker leases.
-- Service-owned schema (never shared with the Go business database).
CREATE SCHEMA IF NOT EXISTS semantic;

CREATE TABLE IF NOT EXISTS semantic.schema_migrations (
    version    TEXT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS semantic.operations (
    operation_id      TEXT PRIMARY KEY,
    tenant_id         BIGINT  NOT NULL,
    kb_id             TEXT    NOT NULL,
    document_id       TEXT    NOT NULL,
    revision          BIGINT  NOT NULL,
    idempotency_key   TEXT    NOT NULL,
    payload_hash      TEXT    NOT NULL,
    state             TEXT    NOT NULL,
    stage             TEXT    NOT NULL DEFAULT '',
    lease_token       BIGINT  NOT NULL DEFAULT 0,
    lease_until       TIMESTAMPTZ,
    lease_owner       TEXT,
    error_code        TEXT,
    result_generation TEXT,
    retry_after       TIMESTAMPTZ,  -- reserved for I05 retry scheduling; unused until then
    request           JSONB   NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT operations_unique_idempotency UNIQUE (tenant_id, kb_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS operations_claimable_idx
    ON semantic.operations (created_at)
    WHERE state IN ('accepted', 'running');

CREATE INDEX IF NOT EXISTS operations_scope_idx
    ON semantic.operations (tenant_id, kb_id, operation_id);
