# Configuration editor textarea spacing fix

Date: 2026-09-15  
Scope: `apps/web/src/configuration/ConfigurationEditor.tsx`, `apps/web/src/configuration/ConfigurationOperations.tsx`, `apps/web/src/settings/ModelDebugPanel.tsx`

The configuration editor and two sibling settings forms had an unterminated Tailwind arbitrary-value class (`py-[.55rem`), so textarea controls silently lost their intended bottom padding. All three classes are now closed as `py-[.55rem]`, restoring consistent form spacing without changing behavior or API contracts.

Validation: `pnpm typecheck:web` passed; `pnpm test:web` passed 895/895; `git diff --check` passed.
