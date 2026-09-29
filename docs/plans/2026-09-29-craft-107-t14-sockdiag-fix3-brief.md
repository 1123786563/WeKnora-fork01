# Task Brief — T14 same-port timeout evidence Fix 3

**Review:** Fix 2 independent Review high finding in `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix2-review.md`.

**Plan:** `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix3-plan.md`.

**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`; HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; test baseline SHA `a7f47443efbc9132b369b1a5e80848f7351a36d4878df425eb4ef22e28bfca3d`. Before source snapshot is preserved at `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/before/test_controller_integration.py` with same SHA.

**Role:** `backend_implementer`. **Validator:** `backend_validator`. Parent assigns independent reviewer.

**Write ownership:** only `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py` and `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix3-report.md`; task snapshots/patch only under `.superpowers/sdd/2026-09-23-craft-107-implementation/t14-fix3/`.

**Task:** TDD recognize bounded `TimeoutError`/`socket.timeout` without errno only when bind succeeded, connect was attempted and did not connect, exact endpoints match, timeout is bounded, exact loopback-drop delta is positive, and both ACK exception deltas are zero. Keep positive errno denial supported. Persist `passed:false` receipts before assertion on subprocess errors or invalid JSON. Do not change production code or weaken any canary.

**Checks:** pure same-port validator selector; all `PolicyTargetCounterUnitTests`; `py_compile`; `git diff --check`. No Docker/discovery.

**Deliver:** exact pre/post hashes, before copy, task `.patch`, RED/GREEN commands and results, failure receipt examples, scope. No live acceptance claim.
