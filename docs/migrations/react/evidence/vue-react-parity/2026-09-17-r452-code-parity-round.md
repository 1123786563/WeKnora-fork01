# 2026-09-17 Round R452 — GitLab projects landed, R449-R451 browser regression (3 parallel agents)

Round type: TDD round with 3 parallel agents (A2 GitLab projects implementation, A1 browser regression
evidence, A3 verifier) — an earlier dispatch attempt was interrupted before execution and re-dispatched.
Verdict: **A2 PASS** (source-level three-way shape verification, not mock-trusting — the R451 lesson applied);
A1's live evidence all green with five new diffs recorded. Gates: test:web 1634/1634 (+8), test:shared
606/606, typecheck clean, build ✓.

## A2 — GitLab projects multi-select (PASS; the last datasource backlog item)

Structured-channel approach (option a): `gitlabProjects` draft rows serialize into
`config.settings.projects = [{ project_id, ref, paths }]` — character-for-character identical to Vue
`syncGitLabProjectsToSettings` (DataSourceEditorDialog.vue L213-221: trim, empty-row filter, paths split on
[\n,]), verified by the verifier against the Vue source AND the backend gitlab connector's parseConfig. A
hand-typed `projects = …` line in the settings textarea is overridden by the structured channel (and the
channel deletes the manual key), scalar keys like verify_ssl survive, non-gitlab connectors never grow the
key. UI: per-project rows (必填 project_id, ref, paths textarea, 添加项目/删除), gitlab type seeds one empty row,
save blocks with projectRequired when rows exist but ids are empty. api-client needed ZERO changes.
data-sources 51/51 (+8); the 13 gitlab i18n keys already existed ×5 locale (guard stays 183).

## A1 — Browser regression over R449-R451 (PASS, 18 screenshots)

- **Lite gating VERIFIED LIVE on both ends**: writing `weknora_lite_mode='true'` hides the sidebar 共享空间
  entry, shows the Lite badge, and hides the user-menu space shortcuts + 退出; removing the key restores
  everything; Vue behaves identically point-for-point. localStorage cleaned and verified null on both ends
  afterwards.
- Datasource form: gitlab shows the R450 专用 labels (GitLab 地址/个人访问令牌/gitlab.example.com placeholder)
  plus the new structured projects rows; feishu credential hints match Vue verbatim. The configured-state of
  the three-state machine could not be observed (no configured connector exists) — recorded.
- User menus match on both ends for the parity account (系统管理 correctly absent — not a platform admin).
- **New diffs recorded (R453+ queue)**: ① React datasource empty-state card has untranslated English
  (No data sources / Add an external connector / Sync status) — i18n not wired on that surface; ② Vue's feishu
  credential step has a 配置指引/打开文档 prereq block React lacks; ③ React's user menu carries 新手引导 which
  Vue keeps in the sidebar shortcut area; ④ the editor interaction is Vue's four-step wizard vs React's inline
  page (arch-level, fields/hints equal); ⑤ untestable due to environment: configured state, lite tenant
  switcher, admin session-source entry.

## Commit-integrity follow-up (orchestrator)

The verifier flagged the dirty packages/i18n files as R451 leftovers — confirmed: HEAD's key-count guard read
180 while R451's committed code consumes the 183-key file (a fresh checkout of c8f3f13f would fail the guard;
local gates stayed green only because tests read the worktree). Committed as `311abf5d` (guard 180→183).
This is the third occurrence of the missed-file pattern (R442/R444/R451) — the per-round
"untracked-or-dirty files referenced by committed code" check must include MODIFIED shared files, not only
untracked ones.

## Gates (final)

`pnpm test:web` 1634/1634 (+8), `pnpm test:shared` 606/606, `pnpm typecheck:web` clean, `pnpm build:web` ✓
(verifier's final run; baseline all green). No Vue, mobile, or Go code modified. Per-agent reports:
.omc/state/r452/report-A{1,2}.md + report-A3-review.md (session artifacts).
