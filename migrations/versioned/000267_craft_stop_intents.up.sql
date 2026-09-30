-- T17 (#136): the durable member stop intent. One row per (tenant, session,
-- run) holds the Run's stop journey state — requested before anything is
-- aborted, confirmed only by the authoritative observation, unknown when the
-- abort outcome could not be observed. A refresh replays the same durable
-- answer and a confirmed stop is never downgraded.
CREATE TABLE craft_stop_intents (
    tenant_id BIGINT NOT NULL,
    session_id VARCHAR(128) NOT NULL,
    run_id VARCHAR(128) NOT NULL,
    status VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, session_id, run_id)
);
