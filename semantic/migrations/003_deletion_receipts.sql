-- I04: fact store for support revocation, tombstones and deletion receipts.
-- The service's OWN fact graph: assertions with support sets, enabling
-- "delete one source keeps other support" and recursive derived invalidation.
CREATE TABLE IF NOT EXISTS semantic.assertions (
    tenant_id     INTEGER NOT NULL,
    kb_id         VARCHAR(255) NOT NULL,
    assertion_id  VARCHAR(128) NOT NULL,
    subject_id    VARCHAR(128) NOT NULL,
    predicate     VARCHAR(128) NOT NULL,
    object_id     VARCHAR(128),
    value         TEXT,
    kind          VARCHAR(32) NOT NULL,
    support_document_id VARCHAR(255) NOT NULL,
    support_revision    INTEGER NOT NULL,
    evidence_ids  TEXT NOT NULL DEFAULT '[]',
    premise_ids   TEXT NOT NULL DEFAULT '[]',
    visible       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, kb_id, assertion_id, support_document_id, support_revision)
);

CREATE INDEX IF NOT EXISTS assertions_subject_idx ON semantic.assertions (tenant_id, kb_id, subject_id, predicate);
CREATE INDEX IF NOT EXISTS assertions_premise_idx ON semantic.assertions (tenant_id, kb_id, visible);

-- Denial barriers: monotone tombstones overlaid on EVERY generation query.
CREATE TABLE IF NOT EXISTS semantic.tombstones (
    tenant_id   INTEGER NOT NULL,
    kb_id       VARCHAR(255) NOT NULL,
    document_id VARCHAR(255) NOT NULL,
    revision    INTEGER NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, kb_id, document_id, revision)
);

-- Per-store deletion cleanup receipts.
CREATE TABLE IF NOT EXISTS semantic.deletion_receipts (
    operation_id       VARCHAR(128) PRIMARY KEY,
    tenant_id          INTEGER NOT NULL,
    kb_id              VARCHAR(255) NOT NULL,
    document_id        VARCHAR(255) NOT NULL,
    tombstone_revision INTEGER NOT NULL,
    graph_state        VARCHAR(32) NOT NULL DEFAULT 'pending',
    vector_state       VARCHAR(32) NOT NULL DEFAULT 'pending',
    object_state       VARCHAR(32) NOT NULL DEFAULT 'pending',
    cache_state        VARCHAR(32) NOT NULL DEFAULT 'pending',
    backup_state       VARCHAR(32) NOT NULL DEFAULT 'retention_pending',
    completed_at       TIMESTAMPTZ,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);