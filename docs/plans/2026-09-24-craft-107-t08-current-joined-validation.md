# T08/#126 current joined validation

Date: 2026-09-24. Read-only validation in `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Scope: current integrated B1 ACL/list, B2 generic direct Session read, B3 preview issue/redemption/file read, and B5 authenticated HTTP joined route evidence. No business or test source was changed. Only this report was added.

## Verdict

**DONE_WITH_CONCERNS.** The same current integration tree passes the focused B1 service/list, B2 service, B3 preview service/handler revocation, B1/B3 container wiring, and B5 authenticated production-route test. A real authenticated HTTP journey exists for list → generic direct Session → versions → file download → revoke/repeat, and exercises migration-backed membership/grant state, denial audit, anonymous/API-key guards, tenant boundaries and stale/rejoined membership. The full acceptance journey is still incomplete: production preview is deliberately disabled; preview issuance/redeem/read is split between this production-route test and focused handler tests, and the B5 test's separately test-gated preview request does not exercise valid ticket redemption and byte serving through production auth. One existing handler test, `TestCraftHTTPListAndVersionDownload`, fails consistently at workspace lookup on this tree. The first test attempts hit a transient compile error in a concurrently edited unrelated file; they passed after that file's hash changed.

## Revision and source hashes

`git rev-parse HEAD` → `a5e9195acd6500c085c85d60c852148e7bbbbf34` (HEAD unchanged; relevant work is uncommitted).

Hashes at final check:

```text
313f0d7248606ab9fbdcdf4ef5c523deab26c085772a988d3edf20a815055e8f  internal/application/service/craft_session.go
e3c08887c24ba03d6088d1ab1b2c7adacdb138a1c8ce44b88b9862cce3ba8241  internal/application/service/session.go
74dd3c54e33906940f8d25e348ac73352f823d514fac2fc57d5edebd4311e855  internal/container/container.go
4f4a5f61e9f9275c77cb936a51a3e7efde7a99bc179502725e18bfd7acc77c3b  internal/handler/session/craft_preview.go
77fa21edd8d80d951716b30e3876b733e5462d20adf48f4b16b38f9d52e29e3f  internal/handler/session/craft_preview_persistent_access_test.go
4798ce71963621f0d5f512155b0680e044572158fcb504bf66331888c46c2951  internal/router/craft_b5_joined_test.go
ddcc0fe7fa024c468b8baf8a879178cecf5e90ca538c4ec1d732e98c8a532b48  internal/handler/session/craft_test.go
7aada59fed70c1647a59eb31908389b7c1bee47824149b0a8aed25383d614260  internal/application/repository/craft_docker_output.go
```

The new unrelated `craft_docker_output.go` changed from SHA-256 `0d2e2a9cce8b951ad66d6e711cd5188cae896cbe49b7356f1cfc3084118eca9a` to `979108fbe7f364f993843347cabc4d48344da3299feee7af158c93f9b3eafb99` while waiting, confirming an owner was editing it. Its initial unused `craft` import caused the first service, handler and router build attempts to exit 1 before running tests. After the source hash changed, reruns compiled; the T08 source hashes above remained stable through final checks.

## Commands and results

| Exact command | Exit | Result |
|---|---:|---|
| `go test ./internal/application/service -run '^(TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes|TestCraftSessionTaskAccessRejectsStaleMembershipAndAdminWithoutGrant|TestCraftSessionListPaginatesOwnedAndGrantedTasksWithCurrentMembership|TestGetSessionCraftTaskRequiresCurrentTaskRead|TestGetSessionCraftTaskDeniesWithoutTaskReadIncludingTenantAdmin|TestGetSessionCraftTaskCannotCrossTenant|TestGetSessionCraftTaskFailsClosedOnClassificationFailure|TestGetSessionCraftTaskFailsClosedWhenRegistrationChangesBeforeReturn|TestCraftT08Journey)$' -count=1 -v` | 0 | PASS; B1 grant/revoke/list/direct/version/download ACL, B2 Craft classification/TaskRead denial and metadata non-leak, migration journey, current grant audit. |
| `go test ./internal/handler/session -run '^(TestCraftHTTPViewerReadOnlyAndCrossTenantInvisible|TestCraftHTTPListAndVersionDownload|TestCraftPreviewCapabilityStopsServingAfterPersistentGrantRevocation|TestCraftPreviewIssueRedeemServe|TestCraftPreviewIssueCrossTenantNotFound|TestCraftPreviewExpiredGrantsReturn404|TestCraftPreviewFailsClosedWithoutBrowserNavigationBoundary)$' -count=1 -v` | 1 | Preview issue/redeem/file-read/revoke selectors and Viewer HTTP denial passed; `TestCraftHTTPListAndVersionDownload` failed at `craft_test.go:522`: `craft not found: workspace ws-http-...`. |
| `go test ./internal/handler/session -run '^TestCraftHTTPListAndVersionDownload$' -count=1 -v` | 1 | Same failure reproduced independently. |
| `go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction$' -count=1 -v` | 0 | PASS; actual `middleware.Auth`, API-key authorizer, registered session/Craft routes and real SQLite migrations/access service. |
| `go test ./internal/container -run '^(TestNewCraftSessionServiceReceivesSharedTaskACLPorts|TestCraftPreviewConstructorUsesCurrentPersistentTaskAccessAndStaysDisabled)$' -count=1 -v` | 0 | PASS; shared current ACL ports and persistent preview checker wiring; linker emitted duplicate `-lc++` warning only. |
| `go test ./internal/application/service -run '^(TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes|TestCraftSessionTaskAccessRejectsStaleMembershipAndAdminWithoutGrant|TestCraftSessionListPaginatesOwnedAndGrantedTasksWithCurrentMembership|TestGetSessionCraftTaskRequiresCurrentTaskRead|TestGetSessionCraftTaskDeniesWithoutTaskReadIncludingTenantAdmin|TestGetSessionCraftTaskCannotCrossTenant|TestGetSessionCraftTaskFailsClosedOnClassificationFailure|TestGetSessionCraftTaskFailsClosedWhenRegistrationChangesBeforeReturn|TestCraftT08Journey)$' -count=1 -v` (first attempt) | 1 | Build stopped before tests: unrelated concurrent `internal/application/repository/craft_docker_output.go:11` imported `craft` but did not use it. Rerun after observed hash change passed. |
| `go test ./internal/handler/session -run '^(TestCraftHTTPViewerReadOnlyAndCrossTenantInvisible|TestCraftHTTPListAndVersionDownload|TestCraftPreviewCapabilityStopsServingAfterPersistentGrantRevocation|TestCraftPreviewIssueRedeemServe|TestCraftPreviewIssueCrossTenantNotFound|TestCraftPreviewExpiredGrantsReturn404|TestCraftPreviewFailsClosedWithoutBrowserNavigationBoundary)$' -count=1 -v` (first attempt) | 1 | Same unrelated package compile error; rerun reached tests and exposed the workspace lookup failure above. |
| `go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction$' -count=1 -v` (first attempt) | 1 | Same unrelated package compile error; rerun passed after the observed edit. |

