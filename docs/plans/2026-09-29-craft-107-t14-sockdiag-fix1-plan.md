# T14 SOCK_DIAG review Fix 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Follow RED -> GREEN -> REFACTOR.

**Goal:** Normalize validated loopback IP literals before SOCK_DIAG tuple comparison so every accepted IPv6 spelling matches the kernel's canonical address representation.

**Source review:** `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-review.md`, finding 2. The original Task 1 implementation and validation are in `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-report.md` and `...-validation.md`.

## Global constraints

- Work only in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`.
- Preserve all existing files and changes. Do not stage, commit, push, stash, run Docker, or edit files outside the two owned source/test files plus this task report.
- Preserve the fixed seven-field flow contract and fail-closed behavior. Do not change ACK policy, parser limits, task interfaces, or live fixture behavior in this fix.
- Commit strategy: uncommitted checkpoint, exact pre/post hashes and task-scoped diff.

## Task 1 — canonicalize accepted IP address spellings

**Dependency:** T14 Task 1 checkpoint has independent Review and backend validation; this repair is narrowly scoped to review finding 2.

**Role:** `backend_implementer`. **Validator:** `backend_validator`. **Independent reviewer:** assigned by parent.

**Owned files:**
- `deploy/craft/render-boundary/policy-helper/helper.py`
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Report: `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix1-report.md`

**Consumes:** the seven-field validated flow contract and exact SOCK_DIAG records whose addresses are canonicalized with `ipaddress.ip_address(...).compressed`.

**Produces:** expected source/destination addresses canonicalized only after the existing matching-family loopback validation; a regression test using an expanded IPv6 loopback spelling and the matching canonical kernel record.

**Steps:**
1. RED: add a parser regression test where `client_addr` and `driver_addr` use `0:0:0:0:0:0:0:1` while the diagnostic record encodes `::1`; demonstrate the current parser rejects the exact record as absent. Keep a test asserting malformed/non-loopback input remains rejected.
2. GREEN: construct `expected_tuple` from the already validated `ipaddress` objects in their compressed form. Do not alter emitted public flow JSON or seven-field contract.
3. REFACTOR: run the targeted IPv4/IPv6 parser selectors, full parser/query-focused selectors, the fixed policy counter class, barrier adapter tests, `py_compile`, and `git diff --check`; no Docker or discovery.
4. Record pre/post source hashes, RED failure and GREEN command results, HEAD and complete task-scoped diff in the report.

**Acceptance:** the expanded IPv6 spelling maps to exactly one canonical `::1` record with matching state, ports and inode; canonical IPv4/IPv6 tests and all prescribed no-Docker checks pass; independent review and backend validation pass against pinned hashes. Preserve the separate live ACK-counter and same-source-port findings for parent acceptance.

**Failure handling:** if canonicalization creates ambiguity, broadens accepted address families, or weakens exact tuple matching, stop and report. Do not edit the live fixture in this fix.
