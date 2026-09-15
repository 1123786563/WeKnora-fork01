# R165 upload graph inputs

- Scope: `apps/web/src/documents/KnowledgeDocumentsPage.tsx`.
- Change: ASR model fallback and graph tag entry fields now use shared `Input`, preserving required validation, localized labels, keyboard handling and controlled updates.
- Validation: documents focused suite passed 121/121; Web typecheck and diff check passed.
- Boundary: protected upload/graph writes and same-session Vue visual comparison remain open.
