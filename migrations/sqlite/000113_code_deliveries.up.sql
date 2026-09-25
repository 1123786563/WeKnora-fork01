-- T22 (#52)：代码交付追溯行。task_id=sessionId（ADR-0004）；action_id 锚定
-- A03 审批（摘要/批准人），commit/pr/remote_login 是远端回执。
CREATE TABLE IF NOT EXISTS code_deliveries (
    id             VARCHAR(64)  NOT NULL PRIMARY KEY,
    tenant_id      BIGINT       NOT NULL,
    task_id        VARCHAR(64)  NOT NULL,
    run_id         VARCHAR(64)  NOT NULL,
    owner_id       VARCHAR(512) NOT NULL DEFAULT '',
    action_id      VARCHAR(64)  NOT NULL DEFAULT '',
    connection_id  VARCHAR(64)  NOT NULL DEFAULT '',
    repo           VARCHAR(256) NOT NULL DEFAULT '',
    baseline_sha   CHAR(40)     NOT NULL DEFAULT '',
    branch         VARCHAR(256) NOT NULL DEFAULT '',
    commit_sha     CHAR(40)     NOT NULL DEFAULT '',
    pr_number      BIGINT       NOT NULL DEFAULT 0,
    pr_url         VARCHAR(512) NOT NULL DEFAULT '',
    remote_login   VARCHAR(256) NOT NULL DEFAULT '',
    state          VARCHAR(32)  NOT NULL DEFAULT 'prepared',
    failure        VARCHAR(512) NOT NULL DEFAULT '',
    created_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_code_deliveries_tenant_run ON code_deliveries (tenant_id, run_id, created_at);
