# T14 SOCK_DIAG same-port timeout evidence Fix 3

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Follow RED -> GREEN -> REFACTOR.

**Goal:** Treat a bounded socket timeout as a valid denial outcome for the same-source-port SYN canary, while preserving timeout details and durable failure evidence.

**Source review:** Fix 2 independent review `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix2-review.md`, high finding: the policy-dropped SYN normally times out with `errno=None`, but the validator requires positive integer errno; additionally subprocess failure/invalid JSON currently asserts before saving the receipt.

**Checkpoint:** Fix 2 test source hash `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d`; Fix 2 report `307145b6380cb12f91c75b14c4eedbaa5eb667819c6f5720043d5b83167a304e`; HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Before snapshot is retained at `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/before/test_controller_integration.py` SHA `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d`.

## Global constraints

- Work only in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`.
- Preserve all existing changes. No Docker, image/daemon changes, staging, commit, push, stash, Issue writes, or discovery.
- Modify only `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` and create `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix3-report.md`.
- Do not weaken exact rule tuple/ACK checks, the positive marker deltas, same-port loopback-drop delta, zero ACK exception deltas, existing distinct-source canary, preview/no-egress checks, or fail-closed outcomes.
- Commit strategy: uncommitted. Keep the exact before snapshot and create the post snapshot/task diff package in `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/`.

## Task 1 — accept and preserve only verified bounded denial outcomes

**Dependency:** Fix 2's exact-rule and live-fixture changes pass independent Review and backend validation; source interface is frozen. This repair addresses only reviewer finding 1 and its adjacent failure-evidence gap.

**Role:** `backend_implementer`. **Validator:** `backend_validator`. **Reviewer:** parent assigns.

**Owned files:**
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix3-report.md`

**Consumes:** bounded 500ms same-port probe evidence with `bind_succeeded`, `connect_attempted`, `connected`, optional positive `connect_errno`, timeout exception type/reason, endpoints and elapsed milliseconds; before/after nft records and counter deltas.

**Produces:** explicit denial classification for a timed-out dropped SYN (`TimeoutError`/`socket.timeout`, absent errno) or a positive errno connect failure, requiring the existing loopback drop delta > 0 and zero ACK-exception deltas; every subprocess/parse failure receipt is written with `passed: false` before assertion.

### RED → GREEN → REFACTOR

1. RED: add pure validator cases proving: (a) `connected=False`, `connect_errno=None`, explicit timeout type/reason, exact endpoints, positive loopback-drop delta and zero exception deltas is accepted; (b) the same timeout without a positive drop delta fails; (c) a successful connection fails; (d) unsupported exception without errno fails; (e) positive errno denial still passes; (f) nonzero Docker exec result or invalid JSON is durably represented as a failed receipt.
2. GREEN: record `connect_error_type` and a stable denial classification from the probe; permit absent errno only for the recognized bounded timeout result. Wrap execution/result parsing so a receipt with command status, stdout/stderr, elapsed bounds, raw result or parse error, counters where available, and `passed:false` is written before raising.
3. REFACTOR: keep denial classification narrow and deterministic; do not treat arbitrary OSError, process failure, absent result, or counter evidence alone as successful denial. Run the pure same-port selector, all `PolicyTargetCounterUnitTests`, `py_compile`, and `git diff --check`; no Docker/discovery.
4. Copy the exact post source into the SDD checkpoint directory and produce a complete `.patch` against the retained before file. Save hashes and test output in the report.

**Acceptance:** pure tests accept the actual Python timeout shape (`TimeoutError`, no errno) only when the probe was attempted, bind succeeded, it did not connect, exact flow endpoints match, timeout duration is bounded, the loopback drop delta is positive, and both exact ACK counters stayed unchanged. All adverse/ambiguous paths fail and retain a `passed:false` receipt before assertion. Existing positive errno-denial semantics stay supported. Exact no-Docker checks pass. Independent review and backend validation pass at pinned hashes.

**Failure handling:** never infer a denied SYN from socket timeout alone; the exact loopback-drop counter is mandatory. If the probe command fails or JSON cannot be parsed, preserve that fact as a failed receipt and fail the test. No live test, Docker use or T14 verification is claimed by this task.
