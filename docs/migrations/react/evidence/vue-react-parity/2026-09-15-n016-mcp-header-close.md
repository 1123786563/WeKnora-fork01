# N016 MCP drawer header close affordance

Vue `SettingDrawer` already supplies the drawer close affordance and keeps the footer cancel action. React previously added a second textual “关闭” button inside the custom header, which duplicated the action and tightened the title/step layout.

React now removes the redundant header button while preserving the footer cancel action and overlay/drawer close behavior.

Verification: MCP settings tests 14/14, Web typecheck, and `git diff --check` passed. Authenticated browser and native drawer evidence remain open.
