PRAGMA foreign_keys=OFF;
CREATE TABLE agent_run_decisions_new (tenant_id INTEGER NOT NULL, run_id VARCHAR(64) NOT NULL, decision_id VARCHAR(255) NOT NULL, pending_id VARCHAR(255) NOT NULL, tool_call_id VARCHAR(255), expected_revision BIGINT NOT NULL, actor_id VARCHAR(512) NOT NULL, action VARCHAR(32) NOT NULL, result TEXT, reason TEXT NOT NULL DEFAULT '', applied BOOLEAN NOT NULL DEFAULT 0, created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, applied_at DATETIME, args_hash VARCHAR(64) NOT NULL DEFAULT '', resource_ref VARCHAR(1024) NOT NULL DEFAULT '', PRIMARY KEY (tenant_id, run_id, decision_id), FOREIGN KEY (tenant_id, run_id) REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE);
INSERT INTO agent_run_decisions_new SELECT tenant_id,run_id,decision_id,pending_id,tool_call_id,expected_revision,actor_id,action,result,reason,applied,created_at,applied_at,args_hash,resource_ref FROM agent_run_decisions;
DROP TABLE agent_run_decisions;
ALTER TABLE agent_run_decisions_new RENAME TO agent_run_decisions;
CREATE UNIQUE INDEX uq_agent_run_decisions_applied_pending ON agent_run_decisions (tenant_id, run_id, pending_id) WHERE applied = 1;
PRAGMA foreign_keys=ON;
