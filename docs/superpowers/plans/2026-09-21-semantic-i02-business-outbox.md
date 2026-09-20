# Semantica I02 Business Revisions and Outbox Implementation Plan

> **For agentic workers:** REQUIRED SKILL: Use superpowers:dispatching-parallel-agents for the independent I01/I02 workstreams; this task's files are owned only by the I02 worker. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the Go business-side semantic revision, transactional outbox, deletion deny barrier, and KB authorization epoch for both supported database migration streams.

**Architecture:** A new semantic control repository owns the outer GORM transaction and passes that exact `tx` to the business mutation callback; it atomically updates document semantic revision, serializes a durable outbox event, and for tombstone/restore writes a denial change plus KB epoch. It does not call current Knowledge/Chunk repository write methods because those methods own their own transactions. Actual application call sites remain for I05.

**Tech Stack:** Go/GORM, PostgreSQL numeric columns, SQLite text columns for exact uint64 persistence, golang-migrate file streams.

**Spec:** `docs/specs/2026-09-11-semantica-graphrag-reasoning-design.md` §§3, 5–6; ADR-0002; rebaseline; I02 in `docs/plans/2026-09-11-semantica-02-indexing.md`.

## Global Constraints

- Go is the unique authority for tenant, KB, document, business mutation, document semantic revision, ACL epoch, and immediate deletion denial.
- Business mutation callback, semantic revision, outbox event, denial barrier, and required epoch change use the exact same transaction; no hidden second transaction.
- Revision compare-and-set is scoped to `(tenant_id, kb_id, document_id)`; outbox delivery is at-least-once and consumers deduplicate by stable event ID.
- Outbox due/lease eligibility and lease expiry are anchored to database time, never the worker host clock. SQLite's conditional claim update re-evaluates due retry and absent/expired lease state after concurrent retry rescheduling.
- SQLite claim selection, conditional lease update, and full event re-read are one GORM transaction. After a successful claim, return the durable values for all event fields, including error code, retry time, owner, token, and lease expiry; never return stale pre-update metadata.
- The new PostgreSQL migration is `000178_semantic_control`; the paired SQLite migration is `000099_semantic_control`, based on verified existing heads PG 177 and SQLite 98. Re-check both heads immediately before creating files; never edit an applied migration.
- Persist `uint64` tenant/revision/epoch/outbox-attempt/fencing values without signed overflow: PostgreSQL uses bounded `NUMERIC(20,0)`; SQLite uses canonical base-10 TEXT with digit/range constraints and the repository converts with `strconv.FormatUint`/`ParseUint`.
- Existing `KnowledgeRepository`/`ChunkRepository` write methods cannot be called inside the callback because they start their own transactions. I02 exposes a tx-taking boundary; I05 performs the real business call-site integration.
- When a caller represents a persisted knowledge document, its semantic `DocumentID` maps to `types.Knowledge.ID`; I02 does not add a competing revision field to `Knowledge` or use per-chunk `ContentRevision` as a document revision.
- I02 creates only `semantic_document_revisions`, `semantic_outbox`, `semantic_access_epochs`, and `semantic_denials`. `semantic_backend_states` belongs to W03; `semantic_completion_receipts` belongs to I04; neither is frozen early here.
- PostgreSQL integration evidence must use a disposable database/schema and real PostgreSQL; no mock or skipped test is counted as verified. SQLite tests use a temporary DB and the formal migration runner.
- No Python service DB changes, operation-worker storage, model/provider, graph/vector storage, API/UI, or production backend promotion.

## Review Focus

- Callback fails after mutating a business row: rollback leaves no document revision, outbox event, deny barrier, or epoch change.
- Concurrent/stale expected revisions: exactly one CAS succeeds; revision never decreases or wraps uint64; simultaneous documents in one KB both update the shared authorization epoch successfully.
- Delete tombstone: same commit writes deny and increments epoch; restore requires the current tombstone revision, clears denial, and increments epoch.
- Outbox redelivery: retry presents the same stable event identity and payload hash; stale lease owner cannot acknowledge or overwrite a newer attempt.
- Lease timing/retry state uses database time; a SQLite race between expired-lease claim and a failure scheduling a future retry cannot replace the retry schedule.
- A stale SQLite claim candidate racing with a failure whose retry time is already due may claim the event, but must return the newly durable failure code and stable event/payload/config identity.
- PostgreSQL race tests use two independent SQL pools/connections on one isolated schema, prove distinct backend connection IDs, and rendezvous inside each transaction before contested writes.
- PG/SQLite equivalence: table constraints, indexes, up/down/up, payload bytes, and maximum uint64 strings remain exact across both dialects.

