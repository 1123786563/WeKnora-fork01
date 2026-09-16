# R047 Chat tag mention discovery

- Scope: the authenticated React chat mention loader now queries tags for each accessible knowledge base through `client.knowledge.documents.tags(..., { page_size: 200 })` and emits `tag` items with `kbId`/`kbName` context.
- Failure behavior: tag calls use `Promise.allSettled`; an unavailable or malformed KB tag endpoint does not discard KB, file, MCP, or skill options. Scope-generation checks run after the asynchronous tag fan-out to prevent stale results after tenant/session changes.
- Verification: Web tests 892/892, `pnpm run typecheck:web`, and `pnpm run build:web` passed.
- Limitation: production browser evidence still needs a tenant containing tags; no tag mutation is performed by this loader.
