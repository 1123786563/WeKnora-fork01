package commercial

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/payment"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// stubRefundProvider is the channel boundary fake for the refund worker
// tests: real channel acceptance stays with the C02/C03 suites; these
// tests prove refund STATE transitions against real SQLite transactions.
type stubRefundProvider struct {
	mu          sync.Mutex
	refundState payment.AttemptState // state returned by Refund
	queryState  payment.AttemptState // state returned by QueryRefund
	refundCalls []string
	queryCalls  []string
}

func (p *stubRefundProvider) Create(context.Context, payment.OrderRequest) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StateSucceeded}, nil
}
func (p *stubRefundProvider) Query(context.Context, string) (payment.AttemptResult, error) {
	return payment.AttemptResult{State: payment.StateSucceeded}, nil
}
func (p *stubRefundProvider) Close(context.Context, string) error { return nil }
func (p *stubRefundProvider) Verify(context.Context, http.Header, []byte) (domain.PaymentFact, error) {
	return domain.PaymentFact{}, nil
}
func (p *stubRefundProvider) Refund(_ context.Context, req payment.RefundRequest) (payment.RefundResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refundCalls = append(p.refundCalls, req.RefundID)
	return payment.RefundResult{State: p.refundState, ProviderID: "pr_" + req.RefundID}, nil
}
func (p *stubRefundProvider) QueryRefund(_ context.Context, refundID string) (payment.RefundResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queryCalls = append(p.queryCalls, refundID)
	return payment.RefundResult{State: p.queryState, ProviderID: "pr_" + refundID}, nil
}

func (p *stubRefundProvider) counts() (int, int, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.refundCalls), len(p.queryCalls), append([]string(nil), p.queryCalls...)
}

// readyEligibility stands in for the future P03 occupancy-refundable
// check so the concurrency tests can exercise the REAL admission
// transaction that P03 will share. Production wiring keeps the default
// not-ready checker.
type readyEligibility struct{}

func (readyEligibility) RefundEligible(context.Context, uint64, string, domain.Credits) error {
	return nil
}

const (
	rfTenant   uint64 = 77
	rfOrderID         = "ord_rf"
	rfLot             = "credits"
	rfLotMicro int64  = 100_000_000 // 100 Credits
)

func setupRefund(t *testing.T, gw domain.CommercialGateway, provider payment.Provider, eligibility domain.RefundEligibilityChecker) (*RefundService, *gorm.DB, *stubRefundProvider) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1) // serialize SQLite writers; races stay logical
	}
	if err := db.AutoMigrate(&repocommercial.OrderRow{}, &repocommercial.PaymentAttemptRow{},
		&repocommercial.OutboxEvent{}, &repocommercial.RefundRow{}, &repocommercial.RefundAllocationRow{},
		&FulfillmentRecord{}); err != nil {
		t.Fatal(err)
	}
	seedPaidOrder(t, repocommercial.NewOrderStore(db), rfOrderID, rfTenant, 100_00)
	// Relative to the REAL clock: EffectiveAt 24h ago makes the seeded
	// period already effective (the review-basis assertion depends on it).
	now := time.Now().UTC()
	if err := db.Create(&FulfillmentRecord{
		Key: domain.FulfillmentKey(rfOrderID, rfLot), TenantID: rfTenant, OrderID: rfOrderID,
		LineID: rfLot, Kind: domain.BenefitKindTopUp, CustomerID: OrderCustomerID(rfTenant),
		Credits: rfLotMicro, State: domain.FulfillmentStateApplied, ExternalID: "om-rf-1",
		EffectiveAt: now.Add(-24 * time.Hour), ExpiresAt: now.Add(365 * 24 * time.Hour),
	}).Error; err != nil {
		t.Fatal(err)
	}
	svc, err := NewRefundService(db, gw, provider, eligibility)
	if err != nil {
		t.Fatal(err)
	}
	sp, _ := provider.(*stubRefundProvider)
	return svc, db, sp
}

func newRefundRequest(t *testing.T, svc *RefundService, fen int64, micro int64) string {
	t.Helper()
	st, err := svc.CreateRequest(context.Background(), rfTenant, rfOrderID, rfLot, domain.CNYFen(fen), domain.Credits(micro))
	if err != nil {
		t.Fatal(err)
	}
	if st.State != domain.RefundStateRequested {
		t.Fatalf("new refund state %q, want requested", st.State)
	}
	return st.ID
}

func refundRow(t *testing.T, db *gorm.DB, id string) repocommercial.RefundRow {
	t.Helper()
	var row repocommercial.RefundRow
	if err := db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatal(err)
	}
	return row
}

func lockedSum(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var sum int64
	if err := db.Table("commercial_refund_allocations").
		Select("COALESCE(SUM(locked_micro),0)").Scan(&sum).Error; err != nil {
		t.Fatal(err)
	}
	return sum
}

