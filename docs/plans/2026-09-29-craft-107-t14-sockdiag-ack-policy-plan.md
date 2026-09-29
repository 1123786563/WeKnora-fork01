# T14 socket diagnostic and ACK-guarded policy Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace T14's unusable conntrack prerequisite with a kernel-verified PID 1 TCP socket attestation and an ACK-guarded exact-tuple nft exception, while preserving no-egress and rejecting new TCP handshakes.

**Architecture ruling:** `docs/plans/2026-09-29-craft-107-t14-conntrack-vs-sockdiag-architecture.md`, updated SHA-256 `32338d88d74d3b951232c0e6d66d18606d0bf386a01fd5170aa84c52273d0501`. The approved product Spec requires no external egress but does not prescribe conntrack. This is an explicit implementation-invariant change from `ct state established` to (1) bounded SOCK_DIAG exact-one TCP_ESTABLISHED tuple + inode proof and (2) two exact four-tuple output rules with `tcp flags & (syn | ack) == ack`. The ACK guard is not conntrack state and rules are not bound to socket inode/cookie; documented residual packet-forgery/stale-tuple risk is accepted only for this reviewed implementation, constrained by renderer CAP_DROP ALL/no CAP_NET_RAW, no-new-privileges, read-only root, exact tuple and renderer lifetime. No broad state, port-only, listener-wide, or new SYN/SYN-ACK is allowed.

**Tech Stack:** Python 3.12 stdlib sockets/struct/unittest; Linux `NETLINK_SOCK_DIAG` (protocol 4), `SOCK_DIAG_BY_FAMILY` (type 20); nftables.

**Spec:** Approved Craft Spec stories 12 and 14 and no-egress invariant; T14 #129; T00 frozen seven-field flow; primary evidence `docs/testing/craft/t14/2026-09-29-sockdiag-dump-probe/query.json` (successful target kernel dump and PID 1 FD/inode match), plus empty successful ctnetlink evidence and architecture ruling.

## Global Constraints

- Work only in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`; no staging/commit/push/stash. Preserve existing T14 worktree files, including untracked evidence and current fix4 checkpoint.
- Owned production/test files only: `deploy/craft/render-boundary/policy-helper/helper.py`, `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; report under this plan's `-report.md`.
- Keep host seven-field JSON and host PID 1 FD ownership discovery unchanged. The helper runs in exact renderer netns, cap-drop ALL + NET_ADMIN only, no-new-privileges, read-only root.
- Use protocol number 4 via explicit constant fallback because Alpine Python may omit `NETLINK_SOCK_DIAG`; query uses raw netlink and type 20. Do not use the failing exact-query operation; use a bounded dump with one request and exact-one filter.
- Keep strict bounded timeout/datagrams/aggregate bytes/message count; validate kernel sender PID 0, request sequence, local port ID, framing, response family, TCP_ESTABLISHED, exact tuple, inode equals host `client_socket_inode`, and fixed cookie representation. TCP protocol is pinned in the request (`sdiag_protocol=IPPROTO_TCP`); response `inet_diag_msg` has no protocol field. Reject missing, duplicate, wrong tuple/state/inode, truncation, interruption, errors, malformed records and no completion. Close socket on all paths.
- Replace only the two T14 `ct state established` conditions with exact tuple plus `tcp flags & (syn | ack) == ack`, with identical address/port specificity. Never add generic accept rules.
- No Docker in implementation. Parent reserves the only Docker slot for the exact disposable integration proof after review and validation.

## Review Focus

1. SOCK_DIAG wire layout follows Linux UAPI: request v2 is 56 bytes; response `inet_diag_msg` is 72 bytes before attrs; validate response family and decode address bytes correctly for IPv4/IPv6. TCP protocol is established by the request (`sdiag_protocol=IPPROTO_TCP`) because response `inet_diag_msg` has no protocol field. Output must not mistake listener or reverse accepted peer for host-attested client inode.
2. Parser rejects malformed header/type/sequence/port ID/sender, bad/ambiguous/missing tuple, wrong state/inode, truncation, dump interruption, error, timeout and over-budget input; test every observable failure class and socket close.
3. The nft expression is exactly `tcp flags & (syn | ack) == ack` on two full directional tuples. Fresh SYN and reverse SYN+ACK fail the exception; established marker packet continues. No broad conntrack/port-only/listener rules remain.
4. No claimed equivalence between ACK and conntrack. Evidence explains stale exact tuple and packet injection limits; policy lifecycle remains bound to renderer and any lost/reconnected attested socket fails the run.

