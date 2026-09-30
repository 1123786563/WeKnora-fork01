# T08 B1 current-state validation

Date: 2026-09-24. Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`. Read-only validation of B1 only at integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34` plus the current shared uncommitted source state. Only this report was added. No source/test edits or child agents.

## Verdict

**DONE_WITH_CONCERNS.** The exact current B1 ACL/list service tests, Craft handler tests, and container provider test pass. Service-seam tests prove grant-aware list/direct Craft/version/file reads and revocation. The HTTP handler tests do not prove the same direct/list/version/download reads for a granted Viewer or Collaborator through production ACL wiring. No B1 test failure was observed. B2 generic Session reads and B3 preview authorization are outside this report.

## Revision and source hashes

`git rev-parse HEAD` → `a5e9195acd6500c085c85d60c852148e7bbbbf34`.

Hashes captured before test execution and rechecked afterward; no differences observed:

```text
7dc59052fefc22a937fd3c77a5844b7b8dca14f6b1a179c89ff2b8938d3e391d  internal/application/service/craft_session.go
a2ad621214e884cc4dc0009dd30b472fb23ae1e5404bc23439c07ce4840e4031  internal/application/service/craft_session_acl_test.go
b9690a1bd0bd5a44a7c13aeae5e3ea268fbd1a161230c5c40740ba8198e410c5  internal/application/service/craft_session_test.go
eadc24be2f15343c273dcb790015ea23507b28ffd7eaeb82351d9d7953399a2d  internal/application/service/craft_task_list_query.go
9be537ce08a4b19d50af47df8740c5f5a441c5a40cc5ed6a7fb4d7bd6f115d84  internal/container/container.go
ec38292300c7b7779b948b40879c3bb0a4243d7131c164dd1106088da142edc6  internal/container/craft_session_provider_test.go
ddcc0fe7fa024c468b8baf8a879178cecf5e90ca538c4ec1d732e98c8a532b48  internal/handler/session/craft_test.go
```

## Evidence

Focused B1 service tests passed:

```text
go test ./internal/application/service -run '^(TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes|TestCraftSessionTaskAccessRejectsStaleMembershipAndAdminWithoutGrant|TestCraftSessionListPaginatesOwnedAndGrantedTasksWithCurrentMembership|TestCraftSessionPermissionChains|TestCraftSessionListCursorFollowsExistingScope|TestCraftSessionVersionsAndDownloadChain)$' -count=1 -v
PASS
```

These cover Viewer/Collaborator grant reads (`Get`, `View`, `ListVersions`, `GetVersion`, `OpenVersionFile`), TaskWrite versus Viewer denial, owner-private Task denial for Admin/nonmember/cross-tenant identities, zero file-reader opens on denied access, current revocation, stale membership after rejoin, list filtering/pagination, and version/download behavior. The list test verifies owner plus current grant results, immediate omission after revoke, empty Admin list without a grant, and no cross-tenant Task IDs.

Focused handler and production provider tests passed:

```text
go test ./internal/handler/session -run '^(TestCraftHTTPViewerReadOnlyAndCrossTenantInvisible|TestCraftHTTPListAndVersionDownload|TestCraftHTTPOwnerCreatesUploadsRunsTwoRounds)$' -count=1 -v
PASS

go test ./internal/container -run '^TestNewCraftSessionServiceReceivesSharedTaskACLPorts$' -count=1 -v
PASS (linker emitted duplicate `-lc++` warning)
```

Handler coverage asserts owner direct Craft read and list/version/download success, ungranted same-tenant Viewer direct Craft/version denial, Admin denial in the owner-only compatibility fixture, and cross-tenant invisibility. `TestNewCraftSessionServiceReceivesSharedTaskACLPorts` confirms production provider injection of persistent ACL and list ports.

## Current B1 gap

`TestCraftSessionTaskAccessGatesContentWritesAndDownloadBytes` proves grantee direct Craft/view/version/file reads at the service boundary. `TestCraftSessionListPaginatesOwnedAndGrantedTasksWithCurrentMembership` proves grant-aware list behavior at that boundary. The handler fixture in `internal/handler/session/craft_test.go` uses a Craft service without Task ACL wiring (`newCraftHTTPEnv`); its tests establish owner success and ungranted Viewer/Admin/cross-tenant denial, but do not grant access and then exercise the actual authenticated HTTP list, direct Craft, version metadata, and download routes. Thus the missing proof is a **granted-role HTTP journey across these reads and current revocation**, not missing service-level B1 coverage.

No direct failure was observed in the tested current B1 source. The worktree has extensive unrelated concurrent uncommitted changes; HEAD is the baseline only. The earlier `2026-09-23-craft-107-t08-content-acl-report.md` and `t08-content-acl-review.md` were reviewed as background; their historical compile blocker does not reproduce in the focused current service run. This validation does not decide B2/B3 or the final B5 journey.
