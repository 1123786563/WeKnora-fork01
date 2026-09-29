# Task 1 Report — Explain DB container setup failure

## Change

Changed only the empty DB-container preflight message in `docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh`. The diagnostic now explains that the local stack may not be running and recommends `lab.sh up`. The existing `return 1 2>/dev/null || exit 1` fail-closed behavior is unchanged.

Updated the Issue #82 ledger with the change and verification record.

## Verification

- `bash -n docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh` — passed.
- `git diff --check` — passed.
- No live stack was used; no live T9/AC3/AC4 evidence is claimed.

## Finding disposition

The low OCR diagnostic finding is addressed by the explicit local recovery hint. No credentials or secret values were added to the message.
