# T24 React candidate Nginx proxy evidence — 2026-09-11

## Scope

- Source baseline: `codex/react-multiclient` at `206c9d3`; this evidence also
  includes the uncommitted Nginx cache-location fix recorded in the same
  worktree before its follow-up commit.
- Static candidate: `dist/react-web/web/`, with `BUILD_INFO.json` identifying
  renderer `react` and commit `0e886be`.
- Runtime image: `weknora-ui:t24-nginx-fix`, built from the repository-root
  Docker context with the pinned Node/Nginx inputs in `frontend/Dockerfile`.
- Backend: an isolated temporary HTTP/upgrade mock, not a production service
  and not a substitute for authenticated backend acceptance.

## Red → green finding

The first black-box run against the existing Nginx template returned
`Cache-Control: no-cache, must-revalidate` for
`/embed/assets/index-CNblXGm-.js`. The broad `location ^~ /embed/` matched before
the main `/assets/` location, so the Embed hashed bundle did not receive the
long-lived cache policy required by the release runbook.

The template now has a dedicated `location ^~ /embed/assets/` before the page
fallback. It uses `try_files $uri =404` and returns
`public, max-age=31536000, immutable` for hashed Embed assets.

## Commands and observed results

```text
git diff --check                                      exit 0
docker build --check -f frontend/Dockerfile .       exit 0
docker build --build-arg VITE_FRONTEND_COMMIT=0e886be \
  -t weknora-ui:t24-nginx-fix -f frontend/Dockerfile . exit 0
```

The candidate was mounted into the rebuilt image. The temporary backend
returned known payloads for `/api/v1/*`, `/files`, `/r/*`, SSE, and a valid
WebSocket handshake. The final black-box assertions passed:

| Request | Result | Evidence |
|---|---:|---|
| `GET /` | 200 | React Web HTML fingerprint; `no-cache` |
| `GET /platform/chat/session-1` | 200 | React Web SPA fallback |
| `GET /embed/channel-1` | 200 | dedicated Embed HTML fingerprint; no `X-Frame-Options` |
| `GET /assets/index-CCoUCqKy.js` | 200 | `public, max-age=31536000, immutable` |
| `GET /embed/assets/index-CNblXGm-.js` | 200 | `public, max-age=31536000, immutable` after the fix |
| `GET /api/v1/proxy-probe` | 200 | backend echoed `/api/v1/proxy-probe` |
| `GET /files` | 200 | backend returned `file-probe` |
| `GET /r/token` | 200 | backend returned `resource-probe` |
| `GET /api/v1/sse` | 200 | `text/event-stream`; both mock events received |
| terminal WS with query ticket | 101 | `Upgrade: websocket`, `Connection: upgrade` |

The WebSocket curl process intentionally timed out after receiving the 101
headers because the mock kept the upgraded socket open; the reported HTTP
status was `101`, not an application error. Nginx access logging omitted the
query string for this terminal route.

## Boundary

This proves the local React candidate artifact and the repository Nginx
template against a deterministic mock backend. It does not prove a deployed
registry image, ingress sub-path behavior, real authenticated SSE/WS tickets,
or the role × tenant × browser/OS matrix. T24 therefore remains `review`.
