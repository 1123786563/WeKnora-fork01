# T03 SQLite Startup Fix Validation

- Revision: `21d45eb799f1d66e73504220835289f566aeb7e1`
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-t03-sqlite-fix/WeKnora-fork01`
- Validation date: 2026-09-24 Asia/Shanghai
- Scope: assigned SQLite startup fix only. No source or test source changed; this report is the only added file.

## Acceptance checked

The versioned SQLite Career schema can be opened by `career.NewOffice` without destructive schema rebuilding, including a second initialization against the same migrated DB, and the schema's unique constraints remain enforced. Focused migration and Career/DB behavior suites pass. A disposable Lite server also initialized on a loopback port with SQLite and memory stream settings. No authenticated Career HTTP acceptance was attempted.

## Commands and results

| Command | Result |
| --- | --- |
| `git rev-parse HEAD` | `21d45eb799f1d66e73504220835289f566aeb7e1` |
| `go test ./internal/database -run 'TestCareerMigrationCreatesPersonalEvidenceSchema|TestCareerOfficeOpensAfterVersionedSQLiteMigration' -count=1` | PASS (`ok .../internal/database 3.259s`) |
| `go test ./internal/modules/career -count=1` | PASS (`ok .../internal/modules/career 1.918s`) |
| `go test ./internal/database -count=1` | PASS (`ok .../internal/database 7.447s`) |
| `curl --max-time 2 -sS -o /dev/null -w 'GET /healthz status=%{http_code}\n' http://127.0.0.1:49173/healthz` | HTTP 401; listener responded, unauthenticated request denied |
| `curl --max-time 2 -sS -o /dev/null -w 'GET /api/v1/career/open status=%{http_code}\n' http://127.0.0.1:49173/api/v1/career/open` | HTTP 401; unauthenticated Career request denied |
| `lsof -nP -iTCP:49173 -sTCP:LISTEN` after termination | No listener remained |
| `git status --short` before report | clean |

## Evidence and limitations

`TestCareerOfficeOpensAfterVersionedSQLiteMigration` runs the versioned SQLite migration into `t.TempDir()`, opens it with GORM, calls `NewOffice` twice, claims a scoped space, then proves duplicate `(tenant_id,user_id,key)` facts, duplicate receipt request IDs, and duplicate `(tenant_id,user_id,revision)` changes are rejected. This directly exercises the regression point and the uniqueness constraints after repeated initialization.

Live startup used the exact entrypoint `go run ./cmd/server` with `DB_DRIVER=sqlite`, a unique temporary `DB_PATH`, `RETRIEVE_DRIVER=sqlite`, `STREAM_MANAGER_TYPE=memory`, `STORAGE_TYPE=local`, temp local storage, `SERVER_HOST=127.0.0.1`, `SERVER_PORT=49173`, `DISABLE_REGISTRATION=false`, `GIN_MODE=release`, `JWT_SECRET="$(openssl rand -hex 32)"`, and `SYSTEM_AES_KEY="$(openssl rand -hex 32)"`. The temporary directory was `/tmp/weknora-t03-sqlite-validator.rSdh0q`; it was removed after the process exited. No key or token was recorded.

The server logged `no such module: fts5` while initializing the separate retrieval subsystem, but continued through initialization and accepted loopback HTTP requests. The assigned Career migration path passed independently. The live probe was intentionally unauthenticated; tenant authorization, authenticated Career operations, cancellation behavior, and PostgreSQL behavior are not part of this SQLite startup-fix evidence. `go run` was stopped with Ctrl-C; its child PID 66276 was then sent SIGTERM, and subsequent process/listener checks were empty.

## Status

**DONE_WITH_CONCERNS** — assigned SQLite migration → `NewOffice` startup behavior and uniqueness acceptance pass at the exact revision. Full authenticated HTTP/browser behavior is outside this focused fix validation. The missing SQLite FTS5 module is a runtime limitation for retrieval, although the app listener and Career DB initialization remained available.
