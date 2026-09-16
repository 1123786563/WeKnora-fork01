# React lazy-loading boundary copy evidence

- Scope: `apps/web/src/main.tsx`
- Change: the protected-route Suspense fallback now resolves `common.loading` through the active browser locale instead of rendering hardcoded English `Loading…`.
- Validation: `pnpm run build:web`, `pnpm run test:web` (891/891), `pnpm run typecheck:web`, `git diff --check`.
