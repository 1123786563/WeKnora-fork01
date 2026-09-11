# T15 Lite skill-catalog migration follow-up — 2026-09-12

## Scope

This follow-up closes the Lite SQLite schema gap found during the T15
configuration smoke. The React configuration surface is unchanged. The
versioned SQLite migration now creates the four durable skill tables and
columns that the PostgreSQL migrations 000086–000090 already provide.

## Implementation

- Added SQLite migration `000014_tenant_skills` (up/down) for
  `tenant_skills`, `tenant_skill_snapshots`, `tenant_user_env_vars`, and
  `tenant_skill_catalog`.
- Added migration assertions for the version, required columns, CRUD paths,
  and active-skill name uniqueness in
  `internal/database/migration_sqlite_versioned_schema_test.go`.
- The migration is forward-compatible with both a fresh Lite database and an
  existing database upgraded from SQLite migration version 4.

## Verification

| Check | Result |
|---|---|
| `go test ./internal/database -count=1` | 0; fresh and v4-upgrade migration tests passed |
| `go test ./internal/application/service -run 'Skill|skill' -count=1` | 0 |
| `go build -tags sqlite_fts5 -o /tmp/weknora-lite-skill-migration ./cmd/server` | 0 |
| Fresh Lite startup | migration version `14`, `dirty=0`; health HTTP 200 |
| Authenticated `GET /api/v1/skills/catalog` | HTTP 200, `{"data":[],"success":true}` |
| Authenticated multipart `POST /api/v1/skills/catalog` | HTTP 201; registered a temporary valid ZIP bundle |
| Authenticated catalog/file reads | HTTP 200; catalog row and `SKILL.md`/script file entries returned |

The live run used a temporary local account, database, storage directory, and
skill archive. No production credentials or data were used. Before this
follow-up, the same endpoint returned HTTP 500 with
`no such table: tenant_skill_catalog`; after the rebuilt binary applied
version 14 it returned the expected success envelope and persisted the
catalog bundle.

## Boundary

This proves the Lite schema and catalog persistence boundary only. Sandbox
provider installation, asynchronous progress, removal/reaper behavior, and
production/provider acceptance remain open for T15/T24 review.
