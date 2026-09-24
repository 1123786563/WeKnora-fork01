# R5 Effect Task 3a Fix1 — Task 1 report

**Checkpoint:** `craft107-r5-effect-task3a-fix1-task1`  
**Plan:** [R5 Effect Task 3a Fix1 Plan](2026-09-24-craft-107-runview-r5-effect-task3a-fix1-plan.md)  
**Reviewed source:** [Task3a Task1 independent review](2026-09-24-craft-107-runview-r5-effect-task3a-task1-review.md)  
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Base HEAD:** recorded in checkpoint JSON. No commit was made.

## Findings addressed

- **R5E-3A-1:** `ObserveContainer` and `ObserveContainerState` now complete identity checks before their respective Docker effect claims. An observation error or wrong identity returns unresolved without inserting or finishing an effect intent. The next exact retry can obtain the one claim and perform one provider call.
- **R5E-3A-2:** the DB-only `BeginSessionCreate` compatibility marker is persisted and validated before the OpenCode effect claim. The OpenCode claim is acquired immediately before `CreateOpenCodeSession`; a marker write failure leaves no OpenCode intent and exact retry can still claim and send once. The marker's `maySend` value remains ignored as provider authorization.

Post-send handling is unchanged: provider errors or missing/mismatched observed receipts remain unknown and the one-shot claim is never sent again. The original admitted Task/fence/digest and generation binding are unchanged.

## RED → GREEN evidence

Before moving the claims, the new stateful fake-authority tests failed for the reported behaviors:

```text
go test ./internal/container -run '^TestCraftRunViewAdmitted(Preclaim|WrongPreclaim|SessionMarker)' -count=1
```

The failure assertions showed an effect claim recorded after a container observation failure, a claim recorded for a wrong observed container identity, a DockerStart claim recorded despite a failed state observation, and an OpenCode claim recorded when the database marker failed.

After moving the pure observations and marker ahead of each claim, final postimage commands passed:

```text
go test ./internal/container -run '^TestCraftRunViewAdmitted' -count=1
go test -race ./internal/container -run '^TestCraftRunViewAdmitted' -count=1
go test ./internal/container -run '^$' -count=1
git diff --check
```

The focused tests include stateful replay after each transient preclaim failure, exact successful retry with one send, post-send unknown/no-resend, changed identity, and cancellation delivered to a claimed provider call. All passed. The only output warning was the existing duplicate `-lc++` linker warning; commands returned exit code 0. The shared package test window was released after the commands completed.

## Scope and remaining gates

Only `internal/container/craft_runview_runtime.go` and `internal/container/craft_runview_admitted_runtime_test.go` changed for this task. No provider, engine, repository authority, central DI or production route was changed. The earlier Task3a boundary remains: the real provider still combines network/create/start and its read path can call `EnsurePrivateNetwork`; the admitted path therefore remains fail-closed pending provider/engine Task3b and a safe server-owned network provisioning decision.

## Exact checkpoint artifacts

- Preimage archive: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-fix1-task1-preimage.tar.gz`
- Preimage manifest: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-fix1-task1-pre.sha256`
- Task-local patch: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-fix1-task1-task-local.patch`
- Postimage archive: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-fix1-task1-postimage.tar.gz`
- Postimage manifest: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-fix1-task1-post.sha256`
- Machine-readable checkpoint: `docs/plans/2026-09-24-craft-107-runview-r5-effect-task3a-fix1-task1-checkpoint.json`

The task-local patch is relative to the exact preimage and includes only the two owned files. It excludes concurrent worktree edits.
