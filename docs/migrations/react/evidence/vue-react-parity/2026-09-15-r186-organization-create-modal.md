# R186 organization create modal

- Scope: `apps/web/src/organizations/OrganizationsPage.tsx`, create shared-space modal.
- Vue baseline: the live `OrganizationEditorModal.vue` flow renders a `1100px` max modal at the captured viewport, a `208px` navigation column, and a two-column basic-information form; the permissions section is reachable from the left navigation.
- Change: React now uses the Vue modal dimensions, renders the create-mode navigation (`基本信息`/`权限说明`), adds the permissions explanation state, and lays out name/description metadata beside shared `Input`/`Textarea` controls. Existing required validation and create payload remain unchanged.
- Browser evidence (Chrome, 1355x720, zh-CN, same seeded account/data): both modals computed as `x=127.5,y=54,w=1100,h=612`; React sidebar computed `w=208px`, with title `创建共享空间` and the two Vue navigation labels.
- Validation: organization suite `26/26`, Web typecheck, and `git diff --check` pass.
- Boundary: avatar picker parity, protected create success/failure, permissions content completeness, join-mode modal, responsive widths, and desktop-native acceptance remain open.
