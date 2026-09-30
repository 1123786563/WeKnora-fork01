# Craft #107 T19 E0 Runner Cleanup Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close independent runner Review RI-2 so failed Docker cleanup cannot be labeled a passing source-boundary smoke.

**Architecture:** For each run-owned container/network/volume, require a successful removal or a specific already-absent response, then a separately successful not-found inspect with exact daemon error classification. Timeout, daemon unavailable, permission denied or unexpected inspect error leaves `source_boundary_smoke=BLOCKED` and records the resource ID for cleanup.

**Tech Stack:** Python Docker fixture runner, fake CLI and disposable smoke tests.

**Spec:** Approved Craft #107 T19; `2026-09-24-craft-107-t19-e0-runner-source-integration-task1-review.md` RI-2. RI-1 image/mount/UID contradiction is separate and remains blocked.

## Global Constraints

- Own only `docs/testing/craft/egress-probe/run_v2.py` and focused runner tests. No verifier/source-recorder edits, no paid egress or full E0 PASS claim.
- Do not erase raw cleanup failures or treat any nonzero inspect as absence. No commit/push/stash; exact incremental checkpoint.

## Review Focus

- Failed rm + daemon/permission inspect error is BLOCKED, retains named resource and raw outputs.
- Successful rm + exact not-found inspect can pass; already absent must be distinguished from generic error.
- Network/volume/container cleanup all use the same fail-closed semantics, including CLI timeout and partial cleanup.

### Task 1: Exact cleanup result classification

**Depends on:** Runner Task1 independent FAIL review. **Owner:** `backend_implementer`; **validator:** `backend_validator`, independent `reviewer`.

- [ ] Capture preimage. RED fake CLI tests for failed removal/daemon error, timeout, permission, unrelated missing resource and exact not-found.
- [ ] Implement bounded classification and record exact stdout/stderr/exit/timeout for each cleanup resource. Keep smoke verdict BLOCKED on any unresolved cleanup.
- [ ] Focused runner tests, one disposable no-egress smoke if resource slot available, py_compile, exact patch/checkpoint/report and independent review.

**Acceptance / failure handling:** Cleanup result is trustworthy for the scoped smoke; RI-1 and full E0 remain blocked.
