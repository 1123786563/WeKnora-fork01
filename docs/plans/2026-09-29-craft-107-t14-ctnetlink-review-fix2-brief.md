# Task Brief: T14 ctnetlink Linux socket constant fallback

- Plan: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix2-plan.md`
- Prior plan/report/review: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix1-plan.md`, `...-fix1-report.md`, `...-fix1-review.md`; original parser plan and review also apply.
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`, with accepted fix1 source changes uncommitted.
- Owned files: `deploy/craft/render-boundary/policy-helper/helper.py`; test `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; report path named below.
- Role: backend_implementer; independent validator and reviewer dispatched by parent.
- Commit strategy: uncommitted checkpoint, no stage/commit. Preserve all unrelated worktree edits and evidence.
- Starting SHA-256 helper: `e9a36006796055cd11fc26ae37f2d53ea08f7a07e7a2cd56ea29b82649ee1b96`; tests: `5f2a5907e24b572a0bcb369c92ce8d453caf4a97cfc39db5657a389c0a1e2d41`.
- Reproduced failure: on built helper image `craft-t14-policy-helper:2026-09-24`, Alpine Python 3.12.14 had `AF_NETLINK=16`, `NETLINK_NETFILTER=None`; real disposable test failed with `AttributeError` before policy install; controller recorded `cleanup status=verified-clean`. Evidence: integration `docs/testing/craft/t14/2026-09-29-ctnetlink-exact-flow/`.
- Linux UAPI defines `NETLINK_NETFILTER` as 12. Production should use named fallback constant `getattr(socket, "NETLINK_NETFILTER", 12)` and keep other behavior unchanged.
- TDD required: implement fake-socket query path test, observe RED due missing attribute, then fix. Test captures constructor protocol, CT_GET request sequence/family, synthetic exact tuple/20-byte completion, result and socket closure.
- Required checks: new test selector, entire `PolicyTargetCounterUnitTests`, `tests.test_barrier_adapter`, `py_compile`, `git diff --check`. No Docker/unittest discovery.
- Report at `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix2-report.md`; exact final hashes and logs required. No code outside owned files.
