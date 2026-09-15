# N007 upload configuration content padding

## Vue baseline

`UploadConfirmDialog.vue` uses `.content-wrapper { padding: 22px 32px 28px; overflow-y: auto; }` on desktop and `padding: 16px` at the 800px responsive breakpoint.

## React change

The dedicated React upload configuration panel now uses `padding: 22px 32px 28px` with the existing independent overflow container and keeps the 16px narrow-screen override.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

This is Vue source comparison plus React unit/type evidence. Authenticated post-change browser screenshot/computed-style and real upload/backend evidence remain open.
