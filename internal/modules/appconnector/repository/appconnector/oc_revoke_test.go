package appconnector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// T08: durable revocation, re-bind-aware activation, orphan cleanup
// ---------------------------------------------------------------------------

// ocRevokeSubject is the canonical revocation subject for these tests.
func ocRevokeSubject(tenant uint64) appconnector.OCSubject {
	return appconnector.OCSubject{TenantID: tenant, ActorID: "alice"}
}

// ocSeedActiveBinding seeds an ACTIVE binding for a tenant connection.
func ocSeedActiveBinding(t *testing.T, store *OCStore, tenant uint64, connID, externalID, alias string, authVersion int64) {
	t.Helper()
	b := appconnector.OCBinding{
		TenantID: tenant, ConnectionID: connID, RuntimeID: "rt-1", Provider: "github",
		ExternalID: externalID, Alias: alias,
		AuthVersion: authVersion, BindingVersion: 1, State: appconnector.OCBindingActive,
	}
	if err := store.SaveBinding(context.Background(), b); err != nil {
		t.Fatal(err)
	}
}

func ocSeedVerifyingAttempt(t *testing.T, store *OCStore, tenant uint64, connID, alias string, authVersion int64, now time.Time) appconnector.OCAuthorizationAttempt {
	t.Helper()
	ctx := context.Background()
	a := appconnector.OCAuthorizationAttempt{
		ID:           uuid.NewString(),
		Subject:      ocRevokeSubject(tenant),
		ConnectionID: connID, RuntimeID: "rt-1", Provider: "github", Alias: alias,
		State: "pending", AuthVersion: authVersion, ExpiresAt: now.Add(15 * time.Minute).UTC(),
	}
	if err := store.SaveOCAttempt(ctx, a); err != nil {
		t.Fatal(err)
	}
	if ok, err := store.AdvanceOCAttempt(ctx, tenant, a.ID, "pending", "authorizing", now); err != nil || !ok {
		t.Fatalf("advance to authorizing: ok=%v err=%v", ok, err)
	}
	if ok, err := store.AdvanceOCAttempt(ctx, tenant, a.ID, "authorizing", "verifying", now); err != nil || !ok {
		t.Fatalf("advance to verifying: ok=%v err=%v", ok, err)
	}
	a.State = "verifying"
	return a
}

func ocConnectionVersion(t *testing.T, db *gorm.DB, tenant uint64, connID string) (string, int64) {
	t.Helper()
	var row ConnectionRow
	if err := db.Where("tenant_id = ? AND id = ?", tenant, connID).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row.State, row.AuthVersion
}

func ocOutboxRowsFor(t *testing.T, db *gorm.DB, tenant uint64, connID, kind string) []OCOperationsOutboxRow {
	t.Helper()
	var rows []OCOperationsOutboxRow
	if err := db.Where("tenant_id = ? AND resource_id = ? AND kind = ?", tenant, connID, kind).Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	return rows
}

// TestOCRevokeRevokesConnectionBindingAndEnqueuesDurableCleanup is the core
// acceptance (plan checkbox 1): revoking an activated connection flips the
// connection and its binding to revoked, bumps the authorization version
// exactly once, and enqueues exactly ONE durable remote-cleanup outbox row in
// the SAME transaction.
func TestOCRevokeRevokesConnectionBindingAndEnqueuesDurableCleanup(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	if err := db.Exec("UPDATE connections SET auth_version = 2 WHERE tenant_id = ? AND id = ?", 7, "conn1").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE connector_connection_bindings SET auth_version = 2 WHERE tenant_id = ? AND connection_id = ?", 7, "conn1").Error; err != nil {
		t.Fatal(err)
	}

	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 2); err != nil {
		t.Fatalf("revoke: %v", err)
	}

	state, version := ocConnectionVersion(t, db, 7, "conn1")
	if state != "revoked" || version != 3 {
		t.Fatalf("connection after revoke = %s@%d, want revoked@3", state, version)
	}
	b, err := store.GetBinding(ctx, 7, "conn1")
	if err != nil || b.State != appconnector.OCBindingRevoked || b.AuthVersion != 3 {
		t.Fatalf("binding after revoke = %+v err=%v, want revoked@3", b, err)
	}
	if b.BindingVersion != 2 {
		t.Fatalf("binding_version after revoke = %d, want 2 (optimistic counter moved)", b.BindingVersion)
	}
	rows := ocOutboxRowsFor(t, db, 7, "conn1", "delete_connection")
	if len(rows) != 1 {
		t.Fatalf("durable cleanup rows = %d, want exactly 1", len(rows))
	}
	if rows[0].ResourceVersion != 3 {
		t.Fatalf("cleanup row version = %d, want the NEW authorization version 3", rows[0].ResourceVersion)
	}
}

