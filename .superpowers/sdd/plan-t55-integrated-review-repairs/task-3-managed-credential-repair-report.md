# T55 Task 3 Managed Credential Repair Report

## Scope

Implemented the assigned Task 1 from `docs/plans/issue30-sweep/plans/2026-09-30-t55-task3-credential-path-repair.md`.

- The Shell boundary test now uses `MCPOAuthBindingStore.CompleteBinding` to persist a fake GitHub OAuth token for tenant `1`, owner `u1`, service `github`; checks the stored token and production `CredentialResolver` result; and exercises Delivery dispatch before Shell execution for the same tenant.
- The test asserts the actual stored token is used in Delivery Authorization and absent from serialized persisted/effective Shell config, Shell create/exec observations, and command output. The operator Shell environment value remains intact.
- The shared recovery fixture supports an injected HTTP client and a managed-credential mode while leaving the default fake source intact for other recovery tests. The Authorization recorder no longer mutates `http.DefaultClient`.

## RED evidence

Ran the boundary test on the R1 baseline test and fixture after temporarily adding the production-reference assertion. It failed as expected: expected `mcp_oauth_token:github`, got synthetic `mcp:conn-gh:github`. Restored the final implementation before verification.

## Verification

- `go test ./internal/application/repository/ -run '^TestManagedDeliveryCredentialNeverEntersGeneralShell$' -count=1 -v` — PASS.
- `go test ./internal/modules/execution/sandbox/ -run 'TestWithWorkspaceEnvDefaults|TestResolveUsesOptionalRemoteClientFactoryAndDefaultsToProductionFactory' -count=1 -v` — PASS (3 matching tests).
- `git diff --check` — PASS.
- Final two-file source diff SHA-256 before report/commit: `9fe8d90e0b55778a0259e001f163b3107225c229b6e61866d53c3197b187249e`.

## Change boundary

Only the two assigned repository test files and this report are included. No production source or resolver seam changed. All outbound provider traffic used the local fake GitHub server and Shell fake client. The fixture seeds the minimal migrated `mcp_services` row required by the token table foreign key.

Implementation commit SHA: `944a097f006b22c28f4ff2dc5254f0a3fba15973`.
