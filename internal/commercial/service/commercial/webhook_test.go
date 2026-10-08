package commercial

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/commercial/repository/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// stubReconcilePlatform is the commercial-platform fake for the webhook and
// reconciliation tests: Reconcile replays a programmable change stream and
// records the cursor it was called with.
type stubReconcilePlatform struct {
	mu       sync.Mutex
	pages    []domain.ReconciliationPage
	calls    []domain.ReconciliationCursor
	unknown  bool
}

func (p *stubReconcilePlatform) SubmitCommand(context.Context, domain.Command) (domain.CommandReceipt, error) {
	return domain.CommandReceipt{}, domain.ErrPlatformUnsupported
}
func (p *stubReconcilePlatform) ReadSnapshot(context.Context, domain.SnapshotQuery) (domain.Snapshot, error) {
	return domain.Snapshot{}, domain.ErrPlatformUnsupported
}
func (p *stubReconcilePlatform) Reconcile(_ context.Context, from domain.ReconciliationCursor) (domain.ReconciliationPage, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, from)
	if p.unknown {
		return domain.ReconciliationPage{}, domain.ErrPlatformUnreachable
	}
	if len(p.pages) == 0 {
		return domain.ReconciliationPage{Next: from}, nil
	}
	page := p.pages[0]
	p.pages = p.pages[1:]
	return page, nil
}

// stubReReader records re-reads and answers whether the projection changed.
type stubReReader struct {
	mu      sync.Mutex
	reads   []string
	changed bool
	err     error
}

func (r *stubReReader) reread(_ context.Context, tenantID uint64, externalID string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads = append(r.reads, fmt.Sprintf("%d/%s", tenantID, externalID))
	return r.changed, r.err
}

const wkTestStream = ReconciliationStream

func setupWebhook(t *testing.T, platform domain.CommercialPlatform, reader *stubReReader) (*WebhookService, *ReconciliationService, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1)
	}
	if err := db.AutoMigrate(&repocommercial.WebhookInboxRow{},
		&repocommercial.ProjectionAuditRow{}, &repocommercial.ReconciliationStateRow{}); err != nil {
		t.Fatal(err)
	}
	hooks := NewWebhookService(db, map[string][]byte{"lago": []byte("whsec-test")}, map[string]ProjectionReReader{
		"subscription": reader.reread,
	})
	recon := NewReconciliationService(db, platform, map[string]ProjectionReReader{
		"subscription": reader.reread,
	})
	return hooks, recon, db
}

func signWebhookBody(secret []byte, body []byte) string {
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func webhookBody(t *testing.T, eventID, kind, externalID string, tenant uint64) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]interface{}{
		"event_id": eventID, "webhook_type": kind, "object_id": externalID,
		"external_customer_id": domain.ExternalCustomerID(tenant),
	})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func auditCount(t *testing.T, db *gorm.DB, action string) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&repocommercial.ProjectionAuditRow{}).Where("action = ?", action).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// AC1: forged signature rejected, nothing persisted.
func TestWebhookForgedSignatureRejected(t *testing.T) {
	hooks, _, db := setupWebhook(t, &stubReconcilePlatform{}, &stubReReader{})
	body := webhookBody(t, "evt-1", "subscription", "sub_ext_1", 42)
	err := hooks.HandleWebhook(context.Background(), "lago", "sha256=" + hex.EncodeToString(make([]byte, 32)), body)
	if !errors.Is(err, ErrWebhookSignatureRejected) {
		t.Fatalf("forged signature accepted: %v", err)
	}
	var n int64
	db.Model(&repocommercial.WebhookInboxRow{}).Count(&n)
	if n != 0 {
		t.Fatalf("forged webhook persisted: %d rows", n)
	}
	if got := auditCount(t, db, "received"); got != 0 {
		t.Fatalf("forged webhook audited: %d", got)
	}
}

// AC2 (duplicates): a redelivery of the same unique key is a no-op — no
// second re-read, no second audit.
func TestWebhookDuplicateDeliveryIsNoOp(t *testing.T) {
	platform := &stubReconcilePlatform{}
	reader := &stubReReader{changed: true}
	hooks, _, db := setupWebhook(t, platform, reader)
	ctx := context.Background()
	body := webhookBody(t, "evt-dup", "subscription", "sub_ext_1", 42)
	sig := signWebhookBody([]byte("whsec-test"), body)
	for i := 0; i < 2; i++ {
		if err := hooks.HandleWebhook(ctx, "lago", sig, body); err != nil {
			t.Fatalf("delivery %d rejected: %v", i, err)
		}
	}
	if len(reader.reads) != 1 {
		t.Fatalf("duplicate delivery re-read %d times, want 1", len(reader.reads))
	}
	if got := auditCount(t, db, "repaired"); got != 1 {
		t.Fatalf("duplicate delivery audited %d times, want 1", got)
	}
}

