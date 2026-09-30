# Independent Review — T14 ctnetlink response port ID fix4

Reviewed 2026-09-29 in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01` at HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`. Scope: the fix4 plan and brief, approved Craft spec stories 12 and 14, `CONTEXT.md`, relevant ADR 0004, and the current helper/parser and test files. This review did not run OCR or Docker and did not change requirements, production code, tests, or issues.

## Verdict

- **Scoped Spec compliance: PASS.** `require_tracked_webdriver_flow()` reads the bound Netlink socket's local port ID after `bind()` and passes it to the parser. The parser independently requires `recvmsg` sender PID 0, the exact response sequence, and every inner header's `nlmsg_pid` equal to that local port ID. It still requires one exact bidirectional ESTABLISHED tuple.
- **Scoped code quality: PASS.** The new nonzero port fixture is accepted only when the inner header matches the bound port; a mismatched nonzero header is rejected. Existing wrong sender/sequence, bounds, truncation, error, completion, and fake socket closure paths remain present. No actionable finding in this scoped change.
- **T14 integration acceptance: OPEN.** The recorded raw-header diagnostic has a `NLMSG_DONE` with status 0, source PID 0, sequence 10828, and header PID 1, but contains no `CT_NEW` record. The fix removes a header-validation rejection; it does not establish that ctnetlink can return the exact loopback tuple in this namespace. A live exact-flow proof remains unresolved. Do not infer a tuple from DONE alone.

## Evidence

1. `helper.py:305-309,323-332` binds the query socket, reads `sock.getsockname()[0]`, preserves the `recvmsg` address PID, and passes the local ID into `parse_ctnetlink_dump()`.
2. `helper.py:233-258` rejects a nonzero outer sender PID, wrong outer or inner sequence, and an inner PID different from the bound local ID. The three checks are separate; `helper.py:286-291` still requires exactly one exact original/reply tuple in ESTABLISHED state.
3. `test_controller_integration.py:96-121,152-245` models the response header PID separately from sender PID, uses bound port 321 in the query-path fake, accepts header PID 321, and rejects header PID 322 against local port 321. `test_controller_integration.py:318-341` retains wrong sender and sequence rejection cases.
4. Independently run four focused unit tests for matching/mismatching nonzero port ID, bad transport/incomplete dump, and the fake socket query path: exit 0, 4 tests passed. No Docker command was run.
5. `docs/testing/craft/t14/2026-09-29-ctnetlink-exact-flow-fix3-retry1/raw-header-diagnostic.json` records one 20-byte DONE datagram and no CT_NEW. The parent-coordinated live diagnostic is separate from this review.

## Checkpoint identity and limits

- `helper.py` SHA-256: `0f129de95e945ee9bf69ccab5f2493a49d9a3c1709f79a68ff646012972ed4c2`.
- `tests/test_controller_integration.py` SHA-256: `8f3dd869a4044f3d130e91934b90e7a512bb78343176546264ba0ca9f57dc724`.
- Both source files remain untracked within the larger worktree, so this verdict concerns their inspected current contents and focused behavior; a Git diff alone cannot isolate the fix4 delta. Runtime namespace feasibility and full T14 acceptance are outside this checkpoint verdict.
