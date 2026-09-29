# T14 SOCK_DIAG live-acceptance instrumentation Fix 2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development. Follow RED -> GREEN -> REFACTOR.

**Goal:** Make the T14 exact-flow live integration fixture prove which ACK-guarded nft rules carry the existing same-FD marker and test a bounded same-source-port connection attempt after that FD is closed.

**Source findings:** independent Task 1 review findings 1 and 3 in `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-review.md`. Fix 1 normalization is reviewed/validated at helper SHA `c8d30121927350e3cea5fd3b5823ca2308f1b4f550af1d730d29ac7e92fd87c2` and test SHA `4026dc762d2e5f7c5535c4c0874279300fb110d9a06042739ab3f77fd1333167`.

## Global constraints

- Work only in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`.
- Preserve every existing worktree file and change. No Docker, image/daemon changes, staging, commits, push, stash, Issue writes, or test discovery in implementation/validation.
- Only edit `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; do not edit controller/helper or other ticket files. Save task report as named below.
- Keep the seven-field flow contract, policy behavior, and exact tuple unchanged. Test code must not weaken existing preview, no-egress, distinct-source fresh SYN, or cleanup assertions.
- Commit strategy: uncommitted checkpoint, exact test pre/post hashes and task delta; the baseline hash is `4026dc762d2e5f7c5535c4c0874279300fb110d9a06042739ab3f77fd1333167`.

## Task 1 — record exact rule use and same-port SYN denial

**Dependency:** SOCK_DIAG Task 1 implementation and Fix 1 are independently reviewed and backend-validated. The source/runtime interfaces are frozen; this task edits only the T14 test fixture.

**Role:** `backend_implementer`. **Validator:** `backend_validator`. **Independent reviewer:** parent assigns.

**Owned files:**
- `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix2-report.md`

**Consumes:** same seven-field PID 1 flow attestation; retained `webdriver-canary-after` and post-marker raw nft snapshots; two exact ACK-guarded output rules emitted by `webdriver_rules`.

**Produces:** test assertions and durable evidence for both directional tuple/ACK rule expressions, positive per-rule counter deltas around the same-FD marker, and a same-source-port connection attempt after the original FD is closed.

### RED → GREEN → REFACTOR

1. **RED:** add pure fixture-level tests for identifying the exact directional rule records in nft JSON, extracting their packet counter, and validating full address/port tuples plus exact `tcp flags & (syn | ack) == ack` AST. Show malformed, missing, duplicate, broad, or non-ACK-gated rules are rejected. Add a test for the same-port probe evidence validator (bind succeeds, connect attempted, connect denied, loopback-drop delta positive).
2. **GREEN:** add bounded fixture controls. A second signal handler in the disposable renderer closes only the original WebDriver client FD and writes an acknowledgement marker. After the same-FD `reuse-ok` marker, the live test signals close, waits with a fixed deadline, creates a fresh TCP socket, binds the attested client loopback address and exact source port, then attempts the exact listener destination with a bounded connect timeout. Record bind/connect errno and elapsed time. Verify each exact ACK rule's full tuple and flag AST from the raw nft snapshots taken immediately before and after the `reuse-ok` marker; assert positive per-direction counter deltas only in that window. Around the same-port attempt, assert connect denial, exact source port, and positive loopback drop delta; verify neither ACK exception counter grows for the blocked SYN. Preserve and retain the existing different-source-port SYN canary.
3. **REFACTOR:** keep helpers narrowly scoped and test malformed evidence/rejection behavior. Run only pure new selectors, the focused `PolicyTargetCounterUnitTests` class, `py_compile`, and `git diff --check`. Do not run class-level integration setup, Docker, or discovery.
4. Save source hash, task delta, RED/GREEN outputs, and any test limitations in this report. Parent will run the exact single integration selector once after independent Review and validation, retaining all evidence under a fresh `docs/testing/craft/t14/2026-09-29-sockdiag-live-acceptance/` directory.

**Acceptance:** Pure tests prove exact nft JSON/tuple/ACK AST selection. On the parent disposable exact-flow run, the attested socket's `reuse-ok` marker succeeds; both direction-specific exact rule counters increase between the immediately preceding and subsequent snapshots; existing different-source SYN canary remains denied with loopback drop delta; after explicit original-FD close, same source address/port bind succeeds and its new SYN is denied with another loopback drop delta and no ACK exception delta; preview remains available; target and invalid DNS counters remain isolated; all receipts bind to the same helper image, renderer ID/PID/netns, tuple/inode, policy ID, and run; renderer/policy/helper cleanup is verified.

**Failure handling:** If bind cannot be safely completed, preserve errno and socket-state evidence as a limitation; do not claim the stale exact-tuple scenario is proven. Any successful same-port connect, missing exact-rule match, absent counter delta, unexpected exception delta for SYN, or cleanup uncertainty leaves T14 unverified and T15 gated. No policy implementation edits are allowed in this task.
