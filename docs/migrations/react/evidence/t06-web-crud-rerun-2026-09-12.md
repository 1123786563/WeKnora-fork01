# T06 React Web knowledge-base CRUD rerun — 2026-09-12

## Scope

This is an isolated local browser run against the current React Web app and
Lite backend. It covers the owner create/edit/list slice only; it does not
claim document upload/processing, detail permissions, pagination across
multiple pages, or delete confirmation behavior.

## Environment

- Worktree: `codex/react-multiclient` at `6f90877`
- React Web: `http://127.0.0.1:5175/` with API base `http://127.0.0.1:18082`
- Backend/data: isolated Lite process and SQLite/files under
  `/tmp/weknora-react-web-chat-20260912/`
- Browser: Codex in-app browser tab 1 using the connected browser control
  surface; no production account or data

## Evidence

1. After registering and signing in as an isolated owner, the React route
   `/platform/knowledge-bases` loaded the server-backed empty state. Its scope
   key contained origin `http://127.0.0.1:18082`, the current user ID, tenant
   `2`, and the `knowledge-bases` resource.
2. The page submitted `React Web CRUD KB` using the React form. The returned
   list rendered one server-generated knowledge-base row with `Open documents`,
   `Edit`, and `Delete` actions.
3. The page entered edit mode, changed the name to `React Web CRUD KB updated`,
   submitted `Save changes`, and rendered the updated name only after the
   mutation completed and the list was reloaded.
4. An independent authenticated `GET /api/v1/knowledge-bases?creator=all`
   returned HTTP `200`, one row, and the exact updated name and `document` type.

The earlier same-day evidence in
`t06-t17-web-live-2026-09-12.md` covers the corresponding create/update
responses with explicit backend `201`/`200` logs; this rerun adds a second
browser-confirmed owner path after the Web API base startup fix.

## Boundary

This is real isolated backend plus browser evidence, not a mock test. It does
not accept T06 yet because detail/document behavior, processing, permission
negatives, delete semantics, and the complete browser journey remain open.
