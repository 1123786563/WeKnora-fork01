# R159 organization controls

- Scope: `apps/web/src/organizations/OrganizationsPage.tsx`.
- Change: organization create/edit, upgrade request, membership role, invite-code and organization search fields now use shared `Input`/`Select`/`Textarea` controls, preserving localized labels, keyboard submission, role gating and controlled state.
- Validation: organization focused suite passed 26/26; Web typecheck and diff check passed.
- Boundary: protected organization mutations and same-session Vue visual comparison remain open.
