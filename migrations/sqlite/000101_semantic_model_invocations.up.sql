CREATE TABLE semantic_model_invocation_runs (
  tenant_id TEXT NOT NULL CHECK(tenant_id GLOB '[0-9]*' AND tenant_id NOT GLOB '*[^0-9]*'), run_id TEXT NOT NULL, kb_id TEXT NOT NULL, scope_hash TEXT NOT NULL,
  policy_version TEXT NOT NULL CHECK(policy_version GLOB '[0-9]*' AND policy_version NOT GLOB '*[^0-9]*' AND policy_version NOT GLOB '0*'), model_id TEXT NOT NULL, funding TEXT NOT NULL CHECK(funding IN ('platform','byok')), price_version TEXT NOT NULL,
  max_input_tokens_per_call INTEGER NOT NULL CHECK(max_input_tokens_per_call > 0), max_output_tokens_per_call INTEGER NOT NULL CHECK(max_output_tokens_per_call > 0), per_call_upper_micro INTEGER NOT NULL CHECK(per_call_upper_micro >= 0),
  max_calls_per_task INTEGER NOT NULL CHECK(max_calls_per_task > 0), max_input_tokens_per_task INTEGER NOT NULL CHECK(max_input_tokens_per_task > 0), max_output_tokens_per_task INTEGER NOT NULL CHECK(max_output_tokens_per_task > 0), expires_at DATETIME NOT NULL, PRIMARY KEY (tenant_id, run_id)
);
CREATE TABLE semantic_model_invocations (
  tenant_id TEXT NOT NULL CHECK(tenant_id GLOB '[0-9]*' AND tenant_id NOT GLOB '*[^0-9]*'), run_id TEXT NOT NULL, call_id TEXT NOT NULL, request_hash TEXT NOT NULL,
  state TEXT NOT NULL CHECK(state IN ('claimed','dispatched','completed','failed_before_dispatch','unknown')), reserved_input_tokens INTEGER NOT NULL CHECK(reserved_input_tokens >= 0), reserved_output_tokens INTEGER NOT NULL CHECK(reserved_output_tokens >= 0),
  result BLOB NULL, input_tokens INTEGER NULL CHECK(input_tokens >= 0), output_tokens INTEGER NULL CHECK(output_tokens >= 0), created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (tenant_id, call_id), FOREIGN KEY (tenant_id, run_id) REFERENCES semantic_model_invocation_runs(tenant_id, run_id)
);
CREATE INDEX idx_semantic_model_invocations_run ON semantic_model_invocations(tenant_id, run_id);
