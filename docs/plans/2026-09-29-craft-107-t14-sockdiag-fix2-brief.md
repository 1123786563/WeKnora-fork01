# Task Brief — T14 SOCK_DIAG live proof fixture Fix 2

**Source:** independent review findings 1 and 3 in `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-review.md`.

**Detailed plan:** `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix2-plan.md`.

**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`; HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; current test baseline SHA `4026dc762d2e5f7c5535c4c0874279300fb110d9a06042739ab3f77fd1333167`. No staging, commit, push, stash, Issue writes, Docker, image build, or discovery.

**Owner role:** `backend_implementer`; **Validator:** `backend_validator`; parent assigns independent reviewer.

**Write ownership:** only `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` and `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix2-report.md`.

**Task:** TDD pure checks that select exactly two complete directional nft tuple rules and assert the exact ACK flag AST/counters; extend only the disposable renderer test fixture with a bounded signal to close the attested client FD; after the same-FD marker, parse immediate pre/post snapshots and assert both exception rule counter deltas; then bind the same source port and verify a fresh SYN is denied with loopback drop delta and no exception delta. Retain existing different-port canary, preview, no-egress and cleanup behavior. Do not edit production code or any other fixture.

**No-Docker checks:** new pure selectors, `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v`, `python3 -m py_compile helper.py tests/test_controller_integration.py`, `git diff --check`. Do not run class setup/discovery.

**Report:** pin before/after test hashes and HEAD; provide task-scoped delta; record RED/GREEN checks and limitations. Parent runs one exact live selector only after Review and independent validation.