---

### Task 1: Implement dual-dialect semantic control and outbox repository

**Files:**
- Create: `migrations/versioned/000178_semantic_control.up.sql`
- Create: `migrations/versioned/000178_semantic_control.down.sql`
- Create: `migrations/sqlite/000099_semantic_control.up.sql`
- Create: `migrations/sqlite/000099_semantic_control.down.sql`
- Create: `internal/types/semantic_control.go`
- Create: `internal/application/repository/semantic_outbox.go`
- Create: `internal/application/repository/semantic_outbox_test.go`
- Create: `internal/database/semantic_migration_test.go`

**Interfaces:**
- `types.SemanticMutation` contains `TenantID uint64`, `KBID string`, `DocumentID string`, `ExpectedRevision uint64`, Go-authoritative opaque `ContentHash string`, nonempty Go-authoritative `ConfigDigest string`, `Deleted bool`, and immutable `Payload []byte` supplied by the caller. For active revisions, payload is the serialized C01 apply request; a delete event uses the tombstone metadata and may have an empty payload.
- `SemanticControlRepository.WithSemanticMutation(ctx, mutation, func(tx *gorm.DB) error) (uint64, error)` requires nonzero tenant, nonempty KB/document/content hash/config digest, nonnil callback, nonempty active payload (delete payload may be empty), creates/locks the scoped revision row, requires stored revision to equal `ExpectedRevision`, increments without uint64 overflow, calls the business callback with that same `tx`, stores Go's supplied content hash and config digest unchanged, inserts one semantic outbox event with stable UUID and `sha256:<hex>` hash of the exact payload bytes, and returns the new revision. Any callback/SQL error rolls back every write.
- First document mutation expects revision 0 and creates revision 1. A later mutation must provide the exact current revision. Deletion writes/advances a scoped deny row and bumps KB epoch in the same transaction. A non-delete mutation whose expected revision is the current tombstone explicitly restores that document, clears its deny row, and bumps KB epoch.
- `BumpSemanticEpoch(tx *gorm.DB, scope types.SemanticScopeKey) (uint64, error)` is callable by A01 inside its already-open transaction; it never starts a nested transaction. It conflict-safely creates epoch zero, locks/re-reads, and retries conditional increments so concurrent first inserts or concurrent document tombstones in one KB do not spuriously roll back.
- `types.SemanticOutboxEvent` carries stable `EventID`, scope, document, revision, content hash, config digest, tombstone flag, payload bytes, SHA-256 payload hash, attempt count, lease owner, lease token, lease expiry, retry time, and last error code; the payload hash is `sha256:` plus lowercase hex of exact stored bytes.
- `ClaimSemanticOutbox(ctx, workerID, leaseSeconds) (*types.SemanticOutboxEvent, error)`, `AckSemanticOutbox(ctx,eventID,workerID,leaseToken) error`, and `FailSemanticOutbox(ctx,eventID,workerID,leaseToken,retryAt,errorCode) error` use owner+fence checks; failure persists its nonempty code and retry time and clears owner/lease so the event can be reclaimed. Claims are due only when `retry_at <= database time` and the prior lease is absent/expired. PostgreSQL uses `FOR UPDATE SKIP LOCKED`; SQLite uses transaction + conditional update. Every string→uint64 conversion failure returns an error; no malformed/out-of-range durable ID may become zero.
- `FailSemanticOutbox` rejects an empty error code before SQL without mutating the event; the empty migration default is only for events not yet failed.
- Migration rows are private GORM models with canonical decimal-string IDs in queries; do not let GORM or SQLite coerce uint64 values through float64.

- [ ] **Step 1: Write failing SQLite transaction/revision/outbox tests**