// TestRefundApproveWithoutP03KeepsReviewing pins the dependency honesty:
// until the P03 occupancy-refundable check exists, Approve never approves
// into a paid-out state — the refund stays reviewing with the reason and
// the manual basis durably recorded, and NO locks or payout events exist.
func TestRefundApproveWithoutP03KeepsReviewing(t *testing.T) {
	svc, db, _ := setupRefund(t, &stubGateway{}, nil, nil) // default = not ready
	ctx := context.Background()
	id := newRefundRequest(t, svc, 40_00, 40_000_000)

	err := svc.Approve(ctx, id, "reviewer_a")
	if !errors.Is(err, domain.ErrRefundNotReady) {
		t.Fatalf("want ErrRefundNotReady, got %v", err)
	}
	row := refundRow(t, db, id)
	if row.State != domain.RefundStateReviewing {
		t.Fatalf("state %q, want reviewing", row.State)
	}
	if row.ReviewNote == "" || row.Reviewer != "reviewer_a" {
		t.Fatalf("review basis not recorded: %+v", row)
	}
	if row.ReviewBasis != domain.RefundBasisPeriodEffective {
		t.Fatalf("manual basis %q, want period_effective for an already-effective period", row.ReviewBasis)
	}
	if got := lockedSum(t, db); got != 0 {
		t.Fatalf("locks written without eligibility: %d", got)
	}
	var events int64
	db.Table("commercial_outbox_events").Where("kind = ?", repocommercial.OutboxKindRefundPayout).Count(&events)
	if events != 0 {
		t.Fatalf("payout event emitted without eligibility: %d", events)
	}
	// Replay stays honest: still reviewing, still no payout.
	if err := svc.Approve(ctx, id, "reviewer_b"); !errors.Is(err, domain.ErrRefundNotReady) {
		t.Fatalf("replay want ErrRefundNotReady, got %v", err)
	}
	if refundRow(t, db, id).State != domain.RefundStateReviewing {
		t.Fatal("replay escaped reviewing")
	}
}

// TestRefundApproveConcurrentUsageRaceNoDoubleSpend is the real-DB
// concurrency race: two partial refunds (60% each) and one usage admission
// (50%) race against the same 100-Credit lot. Every admission runs through
// the SAME admission-transaction coordinator seam, so the held locks can
// never exceed the lot — no double-spend of locked credits — and at most
// one of the two overlapping partial refunds wins.
func TestRefundApproveConcurrentUsageRaceNoDoubleSpend(t *testing.T) {
	svc, db, _ := setupRefund(t, &stubGateway{}, nil, readyEligibility{})
	ctx := context.Background()
	idA := newRefundRequest(t, svc, 60_00, 60_000_000)
	idB := newRefundRequest(t, svc, 60_00, 60_000_000)

	const racers = 3
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, racers)
	wg.Add(racers)
	go func() { defer wg.Done(); <-start; errs <- svc.Approve(ctx, idA, "rev_a") }()
	go func() { defer wg.Done(); <-start; errs <- svc.Approve(ctx, idB, "rev_b") }()
	go func() {
		defer wg.Done()
		<-start
		errs <- svc.refunds.AdmitLotLock(ctx, rfTenant, "usage_race",
			map[string]int64{domain.FulfillmentKey(rfOrderID, rfLot): 50_000_000})
	}()
	close(start)
	wg.Wait()
	close(errs)
	var approveWins int
	for err := range errs {
		if err == nil {
			approveWins++
		}
	}
	if locked := lockedSum(t, db); locked > rfLotMicro {
		t.Fatalf("double-spend: locked %d exceeds lot %d", locked, rfLotMicro)
	}
	rowA, rowB := refundRow(t, db, idA), refundRow(t, db, idB)
	wins := 0
	for _, row := range []repocommercial.RefundRow{rowA, rowB} {
		switch row.State {
		case domain.RefundStatePending:
			wins++
		case domain.RefundStateReviewing, domain.RefundStateRequested:
			if row.State == domain.RefundStateReviewing && row.ReviewNote == "" {
				t.Fatalf("losing refund %s has no recorded reason", row.ID)
			}
		default:
			t.Fatalf("refund %s in unexpected state %s", row.ID, row.State)
		}
	}
	if wins > 1 {
		t.Fatalf("overlapping partial refunds both approved: %d", wins)
	}
	if wins == 1 {
		approved := rowA
		if rowB.State == domain.RefundStatePending {
			approved = rowB
		}
		allocs, err := svc.refunds.ListRefundAllocations(ctx, approved.ID)
		if err != nil {
			t.Fatal(err)
		}
		var held int64
		for _, a := range allocs {
			held += a.LockedMicro
		}
		if held != 60_000_000 {
			t.Fatalf("approved refund holds %d micro, want exactly 60000000", held)
		}
	}
	if approveWins == 0 && lockedSum(t, db) == 0 {
		t.Fatal("no admission succeeded at all — race coordinator refused everything")
	}
}

