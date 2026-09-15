# R170 resource tree checkbox

- Scope: `apps/web/src/data-sources/DataSourcesPage.tsx`.
- Change: resource-tree selection now uses shared `Checkbox` while preserving the visually hidden input, mixed descendant state, data-state marker and custom focus indicator.
- Validation: data-source focused suite passed 4/4; Web typecheck and diff check passed.
- Boundary: protected resource selection persistence and same-session Vue visual comparison remain open.