```go
func TestSemanticMutationRollback(t *testing.T) {
    ctx := context.Background()
    db := newSemanticSQLiteTestDB(t) // uses t.TempDir and the formal migration runner
    require.NoError(t, db.Exec("CREATE TABLE semantic_test_business (id TEXT PRIMARY KEY)").Error)
    repo := NewSemanticControlRepository(db)
    _, err := repo.WithSemanticMutation(ctx, mutationFixture(0, true, nil), func(tx *gorm.DB) error {
        if err := tx.Exec("INSERT INTO semantic_test_business(id) VALUES (?)", "resource-1").Error; err != nil {
            return err
        }
        return errors.New("abort business mutation")
    })
    require.Error(t, err)
    assertRowCount(t, db, "semantic_test_business", 0)
    assertRowCount(t, db, "semantic_document_revisions", 0)
    assertRowCount(t, db, "semantic_outbox", 0)
    assertRowCount(t, db, "semantic_denials", 0)
    assertRowCount(t, db, "semantic_access_epochs", 0)
}

func TestSemanticMutationCommitsBusinessRevisionAndOutboxTogether(t *testing.T) {
    ctx := context.Background()
    db := newSemanticSQLiteTestDB(t)
    require.NoError(t, db.Exec("CREATE TABLE semantic_test_business (id TEXT PRIMARY KEY)").Error)
    repo := NewSemanticControlRepository(db)
    revision, err := repo.WithSemanticMutation(ctx, mutationFixture(0, false, []byte("apply-1")), func(tx *gorm.DB) error {
        return tx.Exec("INSERT INTO semantic_test_business(id) VALUES (?)", "resource-1").Error
    })
    require.NoError(t, err)
    require.Equal(t, uint64(1), revision)
    assertRowCount(t, db, "semantic_test_business", 1)
    assertRowCount(t, db, "semantic_document_revisions", 1)
    assertRowCount(t, db, "semantic_outbox", 1)
}

func TestSemanticMigrationSQLiteUpDownUp(t *testing.T) {
    migration, db := newSemanticSQLiteMigrator(t) // t.TempDir DB; absolute file:// migration path
    require.NoError(t, migration.Up())
    assertSQLiteSemanticTables(t, db, "semantic_document_revisions", "semantic_outbox", "semantic_access_epochs", "semantic_denials")
    require.NoError(t, migration.Steps(-1))
    require.False(t, sqliteTableExists(t, db, "semantic_outbox"))
    require.NoError(t, migration.Up())
    assertSQLiteSemanticTables(t, db, "semantic_document_revisions", "semantic_outbox", "semantic_access_epochs", "semantic_denials")
}

func TestSemanticMutationCASDeleteAndRestore(t *testing.T) {
    ctx := context.Background()
    db := newSemanticSQLiteTestDB(t)
    repo := NewSemanticControlRepository(db)
    firstRevision, err := repo.WithSemanticMutation(ctx, mutationFixture(0, false, []byte("apply-1")), noBusinessWrite)
    require.NoError(t, err)
    require.Equal(t, uint64(1), firstRevision)
    deletedRevision, err := repo.WithSemanticMutation(ctx, mutationFixture(1, true, nil), noBusinessWrite)
    require.NoError(t, err)
    require.Equal(t, uint64(2), deletedRevision)
    assertDenyRevision(t, db, 1, "kb-1", "doc-1", 2)
    assertSemanticEpoch(t, db, 1, "kb-1", 1)
    restoredRevision, err := repo.WithSemanticMutation(ctx, mutationFixture(2, false, []byte("apply-3")), noBusinessWrite)
    require.NoError(t, err)
    require.Equal(t, uint64(3), restoredRevision)
    assertNoDeny(t, db, 1, "kb-1", "doc-1")
    assertSemanticEpoch(t, db, 1, "kb-1", 2)
    _, err = repo.WithSemanticMutation(ctx, mutationFixture(1, false, []byte("stale")), noBusinessWrite)
    require.ErrorIs(t, err, ErrSemanticRevisionConflict)
}

func TestSemanticPayloadHashAndOutboxLeaseFence(t *testing.T) {
    ctx := context.Background()
    db := newSemanticSQLiteTestDB(t)
    repo := NewSemanticControlRepository(db)
    _, err := repo.WithSemanticMutation(ctx, mutationFixture(0, false, []byte("apply-payload")), noBusinessWrite)
    require.NoError(t, err)
    event, err := repo.ClaimSemanticOutbox(ctx, "worker-a", 30)
    require.NoError(t, err)
    require.Equal(t, sha256Hex([]byte("apply-payload")), event.PayloadHash)
    require.Equal(t, "config-v1", event.ConfigDigest)
    second, err := repo.ClaimSemanticOutbox(ctx, "worker-b", 30)
    require.NoError(t, err) // no second unexpired delivery claim
    require.Nil(t, second)
    require.ErrorIs(t, repo.AckSemanticOutbox(ctx, event.EventID, "worker-a", event.LeaseToken-1), ErrSemanticOutboxLeaseLost)
    require.ErrorIs(t, repo.AckSemanticOutbox(ctx, event.EventID, "wrong-worker", event.LeaseToken), ErrSemanticOutboxLeaseLost)
    require.NoError(t, repo.AckSemanticOutbox(ctx, event.EventID, "worker-a", event.LeaseToken))
}

func TestOutboxFailurePreservesEventConfigHashAndErrorForRetry(t *testing.T) {
    ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
    repo := NewSemanticControlRepository(db)
    mutation := mutationFixture(0, false, []byte("serialized-apply"))
    mutation.ConfigDigest = "config-v1"
    _, err := repo.WithSemanticMutation(ctx, mutation, noBusinessWrite)
    require.NoError(t, err)
    first, err := repo.ClaimSemanticOutbox(ctx, "worker-a", 30)
    require.NoError(t, err)
    require.NoError(t, repo.FailSemanticOutbox(ctx, first.EventID, "worker-a", first.LeaseToken, time.Now().Add(-time.Second), "semantic-unavailable"))
    second, err := repo.ClaimSemanticOutbox(ctx, "worker-b", 30)
    require.NoError(t, err)
    require.Equal(t, first.EventID, second.EventID)
    require.Equal(t, first.PayloadHash, second.PayloadHash)
    require.Equal(t, "config-v1", second.ConfigDigest)
    require.Equal(t, "semantic-unavailable", second.ErrorCode)
    require.ErrorIs(t, repo.AckSemanticOutbox(ctx, first.EventID, "worker-a", first.LeaseToken), ErrSemanticOutboxLeaseLost)
}

func TestOutboxFailureRejectsEmptyErrorCodeWithoutChangingLease(t *testing.T) {
    ctx, db := context.Background(), newSemanticSQLiteTestDB(t)
    repo := NewSemanticControlRepository(db)
    _, err := repo.WithSemanticMutation(ctx, mutationFixture(0, false, []byte("payload")), noBusinessWrite)
    require.NoError(t, err)
    event, err := repo.ClaimSemanticOutbox(ctx, "worker-a", 30)
    require.NoError(t, err)
    require.ErrorIs(t, repo.FailSemanticOutbox(ctx, event.EventID, "worker-a", event.LeaseToken, time.Now(), ""), ErrSemanticMutationInvalid)
    require.NoError(t, repo.AckSemanticOutbox(ctx, event.EventID, "worker-a", event.LeaseToken))
}

func TestSemanticMutationPreservesMaximumUint64(t *testing.T) {
    ctx := context.Background()
    db := newSemanticSQLiteTestDB(t)
    repo := NewSemanticControlRepository(db)
    maximum := uint64(math.MaxUint64)
    scope := types.SemanticScopeKey{TenantID: maximum, KBID: "kb-1"}
    mutation := mutationFixture(maximum-1, false, []byte("final"))
    mutation.TenantID = maximum
    _, err := repo.WithSemanticMutation(ctx, mutation, noBusinessWrite)
    require.NoError(t, err)
    revision := loadSemanticRevisionAsUint64(t, db, scope, "doc-1")
    require.Equal(t, maximum, revision)
    event, err := repo.ClaimSemanticOutbox(ctx, "uint64-worker", 30)
    require.NoError(t, err)
    require.Equal(t, maximum, event.Scope.TenantID)
    require.Equal(t, maximum, event.Revision)
    require.Equal(t, uint64(1), event.AttemptCount)
    require.Equal(t, uint64(1), event.LeaseToken)
    overflow := mutationFixture(maximum, false, []byte("overflow"))
    overflow.TenantID = maximum
    _, err = repo.WithSemanticMutation(ctx, overflow, noBusinessWrite)
    require.ErrorIs(t, err, ErrSemanticRevisionOverflow)

    require.NoError(t, db.Exec("INSERT INTO semantic_access_epochs(tenant_id, kb_id, epoch) VALUES (?, ?, ?)",
        strconv.FormatUint(maximum, 10), "kb-1", strconv.FormatUint(maximum-1, 10)).Error)
    var epoch uint64
    err = db.Transaction(func(tx *gorm.DB) error {
        var bumpErr error
        epoch, bumpErr = repo.BumpSemanticEpoch(tx, scope)
        return bumpErr
    }).Error
    require.NoError(t, err)
    require.Equal(t, maximum, epoch)
    err = db.Transaction(func(tx *gorm.DB) error {
        _, bumpErr := repo.BumpSemanticEpoch(tx, scope)
        return bumpErr
    }).Error
    require.ErrorIs(t, err, ErrSemanticEpochOverflow)
}

func TestSemanticMutationPostgresConcurrentExpectedRevisionHasOneWinner(t *testing.T) {
    db1, db2 := newSemanticPostgresTestDBPair(t) // separate connections, same disposable schema
    repo1, repo2 := NewSemanticControlRepository(db1), NewSemanticControlRepository(db2)
    start := make(chan struct{})
    results := make(chan error, 2)
    for _, repo := range []*SemanticControlRepository{repo1, repo2} {
        go func(repo *SemanticControlRepository) {
            <-start
            _, err := repo.WithSemanticMutation(context.Background(), mutationFixture(0, false, []byte("same-revision")), noBusinessWrite)
            results <- err
        }(repo)
    }
    close(start)
    first, second := <-results, <-results
    require.NotEqual(t, first == nil, second == nil) // exactly one commit wins
    assertPostgresOutboxCount(t, db1, fixtureScope, "doc-1", 1)
}
```