// TestOCRevokeDuplicateIsIdempotentWithoutSecondBump: a replayed revoke with
// the caller's stale expectedVersion must succeed WITHOUT bumping the version
// again and WITHOUT enqueuing a second cleanup row.
func TestOCRevokeDuplicateIsIdempotentWithoutSecondBump(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)

	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatalf("first revoke: %v", err)
	}
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatalf("duplicate revoke with stale view: %v", err)
	}

	state, version := ocConnectionVersion(t, db, 7, "conn1")
	if state != "revoked" || version != 2 {
		t.Fatalf("connection after duplicate revoke = %s@%d, want revoked@2 (no second bump)", state, version)
	}
	if rows := ocOutboxRowsFor(t, db, 7, "conn1", "delete_connection"); len(rows) != 1 {
		t.Fatalf("cleanup rows after duplicate revoke = %d, want 1", len(rows))
	}
}

// TestOCRevokeStaleVersionOnLiveConnectionConflicts: a revoke whose
// expectedVersion does not match a LIVE connection is a conflict; nothing is
// written and no cleanup row appears.
func TestOCRevokeStaleVersionOnLiveConnectionConflicts(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 2)
	if err := db.Exec("UPDATE connections SET auth_version = 2 WHERE tenant_id = ? AND id = ?", 7, "conn1").Error; err != nil {
		t.Fatal(err)
	}

	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); !errors.Is(err, ErrOCRevokeConflict) {
		t.Fatalf("stale revoke err = %v, want ErrOCRevokeConflict", err)
	}
	state, version := ocConnectionVersion(t, db, 7, "conn1")
	if state != "active" || version != 2 {
		t.Fatalf("stale revoke changed the connection: %s@%d", state, version)
	}
	if rows := ocOutboxRowsFor(t, db, 7, "conn1", "delete_connection"); len(rows) != 0 {
		t.Fatalf("stale revoke enqueued %d cleanup rows", len(rows))
	}
}

// TestOCRevokeRejectsMalformedInput: unscoped input fails before any database
// access.
func TestOCRevokeRejectsMalformedInput(t *testing.T) {
	store, _ := testOCStore(t)
	ctx := context.Background()
	if err := store.RevokeOCConnection(ctx, appconnector.OCSubject{}, "conn1", 1); !errors.Is(err, ErrOCRevokeInvalid) {
		t.Fatalf("zero subject err = %v, want ErrOCRevokeInvalid", err)
	}
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "", 1); !errors.Is(err, ErrOCRevokeInvalid) {
		t.Fatalf("blank connection err = %v, want ErrOCRevokeInvalid", err)
	}
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 0); !errors.Is(err, ErrOCRevokeInvalid) {
		t.Fatalf("zero version err = %v, want ErrOCRevokeInvalid", err)
	}
}

// TestOCRevokeMissingConnectionFailsClosed: revoking an unknown (or another
// tenant's) connection surfaces record-not-found, never a silent success.
func TestOCRevokeMissingConnectionFailsClosed(t *testing.T) {
	store, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "conn1")
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(8), "conn1", 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("cross-tenant revoke err = %v, want record-not-found", err)
	}
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "nope", 1); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing revoke err = %v, want record-not-found", err)
	}
}

