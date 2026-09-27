-- T07 (#131): pin historical artifact versions to their original evidence.
-- One evidence row per version, written in the SAME transaction that
-- publishes the version: the promoting Run's immutable knowledge record is
-- captured as evidence_json with an integrity digest, so a historical
-- version's citation facts stay provable after the knowledge is updated,
-- revoked or deleted — the read never reconstructs from the Workspace or
-- the current knowledge base (unpinned versions answer not-found).
-- acquired_at NOT NULL carries a zero time legally: an empty record's zero
-- acquisition time is an explicit fact, not missing data.
CREATE TABLE craft_version_evidence (
    version_id VARCHAR(71) NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    evidence_json TEXT NOT NULL,
    digest CHAR(64) NOT NULL,
    acquired_at TIMESTAMPTZ NOT NULL,
    pinned_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT fk_craft_version_evidence_version
        FOREIGN KEY (version_id)
        REFERENCES craft_versions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_version_evidence_tenant ON craft_version_evidence (tenant_id);
