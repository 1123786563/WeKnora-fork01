# T14 ctnetlink review-fix Task 1 Report

## Scope and baseline

Implemented only Task 1 from `2026-09-29-craft-107-t14-ctnetlink-review-fix1-plan.md` in `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`. Owned changes are limited to `deploy/craft/render-boundary/policy-helper/helper.py`, `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`, and this report. Other worktree edits/artifacts were left intact. No staging or commit was performed.

Starting source SHA-256 values matched the brief:

- `helper.py`: `128a8d6f6173030cb17092ec37fa276fcb0001e0cd21ad0e017db59152f2d6c5`
- `tests/test_controller_integration.py`: `ff334bef8d3c54b40ddc006ed9bf9444c80c70d8f9dce05b218ccd469a178090`

## Changes

- Added raw-message parser cases for a 20-byte `NLMSG_DONE` with signed zero status, a nonzero completion status, and zero/nonzero `NLMSG_ERROR` payloads.
- The parser now permits `NLMSG_DONE` with either no payload or exactly one signed 32-bit status payload, requiring zero when present. Other payload lengths and nonzero status remain fail-closed.
- Every `NLMSG_ERROR` now fails closed, including a zero-error ACK.
- Existing sender, sequence, interruption, truncation, completion, tuple, state, and resource checks were not changed.

## TDD and verification evidence

RED, before modifying `helper.py`:

- Command, from `deploy/craft/render-boundary/policy-helper`: `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v`
- Result: 12 tests ran; the new realistic zero-status 20-byte completion errored with `RuntimeError: malformed NLMSG_DONE`; the new zero-error ACK case failed because no `RuntimeError` was raised. The nonzero completion and nonzero error rejection cases passed.

GREEN and required targeted checks, after the parser change:

- `python3 -m unittest tests.test_controller_integration.PolicyTargetCounterUnitTests -v` — 12 tests passed.
- `python3 -m unittest tests.test_barrier_adapter -v` — 18 tests passed.
- `python3 -m py_compile helper.py controller.py barrier_adapter.py` — passed (exit 0, no output).
- `git diff --check -- deploy/craft/render-boundary/policy-helper/helper.py deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py docs/plans/2026-09-29-craft-107-t14-ctnetlink-review-fix1-report.md` — passed (exit 0, no output). Since the owned source files are untracked in this worktree, also ran `git diff --no-index --check /dev/null <file>` for each; each returned exit 1 solely for the expected file difference, with no whitespace diagnostics.

No unittest discovery, Docker command, or daemon/image mutation was run.

## Final hashes and limitations

- `helper.py`: `e9a36006796055cd11fc26ae37f2d53ea08f7a07e7a2cd56ea29b82649ee1b96`
- `tests/test_controller_integration.py`: `5f2a5907e24b572a0bcb369c92ce8d453caf4a97cfc39db5657a389c0a1e2d41`

The two source files remain untracked T14 outputs, and unrelated worktree modifications/artifacts remain untouched. The exact disposable renderer flow (existing connection reuse, fresh same-listener denial, counter delta, cleanup), and real socket query-path behavior were not run here; as assigned, they remain for parent coordination after source review and validator pass. No task-only patch hash was generated because the worktree already contained these untracked source files and their preimages were not preserved.
