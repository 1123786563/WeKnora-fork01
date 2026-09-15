# R157 document graph controls

- Scope: `apps/web/src/documents/KnowledgeDocumentsPage.tsx`.
- Change: graph tags and relation filter fields now use shared `Input`/`Textarea` controls, preserving combobox semantics, forwarded focus, controlled values and Vue graph editing behavior.
- Validation: documents focused suite passed 121/121; Web typecheck and diff check passed.
- Boundary: protected document graph writes and same-session Vue visual comparison remain open.
