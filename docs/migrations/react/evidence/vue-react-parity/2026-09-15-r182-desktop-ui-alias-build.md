# R182 Desktop UI alias build

- Scope: `apps/desktop/vite.config.ts`.
- Change: added shared UI subpath aliases for the Desktop renderer, matching Web resolution and allowing shared button/checkbox/input/textarea consumers to build from source.
- Validation: Desktop production build passed after transforming 2,449 modules; Desktop typecheck passed. Existing large-chunk advisory remains informational.