## Acceptance evidence and gaps

- **B1/B2 and membership:** focused service checks pass for private default, explicit Viewer/Collaborator grants, ungranted Admin/nonmember/cross-tenant denial, list filtering, direct Craft/version/file reads, generic Craft `GetSession`, stale membership/rejoin invalidation and revocation. B5 exercises the authenticated route boundary and zero download opens/bytes on denied reads.
- **B3:** focused persistent handler test confirms a capability stops serving after grant revocation; preview tests cover issue/redeem, expiry, cross-tenant and fail-closed browser boundary. Container test confirms the persistent checker is injected and production preview remains disabled. These do not prove an enabled production preview path.
- **B5 present:** `TestCraftB5JoinedCurrentProduction` uses real `middleware.Auth`, route API-key authorization, migrated SQLite database and `CraftAccessService`. It covers list, generic direct read, version list/detail, file download, grant/revoke/repeat, denied-read audit persistence and detail redaction, anonymous/API-key guards, ungranted Admin, cross-tenant, nonmember, and membership deletion/rejoin. It passed on this exact tree.
- **Still unaccepted:** no single production-authenticated HTTP journey successfully issues a preview ticket, redeems that valid ticket and reads bytes, then revokes and proves the same ticket yields no bytes. Production preview is intentionally gated off; B5 only asserts disabled production issue and a test-gated revoked issue. The test-gated route injects Viewer identity directly and therefore is not production-auth evidence. The focused handler test supplies the valid capability/revoke behavior separately.
- **B1 handler regression signal:** `TestCraftHTTPListAndVersionDownload` fails twice because its HTTP fixture's seeded workspace cannot be resolved. This prevents treating the existing B1 handler list/version/download selector as green. The real B5 integrated test passes the corresponding authenticated list/version/download journey, so this failure is currently limited to the older handler fixture; its cause is not established here.
- **Audit:** B5 proves durable denial audit rows for authenticated content denials and exact successful grant/revoke rows. The approved Spec requires authorization across Craft operations; the test exercises denial audit coverage more broadly than earlier gap-map notes implied. Preview deny audit is asserted in the test-gated route.

## Minimal B5 follow-up and ownership

Add/extend only `internal/router/craft_b5_joined_test.go` to join valid preview issue → valid capability redemption → file-byte read → owner revoke → same capability denial/no bytes, using current persistent access and production route/auth wiring. Keep production preview controls under T14's existing owner; use an explicitly test-gated preview service only if that limitation is stated, and do not present it as production enablement evidence. The API issue request must traverse `middleware.Auth`; the isolated preview-origin redemption should use a valid issued capability and configured host. The integration test owner owns only this test file/fixture. T14/container owners retain production preview gate and networking files. The separate failing legacy handler fixture is owned at `internal/handler/session/craft_test.go` and is not part of B5 ownership.

## Prior evidence consulted

Read approved `docs/specs/2026-09-23-craft-web-artifact-spec.md`; `docs/plans/2026-09-24-craft-107-t08-remaining-gap-map.md`; B1 current validation; B2 Task 1 report and updated independent review; B3 Task 1 report, wiring review and container validation; B5 Task 1 review and fix1 re-review. The updated B2 review supersedes the older B2 review conclusion. Existing reports were treated as historical/scoped evidence; fresh commands above bind the current uncommitted tree.
