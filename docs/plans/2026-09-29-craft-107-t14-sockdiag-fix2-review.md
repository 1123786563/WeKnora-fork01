# Independent review — T14 SOCK_DIAG Fix 2

Reviewed 2026-09-29. Scope: the one-file disposable integration-test change described in the Fix 2 brief, plan, and implementation report. Facts were checked against the approved Craft Spec (stories 12/14 and the no-egress acceptance), `CONTEXT.md`, ADR 0004, the SOCK_DIAG ACK-guard architecture ruling, and the preceding independent review. No Docker, OCR, test discovery, production edit, or remote Issue write was performed.

| Source | Before SHA-256 (pinned task baseline) | Reviewed SHA-256 |
| --- | --- | --- |
| `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` | `4026dc762d2e5f7c5535c4c0874279300fb110d9a06042739ab3f77fd1333167` | `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d` |

The test file is untracked, so Git has no baseline blob or task-scoped diff. The baseline hash is the pinned Fix 1 checkpoint, while the reviewed hash was independently measured. This review covers the reported additions and their interaction with the retained test. HEAD was `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`.

## Finding

1. **High — the expected dropped SYN cannot pass the same-port proof when `connect()` times out.** In `tests/test_controller_integration.py:219-226`, the generated probe sets a 500 ms socket timeout, catches `OSError`, and stores only `x.errno`. `socket.timeout("timed out").errno` is `None` (independently checked with Python). The validator at `:197-210` requires `type(connect_errno) is int` and `connect_errno > 0`. A policy-dropped SYN normally completes by timeout, so even with successful old-port bind, a positive loopback-drop delta, and zero ACK-rule deltas, the test writes `passed: false` and fails at `:1006-1020`. This prevents the required live acceptance and T15 release on the successful denial path. **Smallest correction:** record timeout as an explicit connect outcome and accept that bounded timeout as a denial when the exact bind/endpoints, positive drop delta, and unchanged ACK counters all hold. Keep positive errno handling for immediate socket errors, but do not equate every positive local errno with a policy-denied SYN. Add a pure timeout case to the validator tests.

## Scope checks

- The selector at `:83-189` demands exactly two output-chain candidates containing a TCP-flags payload; each must have the four exact directional address/port matches, the literal `flags & (syn | ack) == ack` AST, one packet counter, one accept, and a distinct integer handle. Its focused tests include missing/duplicate directions, broad source, malformed counter, wrong ACK AST, and duplicate handles. These checks align with the helper's emitted two rules at `helper.py:111-138`. A current nft JSON sample from this repository confirms the address/port match and counter object shapes; no live sample of the new flag expression was produced in this review.
- The live test snapshots raw rule JSON immediately before `USR1` and after the `reuse-ok` marker, requires stable handles and positive deltas for both directions (`:937-975`). The signal handler uses the existing `w` FD (`:760-764`). The new `USR2` handler closes that FD and writes the close marker before the same-port attempt (`:977-988`).
- The probe binds the attested client address and port, attempts the attested destination, and records bind/connect outcomes and elapsed time (`:213-228`). The surrounding test records pre/post snapshots, requires a loopback-drop increase and no ACK-rule growth, and writes the failure receipt before its final assertion (`:990-1020`). If the Python subprocess itself fails or its output is invalid JSON, the test raises before writing `same-port-syn-proof.json`; the brief's failure-receipt requirement is therefore only met for a successfully emitted probe JSON record. This is a **low** diagnostic gap; capture return code/stdout/stderr in a failed receipt before asserting or parsing.
- The pre-existing different-source listener canary remains asserted at `:931-936`; the later preview, target isolation, no-egress, and cleanup path remains after the new block. This was source review only; the parent must still run the exact live selector and inspect correlated receipts.

## Verdicts

**Spec compliance: FAIL for Fix 2 acceptance.** The required same-port fresh-SYN denial proof is not reliably attainable for the normal drop/timeout result, despite the correct tuple, ACK AST, and counter-window checks. A live run has not occurred, so T14 remains unverified.

**Code quality: FAIL pending the high finding.** The pure tests pass according to the implementation report, but they model denial only as integer errno 111 and omit the timeout branch produced by their own 500 ms probe. The low failure-receipt gap should be addressed with the same focused fix if practical.
