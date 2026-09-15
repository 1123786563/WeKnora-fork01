# Parity repair — independent review P1/P2

Date: 2026-09-15
Target: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`

## Repairs

- Restored the project-owned `Dialog` contract: synchronous test-host DOM, Vue-compatible backdrop/outside close, close button, Escape handling, focus restoration, and bounded Tab cycling. Removed the Radix Portal layer that made SSR/tag dialogs empty and triggered the test-host `HTMLInputElement` failure.
- Restored `Sheet` synchronous wrapper output while preserving its existing right/left layout, resize persistence, focus, Escape, outside-close, and Vue class/token contract. This makes Skill settings transcript/timeline/files content observable in the existing test host.
- Corrected the viewer regression fixture so the viewer is not also the KB creator; production creator precedence remains unchanged.
- Added `isCurrentActivityGeneration` and a generation ref to `KnowledgeBaseActivityPanel`. Reset/filter/KB requests advance the generation; stale responses cannot replace rows, cursor, loaded/error state, or loading state. Added the focused stale-generation test.
- Repaired the web auth package seam by exporting `ParsedLogin`/`parseLogin` from `@weknora/api-client`.
- Restored the existing Knowledge Settings helper/type exports required by its focused test and web typecheck.

## TDD evidence

- Initial focused run reproduced the review failures: viewer permission assertion, two tag SSR assertions, four Skill drawer assertions, plus the pre-existing `parseLogin` module error and root `react-dom` resolution issue.
- The activity test was intentionally run before adding the export and failed with “does not provide an export named `isCurrentActivityGeneration`”. The minimal helper/guard was then added.
- Focused repaired run: 9 passed, 0 failed across activity, viewer detail, tags, and Skill settings.

## Final verification

| Check | Result |
|---|---:|
| `pnpm test:web` | 1038 passed, 0 failed |
| `pnpm typecheck:web` | passed |
| `git diff --check` | passed |

## Remaining blocked environment evidence

No authenticated paired Vue/React browser run was available in this repair, so Portal placement at runtime, fixed-viewport visual parity, nested-dialog behavior, all-locale browser behavior, backend success paths, Wails, iOS, and Android remain `blocked-env`/unaccepted. The direct standalone invocation of `KnowledgeSettingsPage.test.ts` also lacks the repository CSS resolve hook and fails on Node `.css` loading; the required full web runner passed it.

Unrelated dirty and untracked worktree changes were preserved. No parity row is marked accepted solely from these unit/typecheck results.
