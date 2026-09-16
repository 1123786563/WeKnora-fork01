# Vue/React public browser evidence — 2026-09-15 run 5

This is a paired, unauthenticated browser capture only. It does not claim
overall Vue/React parity, authenticated parity, permission parity, or backend
production acceptance. Vue remains the visual authority.

## Reproduction

```sh
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient
node artifacts/vue-react-public-browser-evidence.mjs \
  artifacts/browser-evidence-20260915-run5
```

The run used fresh Playwright browser contexts, locale `zh-CN`, and a fixed
viewport of `1355x776`. No credentials, tokens, localStorage overrides, mock
responses, or write-capable API calls were used.

Machine-readable results and screenshots:

- `browser-evidence-20260915-run5/results.json`
- `browser-evidence-20260915-run5/vue-login-public.png`
- `browser-evidence-20260915-run5/react-login-public.png`

## Observed paired results

| Check | Vue `:5180` | React `:5181` |
|---|---|---|
| `GET /login` | HTTP 200; final `/login`; visible `登录` heading and two inputs | HTTP 200; final `/login`; visible `登录` heading and two inputs |
| Unauthenticated `GET /platform/apps` | HTTP 200 document; final `/login`; redirect observed | HTTP 200 document; final `/login?next=%2Fplatform%2Fapps`; redirect observed |
| Failed auth-related response during capture | `GET /api/v1/auth/auto-setup` → 403 | `GET /api/v1/auth/auto-setup` → 404; `GET /api/v1/auth/config` → 404 twice; `GET /api/v1/auth/oidc/config` → 404 twice |
| First selected input geometry/style | `374x24px`, transparent background, `0px` radius, `line-height: 24px` | `400x40px`, white background, `8px` radius, browser-normalized line height |
| Screenshot | `vue-login-public.png` | `react-login-public.png` |

The public login body text, input placeholders, button labels, and title were
the same in this capture. The final URL query-string difference and the
HTTP/error-response differences are concrete observations for follow-up;
they are not interpreted here as a global parity verdict.

## Evidence boundary

- `browser-runtime`: fresh local browser execution against both dev servers.
- `dom-and-computed-style`: selected DOM text, controls, geometry, and styles
  recorded in `results.json`.
- `screenshot`: fixed-viewport PNGs for visual comparison.
- `blocked-env`: `browser-use --doctor` reported Chrome and daemon available,
  but `active browser connections — 0`; the capture therefore used the
  repository's Playwright fallback. No authenticated credentials or permission
  fixtures were available, so authenticated pages, tenant isolation, real
  provider behavior, Wails, iOS, and Android remain unverified.
