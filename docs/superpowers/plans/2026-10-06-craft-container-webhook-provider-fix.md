# Craft T20 Integration Boot Provider Repair Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to execute this plan. Preserve the uncommitted-checkpoint policy; do not commit.

**Goal:** Restore production container startup so the post-F08 Craft T20 live acceptance stack can resolve `router.NewRouter` and run the browser journey.

**Root cause evidence:** At `main` HEAD `6e1b2072a13a798e7ec25be44784e5242bd2f178`, the documented harness `bash apps/web/e2e/craft-stack.sh up mock` failed because `router.NewRouter` requires `*handler.CommercialWebhookHandler`. `internal/container/container.go` registers `newCommercialWebhookService` and `handler.NewCommercialHandler`, but does not register `handler.NewCommercialWebhookHandler`. `internal/handler/commercial_webhooks.go` defines that constructor and consumes the already registered `*commercialsvc.WebhookService`. This is an application boot regression discovered while executing T20; it is a prerequisite to the requested acceptance, not a change to Craft policy.

**Authority:** Approved Craft Spec `docs/specs/2026-09-23-craft-web-artifact-spec.md`; T20 Issue #139 live snapshot and DAG in `docs/plans/2026-10-06-craft-107-live-issue-refresh.md` and `docs/plans/2026-10-06-craft-107-execution-dag.md`; final acceptance plan `docs/superpowers/plans/2026-10-06-craft-107-final-verification.md`; commercial handler contracts in source.

**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-execution/WeKnora-fork01`, baseline `6e1b2072a13a798e7ec25be44784e5242bd2f178`.

**Commit policy:** No staging, commits, push, merge, deploy, or Issue mutation. Use tracked-diff/untracked-file hashes and a durable task report as the checkpoint.

## Global Constraints

- Limit production changes to the missing provider registration. Do not combine the separate reconciliation-table warning without evidence it blocks startup or T20.
- Preserve the existing tests and all unrelated worktree content.
- Use TDD: first extend the isolated real-container boot smoke test to resolve `*gin.Engine` from `BuildContainer` and demonstrate RED with the missing dependency. Then add the provider and prove GREEN.
- Run validation against the same frozen source hashes that the independent reviewer sees. The validator is read-only and may not edit business code or test sources.
- After focused verification/review, rerun the official Craft harness serially (it owns Docker, SQLite, nginx and fixed ports) under the final integration plan.

## Review Focus

- The regression test exercises the real `BuildContainer` production graph and proves router construction, rather than merely checking that a provider text exists.
- The handler constructor receives the registered webhook service and only one provider is added.
- Existing commercial webhook request tests and isolated container boot behavior remain intact.
- No unrelated commercial migration/reconciliation behavior is changed.

## Task 1 — TDD the production router resolution

**Depends on:** T20 boot failure reproduced and diagnosed; no code prerequisites.
**Role:** backend_implementer.
**Validator:** backend_validator, read-only.
**Owned files:** `internal/container/bootsmoke/boot_smoke_test.go`, `internal/container/container.go`; task report under `docs/superpowers/reports/`.
**Interface:** test consumes `container.BuildContainer(dig.New())`; production provider graph produces `*gin.Engine` via `router.NewRouter`.

1. Record initial HEAD, tracked diff and worktree-scoped untracked inventory/hashes.
2. Extend `TestBuildContainerBootsLite` to invoke `func(*gin.Engine)` and require a non-nil router after the current session-handler assertions. Import Gin only for this observable production seam.
3. Run `go test ./internal/container/bootsmoke -run TestBuildContainerBootsLite -count=1`; expect RED with missing `*handler.CommercialWebhookHandler`.
4. Register `handler.NewCommercialWebhookHandler` in the commercial webhook provider block, after `newCommercialWebhookService` and before router construction. Do not alter service behavior or routes.
5. Rerun the same test; expect GREEN. Also run `go test ./internal/handler -run 'TestCommercialWebhook' -count=1` and `go test ./internal/router -run 'TestCommercial' -count=1` if matching tests exist; report explicit no-match rather than implying coverage.
6. Self-review only the owned files, record exact commands/results and final diff hashes in report. Do not commit.

**Acceptance:** the new real-container assertion fails before provider registration for the diagnosed type and passes after it; both existing webhook handler tests pass; diff is limited to the two owned files.
**Failure handling:** if another missing dependency appears, record its exact type and stop to update this plan/DAG with the proven expanded scope before changing additional providers.

## Task 2 — Independent verification and review

**Depends on:** Task 1 GREEN checkpoint.
**Role:** backend_validator (read-only), then independent reviewer (read-only).
**Owned files:** validation/review reports only.
**Consumes:** frozen Task 1 diff and report.

1. Verify the router resolution test and webhook tests against the exact recorded source hashes.
2. Reviewer evaluates Spec compliance and code quality separately, focusing on production DI ordering and regression seam.
3. If either reviewer finds a defect, return it to a new scoped repair pass; validator/reviewer must not edit source or tests.

**Acceptance:** validator results bind to the exact frozen checkpoint; independent reviewer accepts both compliance and quality, or findings are routed through SDD repair.

## Dependency and resource preflight

| Work | Depends on | Files / interfaces | Shared resources | Dispatch |
|---|---|---|---|---|
| Real-container regression and provider | proven boot failure | one test file + one provider file; stable router DI interface | temporary SQLite, Go build cache | single backend implementer |
| Validation and independent review | frozen fix checkpoint | read-only source/test | Go build cache; no browser stack | sequential, separate agents |
| Full Craft Playwright journey | repaired/verified router boot | entire production Craft path | exclusive Docker, SQLite, nginx and ports | only after focused repair gate |
