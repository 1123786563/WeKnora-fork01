# T05 Atomic Material Publisher Report

**Plan:** `docs/plans/2026-09-23-craft-107-t05-publisher-plan.md`  
**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01`  
**Checkpoint:** `t05-publisher-uncommitted-01`  
**HEAD:** `a5e9195acd6500c085c85d60c852148e7bbbbf34` (unchanged; no commit created)  
**Role metadata:** Runtime did not expose role/model/reasoning metadata.

## Scope and implementation

Added a filesystem adapter in `internal/container/craft_knowledge_publisher.go` and focused tests in `internal/container/craft_knowledge_publisher_test.go`. The constructor binds a canonical server-owned root and one admitted Run ID. Prepare seals a digest-verified complete candidate in a private sibling directory, outside the RunView root. Resume verifies the seal and content and can recover a prepared or published package after adapter reconstruction. Publish rejects changed package digests, exposes only a complete payload using a same-filesystem directory rename, makes material files read-only, seals the final directory, and syncs relevant files/directories. Discard removes only the matching private candidate and refuses published content. Paths, symlinks, source/package counts, and byte limits are validated.

The first behavior RED after the adapter compiled showed that this macOS filesystem rejects renaming a sealed 0555 source directory (`permission denied`); a minimal reproduction confirmed the filesystem behavior. The implementation now syncs the complete payload with all files 0444, performs the atomic rename while the payload directory is 0700, and immediately changes the published directory to 0555. This keeps visibility atomic and the content read-only throughout publication; directory write permission is removed before Publish returns. The test helper restores temporary tree permissions during cleanup, and the failed-write assertion checks that no staged digest candidate remains (an empty per-Run private parent may remain).

## RED → GREEN evidence

- Initial RED: the new tests could not compile because `NewCraftKnowledgePackagePublisher` did not exist.
- Behavior RED after adding the adapter: focused tests found the macOS 0555-directory rename failure and the failed-write test's overly broad assertion about the empty private root; temp cleanup also correctly exposed that read-only test artifacts need permission restoration.
- GREEN focused: `go test internal/container/craft_knowledge_publisher.go internal/container/craft_knowledge_publisher_test.go -run CraftKnowledgePublisher -count=1 -v` — PASS, all six Publisher tests.
- GREEN package: `go test ./internal/container -run CraftKnowledgePublisher -count=1` — PASS. An earlier attempt failed at compile time because a concurrent `craft_input_wiring_test.go` referenced the not-yet-landed `wireCraftInputFeature`; the same command passed after that disjoint shared integration edit landed.
- Diff hygiene: `git diff --check -- internal/container/craft_knowledge_publisher.go internal/container/craft_knowledge_publisher_test.go` — PASS.

The six behavior tests cover private prepare/restart resume, complete read-only publish/idempotent replay/digest conflict, failed write/rename and recovery, symlink/traversal rejection, limits and missing resume, and isolation between distinct roots/Runs.

## Checkpoint evidence

The two owned Go files were absent before this task and are untracked at this checkpoint. Shared changes in the Worktree were preserved. The repo HEAD remained `a5e9195acd6500c085c85d60c852148e7bbbbf34` throughout this task.

SHA-256:

- `internal/container/craft_knowledge_publisher.go`: `bd40ecd810aad81d84af45fe8319f75cbae546303c98feadbb76ae0764a1f48c`
- `internal/container/craft_knowledge_publisher_test.go`: `e72f629f5217d6ceed0bf51c4b57175c41483e241533ec501242c4be71a09514`

## Remaining integration gates and risks

This task does not wire the adapter into production. T05 is not production-verified until central integration binds the publisher root to an isolated per-Run RunView, binds accepted Run/package metadata and assembly, and rechecks dispatch authorization. The central per-Run sandbox read boundary remains a separate gate. The current filesystem requires a short interval after atomic rename and before directory chmod where the complete directory is writable; all package files are already read-only in that interval. If delegate execution can race package publication on the same root, central integration must prevent delegate visibility until Publish returns or provide a filesystem primitive/ACL that seals the directory before it becomes visible.
