# R206 Chat batch action menu parity (2026-09-15)

React batch management now follows Vue's session sidebar: the entry is exposed from each session's `更多对话操作` menu, and the bottom toolbar is rendered only while batch mode is active. Empty lists therefore expose only the empty state and user controls.

Verification:

- Shell/session tests: 18/18
- Web typecheck: pass
- `git diff --check`: pass

Protected session deletion and full browser mutation evidence remain open.
