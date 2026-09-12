package commercial

import (
	"context"
	"errors"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func recoveryTestEnv(t *testing.T) (*gorm.DB, *RecoveryService) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.OutboxEvent{}, &repocommercial.RefundRow{}, &RecoveryAuditRow{}); err != nil {
		t.Fatal(err)
	}
	svc, err := NewRecoveryService(db, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetOperatorCheck(func(context.Context, string) error { return nil })
	svc.SetStateResolver(func(_ context.Context, e repocommercial.OutboxEvent) (RecoveryObjectState, error) {
		return RecoveryObjectState{BusinessID: e.EventKey, Version: 1, State: "active"}, nil
	})
	return db, svc
}

func seedOutboxEvent(t *testing.T, db *gorm.DB, key, kind, state string, leaseUntil time.Time) {
	t.Helper()
	if err := db.Create(&repocommercial.OutboxEvent{
		EventKey: key, TenantID: 7, Kind: kind, PayloadJSON: "{}", State: state, LeaseUntil: leaseUntil,
	}).Error; err != nil {
		t.Fatal(err)
	}
}

// TestRecoveryReplayUsesOriginalIdempotencyKey: a replay re-arms the
// ORIGINAL event key — exactly one outbox row remains, still under the
// original key, back to pending — and the audit row names the actor.
func TestRecoveryReplayUsesOriginalIdempotencyKey(t *testing.T) {
	db, svc := recoveryTestEnv(t)
	seedOutboxEvent(t, db, "evt_orig", domain.OperationPaymentCallback, repocommercial.OutboxStateDead, time.Now().UTC().Add(-time.Hour))
	if err := svc.Replay(context.Background(), "evt_orig", "op_alice"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&repocommercial.OutboxEvent{}).Count(&count)
	if count != 1 {
		t.Fatalf("outbox rows = %d, want 1 (no new idempotency key)", count)
	}
	var ev repocommercial.OutboxEvent
	db.First(&ev, "event_key = ?", "evt_orig")
	if ev.State != repocommercial.OutboxStatePending {
		t.Fatalf("state = %q, want pending", ev.State)
	}
	if ev.AttemptCount != 1 {
		t.Fatalf("attempts = %d, want 1", ev.AttemptCount)
	}
	var audits []RecoveryAuditRow
	db.Find(&audits)
	if len(audits) != 1 || audits[0].Actor != "op_alice" || audits[0].Result != "ok" {
		t.Fatalf("audit = %+v", audits)
	}
}

// TestRecoveryReplayRefusesRevokedConnection: a revoked connection can
// never be re-enabled by a replay; the event stays untouched and the
// refusal is audited.
func TestRecoveryReplayRefusesRevokedConnection(t *testing.T) {
	db, svc := recoveryTestEnv(t)
	seedOutboxEvent(t, db, "evt_conn", domain.OperationFulfillmentReplay, repocommercial.OutboxStateDead, time.Now().UTC())
	svc.SetStateResolver(func(context.Context, repocommercial.OutboxEvent) (RecoveryObjectState, error) {
		return RecoveryObjectState{BusinessID: "conn_9", Version: 3, State: "revoked", Revoked: true}, nil
	})
	err := svc.Replay(context.Background(), "evt_conn", "op_alice")
	if !errors.Is(err, ErrRecoveryConnectionRevoked) {
		t.Fatalf("err = %v, want ErrRecoveryConnectionRevoked", err)
	}
	var ev repocommercial.OutboxEvent
	db.First(&ev, "event_key = ?", "evt_conn")
	if ev.State != repocommercial.OutboxStateDead {
		t.Fatalf("state = %q, want unchanged dead", ev.State)
	}
	var audits []RecoveryAuditRow
	db.Find(&audits)
	if len(audits) != 1 || audits[0].Result != "error" || audits[0].Actor != "op_alice" {
		t.Fatalf("audit = %+v", audits)
	}
}

// TestRecoveryCapabilityCheckFailsClosed: without an injected
// platform-operator check nobody can replay; a denying check refuses the
// actor; both paths leave the event untouched.
func TestRecoveryCapabilityCheckFailsClosed(t *testing.T) {
	db, svc := recoveryTestEnv(t)
	seedOutboxEvent(t, db, "evt_1", domain.OperationSettlement, repocommercial.OutboxStatePending, time.Now().UTC())
	svc.SetOperatorCheck(nil)
	if err := svc.Replay(context.Background(), "evt_1", "op_alice"); !errors.Is(err, ErrRecoveryOperatorCheckMissing) {
		t.Fatalf("err = %v, want ErrRecoveryOperatorCheckMissing", err)
	}
	svc.SetOperatorCheck(func(context.Context, string) error { return errors.New("no capability") })
	err := svc.Replay(context.Background(), "evt_1", "op_mallory")
	if !errors.Is(err, ErrRecoveryUnauthorized) {
		t.Fatalf("err = %v, want ErrRecoveryUnauthorized", err)
	}
	var ev repocommercial.OutboxEvent
	db.First(&ev, "event_key = ?", "evt_1")
	if ev.State != repocommercial.OutboxStatePending || ev.AttemptCount != 0 {
		t.Fatalf("event mutated: %+v", ev)
	}
	var audits []RecoveryAuditRow
	db.Find(&audits)
	if len(audits) != 2 {
		t.Fatalf("audit rows = %d, want 2 (both attempts recorded)", len(audits))
	}
}

