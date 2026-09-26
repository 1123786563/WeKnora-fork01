-- T37 (#67): official 与 enterprise 自构建 App 的注册与令牌永不混用（同 sqlite 000114
-- 语义；PostgreSQL 具名约束直接 ALTER）。存量行归 official；升级瞬间仍 pending 的旧
-- 5 段幂等 ID 行在重投影后以 6 段新 ID 再入队一次（at-least-once 容忍）。
ALTER TABLE mobile_devices ADD COLUMN app_id VARCHAR(64) NOT NULL DEFAULT 'official';
ALTER TABLE mobile_devices DROP CONSTRAINT mobile_devices_pkey;
ALTER TABLE mobile_devices ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment, app_id);
DROP INDEX IF EXISTS uq_mobile_devices_active_token;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, app_id, token_hash)
    WHERE revoked_at IS NULL;

ALTER TABLE mobile_notification_intents ADD COLUMN app_id VARCHAR(64) NOT NULL DEFAULT 'official';
ALTER TABLE mobile_notification_intents DROP CONSTRAINT uq_mobile_notification_identity;
ALTER TABLE mobile_notification_intents
    ADD CONSTRAINT uq_mobile_notification_identity
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment, app_id);
