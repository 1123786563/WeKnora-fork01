# T14 SOCK_DIAG live proof fixture Fix 2 validation

Verdict: **DONE_WITH_CONCERNS** for this fixture-only validation. Prescribed no-Docker checks pass and the assigned test source matches the implementer report hash. The parent live acceptance remains unverified by this validation and is still required by the brief.

## Scope and revision

- Brief: `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix2-brief.md`
- Implementer report: `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix2-report.md`
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- HEAD at inspection and after checks: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`
- Test source: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`
- Test source SHA-256 before checks: `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d`
- Test source SHA-256 after checks: `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d`
- Brief baseline SHA-256: `4026dc762d2e5f7c5535c4c0874279300fb110d9a06042739ab3f77fd1333167`
- Source status is untracked at HEAD; the current file hash matches the implementer report's post-change hash. No tracked source was modified during validation.

## Exact commands and results

Executed from `deploy/craft/render-boundary/policy-helper`:

1. `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_nft_selector_requires_two_exact_directional_ack_rules_and_counters tests.test_controller_integration.PolicyTargetCounterUnitTests.test_same_port_probe_validator_requires_bind_denial_and_counter_isolation -v` — exit 0; 2 tests passed.
2. `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — exit 0; 10 tests passed.
3. `python3 -m py_compile helper.py tests/test_controller_integration.py` — exit 0.
4. `git diff --check` — exit 0, with no output. Limitation: the changed test source is untracked, so Git's diff check does not inspect its whitespace.

No Docker, integration selector, test discovery, staging, commit, push, stash, image build, or Issue write was performed.

## Independent findings

- The pure nft selector requires exactly two output candidates containing a TCP flags payload, validates each complete directional IPv4/IPv6 address and TCP port tuple, compares the exact ACK mask AST, requires one valid packet counter and accept expression, and enforces distinct integer handles. Unit cases reject missing/duplicate direction, broad tuple, malformed/missing counters or handle, wrong ACK AST, and duplicate handles.
- The marker proof snapshots nft state before signalling `USR1` and after observing the bounded `reuse-ok` marker; it requires stable handles and positive deltas for both selected directional counters.
- The same-port probe is gated on the `USR2` close acknowledgement. It binds the attested source address and port, bounds connect to 500 ms, records bind/connect outcome and elapsed time, and requires a positive loopback-drop delta with zero deltas for both ACK exception counters. The pre-existing distinct-port canary and surrounding preview/no-egress/cleanup assertions remain in the fixture.
- The new validator also requires source/destination tuple equality, successful bind, attempted but denied connect, positive connect errno, nonnegative elapsed time, positive drop delta, and exactly the two zero ACK counter deltas.

## Acceptance status and risks

- Pure selector behavior and no-Docker fixture checks: **verified**.
- Live `reuse-ok`, positive exact-rule counter deltas, explicit FD-close behavior, same-port successful bind followed by denied SYN, preview/no-egress behavior, evidence attribution, and disposable environment cleanup: **not run here**. These require the single exact live selector reserved for the parent and are explicit brief acceptance conditions.
- Therefore this validation does not establish T14 live acceptance or cleanup. Parent should run the prescribed exact live selector after review and retain its receipts; if same-port bind fails or attribution/cleanup is uncertain, T14 remains unverified as specified by the brief.

No source or test file was edited by the validator; this report is the only validation artifact created.
