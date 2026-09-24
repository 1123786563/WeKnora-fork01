-- T13 (#43): tenant-level task retention policy + compliance access windows.
-- The policy row is tenant-scoped (PK = tenant_id); a missing row means
-- "ungated" (today's behavior). The access windows are an INDEPENDENT flow
-- from task_grants (#42): a compliance window never joins the collaboration
-- list. Archiving stays unrestricted — only deletion lanes obey the policy.
CREATE TABLE tenant_task_policies (
    tenant_id BIGINT PRIMARY KEY,
    retention_days INTEGER NOT NULL DEFAULT 0 CHECK (retention_days >= 0),
    legal_hold BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by VARCHAR(512) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE task_compliance_access (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    task_id VARCHAR(36) NOT NULL,
    admin_id VARCHAR(512) NOT NULL,
    reason TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_compliance_access_task ON task_compliance_access (tenant_id, task_id);
CREATE INDEX idx_task_compliance_access_admin ON task_compliance_access (tenant_id, admin_id);
