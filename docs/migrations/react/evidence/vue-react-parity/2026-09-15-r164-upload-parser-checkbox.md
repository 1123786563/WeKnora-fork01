# R164 upload parser checkbox

- Scope: `apps/web/src/documents/KnowledgeDocumentsPage.tsx`.
- Change: upload parser first-row-header toggle now uses shared `Checkbox`, preserving parser rule state, disabled behavior and update callback.
- Validation: documents focused suite passed 121/121; Web typecheck and diff check passed.
- Boundary: protected upload/reparse runtime and same-session Vue visual comparison remain open.
