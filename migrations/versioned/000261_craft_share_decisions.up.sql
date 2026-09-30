-- T11 (#128): owner consent before sharing restricted-source results.
-- One row per (tenant, session, version) holds the LATEST owner decision,
-- bound to the exact evidence digest it was made against. The service
-- upserts on fresh decisions and marks revoked_at on withdrawal; authority
-- checks re-derive the digest from recorded evidence, so a stale consent
-- (evidence changed, TTL expired, decision revoked/declined) never grants.
CREATE TABLE craft_share_decisions (
    tenant_id BIGINT NOT NULL,
    session_id VARCHAR(128) NOT NULL,
    version_id VARCHAR(128) NOT NULL,
    evidence_digest CHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    decision VARCHAR(16) NOT NULL,
    decided_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id, version_id)
);
CREATE INDEX idx_craft_share_decisions_owner ON craft_share_decisions (owner_id);
