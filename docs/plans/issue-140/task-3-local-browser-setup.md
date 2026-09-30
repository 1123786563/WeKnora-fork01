# T03 Live Local Browser Setup Research

- Revision inspected: `cdd8e08d25d8f71258e90322932d7112f76b84f0` (`git rev-parse HEAD`)
- Worktree: `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01`
- Scope: determine isolated live-server/browser feasibility for T03 Career HTTP acceptance. No source or test files edited; no database, service, credential, or user data touched.
- Inspection date: 2026-09-24 Asia/Shanghai.

## Findings

- `cmd/server/main.go` starts the production Gin router from the dependency-injection container. `internal/config.LoadConfig` searches `./config` and calls `viper.AutomaticEnv()` with `.` mapped to `_`; therefore `SERVER_HOST` and `SERVER_PORT` override `config/config.yaml`'s `0.0.0.0:8080`.
- `internal/container/container.go` selects the DB from `DB_DRIVER`; the SQLite path is `DB_PATH`. `STREAM_MANAGER_TYPE=memory` avoids Redis. `STORAGE_TYPE=local` plus `LOCAL_STORAGE_BASE_DIR` keeps uploads under a caller-selected path. `RETRIEVE_DRIVER=sqlite` selects local SQLite retrieval. These are the same relevant settings documented by `.env.lite.example`.
- Registration is mounted at `POST /api/v1/auth/register` (`internal/router/routes_auth_tenant.go`) and accepts public registration unless `DISABLE_REGISTRATION=true` / invite-only config is set (`internal/handler/auth.go`). Login is `POST /api/v1/auth/login`; a disposable account can be registered and then used to obtain its session token. Do not put that token in reports or logs.
- A Career profile/open/list/action check needs no model. A source-file upload / parse gate is different: it requires the relevant DocReader/parser dependency and possibly a model provider depending on intake behavior. Current T03 scope is the ordinary Career base-profile UI, not T07 profile-file parsing.
- Docker daemon is present (`docker info` exit 0), but no isolated server stack was started. Existing unrelated Docker containers include `WeKnora-postgres-dev`, `WeKnora-redis-dev`, `WeKnora-docreader-dev`, and a Lago project; the documented local Lite setup can avoid contacting/stopping them. No new service or listener was created.
- Go `go1.26.3 darwin/arm64` and pnpm are present; this worktree already has `node_modules`. Thus build/runtime prerequisites appear available. This report does not claim a successful server startup or browser request.

## Safe disposable run recipe (not executed)

Run from the integration worktree; choose a loopback port that is confirmed free immediately before launch. Keep the DB, local files, and logs in a unique temporary directory. Set `SERVER_HOST=127.0.0.1` so the process is not exposed on LAN. Substitute the checked free port for `49173` if occupied.

```sh
tmp="$(mktemp -d /tmp/weknora-t03-browser.XXXXXX)"
mkdir -p "$tmp/files"
DB_DRIVER=sqlite \
DB_PATH="$tmp/weknora.db" \
RETRIEVE_DRIVER=sqlite \
STREAM_MANAGER_TYPE=memory \
STORAGE_TYPE=local \
LOCAL_STORAGE_BASE_DIR="$tmp/files" \
SERVER_HOST=127.0.0.1 \
SERVER_PORT=49173 \
DISABLE_REGISTRATION=false \
JWT_SECRET="$(openssl rand -hex 32)" \
SYSTEM_AES_KEY="$(openssl rand -hex 16)" \
GIN_MODE=release \
go run ./cmd/server
```

The server's signal handler supports graceful termination; terminate only this uniquely started process, verify it exited, then remove only the printed `$tmp` directory. Avoid shell tracing and avoid capturing auth response bodies in persistent logs. The checked-in `config/config.yaml` supplies other application settings; runtime-specific external capabilities should stay unused in the profile base flow. If this command fails during container initialization due to a required non-Career dependency, record the exact startup error and classify the live gate blocked rather than attaching an existing dev service.

## Acceptance feasibility and gaps

- **Isolated authenticated HTTP:** feasible with a disposable SQLite DB, memory stream manager, local temp storage, and a free loopback port. Registration defaults to self-serve in the checked Lite config. Register then log in through the public auth routes; the API request to `/api/v1/career/open` uses the resulting bearer token. T03 owner/single-member tenant handling remains the server's authority.
- **Browser UI:** feasible after the Web client changes are integrated, using the same browser origin/proxy arrangement expected by the web app. The exact web development server/proxy command was not verified here. The backend is configured for loopback HTTP; do not use shared ports `8080`, `80`, or the active development stack ports without checking availability.
- **Not yet evidenced:** successful clean server boot, registration/login round trip, Career open/action round trip, Web login/profile flow, refresh persistence, and cross-tenant rejection on the live router. Current report is setup research only; focused backend tests in `task-3-integrated-backend-validation.md` do not supply this live browser evidence.
- **Dependency caveat:** do not enable/upload documents in this base gate. Career profile intake/parse may contact DocReader or model providers and has separate acceptance. No Docker dependency should be started for base Career HTTP, but whether this branch's complete DI container boots with Lite settings and no external services remains unconfirmed because runtime startup was not attempted.

## Exact inspection commands and results

| Command | Result |
| --- | --- |
| `git rev-parse HEAD` | `cdd8e08d25d8f71258e90322932d7112f76b84f0` |
| `git status --short` | clean before this permitted report was added |
| `rg -n 'DB_DRIVER|DB_PATH|STREAM_MANAGER_TYPE|DISABLE_REGISTRATION|SYSTEM_AES_KEY|TENANT_AES_KEY|RETRIEVE_DRIVER' .env.lite.example .env.example internal/config internal/container` | confirms Lite variable names and DB selection/path |
| `sed -n '760,815p' internal/config/config.go` | confirms config search paths and automatic env override |
| `sed -n '1,125p' cmd/server/main.go` | confirms server entrypoint and configured listener |
| `sed -n '205,220p' internal/router/routes_auth_tenant.go` | confirms public auth route mount |
| `rg -n 'RegisterCareerRoutes|/career' internal/router` | Career routes mounted by router; package path is authenticated v1 API |
| `go version` | `go version go1.26.3 darwin/arm64` |
| `command -v pnpm` | `/opt/homebrew/bin/pnpm` |
| `docker info >/dev/null 2>&1` | exit 0; daemon available, not used |
| `docker ps --format '{{.Names}} {{.Ports}}'` | showed existing unrelated WeKnora dev and Lago containers; none touched |

## Status

**DONE_WITH_CONCERNS** — setup route and isolated knobs are verified by configuration/source inspection, and the live gate is feasible for T03's no-model base profile path. Clean container startup and actual authenticated browser behavior are still unverified; do not count this setup note as satisfying browser acceptance.
