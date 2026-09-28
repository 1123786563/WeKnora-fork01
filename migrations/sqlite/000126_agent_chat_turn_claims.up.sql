ALTER TABLE agent_release_revocations ADD COLUMN run_cancellation_state VARCHAR(16) NOT NULL DEFAULT 'complete';
ALTER TABLE agent_dependency_revocations ADD COLUMN run_cancellation_state VARCHAR(16) NOT NULL DEFAULT 'complete';
ALTER TABLE agent_runs ADD COLUMN security_agent_id VARCHAR(36);
ALTER TABLE agent_runs ADD COLUMN security_local_agent_version_id VARCHAR(36);
ALTER TABLE agent_runs ADD COLUMN security_release_id VARCHAR(36);
ALTER TABLE agent_runs ADD COLUMN security_pin_source VARCHAR(32);
CREATE TEMP TRIGGER reject_agent_security_backfill_conflicts BEFORE UPDATE OF id ON tenants
WHEN EXISTS(SELECT 1 FROM agent_adoption_variants WHERE local_agent_id IS NOT NULL AND local_agent_id<>'' GROUP BY tenant_id,local_agent_id HAVING COUNT(*)>1)
 OR EXISTS(SELECT 1 FROM agent_runs r JOIN agent_adoption_variants v ON v.tenant_id=r.tenant_id AND v.local_agent_id=json_extract(r.snapshot,'$.agent_id') WHERE r.status NOT IN ('succeeded','failed','canceled') AND r.created_at < v.published_at)
 OR EXISTS(SELECT 1 FROM agent_adoption_variants v LEFT JOIN agent_versions av ON av.tenant_id=v.tenant_id AND av.id=v.local_agent_version_id AND av.agent_id=v.local_agent_id WHERE v.local_agent_id IS NOT NULL AND v.local_agent_id<>'' AND (v.local_agent_version_id IS NULL OR v.release_id IS NULL OR v.release_id='' OR av.id IS NULL))
BEGIN SELECT RAISE(ABORT,'agent_run_security_backfill_conflict'); END;
UPDATE tenants SET id=id WHERE id=(SELECT MIN(id) FROM tenants);
DROP TRIGGER reject_agent_security_backfill_conflicts;
UPDATE agent_runs SET
 security_agent_id=json_extract(snapshot,'$.agent_id'),
 security_local_agent_version_id=(SELECT v.local_agent_version_id FROM agent_adoption_variants v WHERE v.tenant_id=agent_runs.tenant_id AND v.local_agent_id=json_extract(agent_runs.snapshot,'$.agent_id')),
 security_release_id=(SELECT v.release_id FROM agent_adoption_variants v WHERE v.tenant_id=agent_runs.tenant_id AND v.local_agent_id=json_extract(agent_runs.snapshot,'$.agent_id')),
 security_pin_source='legacy_backfill'
WHERE status NOT IN ('succeeded','failed','canceled') AND EXISTS(SELECT 1 FROM agent_adoption_variants v WHERE v.tenant_id=agent_runs.tenant_id AND v.local_agent_id=json_extract(agent_runs.snapshot,'$.agent_id') AND v.local_agent_version_id IS NOT NULL AND v.release_id IS NOT NULL AND agent_runs.created_at >= v.published_at);
CREATE TABLE agent_chat_turn_claims (
	 id VARCHAR(36) NOT NULL, source_tenant_id INTEGER NOT NULL, session_tenant_id INTEGER NOT NULL,
	 session_id VARCHAR(36) NOT NULL, owner_id VARCHAR(512) NOT NULL, request_id VARCHAR(128) NOT NULL,
	 request_hash VARCHAR(64) NOT NULL, assistant_message_id VARCHAR(36) NOT NULL, user_message_id VARCHAR(36),
	 agent_id VARCHAR(36) NOT NULL, local_agent_version_id VARCHAR(36), release_id VARCHAR(36),
	 state VARCHAR(16) NOT NULL CHECK(state IN ('active','completed','failed','cancelled')), reason TEXT NOT NULL DEFAULT '',
	 lease_owner VARCHAR(128) NOT NULL, lease_expires_at DATETIME NOT NULL, generation BIGINT NOT NULL CHECK(generation>0),
	 created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	 CHECK((local_agent_version_id IS NULL AND release_id IS NULL) OR (local_agent_version_id IS NOT NULL AND release_id IS NOT NULL)),
	PRIMARY KEY(id,source_tenant_id));
