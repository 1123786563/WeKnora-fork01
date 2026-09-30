-- Server-owned, Run-scoped execution view identity. This row records binding
-- state only; it does not provide a filesystem or process isolation boundary.
CREATE TABLE craft_run_views (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    generation VARCHAR(64) NOT NULL,
    runtime_id VARCHAR(255) NOT NULL DEFAULT '',
    container_id VARCHAR(255) NOT NULL DEFAULT '',
    opencode_session_id VARCHAR(255) NOT NULL DEFAULT '',
    state VARCHAR(16) NOT NULL DEFAULT 'allocating',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id),
    UNIQUE (generation),
    CHECK (state IN ('allocating', 'bound')),
    CHECK (
        (state = 'allocating' AND runtime_id = '' AND container_id = '' AND opencode_session_id = '') OR
        (state = 'bound' AND runtime_id <> '' AND container_id <> '' AND opencode_session_id <> '')
    ),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT
);

CREATE INDEX idx_craft_run_views_recovery ON craft_run_views (state, updated_at);
