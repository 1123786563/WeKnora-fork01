CREATE TEMP TABLE task8_agent_security_down_guard(id INTEGER PRIMARY KEY);
INSERT INTO task8_agent_security_down_guard(id) VALUES (1);
CREATE TEMP TRIGGER refuse_agent_security_down BEFORE UPDATE OF id ON task8_agent_security_down_guard
WHEN EXISTS(SELECT 1 FROM agent_chat_turn_claims) OR EXISTS(SELECT 1 FROM agent_runs WHERE security_agent_id IS NOT NULL) OR EXISTS(SELECT 1 FROM agent_release_revocations) OR EXISTS(SELECT 1 FROM agent_dependency_revocations)
BEGIN SELECT RAISE(ABORT,'agent_security_history_prevents_down'); END;
UPDATE task8_agent_security_down_guard SET id=id WHERE id=1;
DROP TRIGGER refuse_agent_security_down;
DROP TABLE task8_agent_security_down_guard;
DROP TRIGGER IF EXISTS trg_agent_adoption_variant_published_identity_immutable;
DROP TRIGGER IF EXISTS trg_agent_runs_security_pin_complete_update;
DROP TRIGGER IF EXISTS trg_agent_runs_security_pin_complete;
DROP TRIGGER IF EXISTS trg_agent_runs_security_pin_immutable;
DROP TRIGGER IF EXISTS trg_agent_runs_marketplace_pin_required;
DROP INDEX IF EXISTS idx_agent_chat_turn_claims_active;
DROP INDEX IF EXISTS uq_agent_chat_turn_claim_session_user;
DROP INDEX IF EXISTS uq_agent_chat_turn_claim_session_assistant;
DROP INDEX IF EXISTS uq_agent_chat_turn_claim_request;
DROP INDEX IF EXISTS idx_agent_chat_turn_claim_source_state_release;
DROP INDEX IF EXISTS uq_agent_adoption_variant_local_agent;
DROP TABLE IF EXISTS agent_chat_turn_claims;
ALTER TABLE agent_runs DROP COLUMN security_pin_source;
ALTER TABLE agent_runs DROP COLUMN security_release_id;
ALTER TABLE agent_runs DROP COLUMN security_local_agent_version_id;
ALTER TABLE agent_runs DROP COLUMN security_agent_id;
ALTER TABLE agent_dependency_revocations DROP COLUMN run_cancellation_state;
ALTER TABLE agent_release_revocations DROP COLUMN run_cancellation_state;
