CREATE TABLE craft_docker_output_operations (
    tenant_id INTEGER NOT NULL CHECK (tenant_id > 0),
    task_id TEXT NOT NULL CHECK (length(trim(task_id)) > 0),
    run_id TEXT NOT NULL CHECK (length(trim(run_id)) > 0),
    activity_key TEXT NOT NULL CHECK (length(trim(activity_key)) > 0),
    container_id TEXT NOT NULL CHECK (length(trim(container_id)) > 0),
    exec_id TEXT NOT NULL CHECK (length(trim(exec_id)) > 0),
    max_bytes INTEGER NOT NULL CHECK (max_bytes > 0),
    next_sequence INTEGER NOT NULL DEFAULT 0 CHECK (next_sequence >= 0),
    total_bytes INTEGER NOT NULL DEFAULT 0 CHECK (total_bytes >= 0 AND total_bytes <= max_bytes),
    sealed_at DATETIME,
    truncated BOOLEAN NOT NULL DEFAULT FALSE,
    lock_version INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, activity_key),
    UNIQUE (tenant_id, container_id, exec_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id, activity_key)
        REFERENCES craft_charge_start_journal (tenant_id, run_id, activity_key) ON DELETE RESTRICT
);

CREATE TABLE craft_docker_output_chunks (
    tenant_id INTEGER NOT NULL,
    run_id TEXT NOT NULL,
    activity_key TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence > 0),
    stream TEXT NOT NULL CHECK (stream IN ('stdout', 'stderr')),
    bytes BLOB NOT NULL,
    byte_count INTEGER NOT NULL CHECK (byte_count >= 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    input_byte_count INTEGER NOT NULL CHECK (input_byte_count >= byte_count),
    input_sha256 TEXT NOT NULL CHECK (length(input_sha256) = 64),
    input_truncated BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, activity_key, sequence),
    FOREIGN KEY (tenant_id, run_id, activity_key)
        REFERENCES craft_docker_output_operations(tenant_id, run_id, activity_key)
        ON DELETE CASCADE
);

CREATE TRIGGER craft_docker_output_identity_immutable
BEFORE UPDATE OF tenant_id, task_id, run_id, activity_key, container_id, exec_id, max_bytes
ON craft_docker_output_operations
WHEN NEW.tenant_id != OLD.tenant_id OR NEW.task_id != OLD.task_id OR NEW.run_id != OLD.run_id OR
     NEW.activity_key != OLD.activity_key OR NEW.container_id != OLD.container_id OR
     NEW.exec_id != OLD.exec_id OR NEW.max_bytes != OLD.max_bytes
BEGIN
    SELECT RAISE(ABORT, 'immutable Docker output identity');
END;

CREATE TRIGGER craft_docker_output_scope_validate
BEFORE INSERT ON craft_docker_output_operations
WHEN NOT EXISTS (
        SELECT 1 FROM agent_runs r
        WHERE r.tenant_id = NEW.tenant_id AND r.run_id = NEW.run_id AND r.session_id = NEW.task_id
     ) OR NOT EXISTS (
        SELECT 1 FROM craft_charge_start_journal j
        WHERE j.tenant_id = NEW.tenant_id AND j.run_id = NEW.run_id AND j.activity_key = NEW.activity_key
          AND j.protocol = 'docker_coordinator' AND j.provider = 'docker'
          AND j.container_id = NEW.container_id AND j.exec_id = NEW.exec_id AND j.send_claimed_at IS NOT NULL
     )
BEGIN
    SELECT RAISE(ABORT, 'invalid Docker output scope or receipt');
END;
