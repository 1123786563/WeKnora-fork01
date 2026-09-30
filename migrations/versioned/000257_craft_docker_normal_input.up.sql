CREATE TABLE craft_docker_normal_inputs (
    tenant_id BIGINT NOT NULL CHECK (tenant_id > 0),
    task_id TEXT NOT NULL CHECK (length(btrim(task_id)) > 0),
    run_id TEXT NOT NULL CHECK (length(btrim(run_id)) > 0),
    activity_key TEXT NOT NULL CHECK (length(btrim(activity_key)) > 0),
    request_ciphertext TEXT NOT NULL CHECK (length(request_ciphertext) > 0 AND length(request_ciphertext) <= 2796247),
    request_sha256 CHAR(64) NOT NULL,
    stdin_enabled BOOLEAN NOT NULL,
    stdin_byte_count BIGINT NOT NULL CHECK (stdin_byte_count >= 0),
    stdin_sha256 CHAR(64) NOT NULL,
    timeout_ms BIGINT NOT NULL CHECK (timeout_ms > 0),
    output_limit BIGINT NOT NULL CHECK (output_limit > 0),
    output_policy TEXT NOT NULL CHECK (length(btrim(output_policy)) > 0),
    provider TEXT,
    container_id TEXT,
    exec_id TEXT,
    receipt_stdin_enabled BOOLEAN,
    receipt_stdin_byte_count BIGINT,
    receipt_stdin_sha256 CHAR(64),
    receipt_timeout_ms BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, run_id, activity_key),
    UNIQUE (tenant_id, container_id, exec_id),
    FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE RESTRICT,
    FOREIGN KEY (tenant_id, run_id, activity_key) REFERENCES craft_charge_start_journal (tenant_id, run_id, activity_key) ON DELETE RESTRICT,
    CHECK ((provider IS NULL AND container_id IS NULL AND exec_id IS NULL AND receipt_stdin_enabled IS NULL AND receipt_stdin_byte_count IS NULL AND receipt_stdin_sha256 IS NULL AND receipt_timeout_ms IS NULL)
        OR (provider='docker' AND length(btrim(container_id))>0 AND length(btrim(exec_id))>0 AND receipt_stdin_enabled IS NOT NULL AND receipt_stdin_byte_count>=0 AND length(receipt_stdin_sha256)=64 AND receipt_timeout_ms>0
            AND receipt_stdin_enabled=stdin_enabled AND receipt_stdin_byte_count=stdin_byte_count AND receipt_stdin_sha256=stdin_sha256 AND receipt_timeout_ms=timeout_ms))
);

CREATE FUNCTION craft_docker_normal_input_scope_validate_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS (SELECT 1 FROM agent_runs r WHERE r.tenant_id=NEW.tenant_id AND r.run_id=NEW.run_id AND r.session_id=NEW.task_id)
 OR NOT EXISTS (SELECT 1 FROM craft_charge_start_journal j WHERE j.tenant_id=NEW.tenant_id AND j.run_id=NEW.run_id AND j.activity_key=NEW.activity_key AND j.protocol='docker_coordinator' AND j.state='intent' AND j.provider IS NULL AND j.send_claimed_at IS NULL) THEN
  RAISE EXCEPTION 'invalid Docker normal input scope';
 END IF;
 RETURN NEW;
END; $$;
CREATE TRIGGER craft_docker_normal_input_scope_validate BEFORE INSERT ON craft_docker_normal_inputs FOR EACH ROW EXECUTE FUNCTION craft_docker_normal_input_scope_validate_fn();

CREATE FUNCTION craft_docker_normal_input_immutable_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ROW(NEW.tenant_id,NEW.task_id,NEW.run_id,NEW.activity_key,NEW.request_ciphertext,NEW.request_sha256,NEW.stdin_enabled,NEW.stdin_byte_count,NEW.stdin_sha256,NEW.timeout_ms,NEW.output_limit,NEW.output_policy)
    IS DISTINCT FROM ROW(OLD.tenant_id,OLD.task_id,OLD.run_id,OLD.activity_key,OLD.request_ciphertext,OLD.request_sha256,OLD.stdin_enabled,OLD.stdin_byte_count,OLD.stdin_sha256,OLD.timeout_ms,OLD.output_limit,OLD.output_policy)
 OR (OLD.provider IS NOT NULL AND ROW(NEW.provider,NEW.container_id,NEW.exec_id,NEW.receipt_stdin_enabled,NEW.receipt_stdin_byte_count,NEW.receipt_stdin_sha256,NEW.receipt_timeout_ms)
    IS DISTINCT FROM ROW(OLD.provider,OLD.container_id,OLD.exec_id,OLD.receipt_stdin_enabled,OLD.receipt_stdin_byte_count,OLD.receipt_stdin_sha256,OLD.receipt_timeout_ms)) THEN
  RAISE EXCEPTION 'immutable Docker normal input or receipt';
 END IF;
 RETURN NEW;
END; $$;
CREATE TRIGGER craft_docker_normal_input_immutable BEFORE UPDATE ON craft_docker_normal_inputs FOR EACH ROW EXECUTE FUNCTION craft_docker_normal_input_immutable_fn();

CREATE FUNCTION craft_docker_normal_input_receipt_validate_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.provider IS NOT NULL AND NOT EXISTS (
    SELECT 1 FROM craft_charge_start_journal j WHERE j.tenant_id=NEW.tenant_id AND j.run_id=NEW.run_id AND j.activity_key=NEW.activity_key
      AND j.protocol='docker_coordinator' AND j.state='intent' AND j.send_claimed_at IS NULL
      AND j.provider=NEW.provider AND j.container_id=NEW.container_id AND j.exec_id=NEW.exec_id
 ) THEN RAISE EXCEPTION 'normal Docker receipt does not match unclaimed journal receipt'; END IF;
 RETURN NEW;
END; $$;
CREATE TRIGGER craft_docker_normal_input_receipt_validate BEFORE UPDATE ON craft_docker_normal_inputs FOR EACH ROW EXECUTE FUNCTION craft_docker_normal_input_receipt_validate_fn();
