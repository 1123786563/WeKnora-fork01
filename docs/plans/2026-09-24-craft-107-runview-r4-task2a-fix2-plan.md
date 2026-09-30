# Craft #107 T01 R4 Task 2a Fix 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the remaining Medium sibling-directory drift gap in the Run-bound output source.

**Architecture:** Record every visited directory's no-follow identity and entry-set fingerprint, then revalidate the **whole** visited directory set before successful List and before/after each Read, including siblings unrelated to the requested file. This detects observed concurrent mutations; Task2b must still hold an authoritative terminal/quiescent writer fence for a consistent capture.

**Tech Stack:** Go, Linux directory descriptors/stat, deterministic mutation hooks, focused/race tests.

**Spec:** `docs/plans/2026-09-24-craft-107-runview-r4-task2a-fix1-review.md`; approved Craft #107 T01/W01; R4 Task2b architecture.

## Global Constraints

- Keep 1024 per-directory and 4096 total entry limits before unbounded allocation. Preserve no-follow, regular-file, hard-link, Run/generation and byte validation.
- A mutation in any listed root/nested/sibling directory must cause List or Read to reject, even when the requested file's own path is unchanged. No unbounded second traversal.
- Do not claim POSIX enumeration alone produces a stable recursive snapshot. Candidate/draft consumer must enforce terminal quiescence across collection; production route stays default-off.
- No commit/push; exact incremental pre/post checkpoint and independent review.

## Review Focus

- Nested directory visited first changes during later sibling traversal: List fails.
- Sibling nested directory changes between List and reading a different file: Read fails before accepting bytes.
- Insert/remove/rename and same-name replacement are covered, with bounded work under adversarial entry counts.

---

### Task 1: Revalidate the entire listed directory set

**Depends on:** Fix1 independent review FAIL Medium. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

**Owned files:** `internal/container/craft_runtime.go`, `internal/container/craft_runtime_r4_run_source_test.go` only.

**Consumes / produces:** Fix1 bounded Run-bound source; produces whole-set drift detection at List/Read boundaries.

- [ ] Capture exact Fix1 two-file preimage. RED hook mutates already-visited nested directory while walker is in later sibling; another hook mutates an unrelated sibling after List and before reading an unchanged file. Assert both currently succeed incorrectly, then fail after fix. Include insert/remove/rename.
- [ ] Retain a bounded directory identity/fingerprint map. Before successful List and before/after Read, re-open/re-stat every listed directory via verified no-follow path and compare identity and entry-set facts. Reject mutation, missing/foreign directory or budget overrun. Return no partial success.
- [ ] Run `^TestCraftRunViewR4` normal and race suites, package compile-only, format/diff. Save exact reconstructible incremental patch, hashes/report; independent reviewer verifies both findings.

**Acceptance / failure handling:** No successful list/read with changed sibling set. The consumer still needs a terminal/quiescent fence before durable candidate/draft collection.
