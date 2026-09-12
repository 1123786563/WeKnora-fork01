# T11 React Web chat main interface — 2026-09-12

## Scope

Commit `beb7333` completes the shared React Web chat main-interface slice:
typed session/history and suggestion contracts, session management controls,
older-history pagination, source/search/date grouping, scoped drafts,
server-confirmed clear, message-suggestion telemetry, and authoritative
post-stream history refresh.

## Implementation evidence

- `client.sessions` exposes encoded list/create/update/pin/unpin/delete/clear
  and bounded message-history requests. `client.chat.suggestions` validates
  suggestion envelopes and records impression/click/dismiss events.
- The shared chat state keeps server message IDs, de-duplicates prepended and
  refreshed rows, preserves scroll position when older history is inserted,
  and only follows the live edge when the reader is near the bottom.
- The Web route loads the selected session, creates a session on first send,
  forwards source and keyword filters, supports flat/date-grouped sidebar
  views, and refreshes persisted history after a completed stream so the
  transient assistant row is replaced by server-owned message IDs.
- Suggestion generation is polled while the server reports `generating`; an
  old session/message request cannot overwrite the active session after a
  switch or regeneration.

## Verification

- `pnpm test:shared`: exit 0, 201/201.
- `pnpm test:web`: exit 0, 106/106.
- `pnpm typecheck:shared`: exit 0.
- `pnpm typecheck:web`: exit 0.
- `pnpm build:web`: exit 0; Vite transformed 135 modules. The existing
  post-minification chunk-size warning remains non-fatal.
- `node scripts/check-react-boundaries.mjs`: exit 0.
- `git diff --check`: exit 0.

## Evidence boundary

The existing isolated Lite + local model-stub browser run in
`t10-t11-web-sse-live-2026-09-12.md` proves the React session creation,
streaming answer, pending-state completion, and authenticated history read.
This commit's new sidebar mutations, source/search filtering, older-history
button, clear action, and suggestion provider were not run against a live
backend in this increment. Real RAG/Agent/third-party generation,
Last-Event-ID recovery, cancellation timeout, permission negatives, and
native chat acceptance remain open; T11 stays `review`.
