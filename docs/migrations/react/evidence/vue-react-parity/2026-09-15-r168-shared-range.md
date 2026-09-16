# R168 shared range control

- Scope: `packages/ui/src/range.tsx`, `apps/web/src/faq/FAQPage.tsx`, `apps/web/src/agents/AgentEditorModal.tsx`.
- Change: added shared `Range` primitive for native range semantics and migrated FAQ search threshold/count and Agent editor sliders while preserving min/max/step, keyboard behavior and controlled updates.
- Validation: FAQ and Agent focused suites passed; shared suite passed 469/469; Web typecheck and diff check passed.
- Boundary: browser visual comparison remains open.
