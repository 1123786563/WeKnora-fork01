# React/Vue parity evidence — runtime fallback locales

- Scope: `apps/web/src/settings/RuntimeQueuesPanel.tsx`
- Change: added locale-aware fallback dictionaries for runtime queue, worker pool, model limiter and queue status labels in en-US, ja-JP, ko-KR and ru-RU; zh-CN remains the baseline.
- Validation: `pnpm run typecheck:web`, `pnpm run test:web` (891/891), `git diff --check`.
- Limitation: runtime queue availability and authenticated interaction evidence remain environment dependent.
