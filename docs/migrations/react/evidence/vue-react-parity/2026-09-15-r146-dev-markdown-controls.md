# R146 development Markdown page shared controls

- Scope: `apps/web/src/DevMarkdownPage.tsx`, `packages/ui/package.json`.
- Change: Markdown fixture reset and editor now use shared `Button`/`Textarea`; UI package exposes direct subpath exports for these primitives.
- Validation: `pnpm typecheck:web` passed; Web regression passed 895/895.
- Boundary: development-route browser visual comparison remains open.
