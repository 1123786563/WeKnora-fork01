-- Workbench notification read model (MX-021 backing store for GET /workbench/inbox
-- and the overview unread count). Column set aligns with InboxNotificationRow.
CREATE TABLE workbench_notifications (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    owner_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT '',
    deep_link TEXT NOT NULL DEFAULT '',
    read BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_workbench_notifications_owner
    ON workbench_notifications (tenant_id, owner_id, read, created_at);
