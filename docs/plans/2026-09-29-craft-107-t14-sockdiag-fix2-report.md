# T14 SOCK_DIAG live proof fixture Fix 2 report

Status: **implemented; prescribed no-Docker checks pass**. This task only adds live fixture evidence for exact ACK exception rule use and a same-source-port SYN denial attempt. No production/helper/controller files were changed.

## Execution record

- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- HEAD start/end: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Runtime metadata: actual `agent_type`, model and reasoning were not exposed to this task. Parent assignment specifies `backend_implementer` and runtime mapping `gpt-6-luna / low`; no unavailable metadata was inferred.
- Only owned source file changed: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`. This file was untracked at task start, so ordinary `git diff` does not have a Git preimage. Baseline SHA is pinned by the brief/Fix 1 report; final SHA is recorded below. The report is the only other task-created file.

## Source pin and task delta

| File | Before SHA-256 | After SHA-256 |
| --- | --- | --- |
| `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` | `4026dc762d2e5f7c5535c4c0874279300fb110d9a06042739ab3f77fd1333167` | `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d` |

Exact task-scoped delta:

- Added pure fixture helpers that select exactly two output rules carrying the TCP flags match, match each direction's full address/port tuple, require the exact `flags & (syn | ack) == ack` JSON AST, validate exactly one counter and accept expression per rule, and return each direction's integer packet counter and distinct integer nft handle. Missing, malformed, duplicate, broad tuple, wrong ACK AST, bad counter and missing/duplicate handle evidence fail closed.
- Added a pure same-port evidence validator and generated probe code. The disposable probe uses a 500 ms connect timeout and emits bind success/errno, connect attempted/result/errno, endpoints and elapsed time.
- Added USR2 handling to the disposable PID 1 renderer. It closes the original WebDriver client socket and writes a bounded close acknowledgement marker.
- Extended the existing marker test to snapshot raw nft JSON immediately before and after `reuse-ok`, validate stable rule handles, and retain a receipt with per-direction positive counter deltas.
- After confirmed original-FD close, the live fixture binds the attested source address and port, attempts the attested listener destination, snapshots before/after, and requires a loopback-drop increase with zero ACK exception counter deltas. It writes `same-port-syn-proof.json` with `passed` and bind errno before asserting, including failed-bind evidence.
- Existing different-source-port canary, preview, no-egress, target-counter isolation and cleanup assertions were left intact.

The test file was untracked at task start, so `git diff`/a Git patch hash cannot represent its preimage. The source before/after SHA values above, exact owned path, and delta description identify the checkpoint; no unrelated source files were changed.

## RED → GREEN and verification

Commands ran from `deploy/craft/render-boundary/policy-helper`.

- RED: the first two new pure tests failed on the deliberately unimplemented selector and validator (`NotImplementedError`, exit 1).
- RED during implementation: the selector initially found zero candidates because the TCP flags AST is nested; after correcting traversal, the generated same-port probe compile assertion exposed one indentation defect. Both issues were corrected before final verification.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_nft_selector_requires_two_exact_directional_ack_rules_and_counters tests.test_controller_integration.PolicyTargetCounterUnitTests.test_same_port_probe_validator_requires_bind_denial_and_counter_isolation -v` — **passed**, 2 tests.
- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — **passed**, all 10 tests.
- `python3 -m py_compile helper.py tests/test_controller_integration.py` — **passed**.
- `git diff --check` — **passed**. The source remains untracked, so this Git command does not inspect the untracked file's whitespace; the focused tests and `py_compile` did parse the source.

No Docker, image build, discovery, class-level integration setup, staging, commit, push, stash, or Issue write was run. No integration container was started. The exact live selector remains for the parent after independent review/validation.

## Bind-failure evidence and cleanup limits

The probe writes JSON even when the old-port bind fails; `bind_errno` is retained, `connect_attempted` stays false, and the receipt records `passed: false` before the test assertion fails. If bind succeeds but the SYN connects or does not increment the loopback drop counter, the receipt likewise records the observed result and fails the test. The fixture's `tearDown` retains the pre-existing `docker rm -f` cleanup behavior. This task did not execute that integration path, so no runtime cleanup claim is made; the parent live run must inspect its cleanup receipts.