The test imports `context`, `crypto/sha256`, `encoding/hex`, `errors`, `math`, `strconv`, `testing`, `time`, GORM, and `require/assert`. Define `fixtureScope = types.SemanticScopeKey{TenantID: 1, KBID: "kb-1"}`, `noBusinessWrite = func(*gorm.DB) error { return nil }`, and `mutationFixture(expected uint64, deleted bool, payload []byte) types.SemanticMutation` which fills tenant 1, KB `kb-1`, document `doc-1`, content hash `opaque-content-hash`, config digest `config-v1`, and copies payload. `sha256Hex` returns `"sha256:" + hex.EncodeToString(sum[:])` for a local `sha256.Sum256(payload)` variable; `assertRowCount`, `assertDenyRevision`, `assertSemanticEpoch`, `assertNoDeny`, `loadSemanticRevisionAsUint64`, `assertSQLiteSemanticTables`, and `sqliteTableExists` query the real migrated DB. `newSemanticSQLiteTestDB`/`newSemanticSQLiteMigrator` create a `t.TempDir()` DB, resolve the repo root from `runtime.Caller`, and use golang-migrate with an absolute `file://<repoRoot>/migrations/sqlite` source. PostgreSQL migration/outbox integration tests live in `*_pg_test.go` files built with `-tags=semantic_integration`; they require `TRPC_TEST_POSTGRES_DSN` and fail if absent, use a UUID schema/search_path, apply the formal migration stream, verify all I02 tables/indexes before and after down/up, and drop only that schema. The race fixture opens two independent SQL pools/connections on the same schema, verifies backend PIDs differ, and rendezvous barriers in transaction callbacks. The tests cover maximum uint64 tenant/revision/epoch/outbox attempts/fences, reject increments beyond max, reject corrupt stored identifiers, and reject empty failure codes.

