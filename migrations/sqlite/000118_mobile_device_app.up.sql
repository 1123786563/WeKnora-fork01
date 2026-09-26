-- T37 (#67): official 与 enterprise 自构建 App 的注册与令牌永不混用。app_id 进入
-- mobile_devices 主键（同一物理设备双 App 为两行独立）与令牌排他索引（跨 App 不互相
-- 接管）；mobile_notification_intents 的幂等身份含 app_id（投递按 App 路由 Provider）。
-- sqlite 无法 ALTER 主键/表级 UNIQUE，按 000055 同例整表重建。存量行归 official。
-- 注意：升级时仍 pending 的旧 5 段幂等 ID 行在重投影后会以 6 段新 ID 再入队一次
--（at-least-once 语义容忍，窗口仅升级瞬间的 pending 行）。
CREATE TABLE mobile_devices_rebuilt (
    tenant_id INTEGER NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    device_id VARCHAR(128) NOT NULL,
    environment VARCHAR(32) NOT NULL,
    app_id VARCHAR(64) NOT NULL DEFAULT 'official',
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
    PRIMARY KEY (tenant_id, owner_id, device_id, environment, app_id),
    CHECK (platform IN ('ios', 'android')),
    CHECK (revision > 0),
    CHECK (scope_generation >= 0)
);
INSERT INTO mobile_devices_rebuilt
    (tenant_id, owner_id, device_id, environment, app_id, space_id, platform, token_ciphertext,
     token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at)
SELECT tenant_id, owner_id, device_id, environment, 'official', space_id, platform, token_ciphertext,
       token_hash, revision, scope_generation, revoked_at, last_seen_at, created_at, updated_at
FROM mobile_devices;
DROP TABLE mobile_devices;
ALTER TABLE mobile_devices_rebuilt RENAME TO mobile_devices;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, app_id, token_hash)
    WHERE revoked_at IS NULL;
CREATE INDEX idx_mobile_devices_owner
    ON mobile_devices (tenant_id, owner_id, environment, revoked_at, updated_at);

CREATE TABLE mobile_notification_intents_rebuilt (
    id TEXT PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    event_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    device_id TEXT NOT NULL,
    environment TEXT NOT NULL,
    app_id TEXT NOT NULL DEFAULT 'official',
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
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment, app_id),
    CHECK (state IN ('pending', 'in_flight', 'sent', 'expired')),
    CHECK (attempt >= 0),
    CHECK (fence >= 0)
);
INSERT INTO mobile_notification_intents_rebuilt
    (id, tenant_id, event_id, owner_id, device_id, environment, app_id, kind, run_id, expires_at,
     state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
     created_at, updated_at)
SELECT id, tenant_id, event_id, owner_id, device_id, environment, 'official', kind, run_id, expires_at,
       state, attempt, lease_owner, lease_until, next_attempt_at, fence, receipt_id, last_error,
       created_at, updated_at
FROM mobile_notification_intents;
DROP TABLE mobile_notification_intents;
ALTER TABLE mobile_notification_intents_rebuilt RENAME TO mobile_notification_intents;
CREATE INDEX idx_mobile_notification_claim ON mobile_notification_intents (state, lease_until, expires_at, created_at);
CREATE INDEX idx_mobile_notification_owner ON mobile_notification_intents (tenant_id, owner_id, run_id, created_at);
CREATE INDEX idx_mobile_notification_next_attempt ON mobile_notification_intents (state, next_attempt_at, expires_at, created_at);
