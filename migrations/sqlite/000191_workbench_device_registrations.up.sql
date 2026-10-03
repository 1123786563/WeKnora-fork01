-- Workbench device push registration (MX-021 backing store for POST
-- /workbench/inbox/devices and logout revocation). Column set aligns with
-- DeviceRegistrationRow.
CREATE TABLE workbench_device_registrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    device_id TEXT NOT NULL,
    owner_id TEXT NOT NULL,
    token TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT '',
    revoked BOOLEAN NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX uq_workbench_device_registrations_owner_device
    ON workbench_device_registrations (tenant_id, owner_id, device_id);
CREATE INDEX idx_workbench_device_registrations_owner
    ON workbench_device_registrations (tenant_id, owner_id, revoked);
