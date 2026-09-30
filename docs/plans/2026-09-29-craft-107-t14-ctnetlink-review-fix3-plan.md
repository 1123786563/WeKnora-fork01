# T14 ctnetlink query-path test review-fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make the fake-socket query-path regression test prove the full kernel completion, exact bidirectional tuple, and guaranteed socket close on parser failure.

**Architecture:** Keep production behavior unchanged. Extend the unit-test fake transport so its success path returns a 20-byte `NLMSG_DONE` with zero status and compare the complete proof object; add a malformed/error response path that asserts fail-closed parsing still closes the socket.

**Tech Stack:** Python 3.12 stdlib unittest and `unittest.mock`.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`, stories 12 and 14; T14 ctnetlink plan; independent review findings in `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix2-review.md`.

## Global Constraints

- Keep the existing Linux protocol fallback and all production query/parser behavior unchanged.
- No Docker, daemon/image change, unittest discovery, new dependency, stage, commit, push, or shared stash.
- Test only the bounded ctnetlink fake socket seam; no host network or NET_ADMIN requirement.

## Review Focus

1. Query-path tests must use the observed valid 20-byte `NLMSG_DONE` form with signed zero status, not only the legacy empty completion.
2. Assert the entire proof object (source, original tuple, reply tuple, ESTABLISHED state), not only one field.
3. Assert socket close on both successful response and fail-closed response parsing.

---

### Task 1: Strengthen fake query-path assertions

**Dependencies:** T14 fix2 implementation exists; fix2's scoped Review found two MEDIUM test coverage gaps. **Role:** mechanical_worker (one test file only). **Validator:** backend_validator. **Independent reviewer:** parent dispatch after implementation. **Commit policy:** uncommitted checkpoint in existing T14 worktree.

**Files:**

- Test: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Report: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix3-report.md`

**Consumes:** Current fake-socket test `test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol`, ctnetlink raw-message builders, production query API.

**Produces:** Deterministic success-path assertion covering 20-byte zero-status completion and full proof, plus failure-path assertion that a fake socket is closed when the parser rejects an `NLMSG_ERROR` response.

- [ ] Step 1: Refactor the fake socket fixture minimally so each test query can return messages built with the captured request sequence. Keep request protocol/type/flags/family assertions. Add a zero-status `NLMSG_DONE` with a 4-byte payload (20 bytes total); compare the entire returned proof dictionary; assert socket close.
- [ ] Step 2: Add a failure response containing `NLMSG_ERROR` for the captured sequence; assert `require_tracked_webdriver_flow` raises `RuntimeError` and the fake socket was closed.
- [ ] Step 3: Run the query-path selector and full `PolicyTargetCounterUnitTests`; expected: both pass against current correct implementation. This is a test-only evidence-strengthening task, so no production behavior change or expected RED against the already-fixed implementation is required; preserve prior RED evidence for the underlying missing protocol constant and 20-byte parser fixes in the previous reports.
- [ ] Step 4: Run barrier adapter selector, py_compile, and whitespace checks (tracked diff plus no-index check for this untracked test file). Expected all pass without Docker.
- [ ] Step 5: Record exact pre/post test-file hashes and command results in the report. Do not modify `helper.py`.

**Acceptance:** Independent review confirms all three assertions; independent validator reruns the relevant selectors; no production-source hash changes; later parent-run disposable exact-flow remains a separate T14 acceptance requirement.

**Failure handling:** If fake-socket framing fails, preserve the exact response fixture and fail the task without modifying production code; do not weaken the parser or downgrade the observed kernel completion format.
