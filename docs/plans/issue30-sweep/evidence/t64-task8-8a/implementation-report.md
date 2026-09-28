# Task 8A Report — Shared transactional Variant/Release predicate

## Checkpoint

- Worktree: `/Users/wuyongjun/.codex/worktrees/t64-task8-8a/WeKnora-fork01`
- Branch: `codex/issue30-t64-task8-8a`
- BASE: `283022dd30ba9fa5f73a238d1d6c5e6a5d4fa1e9`
- Initial HEAD: `283022dd30ba9fa5f73a238d1d6c5e6a5d4fa1e9`
- Final HEAD / commit: `ffd4bc10f7aaf226b7f3272b91b41212ddde8ac8`
- Commit authorized by the controller; only the two owned files are included.

## Implementation and diff

Implemented `checkLocalAgentReleaseAdmissionTx` with the exact downstream signature. It loads all Variant mappings in `(tenant_id, local_agent_id)` scope ordered by ID and requests row locks before inspecting state. It accepts only one published mapping with an exact, tenant- and Agent-bound immutable Version, then calls `checkReleaseAdmissionTx` on the same transaction. A live tenant-scoped CustomAgent with no Variant lineage is the only non-Marketplace classification; a supplied Version, missing Agent, stale/draft/retired mapping, duplicate mapping, mismatched Version, or failed Release/dependency predicate fails closed.

Added table-driven coverage for active publication, confirmed ordinary Agent, missing/mismatched Version, retired/draft mappings, Release revocation, exact dependency tuple revocation, different dependency version, same version with another digest, and duplicate mappings.

Changed files:

- `internal/application/repository/agent_security_guard.go` — 52 added lines; shared transaction predicate and GORM locking clause.
- `internal/application/repository/agent_security_guard_test.go` — new 94-line focused table-driven test.

No other tracked or untracked implementation files were changed.

## RED → GREEN evidence

RED command:

```text
go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$' -count=1
```

Output:

```text
# github.com/Tencent/WeKnora/internal/application/repository [github.com/Tencent/WeKnora/internal/application/repository.test]
internal/application/repository/agent_security_guard_test.go:63:27: undefined: checkLocalAgentReleaseAdmissionTx
FAIL	github.com/Tencent/WeKnora/internal/application/repository [build failed]
FAIL
```

The first post-implementation run reached test setup but failed before exercising the predicate because the fixture omitted release lineage required by SQLite foreign keys. The observed failure was `FOREIGN KEY constraint failed` while inserting `agent_releases` in each adopted case. I completed the fixture with the required AgentVersion, Listing, Submission and Adoption records; the focused test then passed as recorded below.

After implementation, the exact required focused command was run:

```text
go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$' -count=1
```

Output:

```text
ok  	github.com/Tencent/WeKnora/internal/application/repository	10.044s
```

The exact required decisive-write serialization command was run:

```text
go test ./internal/application/repository -run '^TestTenantSecurityGuardSerializesDecisiveWriteFamilies$' -count=1
```

Output:

```text
ok  	github.com/Tencent/WeKnora/internal/application/repository	7.026s
```

Formatting command run before the focused checks:

```text
gofmt -w internal/application/repository/agent_security_guard.go internal/application/repository/agent_security_guard_test.go
```

Output: no output; exit status 0.

Commit command:

```text
git add internal/application/repository/agent_security_guard.go internal/application/repository/agent_security_guard_test.go && git commit -m "feat: add transactional local agent release admission guard"
```

Output:

```text
[codex/issue30-t64-task8-8a ffd4bc10f] feat: add transactional local agent release admission guard
 2 files changed, 146 insertions(+)
 create mode 100644 internal/application/repository/agent_security_guard_test.go
```

The exact required whitespace check was run:

```text
git diff --check
```

Output: no output; exit status 0.

No broader suite was run, per checkpoint instructions.

## Self-review

- Tenant boundary is part of every Variant and Version lookup; ordinary Agent confirmation is also tenant scoped and excludes soft-deleted `CustomAgent` records through GORM's model scope.
- Variant rows are ordered by stable ID and locked before reading their lifecycle and identity fields. Version is checked only after the Variant mapping, preserving Variant-before-Version lock order.
- The helper neither starts/commits a transaction nor reads a service verdict. Every database read uses the supplied `tx`.
- It does not resolve by display name, listing pointer, or current catalog recommendation. Release identity comes directly from the fixed Variant.
- Exact Release and dependency revocation behavior remains delegated to the existing same-transaction `checkReleaseAdmissionTx`.
- Test cases passed against the repository's migrated SQLite schema with real FK fixture rows.

## Assumptions and remaining risks

- “Confirmed non-Marketplace Agent” is represented by one live `custom_agents` row in the source tenant and no Variant lineage for that local Agent ID. Any lineage row, even if stale or retired, prevents fallback to ordinary-Agent behavior.
- PostgreSQL row-lock execution was not run in this focused checkpoint; the requested serialization suite and new tests use the repository's SQLite fixture path. GORM emits `FOR UPDATE` for the matching Variant/Version queries on dialects that support it.
