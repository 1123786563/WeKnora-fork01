# T07 Web protected preview live evidence — 2026-09-12

Environment: isolated Lite server on `127.0.0.1:18085` and React Web Vite
server on `127.0.0.1:5177`, using the same authenticated fixture account.

## API evidence

- `GET /api/v1/knowledge/<document-id>/preview` without credentials returned
  `401 Unauthorized`.
- The same request with the fixture account bearer token returned `200 OK`,
  `Content-Type: text/markdown; charset=utf-8`,
  `Content-Disposition: inline; filename="Private preview fixture.md"`,
  `Cache-Control: private, no-store`, and `X-Content-Type-Options: nosniff`.
- Response body was `browser preview content\nsecond line` (35 bytes,
  SHA-256 `7e0692730b4e720619c720841333d1a742a38cf2bb7ccc5036d673107671ea0c`).

## Browser evidence

After signing in at `/login` as `react-t07-preview@example.test`, the browser
opened:

`/knowledgeBase/ce344af1-ad15-49d5-8258-afc9c82abd02/documents/acd63939-4f82-4da3-ae57-3d3d3a25cf3d`

The accessibility tree showed:

- heading `Private preview fixture.md`;
- status `completed`;
- preview content `browser preview content` and `second line`;
- button `Download Private preview fixture.md`.

The browser initially showed the authoritative unavailable state while the
document was `processing`; after the disposable isolated SQLite fixture was
advanced to `completed`, the same route rendered the protected preview content
and download action.

This proves authenticated API bytes and the Web success-state renderer. The
full native browser download-byte assertion and all legacy format variants
remain separate acceptance work; T07 remains `review`.
