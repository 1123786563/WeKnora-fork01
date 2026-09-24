CREATE TABLE craft_docker_output_operations (
    tenant_id BIGINT NOT NULL CHECK (tenant_id > 0),
    task_id TEXT NOT NULL CHECK (length(btrim(task_id)) > 0),
    run_id TEXT NOT NULL CHECK (length(btrim(run_id)) > 0),
    activity_key TEXT NOT NULL CHECK (length(btrim(activity_key)) > 0),
    container_id TEXT NOT NULL CHECK (length(btrim(container_id)) > 0),
    exec_id TEXT NOT NULL CHECK (length(btrim(exec_id)) > 0),
    max_bytes BIGINT NOT NULL CHECK (max_bytes > 0),
    next_sequence BIGINT NOT NULL DEFAULT 0 CHECK (next_sequence >= 0),
    total_bytes BIGINT NOT NULL DEFAULT 0 CHECK (total_bytes >= 0 AND total_bytes <= max_bytes),
    sealed_at TIMESTAMPTZ,
    truncated BOOLEAN NOT NULL DEFAULT FALSE,
    lock_version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, activity_key),
    UNIQUE (tenant_id, container_id, exec_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id, activity_key)
        REFERENCES craft_charge_start_journal (tenant_id, run_id, activity_key) ON DELETE RESTRICT
);

CREATE TABLE craft_docker_output_chunks (
    tenant_id BIGINT NOT NULL,
    run_id TEXT NOT NULL,
    activity_key TEXT NOT NULL,
    sequence BIGINT NOT NULL CHECK (sequence > 0),
    stream TEXT NOT NULL CHECK (stream IN ('stdout', 'stderr')),
    bytes BYTEA NOT NULL,
    byte_count BIGINT NOT NULL CHECK (byte_count >= 0),
    sha256 CHAR(64) NOT NULL,
    input_byte_count BIGINT NOT NULL CHECK (input_byte_count >= byte_count),
    input_sha256 CHAR(64) NOT NULL,
    input_truncated BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, activity_key, sequence),
    FOREIGN KEY (tenant_id, run_id, activity_key)
        REFERENCES craft_docker_output_operations(tenant_id, run_id, activity_key)
        ON DELETE CASCADE
);

CREATE FUNCTION craft_docker_output_identity_immutable_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.tenant_id, NEW.task_id, NEW.run_id, NEW.activity_key, NEW.container_id, NEW.exec_id, NEW.max_bytes)
       IS DISTINCT FROM ROW(OLD.tenant_id, OLD.task_id, OLD.run_id, OLD.activity_key, OLD.container_id, OLD.exec_id, OLD.max_bytes) THEN
        RAISE EXCEPTION 'immutable Docker output identity';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER craft_docker_output_identity_immutable
BEFORE UPDATE ON craft_docker_output_operations
FOR EACH ROW EXECUTE FUNCTION craft_docker_output_identity_immutable_fn();

CREATE FUNCTION craft_docker_output_scope_validate_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM agent_runs r
        WHERE r.tenant_id = NEW.tenant_id AND r.run_id = NEW.run_id AND r.session_id = NEW.task_id
    ) OR NOT EXISTS (
        SELECT 1 FROM craft_charge_start_journal j
        WHERE j.tenant_id = NEW.tenant_id AND j.run_id = NEW.run_id AND j.activity_key = NEW.activity_key
          AND j.protocol = 'docker_coordinator' AND j.provider = 'docker'
          AND j.container_id = NEW.container_id AND j.exec_id = NEW.exec_id AND j.send_claimed_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'invalid Docker output scope or receipt';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER craft_docker_output_scope_validate
BEFORE INSERT ON craft_docker_output_operations
FOR EACH ROW EXECUTE FUNCTION craft_docker_output_scope_validate_fn();
