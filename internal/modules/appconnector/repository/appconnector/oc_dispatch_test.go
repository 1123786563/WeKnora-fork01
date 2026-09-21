package appconnector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// T10: atomic claim, operation-scoped idempotency key, DB concurrency leases
// ---------------------------------------------------------------------------

// ocDispatchMigrationSQL loads the REAL sqlite twin migration (000043) so the
// dispatch tests run against the production schema — primary key, unique
// (runtime_id, key), check constraints and all.
func ocDispatchMigrationSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../migrations/sqlite/000043_open_connector_dispatch.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// testDispatchStore builds an isolated sqlite database carrying every parent
// table the claim transaction touches plus both open-connector migrations.
func testDispatchStore(t *testing.T) (*OCStore, *InstallationStore, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000&_foreign_keys=1"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// One pooled connection serializes the racing goroutines' transactions
	// (shared-cache SQLITE_LOCKED guard, same convention as the service
	// suite); TRUE cross-connection concurrency is proven on PostgreSQL.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&AppVersion{}, &InstallationRow{}, &ConnectionRow{}, &ActionRow{}, &ApprovalRow{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(ocMigrationSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(ocDispatchMigrationSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	return NewOCStore(db), NewInstallationStore(db), db
}

func ocTestBindingJSON() string {
	return "{\"RuntimeID\":\"rt-1\",\"Provider\":\"github\",\"ExternalID\":\"ext-42\",\"Alias\":\"alias-1\",\"ActionID\":\"send_message\",\"SchemaDigest\":\"sd-1\",\"BindingVersion\":1}"
}