// TestRecoveryUnknownWriteIsQueryOnly: an operation kind outside the
// rollback allowlist (connector write, new order) is NEVER re-dispatched;
// recovery may only query/reconcile it.
func TestRecoveryUnknownWriteIsQueryOnly(t *testing.T) {
	db, svc := recoveryTestEnv(t)
	for _, kind := range []string{"connector_action", "new_order", "totally_new_kind"} {
		key := "evt_" + kind
		seedOutboxEvent(t, db, key, kind, repocommercial.OutboxStatePending, time.Now().UTC())
		if err := svc.Replay(context.Background(), key, "op_alice"); !errors.Is(err, ErrRecoveryQueryOnly) {
			t.Fatalf("kind %q err = %v, want ErrRecoveryQueryOnly", kind, err)
		}
		var ev repocommercial.OutboxEvent
		db.First(&ev, "event_key = ?", key)
		if ev.AttemptCount != 0 {
			t.Fatalf("kind %q re-dispatched (attempts %d)", kind, ev.AttemptCount)
		}
	}
}

// TestRecoverySwitchesClosedRejectNewConsumptionWhileReplayContinues:
// closing all three config switches rejects NEW consumption lanes but a
// payment_callback replay still processes — recovery of paid work never
// stops with the switches.
func TestRecoverySwitchesClosedRejectNewConsumptionWhileReplayContinues(t *testing.T) {
	db, _ := recoveryTestEnv(t) // migrate tables once
	off := false
	cfg := &config.Config{
		CommercialNewOrders:   &off,
		CommercialNewDispatch: &off,
		ConnectorNewActions:   &off,
	}
	svc, err := NewRecoveryService(db, cfg)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetOperatorCheck(func(context.Context, string) error { return nil })
	svc.SetStateResolver(func(_ context.Context, e repocommercial.OutboxEvent) (RecoveryObjectState, error) {
		return RecoveryObjectState{BusinessID: e.EventKey, Version: 1, State: "active"}, nil
	})
	ctx := context.Background()
	for _, op := range []string{"new_order", "new_dispatch", "connector_action"} {
		if err := svc.CheckNewConsumption(ctx, op); !errors.Is(err, ErrNewConsumptionPaused) {
			t.Fatalf("closed switch accepted %q", op)
		}
	}
	seedOutboxEvent(t, db, "evt_cb", domain.OperationPaymentCallback, repocommercial.OutboxStateDead, time.Now().UTC())
	if err := svc.Replay(ctx, "evt_cb", "op_alice"); err != nil {
		t.Fatalf("callback replay blocked by switches: %v", err)
	}
}

// TestRecoveryQueueListingCategorizesWithAgeAndBusinessID: the queue
// projects category, age, tenant and business ID for undelivered events
// and indeterminate/revocation-pending refunds.
func TestRecoveryQueueListingCategorizesWithAgeAndBusinessID(t *testing.T) {
	db, svc := recoveryTestEnv(t)
	seedOutboxEvent(t, db, "evt_paid", domain.OperationPaymentCallback, repocommercial.OutboxStatePending, time.Now().UTC().Add(-time.Hour))
	seedOutboxEvent(t, db, "evt_settle", domain.OperationSettlement, repocommercial.OutboxStateDead, time.Now().UTC().Add(-2*time.Hour))
	seedOutboxEvent(t, db, "evt_weird", "connector_action", repocommercial.OutboxStatePending, time.Now().UTC())
	if err := db.Create(&repocommercial.RefundRow{ID: "rfd_1", TenantID: 7, OrderID: "ord_1", OrderLineID: "l1", AmountFen: 100, CreditsMicro: 10, State: domain.RefundStateRevocationPending, Version: 1}).Error; err != nil {
		t.Fatal(err)
	}
	items, err := svc.ListQueue(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]domain.RecoveryCategory{
		"evt_paid":   domain.RecoveryCategoryPaidUnfulfilled,
		"evt_settle": domain.RecoveryCategoryBalanceDiscrepancy,
		"evt_weird":  domain.RecoveryCategoryUnknownAction,
		"rfd_1":      domain.RecoveryCategoryRevocationPending,
	}
	if len(items) != len(want) {
		t.Fatalf("items = %d, want %d: %+v", len(items), len(want), items)
	}
	for _, it := range items {
		c, ok := want[it.OperationID]
		if !ok {
			t.Fatalf("unexpected item %q", it.OperationID)
		}
		if it.Category != c {
			t.Fatalf("%q category = %q, want %q", it.OperationID, it.Category, c)
		}
		if it.TenantID != 7 || it.BusinessID == "" {
			t.Fatalf("%q projection incomplete: %+v", it.OperationID, it)
		}
		if it.OperationID == "evt_paid" && it.Age < 30*time.Minute {
			t.Fatalf("evt_paid age = %v, want ~1h", it.Age)
		}
	}
}
