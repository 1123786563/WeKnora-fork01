-- F08 Task L: durable first terminal observation for one fixed web build
-- attempt. This table intentionally has no cascading foreign key so receipt
-- evidence cannot disappear when an unrelated owning row is cleaned up.
CREATE TABLE craft_web_build_receipts (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    tenant_id BIGINT NOT NULL CHECK (tenant_id > 0),
    task_id VARCHAR(128) NOT NULL CHECK (length(trim(task_id)) > 0),
    session_id VARCHAR(64) NOT NULL CHECK (length(trim(session_id)) > 0),
    workspace_id VARCHAR(64) NOT NULL CHECK (length(trim(workspace_id)) > 0),
    run_id VARCHAR(64) NOT NULL CHECK (length(trim(run_id)) > 0),
    activity_key VARCHAR(128) NOT NULL CHECK (length(trim(activity_key)) > 0),
    request_sha256 VARCHAR(64) NOT NULL CHECK (length(request_sha256) = 64),
    command_sha256 VARCHAR(64) NOT NULL CHECK (length(command_sha256) = 64),
    runtime_digest TEXT NOT NULL CHECK (length(trim(runtime_digest)) > 0),
    toolchain_digest VARCHAR(64) NOT NULL CHECK (length(toolchain_digest) = 64),
    template_version VARCHAR(128) NOT NULL CHECK (length(trim(template_version)) > 0),
    template_sha256 VARCHAR(64) NOT NULL CHECK (length(template_sha256) = 64),
    timeout_ms BIGINT NOT NULL CHECK (timeout_ms > 0),
    output_limit BIGINT NOT NULL CHECK (output_limit > 0),
    provider VARCHAR(32) NOT NULL CHECK (provider = 'docker'),
    container_id VARCHAR(256) NOT NULL CHECK (length(trim(container_id)) > 0),
    exec_id VARCHAR(256) NOT NULL CHECK (length(trim(exec_id)) > 0),
    process_state VARCHAR(16) NOT NULL CHECK (process_state IN ('succeeded', 'failed', 'unknown')),
    exit_code INTEGER CHECK (exit_code IS NULL OR exit_code BETWEEN 0 AND 255),
    started BOOLEAN NOT NULL,
    transport_complete BOOLEAN NOT NULL,
    output_complete BOOLEAN NOT NULL,
    output_generation VARCHAR(128) NOT NULL CHECK (length(trim(output_generation)) > 0),
    candidate_manifest_sha256 VARCHAR(64) NOT NULL DEFAULT '',
    observed_at DATETIME NOT NULL,
    CONSTRAINT uq_craft_web_build_receipt_attempt UNIQUE
        (tenant_id, task_id, workspace_id, run_id, activity_key, request_sha256),
    CHECK ((process_state = 'succeeded' AND exit_code = 0 AND started = 1)
        OR (process_state = 'failed' AND exit_code BETWEEN 1 AND 255 AND started = 1)
        OR process_state = 'unknown'),
    CHECK (candidate_manifest_sha256 = '' OR length(candidate_manifest_sha256) = 64),
    CHECK (output_complete = 0 OR (transport_complete = 1 AND length(candidate_manifest_sha256) = 64 AND length(output_generation) > 0))
);

CREATE TRIGGER trg_craft_web_build_receipt_no_update
BEFORE UPDATE ON craft_web_build_receipts
BEGIN
    SELECT RAISE(ABORT, 'craft web build receipt is immutable');
END;

CREATE TRIGGER trg_craft_web_build_receipt_no_delete
BEFORE DELETE ON craft_web_build_receipts
BEGIN
    SELECT RAISE(ABORT, 'craft web build receipt is immutable');
END;
