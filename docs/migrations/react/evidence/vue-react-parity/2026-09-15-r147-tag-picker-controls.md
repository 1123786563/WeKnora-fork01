# R147 tag picker shared controls

- Scope: `apps/web/src/documents/TagPickerDialog.tsx`
- Change: tag search/create fields and batch cancel/confirm actions now use shared `Input`/`Button`, preserving selection state, localized labels, disabled confirmation, and tag callbacks.
- Validation: Web typecheck passed; first full regression hit a transient Contextual Guide timing failure, the identical rerun passed 895/895.
- Boundary: protected tag CRUD and same-session Vue visual comparison remain open.
