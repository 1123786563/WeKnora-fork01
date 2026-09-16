# Independent parity review — react-multiclient

Date: 2026-09-15  
Target: `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient`

## Review boundary

At review start the target had a large dirty diff (`git status --short`: 49 tracked modifications plus untracked parity files) on `c6f94b51`. While checks were running, an external process committed that material as `b61ac395` and subsequently started another dirty change set. Therefore the reproducible implementation range reviewed here is `c6f94b51..b61ac395`; the final snapshot was not stable and must not be treated as a clean review baseline. No product code was changed by this review.

Existing `artifacts/parity-agent-inventory.md`, `docs/migrations/react/vue-react-parity-matrix.md`, and `docs/migrations/react/vue-react-parity-progress.md` were consulted. They correctly keep Vue authoritative and distinguish static/unit/browser/backend/native evidence; the matrix remains predominantly `review`, not `accepted`.

## Findings

### P1 — Shared Dialog migration breaks the existing modal contract and the web suite

`c6f94b51..b61ac395` replaces the project-owned focus/escape/outside-close implementation in `packages/ui/src/dialog.tsx:20-30` with `@radix-ui/react-dialog` Root + Portal. This is not a styling-only substrate change: the existing test host and several consumers depend on the wrapper rendering synchronously into the expected DOM tree. The same commit also portals `Sheet` in `packages/ui/src/sheet.tsx:87-117`.

Reproduction from the target:

```text
pnpm test:web
ℹ tests 999
ℹ pass 988
ℹ fail 11
```

The failures include `HTMLInputElement is not defined` from Radix FocusScope in onboarding/member modal tests, server-rendered TagPicker dialogs becoming empty (`tags-ui.test.tsx`), and four Skill settings drawer tests unable to find the transcript/timeline/files content. This directly regresses modal parity and violates the wrapper comment's stated “test-host contract”. Restore the project-owned contract or add an explicitly verified adapter/test-host strategy before accepting the Radix migration.

### P1 — Current web integration is not typecheckable

During the review snapshot, `pnpm typecheck:web` failed with:

```text
src/auth/api.ts(2,10): error TS2305: Module "@weknora/api-client" has no exported member 'parseLogin'.
src/auth/api.ts(2,27): error TS2305: Module "@weknora/api-client" has no exported member 'ParsedLogin'.
```

At that moment `apps/web/src/auth/api.ts` imported those names, while `packages/api-client/src/index.ts` exported only the endpoint types and not `parseLogin`/`ParsedLogin`. The file was then removed or changed by the concurrent process, so this finding is recorded as a transient but directly observed integration failure, not as a claim about the final unstable snapshot. Re-run typecheck on a frozen SHA and ensure the auth seam and package barrel agree.

### P2 — Added viewer download test is self-contradictory and gives false permission confidence

The added fixture in `apps/web/src/documents/KnowledgeDocumentDetailPage.test.tsx:51-66` makes the test user `user-1` and the KB owner `user-1`, then calls the user `viewer` at line 145 and expects no download at line 147. `computeKBPermissions` intentionally treats the KB creator as writable, so this test fails because it is testing creator precedence, not a viewer-only/shared-read case. The failure was reproduced in `test:web` (`viewer detail keeps Vue download affordances hidden...`). Use a different KB owner or an explicit read-only share permission, then assert the Vue rule from `frontend/src/views/knowledge/KnowledgeBase.vue:328-337` (viewer cannot download). Do not “fix” the production gate to satisfy this fixture.

### P2 — Activity panel has a stale-response race across filter/KB changes

The new `apps/web/src/knowledge-bases/KnowledgeBaseActivityPanel.tsx:47-65` starts `load(true)` on every KB/filter change but has no generation guard or abort. A slow response for the previous filter can overwrite `entries`, `nextCursor`, `loaded`, or `error` after the new request has rendered. This is especially visible with the panel's cursor pagination: an old cursor can make “load more” request the wrong result set. Vue parity evidence elsewhere in the repository explicitly treats superseded responses as a required state boundary. Add a request generation/abort guard and a focused stale-filter test before accepting the activity surface.

### P2 — Dialog/Sheet default semantics are not proven against Vue visual and interaction parity

The current change preserves Vue class names but delegates focus scope, body interaction blocking, portal placement, and dismissal semantics to Radix at `packages/ui/src/dialog.tsx:22-29` and `packages/ui/src/sheet.tsx:117`. Existing artifacts say screenshot/build/unit evidence is insufficient for parity, and the new failures show the actual DOM contract already differs. No paired authenticated Vue/React browser evidence was available in this review; keep the affected surfaces `review`, not `accepted`, until Esc, overlay click, focus restoration, keyboard cycling, nested dialogs, fixed viewport layout, and all five locales are verified on the same runtime.

## Verification ledger

| Check | Result | Evidence boundary |
|---|---:|---|
| `pnpm test:shared` | 482 passed, 0 failed | static/unit only |
| `pnpm test:web` | 988 passed, 11 failed of 999 | reproducible web DOM/unit regression |
| `pnpm typecheck:web` | failed on `parseLogin` / `ParsedLogin` during the observed snapshot | integration/type boundary |
| `git diff --check` | passed at the first dirty snapshot | whitespace only |
| Vue/React authenticated paired browser run | not run; dev servers were not available in existing inventory | no browser acceptance claim |
| backend success-path, Wails, iOS, Android | not established by this review | no runtime/platform acceptance claim |

## Disposition

Do not mark the affected parity rows accepted. First freeze the target SHA, repair the P1 type/build and shared modal contract, correct the viewer fixture, add the activity stale-response regression, then rerun the full web suite and same-condition Vue/React browser evidence. Vue remains the authority and must stay in place until the matrix state and evidence gates are closed.
