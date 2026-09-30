CREATE TABLE career_spaces (
    tenant_id BIGINT NOT NULL,
    owner_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id)
);

CREATE TABLE career_idempotency_receipts (
    tenant_id BIGINT NOT NULL,
    owner_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL,
    response_json JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, request_id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES career_spaces (tenant_id, owner_id) ON DELETE CASCADE
);

CREATE TABLE career_profile_facts (
    tenant_id BIGINT NOT NULL,
    owner_id TEXT NOT NULL,
    fact_id TEXT NOT NULL,
    payload JSONB NOT NULL,
    confirmed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, fact_id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES career_spaces (tenant_id, owner_id) ON DELETE CASCADE
);

CREATE TABLE career_evidence (
    tenant_id BIGINT NOT NULL,
    owner_id TEXT NOT NULL,
    evidence_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    digest TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, evidence_id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES career_spaces (tenant_id, owner_id)
);

CREATE FUNCTION career_evidence_append_only() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'career evidence is append-only';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER career_evidence_no_update
    BEFORE UPDATE OR DELETE ON career_evidence
    FOR EACH ROW EXECUTE FUNCTION career_evidence_append_only();
