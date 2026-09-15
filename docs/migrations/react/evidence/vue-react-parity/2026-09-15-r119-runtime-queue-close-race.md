# Runtime queue task drawer close race guard

Date: 2026-09-15  
Scope: `apps/web/src/settings/RuntimeQueuesPanel.tsx`

Opening a task drawer now records a request generation. Closing the drawer invalidates that generation, and late success or error responses are ignored when they no longer belong to the active drawer. This prevents a slow queue request from reopening a drawer that the user already closed or replacing a newer queue selection.

Validation: `pnpm typecheck:web` passed; `pnpm test:web` passed 895/895; `git diff --check` passed.
