-- 与 sqlite 000114 down 对称（review round 1 修复）：跨 App 数据无法双保留，回滚前
-- 确定性丢弃企业行。缺少 DELETE 时，同设备双 App 数据（official 与 enterprise:<slug>
-- 两行）会让下方 UNIQUE / PRIMARY KEY 重建因重复键直接失败，回滚卡死在 000193。
DELETE FROM mobile_notification_intents WHERE app_id <> 'official';
ALTER TABLE mobile_notification_intents DROP CONSTRAINT uq_mobile_notification_identity;
ALTER TABLE mobile_notification_intents
    ADD CONSTRAINT uq_mobile_notification_identity
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment);
ALTER TABLE mobile_notification_intents DROP COLUMN app_id;

DELETE FROM mobile_devices WHERE app_id <> 'official';
DROP INDEX uq_mobile_devices_active_token;
ALTER TABLE mobile_devices DROP CONSTRAINT mobile_devices_pkey;
ALTER TABLE mobile_devices ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment);
ALTER TABLE mobile_devices DROP COLUMN app_id;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, token_hash)
    WHERE revoked_at IS NULL;
