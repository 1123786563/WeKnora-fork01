# R189 organization avatar picker

- Scope: React organization create modal avatar field and create payload.
- Vue baseline: the create editor shows a generated avatar beside the name field and supports emoji selection in the organization editor flow.
- Change: React now provides a localized avatar-picker trigger, 30 emoji choices, clear action and live `SpaceAvatar` preview; `organizations.create` receives `avatar: "emoji:<value>"` only when selected.
- Browser evidence: the authenticated create modal shows the avatar/name row and retains the Vue two-column form hierarchy; the picker is keyboard-addressable through a labelled button.
- Validation: organization focused tests 14/14, Web typecheck and `git diff --check` pass.
- Boundary: backend persistence of a selected avatar, edit-mode avatar changes, responsive widths and native/Wails rendering remain open.
