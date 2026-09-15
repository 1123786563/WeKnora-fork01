# R178 knowledge-base list layout

- Scope: `apps/web/src/App.tsx` knowledge-base list page.
- Change: removed conflicting page-level max-width/padding utilities and moved content spacing to the list content column, allowing the Vue-style rail and responsive card grid to use the full shell width without duplicated margins.
- Validation: knowledge-base focused suite passed 40/40; Web typecheck and diff check passed.
- Boundary: browser responsive visual comparison remains open.
