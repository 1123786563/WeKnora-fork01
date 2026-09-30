# T14 ctnetlink response port-ID validation fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Accept ctnetlink dump headers that identify the requesting socket port ID while continuing to authenticate the actual kernel sender from the `recvmsg` source address.

**Architecture:** Read the local Netlink port ID from the bound socket, pass it into the bounded parser, and compare each `nlmsghdr.nlmsg_pid` to that ID. Keep `recvmsg` source PID 0 validation as the kernel-sender check. Update the raw-message fixtures and fake socket to model both IDs separately.

**Tech Stack:** Python 3.12 stdlib sockets/struct/unittest; Linux NETLINK_NETFILTER UAPI.

**Spec:** Craft Spec stories 12 and 14; T14 ctnetlink plan; exact raw header evidence `docs/testing/craft/t14/2026-09-29-ctnetlink-exact-flow-fix3-retry1/raw-header-diagnostic.json`; Linux conntrack kernel implementation [nf_conntrack_netlink.c](https://github.com/torvalds/linux/blob/master/net/netfilter/nf_conntrack_netlink.c) emits response records through `nfnl_msg_put(... item->portid ...)`; `nlmsg_put()` documents `portid` as the requesting application's port ID [kernel netlink helper](https://github.com/torvalds/linux/blob/master/include/net/netlink.h).

## Global Constraints

- Continue requiring `recvmsg` sender address PID 0 and the exact request sequence.
- Compare response header PID against the local bound socket port ID; do not weaken it to accept arbitrary values or skip the check.
- Preserve all timeout, byte/message/record limits, truncation/interruption/error checks, exact bidirectional tuple/state checks, cleanup, and policy ordering.
- No new dependency/capability, Docker test, image/daemon change, staging, commit, push, or shared stash in implementation.

## Review Focus

1. Kernel sender identity is the `recvmsg` source sockaddr PID 0; the conntrack response header may carry the querying socket's port ID. Test these fields independently.
2. Valid nonzero local port ID is accepted only when the header matches it; wrong header port ID, wrong source PID, and wrong sequence fail closed.
3. All existing parser tests and the full fake-socket query test retain bounds, completion, exact proof and socket-close behavior.

---

### Task 1: Validate response header against bound socket port ID

**Dependencies:** ctnetlink fake query path fixes1–3 pass targeted independent Review/validation. Exact disposable test now fails `malformed ctnetlink message header`. Bounded no-policy diagnostic reproduced response metadata: `recvmsg` source PID 0; `NLMSG_DONE` length 20, type 3, flags 2, sequence echoed, header PID 1. Same namespace renderer held client port 49152→listener 9515. **Role:** backend_implementer. **Validator:** backend_validator. **Reviewer:** parent dispatch after implementation. **Commit strategy:** uncommitted checkpoint.

**Files:**

- Modify: `deploy/craft/render-boundary/policy-helper/helper.py`
- Test: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Report: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix4-report.md`

**Consumes:** `require_tracked_webdriver_flow(flow)`, `parse_ctnetlink_dump(flow, messages, *, sequence, local_port_id=...)`, existing raw response builders.

**Produces:** parser validates outer `sender_pid == 0`, inner message `nlmsg_seq == sequence`, and inner `nlmsg_pid == local_port_id`; query obtains `local_port_id = sock.getsockname()[0]` after bind. Tests cover nonzero matched port ID accepted and mismatched response port ID rejected.

- [ ] Step 1: Extend raw fixture builder with an explicit `message_pid` parameter. Add/adjust unit case to pass nonzero `local_port_id` and matching header PID, and assert a mismatched header PID raises; retain a test that outer sender PID must be zero.
- [ ] Step 2: Run the targeted parser case and query-path test before production edit. Expected RED: current fake socket lacks `getsockname`/parser argument or nonzero header PID is rejected.
- [ ] Step 3: Extend `parse_ctnetlink_dump` with required keyword `local_port_id`; validate headers against it rather than literal 0. After `sock.bind((0, 0))`, read `sock.getsockname()[0]` and pass it into parser. Keep the independent `recvmsg` address sender check at 0.
- [ ] Step 4: Run exact parser/query selectors, barrier adapter selector, py_compile and diff checks/no-index. Expected all pass.
- [ ] Step 5: Record raw evidence, exact source/test hashes and RED/GREEN results. Do not run Docker here.

**Acceptance:** Independent Review and validation pass. Parent then retries the same disposable exact-flow test; if tuple is absent after header correction, preserve that evidence as a separate blocker rather than relaxing exact-match requirements.

**Failure handling:** Any mismatch between recvmsg source PID, sequence or local response port ID fails closed. Do not accept arbitrary header PIDs. If the ctnetlink dump completes but has no exact tuple, do not synthesize a match from `/proc/net/tcp`, nft counters, or socket LISTEN state.
