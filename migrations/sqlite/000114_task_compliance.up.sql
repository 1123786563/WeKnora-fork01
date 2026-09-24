-- T13 (#43) — sqlite track. Same shape as the versioned migration; a missing
-- policy row means "ungated"; access windows are independent of task_grants.
CREATE TABLE tenant_task_policies (
    tenant_id INTEGER PRIMARY KEY,
    retention_days INTEGER NOT NULL DEFAULT 0 CHECK (retention_days >= 0),
    legal_hold INTEGER NOT NULL DEFAULT 0,
    updated_by TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE task_compliance_access (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    task_id TEXT NOT NULL,
    admin_id TEXT NOT NULL,
    reason TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_task_compliance_access_task ON task_compliance_access (tenant_id, task_id);
CREATE INDEX idx_task_compliance_access_admin ON task_compliance_access (tenant_id, admin_id);
