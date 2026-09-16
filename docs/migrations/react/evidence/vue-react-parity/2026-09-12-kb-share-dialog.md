# Knowledge-base share dialog parity evidence

Date: 2026-09-12

## Scope

The Web knowledge-base list now exposes a localized share action for manageable cards and a dialog that loads eligible organizations and current shares, supports viewer/editor permission selection, confirms unshare, and refreshes the list after a successful mutation. It reuses the existing organization share client and tenant-scoped transport.

This remains `implementing`. The Vue organization avatar/metadata presentation, exact six-locale copy, upload-mask/progress integration, screenshot comparison, authenticated browser, real-backend, Wails, iOS, and Android evidence remain open.

## Evidence

| Layer | Command / result | Classification |
|---|---|---|
| Component SSR | `pnpm exec tsx --test apps/web/src/knowledge-bases/KnowledgeBaseShareDialog.test.tsx` — 1 passed, 0 failed | focused component evidence |
| Full Web | `pnpm test:web` — 274 passed, 0 failed | Web regression evidence |
| Typecheck | Web TypeScript check passed | static integration evidence |
| Browser / real backend / native | not completed; protected route requires authenticated SSO and provider/device environments | missing evidence |

## Remaining work

Do not move N005 to `accepted` until upload-progress/highlight and protected visual/runtime/native evidence are closed.
