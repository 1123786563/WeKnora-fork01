# R205 chat batch-menu interaction

- Scope: platform chat session sidebar batch-management entry.
- Vue baseline: `menu.vue` exposes `batchManage` from each session row's more menu; the bottom batch footer is rendered only while `batchMode` is active.
- Finding: React previously rendered a persistent bottom `批量管理` trigger, including the empty/new-session sidebar state.
- Change: React now places the batch entry in each row menu and renders the footer only in batch mode, preserving accessible selection, delete confirmation, failure retry and cancel behavior.
- Browser evidence: live React DOM exposes `批量管理` as a row `role=menuitem` and no standalone footer trigger before entering batch mode.
- Validation: shell/session focused tests 18/18, Web typecheck and `git diff --check` pass.
- Boundary: protected live session deletion and responsive/desktop shell evidence remain open.
