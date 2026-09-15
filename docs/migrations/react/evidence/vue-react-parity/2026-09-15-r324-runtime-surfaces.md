# R324 runtime-surface parity batch

## Changes

- Knowledge document detail now uses the Vue breadcrumb/title-row anatomy and removes the React-only eyebrow.
- Chat attachment rejection messages and session-source labels now resolve through the active locale, with the Vue literal fallback for the Embed source.
- Platform tenant switching closes the account and tenant menus for both changed-tenant and current-tenant no-op paths.
- Ollama model-management sections are shown only when the service is available and retain the Vue model-library link.
- Native knowledge documents use recursive folder scope only for search/tag-filtered requests, matching the Vue/Web contract.

## Verification

- Web focused batch: 12/12 passed.
- Mobile full test suite: 191/191 passed; mobile typecheck passed.
- Platform focused tests: 141/141 plus tenant-switch 6/6 passed.
- Documents/knowledge focused tests: 142/142 passed.
- `pnpm typecheck:web` and `git diff --check`: passed.
- Live unauthenticated Vue/React Login screenshots and computed geometry were captured at 1440x900; see R323 evidence.

## Evidence boundary

These changes have source comparison and unit/DOM evidence; most protected runtime flows still lack authenticated paired browser capture, real backend mutation, Wails, and release-platform acceptance.
