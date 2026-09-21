# Task 5 Fix Round 1 Report

Status: DONE  
Base commit: `88017d8991c72051f8d20f24855e9733f17ef7cd`

## Findings Addressed

- `GET /agents/:id/versions/:versionId` now compares the tenant-scoped loaded snapshot's `AgentID` to the path `:id`. A mismatch returns the existing structured 404 response.
- The production fail-closed dependency resolver now lists every unavailable immutable dependency as its type and ID (for example, `skill "tenant-skill-a"` and `subagent "tenant-subagent-b"`). It does not synthesize lock fields or resolve through mutable sources.

## TDD Evidence

- **RED, path binding:** The lifecycle test requested a valid same-Tenant version beneath another Agent path and failed as expected: got 200, wanted 404.
- **RED, missing dependency identity:** The focused container test failed because the previous generic resolver error omitted `skill "tenant-skill-a"`.
- **GREEN:** `go test ./internal/router -run TestTenantAgentMarketplaceLifecycleAndAuthorization -count=1` passes; `go test ./internal/container -run TestTenantReleaseDependencyResolverNamesMissingDependencies -count=1` passes.
- **Package suites:** `go test ./internal/handler ./internal/router ./internal/container -count=1` passes. Container emits the existing duplicate `-lc++` linker warning.
- **Diff hygiene:** `git diff --check` passes.

## Changed Files

- `internal/handler/agent_version.go`
- `internal/router/routes_agent_marketplace_test.go`
- `internal/container/container.go`
- `internal/container/tenant_release_dependency_resolver_test.go`
- `.superpowers/sdd/2026-09-20-t28-tenant-catalog-release/task-5-fix-report-round1.md`

## Remaining Risk

The existing Skill and Subagent sources do not provide immutable distributable release records with license metadata. Agents referencing them remain blocked from submission until an approved immutable dependency source exists.
