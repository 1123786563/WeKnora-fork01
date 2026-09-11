package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	repocommercial "github.com/Tencent/WeKnora/internal/application/repository/commercial"
	domain "github.com/Tencent/WeKnora/internal/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Every now/anchor in this file is an explicit time.Time value: no process
// time manipulation is used to fake elapsed months or external boundaries.
func testLifecycle(t *testing.T, issuer Issuer) (*LifecycleService, *repocommercial.SubscriptionStore) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if s, err := db.DB(); err == nil {
		s.SetMaxOpenConns(1) // serialize SQLite writers; claims still race logically
	}
	if err := db.AutoMigrate(&repocommercial.Subscription{}, &repocommercial.BenefitJob{}); err != nil {
		t.Fatal(err)
	}
	store := repocommercial.NewSubscriptionStore(db)
	svc, err := NewLifecycleService(store, issuer)
	if err != nil {
		t.Fatal(err)
	}
	return svc, store
}

func seedSubscription(t *testing.T, store *repocommercial.SubscriptionStore, id string, tenant uint64, anchor, paidUntil time.Time, version int64) {
	t.Helper()
	plan := domain.PlanVersion{Key: "pro", Version: 3, Monthly: domain.Credits(1000), Limits: map[string]int64{"seats": 10}}
	blob, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	err = store.SaveSubscription(context.Background(), &repocommercial.Subscription{
		ID: id, TenantID: tenant, PlanKey: plan.Key, PlanVersion: plan.Version,
		PlanSnapshotJSON: string(blob), Anchor: anchor, PaidUntil: paidUntil, Version: version,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func listJobs(t *testing.T, store *repocommercial.SubscriptionStore) []repocommercial.BenefitJob {
	t.Helper()
	jobs, err := store.ListBenefitJobs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return jobs
}

func TestLifecycleTickSamePeriodMultipleWorkersClaimOnce(t *testing.T) {
	svc, store := testLifecycle(t, Issuer{})
	ctx := context.Background()
	anchor := time.Date(2028, 1, 5, 9, 0, 0, 0, time.UTC)
	seedSubscription(t, store, "sub_c", 42, anchor, domain.MonthBoundary(anchor, 6), 1)
	now := domain.MonthBoundary(anchor, 2) // months at anchor, +1, +2 are due

	const workers = 8
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- svc.Tick(ctx, now)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	jobs := listJobs(t, store)
	if len(jobs) != 3 {
		t.Fatalf("exactly one job per elapsed month, got %d", len(jobs))
	}
	seen := map[string]int{}
	for _, j := range jobs {
		seen[j.Key]++
		if j.Key != domain.MonthlyGrantKey("sub_c", j.MonthStart) {
			t.Fatalf("job key %q does not match monthly key namespace", j.Key)
		}
	}
	for key, n := range seen {
		if n != 1 {
			t.Fatalf("key %q claimed %d times", key, n)
		}
	}
}

func TestLifecycleTickReplayKeepsExternalPendingJobAndRef(t *testing.T) {
	svc, store := testLifecycle(t, Issuer{})
	ctx := context.Background()
	anchor := time.Date(2028, 2, 10, 9, 0, 0, 0, time.UTC)
	seedSubscription(t, store, "sub_x", 43, anchor, domain.MonthBoundary(anchor, 6), 1)
	key := domain.MonthlyGrantKey("sub_x", anchor)
	// The month was granted externally but not confirmed locally: the job is
	// pending with the external batch reference already stored.
	claimed, err := store.ClaimBenefitJob(ctx, repocommercial.BenefitJob{
		Key: key, TenantID: 43, SubscriptionID: "sub_x", MonthStart: anchor,
		Credits: 1000, State: repocommercial.JobStatePending, ExternalRef: "batch_ext_9",
	})
	if err != nil || !claimed {
		t.Fatalf("seed claim failed: claimed=%v err=%v", claimed, err)
	}

	now := anchor.Add(24 * time.Hour)
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	jobs := listJobs(t, store)
	if len(jobs) != 1 {
		t.Fatalf("replay must keep a single job, got %d", len(jobs))
	}
	if jobs[0].Key != key || jobs[0].State != repocommercial.JobStatePending || jobs[0].ExternalRef != "batch_ext_9" {
		t.Fatalf("job mutated on replay: %+v", jobs[0])
	}
}

func TestLifecycleTickAnnualGrantsMonthByMonthAcrossTicks(t *testing.T) {
	svc, store := testLifecycle(t, Issuer{})
	ctx := context.Background()
	anchor := time.Date(2028, 2, 10, 9, 0, 0, 0, time.UTC)
	seedSubscription(t, store, "sub_annual", 44, anchor, domain.MonthBoundary(anchor, 12), 1)

	for _, step := range []struct{ elapsed, want int }{{0, 1}, {1, 2}, {5, 6}} {
		elapsed, want := step.elapsed, step.want
		now := anchor
		if elapsed > 0 {
			now = domain.MonthBoundary(anchor, elapsed)
		}
		if err := svc.Tick(ctx, now); err != nil {
			t.Fatal(err)
		}
		if got := len(listJobs(t, store)); got != want {
			t.Fatalf("after %d elapsed months want %d jobs, got %d", elapsed, want, got)
		}
	}
	if got := len(listJobs(t, store)); got > 6 {
		t.Fatalf("annual term must never be granted all at once, got %d jobs", got)
	}
}

func TestLifecycleTickEarlyRenewalDoesNotPreGrantFutureMonths(t *testing.T) {
	svc, store := testLifecycle(t, Issuer{})
	ctx := context.Background()
	anchor := time.Date(2028, 3, 10, 9, 0, 0, 0, time.UTC)
	seedSubscription(t, store, "sub_early", 45, anchor, domain.MonthBoundary(anchor, 1), 1)
	now := anchor.Add(24 * time.Hour)
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if got := len(listJobs(t, store)); got != 1 {
		t.Fatalf("want 1 job before renewal, got %d", got)
	}

	// Early renewal extends paid_until a full year past the current term.
	seedSubscription(t, store, "sub_early", 45, anchor, domain.MonthBoundary(anchor, 13), 2)
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if got := len(listJobs(t, store)); got != 1 {
		t.Fatalf("renewal must not pre-grant future months at the same now, got %d", got)
	}
	if err := svc.Tick(ctx, domain.MonthBoundary(anchor, 2)); err != nil {
		t.Fatal(err)
	}
	if got := len(listJobs(t, store)); got != 3 {
		t.Fatalf("only elapsed months are granted after renewal, got %d", got)
	}
	for _, j := range listJobs(t, store) {
		if j.MonthStart.UTC().After(domain.MonthBoundary(anchor, 2).UTC()) {
			t.Fatalf("future month %v granted early", j.MonthStart)
		}
	}
}

func TestLifecycleTickExpiredKeepsTopUpsAndDowngradesProjection(t *testing.T) {
	svc, store := testLifecycle(t, Issuer{})
	ctx := context.Background()
	anchor := time.Date(2028, 5, 1, 9, 0, 0, 0, time.UTC)
	paidUntil := domain.MonthBoundary(anchor, 1)
	seedSubscription(t, store, "sub_exp", 46, anchor, paidUntil, 1)
	// One granted monthly job and one purchased top-up (order_line_id
	// namespace, separate from monthly keys) that must survive expiry.
	if _, err := store.ClaimBenefitJob(ctx, repocommercial.BenefitJob{
		Key: domain.MonthlyGrantKey("sub_exp", anchor), TenantID: 46, SubscriptionID: "sub_exp",
		MonthStart: anchor, Credits: 1000, State: repocommercial.JobStateGranted,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimBenefitJob(ctx, repocommercial.BenefitJob{
		Key: "sub_exp/upgrade/ol_topup_1", TenantID: 46, SubscriptionID: "sub_exp",
		MonthStart: anchor, Credits: 5000, State: repocommercial.JobStateGranted,
		ExternalRef: "batch_topup", OrderLineID: "ol_topup_1",
	}); err != nil {
		t.Fatal(err)
	}

	if err := svc.Tick(ctx, paidUntil); err != nil { // now == paid_until -> expired
		t.Fatal(err)
	}
	jobs := listJobs(t, store)
	if len(jobs) != 2 {
		t.Fatalf("expiry must not create or delete jobs, got %d", len(jobs))
	}
	for _, j := range jobs {
		if j.State != repocommercial.JobStateGranted {
			t.Fatalf("purchased rows degraded on expiry: %+v", j)
		}
		if j.Key == "sub_exp/upgrade/ol_topup_1" && (j.ExternalRef != "batch_topup" || j.OrderLineID != "ol_topup_1") {
			t.Fatalf("top-up row mutated on expiry: %+v", j)
		}
	}
	subs, err := store.ListSubscriptions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("subscription row deleted on expiry: %d rows", len(subs))
	}
	if subs[0].DowngradeReason != domain.DowngradeReasonOverLimit {
		t.Fatalf("downgrade reason not recorded: %+v", subs[0])
	}
	var projected domain.PlanVersion
	if err := json.Unmarshal([]byte(subs[0].ProjectionPlanJSON), &projected); err != nil {
		t.Fatal(err)
	}
	if projected.Key != domain.BaseTier.Key {
		t.Fatalf("projection is not the base tier: %+v", projected)
	}
}

func TestLifecycleTickExternalIssuerSavesBatchIDAndReplaysOnce(t *testing.T) {
	var calls int
	issuer := Issuer{Kind: IssuerExternal, GrantMonth: func(ctx context.Context, sub domain.Subscription, monthStart time.Time) (string, error) {
		calls++
		return "batch_ok_1", nil
	}}
	svc, store := testLifecycle(t, issuer)
	ctx := context.Background()
	anchor := time.Date(2028, 7, 3, 9, 0, 0, 0, time.UTC)
	seedSubscription(t, store, "sub_ext", 47, anchor, domain.MonthBoundary(anchor, 3), 1)
	now := anchor.Add(24 * time.Hour)
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("external issuer called %d times, want 1", calls)
	}
	jobs := listJobs(t, store)
	if len(jobs) != 1 || jobs[0].State != repocommercial.JobStateGranted || jobs[0].ExternalRef != "batch_ok_1" {
		t.Fatalf("external batch not saved: %+v", jobs)
	}
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("replay re-issued the same month: %d calls", calls)
	}
	if jobs = listJobs(t, store); len(jobs) != 1 || jobs[0].ExternalRef != "batch_ok_1" {
		t.Fatalf("replay mutated the job: %+v", jobs)
	}
}

func TestLifecycleTickExternalIssuerFailureKeepsKeyStable(t *testing.T) {
	var calls int
	boom := errors.New("provider down")
	issuer := Issuer{Kind: IssuerExternal, GrantMonth: func(ctx context.Context, sub domain.Subscription, monthStart time.Time) (string, error) {
		calls++
		return "", boom
	}}
	svc, store := testLifecycle(t, issuer)
	ctx := context.Background()
	anchor := time.Date(2028, 9, 3, 9, 0, 0, 0, time.UTC)
	seedSubscription(t, store, "sub_fail", 48, anchor, domain.MonthBoundary(anchor, 3), 1)
	now := anchor.Add(24 * time.Hour)
	if err := svc.Tick(ctx, now); !errors.Is(err, boom) {
		t.Fatalf("want provider error, got %v", err)
	}
	key := domain.MonthlyGrantKey("sub_fail", anchor)
	job, err := store.GetBenefitJob(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if job.Key != key || job.State != repocommercial.JobStateFailed || job.ExternalRef != "" {
		t.Fatalf("failed issuance must keep the key and record failure: %+v", job)
	}
	if err := svc.Tick(ctx, now); err != nil {
		t.Fatalf("replay of a claimed-but-failed key must be a no-op, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("claimed key re-issued: %d calls", calls)
	}
	if job, err = store.GetBenefitJob(ctx, key); err != nil || job.Key != key {
		t.Fatalf("key changed after replay: %+v err=%v", job, err)
	}
}
