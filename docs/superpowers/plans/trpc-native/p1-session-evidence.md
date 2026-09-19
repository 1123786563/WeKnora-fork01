# P1 Session SQL probe evidence

**Scope:** Track A only. The standalone module is
`tools/trpc-native-p1/sessionprobe`; no production Session adapter, migration,
root module, or SDK cache was modified.

## Fixed composition

`go.mod` directly requires the root and both SQL submodules at `v1.11.0`, with
no `replace` directive.

| Module | Zip sum | go.mod sum |
| --- | --- | --- |
| `trpc.group/trpc-go/trpc-agent-go v1.11.0` | `h1:LwMxQwT2l6hqWUVARfVA/ef2tq8gJCzbImSHurpaPIo=` | `h1:bIZcN4N9sGpA42sWfE98XCPl9ZgMi6fMDLGYOaXNe9A=` |
| `trpc.group/trpc-go/trpc-agent-go/session/sqlite v1.11.0` | `h1:LI5KVBWuG4JBt7advdO6osb+DmyqA9WvH8bxJ2E6uW8=` | `h1:CQOZ2lSwpwkYnN0hOWZLIOb4NMIhYZMd9n8gScfTnyw=` |
| `trpc.group/trpc-go/trpc-agent-go/session/postgres v1.11.0` | `h1:k6a6Sa9i2mjlLVZVmRnYwEFLdQeOsbIXb2aRFCVHxlg=` | `h1:f8iE70qCVVnXE/ECpMzSbtYwLcwE7oFGrXK37wit/M4=` |

Recorded command: `GOWORK=off go mod download -json <three exact modules>`;
exit 0. `GOWORK=off go list -m all` was attempted, but this host had
`GOPROXY=off` and then proxy TLS timeouts for uncached transitive module
metadata. It is therefore an environment-blocked full-graph listing, not a
module-composition pass. The direct pinned modules and their checksums above
were read from the local module cache.

## Measured commands

| Command | Exit | Actual result |
| --- | ---: | --- |
| `P1_SESSION_PG_DSN=... GOWORK=off go test -count=1 ./...` | 0 | SQLite and PostgreSQL subtests executed. Persistence/isolation/reopen/delete/list paging/Close/error characterization passed; opt-in contract was skipped. |
| `P1_SESSION_PG_DSN=... GOWORK=off go test -race -count=20 -timeout=5m ./...` | 0 | 20 repeated SQL probe executions completed in 14.145s; no race report. |
| `P1_SESSION_PG_DSN=... P1_REQUIRE_CONTRACT=1 GOWORK=off go test -run TestSessionContractReplayRejection -count=1 -v ./...` | 1 | Both SQLite and PostgreSQL returned nil for a changed-content replay under the same stable event ID. Required conflict rejection failed visibly. |

The DSN was supplied through the environment and is intentionally omitted from
this record.

## Capability results

| Check | SQLite | PostgreSQL | Classification |
| --- | --- | --- | --- |
| Create/get/reopen/delete tenant-key isolation | Passed in an isolated file | Passed in an isolated schema | PASS for SDK key-space persistence only |
| Session state update, list offset/limit, service-owned Close, then reopen | Passed | Passed | PASS for measured API behavior |
| GetSession event pagination | Returns `session.ErrEventPageUnsupported` | One-event page returned | SQLite unsupported; PostgreSQL PASS |
| Synchronous cancelled append exposes persistence error | Passed | Passed | PASS; async persistence explicitly disabled |
| Current Session object and reopened persisted events for same-ID replay | Both contain duplicate assistant events | Both contain duplicate assistant events | incompatible with P1 stable-ID dedupe contract |
| Changed content with same stable event ID | Accepted without conflict | Accepted without conflict | incompatible; red opt-in acceptance retained |
| Default PostgreSQL `NewService` schema initialization | `n/a` | Fails required-unique-index verification in a new schema | incompatible composition |

For PostgreSQL CRUD tests, the fixture creates the SDK source-defined tables
in a new schema and uses `WithSkipDBInit(true)`, because the default initializer
above is incompatible. This measures real service method behavior but does not
convert default schema initialization into PASS.

## Limits and required next boundary

These tests prove only database behavior under caller-provided key strings.
They do not prove authentication, authorization, tenant resolution, request
barriers, projection repair, cross-process commits, or production recovery.
P1 must add a durable event receipt keyed by stable event identity plus a
content hash and return a conflict on a changed replay. It must also resolve
the PostgreSQL schema-initialization defect or select a reviewed alternative.
