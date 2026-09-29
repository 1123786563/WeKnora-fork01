# T14 ctnetlink WebDriver flow proof Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Obtain the exact existing renderer-namespace TCP connection tuple from kernel ctnetlink inside the already capability-limited policy helper, so the WebDriver established-flow exception can be installed without an unavailable procfs conntrack table.

**Architecture:** Preserve the existing seven-field host-attested WebDriver flow and existing fail-closed exact-one-match rule. Replace the unavailable `/proc/net/nf_conntrack` dependency with a bounded, read-only `NETLINK_NETFILTER` `IPCTNL_MSG_CT_GET` dump issued only inside the one-shot helper that joins the exact renderer network namespace and already holds only `CAP_NET_ADMIN`; parse original/reply tuples and TCP state before generating existing exact nft rules. The T14 disposable integration test must prove the existing connection still exchanges its marker after policy install while a new same-listener connection from another source port is denied and increments the exact loopback-drop counter.

**Tech Stack:** Python 3.12 stdlib sockets/struct, Linux NETLINK_NETFILTER ctnetlink UAPI, Docker/OrbStack disposable namespace tests.

**Spec:** `docs/specs/2026-09-23-craft-web-artifact-spec.md` stories 12 and 14 and § scope/no-external-egress; native dependencies #119→#129 and #129→#130 in `docs/plans/2026-09-29-craft-107-live-issue-refresh.md`; T14 policy seam audit `docs/plans/2026-09-29-craft-107-t14-conntrack-source-audit.md`; existing seven-field contract and fail-closed requirements in `docs/plans/2026-09-28-craft-107-t14-webdriver-control-policy-plan.md` (T14 Worktree).

## Global Constraints

- Renderer remains `NetworkMode=none`, read-only, nonroot UID 10001, `CapDrop=ALL`, `no-new-privileges`; do not add capabilities to the renderer.
- One-shot helper remains bound to the exact renderer container/network namespace, read-only, `CapDrop=ALL` plus only `CAP_NET_ADMIN`, `no-new-privileges`, bounded memory/CPU/PIDs/runtime.
- Keep exact source/destination addresses, ports, TCP protocol, bidirectional tuple, and kernel `ESTABLISHED` state; install no wildcard or listener-wide exception.
- Any missing permission, malformed/truncated/interrupted netlink dump, uncertain namespace, no exact match, or duplicate match fails closed before policy install.
- Bound socket wall time, bytes/datagram, total dump bytes, record count, netlink message count, attribute depth/length and output; do not install packages or introduce a dependency.
- No commit, staging, push, live product deployment, published port, host mount, shared stash, or edits outside the declared Task files.

## Review Focus

1. Empty or unavailable ctnetlink dump must not be interpreted as no matching record with a permissive fallback; fail closed before rules mutate.
2. Netlink sender PID, sequence, message lengths, multipart completion, dump-interrupted/error flags, truncation and deadline must be validated.
3. Tuple parser must reject wrong family/protocol/state, incomplete or duplicate original/reply attributes, invalid address/port lengths and ambiguous duplicate exact records.
4. Only the kernel TCP `ESTABLISHED` state plus one exact bidirectional tuple can authorize the old-flow exception; socket-table LISTEN/ESTABLISHED observations alone are insufficient.
5. The disposable acceptance test must independently observe old-flow `reuse-ok`, fresh same-listener denial from another source port, exact positive loopback-drop delta, and cleanup.

---

### Task 1: Bounded ctnetlink exact-flow reader

**Dependencies:** T00 verified; T14 seven-field flow contract and policy ordering reviewed; current research audit confirms ctnetlink dump succeeds inside helper's existing capability boundary. This task is uncommitted until integration review. **Role:** backend_implementer. **Validator:** backend_validator. **Independent Review:** parent dispatches after frozen task report. **Start baseline:** T14 Worktree HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; current helper SHA-256 `31be195955c9216ce179e7f2fd5189577d7143acb81a57b70882c76c6de79643`; current integration test SHA-256 `0f9f69699b5da7ef6bab7437497c666fb0988bb5d2dda082811f887cc7d8b68f`. Record an exact preimage/hash snapshot before editing; no shared changes.

**Files:**

