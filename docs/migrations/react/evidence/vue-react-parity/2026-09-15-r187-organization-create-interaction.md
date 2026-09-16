# R187 organization create interaction

- Scope: React organization create modal navigation and required form behavior.
- Browser evidence (Chrome, 1355x720, zh-CN): the `权限说明` navigation changes the active section and renders role guidance; returning to `基本信息` and submitting an empty required name keeps the modal open and focuses `organization-name`.
- Validation: organization suite `26/26`, Web typecheck, and `git diff --check` pass.
- Remaining difference: Vue exposes an avatar picker/visible avatar hint and complete role-permission copy in this flow; React currently renders a generated avatar preview without picker state. Protected create success/failure, join-mode modal, and responsive/desktop acceptance remain open.
