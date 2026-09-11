# T23 mobile API key evidence

Date: 2026-09-11
Branch: `codex/react-multiclient`

## Implemented boundary

- `apps/mobile/app/(app)/management/api-keys.tsx` exposes a native workspace API-key route linked from the management hub and administration screen.
- The route uses the shared `administration.tenantApiKeys` client and the backend's actual Owner-only route contract. Admin, contributor, viewer, and unknown roles remain read-only.
- Scoped keys expose only the bounded mobile capability set (`retrieve`, `chat`, `read_agents`, `ingest`, and `manage_knowledge_bases`). Blank names and empty capability selections are rejected locally.
- Newly returned `token` values are shown once in memory and are not written to SecureStore or included in later list rows. Revoke waits for the server response and reloads; failures preserve the visible list.
- `integrations.api` deployment capability is projected into the mobile matrix; unsupported deployments remain visibly unsupported rather than presenting a writable form.

## Automated evidence

```text
pnpm test:mobile       # 32/32, exit 0
pnpm typecheck:mobile  # exit 0
```

Focused TDD evidence: the API-key helper test first failed because the new module was absent; after the implementation it passed as part of the 32-test mobile suite.

## Isolated live backend evidence

Against a freshly started Lite binary on `127.0.0.1:18082` with a temporary
SQLite database and no production data:

```text
POST /api/v1/auth/register                         201
POST /api/v1/auth/login                            200
GET  /api/v1/auth/me                               200 (tenant 9)
POST /api/v1/tenants/9/api-keys                    201 (scoped retrieve key; token returned once)
GET  /api/v1/tenants/9/api-keys                    200 (1 row)
DELETE /api/v1/tenants/9/api-keys/1                200
```

The temporary process was stopped after the probe. The token value was not
written to the repository or included in this evidence.

## Evidence boundary

This proves the native route wiring, Owner-only client-side affordance, scoped input validation, token non-persistence boundary, shared API client usage, type-safe mobile bundle source, and one isolated Lite create/list/revoke sequence. It does not claim the complete server role/tenant 403 matrix, native device interaction, or production secret-management acceptance; those remain open T23/T24 evidence.

## Native iOS live follow-up — 2026-09-12 00:01 CST

- On the installed `com.weknora.mobile` Release app running on iPhone 17 Pro
  iOS 26.5, the authenticated owner opened Manage → Workspace API keys. The
  screen displayed `Workspace role: owner` and the server-backed empty list.
- Entered `ios-live-probe` with the default scoped `retrieve` capability and
  created the key. The native screen displayed the returned token once and
  rendered the server-created row. The row was then revoked through the
  confirmation dialog; the screen returned to `No API keys returned.`.
- An authenticated direct request to `GET /api/v1/tenants/1/api-keys` after
  the revoke returned HTTP `200` with `{"data":[],"success":true}`.
- Screenshot after revoke:
  `/tmp/weknora-ios-api-key-latest.png` (SHA-256
  `3daed49db47610653b638f6dec41ca53615bb7bc7df4a257b941c94a894e9b31`).

This is native iOS plus isolated Lite owner create/list/revoke evidence. It
does not close non-owner role/tenant negatives, physical-device behavior, or
production secret-management acceptance.
