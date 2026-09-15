# R323 upload source icon

React `UploadSourceDropdown` now uses an inline 24px SVG document-plus icon for the compact “继续添加” control, matching the Vue upload-source affordance and avoiding a text full-width-plus glyph that rendered with platform-dependent geometry.

Verification:

- `upload-confirm-dialog.test.tsx`: 37/37 passed.
- The regression assertion requires the SVG viewBox and rejects the previous `＋` glyph.
- Web typecheck and `git diff --check` passed; the Web suite remains 911/911 after this change.

Evidence boundary: this is source and component-test evidence. Authenticated Vue/React computed-style capture, protected upload mutation, and Wails/native upload evidence remain open under N007.
