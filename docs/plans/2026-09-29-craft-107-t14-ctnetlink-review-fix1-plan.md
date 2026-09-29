# T14 ctnetlink parser review-fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Correct the ctnetlink response parser so valid kernel `NLMSG_DONE` framing is accepted and all `NLMSG_ERROR` messages fail closed.

**Architecture:** Keep the current bounded netlink query and exact tuple policy unchanged. Tighten response framing in `helper.py` and pin both cases with raw message fixtures in the existing parser unit tests.

**Tech Stack:** Python 3.12 standard library sockets/struct; unittest.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`, stories 12 and 14; implementation requirements in `docs/plans/2026-09-29-craft-107-t14-ctnetlink-plan.md`; review findings in `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review.md`.

## Global Constraints

- Preserve the renderer's `NetworkMode=none`, read-only filesystem, nonroot UID 10001, `CapDrop=ALL`, and `no-new-privileges`.
- Preserve the one-shot helper's existing exact renderer network namespace and only `CAP_NET_ADMIN` privilege.
- Missing, malformed, interrupted, truncated, late, ambiguous, or error responses must fail closed before policy install.
- Keep netlink query time, datagram size, total bytes, record count, and message count bounded.
- Do not add dependencies, capabilities, wildcard rules, commits, staging, pushes, or shared stash.

## Review Focus

1. Kernel `NLMSG_DONE` carrying a 4-byte status payload (20-byte message total) must be accepted only when status is zero; test a realistic 20-byte completion and reject nonzero completion status.
2. Every `NLMSG_ERROR`, including zero-error ACK payloads, must be rejected because this dump protocol requires `NLMSG_DONE`; test zero and nonzero error payloads.
3. Preserve sender/sequence checks, dump completion requirement, interruption/truncation rejection, exact single tuple match, and existing parser bounds; existing focused parser selector covers these invariants.

---

### Task 1: Fix kernel completion/error framing

**Dependencies:** T00 and T14 ctnetlink reader implementation complete; initial independent Review found HIGH and MEDIUM findings. **Role:** backend_implementer. **Validator:** backend_validator. **Independent reviewer:** parent dispatch after implementation. **Commit strategy:** uncommitted checkpoint; preserve the existing T14 Worktree and make no commit or stage operation.

**Files:**

- Modify: `deploy/craft/render-boundary/policy-helper/helper.py`
- Test: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Report: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix1-report.md`

**Consumes:** `parse_ctnetlink_dump(flow, messages, *, sequence, now=time.monotonic)` and existing raw netlink message fixtures.

**Produces:** accepts kernel `NLMSG_DONE` with no payload or exactly one native-endian signed 32-bit zero status; rejects malformed or nonzero completion status; rejects every `NLMSG_ERROR` message including zero-error ACK; preserves all other parser behavior.

- [ ] Step 1: Add tests for a 20-byte `NLMSG_DONE` with zero status, a nonzero `NLMSG_DONE` status, a zero-error `NLMSG_ERROR` ACK, and a nonzero `NLMSG_ERROR`. Use the existing raw netlink fixture builders and assert only the valid completion returns the exact tuple proof.
- [ ] Step 2: Run `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` from the policy-helper directory. Expected: the realistic completion test fails with the current malformed-completion error; the ACK test demonstrates current erroneous acceptance.
- [ ] Step 3: Update only completion/error decoding in `helper.py`: allow NLMSG_DONE payload lengths 0 or 4, decode a 4-byte signed status and require zero; reject all NLMSG_ERROR types regardless of payload code. Keep malformed lengths rejected.
- [ ] Step 4: Re-run the parser selector, barrier adapter selector, `python3 -m py_compile helper.py controller.py barrier_adapter.py`, and `git diff --check`. Expected: all pass; parser selector includes the four new cases.
- [ ] Step 5: Record exact pre/post source hashes, tests and outcomes, no commit/staging, and the Docker exact-flow validation still pending in the report.

**Acceptance:** Independent review finds no HIGH or MEDIUM defect in the scoped parser repair; exact disposable-flow test in the primary integration worktree later proves established `reuse-ok`, fresh same-listener denial, positive exact drop-counter delta, and verified cleanup.

**Failure handling:** If kernel completion framing remains ambiguous, leave policy fail-closed and report the raw response. Do not infer completion from socket closure, missing procfs, Docker inspect, or counters. If synthetic tests pass but the exact Docker flow fails, preserve evidence and keep T14 unverified.
