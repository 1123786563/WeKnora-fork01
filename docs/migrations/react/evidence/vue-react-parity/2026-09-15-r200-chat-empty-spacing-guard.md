# R200 Chat empty-state spacing guard (2026-09-15)

The new-conversation starter section keeps Vue-matched `padding-bottom: 56px`. A transient uncommitted change reduced it to 48px; paired viewport evidence from R195 showed 56px places the welcome title and composer at the Vue coordinates, so the regression was reverted.

Verification after restoring the value:

- Shared chat/view tests: 63/63
- Web chat page tests: 27/27
- `git diff --check`: pass

The change is layout-only; streaming and backend behavior are unchanged.
