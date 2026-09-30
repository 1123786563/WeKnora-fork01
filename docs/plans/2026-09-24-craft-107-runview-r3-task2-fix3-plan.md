# T01 RunView R3 Task2 fix3 — atomic no-replace publication

> **For Codex:** Execute this SDD repair with RED→GREEN→REFACTOR, exact uncommitted checkpoint and independent Spec/quality review. No commit.

**Sources:** approved Spec #107/T01, R3 Task2 original/fix1/fix2 reviews, and detailed verified API/test design `2026-09-24-craft-107-runview-r3-task2-fix3-design.md`. The design's publication sequence, deterministic RED interleaving, digest-directory failure table, file ownership and verification commands are normative for this task. Capture live HEAD and complete owned-file preimages/absence, staged/unstaged/untracked before edits.

## Global Constraints

No Linkat+Unlinkat, ordinary rename fallback, or pathname cleanup of temporary/final/digest directory on success or error. Failed/dirty generation stays fail closed before Prompt. Keep current workspace association, duplicate-ref, mode, digest, no-follow and full-tree checks. Linux and Darwin use the exact no-replace APIs verified in the design; other OS fail unsupported. Do not enable runtime or claim live Linux mount isolation.

## Review Focus

Temporary-name replacement after publication, target no-overwrite, prepublication source swap and identity recheck, original syscall error preservation, real created-directory Openat/Fchmod/Fsync/Fstat failure injection, retained-empty-directory retry, Linux/Darwin wrapper compile and native behavior, exact task-local patch.

## Task 1 — close remaining Medium and Low findings

**Depends on:** fix2 independent FAIL review and fresh-context systematic-debugging design. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/container/craft_runtime.go`, `craft_runtime_test.go`, new `craft_input_publish_linux.go`, `_darwin.go`, `_unsupported.go`, optional `craft_input_publish_test.go`, and task report/checkpoint. No other source edits. **Consumes:** existing verified RunView material handle and admitted selected manifest. **Produces:** atomic no-replace publish and fail-closed retained material.

1. Add deterministic behavioral RED tests exactly as design: replacement temporary entry survives old cleanup; digest-directory faults preserve primary errors and entries; no Prompt after staging error. Capture failing assertion, not only compile errors.
2. Replace publication with platform no-replace wrapper; retain open descriptor identity and final full-tree verification; remove all pathname cleanup. Split Fstat syscall error from canonical-mode conflict, and use per-call operation hooks for real failure injection. Keep unsupported failure explicit.
3. Run design's focused normal/race suites, wrapper cross-compiles, native primitive tests, `gofmt`, `git diff --check`; record unavailable Linux runtime evidence honestly. Save exact pre/post content/hashes, task-local patch and test logs, then independent review before downstream R3/H1/R4 work.

**Acceptance:** no concurrent replacement deletion; no target overwrite or unsafe fallback; retained dirty material never reaches Prompt; syscall root cause preserved. **Failure handling:** on unavailable platform primitive or source identity mismatch, return error and retain generation rather than cleanup by path.
