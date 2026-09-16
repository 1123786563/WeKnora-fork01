# Platform shell deep-import evidence

- Scope: `apps/web/src/platform/PlatformShell.tsx`
- Change: replaced the `@weknora/views` barrel import for session sidebar and guide primitives with direct module imports, reducing accidental barrel coupling while preserving exported behavior.
- Validation: `pnpm run typecheck:web`, `pnpm run test:web` (891/891), `pnpm run build:web`, `git diff --check`.
- Build result: route and feature chunks remain emitted; initial index chunk is about 3.05MB after prior route splitting. No runtime behavior change is claimed beyond successful compilation/build.
