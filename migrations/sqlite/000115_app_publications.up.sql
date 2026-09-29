-- External publication receipts (T18, #48) — sqlite track. Same shape as
-- the versioned migration.
CREATE TABLE app_publications (
    tenant_id INTEGER NOT NULL,
    action_id TEXT NOT NULL,
    connection_id TEXT NOT NULL,
    provider TEXT NOT NULL,
    mode TEXT NOT NULL,
    destination TEXT NOT NULL,
    expected_version TEXT NOT NULL DEFAULT '',
    artifact_version_id TEXT NOT NULL DEFAULT '',
    artifact_digest TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL,
    external_id TEXT NOT NULL DEFAULT '',
    external_version TEXT NOT NULL DEFAULT '',
    receipt_json TEXT NOT NULL DEFAULT '',
    progress_json TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, action_id)
);

CREATE INDEX idx_app_publications_destination ON app_publications (tenant_id, connection_id, external_id, state);
