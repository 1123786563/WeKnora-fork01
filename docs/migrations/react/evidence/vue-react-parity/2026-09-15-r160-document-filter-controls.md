# R160 document filter controls

- Scope: `apps/web/src/documents/KnowledgeDocumentsPage.tsx`.
- Change: document search, file-type/parse-status/source filters and date-range fields now use shared `Input`/`Select` controls, preserving Vue filter semantics, localized labels and controlled query state.
- Validation: documents focused suite passed 121/121; Web typecheck and diff check passed.
- Boundary: browser filter interaction and same-session Vue visual comparison remain open.
