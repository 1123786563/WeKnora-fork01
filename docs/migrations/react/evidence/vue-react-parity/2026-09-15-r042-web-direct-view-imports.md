# R042 Web direct view imports

- Scope: `apps/web/src/App.tsx` and `apps/web/src/routes.tsx` now import only the contextual-guide and integration-route modules they use, instead of the `@weknora/views` barrel.
- Intent: reduce eager protected-entry bundle coupling while preserving behavior and package boundaries.
- Verification: `pnpm run typecheck:web` passed; Web tests passed 891/891; `pnpm run build:web` passed.
- Build signal: eager `index` chunk decreased from about 3.05 MB to 2.74 MB in this build. Route chunks remain split (Chat, Knowledge, FAQ, Integrations, settings panels).
- Limitation: the remaining large eager chunk still contains other compatibility-barrel consumers; further splitting requires scoped import migration and independent review.
