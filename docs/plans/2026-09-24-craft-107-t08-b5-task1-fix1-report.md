# T08 B5 Task1 fix1 report: real Auth and precise assertions

## Result

Updated only `internal/router/craft_b5_joined_test.go` for fix1. The joined journey now mounts production `middleware.Auth` and `registerSessionRoutes`, retains production RBAC and API-key route policies, and uses test-only `ValidateToken`/`AuthenticateAPIKey` credential-verification adapters. Tenant lookup and active membership/role resolution use the real migrated-database `TenantService` and `TenantMemberService`; the persistent Craft access, Session and version services remain real.

No production code or other test file changed. No commit was made. The isolated preview request now uses the configured `preview.example.test` host while both production preview gates stay off.

## Findings addressed

- **Auth boundary:** Replaced `X-Test-User`/`X-Test-Key` context injection with bearer/API-key credentials passed through actual `middleware.Auth`. The fake verifier only maps test credential strings to validated user/key records; Auth itself resolves the tenant, membership role, caller/principal context and API-key scope. Anonymous access reaches Auth and returns 401. A nonmember is rejected by Auth; active ungranted members reach the Craft list, receive 200, and the parsed `data[].session_id` excludes `task-b5`.
- **Preview Host:** The disabled `/p/` request sets `Host: preview.example.test`, asserts `AcceptsPreviewHost` for that configured origin, then exercises the preview handler on the isolated path. It returns 404 with zero bytes and no storage open under the disabled production browser gate.
- **List evidence:** The helper requires list 200 for authenticated active ungranted actors and parses the returned IDs. The same-tenant nonmember expects Auth's 403 because production authentication rejects a missing active membership before list handling.
- **Audit exactness:** The failed Viewer grant mutation must not change the durable audit row count or create a Task grant. The successful event multiset is asserted exactly: `member_added|viewer` 3, `member_added|collaborator` 1, `member_revoked|viewer` 1 and `member_revoked|collaborator` 1. Tenant, actor, scope, target and outcome fields are still validated. Denied-read audit remains open: zero rows before successful mutations are logged without asserting an undefined denial action contract.
- **Gin global state:** Saves the prior `gin.Mode()` and restores it in `t.Cleanup`; Craft handler registries are also restored.

## RED / GREEN and evidence

The saved preimage at `docs/plans/2026-09-24-craft-107-t08-b5-task1-fix1-checkpoint/pre/craft_b5_joined_test.go` is SHA-256 `d8eb9ee65a991d64c08f51332a2cdcc9731bc3df349cdfd0a5af3f357cddbbbe`. It shows the previous injected identity adapter, an unqualified `/p/` host, list status omission, row-count-only audit assertion and un-restored Gin mode. RED evidence came from the first run after switching the route to `middleware.Auth`: it rejected the non-Craft test owner because that user had no active membership, and a mistakenly full-access API-key fixture bypassed the missing-chat-capability denial. The test was corrected by seeding that owner as an active tenant member and making the key scoped (non-full-access), after which the capability and membership behavior passed through Auth. The postimage is SHA-256 `19a7eaa72772bd873927f4bde28b0f93710c38da90c154efc069817734793c1d`; its complete test-only delta is in `docs/plans/2026-09-24-craft-107-t08-b5-task1-fix1-task-local.patch` (SHA-256 `6fe14193bc7532e2a6d904c96c8a923f66bf32d9960216b05bdb40be172ba9dc`).

A later assertion refinement confirmed the chat-capable API key reaches the Craft ACL and receives the actual 403 response (the earlier over-specific 404 expectation was corrected). Final commands on HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`:

```text
go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction|^TestCraftAccessProductionSessionAssemblyAuthAndAudit$' -count=1
PASS — ok github.com/Tencent/WeKnora/internal/router 2.378s

go test ./internal/application/service -run '^TestGetSession|^TestCraftT08Journey$' -count=1
PASS — ok github.com/Tencent/WeKnora/internal/application/service 1.548s

go test ./internal/container -run '^TestCraftPreviewConstructorUsesCurrentPersistentTaskAccessAndStaysDisabled$' -count=1
PASS — ok github.com/Tencent/WeKnora/internal/container 3.008s (linker warned about duplicate -lc++)

go test ./internal/handler/session -run '^TestCraftPreview' -count=1
PASS — ok github.com/Tencent/WeKnora/internal/handler/session 1.997s

git diff --check -- internal/router/craft_b5_joined_test.go
PASS
```

The full postimage, exact task patch, source hashes and validation evidence are recorded in `docs/plans/2026-09-24-craft-107-t08-b5-task1-fix1-checkpoint.json`. The overall route journey remains disabled-preview evidence only; successful ticket redemption and browser/no-egress proof remain T14-dependent. The credential fakes isolate only credential verification; this test does not validate cryptographic JWT signature verification or the production API-key storage hash implementation.
