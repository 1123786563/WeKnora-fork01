# React/Vue parity evidence — views subpath runtime migration

Date: 2026-09-15

Remaining runtime consumers in IntegrationsRoutePage, AgentsPage, EmbedApp, and settings surface helpers now use explicit `@weknora/views` subpaths. Package exports, Web TypeScript paths, and Vite aliases are synchronized; the ChatRoutePage esbuild fixture accepts both root and subpath imports.

- Web tests: 895/895 passed.
- Web typecheck: passed.
- Web production build: passed.
- Embed tests: 7/7 passed.
- Embed typecheck: passed.
- `git diff --check`: passed.

The eager index bundle remains approximately 2.74 MB; further reduction requires migrating additional aggregate consumers or changing route chunk boundaries.
