# P1.2 native-agent schema manifest

Migration `000162_native_agent_schema` (PostgreSQL) and
`000083_native_agent_schema` (SQLite) establish the isolated `native_agent_*`
namespace.  They create no copy of legacy `sessions`, old Memory, messages, or
agent-run records.

Migration `000166_native_user_state` (PostgreSQL) and
`000087_native_user_state` (SQLite) add the independent user-state boundary
after that namespace is deployed. `owner_id` is the frozen SessionOwnerID, so
it supports account users, tenant API keys, API external users, and embed
sessions without requiring a synthetic native session or a `users` row.

| Logical record | Physical table | Authority and minimum constraint |
| --- | --- | --- |
| Tenant admission scope | `native_agent_tenants` | one row per business tenant |
| Session scope and state | `native_agent_sessions`, `native_session_state` | tenant/owner/session/state-key revision CAS |
| User state | `native_user_state` | admitted-tenant foreign key and scoped `(tenant,owner,state-key)` revision CAS for a frozen owner identity; no native session is required |
| Run and lease fence | `native_agent_runs` | scoped request/input idempotency, owner/session binding, revision and non-negative epoch |
| Input and config snapshot | `native_agent_inputs`, `native_agent_config_bindings` | immutable scoped hashes |
| Memory governance | `native_agent_memory_scopes`, `native_agent_memory_entries`, `native_memory_jobs` | generation/tombstone CAS and delayed-job generation/through-event fence; new jobs retain their SessionKey `(app,user,session)` for recovery (nullable for pre-P1.4 jobs) |
| Attempts and tools | `native_agent_attempts`, `native_agent_tool_calls`, `native_agent_tool_plans`, `native_agent_tool_results` | attempt/call/plan scope, provider identity and immutable result receipt/hash |
| Pending decisions | `native_agent_pending_decisions` | scoped revision and status index |
| Commit/barrier state | `native_agent_commit_intents`, `native_agent_checkpoints` | intent payload hash, epoch, non-runnable checkpoint default |
| Session receipt | `native_agent_session_events` | stable event identity `(tenant,app,user,session,event)` plus immutable payload hash |
| Business event log | `native_agent_events` | positive `(tenant,run,sequence)` and stable event identity |
| Usage receipt | `native_agent_usage_observations` | `(tenant,run,attempt,observation)` and revision |

All foreign keys are `RESTRICT`: a native row cannot be silently removed by a
legacy cleanup path.  The migration down scripts refuse rollback when any
native table contains data, and only drop a completely empty namespace.

`internal/application/repository/native_schema.go` exposes the matching
logical manifest and checks that every declared record begins with tenant
scope and carries a uniqueness boundary.  P1.3, P1.4 and P1.5 own the runtime
conflict/CAS/barrier behavior; P1.2 only makes those later operations
representable by both active dialects.