```go
func sha256Hex(payload []byte) string {
    sum := sha256.Sum256(payload)
    return "sha256:" + hex.EncodeToString(sum[:])
}

func TestSemanticMutationPostgresConcurrentExpectedRevisionHasOneWinner(t *testing.T) {
    db1, db2 := newSemanticPostgresTestDBPair(t) // separate DB connections, same test schema
    repo1, repo2 := NewSemanticControlRepository(db1), NewSemanticControlRepository(db2)
    start := make(chan struct{})
    results := make(chan error, 2)
    for _, repo := range []*SemanticControlRepository{repo1, repo2} {
        go func(repo *SemanticControlRepository) {
            <-start
            mutation := mutationFixture(0, false, []byte("same-revision"))
            results <- func() error { _, err := repo.WithSemanticMutation(context.Background(), mutation, noBusinessWrite); return err }()
        }(repo)
    }
    close(start)
    first, second := <-results, <-results
    require.NotEqual(t, first == nil, second == nil)
    assertPostgresOutboxCount(t, db1, fixtureScope, "doc-1", 1)
}

func TestSemanticMutationPostgresConcurrentDifferentDocumentDenialsBothCommit(t *testing.T) {
    db1, db2 := newSemanticPostgresTestDBPair(t)
    repo1, repo2 := NewSemanticControlRepository(db1), NewSemanticControlRepository(db2)
    first, second := mutationFixture(0, true, nil), mutationFixture(0, true, nil)
    first.DocumentID, second.DocumentID = "doc-a", "doc-b"
    start := make(chan struct{})
    results := make(chan error, 2)
    for _, pair := range []struct{ repo *SemanticControlRepository; mutation types.SemanticMutation }{{repo1, first}, {repo2, second}} {
        go func(repo *SemanticControlRepository, mutation types.SemanticMutation) {
            <-start
            _, err := repo.WithSemanticMutation(context.Background(), mutation, noBusinessWrite)
            results <- err
        }(pair.repo, pair.mutation)
    }
    close(start)
    require.NoError(t, <-results)
    require.NoError(t, <-results)
    assertPostgresSemanticEpoch(t, db1, fixtureScope, 2)
}
```

