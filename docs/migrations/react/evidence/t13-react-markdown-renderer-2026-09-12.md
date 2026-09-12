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
- `packages/views/src/chat/mermaid.ts` lazily hydrates only marked Mermaid
  blocks, initializes Mermaid with `securityLevel: strict` and
  `startOnLoad: false`, sanitizes returned SVG through the injected DOMPurify
  boundary, and preserves the escaped source block if loading or rendering
  fails. Mermaid and DOMPurify are dynamically imported by the Web message
  list so non-DOM clients keep the source fallback and do not pay the initial
  renderer cost.
- Raw HTML is escaped at the renderer boundary. Link/image destinations are
  allow-listed; unsafe `javascript:`, `vbscript:`, and `data:` destinations are
  omitted.
- `packages/views/src/chat/message-list.tsx` now renders message content via
  the shared renderer, exposes a delegated `onCitationClick` seam, and shows
  metadata-only artifact rows with an `onArtifactDownload` callback.
- `packages/api-client/src/chat/artifacts.ts` adds strict list parsing and a
  binary download seam. The parser drops `source_path`; bytes are fetched only
  through the authenticated message artifact route.
- `packages/views/src/chat/tool-result.tsx` maps every normalized tool-result
  renderer to a readable React view, preserves unknown results as plain text,
  and recursively redacts secret-shaped fields before serialization. Live tool
  calls in the chat page use this view rather than interpolating raw results.
- `packages/views/src/chat/reference-list.tsx` reuses the shared reference
  grouping model for stream/history references, keeps safe web links as links,
  and exposes local document/tool chunk ids as activation buttons. The ChatPage
  combines persisted message reference metadata with current stream references.
- `apps/web/src/chat/citation.ts` restricts host navigation to absolute HTTP(S)
  citation ids; the Web chat route opens those targets in a new tab while local
  chunk ids remain in the in-page activation path.
- `apps/web/src/chat/ChatRoutePage.tsx` now resolves message metadata through
  that API and hands the authenticated bytes to the Web save-file port.
- `apps/web/src/styles.css` adds safe readable layout for Markdown code, tables,
  math, and citation controls.

## Fresh verification

Commands run from the isolated `codex/react-multiclient` worktree:

| Command | Result |
|---|---|
| `pnpm test:shared` | exit 0; 221/221 |
| `pnpm typecheck:shared` | exit 0 |
| `pnpm typecheck:web` | exit 0 |
| `pnpm test:web` | exit 0; 107/107 |
| `pnpm test:web` after artifact-save wiring | exit 0; 109/109 |
| `pnpm test:web` after citation host wiring | exit 0; 110/110 |
| `pnpm test:desktop` | exit 0; 2/2 |
| `pnpm test:embed` | exit 0; 3/3 |
| `pnpm test:mobile` | exit 0; 77/77 |
| `pnpm typecheck:shared`, `pnpm typecheck:web`, `pnpm typecheck:desktop`, `pnpm typecheck:embed`, `pnpm typecheck:mobile` | exit 0 |
| `pnpm build:web`, `pnpm build:desktop-renderer`, `pnpm build:embed` | exit 0; Web 2,211 modules, Desktop 2,212 modules, Embed 2,144 modules |
| `pnpm --filter @weknora/mobile exec expo export --platform ios` | exit 0 |
| `pnpm --filter @weknora/mobile exec expo export --platform android` | exit 0 |

Focused TDD coverage in `packages/views/src/chat/markdown.test.tsx`,
`packages/views/src/chat/mermaid.test.tsx`, and
`packages/views/src/chat/tool-result.test.tsx` verifies
headings/tables/CJK, Mermaid metadata, readable math, XSS-safe raw HTML and
unsafe URLs, citation button attributes, unclosed fences, and MessageList's
shared renderer path, strict Mermaid sanitizer/fallback behavior,
known/unknown tool-result mapping, recursive secret redaction, and grouped
reference modeling with unsafe web destinations omitted.

## Evidence boundary

This is source/Node/build evidence only. It does not prove browser XSS
execution behavior, Mermaid rendering in a real browser/WebView, protected artifact download bytes
or preview against a live backend, citation navigation against a live backend,
or long-message performance. T13 therefore remains `review`.
