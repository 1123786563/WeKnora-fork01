# Task 6 Round 1 Validation

- **Revision:** `f10b2ca3bf4e6961da8b523727123509ac8587ee` (`docs(architecture): correct marketplace and runtime ownership`)
- **Parent:** `2945e41`
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-fix-t6-architecture/WeKnora-fork01`
- **Scope:** Task 6 manifest owner assertions, strict schema/reverse coverage, discovered route and hook totals. Read-only validation.

## Command and result

```sh
go test -count=1 ./tools/architectureguard -run 'TestDiscoverRealRepoRouteTotals|TestDiscoverRealRepoHooks|TestGuardCleanAtHead'
```

**Status: FAIL (expected integration blocker remains).** Route-total and hook-total tests passed (no failure reported). `TestGuardCleanAtHead` loaded all 17 manifests and passed its explicit newly merged route and legacy owner assertions, then failed the zero-diagnostic gate with 8 `forbidden-import` diagnostics listed below. Strict `KnownFields(true)` decoding succeeded for all manifests. The guard's route-file reverse coverage was exercised without a route coverage finding; worker reverse-coverage likewise emitted no finding.

Discovered totals expected by passing focused tests: literal routes `676`, API-key routes `87`, Handle routes `0`, total `763`; lifecycle hooks `59`.

## Remaining diagnostics, attributed to pre-Task-4 imports

1. `internal/modules/codedelivery/code_platform.go` → `internal/modules/appconnector`
2. `internal/modules/codedelivery/dispatcher.go` → `internal/modules/appconnector`
3. `internal/modules/codedelivery/dispatcher.go` → `internal/modules/appconnector/service/appconnector`
4. `internal/modules/codedelivery/gitlab_client.go` → `internal/modules/appconnector/service/appconnector`
5. `internal/modules/codedelivery/service.go` → `internal/modules/agentruntime/agent/runtime`
6. `internal/modules/codedelivery/service.go` → `internal/modules/appconnector`
7. `internal/modules/codedelivery/service.go` → `internal/modules/appconnector/service/appconnector`
8. `internal/modules/workbench/service/workbench/command_queue_next.go` → `internal/modules/agentruntime/agent/runtime`

These findings are the exact cross-module import paths assigned to Task 4 in the integration brief and do not indicate a Task 6 manifest ownership gap. Task 6 zero-diagnostic acceptance cannot be marked complete until Task 4 is integrated and the guard is rerun on that merged revision.

## Acceptance assessment

- 17 manifests strict-decode under the guard schema: **PASS**.
- Added agentcatalog/agentruntime/appconnector/workbench route and legacy owner assertions: **PASS**.
- Discovered route and hook totals match declared expectations: **PASS**.
- Reverse coverage: **PASS for routes and workers in this run**; no route-file/worker-reverse-coverage diagnostics.
- Full guard has zero diagnostics: **NOT MET** due to the 8 unrelated pre-T4 `forbidden-import` findings above.
- No source or test files were modified by validation.

## Risk / limitation

The full guard result is not green at this integration point. Re-run the same command after Task 4 lands; this report only validates `f10b2ca3` and must not be reused as evidence for a later merged revision.

## Task 6 repair round 2 — Public Marketplace route ownership

- **Base:** `f10b2ca3bf4e6961da8b523727123509ac8587ee`
- **Change:** moved `RegisterPublicMarketplaceRoutes` from `workbench.yaml` to `agentcatalog.yaml`, and pinned the exact route entry under `newRoutes["agentcatalog"]` in `TestGuardCleanAtHead`.
- **Focused route ownership assertion:** included in `TestGuardCleanAtHead`; it executes the explicit owner assertion before the final diagnostic gate. The route total and hook discovery tests pass.

Commands and results:

```text
go test -count=1 ./tools/architectureguard -run 'TestGuardCleanAtHead|TestDiscoverRealRepoRouteTotals|TestDiscoverRealRepoHooks'
# FAIL at TestGuardCleanAtHead only: 8 forbidden-import diagnostics in the same
# codedelivery/workbench files attributed to Task 4 above; route ownership,
# strict schema, and route/hook discovery assertions produced no failure.

go test -count=1 ./tools/architectureguard -run 'TestDiscoverRealRepoRouteTotals|TestDiscoverRealRepoHooks|TestRunRejectsManifestRouteEntryMissingFromDiscovery'
# PASS

git diff --check
# PASS
```

The full guard remains blocked by the same eight Task 4 cross-module imports and is not claimed as passing. No files outside Task 6 ownership were changed.
