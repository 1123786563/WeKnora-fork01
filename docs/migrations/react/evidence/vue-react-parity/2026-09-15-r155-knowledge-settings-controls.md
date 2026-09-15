# R155 knowledge-settings controls

- Scope: `apps/web/src/knowledge-settings/KnowledgeSettingsPage.tsx`.
- Change: knowledge-base basic, parser, chunking, indexing and storage fields now use shared `Input`, `Select`, `Textarea` and `Checkbox` controls, preserving Vue-shaped labels, read-only model bindings, validation and save payloads.
- Validation: focused knowledge-settings suite passed 2/2; Web typecheck and diff check passed.
- Boundary: protected settings save/preview runtime and same-session Vue visual comparison remain open.
