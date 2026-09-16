CREATE TABLE mobile_devices (
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    environment VARCHAR(32) NOT NULL,
    space_id VARCHAR(128) NOT NULL DEFAULT '',
    platform VARCHAR(16) NOT NULL,
    token_ciphertext TEXT NOT NULL,
    token_hash VARCHAR(64) NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1,
    scope_generation INTEGER NOT NULL DEFAULT 0,
    revoked_at DATETIME,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, device_id, environment),
    CHECK (platform IN ('ios', 'android')),
    CHECK (revision > 0),
    CHECK (scope_generation >= 0)
);
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, token_hash)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_mobile_devices_owner
    ON mobile_devices (tenant_id, owner_id, environment, revoked_at, updated_at);
