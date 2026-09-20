CREATE TABLE semantic_model_policies (
  tenant_id TEXT NOT NULL CHECK(tenant_id GLOB '[0-9]*' AND tenant_id NOT GLOB '*[^0-9]*' AND (tenant_id = '0' OR tenant_id NOT GLOB '0*') AND (length(tenant_id) < 20 OR (length(tenant_id) = 20 AND tenant_id <= '18446744073709551615'))),
  kb_id TEXT NOT NULL, model_calls_enabled INTEGER NOT NULL DEFAULT 0 CHECK(model_calls_enabled IN (0,1)), model_id TEXT NOT NULL,
  funding TEXT NOT NULL CHECK(funding IN ('platform','byok')), price_version TEXT NOT NULL,
  max_input_tokens_per_call INTEGER NOT NULL CHECK(max_input_tokens_per_call > 0), max_output_tokens_per_call INTEGER NOT NULL CHECK(max_output_tokens_per_call > 0),
  max_calls_per_task INTEGER NOT NULL CHECK(max_calls_per_task > 0), max_input_tokens_per_task INTEGER NOT NULL CHECK(max_input_tokens_per_task > 0), max_output_tokens_per_task INTEGER NOT NULL CHECK(max_output_tokens_per_task > 0),
  per_call_upper_micro INTEGER NOT NULL CHECK(per_call_upper_micro >= 0), task_upper_micro INTEGER NULL CHECK(task_upper_micro > 0),
  policy_version TEXT NOT NULL CHECK(policy_version GLOB '[0-9]*' AND policy_version NOT GLOB '*[^0-9]*' AND policy_version NOT GLOB '0*' AND (length(policy_version) < 20 OR (length(policy_version) = 20 AND policy_version <= '18446744073709551615'))),
  updated_by TEXT NOT NULL, updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP, PRIMARY KEY (tenant_id, kb_id)
);
CREATE TABLE semantic_model_policy_revisions (
  tenant_id TEXT NOT NULL CHECK(tenant_id GLOB '[0-9]*' AND tenant_id NOT GLOB '*[^0-9]*' AND (tenant_id = '0' OR tenant_id NOT GLOB '0*') AND (length(tenant_id) < 20 OR (length(tenant_id) = 20 AND tenant_id <= '18446744073709551615'))),
  kb_id TEXT NOT NULL, model_calls_enabled INTEGER NOT NULL CHECK(model_calls_enabled IN (0,1)), model_id TEXT NOT NULL,
  funding TEXT NOT NULL CHECK(funding IN ('platform','byok')), price_version TEXT NOT NULL,
  max_input_tokens_per_call INTEGER NOT NULL CHECK(max_input_tokens_per_call > 0), max_output_tokens_per_call INTEGER NOT NULL CHECK(max_output_tokens_per_call > 0),
  max_calls_per_task INTEGER NOT NULL CHECK(max_calls_per_task > 0), max_input_tokens_per_task INTEGER NOT NULL CHECK(max_input_tokens_per_task > 0), max_output_tokens_per_task INTEGER NOT NULL CHECK(max_output_tokens_per_task > 0),
  per_call_upper_micro INTEGER NOT NULL CHECK(per_call_upper_micro >= 0), task_upper_micro INTEGER NULL CHECK(task_upper_micro > 0),
  policy_version TEXT NOT NULL CHECK(policy_version GLOB '[0-9]*' AND policy_version NOT GLOB '*[^0-9]*' AND policy_version NOT GLOB '0*' AND (length(policy_version) < 20 OR (length(policy_version) = 20 AND policy_version <= '18446744073709551615'))),
  updated_by TEXT NOT NULL, updated_at DATETIME NOT NULL, PRIMARY KEY (tenant_id, kb_id, policy_version)
);
