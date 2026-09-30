# Task Brief: T14 ctnetlink framing review fixes

- Plan: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix1-plan.md`
- Original implementation plan/report/review: `docs/plans/2026-09-29-craft-107-t14-ctnetlink-plan.md`, `docs/plans/2026-09-29-craft-107-t14-ctnetlink-report.md`, `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review.md`
- Worktree: `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`
- Current revision: `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c` (uncommitted task files)
- Owned files: `deploy/craft/render-boundary/policy-helper/helper.py`, `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`, and this task report only.
- Role: `backend_implementer`; validator: `backend_validator`; parent dispatches independent review.
- Commit policy: no staging or commits. Preserve unrelated worktree files and existing diagnostics; do not revert them.
- Starting source SHA-256: helper.py `128a8d6f6173030cb17092ec37fa276fcb0001e0cd21ad0e017db59152f2d6c5`; test file `ff334bef8d3c54b40ddc006ed9bf9444c80c70d8f9dce05b218ccd469a178090`.
- Findings to fix: (1) a realistic 20-byte NLMSG_DONE (4-byte zero status payload) is rejected; (2) a zero-error NLMSG_ERROR ACK is accepted even though this dump requires NLMSG_DONE.
- Required TDD: add both response fixtures and prove RED before production edits; then GREEN. Keep all framing, sender, sequence, interruption, completion, truncation, resource, tuple and state checks fail-closed.
- Required checks: policy parser selector, barrier adapter selector, py_compile for helper/controller/adapter, diff-check. Do not use unittest discovery; it unexpectedly triggers Docker class setup and image builds.
- Do not run Docker or alter daemon/image state. Exact disposable-flow verification is reserved for the parent after source review and validator pass.
- Report exact post hashes, command output, and any deviations at `docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix1-report.md` in the T14 worktree. No task-only patch hash is available for the parent implementation because its preimage files were not preserved; do not fabricate one.
