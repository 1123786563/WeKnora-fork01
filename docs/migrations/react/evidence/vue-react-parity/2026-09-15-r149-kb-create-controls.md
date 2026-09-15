# R149 knowledge-base create controls

- Scope: `apps/web/src/App.tsx`, `apps/web/src/faq/FAQPage.test.tsx`.
- Change: knowledge-base create/edit form now uses shared `Input`, `Select`, and `Textarea`; FAQ SSR contract accepts the semantic `maxLength` spelling emitted by the shared input primitive while preserving the 200-character limit.
- Validation: Web typecheck passed; Web regression passed 895/895; direct FAQ suite passed 48/48.
- Boundary: protected KB create/edit browser/backend evidence remains open.
