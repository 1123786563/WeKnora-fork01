CREATE TABLE semantic_document_revisions (
 tenant_id NUMERIC(20,0) NOT NULL CHECK (tenant_id >= 0), kb_id TEXT NOT NULL, document_id TEXT NOT NULL,
 revision NUMERIC(20,0) NOT NULL CHECK (revision >= 0), content_hash TEXT NOT NULL, deleted BOOLEAN NOT NULL,
 PRIMARY KEY (tenant_id, kb_id, document_id)
);
CREATE TABLE semantic_access_epochs (tenant_id NUMERIC(20,0) NOT NULL CHECK (tenant_id >= 0), kb_id TEXT NOT NULL, epoch NUMERIC(20,0) NOT NULL CHECK(epoch >= 0), PRIMARY KEY (tenant_id,kb_id));
CREATE TABLE semantic_denials (tenant_id NUMERIC(20,0) NOT NULL CHECK(tenant_id >= 0), kb_id TEXT NOT NULL, document_id TEXT NOT NULL, revision NUMERIC(20,0) NOT NULL CHECK(revision >= 0), PRIMARY KEY(tenant_id,kb_id,document_id), FOREIGN KEY(tenant_id,kb_id,document_id) REFERENCES semantic_document_revisions(tenant_id,kb_id,document_id));
CREATE TABLE semantic_outbox (event_id UUID PRIMARY KEY, tenant_id NUMERIC(20,0) NOT NULL CHECK(tenant_id >= 0), kb_id TEXT NOT NULL, document_id TEXT NOT NULL, revision NUMERIC(20,0) NOT NULL CHECK(revision >= 0), content_hash TEXT NOT NULL, deleted BOOLEAN NOT NULL, payload BYTEA NOT NULL, payload_hash TEXT NOT NULL, attempt_count BIGINT NOT NULL DEFAULT 0 CHECK(attempt_count >= 0), lease_token BIGINT NOT NULL DEFAULT 0 CHECK(lease_token >= 0), lease_expires_at TIMESTAMPTZ, retry_at TIMESTAMPTZ NOT NULL, error_code TEXT NOT NULL DEFAULT '');
CREATE INDEX idx_semantic_outbox_claim ON semantic_outbox(retry_at, event_id);
