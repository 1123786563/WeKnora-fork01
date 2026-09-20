CREATE TABLE semantic_document_revisions (
  tenant_id TEXT NOT NULL CHECK(tenant_id GLOB '[0-9]*' AND tenant_id NOT GLOB '*[^0-9]*' AND (tenant_id = '0' OR tenant_id NOT GLOB '0*')),
  kb_id TEXT NOT NULL, document_id TEXT NOT NULL,
  revision TEXT NOT NULL CHECK(revision GLOB '[0-9]*' AND revision NOT GLOB '*[^0-9]*' AND (revision = '0' OR revision NOT GLOB '0*')),
  content_hash TEXT NOT NULL, deleted INTEGER NOT NULL CHECK(deleted IN (0,1)),
  PRIMARY KEY (tenant_id, kb_id, document_id)
);
CREATE TABLE semantic_access_epochs (
  tenant_id TEXT NOT NULL CHECK(tenant_id GLOB '[0-9]*' AND tenant_id NOT GLOB '*[^0-9]*' AND (tenant_id = '0' OR tenant_id NOT GLOB '0*')),
  kb_id TEXT NOT NULL, epoch TEXT NOT NULL CHECK(epoch GLOB '[0-9]*' AND epoch NOT GLOB '*[^0-9]*' AND (epoch = '0' OR epoch NOT GLOB '0*')),
  PRIMARY KEY (tenant_id, kb_id)
);
CREATE TABLE semantic_denials (
  tenant_id TEXT NOT NULL, kb_id TEXT NOT NULL, document_id TEXT NOT NULL,
  revision TEXT NOT NULL CHECK(revision GLOB '[0-9]*' AND revision NOT GLOB '*[^0-9]*' AND (revision = '0' OR revision NOT GLOB '0*')),
  PRIMARY KEY (tenant_id, kb_id, document_id),
  FOREIGN KEY (tenant_id, kb_id, document_id) REFERENCES semantic_document_revisions(tenant_id, kb_id, document_id)
);
CREATE TABLE semantic_outbox (
  event_id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, kb_id TEXT NOT NULL, document_id TEXT NOT NULL,
  revision TEXT NOT NULL CHECK(revision GLOB '[0-9]*' AND revision NOT GLOB '*[^0-9]*' AND (revision = '0' OR revision NOT GLOB '0*')), content_hash TEXT NOT NULL, config_digest TEXT NOT NULL, deleted INTEGER NOT NULL CHECK(deleted IN (0,1)), payload BLOB NOT NULL, payload_hash TEXT NOT NULL,
  attempt_count TEXT NOT NULL DEFAULT '0' CHECK(attempt_count GLOB '[0-9]*' AND attempt_count NOT GLOB '*[^0-9]*' AND (attempt_count = '0' OR attempt_count NOT GLOB '0*')), lease_token TEXT NOT NULL DEFAULT '0' CHECK(lease_token GLOB '[0-9]*' AND lease_token NOT GLOB '*[^0-9]*' AND (lease_token = '0' OR lease_token NOT GLOB '0*')), lease_owner TEXT NOT NULL DEFAULT '', lease_expires_at DATETIME, retry_at DATETIME NOT NULL, error_code TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_semantic_outbox_claim ON semantic_outbox(retry_at, event_id);
