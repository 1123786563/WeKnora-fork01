# R187 organization editor modal

- Conditions: Chrome, same authenticated seeded account, zh-CN, 1355x720 viewport, React `:5181` and Vue `:5173`.
- React create modal now matches the Vue editor anatomy: wide 90vw/85vh shell, left navigation with `基本信息` and `权限说明`, avatar preview beside the name field, two-column label/control rows, localized placeholders, and a stable footer cancel action.
- Runtime AX confirms the dialog, close action, two navigation buttons, name field, description field and create action are present and labelled. The permission section is reachable without changing route state.
- Vue comparison confirms the same modal shell and field hierarchy; Vue additionally renders its avatar helper/count affordances through TDesign, while React retains the same visible avatar and controlled text behavior.
- Validation: organization page focused tests 14/14, Web typecheck, full Web regression 895/895 and `git diff --check` pass.
- Boundary: no organization fixture was available for edit/member mutation verification; responsive widths, other locales and native/Wails rendering remain open.
