# T17 settings resource live evidence — 2026-09-11

## Scope

This increment closes the React Web operation gap for Storage, Vector Stores, and Web Search settings. The UI now exposes typed create/update/delete/test actions for the resource APIs, plus the supported default-storage action. It does not certify provider credentials, external connectivity, Lite feature availability, or native settings runtime.

## Implementation evidence

- `apps/web/src/settings/ResourceSettingsPanel.tsx` renders resource-specific create/edit/test/delete controls against the existing typed settings client. Server errors leave the current form/list state intact; success is shown only after the awaited server response.
- Secret-shaped fields are removed before an existing resource is placed into the editable configuration text. The user may provide a new secret when creating or updating, but the server response is never used to prefill it.
- `apps/web/src/settings/surface.ts` validates resource name/type and requires a JSON object configuration. `settingsResourceRows` handles the Storage envelope (`backends`) separately from Vector Store and Web Search arrays.
- TDD RED was observed when `surface.test.ts` imported the missing helpers; GREEN passed after the minimal helpers and panel integration were added.

## Isolated Lite API probe

Environment: the recoverable test database `/tmp/weknora-react-live-20260911.db`, the Go server on `http://127.0.0.1:18080`, RBAC enabled, and the previously registered isolated owner account. No production/provider credentials were used.

| Request | HTTP | Observed result |
|---|---:|---|
| `GET /api/v1/auth/me` | 200 | `data.user`, `data.tenant`, memberships, and capabilities |
| `GET /api/v1/tenants/kv/retrieval-config` | 200 | six retrieval fields |
| `GET /api/v1/tenants/kv/parser-engine-config` | 200 | endpoint plus redacted API-key metadata |
| `GET /api/v1/tenants/kv/chat-history-config` | 200 | enabled/model/knowledge-base fields |
| `GET /api/v1/storage-backends` | 200 | one configured backend |
| `GET /api/v1/vector-stores` | 200 | empty configured-resource list |
| `GET /api/v1/web-search-providers` | 200 | empty configured-resource list |
| `GET /api/v1/initialization/ollama/status` | 200 | availability/base URL/error payload |
| `GET /api/v1/system/parser-engines` | 200 | eight parser engine rows |
| `GET /api/v1/system/info` | 200 | deployment metadata payload |

The probe confirms the three resource panels have reachable authenticated read contracts. It is not a claim that a provider was created or connected; doing so would require real provider configuration and a separate credential approval.

## Verification

| Command | Result |
|---|---|
| `pnpm --filter @weknora/web test` | exit 0, 58/58 |
| `pnpm --filter @weknora/web build` | exit 0 |
| `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` | exit 0 |
| `node scripts/check-react-boundaries.mjs` | exit 0 |
| `git diff --check` | exit 0 |

## Limitations

T17 remains `review`: browser interaction evidence for the new panel, admin/viewer negative matrix for every resource operation, external connection tests, the remaining parser/retrieval/chat-history/Ollama/WeKnora Cloud writes, and Lite/native capability filtering remain open. The isolated Lite server still reports the pre-existing `tenant_skills` schema warning and has no SQLite FTS5 module.