// TestOCRevokeNativeConnectionWithoutBindingStillRevokes: a connection with
// no OC binding row revokes normally (the connection row is the authority)
// and still enqueues its durable cleanup row.
func TestOCRevokeNativeConnectionWithoutBindingStillRevokes(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "conn1")

	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatalf("native revoke: %v", err)
	}
	state, version := ocConnectionVersion(t, db, 7, "conn1")
	if state != "revoked" || version != 2 {
		t.Fatalf("native connection after revoke = %s@%d, want revoked@2", state, version)
	}
	if rows := ocOutboxRowsFor(t, db, 7, "conn1", "delete_connection"); len(rows) != 1 {
		t.Fatalf("native revoke cleanup rows = %d, want 1", len(rows))
	}
}

// TestOCRevokeBlocksPreClaimActivationAfterwards (matrix: revoke/claim
// interleave, serialized shape): once the revocation transaction committed,
// an in-flight authorization attempt from the previous generation can no
// longer activate — the claim-side freshness check fails closed.
func TestOCRevokeBlocksPreClaimActivationAfterwards(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	att := ocSeedVerifyingAttempt(t, store, 7, "conn1", "alias-9", 1, now)

	// "Claim" wins first: the dispatcher holds generation 1. Then revoke bumps
	// the generation to 2.
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatal(err)
	}
	if err := store.ActivateOCAttempt(ctx, 7, att.ID, "ext-99", now); !errors.Is(err, ErrOCAttemptConflict) {
		t.Fatalf("post-revoke activation err = %v, want ErrOCAttemptConflict", err)
	}
	if _, version := ocConnectionVersion(t, db, 7, "conn1"); version != 2 {
		t.Fatalf("version = %d, want 2", version)
	}
}

// ---------------------------------------------------------------------------
// ruling 1b: re-bind-aware activation
// ---------------------------------------------------------------------------

// TestOCActivateRebindIdempotentOnSameIdentity: when the connection already
// holds an ACTIVE binding with the SAME external id at the SAME generation,
// activation is an idempotent consume — the attempt activates, the binding is
// NOT rewritten (no PK collision, no blind insert).
func TestOCActivateRebindIdempotentOnSameIdentity(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	att := ocSeedVerifyingAttempt(t, store, 7, "conn1", "alias-1", 1, now)

	if err := store.ActivateOCAttemptRebind(ctx, 7, att.ID, "ext-42", now); err != nil {
		t.Fatalf("idempotent rebind activation: %v", err)
	}
	got, err := store.GetOCAttempt(ctx, 7, att.ID)
	if err != nil || got.State != "active" {
		t.Fatalf("attempt after idempotent activation = %+v err=%v, want active", got, err)
	}
	var n int64
	db.Table("connector_connection_bindings").Where("tenant_id = ?", 7).Count(&n)
	if n != 1 {
		t.Fatalf("binding rows = %d, want exactly 1 (no duplicate insert)", n)
	}
	b, _ := store.GetBinding(ctx, 7, "conn1")
	if b.BindingVersion != 1 {
		t.Fatalf("idempotent replay must not rewrite the binding (binding_version = %d)", b.BindingVersion)
	}
}

