package repository

import "fmt"

const NativeSchemaVersion = 1

// NativeSchemaTable is the logical P1.2 manifest.  The SQL migrations are the
// physical source of truth; this manifest prevents later repository work from
// treating the new namespace as an unscoped collection of tables.
type NativeSchemaTable struct {
	Name         string
	Columns      []string
	ScopeColumns []string
	UniqueKeys   [][]string
	Indexes      [][]string
}

// NativeSchemaManifest describes the smallest durable namespace required by
// the controlled Session/Memory facade.  It deliberately contains no legacy
// Session or Memory table: P1.6 owns read-only archival, and P1.3/P1.4 own
// the facade behavior built on this schema.
func NativeSchemaManifest() []NativeSchemaTable {
	return []NativeSchemaTable{
		nativeTable("native_agent_tenants", []string{"tenant_id"}, []string{"tenant_id"}),
		nativeTable("native_agent_sessions", []string{"tenant_id", "owner_id", "session_id"}, []string{"tenant_id", "owner_id", "session_id"}),
		nativeTable("native_session_state", []string{"tenant_id", "owner_id", "session_id", "state_key", "revision", "state_value"}, []string{"tenant_id", "owner_id", "session_id", "state_key"}),
		nativeTable("native_agent_runs", []string{"tenant_id", "run_id", "owner_id", "session_id", "request_id", "input_hash", "revision", "lease_epoch"}, []string{"tenant_id", "run_id"}),
		nativeTable("native_agent_inputs", []string{"tenant_id", "run_id", "input_id", "input_hash", "payload", "role"}, []string{"tenant_id", "run_id", "input_id"}),
		nativeTable("native_agent_config_bindings", []string{"tenant_id", "run_id", "binding_kind", "schema_version", "sdk_version", "graph_version", "config_hash", "credential_ref", "tool_set_hash"}, []string{"tenant_id", "run_id", "binding_kind"}),
		nativeTable("native_agent_memory_scopes", []string{"tenant_id", "user_id", "generation", "tombstone_generation"}, []string{"tenant_id", "user_id"}),
		nativeTable("native_agent_memory_entries", []string{"tenant_id", "user_id", "memory_id", "generation", "tombstoned"}, []string{"tenant_id", "user_id", "memory_id"}),
		nativeTable("native_memory_jobs", []string{"tenant_id", "subject_id", "job_id", "generation", "through_event_id", "status"}, []string{"tenant_id", "subject_id", "job_id"}),
		nativeTable("native_agent_attempts", []string{"tenant_id", "run_id", "attempt_id", "kind", "logical_call_id", "invocation_id", "attempt_number", "provider_request_id", "lease_epoch"}, []string{"tenant_id", "run_id", "attempt_id"}),
		nativeTable("native_agent_tool_calls", []string{"tenant_id", "run_id", "attempt_id", "call_id", "plan_version", "args_hash"}, []string{"tenant_id", "run_id", "attempt_id", "call_id"}),
		nativeTable("native_agent_tool_plans", []string{"tenant_id", "run_id", "attempt_id", "call_id", "plan_version", "args", "args_hash", "policy", "idempotency_key"}, []string{"tenant_id", "run_id", "attempt_id", "call_id", "plan_version"}),
		nativeTable("native_agent_tool_results", []string{"tenant_id", "run_id", "attempt_id", "call_id", "provider_receipt", "query_anchor", "result_hash", "effect_state", "is_error", "content"}, []string{"tenant_id", "run_id", "attempt_id", "call_id"}),
		nativeTable("native_agent_pending_decisions", []string{"tenant_id", "run_id", "pending_id", "call_id", "plan_version", "args_hash", "wait_kind", "expected_revision", "decision_id"}, []string{"tenant_id", "run_id", "pending_id"}),
		nativeTable("native_agent_commit_intents", []string{"tenant_id", "run_id", "intent_id", "version", "payload", "payload_hash", "lease_epoch", "terminal_status"}, []string{"tenant_id", "run_id", "intent_id"}),
		nativeTable("native_agent_checkpoints", []string{"tenant_id", "run_id", "checkpoint_id", "schema_version", "sdk_version", "graph_version", "namespace", "lineage_id", "request_payload"}, []string{"tenant_id", "run_id", "checkpoint_id"}),
		nativeTable("native_agent_session_events", []string{"tenant_id", "app_name", "user_id", "session_id", "stable_event_id", "payload", "payload_hash", "ordinal"}, []string{"tenant_id", "app_name", "user_id", "session_id", "stable_event_id"}),
		nativeTable("native_agent_events", []string{"tenant_id", "run_id", "sequence", "event_id", "intent_id", "payload", "payload_hash"}, []string{"tenant_id", "run_id", "sequence"}),
		nativeTable("native_agent_usage_observations", []string{"tenant_id", "run_id", "attempt_id", "observation_id", "revision", "provider", "model", "input_tokens", "output_tokens", "payload_hash"}, []string{"tenant_id", "run_id", "attempt_id", "observation_id"}),
	}
}

func nativeTable(name string, columns, key []string) NativeSchemaTable {
	return NativeSchemaTable{Name: name, Columns: columns, ScopeColumns: []string{"tenant_id"}, UniqueKeys: [][]string{key}}
}

// ValidateNativeSchemaManifest catches accidental removal of tenant scoping or
// a stable uniqueness key before a repository starts issuing queries.
func ValidateNativeSchemaManifest() error {
	seen := make(map[string]struct{})
	for _, table := range NativeSchemaManifest() {
		if table.Name == "" || len(table.ScopeColumns) == 0 || table.ScopeColumns[0] != "tenant_id" {
			return fmt.Errorf("native schema table %q must start with tenant scope", table.Name)
		}
		if len(table.UniqueKeys) == 0 {
			return fmt.Errorf("native schema table %q must have a unique key", table.Name)
		}
		if len(table.Columns) == 0 {
			return fmt.Errorf("native schema table %q must declare columns", table.Name)
		}
		if _, duplicate := seen[table.Name]; duplicate {
			return fmt.Errorf("native schema table %q is declared more than once", table.Name)
		}
		seen[table.Name] = struct{}{}
	}
	return nil
}
