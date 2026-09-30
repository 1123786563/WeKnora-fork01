# T01 RunView R3 Task 2 — four material-boundary corrections

> **For Codex:** Execute this focused SDD fix with RED→GREEN→REFACTOR, exact uncommitted before/after checkpoint, independent Spec and quality review. No commit.

**Sources:** `2026-09-24-craft-107-runview-r3-material-plan.md`, Task2 report/checkpoint, independent `2026-09-24-craft-107-runview-r3-material-task2-review.md`, approved Spec #107/T01. Keep R3/T01 unverified until findings close. Integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`; capture live HEAD and full owned-file preimage/diff/staged/unstaged before edit.

## Global Constraints

Only the admitted RunView generation can supply the input tree. Empty inputs still require current workspace association. Every selected ref must be verified or an ambiguous duplicate rejected. Newly staged and retried files have canonical non-executable mode; live mount `noexec` remains a separate release gate. Concurrent publication must never overwrite another object or roll it back by path alone.

## Review Focus

The four Medium findings, Linux/Darwin atomic no-replace behavior, inode-safe rollback, no weakening of digest/association/path checks, exact task-local delta, unchanged fail-closed runtime assembly.

## Task 1 — close R3 Task2 findings

**Depends on:** Task2 failed independent review. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/container/craft_runtime.go`, `internal/container/craft_runtime_test.go` only. Other container, preview, access, T05, gateway, migrations, and Docker files are read-only. **Consumes:** existing `StageWorkspaceInputs`/material handle; **Produces:** exact authorized non-executable input tree under this generation.

1. RED tests: revoked/mismatched workspace with empty manifest fails; distinct selected refs with identical canonical path fail before any FileService read/material, or both are explicitly verified; 0755 existing matching bytes fails retry; deterministic race creates target after preflight and publication refuses without overwriting it, including rollback preservation.
2. Move current workspace association before empty return. Reject distinct-ref duplicate canonical path as ambiguous (simplest fail-closed policy) while preserving exact same-ref idempotent retry. Require regular single-link mode exactly 0644 for inspected staged input files.
3. Replace ordinary `Renameat` with a same-directory atomic no-replace primitive available on the target OS; a hard-link publish plus unlink temporary name is acceptable if link count/temp cleanup and fsync are correct. Track actual published file identity and roll back only that inode, never a replacement created by another writer. Preserve no-follow path walking and full tree validation.
4. Run focused normal/race container tests, `gofmt`, `git diff --check`. Save full before/after owned-file contents and hashes, task-local delta patch (not cumulative HEAD patch), commands/exits, review package and report. Independent reviewer must recheck all four findings.

**Acceptance:** no selected or empty manifest bypass, no overwrite/loss on concurrent publication, no unverified ref alias, and no executable retry file. **Failure handling:** if a portable no-replace operation cannot be proven on Linux and Darwin, fail closed and report exact platform gap; do not retain ordinary `Renameat`.
