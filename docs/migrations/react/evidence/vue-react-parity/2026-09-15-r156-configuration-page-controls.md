# R156 configuration page controls

- Scope: `apps/web/src/configuration/ConfigurationPage.tsx`.
- Change: agent search and agent-source filtering now use shared `Input`/`Select` controls, preserving grouping, creator filtering and controlled state.
- Validation: configuration focused suite passed 40/40; Web typecheck and diff check passed.
- Boundary: protected configuration CRUD and same-session Vue visual comparison remain open.
