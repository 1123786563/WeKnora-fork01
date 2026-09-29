# Independent Review — T14 ctnetlink review-fix1

Reviewed 2026-09-29. Read-only source review of the repair in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01` against the approved Craft web artifact spec (stories 12 and 14, offline preview boundary), `CONTEXT.md`, ADR 0004, the ctnetlink implementation and fix plans, the fix brief, and the prior independent review. No source, test source, requirements, issue, Git index, or Docker state was changed. This record is in the integration checkout.

## Verdict

- **Scoped spec compliance: PASS.** The two parser defects identified in the prior review are corrected. The parser accepts `NLMSG_DONE` of 16 bytes or 20 bytes with a native signed zero status, rejects nonzero status and other lengths, and rejects every `NLMSG_ERROR`, including a zero ACK. The existing sender, sequence, interruption, truncation, completion, byte/message/record limits, and exact single established tuple checks remain in the inspected path.
- **Scoped code quality: PASS.** The completion/status branch is small and explicit, and focused behavioral tests cover zero and nonzero 20-byte completions and zero and nonzero error frames. No new high or medium finding was found in the repair.
- **Full T14 acceptance: PENDING.** The prior review's query-path testing gap and required disposable old-flow reuse, fresh same-listener denial, positive counter delta, and cleanup evidence remain outside this repair and were not independently run here. This scoped pass does not verify the live socket query or full T14 behavior.

## Evidence and finding disposition

| Prior finding | Disposition and evidence |
| --- | --- |
| High: valid 20-byte `NLMSG_DONE` rejected | Resolved at `helper.py:264-269`: payload length is restricted to zero or exactly four bytes; `struct.unpack_from("=i", ...)` requires zero. Positive and nonzero fixtures are at `tests/test_controller_integration.py:162-183`. |
| Medium: zero `NLMSG_ERROR` ACK accepted | Resolved at `helper.py:262-263`: all messages of this type raise before payload interpretation. Zero and nonzero fixtures are at `tests/test_controller_integration.py:185-205`. |
| Medium: query-path and live acceptance unverified | Carried forward, not a new repair finding. `require_tracked_webdriver_flow()` at `helper.py:300-334` still has no isolated fake-socket test; focused tests call `parse_ctnetlink_dump()` directly. The required disposable exact-flow check is reserved for parent coordination. Smallest defensible correction before full T14 sign-off: add a fake-socket query-path test and run the specified disposable integration acceptance with cleanup evidence. |

The parser checks the datagram sender and expected sequence at `helper.py:235-238`, then validates each message's sequence and kernel PID at `:255-257`; it rejects dump interruption at `:260-261`, data after completion at `:271-273`, missing completion at `:283-284`, and absent or ambiguous exact established matches at `:285-290`. Existing resource bounds remain at `:229-245` and `:278-280`. These observations are of the current complete files; because both source files are untracked and their preimages were not saved, they cannot establish a line-level task-only delta.

## Provenance and checks

- Worktree HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; both owned source files remain untracked.
- Fix brief starting SHA-256: `helper.py` `128a8d6f6173030cb17092ec37fa276fcb0001e0cd21ad0e017db59152f2d6c5`; test file `ff334bef8d3c54b40ddc006ed9bf9444c80c70d8f9dce05b218ccd469a178090`. These are recorded provenance only; no preserved preimages exist for an independent task-only diff.
- Recomputed current SHA-256: `helper.py` `e9a36006796055cd11fc26ae37f2d53ea08f7a07e7a2cd56ea29b82649ee1b96`; test file `5f2a5907e24b572a0bcb369c92ce8d453caf4a97cfc39db5657a389c0a1e2d41`. They match the implementer report.
- Independently ran `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -q`: 12 passed; `python3 -m unittest tests.test_barrier_adapter -q`: 18 passed; `python3 -m py_compile helper.py controller.py barrier_adapter.py`: exit 0. No unittest discovery, Docker, or OCR was run.
