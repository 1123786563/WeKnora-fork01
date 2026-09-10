# T17 tenant settings live evidence — 2026-09-11

## Scope

This evidence covers only the active tenant read and name/description update slice of T17. It does not certify the remaining settings sections, role matrix, Lite capability filtering, or native settings UI.

## Environment

- React Web: Vite at `http://localhost:5173`
- Backend: isolated Go server at `http://127.0.0.1:18080`
- Database: temporary SQLite file `/tmp/weknora-react-live-20260911.db`
- Browser: Chrome, authenticated test account `react-live@example.com`
- No production data or external payment/provider credentials were used.

## Evidence

1. The pre-fix browser request was `GET /api/v1/auth/tenant`; the Go request log recorded HTTP `404`. The backend's supported identity response is `GET /api/v1/auth/me` with the active tenant at `data.tenant`.
2. The shared client was corrected to use `/api/v1/auth/me`, and `packages/api-client/src/settings/index.test.ts` asserts the exact path and extraction of `data.tenant`.
3. Chrome loaded `/platform/settings?section=tenant` and rendered the authenticated tenant (`id: 1`, status `active`) with editable name and description fields.
4. Saving `React live workspace updated` / `Live tenant settings verification` produced `PUT /api/v1/tenants/1`, HTTP `200`, and the backend log message `Tenant updated successfully`. The UI then displayed both updated values and the updated server timestamp.

## Fresh verification

| Command | Result |
|---|---|
| `pnpm test:shared` | exit 0, 150/150 |
| `pnpm test:web` | exit 0, 57/57 |
| `pnpm typecheck:shared` | exit 0 |
| `pnpm typecheck:web` | exit 0 |
| `pnpm build:web` | exit 0 |
| `node scripts/check-react-boundaries.mjs` | exit 0 |
| `git diff --check` | exit 0 |

## Additional memory evidence

- `/platform/settings?section=memory` rendered the tenant-admin workspace control with the server's `enabled: false` value. Saving the switch sent `PUT /api/v1/tenants/kv/memory-config` with the typed fields `enabled`, `write_mode`, `max_items`, `vector_recall`, and `retrieval_conditioning`, and the backend returned HTTP `200`.
- Before that workspace write, `/platform/settings?section=mymemory` attempted to create a personal memory and the server returned the explicit error `memory is disabled`; the UI kept the draft and displayed the error.
- After the workspace was enabled, creating `React migration live memory` returned HTTP `200` from `POST /api/v1/memory/items` with an active `fact` row, and the React page rendered the returned row. This is isolated test data only.

## Limitations

This isolated server logged pre-existing `no such table: tenant_skills` reaper warnings and lacks SQLite FTS5, so it is not evidence for Skills or full retrieval acceptance. Remaining T17 settings mutations and permission-negative/live Lite/native evidence are still required.