// ocSeedClaimable seeds one tenant's full chain — installation, active
// connection at auth version v, ACTIVE binding — and one authorized
// open-connector action with an approval of the given remaining count.
func ocSeedClaimable(t *testing.T, inst *InstallationStore, store *OCStore, db *gorm.DB, tenant uint64, actionID string, authVersion int64, remaining int64) {
	t.Helper()
	ctx := context.Background()
	instID := fmt.Sprintf("inst-%d", tenant)
	// Idempotent parent seeding: several actions share one tenant's chain.
	if _, _, err := inst.GetInstallation(ctx, tenant, "app-oc"); errors.Is(err, gorm.ErrRecordNotFound) {
		if err := inst.ApplyInstallation(ctx, appconnector.Installation{ID: instID, AppID: "app-oc", Version: "1.0.0", State: appconnector.InstallationActive, TenantID: tenant}, 0); err != nil {
			t.Fatal(err)
		}
	} else if err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconnector.Connection{
		ID: "conn1", InstallationID: instID, Kind: appconnector.ConnectionKindSpace,
		OwnerID: "owner", CredentialRef: "cred/x", State: appconnector.ConnectionActive,
		TenantID: tenant, AuthVersion: authVersion,
	}); err != nil {
		t.Fatal(err)
	}
	// The alias is per-tenant: unique(runtime_id, provider, alias) must
	// not collide across the subtests sharing one database.
	if _, err := store.GetBinding(ctx, tenant, "conn1"); errors.Is(err, gorm.ErrRecordNotFound) {
		ocSeedActiveBinding(t, store, tenant, "conn1", fmt.Sprintf("ext-%d", tenant), fmt.Sprintf("alias-%d", tenant), authVersion)
	} else if err != nil {
		t.Fatal(err)
	}
	row := ActionRow{
		ID: actionID, TenantID: tenant, ActorID: "alice", ConnectionID: "conn1",
		AppVersion: "1.0.0", Target: "send_message", Risk: appconnector.RiskSend,
		AuthVersion: authVersion, ArgsSnapshot: "{\"body\":\"hi\"}", ArgsDigest: "dg-" + actionID,
		State: appconnector.ActionAuthorized, OCBindingJSON: ocTestBindingJSON(), DigestVersion: 2,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	ap := ApprovalRow{ArgsDigest: row.ArgsDigest, ActionID: actionID, Actor: "alice", Expiry: time.Now().Add(time.Hour).UTC(), Remaining: remaining}
	if err := db.Create(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if remaining == 0 {
		// gorm skips zero-valued fields that carry a column default — an
		// exhausted approval must be written explicitly.
		if err := db.Model(&ApprovalRow{}).Where("args_digest = ?", row.ArgsDigest).Update("remaining", 0).Error; err != nil {
			t.Fatal(err)
		}
	}
}

// TestOCKeyIsOperationScoped is VERBATIM from the plan: keys are unique,
// bounded and prefixed — never a connection id or any other reusable identity.
func TestOCKeyIsOperationScoped(t *testing.T) {
	a, b := NewOCKey(), NewOCKey()
	if a == b || len(a) > 255 || !strings.HasPrefix(a, "wk-oc-") {
		t.Fatal("invalid keys")
	}
}

// TestOCClaimRecordsKeyOnceAndLoserReadsWinner: the (tenant, action) primary
// key makes the claim a single linearization point — one winner consumes
// exactly one approval count and generates the key exactly once; the racing
// loser gets the WINNER's record (shared reservation) and cannot re-claim.
func TestOCClaimRecordsKeyOnceAndLoserReadsWinner(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	ocSeedClaimable(t, inst, store, db, 7, "act-1", 1, 5)

	rec, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-1", "res-77")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Key == "" || !strings.HasPrefix(rec.Key, "wk-oc-") {
		t.Fatalf("key = %q, want wk-oc- prefixed", rec.Key)
	}
	if rec.RuntimeID != "rt-1" || rec.State != "dispatched" || rec.ReservationID != "res-77" || rec.Fence != 1 {
		t.Fatalf("record = %+v", rec)
	}
	if rec.FirstSentAt.IsZero() {
		t.Fatal("FirstSentAt must carry database time")
	}
	if want := 23*time.Hour + 50*time.Minute; rec.ReplayUntil.Sub(rec.FirstSentAt) != want {
		t.Fatalf("replay window = %s, want %s", rec.ReplayUntil.Sub(rec.FirstSentAt), want)
	}
	var ap ApprovalRow
	if err := db.Where("action_id = ?", "act-1").First(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if ap.Remaining != 4 {
		t.Fatalf("remaining = %d, want decremented exactly once to 4", ap.Remaining)
	}

	loser, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-1", "res-88")
	if !errors.Is(err, ErrOCDispatchClaimed) {
		t.Fatalf("loser error = %v, want ErrOCDispatchClaimed", err)
	}
	if loser.Key != rec.Key || loser.ReservationID != "res-77" {
		t.Fatalf("loser record = %+v, want winner's key and shared reservation", loser)
	}
	if err := db.Where("action_id = ?", "act-1").First(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if ap.Remaining != 4 {
		t.Fatalf("loser consumed approval: remaining = %d", ap.Remaining)
	}
	var n int64
	db.Model(&OCDispatchRecordRow{}).Where("tenant_id = ?", 7).Count(&n)
	if n != 1 {
		t.Fatalf("dispatch records = %d, want exactly 1", n)
	}
}

// TestOCClaimTwoActionsSameConnectionProduceDistinctKeys pins the
// operation-scoped key rule: the SAME connection dispatching two different
// actions gets two different keys — a connection id is never the idempotency
// key, and no key is ever reused across operations.
func TestOCClaimTwoActionsSameConnectionProduceDistinctKeys(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	ocSeedClaimable(t, inst, store, db, 7, "act-a", 1, 5)
	ocSeedClaimable(t, inst, store, db, 7, "act-b", 1, 5)

	ra, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-a", "res-a")
	if err != nil {
		t.Fatal(err)
	}
	rb, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-b", "res-b")
	if err != nil {
		t.Fatal(err)
	}
	if ra.Key == rb.Key {
		t.Fatal("two actions on one connection share a key")
	}
	for _, k := range []string{ra.Key, rb.Key} {
		if k == "conn1" || strings.Contains(k, "conn1") {
			t.Fatalf("key %q derives from the connection id", k)
		}
	}
}

// TestOCClaimRevokedAfterCheckRaceRejected closes T04-QF-2: a revocation
// landing between the service's A02 Check and the claim transaction must be
// rejected BY the claim — no approval consumed, no dispatch record, the
// action stays authorized-but-blocked.
func TestOCClaimRevokedAfterCheckRaceRejected(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	ocSeedClaimable(t, inst, store, db, 7, "act-1", 1, 5)

	// The revocation a racing request performs after Check passed.
	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-1", "res-1"); !errors.Is(err, ErrOCDispatchConflict) {
		t.Fatalf("claim after revoke error = %v, want ErrOCDispatchConflict", err)
	}
	var ap ApprovalRow
	if err := db.Where("action_id = ?", "act-1").First(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if ap.Remaining != 5 {
		t.Fatalf("revoked claim consumed approval: remaining = %d", ap.Remaining)
	}
	var n int64
	db.Model(&OCDispatchRecordRow{}).Where("tenant_id = ?", 7).Count(&n)
	if n != 0 {
		t.Fatalf("revoked claim wrote a dispatch record: %d", n)
	}
	var row ActionRow
	if err := db.Where("id = ?", "act-1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconnector.ActionAuthorized {
		t.Fatalf("action state = %s, want still authorized", row.State)
	}
}

// TestOCClaimGuardsConnectionInstallationAndInput walks the remaining
// rejection paths of the claim transaction.
func TestOCClaimGuardsConnectionInstallationAndInput(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	subject := appconnector.OCSubject{TenantID: 7, ActorID: "alice"}

	t.Run("inactive connection state", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 7, "act-1", 1, 5)
		if err := db.Exec("UPDATE connections SET state = 'revoked' WHERE tenant_id = 7 AND id = 'conn1'").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.ClaimOCDispatch(ctx, subject, "act-1", "res-1"); !errors.Is(err, ErrOCDispatchConflict) {
			t.Fatalf("err = %v, want ErrOCDispatchConflict", err)
		}
	})

	t.Run("stale auth version", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 8, "act-2", 1, 5)
		if err := db.Exec("UPDATE connections SET auth_version = 2 WHERE tenant_id = 8 AND id = 'conn1'").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 8, ActorID: "alice"}, "act-2", "res-1"); !errors.Is(err, ErrOCDispatchConflict) {
			t.Fatalf("err = %v, want ErrOCDispatchConflict", err)
		}
	})

	t.Run("disabled installation", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 9, "act-3", 1, 5)
		if err := db.Exec("UPDATE installations SET state = 'disabled' WHERE tenant_id = 9 AND id = 'inst-9'").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 9, ActorID: "alice"}, "act-3", "res-1"); !errors.Is(err, ErrOCDispatchConflict) {
			t.Fatalf("err = %v, want ErrOCDispatchConflict", err)
		}
	})

	t.Run("revoked binding", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 10, "act-4", 1, 5)
		if err := db.Exec("UPDATE connector_connection_bindings SET state = 'revoked' WHERE tenant_id = 10 AND connection_id = 'conn1'").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 10, ActorID: "alice"}, "act-4", "res-1"); !errors.Is(err, ErrOCDispatchConflict) {
			t.Fatalf("err = %v, want ErrOCDispatchConflict", err)
		}
	})

	t.Run("exhausted approval", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 11, "act-5", 1, 0)
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 11, ActorID: "alice"}, "act-5", "res-1"); !errors.Is(err, ErrApprovalExhausted) {
			t.Fatalf("err = %v, want ErrApprovalExhausted", err)
		}
	})

	t.Run("expired approval", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 12, "act-6", 1, 5)
		if err := db.Exec("UPDATE app_action_approvals SET expiry = ? WHERE action_id = ?", time.Now().Add(-time.Minute).UTC(), "act-6").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 12, ActorID: "alice"}, "act-6", "res-1"); !errors.Is(err, ErrApprovalExhausted) {
			t.Fatalf("err = %v, want ErrApprovalExhausted", err)
		}
	})

	t.Run("native action without binding", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 13, "act-7", 1, 5)
		if err := db.Exec("UPDATE app_actions SET oc_binding_json = '' WHERE id = 'act-7'").Error; err != nil {
			t.Fatal(err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 13, ActorID: "alice"}, "act-7", "res-1"); !errors.Is(err, ErrOCDispatchInvalid) {
			t.Fatalf("err = %v, want ErrOCDispatchInvalid", err)
		}
	})

	t.Run("malformed input", func(t *testing.T) {
		ocSeedClaimable(t, inst, store, db, 14, "act-8", 1, 5)
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 0, ActorID: "alice"}, "act-8", "res-1"); !errors.Is(err, ErrOCDispatchInvalid) {
			t.Fatalf("zero tenant err = %v", err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 14, ActorID: ""}, "act-8", "res-1"); !errors.Is(err, ErrOCDispatchInvalid) {
			t.Fatalf("empty actor err = %v", err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 14, ActorID: "alice"}, "", "res-1"); !errors.Is(err, ErrOCDispatchInvalid) {
			t.Fatalf("empty action err = %v", err)
		}
		// A foreign TENANT cannot even see the action (tenant-scoped
		// lookup, indistinguishable from missing); a same-tenant foreign
		// ACTOR is an explicit invalid-claim rejection.
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 99, ActorID: "mallory"}, "act-8", "res-1"); !errors.Is(err, ErrActionNotFound) {
			t.Fatalf("foreign tenant err = %v", err)
		}
		if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 14, ActorID: "mallory"}, "act-8", "res-1"); !errors.Is(err, ErrOCDispatchInvalid) {
			t.Fatalf("foreign actor err = %v", err)
		}
	})

	t.Run("unknown action", func(t *testing.T) {
		if _, err := store.ClaimOCDispatch(ctx, subject, "act-missing", "res-1"); !errors.Is(err, ErrActionNotFound) {
			t.Fatalf("err = %v, want ErrActionNotFound", err)
		}
	})
}

