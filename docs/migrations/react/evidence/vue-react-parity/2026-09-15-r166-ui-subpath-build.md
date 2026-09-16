# R166 shared UI subpath build resolution

- Scope: `apps/web/tsconfig.json`, `apps/web/vite.config.ts`.
- Change: added TypeScript path and Vite aliases for shared UI subpath imports (`button`, `checkbox`, `input`, `textarea`), fixing production resolution from the `@weknora/ui` index alias.
- Validation: Web typecheck and production build pass; Vite transformed 2,447 modules and emitted the production bundle. Existing large-chunk advisory remains informational.
