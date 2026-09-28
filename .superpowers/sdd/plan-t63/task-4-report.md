# T63 Task 4 Implementation Report

- Task: #63 plan Task 4, HTTP lifecycle endpoints and wiring.
- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t63`
- Branch: `codex/issue30-t63`
- Base HEAD: `b34a4702dee2186004de4848396005c16fe31aeb` (Task 3 review fixes R1/R2 included).
- Scope: only Task 4 owned files plus this report.

## Changes

- Added lifecycle HTTP handler methods for retire, end, unlist and deprecate. They read tenant/actor identity from authenticated context, strictly decode the required deprecation body with the existing 1 MiB cap, and map lifecycle sentinels to 400/404/409.
- Added four POST routes protected by both Admin RBAC and the full-access API-key floor; a nil handler leaves all routes unmounted.
- Wired the lifecycle service and handler in the DI container and registered the handler in `RouterParams`.
- Added route-policy, nil-handler, real SQLite migration-stack happy-path, cross-tenant not-found, authorization, response DTO, required-body and oversize-body tests.

## Evidence

1. RED, before implementation:
   - Command: `go test ./internal/router/ -run 'TestAgentMarketplaceLifecycleRoutesRequireAdminAndFullAccess|TestLifecycleEndpointsHappyPathOverRealStack' -count=1`
   - Result: expected compile failure; `RegisterAgentMarketplaceLifecycleRoutes` and `handler.NewAgentMarketplaceLifecycleHandler` were undefined.
2. Focused lifecycle tests:
   - Command: `go test ./internal/router/ -run 'AgentMarketplaceLifecycle|LifecycleEndpoints|LifecycleHTTP' -count=1`
   - Result: `ok github.com/Tencent/WeKnora/internal/router 1.997s`.
3. Plan verification command:
   - Command: `go test ./internal/router/ -run 'AgentMarketplaceLifecycle|LifecycleEndpoints' -count=1`
   - Result: `ok github.com/Tencent/WeKnora/internal/router 2.087s`.
4. Build:
   - Command: `go build ./...`
   - Result: exit code 0. Linker emitted warnings for duplicate `-lc++` in `cmd/desktop` and `cmd/server`; no build failures.
5. Whitespace:
   - Command: `git diff --check`
   - Result: exit code 0, no output.

## Assumptions and limits

- Retire, end and unlist requests intentionally have no body; deprecate requires `successor_release_id` and uses the shared strict JSON decoder.
- Lifecycle service constructors and sentinel definitions from accepted Task 3 are consumed as-is. No migration or service behavior was changed.
- The test identity middleware follows the existing router test convention and is test-only; production authorization remains in `rbacGuards` route middleware.
- Full diff and route/container wiring were reviewed before commit. Independent task review/validation remains pending; Task 5 was not started.

## Commit

- Pending at report creation; implementation commit SHA will be added after commit.