CREATE UNIQUE INDEX uq_agent_chat_turn_claim_request ON agent_chat_turn_claims(session_tenant_id,session_id,owner_id,request_id);
CREATE UNIQUE INDEX uq_agent_chat_turn_claim_session_assistant ON agent_chat_turn_claims(session_tenant_id,assistant_message_id);
CREATE UNIQUE INDEX uq_agent_chat_turn_claim_session_user ON agent_chat_turn_claims(session_tenant_id,user_message_id);
CREATE INDEX idx_agent_chat_turn_claims_active ON agent_chat_turn_claims(session_tenant_id,session_id,state,lease_expires_at);
CREATE INDEX idx_agent_chat_turn_claim_source_state_release ON agent_chat_turn_claims(source_tenant_id,state,release_id);
CREATE UNIQUE INDEX uq_agent_adoption_variant_local_agent ON agent_adoption_variants(tenant_id,local_agent_id) WHERE local_agent_id IS NOT NULL AND local_agent_id<>'';
CREATE TRIGGER trg_agent_runs_security_pin_immutable BEFORE UPDATE OF security_agent_id,security_local_agent_version_id,security_release_id,security_pin_source ON agent_runs
WHEN OLD.security_agent_id IS NOT NEW.security_agent_id OR OLD.security_local_agent_version_id IS NOT NEW.security_local_agent_version_id OR OLD.security_release_id IS NOT NEW.security_release_id OR OLD.security_pin_source IS NOT NEW.security_pin_source
BEGIN SELECT RAISE(ABORT,'agent_run_security_pin_immutable'); END;
CREATE TRIGGER trg_agent_runs_security_pin_complete BEFORE INSERT ON agent_runs
WHEN (NEW.security_agent_id IS NULL) <> (NEW.security_local_agent_version_id IS NULL) OR (NEW.security_agent_id IS NULL) <> (NEW.security_release_id IS NULL) OR (NEW.security_agent_id IS NULL) <> (NEW.security_pin_source IS NULL) OR (NEW.security_pin_source IS NOT NULL AND NEW.security_pin_source NOT IN ('admission','legacy_backfill'))
BEGIN SELECT RAISE(ABORT,'agent_run_security_pin_incomplete'); END;
CREATE TRIGGER trg_agent_runs_security_pin_complete_update BEFORE UPDATE OF security_agent_id,security_local_agent_version_id,security_release_id,security_pin_source ON agent_runs
WHEN (NEW.security_agent_id IS NULL) <> (NEW.security_local_agent_version_id IS NULL) OR (NEW.security_agent_id IS NULL) <> (NEW.security_release_id IS NULL) OR (NEW.security_agent_id IS NULL) <> (NEW.security_pin_source IS NULL) OR (NEW.security_pin_source IS NOT NULL AND NEW.security_pin_source NOT IN ('admission','legacy_backfill'))
BEGIN SELECT RAISE(ABORT,'agent_run_security_pin_incomplete'); END;
CREATE TRIGGER trg_agent_runs_marketplace_pin_required BEFORE INSERT ON agent_runs
WHEN EXISTS(SELECT 1 FROM agent_adoption_variants v WHERE v.tenant_id=NEW.tenant_id AND v.local_agent_id=json_extract(NEW.snapshot,'$.agent_id')) AND NEW.security_pin_source IS NULL
BEGIN SELECT RAISE(ABORT,'agent_run_marketplace_security_pin_required'); END;
CREATE TRIGGER trg_agent_adoption_variant_published_identity_immutable BEFORE UPDATE OF local_agent_id,local_agent_version_id,release_id ON agent_adoption_variants
WHEN OLD.published_at IS NOT NULL AND (OLD.local_agent_id IS NOT NEW.local_agent_id OR OLD.local_agent_version_id IS NOT NEW.local_agent_version_id OR OLD.release_id IS NOT NEW.release_id)
BEGIN SELECT RAISE(ABORT,'published_agent_variant_identity_immutable'); END;
