# T17 field-level configuration live evidence — 2026-09-11

## Scope

This increment adds field-level React Web forms for Retrieval, Chat History, and Parser Engine configuration. It does not certify a real external model/parser connection or the remaining Ollama/WeKnora Cloud credential lifecycle.

## Implementation evidence

- `apps/web/src/settings/ConfigSettingsPanel.tsx` loads and saves the supported fields for each section, keeps the server response as the confirmation boundary, and preserves in-progress form values when a save fails.
- Retrieval uses numeric range controls for top-K and thresholds. Chat History exposes only the server-supported enable/model fields; the server-owned generated knowledge-base identity remains read-only. Parser exposes endpoint plus a blank-by-default API-key input and a separate parser-engine check action.
- `apps/web/src/settings/surface.ts` validates ranges, HTTP(S) parser endpoints, and boolean chat-history state. A blank parser API key is omitted from the update payload so an existing server-side secret is not overwritten or displayed.
- The Parser section now loads both the engine availability probe and the tenant config rather than leaving the config as a read-only summary.
- `apps/web/src/settings/OllamaSettingsPanel.tsx` now provides server-backed status/retest, model listing, download initiation, and task-progress refresh; the deployment-owned base URL remains read-only.
- `apps/web/src/settings/CloudSettingsPanel.tsx` now provides server-backed status and credential submission. App ID and app secret are required, cleared after success, and never read back into the form.

## Isolated Lite API probe

Environment: recoverable database `/tmp/weknora-react-live-20260911.db`, Go server `http://127.0.0.1:18080`, RBAC enabled, isolated owner account, and no external provider credentials.

| Request | HTTP | Result |
|---|---:|---|
| `PUT /api/v1/tenants/kv/retrieval-config` | 200 | `Retrieval configuration updated successfully`; payload restored to the observed baseline |
| `PUT /api/v1/tenants/kv/chat-history-config` | 200 | `Chat history configuration updated successfully`; payload restored to disabled baseline |
| `PUT /api/v1/tenants/kv/parser-engine-config` | 200 | `解析引擎配置已更新`; blank API key was omitted by the React payload helper |
| `POST /api/v1/system/parser-engines/check` | 200 | `code: 0`, `connected: false`; disconnected state remains visible rather than inferred as healthy |

The same authenticated probe returned `GET /api/v1/initialization/ollama/status` with HTTP 200 and `available: false`; no Ollama service was running, so no model download was attempted. The WeKnora Cloud status read was not used to claim configured credentials.

The responses prove the authenticated owner write/check contracts, not successful third-party connectivity. The parser probe reported the expected disconnected state because no DocReader was configured.

## Verification

| Command | Result |
|---|---|
| `pnpm --filter @weknora/web test` | exit 0, 61/61 focused/full slice tests |
| `pnpm --filter @weknora/web exec tsc -p tsconfig.json --noEmit` | exit 0 |

Full repository verification is rerun before the corresponding commit is finalized.

## Limitations

T17 remains `review`: browser interaction evidence for these new controls, viewer/admin negative coverage for every setting write, Ollama model download/progress, WeKnora Cloud credentials, and native/Lite capability filtering remain open. The isolated Lite backend still has the pre-existing `tenant_skills` schema warning and no SQLite FTS5 module.
