# R153 administration controls

- Scope: `apps/web/src/administration/AdministrationPage.tsx`.
- Change: member role and invitation email/role fields now use shared `Select`/`Input` controls, preserving native select semantics, owner gating, controlled state and invitation payloads.
- Validation: Web typecheck passed; full Web regression passed 895/895; diff check passed.
- Boundary: protected administration mutation and same-session Vue visual comparison remain open.
