# T24 current React candidate Nginx rerun — 2026-09-12

## Scope

The current React Web/Embed candidate was rebuilt at source commit `f231d1c`
and mounted into the locally built Nginx image `weknora-react-current:20260912`.
The container used an unused backend port, so no API request was represented
as a successful backend call. This is a local static/proxy check only.

## Commands and results

```text
docker build --check -f frontend/Dockerfile .                 exit 0
docker build --build-arg VITE_FRONTEND_COMMIT=8992438 \
  -f frontend/Dockerfile -t weknora-react-current:20260912 .  exit 0
pnpm build:react-bundle                                        exit 0
```

The Nginx image build is the current-source image build; the historical
`VITE_FRONTEND_COMMIT` argument shown in the command is not used to identify
the mounted candidate. The mounted candidate identity is taken from its
`dist/react-web/web/BUILD_INFO.json`, which reports `f231d1c`.

The rebuilt candidate `dist/react-web/web/BUILD_INFO.json` reports
`renderer: react`, `version: dev`, and `commit: f231d1c`. A container started
from the current-source Nginx image with the candidate artifact mounted at
`/usr/share/nginx/html`; after the normal short startup retry, all assertions
passed:

| Request/assertion | Result |
|---|---|
| `GET /` | HTTP `200`, React Web fingerprint |
| `GET /platform/chat/session-1` | HTTP `200`, Web SPA fallback |
| `GET /embed/channel-1` | HTTP `200`, dedicated Embed fallback |
| hashed Web asset | `Cache-Control: public, max-age=31536000, immutable` |
| hashed Embed asset | `Cache-Control: public, max-age=31536000, immutable` |

The test container was removed after the assertions. The candidate artifact
and test database remain outside Git; no production service or data was used.

## Boundary

This rerun strengthens current-source Docker/Nginx static evidence. It does
not prove API proxying against a real backend, deployed registry behavior,
sub-path ingress, SSE/terminal upgrade, performance thresholds, or the full
role/OS matrix. T24 remains `review`, and the Vue artifact remains the
production/recovery input until the documented retirement gates are met.
