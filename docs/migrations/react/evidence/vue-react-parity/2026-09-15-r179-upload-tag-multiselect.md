# R179 upload tag multi-select

- Scope: `apps/web/src/documents/KnowledgeDocumentsPage.tsx`.
- Change: upload confirmation tag selection now reuses the existing Vue-style `UploadMultiSelect` instead of a native multiple select, preserving selected tag IDs, add/remove interactions and localized empty-state copy.
- Validation: documents focused suite passed 121/121; Web typecheck and diff check passed.
- Boundary: protected upload tag persistence and browser visual comparison remain open.
