# React/Vue parity evidence — runtime queue task copy

- Scope: `apps/web/src/settings/RuntimeQueuesPanel.tsx`
- Change: localized task drawer labels and runtime task unavailable/loading/empty/failure states for five locales.
- Behavior preserved: queue task listing, count buttons, runtime availability gating and model limiter rendering.
- Validation: `pnpm run typecheck:web`, `pnpm run test:web` (891/891), `git diff --check`.
- Limitation: authenticated runtime queue task drawer evidence remains pending because deployment availability varies by environment.
