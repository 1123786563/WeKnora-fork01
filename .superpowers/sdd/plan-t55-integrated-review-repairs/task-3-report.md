# T55 Integrated Review Repairs — Task 3 Report

Status: **NEEDS_CONTEXT — stopped at the task's explicit seam boundary**

## Scope and evidence

Task 3 requires a single production-wiring test that uses the persisted tenant Shell configuration loader and `SessionBoundManager` to create and execute through a fake provider, while independently resolving the fake managed Delivery credential through production Delivery wiring.

The Shell production path is traceable:

- `internal/application/service/tenant_sandbox_resolve.go` implements `NewTenantSandboxConfigLoader` and loads tenant-scoped persisted config through `TenantSandboxConfigRepository`.
- `internal/modules/execution/sandbox/tenant_resolver.go` calls that loader, resolves effective tenant config, then its private `buildClient` constructs concrete Cube/E2B/Docker clients and passes the result to `NewSessionBoundManager`.
- `internal/modules/execution/sandbox/session_manager.go` exposes `SessionBoundManagerConfig.Client`, but this is only injectable when constructing the manager directly; the production `TenantSandboxResolver` offers no client factory / provider override.
- `internal/container/sandbox.go` wires the production tenant resolver to the concrete resolver and guarded shared transport.

Therefore the test cannot exercise the production tenant-config resolution path and execute a benign command against a fake `RemoteSandboxClient` without widening/changing the resolver composition seam or introducing an alternate helper-only path. The plan explicitly says to stop and report the exact seam if production Shell creation cannot be exercised from this repository integration test without expanding owned files. No source changes were made and no fake-only substitute test was added.

## Verification

- `sed -n '1,250p' internal/modules/execution/sandbox/tenant_resolver.go` — confirmed tenant loader is called by `Resolve`; confirmed private `buildClient` hardcodes concrete Cube/E2B/Docker constructors.
- `sed -n '1,180p' internal/modules/execution/sandbox/session_manager.go` — confirmed `SessionBoundManagerConfig.Client` is injectable only at direct construction.
- `sed -n '1,170p' internal/container/sandbox.go` — confirmed container uses `NewTenantSandboxResolver` with production loader and no fake provider seam.
- Focused boundary test, `TestWithWorkspaceEnvDefaults`, and `git diff --check` were not run because no boundary test was safely implementable within the owned files and acceptance explicitly requires stopping before widening.

## Handoff needed

Provide an approved, testable provider-factory seam to `TenantSandboxResolver` (or authorize a narrowly scoped resolver test file/change) so a test can retain the production tenant config loader and `SessionBoundManager` while injecting a fake `RemoteSandboxClient`. Then the required assertions can be built without real provider credentials or network writes.
