# W23 targeted fix report

## Scope

This fix closes the revoke-wins admission side effect, routes the contract
revoke endpoint through `ExecutionTargetHandler`, strengthens registration
projection matching, and makes the Paseo personal-node composition fail closed
and scrub secure credentials on cleanup errors.

Commits:

- `9bc2cee8` — W23 implementation fix and focused evidence.
- `81a70b94` — isolated repository baseline repair for the W20 `dispatch` field/method name collision.

## Evidence

- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test ./internal/application/service/workbench -run 'TestPersonalTargetAdmission' -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test -race ./internal/application/service/workbench -run 'TestPersonalTargetAdmission' -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test ./internal/application/repository -run 'TestExecutionTarget' -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test -race ./internal/application/repository -run 'TestExecutionTargetFacade' -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test ./internal/router -run 'TestExecutionRegistrationRoutesUseTargetFacade' -count=1` — PASS.
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test ./internal/handler -run 'TestExecutionTargetRegistrationGinSQLiteLifecycle' -count=1` — PASS (legacy direct lifecycle fixture).
- `DEVELOPER_DIR=/Library/Developer/CommandLineTools GOWORK=off go test ./internal/application/service/workbench -count=1` — PASS.
- `pnpm exec tsx --test services/paseo-adapter/src/node-connector.test.ts` — PASS, 7/7.
- `pnpm --filter @weknora/paseo-adapter typecheck` — PASS.
- `git diff --check` — PASS.

## Remaining evidence limits

- No live Paseo personal node/provider endpoint was authorized; live bridge
  acceptance remains `blocked-env`.
- No PostgreSQL runtime or applied migration acceptance was available in this
  worktree; SQLite focused tests use the existing migration/test harness.
- The production route mapping now passes `ExecutionTargetHandler` from the
  router composition. Full `NewRouter` boot with production API-key and RBAC
  middleware remains environment-sensitive and is not claimed by the focused
  route test.
