# R188 tenant members empty invitation state

- Conditions: Chrome, same authenticated seeded account, zh-CN, 1355x720 viewport, React `:5181` and Vue `:5173` member settings.
- Finding: Vue renders the zero-pending-invitations message in a rounded dashed-border surface with canvas background; React previously rendered plain text.
- Change: React `TenantMembersPanel` now applies the matching dashed border, radius, padding, muted text and surface background while keeping the existing localized copy and manager gate.
- Browser evidence: refreshed React settings page visibly shows the bordered empty invitation surface above the member table, matching the Vue hierarchy and spacing at the captured viewport.
- Validation: TenantMembers focused tests 9/9, Web typecheck and `git diff --check` pass.
- Boundary: non-empty invitation rows, protected invite/revoke mutations, responsive widths, other locales and native/Wails rendering remain open.
