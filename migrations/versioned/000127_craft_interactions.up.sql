-- C02: durable craft interactions and the decision delivery outbox.
--
-- One pending question or permission raised by a delegated sub-execution is
-- one craft_interactions row; the user decision and its pending-delivery
-- outbox item are written in the SAME transaction (ApplyInteractionDecision),
-- so a crash can never record a decision without its redelivery record or
-- vice versa. Referential design follows 000121_craft: the interaction FK
-- targets sessions(id) — the retention root — and the outbox cascades with
-- its interaction. No physical FK to agent_runs: identity is enforced by the
-- store under the run row at write time, and journal-table drops stay
-- replayable.
CREATE TABLE craft_interactions (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    tenant_id INTEGER NOT NULL,
    session_id VARCHAR(36) NOT NULL,
    owner_id VARCHAR(512) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    task_id VARCHAR(64) NOT NULL DEFAULT '',
    tool_call_id VARCHAR(255) NOT NULL DEFAULT '',
    pending_id VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    args_hash VARCHAR(64) NOT NULL,
    prompt TEXT NOT NULL,
    oc_session_id VARCHAR(64) NOT NULL DEFAULT '',
    oc_request_id VARCHAR(64) NOT NULL DEFAULT '',
    payload_json JSONB,
    status VARCHAR(32) NOT NULL DEFAULT 'pending',
    delivery VARCHAR(32) NOT NULL DEFAULT 'pending',
    decision_id VARCHAR(128) NOT NULL DEFAULT '',
    decided_action VARCHAR(32) NOT NULL DEFAULT '',
    answers_json JSONB,
    decided_by VARCHAR(512) NOT NULL DEFAULT '',
    revision BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT fk_craft_interactions_session
        FOREIGN KEY (session_id)
        REFERENCES sessions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_interactions_session ON craft_interactions (tenant_id, session_id, status);
CREATE INDEX idx_craft_interactions_run ON craft_interactions (tenant_id, run_id, status);

CREATE TABLE craft_decision_outbox (
    tenant_id INTEGER NOT NULL,
    interaction_id VARCHAR(64) NOT NULL,
    id VARCHAR(128) NOT NULL,
    run_id VARCHAR(64) NOT NULL,
    pending_id VARCHAR(64) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    action VARCHAR(32) NOT NULL,
    answers_json JSONB,
    args_hash VARCHAR(64) NOT NULL,
    expected_revision BIGINT NOT NULL,
    payload_hash VARCHAR(64) NOT NULL,
    delivery VARCHAR(32) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT,
    decided_by VARCHAR(512) NOT NULL DEFAULT '',
    oc_session_id VARCHAR(64) NOT NULL DEFAULT '',
    oc_request_id VARCHAR(64) NOT NULL DEFAULT '',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    delivered_at TIMESTAMP,
    PRIMARY KEY (tenant_id, interaction_id, id),
    CONSTRAINT fk_craft_decision_outbox_interaction
        FOREIGN KEY (interaction_id)
        REFERENCES craft_interactions (id) ON DELETE CASCADE
);

CREATE INDEX idx_craft_decision_outbox_run ON craft_decision_outbox (tenant_id, run_id, delivery);
