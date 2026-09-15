# R204 chat empty sidebar actions

- Scope: platform chat sidebar with no sessions.
- Vue baseline: `frontend/src/components/menu.vue` renders the batch-management footer only while `batchMode` is active; an empty session list shows only the empty copy and user menu.
- Finding: React rendered the batch-management trigger even when the session list was empty.
- Change: `SessionSidebarList` now suppresses the batch toolbar when `totalItems === 0`; populated lists retain batch selection, delete and retry behavior.
- Validation: shell/session focused tests 18/18, Web typecheck and `git diff --check` pass. Full Web baseline remains 895/895 before this narrow change.
- Boundary: live screenshot exposed the mismatch; protected session mutation parity remains open.
