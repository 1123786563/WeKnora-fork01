# T10 narrow shell implementation report

- **Task:** #150 / T10 narrow viewport shell repair
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-narrow-shell/WeKnora-fork01`
- **Base:** `5aac383ea8b355c3ccdbb5dfe281579a8cfb4325`
- **Implementation commit:** `a44679859dcb8ad0a95768b790881968f08e6696`
- **Status:** implementation and checks complete; controller browser acceptance and independent review/validation remain external to this task

## Changes

- `apps/web/src/platform/PlatformShell.tsx`: initialize navigation collapsed at viewport widths <=640px. Initialization does not write or replace the persisted desktop preference; the existing labeled toggle remains available and persists explicit user actions.
- `apps/web/src/platform/platform-u.css`: remove the `.wk-shell-1` 600px minimum width at <=640px.
- `apps/web/src/platform/platform-shell-sidebar-state.test.tsx`: cover narrow initialization with a saved expanded preference, accessible expand toggle, and narrow CSS override.

## RED evidence

After installing workspace dependencies with `pnpm install --frozen-lockfile`, the focused regression tests failed as expected: narrow initialization did not expose the expand button, and the stylesheet did not contain a <=640px min-width override. The earlier pre-install test attempt could not load `tsx` because the worktree had no `node_modules`.

## Verification

Focused shell test:

```text
pnpm --filter @weknora/web exec node --import tsx --test src/platform/platform-shell-sidebar-state.test.tsx
PASS: 5 tests, 0 failures
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

The build emitted existing CSS `@import` ordering, invalid `calc()` spacing, and large chunk warnings.

Full Web test suite:

```text
pnpm --filter @weknora/web test
PASS: 2,353 tests, 0 failures (duration 135.2s)
```

Whitespace/diff check:

```text
git diff --check
PASS (exit 0)
```

## Browser and remaining evidence

The assigned live defect evidence measured `innerWidth=390`, `document.scrollWidth=600`, with the outlet starting at x=60 and width 540 due to the shell's 600px minimum width. This implementation was not re-measured in a live browser in this worker. Controller should verify the integrated 390px viewport width, visible verdict/action/evidence, and desktop navigation behavior with a screenshot and DOM measurement. JSDOM coverage verifies state and CSS source but cannot establish rendered layout dimensions.
