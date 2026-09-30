# T10 narrow sidebar resize repair report

- **Task:** #150 / T10 responsive shell resize repair
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-narrow-shell/WeKnora-fork01`
- **Base:** `2674de91bf231579407afc95297523401357cacf`
- **Implementation commit:** `2d391912b9015866654ec25e727848061b9b4923`
- **Status:** implementation and checks complete; live browser acceptance and independent review/validation remain with controller

## Changes

- `PlatformShell.tsx`: subscribe to viewport resize, keep the persisted desktop collapse preference separate from narrow-only expanded state, automatically collapse on entry to <=640px without writing storage, restore the desktop preference when leaving narrow mode, and preserve explicit desktop persistence. Guide expansion and drag expansion use the appropriate mode.
- `platform-shell.td.css`: expanded narrow navigation is fixed over the route so the content retains its width and the existing accessible collapse button stays reachable.
- `platform-shell-sidebar-state.test.tsx`: add desktop→390→desktop preference tests, stored-collapsed coverage, narrow explicit overlay expand/close/re-enter behavior, and CSS overlay assertion.

## RED evidence

After adding the resize and narrow overlay assertions, the focused run failed as expected: resizing 1024→390 did not collapse navigation, and narrow expansion did not receive overlay layout. This reproduced the initializer-only state gap and content-squeezing behavior before implementation.

## Verification

Focused sidebar tests:

```text
pnpm --filter @weknora/web exec node --import tsx --test src/platform/platform-shell-sidebar-state.test.tsx
PASS: 8 tests, 0 failures
```

Web typecheck:

```text
pnpm typecheck:web
PASS (exit 0)
```

Web production build:

```text
pnpm build:web
PASS (exit 0; Vite built successfully)
```

Build emitted existing CSS `@import` ordering, `calc()` spacing, and large chunk warnings.

Full Web suite:

```text
pnpm --filter @weknora/web test
PASS: 2,356 tests, 0 failures (duration 138.9s)
```

Whitespace/diff check:

```text
git diff --check
PASS (exit 0)
```

## Browser limitation

JSDOM tests verify responsive state transitions, labels, preference persistence, and the overlay CSS source. They do not calculate rendered widths or confirm visual stacking in an actual browser. Controller should repeat the 390px live DOM width/screenshot check and verify desktop restore after resizing.
