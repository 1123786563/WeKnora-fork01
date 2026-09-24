-- T01/T05/T08 durable first-frontier facts for Craft web artifacts.
ALTER TABLE craft_workspace_inputs
    ADD COLUMN recognition_accepted BOOLEAN;
ALTER TABLE craft_workspace_inputs
    ADD COLUMN recognition_understood BOOLEAN;
ALTER TABLE craft_workspace_inputs
    ADD COLUMN recognition_reason TEXT;

CREATE TABLE craft_knowledge_records (
    tenant_id BIGINT NOT NULL,
    session_id VARCHAR(128) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    record_json TEXT NOT NULL,
    digest CHAR(64) NOT NULL,
    acquired_at TIMESTAMP NOT NULL,
    PRIMARY KEY (tenant_id, session_id, run_id)
);

CREATE TABLE craft_task_grants (
    tenant_id BIGINT NOT NULL,
    membership_id BIGINT NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    user_id VARCHAR(512) NOT NULL,
    role VARCHAR(16) NOT NULL,
    granted_by VARCHAR(512) NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    PRIMARY KEY (tenant_id, session_id, user_id)
);

CREATE INDEX idx_craft_task_grants_tenant_user
    ON craft_task_grants (tenant_id, user_id);
