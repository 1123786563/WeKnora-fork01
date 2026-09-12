# Skill settings parity evidence

Date: 2026-09-12

## Scope

The React Settings `skills` entry now uses the existing typed skill catalog operations for administrators and a read-only installed-skill inventory for viewers. The operation surface reuses the existing client routes for catalog registration, asynchronous installation and polling, safe file listing/content reads, and stopping in-progress installs.

This remains `implementing`. The Vue settings editor/catalog layout, all six locales, exact visual states, complete sandbox/template coupling, authenticated browser evidence, real-backend evidence, Wails, iOS, and Android evidence remain open.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| Component SSR | `pnpm exec tsx --test apps/web/src/settings/SkillSettingsPanel.test.tsx` — 2 passed, 0 failed | focused component evidence |
| Full Web | `pnpm test:web` — 271 passed, 0 failed | Web regression evidence |
| Typecheck | Web TypeScript check passed | static integration evidence |
| Browser / real backend / native | not completed; protected route needs authenticated SSO and device/provider environments are unavailable in this slice | missing evidence |

## Remaining work

Do not move R033 to `accepted` until the Vue child surfaces and protected runtime/visual/native evidence are closed.
