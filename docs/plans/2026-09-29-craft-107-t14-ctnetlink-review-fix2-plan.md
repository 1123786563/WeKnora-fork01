# T14 ctnetlink socket protocol compatibility fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make the bounded ctnetlink query open the Linux NETLINK_NETFILTER socket on the shipped Python 3.12 Alpine image and prove the request path without Docker.

**Architecture:** Define the Linux UAPI protocol number in one named helper constant when Python does not export it, then test `require_tracked_webdriver_flow()` through a fake socket that captures the request and returns an exact synthetic conntrack record/completion. Keep the live exact-flow proof as the separate disposable validation already scheduled.

**Tech Stack:** Python 3.12 stdlib sockets/struct/unittest; Linux UAPI `NETLINK_NETFILTER = 12`.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`, stories 12 and 14; T14 ctnetlink plan and review-fix reports; production failure evidence in `docs/testing/craft/t14/2026-09-29-ctnetlink-exact-flow/test.log`. Linux UAPI reference: [include/uapi/linux/netlink.h](https://github.com/torvalds/linux/blob/master/include/uapi/linux/netlink.h).

## Global Constraints

- Preserve renderer `NetworkMode=none`, read-only filesystem, nonroot UID 10001, `CapDrop=ALL`, and `no-new-privileges`.
- Preserve one-shot helper exact renderer namespace, only `CAP_NET_ADMIN`, bounded timeout/bytes/records/messages, and fail-closed tuple proof.
- No new dependencies, capabilities, wildcard rules, commits, staging, pushes, or shared stash.
- Fake-socket tests must not require NET_ADMIN, a Docker daemon, or kernel conntrack availability.

## Review Focus

1. Python runtime omission of `socket.NETLINK_NETFILTER` must not prevent socket creation; use the stable Linux UAPI protocol number 12 and test constructor arguments.
2. The socket query seam must keep request sequence and family, validate sender PID and the complete tuple response, and close the socket on success and failure; test the full query path with deterministic fake responses.
3. Existing timeout, message/byte caps, truncation, interruption, completion, parser fail-closed behavior, and policy installation order remain intact; existing focused selectors exercise these invariants.

---

### Task 1: Add a deterministic ctnetlink query-path test and protocol fallback

**Dependencies:** T14 ctnetlink parser review-fix1 independent Spec/quality Review and targeted validation passed. The exact disposable-flow attempt reproduced a real failure before policy mutation: Alpine Python 3.12.14 provides `AF_NETLINK=16` but no `socket.NETLINK_NETFILTER`; controller returned cleanup `verified-clean`. **Role:** backend_implementer. **Validator:** backend_validator. **Independent reviewer:** parent dispatch after implementation. **Commit policy:** uncommitted checkpoint in existing T14 worktree.

**Files:**

- Modify: `deploy/craft/render-boundary/policy-helper/helper.py`
- Test: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Report: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix2-report.md`

**Consumes:** `require_tracked_webdriver_flow(flow)`, `_ct_request(sequence, family)`, parser fixture builder in `PolicyTargetCounterUnitTests`.

**Produces:** a named `_NETLINK_NETFILTER` protocol constant equal to Linux UAPI 12, selected without depending on the runtime socket module exporting it; a fake-socket test that exercises the actual query function and asserts `socket(AF_NETLINK, SOCK_DGRAM, 12)`, the CT_GET request type/family/sequence, exact returned tuple proof, and socket close.

- [ ] Step 1: Add a test with `unittest.mock.patch` replacing `module.socket.socket` by a deterministic fake. Have `sendto` capture and decode the request sequence/family; have `recvmsg` return a synthetic matching CT_NEW and a 20-byte zero-status NLMSG_DONE using that sequence. Assert constructor `(AF_NETLINK, SOCK_DGRAM, 12)`, request CT_GET type, selected family, successful exact proof, and `close()` called. Demonstrate that current code raises because `socket.NETLINK_NETFILTER` is absent.
- [ ] Step 2: Run only `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol -v` from the policy-helper directory. Expected RED: `AttributeError` from the missing socket-module constant.
- [ ] Step 3: Define `_NETLINK_NETFILTER = getattr(socket, "NETLINK_NETFILTER", 12)` near the netlink constants and use it in the socket constructor. Keep the request framing, parse rules, bounds, and cleanup unchanged.
- [ ] Step 4: Run the new test, full parser selector, barrier adapter selector, py_compile, and diff-check. Expected: all pass.
- [ ] Step 5: Record exact pre/post hashes and output; do not run Docker in the implementation task. Parent will rerun the one disposable exact-flow integration after independent Review and validation.

**Acceptance:** Fake-socket path proves correct protocol number/request/reply/close; independent Review and validation pass; subsequent parent-run disposable exact-flow integration proves existing connection reuse, fresh same-listener denial, positive exact drop-counter delta, and verified cleanup.

**Failure handling:** If fake-socket proof cannot establish request identity and one exact response, retain fail-closed policy and report. Do not silently use a different netlink protocol or rely on host-specific Python constants.
