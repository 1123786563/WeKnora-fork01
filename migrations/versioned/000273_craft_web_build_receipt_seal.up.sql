-- F08 collector-seal fence: the receipt stays immutable except for exactly
-- one server-side annotation — the collector sealing the candidate manifest
-- digest it produced. The exception admits only: empty seal -> 64-char
-- digest, output_complete may flip false -> true (table CHECKs still bind it
-- to the transport/seal facts), and every other column unchanged.
CREATE OR REPLACE FUNCTION craft_web_build_receipt_no_mutation_fn() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'craft web build receipt is immutable';
    END IF;
    IF OLD.candidate_manifest_sha256 = ''
       AND NEW.candidate_manifest_sha256 <> ''
       AND (NEW.output_complete = OLD.output_complete OR (OLD.output_complete = false AND NEW.output_complete = true))
       AND NEW.id IS NOT DISTINCT FROM OLD.id
       AND NEW.tenant_id IS NOT DISTINCT FROM OLD.tenant_id
       AND NEW.task_id IS NOT DISTINCT FROM OLD.task_id
       AND NEW.session_id IS NOT DISTINCT FROM OLD.session_id
       AND NEW.workspace_id IS NOT DISTINCT FROM OLD.workspace_id
       AND NEW.run_id IS NOT DISTINCT FROM OLD.run_id
       AND NEW.activity_key IS NOT DISTINCT FROM OLD.activity_key
       AND NEW.request_sha256 IS NOT DISTINCT FROM OLD.request_sha256
       AND NEW.command_sha256 IS NOT DISTINCT FROM OLD.command_sha256
       AND NEW.runtime_digest IS NOT DISTINCT FROM OLD.runtime_digest
       AND NEW.toolchain_digest IS NOT DISTINCT FROM OLD.toolchain_digest
       AND NEW.template_version IS NOT DISTINCT FROM OLD.template_version
       AND NEW.template_sha256 IS NOT DISTINCT FROM OLD.template_sha256
       AND NEW.timeout_ms IS NOT DISTINCT FROM OLD.timeout_ms
       AND NEW.output_limit IS NOT DISTINCT FROM OLD.output_limit
       AND NEW.provider IS NOT DISTINCT FROM OLD.provider
       AND NEW.container_id IS NOT DISTINCT FROM OLD.container_id
       AND NEW.exec_id IS NOT DISTINCT FROM OLD.exec_id
       AND NEW.process_state IS NOT DISTINCT FROM OLD.process_state
       AND NEW.exit_code IS NOT DISTINCT FROM OLD.exit_code
       AND NEW.started IS NOT DISTINCT FROM OLD.started
       AND NEW.transport_complete IS NOT DISTINCT FROM OLD.transport_complete
       AND NEW.output_generation IS NOT DISTINCT FROM OLD.output_generation
       AND NEW.observed_at IS NOT DISTINCT FROM OLD.observed_at
    THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'craft web build receipt is immutable';
END;
$$;
