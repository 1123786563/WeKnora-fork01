# React/Vue parity evidence — views subpath export boundary

Date: 2026-09-15

Added explicit `@weknora/views` subpath exports and matching Web/Vite aliases for chat, guide, integration, and settings modules. Selected Web runtime consumers now bypass the aggregate barrel, including Settings, Chat, Markdown, Integrations, and Agents routes. The affected esbuild test stub accepts both root and subpath imports, preserving the existing mock contract.

- Focused ChatRoutePage send test: passed.
- Web tests: 895/895 passed.
- Web typecheck: passed.
- Web production build: passed.
- `git diff --check`: passed.

The eager index bundle remains approximately 2.74 MB; this slice establishes the import boundary for subsequent import-by-import chunk reduction.
