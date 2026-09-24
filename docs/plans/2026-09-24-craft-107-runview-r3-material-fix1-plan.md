# T01 R3 material handle directory identity fix 1

> **For Codex:** Repair scoped M1 and L1 from `2026-09-24-craft-107-runview-r3-material-task1-review.md` with TDD, exact uncommitted checkpoint and independent re-review. No production enablement.

**Source:** approved Craft Spec #107/T01, R3 material Task1 plan/report/review. Record current HEAD and pre-task owned-file content/hashes. No commits.

## Global Constraints

Bound verification is read-only: it must not create a missing root or child directory. Host paths derive from the persisted generation; same pathname with a fresh directory inode is not proof of the container's existing bind mount. On provider restart, durable layout identity must remain verifiable without relying solely on in-memory binding. Missing/altered identity fails closed; no host home/source/symlink acceptance. Fake-engine proof does not replace Linux mount inspection.

## Review Focus

Missing child, replaced child at same path, replaced root, stale or tampered identity manifest, provider restart with same generation, creation before Docker call, exact GET-by-ID error/mismatch, no second create after unknown.

## Task 1 — persist and verify generation layout identity

**Depends on:** R3 Task1 scoped review FAIL M1/L1. **Owner:** backend_implementer. **Validator:** backend_validator. **Owned files:** `internal/container/craft_runview_container_provider.go`, `craft_runview_material.go`, `craft_runview_container_provider_test.go`, optional new focused material test file. **Consumes:** deterministic generation host layout and durable create marker. **Produces:** immutable root/child filesystem identity recorded before first container Create, rechecked on every bound MaterialHandle and recovery.

1. RED tests: after initial valid bind, rename/replace `inputs`, `knowledge`, `output` or root at identical path; `MaterialHandle` must return unresolved without recreating. Missing child/root and manifest tampering fail; provider restart with intact layout succeeds; exact GET missing/error/altered ID/project/directory fails closed.
2. Separate creation from read-only bound verification. Record root and mount-child device/inode identities in a root-owned durable manifest (or equivalently strong persisted evidence) before engine Create; write atomically and fsync. On bound resolution, reject missing identity/dirs/symlinks and compare identities before handing out paths. Do not infer live mount identity from source pathname alone; retain full engine reinspection and probe. If filesystem identity cannot be trusted for the configured volume, fail closed.
3. Focused provider/material tests and focused race, format/diff checks; save exact pre/post hashes/task-local patch/report.

**Acceptance:** replacement at same pathname never yields a usable material handle, including after provider restart. **Failure handling:** if a durable manifest cannot be safely added under current create flow, report the exact lifecycle seam before modifying unrelated store/engine files.
