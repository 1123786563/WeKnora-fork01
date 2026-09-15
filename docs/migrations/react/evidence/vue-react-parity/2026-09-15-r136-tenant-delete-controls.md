# R136 tenant delete danger-zone controls

- Scope: `apps/web/src/settings/TenantDeleteZone.tsx`
- Change: destructive confirmation input and delete action now use shared `Input` and `Button`, preserving typed-name gating, busy state, localized labels, test id and delete callback.
- Validation: existing Web typecheck and 895/895 regression remain green after the control migration.
- Boundary: owner-authorized live deletion and same-session Vue visual comparison remain open.
