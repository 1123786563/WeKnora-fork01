-- T13 (#133): owner consent before exporting restricted derived data.
-- One row per (tenant, session, version) holds the LATEST owner decision,
-- bound to the exact export-manifest digest it was made against. The service
-- upserts on fresh decisions (a new decision overwrites the previous one);
-- GrantsExportAuthority re-derives the digest from the immutable version, so
-- a stale consent (manifest changed, decision replaced, owner moved) never
-- grants. No TTL by design: no shareable URL exists, every download
-- re-verifies fresh, and a rejection can overwrite an approval anytime.
CREATE TABLE craft_export_decisions (
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(128) NOT NULL,
    version_id VARCHAR(128) NOT NULL,
    manifest_digest CHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    decision VARCHAR(16) NOT NULL,
    decided_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id, version_id)
);
