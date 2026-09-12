-- I03: generation manifests, active pointers and read leases.
CREATE TABLE IF NOT EXISTS semantic.generations (
    tenant_id     INTEGER NOT NULL,
    kb_id         VARCHAR(255) NOT NULL,
    generation    VARCHAR(128) NOT NULL,
    base_generation VARCHAR(128),
    complete      BOOLEAN NOT NULL DEFAULT FALSE,
    manifest      JSONB NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, kb_id, generation)
);

CREATE TABLE IF NOT EXISTS semantic.active_generations (
    tenant_id  INTEGER NOT NULL,
    kb_id      VARCHAR(255) NOT NULL,
    generation VARCHAR(128) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, kb_id)
);

CREATE TABLE IF NOT EXISTS semantic.read_leases (
    lease_id    VARCHAR(128) PRIMARY KEY,
    tenant_id   INTEGER NOT NULL,
    kb_id       VARCHAR(255) NOT NULL,
    generation  VARCHAR(128) NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS read_leases_scope_idx ON semantic.read_leases (tenant_id, kb_id, expires_at);
