# R184 settings wide content wrapper

- Scope: `apps/web/src/settings/SettingsPage.tsx` and `apps/web/src/styles.css`, settings drawer members/system/integration sections.
- Vue baseline: `frontend/src/views/settings/Settings.vue` applies `content-wrapper--wide` to `members` (`padding:32px 36px 40px`) and `content-wrapper--full` to system-admin/integration sections (`30px 34px 40px`).
- Change: React now selects the same wrapper variants and defines the corresponding Tailwind-compatible CSS bridge; the system-admin section set is explicit and preserves existing role-gated rendering.
- Browser evidence (Chrome, 1355x720, zh-CN, same seeded account/data): Vue and React member title both resolve to `x=382.5,y=52`; React wrapper computed as `max-width:none`, `padding:32px 36px 40px`.
- Validation: settings suite `141/141`, Web typecheck, and `git diff --check` pass.
- Boundary: protected member mutations, all settings sections at responsive widths, and desktop-native acceptance remain open.
