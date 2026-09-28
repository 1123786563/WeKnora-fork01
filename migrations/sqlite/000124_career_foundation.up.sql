CREATE TABLE career_spaces (
    tenant_id INTEGER NOT NULL,
    owner_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id)
);

CREATE TABLE career_idempotency_receipts (
    tenant_id INTEGER NOT NULL,
    owner_id TEXT NOT NULL,
    request_id TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    status TEXT NOT NULL,
    response_json TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, request_id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES career_spaces (tenant_id, owner_id) ON DELETE CASCADE
);

CREATE TABLE career_profile_facts (
    tenant_id INTEGER NOT NULL,
    owner_id TEXT NOT NULL,
    fact_id TEXT NOT NULL,
    payload TEXT NOT NULL,
    confirmed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, fact_id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES career_spaces (tenant_id, owner_id) ON DELETE CASCADE
);

CREATE TABLE career_evidence (
    tenant_id INTEGER NOT NULL,
    owner_id TEXT NOT NULL,
    evidence_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    version_id TEXT NOT NULL,
    digest TEXT NOT NULL,
    payload TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, owner_id, evidence_id),
    FOREIGN KEY (tenant_id, owner_id) REFERENCES career_spaces (tenant_id, owner_id)
);

CREATE TRIGGER career_evidence_no_update
    BEFORE UPDATE ON career_evidence
    BEGIN SELECT RAISE(ABORT, 'career evidence is append-only'); END;

CREATE TRIGGER career_evidence_no_delete
    BEFORE DELETE ON career_evidence
    BEGIN SELECT RAISE(ABORT, 'career evidence is append-only'); END;
