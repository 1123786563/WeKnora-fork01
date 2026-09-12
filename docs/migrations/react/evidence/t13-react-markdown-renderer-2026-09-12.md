# T13 React Markdown renderer follow-up — 2026-09-12

## Scope

This increment closes the React Web message-list plain-text gap with a small
shared `@weknora/views` Markdown renderer. It is intentionally bounded: the
renderer owns Markdown structure and safe HTML generation; citation buttons
emit an opaque citation id for the host view to resolve; it does not invent a
public storage URL or a client-side artifact download path.

## Implementation

- `packages/views/src/chat/markdown.ts` uses `marked` with GFM tables, fenced
  code, CJK text, unclosed-fence handling, controlled Mermaid code metadata,
  KaTeX rendering with `trust: false`, and citation protocol conversion.
- Raw HTML is escaped at the renderer boundary. Link/image destinations are
  allow-listed; unsafe `javascript:`, `vbscript:`, and `data:` destinations are
  omitted.
- `packages/views/src/chat/message-list.tsx` now renders message content via
  the shared renderer, exposes a delegated `onCitationClick` seam, and shows
  metadata-only artifact rows with an `onArtifactDownload` callback.
- `packages/api-client/src/chat/artifacts.ts` adds strict list parsing and a
  binary download seam. The parser drops `source_path`; bytes are fetched only
  through the authenticated message artifact route.
- `apps/web/src/styles.css` adds safe readable layout for Markdown code, tables,
  math, and citation controls.

## Fresh verification

Commands run from the isolated `codex/react-multiclient` worktree:

| Command | Result |
|---|---|
| `pnpm test:shared` | exit 0; 212/212 |
| `pnpm typecheck:shared` | exit 0 |
| `pnpm typecheck:web` | exit 0 |
| `pnpm test:web` | exit 0; 107/107 |
| `pnpm build:web` | exit 0; 137 modules |

Focused TDD coverage in `packages/views/src/chat/markdown.test.tsx` verifies
headings/tables/CJK, Mermaid metadata, readable math, XSS-safe raw HTML and
unsafe URLs, citation button attributes, unclosed fences, and MessageList's
shared renderer path.

## Evidence boundary

This is source/Node/build evidence only. It does not prove browser XSS
execution behavior, Mermaid SVG hydration, protected artifact download bytes
or preview against a live backend, citation navigation against a live backend,
or long-message performance. T13 therefore remains `review`.
