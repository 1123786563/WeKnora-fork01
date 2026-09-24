-- Durable tenant-scoped journal for chargeable external start attempts.
CREATE TABLE craft_charge_start_journal (
    tenant_id       BIGINT       NOT NULL,
    run_id          VARCHAR(128) NOT NULL,
    activity_key    VARCHAR(256) NOT NULL,
    grant_id        VARCHAR(128) NOT NULL,
    call_id         VARCHAR(256) NOT NULL,
    reservation_key VARCHAR(320) NOT NULL,
    state           VARCHAR(32)  NOT NULL,
    run_revision    BIGINT       NOT NULL,
    created_at      DATETIME    NOT NULL,
    updated_at      DATETIME    NOT NULL,
    PRIMARY KEY (tenant_id, run_id, activity_key),
    UNIQUE (tenant_id, reservation_key),
    CHECK (state IN ('intent', 'started', 'unknown', 'definitely_unstarted')),
    FOREIGN KEY (tenant_id, run_id)
        REFERENCES agent_runs (tenant_id, run_id)
);

CREATE INDEX idx_craft_charge_start_journal_state_updated_at
    ON craft_charge_start_journal (state, updated_at);