// TestRefundSuccessRevokeFailureRetriesRevocationOnly proves the recovery
// invariant: after a successful channel payout whose precise-credits
// revocation fails, recovery retries ONLY the revocation — the channel
// refund is never issued a second time, and the refund completes only
// after the revocation is confirmed.
func TestRefundSuccessRevokeFailureRetriesRevocationOnly(t *testing.T) {
	gw := &stubGateway{}
	gw.setRevokeErr(errors.New("openmeter down"))
	provider := &stubRefundProvider{refundState: payment.StateSucceeded}
	svc, db, sp := setupRefund(t, gw, provider, readyEligibility{})
	ctx := context.Background()
	id := newRefundRequest(t, svc, 30_00, 30_000_000)
	if err := svc.Approve(ctx, id, "rev_c"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessPayouts(ctx); err != nil {
		t.Fatal(err)
	}
	row := refundRow(t, db, id)
	if row.State != domain.RefundStateRevocationPending {
		t.Fatalf("state %q, want revocation_pending after payout with failed revocation", row.State)
	}
	if row.ProviderRefundID == nil || *row.ProviderRefundID == "" {
		t.Fatal("channel settlement identity not recorded")
	}
	if locked := lockedSum(t, db); locked != 30_000_000 {
		t.Fatalf("locks released too early: %d", locked)
	}
	if n, _, _ := sp.counts(); n != 1 {
		t.Fatalf("channel refunded %d times, want exactly 1", n)
	}

	// Recovery with the gateway healthy: manual retry revokes again.
	gw.setRevokeErr(nil)
	if err := svc.RetryRevocation(ctx, id); err != nil {
		t.Fatal(err)
	}
	row = refundRow(t, db, id)
	if row.State != domain.RefundStateCompleted {
		t.Fatalf("state %q, want completed after confirmed revocation", row.State)
	}
	if n, _, _ := sp.counts(); n != 1 {
		t.Fatalf("recovery paid out again: %d channel refunds", n)
	}
	if gw.revokeCount() < 2 {
		t.Fatalf("revocation retried %d times, want >= 2 (failed attempt + retry)", gw.revokeCount())
	}
	if locked := lockedSum(t, db); locked != 0 {
		t.Fatalf("locks not settled at completion: %d", locked)
	}
}

// TestRefundUnknownChannelRequeriesOriginalKey pins the unknown path: an
// indeterminate channel result parks the refund in reviewing and the next
// pass re-queries by the ORIGINAL refund key — the refund key is never
// replaced and no second payout is attempted.
func TestRefundUnknownChannelRequeriesOriginalKey(t *testing.T) {
	provider := &stubRefundProvider{refundState: payment.StateUnknown, queryState: payment.StateSucceeded}
	svc, db, sp := setupRefund(t, &stubGateway{}, provider, readyEligibility{})
	ctx := context.Background()
	id := newRefundRequest(t, svc, 10_00, 10_000_000)
	if err := svc.Approve(ctx, id, "rev_d"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessPayouts(ctx); err != nil {
		t.Fatal(err)
	}
	row := refundRow(t, db, id)
	if row.State != domain.RefundStateReviewing {
		t.Fatalf("state %q after unknown channel outcome, want reviewing", row.State)
	}
	if n, q, _ := sp.counts(); n != 1 || q != 0 {
		t.Fatalf("refund calls %d query calls %d, want 1/0", n, q)
	}

	if err := svc.ProcessPayouts(ctx); err != nil {
		t.Fatal(err)
	}
	n, q, queryKeys := sp.counts()
	if n != 1 {
		t.Fatalf("re-query path re-issued the channel refund: %d calls", n)
	}
	if q != 1 || len(queryKeys) != 1 || queryKeys[0] != id {
		t.Fatalf("re-query used %v, want the ORIGINAL refund key %q", queryKeys, id)
	}
	row = refundRow(t, db, id)
	if row.State != domain.RefundStateCompleted {
		t.Fatalf("state %q after confirmed payout+revocation, want completed", row.State)
	}
}

// TestRefundFailedConfirmedReleasesLocks: only a CONFIRMED channel failure
// unlocks — the allocations are released and the state records why.
func TestRefundFailedConfirmedReleasesLocks(t *testing.T) {
	provider := &stubRefundProvider{refundState: payment.StateClosed} // confirmed failure
	svc, db, _ := setupRefund(t, &stubGateway{}, provider, readyEligibility{})
	ctx := context.Background()
	id := newRefundRequest(t, svc, 20_00, 20_000_000)
	if err := svc.Approve(ctx, id, "rev_e"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ProcessPayouts(ctx); err != nil {
		t.Fatal(err)
	}
	row := refundRow(t, db, id)
	if row.State != domain.RefundStateFailedConfirmed {
		t.Fatalf("state %q, want failed_confirmed", row.State)
	}
	if !domain.CanUnlockRefund(row.State) {
		t.Fatal("failed_confirmed must unlock")
	}
	if locked := lockedSum(t, db); locked != 0 {
		t.Fatalf("locks not released on confirmed failure: %d", locked)
	}
}