// TestOCClaimConcurrentExactlyOneWinner: even on sqlite's serialized writer,
// racing claims on one action produce exactly one winner and one decrement.
func TestOCClaimConcurrentExactlyOneWinner(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	ocSeedClaimable(t, inst, store, db, 7, "act-1", 1, 5)

	const n = 6
	start := make(chan struct{})
	var wg sync.WaitGroup
	records := make([]appconnector.OCDispatchRecord, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			records[i], errs[i] = store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-1", fmt.Sprintf("res-%d", i))
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
		case errors.Is(err, ErrOCDispatchClaimed):
			if records[i].ReservationID == "" {
				t.Fatalf("loser %d did not read the winner's reservation", i)
			}
		default:
			t.Fatalf("unexpected claim error: %v", err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d, want exactly 1", winners)
	}
	var ap ApprovalRow
	if err := db.Where("action_id = ?", "act-1").First(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if ap.Remaining != 4 {
		t.Fatalf("remaining = %d, want 4 (single decrement)", ap.Remaining)
	}
}

// TestOCClaimFenceDriftRejected: a claim whose action row moved (fence/state
// guard) writes nothing.
func TestOCClaimFenceDriftRejected(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	ocSeedClaimable(t, inst, store, db, 7, "act-1", 1, 5)
	if err := db.Exec("UPDATE app_actions SET state = 'unknown' WHERE id = 'act-1'").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-1", "res-1"); !errors.Is(err, ErrOCDispatchConflict) {
		t.Fatalf("err = %v, want ErrOCDispatchConflict", err)
	}
}

// TestOCDispatchFinishGuardsAndNeverAdvancesDeadline: result updates run
// WHERE tenant/action/fence/state and never move FirstSentAt or ReplayUntil.
func TestOCDispatchFinishGuardsAndNeverAdvancesDeadline(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	ocSeedClaimable(t, inst, store, db, 7, "act-1", 1, 5)
	rec, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-1", "res-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := store.FinishOCDispatch(ctx, 7, "act-1", rec.Fence+1, "dispatched", appconnector.ActionSucceeded, "exec-9"); !errors.Is(err, ErrOCDispatchConflict) {
		t.Fatalf("stale fence err = %v, want ErrOCDispatchConflict", err)
	}
	if err := store.FinishOCDispatch(ctx, 7, "act-1", rec.Fence, "succeeded", appconnector.ActionFailed, "exec-9"); !errors.Is(err, ErrOCDispatchConflict) {
		t.Fatalf("wrong from-state err = %v, want ErrOCDispatchConflict", err)
	}
	if err := store.FinishOCDispatch(ctx, 7, "act-1", rec.Fence, "dispatched", appconnector.ActionSucceeded, "exec-9"); err != nil {
		t.Fatal(err)
	}
	var row OCDispatchRecordRow
	if err := db.Where("tenant_id = ? AND action_id = ?", 7, "act-1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconnector.ActionSucceeded || row.ExecutionID != "exec-9" {
		t.Fatalf("row = %+v", row)
	}
	if !row.FirstSentAt.Equal(rec.FirstSentAt) || !row.ReplayUntil.Equal(rec.ReplayUntil) {
		t.Fatalf("deadline moved on finish: first %v->%v replay %v->%v", rec.FirstSentAt, row.FirstSentAt, rec.ReplayUntil, row.ReplayUntil)
	}
	// Replay of the same finish is a guarded no-op.
	if err := store.FinishOCDispatch(ctx, 7, "act-1", rec.Fence, "dispatched", appconnector.ActionSucceeded, "exec-9"); !errors.Is(err, ErrOCDispatchConflict) {
		t.Fatalf("replay err = %v, want ErrOCDispatchConflict", err)
	}
	got, err := store.GetOCDispatch(ctx, 7, "act-1")
	if err != nil || got.Key != rec.Key {
		t.Fatalf("GetOCDispatch = %+v %v", got, err)
	}
}

// TestOCLeaseLimitReleaseAndCAS: the DB lease enforces the configured limit,
// releases free the slot, and a stale owner/fence can never release someone
// else's slot (compare-and-swap).
func TestOCLeaseLimitReleaseAndCAS(t *testing.T) {
	store, _, _ := testDispatchStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	a, err := store.AcquireOCLease(ctx, "oc-provider:github", 2, "replica-1", 7, "act-1", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	b, err := store.AcquireOCLease(ctx, "oc-provider:github", 2, "replica-1", 7, "act-2", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("two acquisitions share a lease id")
	}
	if _, err := store.AcquireOCLease(ctx, "oc-provider:github", 2, "replica-1", 7, "act-3", now, now.Add(time.Minute)); !errors.Is(err, ErrOCLeaseBusy) {
		t.Fatalf("third acquisition err = %v, want ErrOCLeaseBusy", err)
	}
	// Stale fence and foreign owner both fail closed.
	ok, err := store.ReleaseOCLease(ctx, a.ID, "replica-1", a.Fence+5, now)
	if err != nil || ok {
		t.Fatalf("stale fence release ok=%v err=%v", ok, err)
	}
	ok, err = store.ReleaseOCLease(ctx, a.ID, "replica-2", a.Fence, now)
	if err != nil || ok {
		t.Fatalf("foreign owner release ok=%v err=%v", ok, err)
	}
	if ok, err := store.ReleaseOCLease(ctx, a.ID, "replica-1", a.Fence, now); err != nil || !ok {
		t.Fatalf("release ok=%v err=%v", ok, err)
	}
	c, err := store.AcquireOCLease(ctx, "oc-provider:github", 2, "replica-2", 8, "act-3", now, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if c.Owner != "replica-2" {
		t.Fatalf("reacquire owner = %s", c.Owner)
	}
}

// TestOCLeaseExpiredSlotReclaimedWithoutResending: a crashed holder's expired
// lease is reclaimed by the next acquisition, and the reclaim never touches
// the dispatch record or re-queues the action (no automatic re-send).
func TestOCLeaseExpiredSlotReclaimedWithoutResending(t *testing.T) {
	store, inst, db := testDispatchStore(t)
	ctx := context.Background()
	ocSeedClaimable(t, inst, store, db, 7, "act-1", 1, 5)
	rec, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-1", "res-1")
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now().UTC()
	crashed, err := store.AcquireOCLease(ctx, "oc-conn:7:conn1", 1, "replica-1", 7, "act-1", now, now.Add(50*time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}
	_ = crashed // crash: no release call ever happens
	time.Sleep(80 * time.Millisecond)
	next, err := store.AcquireOCLease(ctx, "oc-conn:7:conn1", 1, "replica-2", 7, "act-1", now.Add(time.Second), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("expired lease not reclaimed: %v", err)
	}
	if next.ID == crashed.ID {
		t.Fatal("reclaimed slot kept the dead lease row")
	}
	var row OCDispatchRecordRow
	if err := db.Where("tenant_id = ? AND action_id = ?", 7, "act-1").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != "dispatched" || row.Key != rec.Key || row.ReservationID != "res-1" {
		t.Fatalf("lease reclaim disturbed the dispatch: %+v", row)
	}
	var action ActionRow
	if err := db.Where("id = ?", "act-1").First(&action).Error; err != nil {
		t.Fatal(err)
	}
	if action.State != appconnector.ActionDispatched {
		t.Fatalf("lease reclaim re-queued the action: state = %s", action.State)
	}
	var outbox int64
	db.Model(&OCOperationsOutboxRow{}).Where("tenant_id = ?", 7).Count(&outbox)
	if outbox != 0 {
		t.Fatalf("lease reclaim enqueued %d re-send operations", outbox)
	}
}

// TestOCProviderRetryAfterForwardOnly: a stored provider Retry-After is
// readable until it passes and can only move forward.
func TestOCProviderRetryAfterForwardOnly(t *testing.T) {
	store, _, _ := testDispatchStore(t)
	ctx := context.Background()
	now := time.Now().UTC()

	if d, err := store.OCProviderRetryAfter(ctx, "github", now); err != nil || d != 0 {
		t.Fatalf("empty state = %d %v", d, err)
	}
	if err := store.NoteOCProviderRetryAfter(ctx, "github", now.Add(30*time.Second), now); err != nil {
		t.Fatal(err)
	}
	d, err := store.OCProviderRetryAfter(ctx, "github", now.Add(time.Second))
	if err != nil || d <= 0 {
		t.Fatalf("cooling = %d %v, want > 0", d, err)
	}
	// Backward move is refused.
	if err := store.NoteOCProviderRetryAfter(ctx, "github", now.Add(time.Second), now); !errors.Is(err, ErrOCDispatchConflict) {
		t.Fatalf("backward move err = %v, want ErrOCDispatchConflict", err)
	}
	d, err = store.OCProviderRetryAfter(ctx, "github", now.Add(31*time.Second))
	if err != nil || d != 0 {
		t.Fatalf("after window = %d %v, want 0", d, err)
	}
}

// ---------------------------------------------------------------------------
// PostgreSQL concurrency acceptance (coordinator R8): disposable container,
// NEVER the shared 5432 instance.
//
//	docker run -d --name weknora-oc-t10-pg-1 -e POSTGRES_PASSWORD=oc_test \
//	  -e POSTGRES_DB=oc_test -p 127.0.0.1:55432:5432 postgres:16-alpine
//	for f in 000117_app_installations 000119_app_actions \
//	         000121_open_connector_bindings 000122_open_connector_actions \
//	         000123_open_connector_dispatch; do
//	  docker exec -i weknora-oc-t10-pg-1 psql -U postgres -d oc_test \
//	    < migrations/versioned/$f.up.sql
//	done
//	OC_T10_PG_DSN='host=127.0.0.1 port=55432 user=postgres \
//	  password=oc_test dbname=oc_test sslmode=disable' \
//	  go test ./internal/application/repository/appconnector -run TestOCT10PG -count=1 -v
//
// Afterwards: docker stop weknora-oc-t10-pg-1 && docker rm weknora-oc-t10-pg-1
// ---------------------------------------------------------------------------

func ocT10PGDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("OC_T10_PG_DSN")
	if dsn == "" {
		t.Skip("OC_T10_PG_DSN not set: run with the disposable postgres container (R8)")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:  gormlogger.Discard,
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	for _, table := range []string{
		"connector_provider_retry_state", "connector_dispatch_leases", "connector_dispatch_lease_scopes",
		"connector_dispatch_records", "app_action_approvals", "app_actions",
		"connector_operations_outbox", "connector_connection_bindings", "connector_authorization_attempts",
		"connections", "installations", "app_versions",
	} {
		if err := db.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("clean %s: %v", table, err)
		}
	}
	return db
}

func ocT10PGSeed(t *testing.T, db *gorm.DB, tenant uint64, actionID string, remaining int64) *OCStore {
	t.Helper()
	if err := db.Exec("INSERT INTO app_versions (app_id, version) VALUES ('app-oc','1.0.0') ON CONFLICT DO NOTHING").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(fmt.Sprintf("INSERT INTO installations (id, tenant_id, app_id, app_version, state) VALUES ('inst-%d',%d,'app-oc','1.0.0','active') ON CONFLICT DO NOTHING", tenant, tenant)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(fmt.Sprintf("INSERT INTO connections (tenant_id, id, installation_id, kind, owner_id, credential_ref, state, auth_version) VALUES (%d,'conn1','inst-%d','space','owner','ref/cred','active',1) ON CONFLICT DO NOTHING", tenant, tenant)).Error; err != nil {
		t.Fatal(err)
	}
	store := NewOCStore(db)
	ocSeedActiveBinding(t, store, tenant, "conn1", "ext-42", "alias-1", 1)
	if actionID != "" {
		if err := db.Exec(fmt.Sprintf("INSERT INTO app_actions (id, tenant_id, actor_id, connection_id, app_version, target, risk, auth_version, args_snapshot, args_digest, state, oc_binding_json, digest_version) VALUES ('%s',%d,'alice','conn1','1.0.0','send_message','send',1,'{}','dg-%s','authorized',?,2) ON CONFLICT DO NOTHING", actionID, tenant, actionID), ocTestBindingJSON()).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec(fmt.Sprintf("INSERT INTO app_action_approvals (args_digest, action_id, actor, expiry, remaining) VALUES ('dg-%s','%s','alice', CURRENT_TIMESTAMP + INTERVAL '1 hour', %d) ON CONFLICT DO NOTHING", actionID, actionID, remaining)).Error; err != nil {
			t.Fatal(err)
		}
	}
	return store
}

// TestOCT10PGTwentyConcurrentClaimsExactlyOneWinner: 20 truly concurrent
// claims on one action inside real PostgreSQL — exactly one winner, one
// decrement, one record; every loser reads the winner's reservation.
func TestOCT10PGTwentyConcurrentClaimsExactlyOneWinner(t *testing.T) {
	db := ocT10PGDB(t)
	store := ocT10PGSeed(t, db, 7, "act-pg", 5)
	ctx := context.Background()
	subject := appconnector.OCSubject{TenantID: 7, ActorID: "alice"}

	const n = 20
	start := make(chan struct{})
	var wg sync.WaitGroup
	records := make([]appconnector.OCDispatchRecord, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			records[i], errs[i] = store.ClaimOCDispatch(ctx, subject, "act-pg", fmt.Sprintf("res-%d", i))
		}(i)
	}
	close(start)
	wg.Wait()

	winners := 0
	winnerIdx := -1
	for i, err := range errs {
		switch {
		case err == nil:
			winners++
			winnerIdx = i
		case errors.Is(err, ErrOCDispatchClaimed):
		default:
			t.Fatalf("unexpected claim error %d: %v", i, err)
		}
	}
	if winners != 1 {
		t.Fatalf("winners = %d, want exactly 1", winners)
	}
	winnerKey, winnerReservation := records[winnerIdx].Key, records[winnerIdx].ReservationID
	for i, err := range errs {
		if errors.Is(err, ErrOCDispatchClaimed) {
			if records[i].Key != winnerKey || records[i].ReservationID != winnerReservation {
				t.Fatalf("loser %d = %q/%q, want winner's %q/%q", i, records[i].Key, records[i].ReservationID, winnerKey, winnerReservation)
			}
		}
	}
	var remaining int64
	if err := db.Raw("SELECT remaining FROM app_action_approvals WHERE action_id = 'act-pg'").Row().Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 4 {
		t.Fatalf("remaining = %d, want 4 (decremented once)", remaining)
	}
	var count int64
	if err := db.Raw("SELECT COUNT(*) FROM connector_dispatch_records WHERE tenant_id = 7 AND action_id = 'act-pg'").Scan(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("dispatch records = %d, want 1", count)
	}
	var state string
	var fence int64
	if err := db.Raw("SELECT state, fence FROM app_actions WHERE id = 'act-pg'").Row().Scan(&state, &fence); err != nil {
		t.Fatal(err)
	}
	if state != "dispatched" || fence != 1 {
		t.Fatalf("action = %s@%d, want dispatched@1", state, fence)
	}
}

// TestOCT10PGClaimBlocksOnConnectionRowLock: the claim transaction takes the
// SAME connections row lock the T08 revocation takes — a transaction already
// holding it blocks the claim until it commits (unified lock order, no
// deadlock, no reverse order).
func TestOCT10PGClaimBlocksOnConnectionRowLock(t *testing.T) {
	db := ocT10PGDB(t)
	store := ocT10PGSeed(t, db, 7, "act-pg", 5)
	ctx := context.Background()

	tx := db.Begin()
	if err := tx.Exec("SELECT id FROM connections WHERE tenant_id = ? AND id = ? FOR UPDATE", 7, "conn1").Error; err != nil {
		tx.Rollback()
		t.Fatalf("lock: %v", err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-pg", "res-lock")
		done <- err
	}()
	select {
	case err := <-done:
		tx.Rollback()
		t.Fatalf("claim did not block behind the connection row lock: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("claim after lock release: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("claim never completed after the lock was released")
	}
}

// TestOCT10PGRevocationAfterCheckRejected: a revocation committing between
// the service Check and the claim is rejected by the claim transaction
// itself — T04-QF-2 closure on real PostgreSQL.
func TestOCT10PGRevocationAfterCheckRejected(t *testing.T) {
	db := ocT10PGDB(t)
	store := ocT10PGSeed(t, db, 7, "act-pg", 5)
	ctx := context.Background()

	if err := store.RevokeOCConnection(ctx, ocRevokeSubject(7), "conn1", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-pg", "res-1"); !errors.Is(err, ErrOCDispatchConflict) {
		t.Fatalf("claim after revoke err = %v, want ErrOCDispatchConflict", err)
	}
	var remaining int64
	if err := db.Raw("SELECT remaining FROM app_action_approvals WHERE action_id = 'act-pg'").Row().Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 5 {
		t.Fatalf("revoked claim consumed approval: remaining = %d", remaining)
	}
	var n int64
	if err := db.Raw("SELECT COUNT(*) FROM connector_dispatch_records WHERE tenant_id = 7").Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("revoked claim wrote %d records", n)
	}
}

// TestOCT10PGLeaseTwoOwnersTwoSpaces: two lease owners (replicas) across two
// spaces contend on the shared provider scope — the limit holds at every
// observation, releases hand capacity to the waiting space (no starvation),
// and a stale owner can never CAS someone else's slot away.
func TestOCT10PGLeaseTwoOwnersTwoSpaces(t *testing.T) {
	db := ocT10PGDB(t)
	store := NewOCStore(db)
	ctx := context.Background()
	const scope = "oc-provider:github"
	const limit = 2
	liveCount := func() int64 {
		var live int64
		if err := db.Raw("SELECT COUNT(*) FROM connector_dispatch_leases WHERE scope = ? AND released_at IS NULL AND until > CURRENT_TIMESTAMP", scope).Scan(&live).Error; err != nil {
			t.Fatal(err)
		}
		if live > limit {
			t.Fatalf("live leases = %d > limit %d", live, limit)
		}
		return live
	}

	held := make([]OCDispatchLeaseRow, 2)
	// One acquisition per space first: both spaces hold a slot.
	for i, tenant := range []uint64{7, 8} {
		l, err := store.AcquireOCLease(ctx, scope, limit, fmt.Sprintf("replica-%d", i+1), tenant, fmt.Sprintf("act-%d", tenant), time.Now().UTC(), time.Now().UTC().Add(time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		held[i] = l
		liveCount()
	}
	// The scope is full: a third acquisition from either space is refused.
	if _, err := store.AcquireOCLease(ctx, scope, limit, "replica-1", 7, "act-7b", time.Now().UTC(), time.Now().UTC().Add(time.Minute)); !errors.Is(err, ErrOCLeaseBusy) {
		t.Fatalf("err = %v, want ErrOCLeaseBusy", err)
	}
	// Stale owner cannot release a foreign slot (CAS discipline).
	if ok, err := store.ReleaseOCLease(ctx, held[1].ID, "replica-1", held[1].Fence, time.Now().UTC()); err != nil || ok {
		t.Fatalf("foreign CAS release ok=%v err=%v", ok, err)
	}
	// Releasing one slot hands capacity to the REFUSED space.
	if ok, err := store.ReleaseOCLease(ctx, held[0].ID, held[0].Owner, held[0].Fence, time.Now().UTC()); err != nil || !ok {
		t.Fatalf("release ok=%v err=%v", ok, err)
	}
	next, err := store.AcquireOCLease(ctx, scope, limit, "replica-1", 7, "act-7b", time.Now().UTC(), time.Now().UTC().Add(time.Minute))
	if err != nil {
		t.Fatalf("waiting space did not win the freed slot: %v", err)
	}
	held[0] = next
	// Round-robin across spaces: alternating acquire/release keeps both
	// spaces progressing equally under the limit.
	counts := map[uint64]int{7: 0, 8: 0}
	for round := 0; round < 4; round++ {
		for _, tenant := range []uint64{7, 8} {
			if _, err := store.ReleaseOCLease(ctx, held[0].ID, held[0].Owner, held[0].Fence, time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			l, err := store.AcquireOCLease(ctx, scope, limit, "replica-1", tenant, fmt.Sprintf("act-%d-%d", tenant, round), time.Now().UTC(), time.Now().UTC().Add(time.Minute))
			if err != nil {
				t.Fatalf("round %d space %d starved: %v", round, tenant, err)
			}
			held[0] = l
			counts[tenant]++
			liveCount()
		}
	}
	if counts[7] != counts[8] {
		t.Fatalf("space counts = %v, want equal progress", counts)
	}
}

// TestOCT10PGCrashedLeaseReleaseDoesNotResendDispatched: a crashed holder's
// expired slot is reclaimed, and the reclaim never re-sends a dispatched
// action — record, key, reservation, deadline and action state all stay
// untouched, and no outbox operation appears.
func TestOCT10PGCrashedLeaseReleaseDoesNotResendDispatched(t *testing.T) {
	db := ocT10PGDB(t)
	store := ocT10PGSeed(t, db, 7, "act-pg", 5)
	ctx := context.Background()

	rec, err := store.ClaimOCDispatch(ctx, appconnector.OCSubject{TenantID: 7, ActorID: "alice"}, "act-pg", "res-crash")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := store.AcquireOCLease(ctx, "oc-conn:7:conn1", 1, "replica-1", 7, "act-pg", now, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	// Crash: the holder never releases. After expiry another replica reclaims.
	time.Sleep(1200 * time.Millisecond)
	if _, err := store.AcquireOCLease(ctx, "oc-conn:7:conn1", 1, "replica-2", 7, "act-pg", now.Add(2*time.Second), now.Add(time.Minute)); err != nil {
		t.Fatalf("expired lease not reclaimed: %v", err)
	}
	var state, key, reservation string
	var firstSent, replayUntil time.Time
	if err := db.Raw("SELECT state, key, reservation_id, first_sent_at, replay_until FROM connector_dispatch_records WHERE tenant_id = 7 AND action_id = 'act-pg'").Row().Scan(&state, &key, &reservation, &firstSent, &replayUntil); err != nil {
		t.Fatal(err)
	}
	if state != "dispatched" || key != rec.Key || reservation != "res-crash" {
		t.Fatalf("lease reclaim disturbed the dispatch: %s %s %s", state, key, reservation)
	}
	if !firstSent.Equal(rec.FirstSentAt) || !replayUntil.Equal(rec.ReplayUntil) {
		t.Fatal("lease reclaim moved the replay deadline")
	}
	var actionState string
	if err := db.Raw("SELECT state FROM app_actions WHERE id = 'act-pg'").Row().Scan(&actionState); err != nil {
		t.Fatal(err)
	}
	if actionState != "dispatched" {
		t.Fatalf("lease reclaim re-queued the action: %s", actionState)
	}
	var outbox int64
	if err := db.Raw("SELECT COUNT(*) FROM connector_operations_outbox WHERE tenant_id = 7").Scan(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if outbox != 0 {
		t.Fatalf("lease reclaim enqueued %d re-send operations", outbox)
	}
}
