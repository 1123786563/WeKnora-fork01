# R2 Task 1 Report — Guard append APIs and prove lock order

## Scope and checkpoint

- Brief: `task-1-brief.md`; implements R1 findings T64-6-R1-1 and T64-6-R1-2 only.
- Source worktree: `/Users/wuyongjun/.codex/worktrees/issue30-t64-t6/WeKnora-fork01`.
- Base: `236e2c2429f8872cebb80aea14b1632a86a98df8`.
- Modified files: `internal/application/repository/agent_security.go`, `internal/application/repository/agent_security_test.go`.
- Commit: `22571cd3203a2cb76d2cb90532b0091189a7d055` (`fix: guard agent revocation append paths`); local only, no push.

## Implementation

- Added transaction-scoped `appendReleaseRevocationTx` and `appendDependencyRevocationTx` helpers.
- Public no-audit append methods now acquire `withTenantSecurityGuard`; WithAudit methods reuse the helpers inside the same guard transaction and append the audit record atomically without nested transactions.
- Changed revocation-first coverage for adoption, variant, publish, and proposal to use a revocation-table Create callback barrier, then a separately observed admission tenant-guard attempt, followed by explicit channel release and assertions.
- Added exact dependency tuple `(skill, weather, 1.2.3, sha256:abc)` admission-first and revocation-first interleavings using public `AppendDependencyRevocation`.
- Concurrent test DB pools are configured for four open connections. No sleeps are used.

## Verification

Commands run from the source worktree:

1. `gofmt -w internal/application/repository/agent_security.go internal/application/repository/agent_security_test.go` — passed.
2. `go test ./internal/application/repository/ -run '^(TestTenantSecurityGuardSerializesDecisiveWriteFamilies|TestAgentSecurityStoreAppendAndListKeepsHistoryTenantScoped|TestTransactionAdmissionMatchesCompleteDependencyTuple|TestAgentSecurityStoreAppendReleaseAndAuditRollsBackTogether|TestAgentSecurityStoreAppendDependencyAndAuditRollsBackTogether|TestAppendDependencyRevocationSerializesAgainstAdmission)$' -count=10 -timeout=180s` — passed (`ok`, 60.608s).
3. `go test ./internal/application/repository/ ./internal/application/service/ -count=1` — passed (`repository` 247.788s; `service` 194.084s).
4. `go build ./...` — passed (exit 0); linker emitted duplicate `-lc++` library warnings for `cmd/desktop` and `cmd/server`.
5. `git diff --check` — passed.

## Ordering evidence and limits

The callback barriers establish that the first transaction already acquired the tenant guard and entered its decisive insert/write callback before the second transaction's tenant-guard SQL reaches the registered raw callback. The second callback is released only after the first transaction has committed. In the revocation-first cases, admission then proceeds and returns `ErrAgentSecurityReleaseBlocked`. This records ordering at the repository/driver callback boundary; it does not inspect internal SQLite lock-manager state. Runtime PostgreSQL behavior was not exercised in this environment.

## Review package note

The committed scope is exactly the two owned files. Commit parent is BASE `236e2c2429f8872cebb80aea14b1632a86a98df8`; new HEAD is `22571cd3203a2cb76d2cb90532b0091189a7d055`. After commit, `git status --short` was empty. The commit is local only and was not pushed.
