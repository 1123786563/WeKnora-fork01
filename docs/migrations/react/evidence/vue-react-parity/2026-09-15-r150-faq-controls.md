# R150 FAQ editor and tag-management controls

- Scope: `apps/web/src/faq/FAQPage.tsx`.
- Change: FAQ editor fields and tag-management search/create/edit controls now use shared `Input`, `Textarea`, `Select`, `Checkbox`, and `Radio` primitives while preserving Vue-shaped labels, limits, selection state, and callbacks.
- Validation: Web regression 895/895 and direct FAQ suite 48/48 pass after commits `0e3cab8b` and `4c0a5db7`.
- Boundary: protected FAQ CRUD/import runtime and same-session Vue visual comparison remain open.
