# T01 R3 Task2 fix2: never remove a concurrent replacement

> **For Codex:** Execute the one remaining Medium finding and Low cleanup finding with RED→GREEN→REFACTOR, exact uncommitted checkpoint and independent review. No commit.

**Sources:** `2026-09-24-craft-107-runview-r3-task2-fix1-review.md`, fix1 plan/report and approved Spec #107/T01. Current integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`; capture live HEAD and full preimage of owned files before editing.

## Global Constraints

Do not unlink a final input path after a separately timed identity check: no pathname unlink can guarantee it still refers to the checked inode under concurrent replacement. Preserve atomic `Linkat` no-replace publication, current workspace association (including empty manifest), duplicate-ref rejection, 0644 mode, no-follow reads, digest/full-tree checks and fail-closed runtime assembly. Any partial generation after failed staging must be unusable until exact complete manifest validation or a new generation is allocated; availability cleanup cannot delete another actor's content.

## Review Focus

Deterministic interleaving where another writer swaps file or directory after failed publication; no unlink of replacement. Dirty-generation treatment on failure/retry, temp-only safe cleanup, no partial prompt, original error preservation, no leaked created-directory handle. Exact fix-only pre/post patch and focused race tests.

## Task 1 — safe failure cleanup

**Depends on:** Fix1 independent FAIL review. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/container/craft_runtime.go`, `internal/container/craft_runtime_test.go` only. **Consumes:** current selected manifest and verified generation handle. **Produces:** no concurrent replacement deletion, fail-closed partial state.

1. RED test: force a failure after one published file, replace that path or containing directory before cleanup, and assert replacement remains; staging fails and no execution/prompt runs. Test incomplete prior generation is rejected or completed only from exact admitted manifest after reauthorization/full validation. Force created digest-directory helper failure and assert original error and no orphaned directory.
2. Remove final-path rollback that uses `Fstatat` then `Unlinkat`; safest narrow behavior may leave published files in the private generation on failure, mark/handle it as incomplete, and require full manifest verification before reuse. Clean only a uniquely created temp name while its own inode is still controlled, or leave it fail-closed if deletion cannot be atomic. Do not weaken no-replace publication or adopt a shared root. Fix created-dir error return so no `created=true, fd=-1` path masks the primary error/leaks directory.
3. Run focused normal/race tests, gofmt and `git diff --check`; save exact pre/post contents/hashes/task-local patch, commands and report. Independent reviewer verifies Medium and Low findings with no new security regression.

**Acceptance:** no concurrent replacement can be removed by cleanup, partial material never reaches Prompt, and error cleanup preserves the root cause. **Failure handling:** if a cleanup race cannot be eliminated, retain dirty generation and fail closed rather than remove by pathname.
