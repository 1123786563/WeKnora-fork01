# Independent Review — T14 ctnetlink protocol fallback fix2

Reviewed 2026-09-29. Scope: the two fix2 owned source files in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01` at HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`, against the fix2 plan and brief, original ctnetlink plan and review, approved Craft web artifact spec, `CONTEXT.md`, and ADR 0004. This was a read-only source review; no OCR, Docker, unittest discovery, source edit, or issue edit was performed.

## Verdict

- **Spec compliance: PARTIAL.** The production fallback satisfies the assigned protocol change: Linux UAPI `include/uapi/linux/netlink.h` defines `NETLINK_NETFILTER` as 12, and the helper uses `getattr(socket, "NETLINK_NETFILTER", 12)` at the existing socket constructor. The fake-socket test checks the constructor, CT_GET request type, request/dump flags, IPv4 family, positive sequence, and successful closure. It does not exercise the required observed 20-byte zero-status completion, verify the full exact original/reply tuple, or cover failure-path closure. The parent-run disposable exact-flow acceptance remains outstanding.
- **Code quality: CONDITIONAL PASS for the two-line production change; test acceptance remains open.** No unintended policy, capability, request-framing, bounds, parser, or cleanup change is visible in the fix2 implementation as reported. The existing `finally: sock.close()` covers both successful parsing and exceptions after socket creation, but the new test proves only the success case. No high-severity production defect was identified in this scoped review.

## Findings

### Medium — Fake-socket success fixture does not use the observed kernel completion

- **Affected:** `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py:119,172-176`.
- **Evidence:** `ctnetlink_test_dump()` constructs `NLMSG_DONE` with length 16 and no status payload. The new query-path fake reuses that fixture. The fix2 plan and brief explicitly require a 20-byte zero-status `NLMSG_DONE`, matching the prior kernel probe and fix1 parser correction.
- **Impact:** The query-path test cannot catch a regression in handling the completion shape observed from the kernel. It gives weaker evidence for the exact disposable flow that previously failed before policy installation.
- **Smallest correction:** Supply `struct.pack("=IHHIIi", 20, 3, 0, sequence, 0, 0)` as the fake completion in this test, then rerun its targeted selector.

### Medium — Exact proof and failure closure are not asserted by the new query-path test

- **Affected:** `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py:183-199`; production `require_tracked_webdriver_flow()` at `helper.py:301-335`.
- **Evidence:** The test asserts only `proof["state"]`, `proof["tuple"]["sport"]`, and `fake_socket.closed` on success. It does not assert the full original and reverse reply tuples or inject a failed `recvmsg`/malformed response to prove closure on failure. The production `finally` does close on those paths, but the assigned plan's query-path proof asks for an exact response and success/failure closure.
- **Impact:** A request-path or result-mapping regression could preserve the single asserted source port while returning an incorrect endpoint or reply tuple; an exception-path cleanup regression would be missed.
- **Smallest correction:** Assert the complete returned original and reply tuple dictionaries against the seven-field flow. Add one fake-socket exception case (for example `socket.timeout`) and assert both the fail-closed error and `close()`.

## Evidence and limits

- Current SHA-256 values were independently recomputed and match the implementer report: `helper.py` `b89f1fd93c7368f4e4d9aecfec0dba1e875b72459506fabda261b54eaaff4958`; `tests/test_controller_integration.py` `e4dbd65ad5f7b7ec34c995583740a0958bff3100ce2ea8695059cb39fd54fbb5`. The brief's preimage hashes agree with the fix1 report's final hashes. Both owned source files are untracked in the worktree, and no preimage patch is available; task-only attribution relies on the report plus source inspection.
- The new test decodes and checks request length, type 257, flags `0x301`, nonzero sequence, PID zero, and IPv4 nfgen `(2, 0, 0)`; the fake reply copies that request sequence. It checks destination `(0, 0)`, but does not explicitly assert the fake's recorded bind address. The test patches `AF_NETLINK=16` for the macOS host. It does not force `NETLINK_NETFILTER` to be absent, so on a host exporting that constant, its pass alone would not prove the fallback branch; the report's RED evidence was observed on a host lacking the attribute.
- The fix2 brief records the Alpine Python 3.12.14 `AttributeError` and `verified-clean` cleanup from the disposable attempt. The referenced raw `docs/testing/craft/t14/2026-09-29-ctnetlink-exact-flow/test.log` was not present in this review worktree or the integration tree; that exact log could not be independently inspected here. The implementer reports targeted selectors (1, 13, and 18 tests), `py_compile`, and whitespace checks passing; this review did not rerun them.
- Linux UAPI source for protocol 12: https://github.com/torvalds/linux/blob/master/include/uapi/linux/netlink.h . This scoped review cannot certify live socket behavior or the original plan's old-flow reuse, new same-listener denial, positive drop-counter delta, and cleanup until the parent runs the disposable integration.
