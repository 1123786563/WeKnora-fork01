# R148 playground and graph shared controls

- Scopes: `apps/web/src/integrations/ApiPlaygroundDrawer.tsx`, `apps/web/src/knowledge/KnowledgeGraphPage.tsx`, `packages/ui/src/input.tsx`, `packages/ui/src/textarea.tsx`.
- Change: API Playground agent/external-user/question fields and Knowledge Graph search now use shared inputs; `Input`/`Textarea` expose forwarded refs for focus and combobox behavior.
- Validation: shared regression 469/469, Web typecheck, and Web regression 895/895 pass after commits `88b45e9c` and `dbe3f184`.
- Boundary: browser visual comparison, focus-return and protected backend success paths remain open.
