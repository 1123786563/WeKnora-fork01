-- Durable, generation-bound permissions for RunView allocation and runtime
-- mutations. Pending and unknown intents remain unresolved across restarts.
CREATE UNIQUE INDEX uq_craft_run_views_run_generation
    ON craft_run_views (tenant_id, run_id, generation);

CREATE TABLE craft_run_view_effect_intents (
    tenant_id BIGINT NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    generation VARCHAR(64) NOT NULL,
    effect_kind VARCHAR(32) NOT NULL,
    claim_token VARCHAR(64) NOT NULL,
    actor_user_id VARCHAR(64) NOT NULL,
    writer_owner VARCHAR(128) NOT NULL,
    fence_epoch BIGINT NOT NULL,
    snapshot_digest_version INTEGER NOT NULL,
    snapshot_digest VARCHAR(64) NOT NULL,
    state VARCHAR(16) NOT NULL,
    outcome VARCHAR(16) NOT NULL DEFAULT '',
    receipt VARCHAR(2048) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMPTZ NULL,
    CONSTRAINT pk_craft_run_view_effect_intents PRIMARY KEY (tenant_id, run_id, generation, effect_kind),
    CONSTRAINT uq_craft_run_view_effect_claim_token UNIQUE (claim_token),
    CONSTRAINT ck_craft_run_view_effect_kind CHECK (effect_kind IN ('allocate', 'docker_create', 'docker_start', 'opencode_create')),
    CONSTRAINT ck_craft_run_view_effect_epoch CHECK (fence_epoch > 0),
    CONSTRAINT ck_craft_run_view_effect_digest_version CHECK (snapshot_digest_version > 0),
    CONSTRAINT ck_craft_run_view_effect_state CHECK (state IN ('pending', 'finished', 'unknown')),
    CONSTRAINT ck_craft_run_view_effect_outcome CHECK (outcome IN ('', 'succeeded', 'failed', 'unknown')),
    CONSTRAINT ck_craft_run_view_effect_transition CHECK (
        (state = 'pending' AND outcome = '' AND finished_at IS NULL) OR
        (state = 'finished' AND outcome IN ('succeeded', 'failed') AND finished_at IS NOT NULL) OR
        (state = 'unknown' AND outcome = 'unknown' AND finished_at IS NULL)
    ),
    CONSTRAINT fk_craft_run_view_effect_run FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT,
    CONSTRAINT fk_craft_run_view_effect_view FOREIGN KEY (tenant_id, run_id, generation)
        REFERENCES craft_run_views (tenant_id, run_id, generation) ON DELETE RESTRICT
);

CREATE INDEX idx_craft_run_view_effect_unresolved
    ON craft_run_view_effect_intents (tenant_id, run_id, state);
