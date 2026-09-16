# Configuration list filter race guard

Date: 2026-09-15  
Scope: `apps/web/src/configuration/ConfigurationPage.tsx`

Agent-source changes now invalidate earlier configuration loads. When the user switches between All, Mine, and Shared, a late `Promise.allSettled` result from the previous filter is ignored instead of replacing the current records, errors, or availability state.

Validation: Web typecheck passed; configuration helper/editor test suite passed 40/40; `git diff --check` passed.