### Task 1: Verify the exact host-attested socket with bounded SOCK_DIAG and constrain nft exceptions

**Dependencies:** T00 seven-field contract verified; current fix4 checkpoint reviewed/validated; read-only target dump proves tuple/state/inode and `/proc/1/fd/3` match. Architecture ruling updated with ACK-guarded stateless option. **Role:** backend_implementer. **Validator:** backend_validator. **Reviewer:** independent reviewer after implementation. **Commit strategy:** uncommitted checkpoint; exact source/test pre/post hashes and task delta required.

**Files:**
- Modify: `deploy/craft/render-boundary/policy-helper/helper.py`
- Modify tests: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Create report: `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-report.md`

**Consumes:** host-attested `flow` with exact keys family, client/driver addresses and ports, ESTABLISHED, client socket inode; architecture ruling and saved target-kernel dump evidence.

**Produces:** `require_attested_webdriver_socket(flow)` proof identifying exact diag tuple/state/inode/cookie; two exact ACK-set/SYN-clear directional rules; tests and report.

- [ ] Step 1 (RED): Add UAPI-shaped fixtures and parser/query path tests using fake raw socket. Cover IPv4 and IPv6, matched header local port ID and kernel sender address, exact client inode, completion; reject no/multiple matches, wrong response family, wrong request protocol, state/tuple/inode, bad framing/length/sequence/header PID/sender PID, truncation, interrupted dump, NLMSG_ERROR, timeout, byte/message overflow. Assert socket closes on success and failure.
- [ ] Step 2 (GREEN): Implement bounded `NETLINK_SOCK_DIAG` dump request with `SOCK_DIAG_BY_FAMILY`, `NLM_F_REQUEST|NLM_F_DUMP`, TCP, target family; parse records and completion under the existing defensive bound style. Return only exact one client-direction record matching ESTABLISHED and the host inode. Record but do not require the dump cookie to match host data (the frozen 7-field contract has no cookie).
- [ ] Step 3 (RED): Update rule tests to require exactly two full directional tuples and ACK set/SYN clear; assert no `ct state`, broad accept, or omitted tuple member.
- [ ] Step 4 (GREEN): Change rule text and installation precondition from conntrack proof to socketdiag proof. Preserve chain ordering, output default drop, loopback drop counters, helper sandbox and failure cleanup.
- [ ] Step 5 (REFACTOR): Factor wire parsing/rule matching cleanly; ensure diagnostics avoid unbounded data. Run focused parser/query tests, all helper unit selectors, barrier adapter tests, `py_compile`, `git diff --check`, and relevant static tests; no Docker.
- [ ] Step 6: Save source/test hashes, RED/GREEN evidence, commands/results, review-package patch including modified and untracked files, and explicit residual risk in report. Parent runs the single disposable exact-flow test after independent Review and validation.

**Acceptance:** Independent Review Spec compliance and quality PASS; backend validation passes against exact hashes; parent exact disposable test proves same existing PID 1 socket marker succeeds and increments the exact exception counter; a fresh TCP connection to same listener from a different source port fails and increments loopback-drop counter; include a same-source-port reuse negative test if safely constructible; preview and external-egress negative canaries pass; helper/image/namespace/tuple/inode/rules/counters/cleanup receipts correlate to one immutable run. Only then T14 can be verified and T15 released.

**Failure handling:** Any parser ambiguity, ACK rule mismatch, live marker failure, new connection success, external egress success, or cleanup uncertainty keeps T14 blocked/fail-closed; preserve evidence and do not proceed to browser matrix.
