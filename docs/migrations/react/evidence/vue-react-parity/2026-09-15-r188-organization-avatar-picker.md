# R188 organization avatar picker

- Scope: create shared-space basic-information form.
- Vue baseline: `OrganizationSettingsModal.vue` exposes the Emoji avatar picker with the retained `emoji:<value>` form representation, selected avatar preview, and clear action.
- Change: React adds the same 30-option Emoji picker using Tailwind controls, previews the selected avatar through `SpaceAvatar`, exposes the localized picker/clear labels, and includes the selected avatar in the create payload.
- Browser evidence (Chrome, 1355x720, zh-CN): picker opens from `选择 Emoji 作为共享空间头像`; selecting `🚀` updates the rendered avatar and the clear action is available for a non-empty selection.
- Validation: organization suite `26/26`, Web typecheck, and `git diff --check` pass.
- Boundary: protected create success/failure, avatar persistence after reload, join-mode modal, responsive widths, and desktop-native acceptance remain open.
