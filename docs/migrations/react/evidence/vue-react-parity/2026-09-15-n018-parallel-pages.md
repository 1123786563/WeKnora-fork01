# N018 parallel page parity batch

## Scope

The parallel batch compared the React implementations against the existing Vue page anatomy and behavior for Login, Knowledge Base list, Agents, Organizations, Wiki, FAQ, and General settings. Integrations/Embed was inspected but required no code change.

## Changes and focused evidence

- Login: carousel fade stacking and keyboard-operable language menu; `apps/web/src/auth/login-page.test.tsx`, 6/6.
- Knowledge Base list: query deep-link filtering and filled pin icon; `apps/web/src/knowledge-bases/kb-list-anatomy.test.tsx`, 42/42.
- Agents: Vue close accessible name and collapse chevron; `apps/web/src/agents/AgentsPage.test.tsx` and `agent-editor.test.tsx`, included in final suite.
- Organizations: Vue create dialog dimensions and footer action placement; `apps/web/src/organizations/OrganizationsPage.test.tsx`, 27/27.
- Wiki: submit-only search and localized no-results state; `apps/web/src/wiki/editor.test.ts`, Wiki tests 10/10.
- FAQ: tag search and responsive masonry/reflow; `apps/web/src/faq/FAQPage.test.tsx`, 81/81.
- General settings: persisted font preferences on mount and semantic preview tokens; `apps/web/src/settings/GeneralPreferencesPanel.test.tsx`, 4/4.

## Verification

- Full Web test suite: 925/925 passed.
- `pnpm typecheck:web`: passed.
- `git diff --check`: passed.

## Evidence boundary

These are Vue-source/static comparisons and React unit/DOM tests. Authenticated browser screenshots/computed styles, real backend flows, Wails/native, and mobile acceptance remain open.
