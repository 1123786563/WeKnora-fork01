-- Workbench device push registration (MX-021 backing store for POST
-- /workbench/inbox/devices and logout revocation). Column set aligns with
-- DeviceRegistrationRow (internal/handler/session/workbench_inbox.go):
-- tenant_id/device_id/owner_id/token/platform/revoked/created_at/updated_at.
CREATE TABLE workbench_device_registrations (
    id BIGSERIAL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    device_id VARCHAR(255) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    token TEXT NOT NULL,
    platform VARCHAR(32) NOT NULL DEFAULT '',
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_workbench_device_registrations_owner_device
    ON workbench_device_registrations (tenant_id, owner_id, device_id);
CREATE INDEX idx_workbench_device_registrations_owner
    ON workbench_device_registrations (tenant_id, owner_id, revoked);
