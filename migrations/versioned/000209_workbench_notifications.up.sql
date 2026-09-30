-- Workbench notification read model (MX-021 backing store for GET /workbench/inbox
-- and the overview unread count). Column set aligns with InboxNotificationRow
-- (internal/handler/session/workbench_inbox.go): tenant_id/id/owner_id/kind/title/
-- body/deep_link/read/created_at.
CREATE TABLE workbench_notifications (
    id VARCHAR(64) PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    kind VARCHAR(64) NOT NULL,
    title VARCHAR(255) NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    deep_link VARCHAR(512) NOT NULL DEFAULT '',
    read BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workbench_notifications_owner
    ON workbench_notifications (tenant_id, owner_id, read, created_at);
