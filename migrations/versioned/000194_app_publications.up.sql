-- External publication receipts (T18, #48): the durable record of an
-- Action Plan that publishes a confirmed artifact version to an external
-- office system. planned rows are written at plan formation; terminal
-- states are settled from the authoritative app_actions outcome. The NO-03
-- multi-step progress checkpoint rides progress_json. Provider column
-- starts with notion; feishu (#49) and confluence (#50) join later.
CREATE TABLE app_publications (
    tenant_id INTEGER NOT NULL,
    action_id VARCHAR(64) NOT NULL,
    connection_id VARCHAR(64) NOT NULL,
    provider VARCHAR(32) NOT NULL,
    mode VARCHAR(16) NOT NULL,
    destination VARCHAR(128) NOT NULL,
    expected_version VARCHAR(64) NOT NULL DEFAULT '',
    artifact_version_id VARCHAR(64) NOT NULL DEFAULT '',
    artifact_digest VARCHAR(64) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL,
    external_id VARCHAR(128) NOT NULL DEFAULT '',
    external_version VARCHAR(64) NOT NULL DEFAULT '',
    receipt_json TEXT NOT NULL DEFAULT '',
    progress_json TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, action_id)
);

CREATE INDEX idx_app_publications_destination ON app_publications (tenant_id, connection_id, external_id, state);
