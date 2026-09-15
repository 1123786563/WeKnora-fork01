# Vue/React public browser evidence

This evidence is intentionally limited to the public, unauthenticated surface.
Vue remains the visual authority. The script does not use credentials, mock
responses, localStorage tokens, or write-capable API calls.

## Reproduction

From the target worktree:

```sh
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/react-multiclient
node artifacts/vue-react-public-browser-evidence.mjs artifacts/browser-evidence-YYYYMMDD
```

The Vue dev server must be reachable at `127.0.0.1:5180` and the React dev
server at `127.0.0.1:5181`. Override them with `VUE_URL` and `REACT_URL` when
needed. The default browser context is `1355x776` CSS pixels and `zh-CN`.

The script captures `/login` and `/platform/apps` for each client. It writes a
fixed-viewport PNG, `results.json` containing response/final URLs, visible DOM
summary, and selected computed styles for `h1`, `input`, and `button`.

## Evidence classification

| Check | Classification | Meaning |
|---|---|---|
| `/login` HTTP/DOM render | browser runtime, public | Proves only that the public login document rendered in the observed dev server. |
| `/platform/apps` without credentials | browser runtime, unauthenticated | A final `/login` URL proves the client-side unauthenticated guard observed in this run. |
| PNG and computed styles | visual/DOM evidence | Useful for paired comparison; not a backend or permission acceptance claim. |
| Authenticated app catalog, connection, approval, and action states | `blocked-env` | No test credentials/permission fixtures were supplied for this run. |
| Backend success/error and tenant isolation | `blocked-env` | The local backend listener may exist, but this public-only run did not establish authenticated handler behavior. |
| Wails, iOS, Android | `blocked-env` | This script runs in a desktop browser only. |

## Run record

Run completed on 2026-09-15 at `2026-09-15T12:07:12Z` in
`artifacts/browser-evidence-20260915-run3/`.

| Client | `/login` | Unauthenticated `/platform/apps` | Browser console |
|---|---|---|---|
| Vue | HTTP 200; login heading and form visible | HTTP 200 document; final URL `/login`; redirect observed | `GET /api/v1/auth/auto-setup` → 403 |
| React | HTTP 200; login heading and form visible | HTTP 200 document; final URL `/login?next=%2Fplatform%2Fapps`; redirect observed | `GET /api/v1/auth/auto-setup` → 404; `GET /api/v1/auth/config` → 404 (twice); `GET /api/v1/auth/oidc/config` → 404 (twice) |

Both pages exposed the same zh-CN public login content in this run. The
captured computed-style records show the current implementation difference:
the first email input was `374x24px`, transparent, radius `0px` in Vue and
`400x40px`, white, radius `8px` in React. This is a parity observation for
follow-up, not an acceptance decision.

The generated `results.json` and PNGs are the machine-readable/run-specific
record. Keep their timestamp and output directory when adding a row to the
parity ledger. Do not promote this report to full Vue/React parity acceptance.

## Tooling note

`browser-use --doctor` confirmed Chrome and its daemon, but the CLI execution
could not import the external `agent_helpers` module in this environment. The
Playwright Core script is therefore the executable fallback; the missing CLI
helper is recorded rather than hidden.