// The notification is ONLY a trigger: the projection update comes from the
// authoritative re-read, and the audit records the divergence + repair.
func TestWebhookTriggersAuthoritativeReRead(t *testing.T) {
	reader := &stubReReader{changed: true}
	hooks, _, db := setupWebhook(t, &stubReconcilePlatform{}, reader)
	body := webhookBody(t, "evt-rr", "subscription", "sub_ext_1", 42)
	if err := hooks.HandleWebhook(context.Background(), "lago", signWebhookBody([]byte("whsec-test"), body), body); err != nil {
		t.Fatal(err)
	}
	if len(reader.reads) != 1 || reader.reads[0] != "42/sub_ext_1" {
		t.Fatalf("re-reads %v", reader.reads)
	}
	if got := auditCount(t, db, "repaired"); got != 1 {
		t.Fatalf("repair audit rows %d, want 1", got)
	}
}

// AC3: a lost notification is recovered by the reconciliation pass — the
// cursor advances monotonically and the same change is never re-processed.
func TestReconciliationRecoversMissedNotification(t *testing.T) {
	platform := &stubReconcilePlatform{pages: []domain.ReconciliationPage{{
		Changes: []domain.CommercialChange{{Kind: "subscription", TenantID: 42, ExternalID: "sub_ext_1", At: time.Now()}},
		Next:    domain.ReconciliationCursor{Stream: wkTestStream, Value: "cursor-2"},
	}}}
	reader := &stubReReader{changed: true}
	_, recon, db := setupWebhook(t, platform, reader)
	ctx := context.Background()
	if err := recon.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if len(reader.reads) != 1 {
		t.Fatalf("reconcile re-reads %v, want exactly the missed change", reader.reads)
	}
	if got := auditCount(t, db, "repaired"); got != 1 {
		t.Fatalf("repair audit rows %d, want 1", got)
	}
	var state repocommercial.ReconciliationStateRow
	if err := db.Where("stream = ?", wkTestStream).First(&state).Error; err != nil {
		t.Fatal(err)
	}
	if state.CursorValue != "cursor-2" {
		t.Fatalf("cursor %q, want cursor-2", state.CursorValue)
	}

	// Second pass from the advanced cursor: the authoritative stream
	// reports nothing between cursor-2 and cursor-3 — no re-read, no new
	// audit, the watermark still advances.
	platform.pages = []domain.ReconciliationPage{{
		Next: domain.ReconciliationCursor{Stream: wkTestStream, Value: "cursor-3"},
	}}
	if err := recon.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if len(reader.reads) != 1 {
		t.Fatalf("watermark did not advance: re-reads %v", reader.reads)
	}

	// An unreachable authority never moves the cursor.
	platform.unknown = true
	if err := recon.ReconcileOnce(ctx); err == nil {
		t.Fatal("unreachable authority must surface an error")
	}
	var after repocommercial.ReconciliationStateRow
	if err := db.Where("stream = ?", wkTestStream).First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.CursorValue != "cursor-3" {
		t.Fatalf("cursor moved on unreachable authority: %q", after.CursorValue)
	}
}

// AC2 (out-of-order): a late webhook for an older change cannot regress
// the projection — the re-read is idempotent on the authoritative value.
func TestWebhookOutOfOrderDoesNotRegress(t *testing.T) {
	reader := &stubReReader{}
	hooks, _, db := setupWebhook(t, &stubReconcilePlatform{}, reader)
	ctx := context.Background()
	// Older event delivered after a newer one: no divergence (authoritative
	// re-read already current) → no repair row, only receipt records.
	for _, id := range []string{"evt-new", "evt-old"} {
		body := webhookBody(t, id, "subscription", "sub_ext_1", 42)
		if err := hooks.HandleWebhook(ctx, "lago", signWebhookBody([]byte("whsec-test"), body), body); err != nil {
			t.Fatal(err)
		}
	}
	if got := auditCount(t, db, "repaired"); got != 0 {
		t.Fatalf("out-of-order delivery fabricated repairs: %d", got)
	}
	if got := auditCount(t, db, "received"); got != 2 {
		t.Fatalf("receipts %d, want 2", got)
	}
}
