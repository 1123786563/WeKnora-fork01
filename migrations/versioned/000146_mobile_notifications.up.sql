-- W14 follows W13's mobile_devices migration (000136). The plan's original
-- 000125 label collided with the existing craft migration and is preserved in
-- the SDD ruling instead of replacing an applied migration.
CREATE TABLE mobile_notification_intents (
    id VARCHAR(768) PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    event_id VARCHAR(256) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    environment VARCHAR(32) NOT NULL,
    kind VARCHAR(64) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'pending',
    attempt BIGINT NOT NULL DEFAULT 0,
    lease_owner VARCHAR(128) NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    fence BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_mobile_notification_identity UNIQUE (tenant_id, event_id, owner_id, device_id, environment),
    CHECK (state IN ('pending', 'in_flight', 'sent', 'expired')),
    CHECK (attempt >= 0),
    CHECK (fence >= 0)
);
CREATE INDEX idx_mobile_notification_claim ON mobile_notification_intents (state, lease_until, expires_at, created_at);
CREATE INDEX idx_mobile_notification_owner ON mobile_notification_intents (tenant_id, owner_id, run_id, created_at);
CREATE TABLE mobile_notification_checkpoints (
    consumer VARCHAR(128) NOT NULL,
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    cursor BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (consumer, tenant_id, run_id)
);
