ALTER TABLE mobile_notification_intents DROP CONSTRAINT uq_mobile_notification_identity;
ALTER TABLE mobile_notification_intents
    ADD CONSTRAINT uq_mobile_notification_identity
    UNIQUE (tenant_id, event_id, owner_id, device_id, environment);
ALTER TABLE mobile_notification_intents DROP COLUMN app_id;

DROP INDEX uq_mobile_devices_active_token;
ALTER TABLE mobile_devices DROP CONSTRAINT mobile_devices_pkey;
ALTER TABLE mobile_devices ADD PRIMARY KEY (tenant_id, owner_id, device_id, environment);
ALTER TABLE mobile_devices DROP COLUMN app_id;
CREATE UNIQUE INDEX uq_mobile_devices_active_token
    ON mobile_devices (environment, token_hash)
    WHERE revoked_at IS NULL;
