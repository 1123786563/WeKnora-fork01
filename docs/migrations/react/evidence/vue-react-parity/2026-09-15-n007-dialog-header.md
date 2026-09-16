# N007 upload-confirm dialog header placement

## Vue baseline

Vue `UploadConfirmDialog.vue` places the dialog title in the 220px files sidebar header (`min-height: 56px`, 12px padding, 16px/600 title) and positions the 32px close control absolutely at the modal's top-right (`top/right: 20px`). The modal body begins at the top of the three-column layout.

## React change

The upload-confirm Dialog now hides the generic title row visually for this specialized surface, keeps its close control as an absolute overlay, and renders the dialog title in a dedicated 56px files-column header. Other Dialog consumers retain the shared header behavior.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type evidence. Authenticated post-change browser screenshot/computed-style and real upload backend evidence remain open.
