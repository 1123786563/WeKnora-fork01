# T55 Integrated Review Repairs — Task 3 R1 Report

Status: **DONE**

## Changed files

- `internal/modules/execution/sandbox/tenant_resolver.go`: added optional `RemoteClientFactory` to `TenantSandboxResolverDeps`. Resolver uses it only when explicitly supplied; nil continues through the existing private `buildClient`, which builds concrete Cube/E2B/Docker clients using existing guarded transports. Production container wiring remains unchanged and leaves it nil.
- `internal/modules/execution/sandbox/tenant_resolver_test.go`: added coverage for injected factory selection and the default concrete-client manager path.
- `internal/application/repository/delivery_shell_isolation_http_test.go`: added a production-wiring credential boundary test.

## Behavior and evidence

`TestManagedDeliveryCredentialNeverEntersGeneralShell` persists a tenant E2B configuration via the real GORM `TenantSandboxConfigRepository`, loads it through production `NewTenantSandboxConfigLoader`, resolves a real `SessionBoundManager` through `NewTenantSandboxResolver`, and executes via `ExecShellCommandWithOptions` against a fake provider client at the optional factory seam. It also drives the real Delivery baseline → approval → dispatch HTTP path with the fake credential source and GitHub wire stub, capturing outbound `Authorization` through the HTTP transport.

The test asserts that all Delivery requests carry exactly `Bearer ghp_T25PROBE_7f3a9c1e`; the probe is absent from serialized persisted tenant config, effective Shell config, provider create requests, both provider exec requests (workspace bootstrap and requested command), and command output. The distinct operator Shell value `operator-shell-value-keep-me` survives persisted/effective configuration and appears unchanged in the provider create environment.

## RED → GREEN and verification

- RED: before the resolver seam, the new resolver test failed to compile with `unknown field RemoteClientFactory`.
- GREEN: after adding the optional default-preserving factory seam, the focused resolver test passed.
- `go test ./internal/application/repository/ -run '^TestManagedDeliveryCredentialNeverEntersGeneralShell$' -count=1 -v` — PASS.
- `go test ./internal/modules/execution/sandbox/ -run 'TestWithWorkspaceEnvDefaults|TestResolveUsesOptionalRemoteClientFactoryAndDefaultsToProductionFactory' -count=1 -v` — PASS (workspace env preservation and resolver seam/default behavior).
- `go test ./internal/modules/execution/sandbox/ -run 'TestWithWorkspaceEnvDefaults|TestResolve' -count=1` — PASS (complete resolver-focused regression selection).
- `git diff --check` — PASS.

## Scope notes

The initial Task 3 report documented the lack of provider injection. The authorized R1 plan commit `65e02e15f` amended ownership to allow this narrow optional seam and resolver test; this report supersedes the initial NEEDS_CONTEXT status. No production credential behavior changed, no operator Shell environment values were stripped, and no real provider credential or write was used.
