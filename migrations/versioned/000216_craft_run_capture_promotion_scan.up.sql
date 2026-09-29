CREATE TABLE craft_run_capture_promotion_cursor (
    id BIGINT PRIMARY KEY CHECK (id = 1),
    updated_at TIMESTAMPTZ NULL,
    tenant_id BIGINT NULL,
    workspace_id VARCHAR(64) NULL,
    run_id VARCHAR(64) NULL
);
INSERT INTO craft_run_capture_promotion_cursor (id) VALUES (1);

CREATE INDEX idx_craft_run_captures_promotion_scan
    ON craft_run_captures (updated_at, tenant_id, workspace_id, run_id)
    WHERE state IN ('sealed', 'advanced') AND draft_revision IS NOT NULL;

CREATE TABLE craft_run_capture_promotion_attempts (
    tenant_id BIGINT NOT NULL,
    workspace_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    retry_after TIMESTAMPTZ NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT FALSE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, workspace_id, run_id),
    FOREIGN KEY (tenant_id, workspace_id, run_id)
        REFERENCES craft_run_captures(tenant_id, workspace_id, run_id) ON DELETE CASCADE
);
CREATE INDEX idx_craft_capture_promotion_due
    ON craft_run_capture_promotion_attempts (completed, retry_after, tenant_id, workspace_id, run_id);
