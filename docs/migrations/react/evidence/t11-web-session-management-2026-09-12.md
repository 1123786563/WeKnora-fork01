# T11 Web session management (2026-09-12)

## Scope

The React Web chat now exposes server-confirmed session title, pin, unpin, and delete operations in the conversation sidebar.

## Implementation evidence

- Commit `fecbbb7` adds typed `update`, `pin`, `unpin`, and `remove` methods to `client.sessions`, with encoded session IDs and strict success/session parsing.
- The sidebar renders Rename, Pin/Unpin, and Delete actions. The route updates local rows only after the server response succeeds; deleting the selected session returns to `/platform/creatChat` and clears its local message/stream state.
- Rename uses the browser prompt and rejects an empty title before issuing a mutation. Delete remains confirmation-gated.

## Verification

- `pnpm exec tsx --test packages/api-client/src/chat/sessions.test.ts`: exit 0, 4/4.
- `pnpm --filter @weknora/web test`: exit 0, 91/91.
- `pnpm typecheck:shared`: exit 0.
- `pnpm typecheck:web`: exit 0.

This proves the typed and rendered session-management slice. Live browser mutation, server error/permission negatives, title-generation, source filtering, and native acceptance remain open; T11 stays `review`.
