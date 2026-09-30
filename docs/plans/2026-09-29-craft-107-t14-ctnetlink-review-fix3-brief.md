# Task Brief: T14 ctnetlink fake-socket review coverage

- Plan: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix3-plan.md`
- Prior implementation/report/review: fix2 plan/report/review and original ctnetlink plan/review in `docs/plans/`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` plus uncommitted T14 work.
- Owned file: `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; report `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix3-report.md`.
- Role: mechanical_worker (test-only one-file task); independent backend validator and reviewer assigned by parent.
- Commit policy: uncommitted; do not stage/commit. Preserve all unrelated files.
- Starting test SHA-256: `e4dbd65ad5f7b7ec34c995583740a0958bff3100ce2ea8695059cb39fd54fbb5`. Production helper SHA-256 expected unchanged: `b89f1fd93c7368f4e4d9aecfec0dba1e875b72459506fabda261b54eaaff4958`.
- Review findings: success query-path test must send 20-byte NLMSG_DONE (4-byte zero status), assert complete proof object; failure path must prove socket closed after rejected NLMSG_ERROR.
- Test selector: `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests.test_require_tracked_webdriver_flow_uses_linux_netfilter_protocol -v`; full parser selector also required. Then barrier adapter tests, py_compile, diff-check plus explicit no-index check.
- This test-only task intentionally has no functional RED requirement against the already-fixed implementation; do not perturb production code to manufacture a failure. Prior RED/GREEN evidence remains in fix1/fix2 reports.
- No Docker, unittest discovery, daemon/image operations, staging, or commits. Live exact-flow is parent-coordinated after independent gates.
