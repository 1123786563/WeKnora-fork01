# T24 React Docker candidate — 2026-09-12

## Scope

This verifies the separate React/Nginx candidate image built from the
repository root context. The existing `frontend/Dockerfile` and its Vue image
remain the production/recovery input until T25 is accepted.

## Build and runtime

```text
docker build --check -f apps/web/Dockerfile .
# Check complete, no warnings found.

docker build --build-arg REACT_BUILD_VERSION=verify \
  --build-arg VITE_FRONTEND_COMMIT=f231d1c \
  -f apps/web/Dockerfile -t weknora-ui-react:candidate-20260912 .
# exit 0; image manifest sha256:c33a1aaee8a61ca8514498ea50f98ac67f344d5cd921aa2ee702bb13a7dc372f
```

The first build attempt exposed a 4.8 GB context caused by local native and
Node build outputs. `.dockerignore` now excludes root and nested
`node_modules`, mobile `android`/`ios`, nested build/dist outputs, TypeScript
build info, and local binaries. The successful retry transferred about 24 KB
of context and built the image successfully after the pinned dependency
install completed.

The image was started with `APP_HOST=127.0.0.1` (no backend was attached) and
the following assertions passed:

| Request | Result |
|---|---|
| `GET /` | HTTP 200, React Web fingerprint |
| `GET /platform/chat/session-1` | HTTP 200, Web SPA fallback |
| `GET /embed/channel-1` | HTTP 200, Embed fingerprint |
| `GET /BUILD_INFO.json` | `renderer: react`, commit `f231d1c`, Web/Embed entries |

The test container was removed after the assertions. No registry push or
production service was used.

## Boundary

This closes a reproducible root-context React candidate Docker build and local
static runtime smoke. It does not prove registry/deployed behavior, backend
proxying, SSE/Terminal WS, sub-path ingress, or T24 acceptance.
