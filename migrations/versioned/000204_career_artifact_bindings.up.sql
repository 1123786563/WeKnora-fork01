CREATE TABLE career_artifact_bindings (
    tenant_id BIGINT NOT NULL,
    owner_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    version_id VARCHAR(64) NOT NULL,
    revoked_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, resource_id, version_id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES career_spaces (tenant_id, owner_id),
    FOREIGN KEY (tenant_id, version_id) REFERENCES artifact_versions (tenant_id, id)
);

CREATE INDEX career_artifact_binding_version
    ON career_artifact_bindings (tenant_id, version_id);
