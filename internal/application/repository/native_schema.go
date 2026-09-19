package repository

import "fmt"

// NativeSchemaTable is the logical P1.2 manifest.  The SQL migrations are the
// physical source of truth; this manifest prevents later repository work from
// treating the new namespace as an unscoped collection of tables.
type NativeSchemaTable struct {
	Name         string
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
		{"native_agent_tenants", []string{"tenant_id"}, [][]string{{"tenant_id"}}, nil},
		{"native_agent_sessions", []string{"tenant_id", "user_id", "session_id"}, [][]string{{"tenant_id", "user_id", "session_id"}, {"tenant_id", "session_id"}}, nil},
		{"native_agent_runs", []string{"tenant_id", "run_id", "session_id", "user_id"}, [][]string{{"tenant_id", "run_id"}}, [][]string{{"tenant_id", "session_id", "status", "lease_expires_at"}}},
		{"native_agent_inputs", []string{"tenant_id", "run_id", "input_id"}, [][]string{{"tenant_id", "run_id", "input_id"}, {"tenant_id", "run_id", "input_hash"}}, nil},
		{"native_agent_config_bindings", []string{"tenant_id", "run_id", "binding_kind"}, [][]string{{"tenant_id", "run_id", "binding_kind"}}, nil},
		{"native_agent_memory_scopes", []string{"tenant_id", "user_id"}, [][]string{{"tenant_id", "user_id"}}, nil},
		{"native_agent_memory_entries", []string{"tenant_id", "user_id", "memory_id"}, [][]string{{"tenant_id", "user_id", "memory_id"}}, [][]string{{"tenant_id", "user_id", "generation", "tombstoned"}}},
		{"native_agent_attempts", []string{"tenant_id", "run_id", "attempt_id"}, [][]string{{"tenant_id", "run_id", "attempt_id"}}, nil},
		{"native_agent_tool_calls", []string{"tenant_id", "run_id", "attempt_id", "call_id"}, [][]string{{"tenant_id", "run_id", "attempt_id", "call_id"}}, nil},
		{"native_agent_tool_results", []string{"tenant_id", "run_id", "attempt_id", "call_id"}, [][]string{{"tenant_id", "run_id", "attempt_id", "call_id"}}, nil},
		{"native_agent_pending_decisions", []string{"tenant_id", "run_id", "pending_id"}, [][]string{{"tenant_id", "run_id", "pending_id"}}, [][]string{{"tenant_id", "run_id", "status", "created_at"}}},
		{"native_agent_commit_intents", []string{"tenant_id", "run_id", "intent_id"}, [][]string{{"tenant_id", "run_id", "intent_id"}, {"tenant_id", "run_id", "payload_hash"}}, nil},
		{"native_agent_checkpoints", []string{"tenant_id", "run_id", "checkpoint_id"}, [][]string{{"tenant_id", "run_id", "checkpoint_id"}}, [][]string{{"tenant_id", "run_id", "runnable", "created_at"}}},
		{"native_agent_session_events", []string{"tenant_id", "app_name", "user_id", "session_id", "stable_event_id"}, [][]string{{"tenant_id", "app_name", "user_id", "session_id", "stable_event_id"}}, nil},
		{"native_agent_events", []string{"tenant_id", "run_id", "sequence"}, [][]string{{"tenant_id", "run_id", "sequence"}, {"tenant_id", "run_id", "event_id"}}, nil},
		{"native_agent_usage_observations", []string{"tenant_id", "run_id", "attempt_id", "observation_id"}, [][]string{{"tenant_id", "run_id", "attempt_id", "observation_id"}}, nil},
	}
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
		if _, duplicate := seen[table.Name]; duplicate {
			return fmt.Errorf("native schema table %q is declared more than once", table.Name)
		}
		seen[table.Name] = struct{}{}
	}
	return nil
}
