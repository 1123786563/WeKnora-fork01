-- 回滚：跨 App 数据无法双保留，确定性丢弃企业行后重建无 app_id 的原表（000058/000059+000060 形状）。
DELETE FROM mobile_notification_intents WHERE app_id <> 'official';
CREATE TABLE mobile_notification_intents_rebuilt (
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
    next_attempt_at DATETIME,
    fence INTEGER NOT NULL DEFAULT 0,
    receipt_id VARCHAR(256) NOT NULL DEFAULT '',
    last_error TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment),
    CHECK (state IN ('pending', 'in_flight', 'sent', 'expired')),
    CHECK (attempt >= 0),
    CHECK (fence >= 0)
);
INSERT INTO mobile_notification_intents_rebuilt
    (id, tenant_id, event_id, owner_id, device_id, environment, kind, run_id, expires_at,
     state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
     created_at, updated_at)
SELECT id, tenant_id, event_id, owner_id, device_id, environment, kind, run_id, expires_at,
       state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
       created_at, updated_at
FROM mobile_notification_intents;
DROP TABLE mobile_notification_intents;
ALTER TABLE mobile_notification_intents_rebuilt RENAME TO mobile_notification_intents;
CREATE INDEX idx_mobile_notification_claim ON mobile_notification_intents (state, lease_until, expires_at, created_at);
CREATE INDEX idx_mobile_notification_owner ON mobile_notification_intents (tenant_id, owner_id, run_id, created_at);
CREATE INDEX idx_mobile_notification_next_attempt ON mobile_notification_intents (state, next_attempt_at, expires_at, created_at);

DELETE FROM mobile_devices WHERE app_id <> 'official';
CREATE TABLE mobile_devices_rebuilt (
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
INSERT INTO mobile_devices_rebuilt
    (tenant_id, owner_id, device_id, environment, space_id, platform, token_ciphertext,
     token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at)
SELECT tenant_id, owner_id, device_id, environment, space_id, platform, token_ciphertext,
       token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at
FROM mobile_devices;
DROP TABLE mobile_devices;
ALTER TABLE mobile_devices_rebuilt RENAME TO mobile_devices;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, token_hash)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_mobile_devices_owner
    ON mobile_devices (tenant_id, owner_id, environment, revoked_at, updated_at);
