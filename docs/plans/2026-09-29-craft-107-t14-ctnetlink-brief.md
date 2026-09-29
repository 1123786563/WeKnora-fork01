# Task Brief — T14 ctnetlink exact WebDriver flow

- **Root Issue/Ticket:** Craft #107 / T14 #129, stories 12 and 14; exact blocker recovered from `docs/plans/2026-09-29-craft-107-live-issue-refresh.md`.
- **Approved behavior/context:** `docs/specs/2026-09-23-craft-web-artifact-spec.md`, `CONTEXT.md`, `docs/adr/0004-task-is-session.md`, `docs/plans/2026-09-28-craft-107-t14-webdriver-control-policy-plan.md` and its Fix3/validation reports in T14 Worktree; feasibility audit `docs/plans/2026-09-29-craft-107-t14-conntrack-source-audit.md`.
- **Task plan:** `docs/plans/2026-09-29-craft-107-t14-ctnetlink-plan.md` in the integration Worktree.
- **Worktree:** existing `/Users/wuyongjun/.codex/worktrees/craft-107-t14/WeKnora-fork01`; preserve all 300+ unrelated T14 artifacts and the current HEAD `e2f335dc6d8c2a5166be00a8b282b1a02e7cdd9c`.
- **Starting owned-file hashes:** `helper.py` `31be195955c9216ce179e7f2fd5189577d7143acb81a57b70882c76c6de79643`; `tests/test_controller_integration.py` `0f9f69699b5da7ef6bab7437497c666fb0988bb5d2dda082811f887cc7d8b68f`.
- **Commit policy:** no commits or staging. Save task-only before/after source snapshots and patch hash under this Task's SDD directory.
- **Owned files:** only `deploy/craft/render-boundary/policy-helper/helper.py` and `deploy/craft/render-boundary/policy-helper/tests/test_controller_integration.py`; task report path `docs/plans/2026-09-29-craft-107-t14-ctnetlink-report.md`.
- **Forbidden scope:** no renderer capability changes, no broader network, no additional helper capabilities, no package/dependency installation, no edits to controller/barrier/browser/build/migration/shared files, no commits, no shared cache/port/database use.
- **Resource lock:** one Docker/OrbStack slot, one unique disposable renderer/helper/evidence ID, no overlap with other Docker/browser runs. Unit tests are non-Docker and may run while independent Go-only service tests run.
- **Consumes/produces:** preserve seven-field `webdriver_flow`; make helper issue a bounded in-namespace ctnetlink dump under its existing sole `CAP_NET_ADMIN`; return proof only for one exact reverse tuple whose TCP kernel state is ESTABLISHED.
- **Verification:** implement plan Task 1 steps; unit suite, `py_compile`, `git diff --check`, then one exact disposable policy test proving reused old flow, fresh same-listener denial/counter and cleanup. Stop before browser matrix unless proof passes review.
- **Report:** include task-only hashes/diff, exact outputs, container image/caps/network namespace identity, bounded evidence and cleanup. No claim of full T14 until live 37-cell matrix and OCR are complete.
