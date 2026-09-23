# T04/#142 isolated live artifact-version HTTP acceptance

Date: 2026-09-24 Asia/Shanghai. Candidate: integration branch after `25a654f74`, including the reviewed T04 implementation commits. Source: Issue #142 acceptance and `task-4-integrated-validation.md`.

An isolated Lite server ran on `127.0.0.1:54512` with a fresh SQLite database, local file storage, memory stream and generated JWT/AES/artifact-signing keys. Two disposable users registered through the public auth API, each with a separate personal Tenant. No shared database, account, service, model or deployment was used. A test fixture inserted a session, an owned Workbench run, and two immutable `ready` artifact-version rows into the fresh database. Their object keys resolved to actual files under this server's isolated local storage. The fixture bypassed Task generation; it exercised the deployed router, middleware, signed-grant handler, repository and file streamer with real stored bytes.

## Observations

| Live request / check | Result |
| --- | --- |
| Owner bearer, `POST /api/v1/workbench/executions/r1/artifact-versions/v1/signed-url` | 200; response carried fixed version, digest, size, MIME and signed URL |
| Credential-free `GET` of that signed URL | 200; 23 downloaded bytes exactly equal the stored file; SHA-256 of downloaded bytes equals both stored file SHA-256 and response `data.digest` |
| Owner bearer, `DELETE /api/v1/workbench/executions/r1/artifact-versions/v1` | 200 |
| Reuse original signed URL after revoke | 404 |
| Different Tenant bearer attempts to sign run `r1` / version `v2` | 404 |
| Change `version_id` in a valid signed URL without changing signature | 404 |
| Sign version `v2` with `ttl_seconds=1`, request after 2 seconds | 404 |

All assertions ran against the live loopback server and passed. The existing artifact download compatibility and migration test evidence remains in `task-4-integrated-validation.md`. The local probe was an HTTP client request to the Web server's public API, not a browser UI click and not an artifact produced by an LLM-backed Task. Those two broader interpretations of “从 Web 下载一个真实产物” remain unverified. No credentials, tokens, signed URLs or full private response bodies are retained in this report. The isolated process and database were stopped and removed after the run.

## Reproduction outline

Start `go run ./cmd/server` with `DB_DRIVER=sqlite`, a new `DB_PATH`, `RETRIEVE_DRIVER=sqlite`, `STREAM_MANAGER_TYPE=memory`, `STORAGE_TYPE=local`, `LOCAL_STORAGE_BASE_DIR` under the same temporary directory, `SERVER_HOST=127.0.0.1`, a free `SERVER_PORT`, `DISABLE_REGISTRATION=false`, generated JWT/AES keys, `WEKNORA_ARTIFACT_SIGNING_KEY` generated from 32 random bytes, and `APP_EXTERNAL_URL` set to that loopback origin. Register and log in through `POST /api/v1/auth/{register,login}`; use its bearer token and active Tenant ID for owner requests. In the resulting migrated SQLite DB, add one `sessions` row owned by that user, one `agent_runs` row with the same `(tenant_id, session_id, owner_id)`, and a `ready` `artifact_versions` row bound to the same run/session. Set its `object_key` to `local://tenant/fixture.txt`, `size` and `digest` to the actual file bytes in `$LOCAL_STORAGE_BASE_DIR/tenant/fixture.txt`; this run used `exact artifact payload\n` (23 bytes). Then call the signed-url route above with bearer and `X-Tenant-ID`, download the returned `data.url` without authentication, and compare SHA-256 of the downloaded file, stored file, and response `data.digest`. Finally use the owner bearer for the DELETE route and retry that same URL. For cross-Tenant, register a second user and use its own bearer/Tenant ID against the first run. For tampering, change the `version_id` query parameter of an issued URL. For expiry, request a separate ready version with `?ttl_seconds=1` and retry its URL after 2 seconds. The fixture was removed, so IDs and secrets must be regenerated for another run.
