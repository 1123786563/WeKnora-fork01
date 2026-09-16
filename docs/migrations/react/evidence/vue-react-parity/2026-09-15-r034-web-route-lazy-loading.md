# React Web route lazy-loading evidence

- Scope: `apps/web/src/main.tsx`
- Change: protected route pages now use `React.lazy` and a shared `Suspense` loading boundary. Authentication pages remain eagerly loaded; route behavior, guards and page props are unchanged.
- Build evidence: `pnpm run build:web` succeeds. Initial `index` JavaScript decreased from about 4.92 MB minified to about 3.05 MB; chat, knowledge and settings pages emit separate route chunks.
- Regression evidence: `pnpm run test:web` 891/891; `pnpm run typecheck:web`; `git diff --check`.
- Limitation: bundle warning remains for the settings route chunk and visualization chunks; further splitting should be measured separately.
