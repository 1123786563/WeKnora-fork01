# T03/#141 isolated live HTTP validation

- Candidate: `e705875cd794247a462e951f4bf3c3df6bf0c2b5` on `codex/issue-140-integration`, 2026-09-24 Asia/Shanghai.
- Scope: actual Lite server and authenticated Career backend HTTP. T03 Web client and browser UI were not part of this run.
- Isolation: newly created `/tmp/weknora-t03-live.S47k3w` SQLite database/local files, memory stream, loopback `127.0.0.1:62628`, generated JWT/AES keys and disposable accounts. Existing services, ports, databases and accounts were not used. The process was stopped and both test directories were removed after evidence capture.

## Commands and observed results

`go test ./internal/modules/career/... ./internal/database/... ./internal/router/... ./internal/modules/workbench/... ./internal/handler/session/...` passed at this integration SHA. The process then launched with `DB_DRIVER=sqlite`, a new `DB_PATH`, `RETRIEVE_DRIVER=sqlite`, `STREAM_MANAGER_TYPE=memory`, `STORAGE_TYPE=local`, `LOCAL_STORAGE_BASE_DIR` in the same temporary directory, `SERVER_HOST=127.0.0.1`, `SERVER_PORT=62628`, public registration enabled, generated signing/encryption keys, and `go run ./cmd/server`.

The server remained listening. `GET /api/v1/career/open` without authentication returned **401**. A Python standard-library HTTP probe created two disposable accounts in the isolated database: both registration requests returned **201**, both logins **200**, and their active Tenant IDs differed. The login wire has `active_tenant` (the older API example's `tenant` key is not current); the first probe stopped after discovering this shape and made no Career mutation. A subsequent probe then observed:

| Probe | Result |
| --- | --- |
| Account A, `GET /career/open` with bearer and own `X-Tenant-ID` | 200, revision 0 |
| Account A, `POST /career/act` propose with stable request ID and expected revision | 200, `kind=proposed` |
| Account A, `POST /career/act` confirm proposal with new request ID | 200, `kind=confirmed` |
| Account A, reopen Career | 200, revision 2, one confirmed fact |
| Account A, lookup confirmation by its request ID | 200, revision matches confirmation receipt |
| Account A bearer with account B's `X-Tenant-ID` | 403, no profile fields |

The first cross-Tenant probe script failed while inspecting an error body's type; a fresh two-account probe independently asserted own-scope **200**, cross-Tenant **403**, and absence of `facts`/`proposals` in the denied body. No token, password, signed URL, account identifier, or full response body was written to this report.

## Limits

This establishes real router/auth/membership and Career Office behavior on a fresh SQLite Lite server. It does not establish Web login, UI persistence after browser reload, delayed response suppression, or actual device behavior. PostgreSQL runtime concurrency has separate evidence in the T03 backend round-4 report. Startup logged SQLite FTS5 unavailable and repeated missing optional mobile-notification-provider state; neither prevented these Career HTTP probes. The untracked disposable database and process were cleaned up.
