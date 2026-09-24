-- A new physical effect row cannot be represented by the pre-000126 schema.
-- Refuse rollback while such rows exist rather than discarding or relabeling
-- their durable authorization history.
CREATE TEMP TABLE IF NOT EXISTS craft_run_view_effect_down_guard (ok INTEGER CHECK (ok = 1));
CREATE TEMP TRIGGER IF NOT EXISTS craft_run_view_effect_down_guard_trigger
BEFORE INSERT ON craft_run_view_effect_down_guard
WHEN EXISTS (
    SELECT 1 FROM craft_run_view_effect_intents
    WHERE effect_kind IN ('docker_network_create', 'docker_probe')
)
BEGIN
    SELECT RAISE(ABORT, 'cannot downgrade RunView effect request digests while new effect rows exist');
END;
INSERT INTO craft_run_view_effect_down_guard (ok) VALUES (1);
DROP TRIGGER craft_run_view_effect_down_guard_trigger;
DROP TABLE craft_run_view_effect_down_guard;

CREATE TABLE craft_run_view_effect_intents_pre_request_digest (
    tenant_id INTEGER NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    generation VARCHAR(64) NOT NULL,
    effect_kind VARCHAR(32) NOT NULL,
    claim_token VARCHAR(64) NOT NULL UNIQUE,
    actor_user_id VARCHAR(64) NOT NULL,
    writer_owner VARCHAR(128) NOT NULL,
    fence_epoch INTEGER NOT NULL,
    snapshot_digest_version INTEGER NOT NULL,
    snapshot_digest VARCHAR(64) NOT NULL,
    state VARCHAR(16) NOT NULL,
    outcome VARCHAR(16) NOT NULL DEFAULT '',
    receipt VARCHAR(2048) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME NULL,
    PRIMARY KEY (tenant_id, run_id, generation, effect_kind),
    CHECK (effect_kind IN ('allocate', 'docker_create', 'docker_start', 'opencode_create')),
    CHECK (fence_epoch > 0),
    CHECK (snapshot_digest_version > 0),
    CHECK (state IN ('pending', 'finished', 'unknown')),
    CHECK (outcome IN ('', 'succeeded', 'failed', 'unknown')),
    CHECK (
        (state = 'pending' AND outcome = '' AND finished_at IS NULL) OR
        (state = 'finished' AND outcome IN ('succeeded', 'failed') AND finished_at IS NOT NULL) OR
        (state = 'unknown' AND outcome = 'unknown' AND finished_at IS NULL)
    ),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id, generation)
        REFERENCES craft_run_views (tenant_id, run_id, generation) ON DELETE RESTRICT
);

INSERT INTO craft_run_view_effect_intents_pre_request_digest (
    tenant_id, run_id, owner_id, session_id, generation, effect_kind, claim_token,
    actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
    state, outcome, receipt, created_at, updated_at, finished_at
)
SELECT tenant_id, run_id, owner_id, session_id, generation, effect_kind, claim_token,
       actor_user_id, writer_owner, fence_epoch, snapshot_digest_version, snapshot_digest,
       state, outcome, receipt, created_at, updated_at, finished_at
FROM craft_run_view_effect_intents;

DROP INDEX IF EXISTS idx_craft_run_view_effect_unresolved;
DROP TABLE craft_run_view_effect_intents;
ALTER TABLE craft_run_view_effect_intents_pre_request_digest RENAME TO craft_run_view_effect_intents;
CREATE INDEX idx_craft_run_view_effect_unresolved
    ON craft_run_view_effect_intents (tenant_id, run_id, state);
