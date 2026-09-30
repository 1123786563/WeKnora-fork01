# T08 B5 Task 1 report: current-production joined HTTP denial journey

## Result

Added a test-only joined journey at the production `registerSessionRoutes` assembly seam. It uses production `Viewer` and API-key route guards and the production API-key authorizer, with a test credential-verifier adapter that supplies authenticated context. The test uses a fully migrated fresh SQLite database, a persistent `CraftAccessService`, a real `SessionService.GetSession`, the real Craft session/version stores and service, and a configured preview service whose production browser/network gates remain closed.

No production source or existing test was changed. No commit was made. Task 1 is verified as a current-production subset; this is not full B5/T08 acceptance.

## Changed files

- `internal/router/craft_b5_joined_test.go` — one new router test and local migrated DB/seed helpers.
- `docs/plans/2026-09-24-craft-107-t08-b5-task1-report.md` — this report.
- `docs/plans/2026-09-24-craft-107-t08-b5-task1-checkpoint.json` — exact owned-scope status, HEAD, validation and hashes.
- `docs/plans/2026-09-24-craft-107-t08-b5-task1-task-local.patch` — complete task-local new-file patch.

## Coverage and evidence

- Seeds tenant A owner, Viewer, Collaborator, ordinary Admin and a nonmember, plus tenant B actor; seeds a registered Craft Task, persistent workspace and immutable version with known HTML bytes.
- Before grants, after owner revocation and after removing/rejoining the Viewer with a new membership ID, asserts list responses omit the Task, direct Session/version metadata and download routes follow their actual 403/404 contract, identifying fields and pinned bytes are absent, and the instrumented file opener count does not increase. A Viewer access mutation is denied without persisting a grant. A fresh explicit owner grant restores access after rejoin.
- Grants Viewer and Collaborator over the HTTP access route, then traverses Craft list → generic `GET /sessions/:id` → version list → version metadata → exact pinned file download. The owner still reads the version after the revocations.
- Confirms ordinary tenant Admin and tenant B actor do not gain Craft access; anonymous requests return 401; API keys without chat capability return 403; a chat-capable API key without a Task grant cannot download bytes or open storage.
- Compares non-Craft direct Session behavior: the ordinary owner reads their Session while another Admin cannot open that personal web Session; the Admin+ API-key Session policy remains intact and Viewer remains denied.
- The configured production preview issue route returns unsupported/no ticket for actors while `BrowserNavigationProtected` is false and `NetworkChecker` is nil. Isolated `/p/` traffic is tested before the test credential adapter, matching production registration order, and returns 404 with no bytes/storage open. This is disabled-state evidence only; it does not prove successful preview/browser isolation.
- Grant/revoke mutations persist six success audit rows attributed to owner, tenant, Task scope and targets. The migrated DB has zero audit rows after denied reads before the first access mutation. The test logs that observation rather than inventing an action name or asserting a denial-audit contract: the issue snapshot says authorization is audited, while the current service persists only grant/revoke success and the approved action name for denial events remains unresolved.

## TDD / verification record

The first selector attempt exposed a test-fixture `NewHandler` arity error; after correcting it, the next exposed missing file digest in the version seed. A later run was briefly blocked by the concurrent S1 edit referencing `craftDraftHeadRow` while its new file was being written; no S1 source was changed. Once that shared compile issue stabilized, route runs exposed three fixture/contract mismatches (the error handler/auth edge, isolated `/p/` registration order, and SQLite's unique membership index). Those were corrected in the test fixture. There was no production behavior fix in this test-only task. The current container selector rerun is pending the concurrent R3 fix2 checkpoint; its owner file was left untouched.

Final targeted commands on HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`:

```text
go test ./internal/router -run '^TestCraftB5JoinedCurrentProduction|^TestCraftAccessProductionSessionAssemblyAuthAndAudit$' -count=1
PASS — ok github.com/Tencent/WeKnora/internal/router 1.437s

go test ./internal/application/service -run '^TestGetSession|^TestCraftT08Journey$' -count=1
PASS — ok github.com/Tencent/WeKnora/internal/application/service 1.052s

go test ./internal/container -run '^TestCraftPreviewConstructorUsesCurrentPersistentTaskAccessAndStaysDisabled$' -count=1
PASS on the earlier stable shared-source checkpoint — ok github.com/Tencent/WeKnora/internal/container 1.852s (linker warned about duplicate -lc++). The latest rerun is pending: concurrent R3 fix2 temporarily made `craft_runtime_test.go:418` reference undefined `openOrCreateRunViewInputDigestDirWithAfterCreate`.

go test ./internal/handler/session -run '^TestCraftPreview' -count=1
PASS — ok github.com/Tencent/WeKnora/internal/handler/session 1.024s

git diff --check -- internal/router/craft_b5_joined_test.go
PASS
```

Source SHA-256: `d8eb9ee65a991d64c08f51332a2cdcc9731bc3df349cdfd0a5af3f357cddbbbe`  
Task-local patch SHA-256: `9a3dfe226bcd64739360ca6277b5ab3f448d5b30a2466d58444eceb2c172ef75`

## Limits / remaining risks

- Independent validation and review are pending the parent orchestration.
- The test deliberately verifies the disabled preview contract only. Successful ticket redemption, live browser navigation isolation and no-egress proof remain outside Task 1 and depend on reviewed T14 gates.
- Denial-audit acceptance remains unresolved. The observed zero pre-mutation audit rows is recorded; no claim is made that full denial auditing is satisfied.
- No full repository test suite was run; the assigned verification is the targeted router, Session/access, preview constructor/handler selectors above.