- [ ] **Step 2: Run RED**

Run: `go test ./internal/application/repository ./internal/database -run 'TestSemanticMutation|TestSemanticMigration' -count=1`

Expected: fail because I02 types, repository, migrations, and tests do not exist.

- [ ] **Step 3: Write migration-head and migration-up/down/up tests first**

In `internal/database/semantic_migration_test.go`, add `TestSemanticMigrationSQLiteUpDownUp`: migrate a fresh temp DB to the current head, assert head 99 and all four owned tables/indexes, migrate down one migration, assert all four I02 tables/indexes are absent, then migrate up and reassert head 99, clean state, and all four tables/indexes. In `internal/database/semantic_migration_pg_test.go` under `//go:build semantic_integration`, add the same PG test using a disposable DB plus UUID schema and formal migration stream. The PG test fails if `TRPC_TEST_POSTGRES_DSN` is absent; it is not skipped.

Run: `go test ./internal/database -run '^TestSemanticMigrationSQLiteUpDownUp$' -count=1`

Expected: FAIL because migration 99 does not yet create semantic control tables and the up/down/up contract fails.

- [ ] **Step 4: Implement migrations and private exact-uint64 rows**

PostgreSQL uses `NUMERIC(20,0)` plus canonical nonnegative decimal checks; SQLite stores canonical decimal TEXT. The only I02 tables are document revisions, outbox, access epochs, and denials. Add down scripts that remove only these tables in reverse dependency order.

- [ ] **Step 5: Implement transaction callback, denial/epoch and fenced outbox methods**

Use a conditional revision row insert for first revision, then `SELECT ... FOR UPDATE` on PostgreSQL and revision conditional update on both dialects. Insert the caller's business row through the supplied tx before commit. Compute event payload hash once, store immutable bytes, and require matching lease token for ack/failure.

- [ ] **Step 6: Run GREEN on SQLite and isolated PostgreSQL**

Run: `go test ./internal/application/repository -run '^TestSemanticMutation' -count=1` and `go test ./internal/database -run '^TestSemanticMigrationSQLiteUpDownUp$' -count=1`.

Expected: SQLite tests exercise formal up/down/up and rollback. Also run `go test -tags=semantic_integration ./internal/application/repository ./internal/database -run 'TestSemantic(Postgres|MutationPostgres)' -count=1` with `TRPC_TEST_POSTGRES_DSN` set to a disposable test database; the PG tests fail if it is missing and must prove isolation/up/down/up/transaction rollback with real independent connections. Never use the running WeKnora development database or a user's application DB.

- [ ] **Step 7: Commit Task I02**

Commit `feat(semantic): i02 business revisions and outbox` with only I02 Go types, migrations, repository, and tests.

---

## Final I02 Gate

- [ ] Request an independent read-only review of the complete I02 range; fix Critical/Important findings with RED→GREEN behavior tests.
- [ ] Record the actual PostgreSQL and SQLite evidence separately; no production KB mutation, permission-service integration, or live outbox delivery claim.
