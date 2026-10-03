-- F08 collector-seal fence: the receipt stays immutable except for exactly
-- one server-side annotation — the collector sealing the candidate manifest
-- digest it produced. The exception admits only: empty seal -> 64-char
-- digest, OutputComplete may flip 0 -> 1 (table CHECKs still bind it to the
-- transport/seal facts), and every other column byte-identical.
DROP TRIGGER IF EXISTS trg_craft_web_build_receipt_no_update;
CREATE TRIGGER trg_craft_web_build_receipt_no_update
BEFORE UPDATE ON craft_web_build_receipts
WHEN NOT (
    OLD.candidate_manifest_sha256 = ''
    AND NEW.candidate_manifest_sha256 <> ''
    AND (NEW.output_complete = OLD.output_complete OR (OLD.output_complete = 0 AND NEW.output_complete = 1))
    AND NEW.id = OLD.id
    AND NEW.tenant_id = OLD.tenant_id
    AND NEW.task_id = OLD.task_id
    AND NEW.session_id = OLD.session_id
    AND NEW.workspace_id = OLD.workspace_id
    AND NEW.run_id = OLD.run_id
    AND NEW.activity_key = OLD.activity_key
    AND NEW.request_sha256 = OLD.request_sha256
    AND NEW.command_sha256 = OLD.command_sha256
    AND NEW.runtime_digest = OLD.runtime_digest
    AND NEW.toolchain_digest = OLD.toolchain_digest
    AND NEW.template_version = OLD.template_version
    AND NEW.template_sha256 = OLD.template_sha256
    AND NEW.timeout_ms = OLD.timeout_ms
    AND NEW.output_limit = OLD.output_limit
    AND NEW.provider = OLD.provider
    AND NEW.container_id = OLD.container_id
    AND NEW.exec_id = OLD.exec_id
    AND NEW.process_state = OLD.process_state
    AND NEW.exit_code IS OLD.exit_code
    AND NEW.started = OLD.started
    AND NEW.transport_complete = OLD.transport_complete
    AND NEW.output_generation = OLD.output_generation
    AND NEW.observed_at = OLD.observed_at
)
BEGIN
    SELECT RAISE(ABORT, 'craft web build receipt is immutable');
END;
