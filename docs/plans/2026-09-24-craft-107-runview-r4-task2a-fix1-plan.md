# Craft #107 T01 R4 Task 2a Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the two Medium gaps in the Run-bound output source before Task 2b consumes it.

**Architecture:** Bound directory entry enumeration before allocating untrusted names, and keep a directory-set generation fingerprint from list through each read. A concurrent tree mutation must either be caught by kernel-backed directory metadata or make the source unavailable; an unprovable live snapshot may never be presented as complete.

**Tech Stack:** Go, Linux `openat`/`fstatat`/`ReadDir`, focused deterministic and race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r4-task2a-review.md`; `docs/plans/2026-09-24-craft-107-runview-r4-task2a-plan.md`; approved Craft #107 W01/T01.

## Global Constraints

- Run, generation, tenant/Workspace/actor and root identity remain bound at every list/read; no session-wide collector, shared pointer, direct publish or draft advance.
- Reject over-limit files **and directories**, including zero-byte entries, before unbounded `ReadDir(-1)` materialization. Preserve the approved path/byte limits and exact error classes.
- A file or directory inserted, removed or renamed during a successful list or before/during any read must not silently disappear from the returned manifest. If kernel metadata cannot prove a stable entry set, fail closed and document the required quiescence seam for Task2b.
- No commit or production route. Exact task-local pre/post checkpoint, focused/race tests and independent review.

## Review Focus

- F1 unbounded directory enumeration and total entry count; nested empty directories and zero-byte files consume a limit.
- F2 extra entry insertion between enumeration, final list and read; changes to root and nested directories fail even if previously listed files are unchanged.
- Existing no-follow, hard-link, inode and content checks stay intact.

---

### Task 1: Bound and seal the Run-bound output listing

**Depends on:** Task2a exact-checkpoint independent review FAIL. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runtime.go`, `internal/container/craft_runtime_r4_run_source_test.go` only. Coordinate with any T01 assembly owner before package runs.

**Consumes / produces:** Existing `runBoundCraftArtifactSource` and per-Run verified root; produces bounded enumeration and fail-closed directory-set validation.

- [ ] Capture two-file preimage. RED tests for more than the chosen documented global entry cap using zero-byte files and empty directories, plus per-directory over-cap before unbounded read. RED deterministic hook inserts an extra file immediately after a directory is enumerated and another after `ListSessionFiles` but before a listed file `ReadSessionFile`; both must reject. Include nested insertion and remove/rename cases.
- [ ] Use finite `ReadDir(limit+1)` or equivalent streaming count with a global entry budget; reject overflow without retaining all names. Keep depth/path caps. Add directory metadata fingerprint (device/inode/mode and change timestamps or stronger verified generation) before and after enumeration and validate across subsequent reads. Revalidate every listed directory, including root, before accepting bytes. If timestamp granularity or concurrent semantics cannot establish one snapshot, add a quiescence prerequisite and return unresolved rather than success.
- [ ] Run focused `TestCraftRunViewR4` and race tests, compile-only, `gofmt -d`, `git diff --check`. Save exact checkpoint, tests and incremental patch; request independent review.

**Acceptance / failure handling:** Both findings closed without false complete listing or runaway allocation. Full Task2b and real Docker acceptance remain gated separately.
