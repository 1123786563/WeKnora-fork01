# Issue 140 Task 11 — Rule Claim Repair Report

## Scope and checkpoint

- Task: Make due-rule claiming atomic with pause/edit and bound rule-list reads.
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-task11-rule-claim/WeKnora-fork01`
- Base: `1ff972596d8a3ce619698b96911fe444c3bf574a`
- Implementation commit: `18de01641a2d0fac87603fee219f1d84cd754dc3`
- Migration ruling: no schema migration was added. Existing `career_search_rule_runs` already has the scoped unique `(tenant_id,user_id,rule_id,period)` key plus request ID, status and body fields. The claim stores its durable `started` state and frozen recovery intent in those fields. Existing rule migration IDs remain unchanged; SQLite up/down coverage passed.

## Behavior and evidence

- The claim transaction locks the scoped current rule and compares enabled status, scanned revision, query, last period, exact scanned due time and `next_due_at <= now`. A stale candidate is skipped before quota admission.
- The transaction persists a `started` run with deterministic `rule:<id>:<period>`, frozen query and profile revision. That commit is the linearization point. Quota admission and `SearchOnce` run after the transaction closes.
- Started runs recover under the same request ID, including when the rule is paused after claim. Recovery does not use a time-only takeover for the rule-period claim. SearchOnce retains its existing request-ID based claim/replay behavior.
- Rule list pages are owner scoped, keyset ordered by `updated_at DESC, id ASC`, limited to 50 summaries, and exclude run/todo histories. Cursor payloads are owner-bound. `nextDueAt` is always serialized and is null for paused/disabled rules; an enabled rule missing its due time fails closed.
- RED evidence before implementation: the pause-after-scan regression reached quota twice and the claim-first regression found no persisted run before quota. The list contract initially failed to compile because `Office.ListRules` was absent.

## Commands and results

- `go test ./internal/career -run 'TestDueRulePauseCommittedAfterScanPreventsClaimAndSearch|TestDueRuleClaimBeforePauseFinishesSameRequest|TestListRulesUsesStableBoundedOwnerScopedPages' -count=1` — RED before implementation: stale candidate reached quota; no started claim existed before quota; list method was missing.
- `go test ./internal/career -run 'TestDueRule|TestSecondStale|TestStartedRule|TestListRules|TestCareerRuleHTTPContract|TestSetRule|TestRuleTrigger|TestRuleBudget|TestRuleIncomplete' -count=1` — passed.
- `go test -count=1 ./internal/career` — passed.
- `go test -count=1 ./internal/database -run TestSearchRuleMigrationUpAndDown` — passed.
- `go test -count=1 ./internal/router -run TestCareerSearchRuleRoutesAreRegistered` — passed.
- `git diff --check` — passed.

Postgres migration integration was not run; the repository's search-rule migration test is SQLite up/down only and no Postgres test is wired to this schema in the relevant package.

## Changed files

- `internal/career/search_rule.go`
- `internal/career/search_rule_test.go`
- `internal/career/handler.go`
- `internal/career/handler_test.go`
- `internal/router/routes_career.go`
- `internal/router/routes_career_test.go`
