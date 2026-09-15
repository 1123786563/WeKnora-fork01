# R185 organization rail layout

- Scope: `apps/web/src/organizations/OrganizationsPage.tsx`, shared-space list default empty state.
- Vue baseline: `ListSpaceSidebar.vue` collapsed organization rail is `56px` wide; `.icon-strip` uses `padding:12px 0 6px`, `gap:4px`; each labeled item is `46px` wide. The list content starts after the rail and uses `padding:20px 28px 0`.
- Change: React removed the page-root `mx-auto` shrink-to-fit behavior and aligned the organization rail to the Vue collapsed dimensions and spacing.
- Browser evidence (Chrome, 1355x720, zh-CN, same seeded account/data): React rail `x=260,w=56,padding=12px 0 6px`; content `x=316,w=1039`; heading `x=344,y=20`, matching Vue.
- Validation: organization suite `26/26`, Web typecheck, and `git diff --check` pass.
- Boundary: organization create/join/edit protected mutations, expanded/dragged rail states, responsive widths, and desktop-native acceptance remain open.
