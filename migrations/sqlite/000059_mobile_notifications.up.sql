CREATE TABLE mobile_notification_intents (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    event_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    kind TEXT NOT NULL,
    run_id TEXT NOT NULL,
    expires_at DATETIME NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending',
    attempt INTEGER NOT NULL DEFAULT 0,
    lease_owner TEXT NOT NULL DEFAULT '',
    lease_until DATETIME,
    fence INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment),
    CHECK (state IN ('pending', 'in_flight', 'sent', 'expired')),
    CHECK (attempt >= 0),
    CHECK (fence >= 0)
);
CREATE INDEX idx_mobile_notification_claim ON mobile_notification_intents (state, lease_until, expires_at, created_at);
CREATE INDEX idx_mobile_notification_owner ON mobile_notification_intents (tenant_id, owner_id, run_id, created_at);
