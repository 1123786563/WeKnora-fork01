# H1 RunView T05 package report

## Implementation

Added a narrow RunView knowledge builder in `internal/container/craft_knowledge_runview.go`. It strictly parses the admitted Run snapshot, requires the Craft manifest and immutable knowledge selection, binds ACL and source-record scope to the durable actor, and checks the opaque material handle against tenant, owner, session and Run. A small store adapter resolves only the persisted owner-bound Workspace and then presents the verified actor scope to the existing T05 service.

The builder creates one publisher per admitted Run from the verified handle's generation root, passes that exact publisher to the T05 service, loads the published record by actor scope and verifies the sealed package through the same publisher's `Resume` method. The accepted result contains only RunID and package digest. Direct Workspace writes are disabled in this path.

T05's KB-selection service previously rejected a zero-length KB selection. H1 requires this to produce a durable empty package. Added a zero-KB branch that validates the retrieval query and enters the existing Run record, TaskWrite, Prepare/Save/Publish/MarkPublished flow with an empty manifest. It does not call the search or document-access ports. Same-query empty replay resumes the immutable package without search; changed query conflicts; current TaskWrite is still required.

## TDD and verification

- RED: `go test ./internal/application/service -run '^TestCraftT05BuildForKnowledgeBasesExplicitEmptySelectionPublishesWithoutSearch$' -count=1` failed as expected with `craft invalid input: knowledge-base selection must contain 1 to 20 IDs`.
- GREEN: the same service test command — PASS (`ok`, 1.011s).
- Focused adapter tests: `go test ./internal/container -run '^TestCraftKnowledgeRunViewBuilder' -count=1` — PASS (`ok`, 1.700s; linker emitted the existing duplicate `-lc++` library warning).
- Final focused checks: `go test ./internal/application/service -run '^TestCraftT05BuildForKnowledgeBasesExplicitEmptySelectionPublishesWithoutSearch$' -count=1 && go test ./internal/container -run '^TestCraftKnowledgeRunViewBuilder' -count=1 && git diff --check -- internal/application/service/craft_knowledge.go internal/application/service/craft_knowledge_t05_test.go internal/container/craft_knowledge_runview.go internal/container/craft_knowledge_runview_test.go` — PASS; both Go commands passed and `git diff --check` emitted no output.
- `gofmt -w` ran on the four owned Go files before the final focused run.
- Broader package command `go test ./internal/application/service ./internal/container` — FAIL after 228.299s. Failures include `TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery` (missing Workspace seed), `TestAgentRunWorkerTickSkipsPaseo` / `TestAgentRunWorkerRemoteDispatchUsesDurableIntent` (agent runtime conflict), `TestCraftAccessFeatureRegistriesAreAssemblyOwned` (missing injected TaskAccessChecker), and `TestWireCraftInteractionRegistrarRegistersPendingInteractions` (agent runtime conflict). These exercise other unowned integration wiring and files; the scoped H1 tests pass. This failure is recorded, not attributed to H1.

Focused coverage proves: durable original query and KB selection are used; actor scope differs safely from Workspace owner scope; conflicting caller and foreign Run handle deny before search; selected KB and document ACL revocation deny; empty selection publishes a one-manifest package without search; empty replay preserves the same package and digest without search; changed query or KB selection conflicts before search; and current TaskWrite is checked on replay.

## Scope and checkpoint

Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`; HEAD remains `a5e9195acd6500c085c85d60c852148e7bbbbf34`; no commit created. The task-local patch contains only this task's four Go-file changes, excluding the pre-existing integration edits. `craft_knowledge.go` and `craft_knowledge_t05_test.go` already contained earlier uncommitted T05 work at assignment; the preimage folder reconstructs their state by removing only this task's additions. The new adapter files were absent at assignment.

See `2026-09-24-craft-107-t05-r3-h1-package-checkpoint.json` and `2026-09-24-craft-107-t05-r3-h1-package-task-local.patch` for hashes and the exact task delta. No publisher defect was found; publisher files were not changed. No runtime or central assembly wiring was changed, so production execution remains fail-closed pending H2/H3.
