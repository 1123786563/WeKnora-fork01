CREATE TABLE mobile_notification_provider_state (
    provider_key VARCHAR(64) PRIMARY KEY,
    paused BOOLEAN NOT NULL DEFAULT FALSE,
    reason TEXT NOT NULL DEFAULT '',
    alert_count BIGINT NOT NULL DEFAULT 0,
    paused_at TIMESTAMPTZ,
    recovered_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
