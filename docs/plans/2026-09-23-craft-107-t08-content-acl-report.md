# T08 central-B B1 Craft Content ACL/List Report

## Checkpoint

- Checkpoint ID: `T08-B1-2026-09-23-checkpoint-1`
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`
- HEAD before/after: `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit created)
- Source manifest: `docs/plans/2026-09-23-craft-107-t08-content-acl-checkpoint.json`
- Source manifest SHA-256: `ffa894ca3bdc33015cd87b88b273efa6b8b1b805c67ace3edfd309c512d70747`
- Full current scoped diff checkpoint: `docs/plans/2026-09-23-craft-107-t08-content-acl-checkpoint.patch`
- Patch SHA-256: `b10054d5140ddcae0cc375fa65cc17c1a6f77f4d31aec3e19abc215cb1ff58ca`
- Agent role: `backend_implementer` (per task assignment). Runtime model and effort are not exposed in this task context.

The checkpoint hashes the complete current content of the seven scoped source files. `craft_session.go` and `container.go` already carried reviewed T01/input-feature changes at B1 start; this checkpoint patch is relative to shared HEAD and therefore also contains those pre-existing T01 hunks. B1 additions are the Task ACL/list ports and focused wiring/tests; no T01 implementation was intentionally changed. Other shared worktree changes are excluded from the owned source manifest and were left untouched.

## Implemented B1 scope

- Inject the current `CraftAccessService` as `craft.TaskAccessChecker` and the bounded Craft-list query in production `newCraftSessionService` wiring.
- Authorize Craft reads (`Get`, `View`, workspace inputs, version metadata and file opening) against current TaskRead before resolving content/storage. Authorize input association and Run start against TaskWrite. The owner scope remains the persistence/run ownership scope after a grantee passes the caller-specific check.
- Make missing-ACL service assemblies owner-only; they do not inherit the generic SessionService tenant-Admin fallback. If ACL is configured but the bounded list port is absent, listing fails closed.
- Add a server-side keyset-paginated query over Craft sessions, current active tenant membership incarnation, and current explicit Viewer/Collaborator grant. It caps query size at 101 rows (100 visible plus next-page sentinel), orders by `(updated_at DESC, id DESC)`, and does not materialize all Task IDs.
- Add focused service coverage for Viewer/Collaborator access, Viewer write denial, current revocation, stale membership after rejoin, same-tenant Admin without grant, nonmember, cross-tenant access, no-reader-on-denial, grant-aware pagination, cursor replay, and provider wiring.

## Approved existing expectation changes

Per the controller's narrow test-ownership amendment, two Craft-specific test files were updated where their old assertions assumed tenant Admin could access Craft content without a Task grant:

- `internal/application/service/craft_session_test.go`, `TestCraftSessionPermissionChains`: old expectation that the Admin Craft `Get` succeeds and input association returns `ErrForbidden`; now both expect `craft.ErrNotFound` in the no-T08-ACL legacy harness.
- `internal/handler/session/craft_test.go`, `TestCraftHTTPViewerReadOnlyAndCrossTenantInvisible`: old Admin Craft view expectation of HTTP 200 and write/Run expectations of HTTP 403; now these expect HTTP 404 in that legacy owner-only harness.

These are Craft-only assertions. No production handler/router code or generic Session Admin behavior was changed in this B1 task.

## TDD and verification evidence

RED was established while adding the ACL tests: they did not compile before `CraftSessionConfig.Access` / `TaskList` and the query implementation were added. After implementing and wiring these ports, the focused ACL/list tests passed. The provider injection test also passed.

Previously passing commands (these results bind the source state at that time, before the final expanded denial-helper matrix and SQL maximum/role predicate edits):

```text
go test ./internal/application/service -run '^TestCraftSessionTaskAccess|^TestCraftSessionListPaginates' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/application/service 3.034s

go test ./internal/container -run '^TestNewCraftSessionServiceReceivesSharedTaskACLPorts$' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/container 1.684s
```

Current-format check after `gofmt -w` on the seven owned source files:

```text
git diff --check
PASS (exit 0)
```

The first recheck found an undefined `err` in the newly added ACL test; this has been repaired by declaring `err` at the stale-membership Admin assertion (and retaining assignment at the earlier assertion in the other test). The current test-file syntax/typecheck proceeds past that error.

The latest focused test reruns now stop on concurrent T05 Craft knowledge source/tests, before B1 tests execute:

```text
go test ./internal/application/service -run '^TestCraftSessionTaskAccess|^TestCraftSessionListPaginates' -count=1
FAIL before tests: `craft_knowledge.go` references `canonicalCraftKnowledgeBaseIDs`, `craftKnowledgeBaseRequestDigest`, `buildPackageForKnowledgeBases`, and `reauthorizeSelectedKnowledgeBases` which are not present on the concurrent source at this checkpoint.

go test ./internal/container -run '^TestNewCraftSessionServiceReceivesSharedTaskACLPorts$' -count=1
PASS: ok github.com/Tencent/WeKnora/internal/container 3.992s (linker emitted a duplicate-library warning)

The immediately preceding shared compile state had instead failed at T19's `agent_run_lifecycle.go:24: undefined: errors`; before that, T05 RunView temporarily lacked `BeginSessionCreate`. Those concurrent errors have moved during cross-lane integration; none of those files were edited here.
```

The current final B1 source has not yet received a successful service ACL/list test run against its exact checkpoint because of the concurrent T05 compile blocker. Rerun focused service ACL/list, Craft service, and handler tests after those files compile; until then B1 is **implemented, not fully verified**. The container provider test passed after the ACL test syntax repair.

## Changed source file hashes

Exact SHA-256 values and byte lengths are recorded in the source manifest. The seven source paths are:

```text
internal/application/service/craft_session.go
internal/application/service/craft_session_acl_test.go
internal/application/service/craft_session_test.go
internal/application/service/craft_task_list_query.go
internal/container/container.go
internal/container/craft_session_provider_test.go
internal/handler/session/craft_test.go
```

No migration, production handler/router, frontend, T05, T14, T19 or RunView source was edited for B1. This report does not claim full T08 or Ticket #107 verification; B2/B3/B5 and the final B1 rerun remain separate gates.
