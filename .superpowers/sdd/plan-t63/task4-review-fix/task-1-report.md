# T63 Task 4 Review Fix — Task 1 Report

- Finding: `T63-T4-R1` (High), lifecycle handler interface missing from production Dig graph.
- Worktree: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue30-sweep/.worktrees/issue30-b6-coordination/.worktrees/issue30-b6-t63`
- Base HEAD: `b8bf351dd242d7811338b03a1ae9a9b5b518c070`.
- Scope: lifecycle service/handler container binding and a focused container resolution test. No Task 5 files changed.

## Change

Added `provideAgentMarketplaceLifecycle`, which registers the concrete lifecycle service, adapts that instance to `interfaces.AgentMarketplaceLifecycleService`, and registers the handler. `BuildContainer` now uses this same helper. The test constructs a narrow Dig graph with the real SQLite-backed repository constructors, invokes the same provider helper used by production, and requires a lifecycle handler in a required router-parameter probe.

## Evidence

1. RED before the provider fix:
   - Command: `go test ./internal/container/ -run TestAgentMarketplaceLifecycleHandlerResolvesForRouterParams -count=1`
   - Result: expected failure. Dig reported missing `interfaces.AgentMarketplaceLifecycleService` and suggested `*service.AgentMarketplaceLifecycleService`.
2. GREEN container resolution:
   - Command: `go test ./internal/container/ -run TestAgentMarketplaceLifecycleHandlerResolvesForRouterParams -count=1`
   - Result: `ok github.com/Tencent/WeKnora/internal/container 2.322s`.
3. Lifecycle route regression:
   - Command: `go test ./internal/router/ -run 'AgentMarketplaceLifecycle|LifecycleEndpoints' -count=1`
   - Result: `ok github.com/Tencent/WeKnora/internal/router 1.880s`.
4. Build:
   - Command: `go build ./...`
   - Result: exit code 0. Existing duplicate `-lc++` linker warnings appeared for `cmd/server` and `cmd/desktop`.
5. Whitespace:
   - Command: `git diff --check`
   - Result: exit code 0, no output.

## Review focus and limits

- The production and test graph use the same provider helper, so the concrete-to-interface binding is exercised rather than reconstructed separately in the test.
- Existing router tests continue to verify lifecycle route mounting, Admin/full-access policy, and HTTP behavior. `router.NewRouter` itself is not constructed in this isolated test because it requires the application's much larger graph; the focused graph resolves the required `RouterParams` handler dependency that previously failed startup.
- Independent scoped re-review remains pending.

## Commit

- Implementation commit: `70e2dc784bcfc984624c2201d9234814680d0b00`.
- The report is recorded in a follow-up documentation commit.
