# Live Paseo terminal/task map — 2026-09-30 02:13 CST

Source: `/tmp/issue72-live-terminal-map-20260930-0213.md` (SHA-256 `a97b0c1cdeeadac4213ab59482a11004358436bf98e14df860c9b50e0a836ca4`). Read-only capture timestamp: `2026-09-29T18:13:28Z`.

The PID-to-terminal association below comes directly from the whitelisted `PASEO_TERMINAL_ID` process environment. Task identity is classified from the captured terminal output. All four processes had cwd `/Users/wuyongjun/trea/WeKnora-fork01`, and all were in local Paseo workspace `wks_62501f565cdc04d8`. Workspace title is generic and is not used as task evidence.

| TTY / PID | Paseo terminal ID | Captured task/session | Directly supported worktree evidence | Status implication |
|---|---|---|---|---|
| `ttys001` / `78779` | `77f4de37-00a5-4173-82c6-0a770cae885e` | Issue #72 OCR continuation; capture shows resumed OCR session and repeated waiting for terminal result. | Process cwd is repository root. No dedicated task worktree is proven by captured tail. | Review/verification activity; waiting does not establish lane release. |
| `ttys002` / `81039` | `b24b67dc-7f00-4c25-be0d-d681763d5770` | Active controller session handling Issue #72 recovery; capture includes process-map commands and child-agent results. | Process cwd is repository root; this documentation work occurs separately in `.worktrees-issue72/parallel-refresh-20260929`. | #72 recovery/controller documentation activity; no production lane release. |
| `ttys003` / `84486` | `05583066-801b-4e25-a267-bdc09d36d562` | Issue #140 final OCR and migration evidence check; capture references `2026-09-30-issue140-final-ocr-fresh.md`. | Process cwd is repository root. No narrower worktree path confirmed by captured lines. | Separate Issue #140 activity; no #72 lane release. |
| `ttys004` / `88833` | `bf9caa26-07d4-4de0-8a65-699a93aae68e` | Craft #107 integration audit/ledger activity. | Capture explicitly shows an edited file under `/Users/wuyongjun/.codex/worktrees/craft-107-integration/WeKnora-fork01/docs/plans/2026-09-23-craft-107-ledger.md`. | Separate Craft #107 worktree; no #72 lane release. |

The capture also reports all four processes as `codex`, parent PID `30152`. `paseo terminal ls --all --json` returned all four Codex terminals plus unrelated Claude terminals. `paseo --json ls` showed only an unrelated idle `deepseek-harness` agent. These observations identify the captured shells as #72 OCR, the current #72 recovery controller, #140, and Craft #107 activity. The #72 activity is OCR/recovery documentation activity, not the `lago-int` production implementation lane. No production lane was released or changed. This evidence does not identify an exact #72 implementation worktree and cannot establish that any other #72 worktree is free.

R-6 remains explicit published Billable Metric-to-dimension ownership, with ambiguity failing closed only for the affected dimension. R-3 continues excluding Lago Usage Charges from paid plan purchases. #87 remains gated by #86 acceptance and Lago Task 0 runtime contract evidence. No readiness or acceptance gate is promoted by this terminal map.

## Final OCR coverage record

The exact output artifact is [`issue-72-r16-task0-parent-config-audit-2026-09-30-final-ocr.md`](issue-72-r16-task0-parent-config-audit-2026-09-30-final-ocr.md), SHA-256 `7208a3cba954d78c4264cedc0966ef968440df4866b44fd716289c842b75b737`. It contains `Review skipped: no items were selected.` The full task range was BASE `59ad12e0f6b445186894a4a9cd8ee8422a39dddf` to HEAD `2ef29a67a06c42e8a55a844879670dc7bf309821`, session `a01f3ae3-1274-4dc8-b103-7a7487a7d93c`, exit code 0, and 0 files reviewed. This is incomplete OCR coverage, not a pass.
