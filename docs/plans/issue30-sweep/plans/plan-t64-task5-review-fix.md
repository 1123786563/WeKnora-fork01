# T64 Task5 Review Fix — atomic canceled count and dependency-path coverage

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Narrow repair to Task5 commit `fd046e8dfe26ff76eedd4cf607ced657b4c4956e`.

**Goal:** Keep revocation history's canceled count consistent with the run state even if persistence fails, and exercise dependency cancellation/rollback paths.

**Findings:** R1 MEDIUM: run cancellations commit in `CancelRunsByAgents`, then canceled_run_count is updated separately; if final update fails, history permanently says zero after successful cancellation. R2 LOW: dependency cancel path has no behavioral coverage. R3 LOW: dependency ledger+audit rollback path is not tested.

## Constraints and ownership
- Continue in T64 worktree after `fd046e8d` Task5 implementation.
- Owned files: `internal/application/repository/agent_run_security_cancel.go` and its test; `internal/application/service/agent_security.go` and its test; if strictly needed, `repository/agent_security.go` and test for typed row/table helper. No T63 files.
- Preserve ledger+audit atomic transaction from supplement, append-only history, post-commit in-flight disposition, release/dependency predicates, bounded reason, tenant scope.

## Task 1 — make cancellation + count one transaction
1. Add typed repository operation(s) for canceling runs in the context of a persisted release/dependency revocation ID. Inside existing AgentRunStore cancellation transaction, cancel matching runs and update the corresponding tenant-scoped ledger row's `canceled_run_count` before commit; if count update fails, roll back all cancellations/events/session releases. Preserve existing CancelRunsByAgents API if used elsewhere.
2. Service calls this operation after ledger+audit commit and removes the separate `Update*RevocationCanceled` write from the successful cancel path. The returned count populates response. `allow` path remains zero and does not cancel.
3. RED test forces ledger-count update failure after cancellation changes are staged and proves run/event/session + ledger count all roll back. No sleeps; use registered callback/constraint. Add dependency revoke cancel case: matching locked release only, other digest/version and foreign tenant stay running, response and stored count equal one.
4. Add dependency audit rollback test proving both dependency ledger and audit row absent after forced audit insertion failure.
5. Run focused cancellation/revocation/rollback tests, related repo/service filters, build and diff-check. Commit only owned files and report exact evidence.

**Review focus:** No commit point can leave canceled runs with stale count; all database effects roll back together when final count persistence fails. Audit+ledger remains its earlier transaction; cancellations/count are an explicit subsequent atomic transaction.
