# Task Brief — T14 SOCK_DIAG review Fix 1

**Source finding:** Independent review finding 2 in `docs/plans/2026-09-29-craft-107-t14-sockdiag-ack-policy-review.md`.

**Plan:** `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix1-plan.md`.

**Worktree:** `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`, current base `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`; use uncommitted checkpoint, no staging/commit/push/stash/Docker.

**Role:** `backend_implementer`. **Validation:** `backend_validator`. Parent assigns independent reviewer.

**Write ownership:** only `deploy/craft/render-boundary/policy-helper/helper.py`, `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`, and `docs/plans/2026-09-29-craft-107-t14-sockdiag-fix1-report.md`.

**Change:** TDD normalize flow addresses with `ipaddress.ip_address(...).compressed` after current family/loopback validation; add an expanded IPv6 loopback input test matched against canonical kernel `::1`. Preserve the seven-field interface and exact tuple checks. Do not address live counter or same-port acceptance findings here.

**Checks:** targeted parser IPv4/IPv6 selectors, focused SOCK_DIAG parser/query selectors, `PolicyTargetCounterUnitTests`, barrier adapter tests, `py_compile`, `git diff --check`. No Docker/discovery.

**Report:** record exact baseline/final hashes, HEAD and diff, RED and GREEN evidence. Preserve unrelated files and the original implementation reports.
