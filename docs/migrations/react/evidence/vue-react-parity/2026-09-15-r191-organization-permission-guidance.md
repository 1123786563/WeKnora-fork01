# R191 organization permission guidance

- Scope: create-mode organization editor permission section.
- Finding: Vue lists role-specific permission bullets and an owner note; React previously showed only one summary line per role.
- Change: React now renders localized admin/editor/viewer permission lists, allowed/denied markers, access-level badges and the owner note using the existing i18n contract.
- Browser evidence: authenticated Chrome create modal reaches `权限说明`; AX exposes all three role headings, 15 permission entries and the owner note, and the screenshot matches the Vue information hierarchy.
- Validation: organization focused tests 14/14, Web typecheck and `git diff --check` pass.
- Boundary: permission copy is informational; server-side role enforcement and protected mutation scenarios remain covered by existing gates and still need live fixtures.
