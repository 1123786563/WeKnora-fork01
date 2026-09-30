# Task Brief: T14 ctnetlink response local port-ID fix

- Plan: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix4-plan.md`
- Prior fix reports/reviews 1–3 and exact-flow failure report/evidence are in `docs/plans/` and `docs/testing/craft/t14/`.
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` with prior uncommitted T14 source/tests.
- Owned files: `deploy/craft/render-boundary/policy-helper/helper.py`; `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; report `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix4-report.md`.
- Role: backend_implementer; parent dispatches backend_validator and independent reviewer.
- Commit strategy: uncommitted only; no staging/commit. Preserve unrelated worktree artifacts.
- Start hashes: helper `b89f1fd93c7368f4e4d9aecfec0dba1e875b72459506fabda261b54eaaff4958`; test `2576671650710e3fbf86e93206473534354573f7b35c5df93238b67614fb8281`.
- Raw diagnostic: recvmsg address PID 0; NLMSG_DONE header 20 bytes, type=3, flags=2, sequence=10828, message header PID=1, payload/status 4/0. Evidence path `docs/testing/craft/t14/2026-09-29-ctnetlink-exact-flow-fix3-retry1/raw-header-diagnostic.json`.
- Root cause: parser conflates kernel sender PID from sockaddr with `nlmsg_pid` in ctnetlink response; Linux ctnetlink emits response header using requesting socket port ID. Validate sender address == 0, sequence exact, and header PID == bound socket `getsockname()[0]`.
- Required RED/GREEN: parser tests for nonzero matching local port ID accepted, mismatch rejected; fake socket implements `getsockname()` and returns matching header PID; preserve existing exact-proof and socket-close checks.
- Run exact query/parser selectors, full parser selector, barrier adapter selector, py_compile and no-index diff checks only. No Docker/unittest discovery/image/daemon work.
- Expected task report includes full hashes/results and notes no live-flow in this task. Parent runs disposable test after Review/validation.
