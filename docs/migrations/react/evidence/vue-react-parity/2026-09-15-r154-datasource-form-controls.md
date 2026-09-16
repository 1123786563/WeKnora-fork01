# R154 data-source form controls

- Scope: `apps/web/src/data-sources/DataSourcesPage.tsx`.
- Change: create/edit data-source fields now use shared `Input`, `Select`, `Textarea` and `Checkbox` controls, preserving localized labels, controlled form values and payload callbacks.
- Validation: data-source focused suite passed 4/4; Web typecheck passed; diff check passed.
- Boundary: protected connector validation/sync and same-session Vue visual comparison remain open.
