# Task 8A Fix Round 1 Report

## Checkpoint

- BASE: `ffd4bc10f7aaf226b7f3272b91b41212ddde8ac8`
- Final HEAD / commit: `8090adb3c46431f3c706dca3d557a928db6baa04`
- Worktree: `/Users/wuyongjun/.codex/worktrees/t64-task8-8a/WeKnora-fork01`
- Ownership respected: only `internal/application/repository/agent_security_guard.go` and `internal/application/repository/agent_security_guard_test.go` changed.

## Findings addressed

- **T64-8A-R1-1:** The adopted path previously validated Variant, Version and Release without confirming that the tenant's local CustomAgent remained live. The helper now locks the tenant-scoped CustomAgent after Variant rows and before Version rows. GORM's CustomAgent model scope filters soft-deleted rows; absence returns `ErrAgentSecurityReleaseUnresolvable`. This check applies to both adopted and ordinary Agent paths. Composite `(id, tenant_id)` primary key bounds the live identity to at most one row.
- **T64-8A-R1-2:** Every failing mapping/Agent case in the table asserts `ErrAgentSecurityReleaseUnresolvable`; Release and exact dependency revocations assert `ErrAgentSecurityReleaseBlocked`.

New cases preserve valid Variant, Version and Release fixtures while removing the local Agent row or soft-deleting it.

## TDD and verification

Formatting command:

```text
gofmt -w internal/application/repository/agent_security_guard_test.go
```

Output: no output; exit status 0.

The first RED command had a test-authoring type error (`wantErr` had been changed from bool to error but the ordinary case still set `false`):

```text
go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$' -count=1
```

Output:

```text
# github.com/Tencent/WeKnora/internal/application/repository [github.com/Tencent/WeKnora/internal/application/repository.test]
internal/application/repository/agent_security_guard_test.go:28:61: cannot use false (constant of type bool) as error value in struct literal: bool does not implement error (missing method Error)
FAIL	github.com/Tencent/WeKnora/internal/application/repository [build failed]
FAIL
```

After correcting the test literal, RED command:

```text
go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$' -count=1
```

Output showed both expected regressions before the implementation fix:

```text
--- FAIL: TestCheckLocalAgentReleaseAdmissionTx (10.16s)
    --- FAIL: TestCheckLocalAgentReleaseAdmissionTx/missing_local_agent_fails_closed (0.67s)
        agent_security_guard_test.go:73:
            Error: Expected error with "release is not resolvable in this tenant" in chain but got nil.
    --- FAIL: TestCheckLocalAgentReleaseAdmissionTx/soft_deleted_local_agent_fails_closed (0.65s)
        agent_security_guard_test.go:73:
            Error: Expected error with "release is not resolvable in this tenant" in chain but got nil.
FAIL
FAIL	github.com/Tencent/WeKnora/internal/application/repository	12.326s
FAIL
```

After implementation and formatting, exact focused GREEN command:

```text
gofmt -w internal/application/repository/agent_security_guard.go internal/application/repository/agent_security_guard_test.go && go test ./internal/application/repository -run '^TestCheckLocalAgentReleaseAdmissionTx$' -count=1
```

Output:

```text
ok   github.com/Tencent/WeKnora/internal/application/repository 15.328s
```

Diff check:

```text
git diff --check
```

Output: no output; exit status 0.

Diff summary before commit: 2 files changed, 42 insertions(+), 29 deletions(-). `git status --short` listed only the two owned files.

Commit command:

```text
git add internal/application/repository/agent_security_guard.go internal/application/repository/agent_security_guard_test.go && git commit -m "fix: require live local agent for release admission"
```

Output:

```text
[codex/issue30-t64-task8-8a 8090adb3c] fix: require live local agent for release admission
 2 files changed, 42 insertions(+), 29 deletions(-)
```

## Self-review and risks

Lock order is Variant mappings (ordered by ID), live tenant-scoped CustomAgent, immutable AgentVersion, then Release/revocation reads. The helper still uses only its supplied transaction and retains the exact signature. Soft-deleted CustomAgents are not accepted. Focused tests exercised the migrated SQLite schema; PostgreSQL-specific row-lock behavior remains unverified here.