// TestOCActivateRebindConflictsOnDifferentIdentity: a binding already exists
// with a DIFFERENT external id (or in a terminal state): the activation is a
// CLEAR conflict — never a blind PK insert that the worker would retry
// against forever (T07 spec F-02).
func TestOCActivateRebindConflictsOnDifferentIdentity(t *testing.T) {
	store, inst := testOCStore(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	att := ocSeedVerifyingAttempt(t, store, 7, "conn1", "alias-9", 1, now)

	if err := store.ActivateOCAttemptRebind(ctx, 7, att.ID, "ext-OTHER", now); !errors.Is(err, ErrOCAttemptConflict) {
		t.Fatalf("rebind with different external id err = %v, want ErrOCAttemptConflict", err)
	}
	got, _ := store.GetOCAttempt(ctx, 7, att.ID)
	if got.State != "verifying" {
		t.Fatalf("conflicting rebind must not consume the attempt, state = %s", got.State)
	}
	b, _ := store.GetBinding(ctx, 7, "conn1")
	if b.ExternalID != "ext-42" || b.State != appconnector.OCBindingActive {
		t.Fatalf("existing binding must stay untouched: %+v", b)
	}
}

// TestOCActivateRebindConflictsOnRevokedBinding: after a revocation, a fresh
// re-authorization (new alias, new external connection) against the SAME
// connection row is a clear conflict — the binding identity of a connection
// is frozen (T03); replacing an account means a new connection.
func TestOCActivateRebindConflictsOnRevokedBinding(t *testing.T) {
	store, inst := testOCStore(t)
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatal(err)
	}

	att := ocSeedVerifyingAttempt(t, store, 7, "conn1", "alias-new", 2, now)
	if err := store.ActivateOCAttemptRebind(ctx, 7, att.ID, "ext-NEW", now); !errors.Is(err, ErrOCAttemptConflict) {
		t.Fatalf("rebind after revoke err = %v, want ErrOCAttemptConflict", err)
	}
	b, _ := store.GetBinding(ctx, 7, "conn1")
	if b.State != appconnector.OCBindingRevoked || b.ExternalID != "ext-42" {
		t.Fatalf("revoked binding must stay frozen: %+v", b)
	}
}

// TestOCActivateRebindFreshConnectionMatchesLegacyActivation: with no
// existing binding row the rebind-aware activation behaves exactly like the
// frozen ActivateOCAttempt (additive change, same outcome).
func TestOCActivateRebindFreshConnectionMatchesLegacyActivation(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	ocSeedConnection(t, inst, 7, "conn1")
	att := ocSeedVerifyingAttempt(t, store, 7, "conn1", "alias-1", 1, now)

	if err := store.ActivateOCAttemptRebind(ctx, 7, att.ID, "ext-42", now); err != nil {
		t.Fatalf("fresh rebind activation: %v", err)
	}
	b, err := store.GetBinding(ctx, 7, "conn1")
	if err != nil || b.State != appconnector.OCBindingActive || b.ExternalID != "ext-42" || b.Alias != "alias-1" || b.BindingVersion != 1 {
		t.Fatalf("fresh activation binding = %+v err=%v", b, err)
	}
	var n int64
	db.Table("connector_connection_bindings").Where("tenant_id = ?", 7).Count(&n)
	if n != 1 {
		t.Fatalf("binding rows = %d, want 1", n)
	}
}

// ---------------------------------------------------------------------------
// ruling 1a: extended cleanup triple for adopted-alias orphans
// ---------------------------------------------------------------------------

// TestOCEnqueueOrphanCleanupSchedulesDeleteForUnattributedExternal: when a
// submitted external connection cannot be attributed to the attempt's stored
// alias (the runtime minted its own alias and the adoption write failed), the
// reconciliation schedules a delete_connection for exactly that external id,
// deduplicated by a deterministic row id.
func TestOCEnqueueOrphanCleanupSchedulesDeleteForUnattributedExternal(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	ocSeedConnection(t, inst, 7, "conn1")
	att := ocSeedVerifyingAttempt(t, store, 7, "conn1", "alias-1", 1, now)

	if err := store.EnqueueOCOrphanCleanup(ctx, 7, att.ID, "ext-ORPHAN", now); err != nil {
		t.Fatalf("orphan cleanup: %v", err)
	}
	if err := store.EnqueueOCOrphanCleanup(ctx, 7, att.ID, "ext-ORPHAN", now); err != nil {
		t.Fatalf("duplicate orphan cleanup: %v", err)
	}
	var rows []OCOperationsOutboxRow
	if err := db.Where("tenant_id = ? AND kind = ?", 7, "delete_connection").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("orphan cleanup rows = %d, want exactly 1 (deduplicated)", len(rows))
	}
	if want := att.ID + "|ext-ORPHAN"; rows[0].ResourceID != want {
		t.Fatalf("orphan cleanup resource_id = %q, want %q", rows[0].ResourceID, want)
	}
}

