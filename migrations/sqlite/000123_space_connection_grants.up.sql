-- T23 (#53): sqlite twin of versioned 000198 (per-actor grants on SPACE
-- connections). See the versioned file for the policy rationale.
CREATE TABLE app_space_connection_grants (
    tenant_id     INTEGER      NOT NULL,
    connection_id VARCHAR(64)  NOT NULL,
    actor_id      VARCHAR(512) NOT NULL,
    granted_by    VARCHAR(512) NOT NULL DEFAULT '',
    created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, connection_id, actor_id)
);
