# T55 Task 3 scope negative regression report

## Scope

Added table-driven tenant, owner, and service mismatch dispatch cases using the real `MCPOAuthBindingStore` and `CredentialResolver`. Each case completes the normal baseline, prepare, and approval flow with the matching managed credential, deletes the matching OAuth token row, then seeds only the mismatched scope with `CompleteBinding`. The selected `conn-gh` remains `(tenant=1, owner=u1, credential_ref=mcp_oauth_token:github)`. The Authorization recorder is reset immediately before dispatch.

Each dispatch is rejected (HTTP 409), the post-reset Authorization observation is empty, and the local GitHub stub reports zero writes. The existing positive managed credential Shell boundary test remains unchanged and passes.

## RED evidence

The first attempted fixture seeded only the wrong-scope credential before baseline. It failed during baseline materialization (HTTP 500) because baseline itself resolves credentials. That exposed a test-fixture ordering constraint, not a production defect. Updated the test to prepare and approve with the matching token before replacing its row with the mismatched binding. No production query or behavior was changed.

## Verification

- `go test ./internal/application/repository/ -run 'TestManagedCredentialWrongScopeCannotDispatch' -count=1 -v` — PASS; tenant, owner, and service subtests all pass. Each logs dispatch rejected with HTTP 409; assertions verify zero post-reset Authorization requests and zero provider writes.
- `go test ./internal/application/repository/ -run 'TestManagedDeliveryCredentialNeverEntersGeneralShell' -count=1 -v` — PASS.
- `git diff --check` — PASS.
- Source diff SHA-256 before report/commit: `0f3568595281fcd4c084eb62e7f004eb7c4cfa578b7cc707fe7c292c1172b18a`.

## Changed files

- `internal/application/repository/delivery_recovery_http_test.go`
- `internal/application/repository/delivery_shell_isolation_http_test.go`
- `.superpowers/sdd/plan-t55-integrated-review-repairs/task-3-scope-negative-report.md`

No production code changed.
