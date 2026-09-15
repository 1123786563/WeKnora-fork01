# R195 chat empty-state centering

- Conditions: Chrome, same authenticated seeded account, zh-CN, 1355x720 viewport, React `:5181/platform/creatChat` and Vue `:5173/platform/creatChat`.
- Finding: React's empty chat cluster was rendered near the top because the chat main section did not establish a full-height flex column; Vue centers the welcome title and composer in the available viewport.
- Change: the shared `ChatPage` main section now has an explicit flex/min-height layout, the page uses the full viewport height, and the empty starter wrapper restores the Vue vertical rhythm.
- Browser evidence: React title is approximately y=260 and composer y=346 after the change, matching Vue title y=263 and composer y=343 at the same viewport. AX exposes the draft field, agent picker, attachment, knowledge-base and send controls.
- Validation: chat/view focused tests 157/157, shared and Web typechecks, and `git diff --check` pass.
- Boundary: successful stream, attachment upload, backend mention data, responsive widths and native/Wails rendering remain open.
