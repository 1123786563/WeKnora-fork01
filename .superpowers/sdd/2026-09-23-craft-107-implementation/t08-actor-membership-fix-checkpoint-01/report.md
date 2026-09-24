# T08 Actor Membership Admission Fix Report

Date: 2026-09-23. This is a scoped repair of T08-A1 in `2026-09-23-craft-107-t08-actor-admission-review.md`. No commit was created.

## Result

- Replaced the `users.tenant_id` actor check with a query joining `tenant_members AS tm` to `users AS u` by user ID. Admission accepts only `tm.tenant_id` matching the Run tenant, `tm.user_id` matching the immutable actor, `tm.status = 'active'`, `tm.deleted_at IS NULL`, and an active, undeleted user. Home tenant is not used as authorization.
- Preserved actor/User separation, owner/session/input-claim scope, cross-actor replay checks, and generic default-to-owner semantics. Generic Runs also accept a cross-home actor when that actor has an active target-tenant membership.
- Added cross-home Craft and generic admission tests, plus suspended, removed/soft-deleted, missing membership, inactive-user, and deleted-user denial cases. Updated the shared Run test fixture with its canonical active `u1` membership; no production authority was weakened.

## Changed files

- `internal/application/repository/agent_run.go`
- `internal/application/repository/agent_run_actor_test.go`
- `internal/application/repository/agent_run_test.go` (fixture only)

## RED → GREEN and verification

- RED: `go test ./internal/application/repository -run '^TestAdmissionUsesActiveMembershipInsteadOfHomeTenant$|^TestCraftRunAdmissionRejectsInactiveActorMembershipAndUser$' -count=1` failed before the predicate fix. Cross-home active Craft admission returned `agent runtime conflict`; suspended, removed, and missing membership cases were incorrectly accepted.
- GREEN: `go test ./internal/application/repository -run 'TestAgentRunAdmission|TestCraftRunAdmission|TestGenericRunAdmission|TestAdmissionUsesActiveMembership' -count=1` passed (`ok github.com/Tencent/WeKnora/internal/application/repository 15.967s`). Exact output is in `repository-focused.log`.
- GREEN: `go test ./internal/modules/agentruntime/agent/runtime -count=1` passed (`ok github.com/Tencent/WeKnora/internal/modules/agentruntime/agent/runtime 0.663s`). Exact output is in `runtime-focused.log`.
- `git diff --check -- internal/application/repository/agent_run.go internal/application/repository/agent_run_actor_test.go internal/application/repository/agent_run_test.go` passed.
- The targeted repository tests exercise SQLite. The test helper's PostgreSQL subtests were skipped because `TRPC_TEST_POSTGRES_DSN` is unset. No PostgreSQL repository behavior is claimed.

## Checkpoint and limits

Exact before/after source copies, task delta, full worktree status, HEAD, RED/GREEN logs, and SHA-256 manifests are saved under `.superpowers/sdd/2026-09-23-craft-107-implementation/t08-actor-membership-fix-checkpoint-01/`. The before files are copied from the prior reviewed Task 1 checkpoint, so the delta isolates this membership fix while preserving concurrent T01/T19 content.

Task 2 still supplies the actor from authenticated context and enforces TaskWrite; Task 3 restores actor authority for worker capability execution. Independent re-review of T08-A1 is pending. `agent_run.go` is released at this checkpoint.

## Exact checkpoint evidence

- Worktree HEAD at checkpoint: `a5e9195acd6500c085c85d60c852148e7bbbbf34`.
- Live source SHA-256: `agent_run.go` `606f64e479772dd5520420c9a0b000f7e4caf404c9ddf15fe2b59aa023941d72`; `agent_run_actor_test.go` `1c10b70ba042e37e471ba24f60b56cdb2f1c9a0bbd341650f2db41ab4d4aba87`; `agent_run_test.go` `1ecd83fba3c0c6bb32f62731ccd931429a5d0a439e7ebf844d7464b0efe07b12`.
- Checkpoint includes exact before/after files and manifests, task delta, RED/GREEN logs, HEAD, and complete shared-worktree status. Evidence hashes: `red.log` `ea6c69e855cf40893c78a54f2883fb3fb6d7d6376cac4198be1769c21fc4a098`; `repository-focused.log` `ddc753d195a2de9bbfd1340f2409884fb4cb214659d60b962d03cba2775e74ff`; `runtime-focused.log` `d81f6caf841c01b28a827a78657adea3c1ce62310332421623d0267f0191ee04`; `task-delta.patch` `9282a6c33e957a009262672ec71eb9539800bcc84ae12e5fca4023ca6b50024b`.
- `git diff --check` passed after checkpoint capture. Current re-review is pending; checkpoint is not a completion claim for T08 or T19.
