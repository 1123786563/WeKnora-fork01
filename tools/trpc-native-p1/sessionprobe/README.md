# Native P1 Session SQL probe

This standalone module tests the exact `v1.11.0` root, SQLite Session, and
PostgreSQL Session modules. It is evidence only: it does not wire a Session
service into WeKnora and has no `replace` directive.

Run the normal characterization suite with a PostgreSQL DSN supplied out of
band (do not print it):

```sh
cd tools/trpc-native-p1/sessionprobe
P1_SESSION_PG_DSN="$P1_SESSION_PG_DSN" GOWORK=off go test -run TestSession -count=1 -v ./...
P1_SESSION_PG_DSN="$P1_SESSION_PG_DSN" GOWORK=off go test -race -count=20 -timeout=5m ./...
```

The fixture creates a unique SQLite file or PostgreSQL schema per subtest.
The PostgreSQL service owns and closes its own connection; the fixture keeps a
separate admin connection only to create/drop the isolated schema. The service
is reopened against the same durable database after each persistence check.
`WithEnableAsyncPersist(false)` is always explicit.

`P1_REQUIRE_CONTRACT=1` enables a deliberately red acceptance suite:

```sh
P1_SESSION_PG_DSN="$P1_SESSION_PG_DSN" P1_REQUIRE_CONTRACT=1 \
  GOWORK=off go test -run TestSessionContractReplayRejection -count=1 -v ./...
```

It requires a changed replay under a stable `event.Event.ID` to return an error and to
leave one persisted assistant event. Native v1.11.0 accepts the changed replay
for both SQL backends, so this command must exit nonzero until an application
dedupe/conflict barrier is added.

The probe creates valid response events with `event.NewResponseEvent`, a
non-partial, done `model.Response`, `chat.completion` object, and an assistant
choice. It explicitly assigns `Event.ID = "event-1"`; the changed replay is a
clone with that Event ID and its timestamp restored, and only the assistant
response content changes. A bootstrap user event is included solely because
the SDK's session read filtering removes assistant-only histories.

List characterization verifies updated-at descending IDs, bounded and
out-of-range pages, invalid page bounds, merged app/user/session state, and
that `WithListSessionOnlyMeta` removes history even when it was persisted. A
cancelled synchronous assistant append returns an error after updating the
caller Session object; after reopen, only the bootstrap event remains in the
database.

The PostgreSQL default schema initializer is independently characterized as
incompatible: it rejects an otherwise empty unique schema during required
unique-index verification. To measure actual CRUD and
append behavior, the fixture builds the source-defined tables in its private
schema and uses `WithSkipDBInit(true)`. This is not approval of the default
PostgreSQL composition.
