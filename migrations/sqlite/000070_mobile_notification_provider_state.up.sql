CREATE TABLE mobile_notification_provider_state (
    provider_key TEXT PRIMARY KEY,
    paused INTEGER NOT NULL DEFAULT 0,
    reason TEXT NOT NULL DEFAULT '',
    alert_count INTEGER NOT NULL DEFAULT 0,
    paused_at DATETIME,
    recovered_at DATETIME,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