// TestOCEnqueueOrphanCleanupNeverDeletesNewest: an ACTIVE binding owning the
// external connection means a newer activation won — no cleanup is scheduled.
func TestOCEnqueueOrphanCleanupNeverDeletesNewest(t *testing.T) {
	store, inst := testOCStore(t)
	db := store.db
	ctx := context.Background()
	now := time.Unix(1700000000, 0).UTC()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	att := ocSeedVerifyingAttempt(t, store, 7, "conn1", "alias-1", 1, now)

	if err := store.EnqueueOCOrphanCleanup(ctx, 7, att.ID, "ext-42", now); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Table("connector_operations_outbox").Where("tenant_id = ?", 7).Count(&n)
	if n != 0 {
		t.Fatalf("cleanup scheduled against the newest activation: %d rows", n)
	}
}

// ---------------------------------------------------------------------------
// matrix: stale fence write-back on the revocation cleanup row
// ---------------------------------------------------------------------------

// TestOCRevokeCleanupRowStaleFenceCompletesZeroRows: the durable cleanup row
// follows the frozen T05 lease protocol — after a takeover at fence+1 the
// ORIGINAL worker's Complete matches zero rows.
func TestOCRevokeCleanupRowStaleFenceCompletesZeroRows(t *testing.T) {
	store, inst := testOCStore(t)
	ctx := context.Background()
	ocSeedConnection(t, inst, 7, "conn1")
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatal(err)
	}

	claimNow := time.Now().UTC()
	a, err := store.ClaimNextOperation(ctx, "wA", claimNow, 30*time.Second)
	if err != nil || a == nil {
		t.Fatalf("first claim: %+v %v", a, err)
	}
	b, err := store.ClaimNextOperation(ctx, "wB", claimNow.Add(31*time.Second), 30*time.Second)
	if err != nil || b == nil || b.Fence != a.Fence+1 {
		t.Fatalf("takeover claim: %+v %v", b, err)
	}
	ok, err := store.CompleteOperation(ctx, a.ID, "wA", a.Fence)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("stale worker's complete affected rows after takeover")
	}
	ok, err = store.CompleteOperation(ctx, b.ID, "wB", b.Fence)
	if err != nil || !ok {
		t.Fatalf("new owner complete: ok=%v err=%v", ok, err)
	}
}

// ---------------------------------------------------------------------------
// PostgreSQL concurrency acceptance (coordinator R8): disposable container,
// NEVER the shared 5432 instance.
//
//	docker run -d --name weknora-oc-t08-pg-<n> -e POSTGRES_PASSWORD=oc_test \
//	  -e POSTGRES_DB=oc_test -p <free-port>:5432 postgres:16-alpine
//	docker exec -i weknora-oc-t08-pg-<n> psql -U postgres -d oc_test \
//	  < migrations/versioned/000117_app_installations.up.sql
//	docker exec -i weknora-oc-t08-pg-<n> psql -U postgres -d oc_test \
//	  < migrations/versioned/000121_open_connector_bindings.up.sql
//	OC_T08_PG_DSN='host=127.0.0.1 port=<free-port> user=postgres \
//	  password=oc_test dbname=oc_test sslmode=disable' \
//	  go test ./internal/application/repository/appconnector -run TestOCRevokePG -count=1 -v
//
// Afterwards: docker stop weknora-oc-t08-pg-<n> && docker rm weknora-oc-t08-pg-<n>
// ---------------------------------------------------------------------------

func ocRevokePGDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("OC_T08_PG_DSN")
	if dsn == "" {
		t.Skip("OC_T08_PG_DSN not set: run with the disposable postgres container (R8)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:  gormlogger.Discard,
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	for _, table := range []string{"connector_operations_outbox", "connector_connection_bindings", "connector_authorization_attempts", "connections", "installations", "app_versions"} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
	return db
}

func ocRevokePGSeed(t *testing.T, db *gorm.DB) *OCStore {
	t.Helper()
	if err := db.Exec("INSERT INTO app_versions (app_id, version) VALUES ('app-oc','1.0.0') ON CONFLICT DO NOTHING").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO installations (id, tenant_id, app_id, app_version, state) VALUES ('inst-7',7,'app-oc','1.0.0','active') ON CONFLICT DO NOTHING").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version) VALUES (7,'conn1','inst-7','space','owner','ref/cred','active',1) ON CONFLICT DO NOTHING").Error; err != nil {
		t.Fatal(err)
	}
	store := NewOCStore(db)
	ocSeedActiveBinding(t, store, 7, "conn1", "ext-42", "alias-1", 1)
	return store
}

// TestOCRevokePGConcurrentRevokesBumpExactlyOnce: N concurrent revocations
// racing on one connection inside real PostgreSQL transactions must produce
// exactly ONE version bump and exactly ONE durable cleanup row; every caller
// observes a coherent outcome (nil on the winner or the idempotent replay,
// conflict on a stale view).
func TestOCRevokePGConcurrentRevokesBumpExactlyOnce(t *testing.T) {
	db := ocRevokePGDB(t)
	store := ocRevokePGSeed(t, db)
	ctx := context.Background()
	subject := ocRevokeSubject(7)

	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	outcomes := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			outcomes[i] = store.RevokeOCConnection(ctx, subject, "conn1", 1)
		}(i)
	}
	close(start)
	wg.Wait()

	for i, err := range outcomes {
		if err != nil {
			t.Fatalf("concurrent revoke %d failed: %v (idempotent replay must succeed)", i, err)
		}
	}
	var state string
	var version int64
	if err := db.Raw("SELECT state, auth_version FROM connections WHERE tenant_id = ? AND id = ?", 7, "conn1").Row().Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != "revoked" || version != 2 {
		t.Fatalf("connection = %s@%d, want revoked@2 (exactly one bump)", state, version)
	}
	var rows int64
	if err := db.Raw("SELECT COUNT(*) FROM connector_operations_outbox WHERE tenant_id = ? AND resource_id = 'conn1' AND kind = 'delete_connection'", 7).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	if rows != 1 {
		t.Fatalf("durable cleanup rows = %d, want exactly 1", rows)
	}
}

// TestOCRevokePGSerializesWithConnectionRowLock: the locking contract T10's
// claim will rely on — revocation takes the connections row lock (SELECT ...
// FOR UPDATE) FIRST; a concurrent transaction already holding that lock
// blocks the revoke until it commits, and the version guard still holds
// afterwards.
func TestOCRevokePGSerializesWithConnectionRowLock(t *testing.T) {
	db := ocRevokePGDB(t)
	store := ocRevokePGSeed(t, db)
	ctx := context.Background()

	tx := db.Begin()
	if err := tx.Exec("SELECT id FROM connections WHERE tenant_id = ? AND id = ? FOR UPDATE", 7, "conn1").Error; err != nil {
		tx.Rollback()
		t.Fatalf("lock: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1) }()
	select {
	case err := <-done:
		tx.Rollback()
		t.Fatalf("revoke did not block behind the connection row lock: %v", err)
	case <-time.After(500 * time.Millisecond):
		// still blocked — the lock discipline holds.
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("revoke after lock release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("revoke never completed after the lock was released")
	}
	var state string
	var version int64
	if err := db.Raw("SELECT state, auth_version FROM connections WHERE tenant_id = ? AND id = ?", 7, "conn1").Row().Scan(&state, &version); err != nil {
		t.Fatal(err)
	}
	if state != "revoked" || version != 2 {
		t.Fatalf("connection = %s@%d, want revoked@2", state, version)
	}
	_ = fmt.Sprint()
}
