CREATE TABLE craft_docker_normal_inputs (
    tenant_id INTEGER NOT NULL CHECK (tenant_id > 0),
    task_id TEXT NOT NULL CHECK (length(trim(task_id)) > 0),
    run_id TEXT NOT NULL CHECK (length(trim(run_id)) > 0),
    activity_key TEXT NOT NULL CHECK (length(trim(activity_key)) > 0),
    request_ciphertext TEXT NOT NULL CHECK (length(request_ciphertext) > 0 AND length(request_ciphertext) <= 2796247),
    request_sha256 TEXT NOT NULL CHECK (length(request_sha256) = 64),
    stdin_enabled BOOLEAN NOT NULL,
    stdin_byte_count INTEGER NOT NULL CHECK (stdin_byte_count >= 0),
    stdin_sha256 TEXT NOT NULL CHECK (length(stdin_sha256) = 64),
    timeout_ms INTEGER NOT NULL CHECK (timeout_ms > 0),
    output_limit INTEGER NOT NULL CHECK (output_limit > 0),
    output_policy TEXT NOT NULL CHECK (length(trim(output_policy)) > 0),
    provider TEXT,
    container_id TEXT,
    exec_id TEXT,
    receipt_stdin_enabled BOOLEAN,
    receipt_stdin_byte_count INTEGER,
    receipt_stdin_sha256 TEXT,
    receipt_timeout_ms INTEGER,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, activity_key),
    UNIQUE (tenant_id, container_id, exec_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id, activity_key)
        REFERENCES craft_charge_start_journal (tenant_id, run_id, activity_key) ON DELETE RESTRICT,
    CHECK ((provider IS NULL AND container_id IS NULL AND exec_id IS NULL AND receipt_stdin_enabled IS NULL AND receipt_stdin_byte_count IS NULL AND receipt_stdin_sha256 IS NULL AND receipt_timeout_ms IS NULL)
        OR (provider = 'docker' AND length(trim(container_id)) > 0 AND length(trim(exec_id)) > 0 AND receipt_stdin_enabled IS NOT NULL AND receipt_stdin_byte_count >= 0 AND length(receipt_stdin_sha256) = 64 AND receipt_timeout_ms > 0
            AND receipt_stdin_enabled = stdin_enabled AND receipt_stdin_byte_count = stdin_byte_count AND receipt_stdin_sha256 = stdin_sha256 AND receipt_timeout_ms = timeout_ms))
);

CREATE TRIGGER craft_docker_normal_input_scope_validate
BEFORE INSERT ON craft_docker_normal_inputs
WHEN NOT EXISTS (SELECT 1 FROM agent_runs r WHERE r.tenant_id=NEW.tenant_id AND r.run_id=NEW.run_id AND r.session_id=NEW.task_id)
 OR NOT EXISTS (SELECT 1 FROM craft_charge_start_journal j WHERE j.tenant_id=NEW.tenant_id AND j.run_id=NEW.run_id AND j.activity_key=NEW.activity_key AND j.protocol='docker_coordinator' AND j.state='intent' AND j.provider IS NULL AND j.send_claimed_at IS NULL)
BEGIN SELECT RAISE(ABORT, 'invalid Docker normal input scope'); END;

CREATE TRIGGER craft_docker_normal_input_immutable
BEFORE UPDATE ON craft_docker_normal_inputs
WHEN NEW.tenant_id != OLD.tenant_id OR NEW.task_id != OLD.task_id OR NEW.run_id != OLD.run_id OR NEW.activity_key != OLD.activity_key OR
 NEW.request_ciphertext != OLD.request_ciphertext OR NEW.request_sha256 != OLD.request_sha256 OR NEW.stdin_enabled != OLD.stdin_enabled OR
 NEW.stdin_byte_count != OLD.stdin_byte_count OR NEW.stdin_sha256 != OLD.stdin_sha256 OR NEW.timeout_ms != OLD.timeout_ms OR
 NEW.output_limit != OLD.output_limit OR NEW.output_policy != OLD.output_policy OR
 (OLD.provider IS NOT NULL AND (NEW.provider IS NOT OLD.provider OR NEW.container_id IS NOT OLD.container_id OR NEW.exec_id IS NOT OLD.exec_id OR
 NEW.receipt_stdin_enabled IS NOT OLD.receipt_stdin_enabled OR NEW.receipt_stdin_byte_count IS NOT OLD.receipt_stdin_byte_count OR
 NEW.receipt_stdin_sha256 IS NOT OLD.receipt_stdin_sha256 OR NEW.receipt_timeout_ms IS NOT OLD.receipt_timeout_ms))
BEGIN SELECT RAISE(ABORT, 'immutable Docker normal input or receipt'); END;

CREATE TRIGGER craft_docker_normal_input_receipt_validate
BEFORE UPDATE OF provider, container_id, exec_id, receipt_stdin_enabled, receipt_stdin_byte_count, receipt_stdin_sha256, receipt_timeout_ms
ON craft_docker_normal_inputs
WHEN NEW.provider IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM craft_charge_start_journal j
    WHERE j.tenant_id=NEW.tenant_id AND j.run_id=NEW.run_id AND j.activity_key=NEW.activity_key
      AND j.protocol='docker_coordinator' AND j.state='intent' AND j.send_claimed_at IS NULL
      AND j.provider=NEW.provider AND j.container_id=NEW.container_id AND j.exec_id=NEW.exec_id
)
BEGIN SELECT RAISE(ABORT, 'normal Docker receipt does not match unclaimed journal receipt'); END;
