# N007 upload sidebar header order

## Vue baseline

The Vue upload dialog keeps the title, count/add controls, and destination breadcrumb together in the files-sidebar header. The file list starts only after that header, so the destination never appears below the staged rows.

## React change

React now groups the title and file actions in a header row and places the destination breadcrumb in the same header before `UploadFilesPanel`. The dialog's nested URL editor and all staging callbacks remain unchanged.

## Verification

- Focused upload-confirm tests: 37/37 passed.
- Web typecheck and `git diff --check` passed.

## Evidence boundary

This is source and component-test evidence. Authenticated screenshots/computed-style, real upload/folder mutation and Wails/native evidence remain open.
