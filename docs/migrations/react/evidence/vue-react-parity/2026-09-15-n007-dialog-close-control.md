# N007 upload-confirm close control

## Vue baseline

Vue `UploadConfirmDialog.vue` defines the close control as a 32px square at the modal's top-right offset by 20px. It uses a 20px icon, 6px radius, secondary surface background, and centered content.

## React change

The specialized React upload-confirm dialog now overrides the shared close button with an explicit 32px by 32px flex box, zero padding, centered alignment, 20px icon size, 1 line-height, 6px radius, and the Vue-matched muted surface token. The rule is scoped to `.wk-upload-confirm-dialog`.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type/static evidence. Authenticated post-change browser computed-style/screenshot, real upload backend, and Wails/native evidence remain open.
