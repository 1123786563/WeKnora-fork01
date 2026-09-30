# Independent Review — T14 ctnetlink Task 1

Reviewed 2026-09-29. Scope: current `helper.py` and `tests/test_controller_integration.py` in the T14 worktree, against the approved Craft web artifact spec (stories 12, 14, offline preview boundary), `CONTEXT.md`, ADR 0004, the ctnetlink plan and brief, source audit, and implementer report. No source, requirements, issue, Docker state, or Git index was changed.

## Verdict

- **Spec compliance: FAIL for Task 1 completion.** The parser rejects a completion form observed in the actual kernel capability probe, so the exact flow cannot yet be accepted on that evidence. The required disposable old-flow reuse, fresh-connection denial, counter, and cleanup test was not run. The read-only narrow helper architecture and exact tuple intent align with the plan.
- **Code quality: FAIL pending correction and live verification.** The synthetic parser selector passes (8 tests), but its completion fixture encodes the incompatible 16-byte shape. The actual socket query path has no isolated behavioral test. The parser otherwise enforces substantial sender, sequence, tuple, state, truncation, and size checks.

## Findings

### High — Actual 20-byte `NLMSG_DONE` is rejected

- **Affected:** `deploy/craft/render-boundary/policy-helper/helper.py:265-267`; fixture at `tests/test_controller_integration.py:118`.
- **Evidence:** The source audit records an authorized kernel `NLMSG_DONE` response of 20 bytes from the same ctnetlink request (`docs/plans/2026-09-29-craft-107-t14-conntrack-source-audit.md`, fact 4). The parser requires `len(msg) == 16`; all positive fixtures generate a 16-byte completion. Replacing the fixture completion with `struct.pack('=IHHIIi', 20, 3, 2, 77, 0, 0)` yields `RuntimeError: malformed NLMSG_DONE` in a local read-only reproduction.
- **Impact:** A valid kernel dump with this completion fails closed, preventing policy installation and the existing WebDriver flow acceptance. The required live proof cannot pass with the observed kernel response form.
- **Smallest correction:** Accept the documented 20-byte completion with a zero status payload, while rejecting nonzero status, malformed lengths, extra payload, sequence/sender mismatch, and interrupted dumps. Add a positive test using the observed 20-byte form and negative tests for nonzero/truncated completion; then run the disposable integration acceptance.

### Medium — Successful `NLMSG_ERROR` is accepted despite the plan's reject rule

- **Affected:** `deploy/craft/render-boundary/policy-helper/helper.py:262-264`.
- **Evidence:** `NLMSG_ERROR` raises only when its 32-bit error field is nonzero or short. A zero-error message is silently accepted, whereas plan Review Focus 2 and Global Constraints say to reject `NLMSG_ERROR` and uncertain dumps. No test covers a zero-error frame.
- **Impact:** The reader accepts a response class outside its declared proof protocol. This is unlikely to create a false exact tuple by itself, but it weakens the fail-closed framing invariant.
- **Smallest correction:** Reject every `NLMSG_ERROR` in this dump protocol, or explicitly amend the approved contract if a zero ACK is required by verified kernel behavior; test the chosen rule.

### Medium — Query-path behavior and mandatory live acceptance remain unverified

- **Affected:** `require_tracked_webdriver_flow()` in `helper.py:298-332`; `PolicyTargetCounterUnitTests` and `PolicyControllerIntegrationTests.test_target_counters_cover_random_preview_loopback_and_controlled_dns`.
- **Evidence:** Current tests call `parse_ctnetlink_dump()` directly with constructed message dictionaries; none exercises socket creation, request encoding, `recvmsg`, sender address, deadline, or the handoff into `install()`. The implementer report states the required Docker exact-flow test was not run. I ran only the parser selector: `Ran 8 tests ... OK`; I did not invoke Docker.
- **Impact:** No evidence yet shows that the current helper receives and accepts a real tuple in the renderer namespace, or that the old connection survives policy while a new same-listener connection is denied and counted. The task's expected result remains unproven.
- **Smallest correction:** Add a small fake-socket test around the query path for completion/error/truncation/deadline handling, then run the plan's single disposable exact-flow integration test with capability, namespace, counter, marker, and cleanup evidence.

## Provenance and limits

- Current T14 worktree HEAD: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`.
- Current SHA-256: `helper.py` `128a8d6f6173030cb17092ec37fa276fcb0001e0cd21ad0e017db59152f2d6c5`; `tests/test_controller_integration.py` `ff334bef8d3c54b40ddc006ed9bf9444c80c70d8f9dce05b218ccd469a178090`.
- Both owned files pre-existed as untracked files. The brief and report record starting hashes (`31be195955c9216ce179e7f2fd5189577d7143acb81a57b70882c76c6de79643` and `0f9f69699b5da7ef6bab7437497c666fb0988bb5d2dda082811f887cc7d8b68f`), but no source preimages or task-only patch were preserved. Findings therefore describe the current implementation and cannot prove each line originated in this task.
- This review did not run OCR or Docker and does not make a claim about the full T14 browser matrix or other worktrees.