- Modify: `deploy/craft/render-boundary/policy-helper/helper.py`
- Test: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Report: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-report.md` in this Worktree, copied to integration only after Review PASS.

**Consumes:** existing `validate_webdriver_flow()` seven-field record in `barrier_adapter.py`; controller currently invokes `helper.py install` inside `--network container:<exact renderer ID>` with only `CAP_NET_ADMIN`; existing output-chain priority and exact bidirectional `webdriver_rules()`.

**Produces:** existing `require_tracked_webdriver_flow(flow)` behavior backed by a bounded ctnetlink query; helper receipt continues to report the exact matched tuple/state and existing rules consume the same flow object. No new external interface or permission.

- [ ] **Step 1: Write failing parser/request tests.** In `tests/test_controller_integration.py`, add raw multipart ctnetlink fixtures for one exact IPv4 and IPv6 TCP ESTABLISHED record, wrong family/protocol/TCP state, tuple mismatch, duplicate exact match, malformed attribute length/port/address, unexpected sender PID/sequence, `NLMSG_ERROR`, `NLM_F_DUMP_INTR`, missing `NLMSG_DONE`, over-limit datagram/aggregate bytes/record count, and late response deadline. Assert only exactly one expected bidirectional record returns proof; all other cases raise bounded errors before install. Keep existing valid-proc parser unit tests only if the implementation retains that test seam; production may not treat absent procfs as proof.
- [ ] **Step 2: Run unit selector against current code.** From `deploy/craft/render-boundary/policy-helper`, run `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v`; expected new ctnetlink reader/parser tests fail because there is no ctnetlink reader.
- [ ] **Step 3: Implement bounded stdlib request/parser in helper.py.** Send one sequenced `IPCTNL_MSG_CT_GET` multipart dump to `NETLINK_NETFILTER` in the helper's current renderer network namespace; validate kernel sender, matching sequence, `NLMSG_DONE`, error/interrupted/truncation flags, monotonic timeout, maximum message/record/byte counts, and strict nested CTA original/reply IP/protocol/TCP-state attributes. Compare one exact reverse tuple and TCP state `ESTABLISHED`; reject absent/duplicate/ambiguous results. Keep every existing cleanup, flow binding and policy rule unchanged.
- [ ] **Step 4: Run parser tests GREEN.** Re-run `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v`, `python3 -m unittest discover -s tests`, `python3 -m py_compile helper.py controller.py barrier_adapter.py`, and `git diff --check`; expect all bounded non-Docker policy tests pass with only Docker-gated tests skipped.
- [ ] **Step 5: Exercise one real disposable exact-flow policy integration.** Ensure no other Docker/probe/build process is active. Using the existing exact renderer image by immutable ID and rebuilding only the helper from the exact current helper source, run `PolicyControllerIntegrationTests.test_target_counters_cover_random_preview_loopback_and_controlled_dns` with `POLICY_HELPER_RUN_DOCKER_TESTS=1` and a unique evidence directory. Expect: helper ctnetlink finds exactly the existing socket; preview still works; fresh same-listener connection is denied from a different source port; drop counter delta >0; signaling PID1 yields exact `reuse-ok`; cleanup is verified. Capture image IDs/source hashes, helper capability/namespace inspect, raw bounded evidence and cleanup. No full browser matrix until this exact-flow proof passes review.
- [ ] **Step 6: Freeze and report.** Record before/after file hashes, task-only diff/patch hash, exact test outputs, image IDs, evidence directory and cleanup status. Report any skipped/failed live attempt honestly. Do not commit.

**Expected result:** bounded synthetic netlink tests pass; policy-helper non-Docker suite passes; the exact existing TCP connection carries `reuse-ok` after policy while a new same-listener connection is denied and increments its drop counter; all test containers are confirmed removed.

**Failure handling:** If the ctnetlink tuple/state attributes cannot be proven from raw kernel responses, keep fail-closed policy and report the exact limitation; do not replace kernel conntrack evidence with `/proc/net/tcp`, Docker inspect, nft counters, or an unrestricted allow. If helper rebuild or daemon times out, preserve raw process/cleanup/image evidence and stop before the immutable 37-cell matrix; no blind rerun.
