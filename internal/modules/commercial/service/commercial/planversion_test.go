package commercial

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/modules/commercial"
	repocommercial "github.com/Tencent/WeKnora/internal/modules/commercial/repository/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// spyPlatform is the scripted CommercialPlatform stub: it records every
// submitted command (byte-comparable on retry) and answers from a script
// indexed by call number (nil script = always succeed).
type spyPlatform struct {
	mu       sync.Mutex
	submits  int
	commands []domain.Command
	script   func(call int) (domain.CommandReceipt, error)
}

func (s *spyPlatform) SubmitCommand(_ context.Context, cmd domain.Command) (domain.CommandReceipt, error) {
	s.mu.Lock()
	s.submits++
	call := s.submits
	s.commands = append(s.commands, cmd)
	s.mu.Unlock()
	if s.script != nil {
		return s.script(call)
	}
	return domain.CommandReceipt{Key: cmd.Key, ExternalID: "weknora-spy", RecordedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC)}, nil
}

func (s *spyPlatform) ReadSnapshot(context.Context, domain.SnapshotQuery) (domain.Snapshot, error) {
	return domain.Snapshot{}, domain.ErrPlatformUnsupported
}

func (s *spyPlatform) Reconcile(context.Context, domain.ReconciliationCursor) (domain.ReconciliationPage, error) {
	return domain.ReconciliationPage{}, domain.ErrPlatformUnsupported
}

func (s *spyPlatform) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.submits
}

func (s *spyPlatform) recorded() []domain.Command {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.Command(nil), s.commands...)
}

// testPlanVersionService builds the sqlite harness: stores bootstrapped via
// EnsureSchema, the subscriptions table AutoMigrated for the non-impact
// proof, and the scripted platform injected.
func testPlanVersionService(t *testing.T, platform domain.CommercialPlatform) (*PlanVersionService, *gorm.DB, *spyPlatform) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&repocommercial.Subscription{}); err != nil {
		t.Fatal(err)
	}
	svc, err := NewPlanVersionService(db, platform)
	if err != nil {
		t.Fatal(err)
	}
	spy, _ := platform.(*spyPlatform)
	return svc, db, spy
}

func validDraft(planKey string) DraftInput {
	return DraftInput{
		PlanKey:              planKey,
		Name:                 "Pro",
		AmountFen:            9900,
		IncludedCreditsMicro: 9_900_000,
		Features:             map[string]bool{"api_access": true},
		Limits:               map[string]int64{"members": 10},
		Currency:             "CNY",
	}
}

func mustCreateDraft(t *testing.T, svc *PlanVersionService, planKey string) repocommercial.VersionView {
	t.Helper()
	view, err := svc.CreateDraft(context.Background(), "operator-1", validDraft(planKey))
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	return view
}

func mustPublish(t *testing.T, svc *PlanVersionService, planKey string, version int64) PublishResult {
	t.Helper()
	res, err := svc.Publish(context.Background(), "operator-1", "test", planKey, version)
	if err != nil {
		t.Fatalf("publish %s v%d: %v", planKey, version, err)
	}
	return res
}

// TestPublishHappyPath: draft → validate (all axes pass) → publish: the
// catalog row is published and the publication row carries the
// deterministic plan code plus the recorded receipt.
func TestPublishHappyPath(t *testing.T) {
	svc, _, _ := testPlanVersionService(t, &spyPlatform{})
	ctx := context.Background()

	view := mustCreateDraft(t, svc, "pro")
	if view.State != domain.PlanStateDraft || view.Version != 1 {
		t.Fatalf("draft view = %+v", view)
	}
	report, err := svc.Validate(ctx, "pro", 1)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if !report.Valid {
		t.Fatalf("clean draft must validate, got %+v", report.Axes)
	}
	res := mustPublish(t, svc, "pro", 1)
	if res.Version.State != domain.PlanStatePublished {
		t.Fatalf("state = %q, want published", res.Version.State)
	}
	if res.Receipt == nil || !res.Receipt.Received || res.Receipt.CommandKey != domain.PublishCommandKey("pro", 1) {
		t.Fatalf("receipt = %+v", res.Receipt)
	}
	if res.Receipt.PublishedAt.IsZero() {
		t.Fatal("receipt PublishedAt must be set")
	}
	pub, err := svc.GetPublication(ctx, "pro", 1)
	if err != nil {
		t.Fatal(err)
	}
	if pub.PlanCode != domain.DeterministicPlanCode("pro", 1) {
		t.Fatalf("publication plan code = %q", pub.PlanCode)
	}
	var receipt domain.CommandReceipt
	if err := json.Unmarshal([]byte(pub.ReceiptJSON), &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.Key != domain.PublishCommandKey("pro", 1) {
		t.Fatalf("recorded receipt key = %q", receipt.Key)
	}
}

// TestPublishValidationFailsClosedStaysDraft: one failing axis per group —
// publish returns the itemized report, the row stays draft, and the seam
// sees ZERO commands.
func TestPublishValidationFailsClosedStaysDraft(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*DraftInput)
		axis string
	}{
		{"off-ladder price", func(in *DraftInput) { in.AmountFen = 12345 }, "base_price_tier"},
		{"non-CNY", func(in *DraftInput) { in.Currency = "USD" }, "currency_cny"},
		{"unknown feature", func(in *DraftInput) { in.Features = map[string]bool{"teleport": true} }, "entitlements"},
		{"negative limit", func(in *DraftInput) { in.Limits = map[string]int64{"members": -1} }, "resource_quota"},
		{"zero credits", func(in *DraftInput) { in.IncludedCreditsMicro = 0 }, "included_credits"},
		{"graduated charge", func(in *DraftInput) {
			in.Charges = []domain.PlanCharge{{Dimension: "model-units", Model: "graduated", AmountFen: 7}}
		}, "cost_upper_bound"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, spy := testPlanVersionService(t, &spyPlatform{})
			view := mustCreateDraft(t, svc, "pro")
			in := validDraft("pro")
			tc.mut(&in)
			if _, err := svc.UpdateDraft(context.Background(), "pro", view.Version, in); err != nil {
				t.Fatalf("update draft: %v", err)
			}
			res, err := svc.Publish(context.Background(), "operator-1", "r", "pro", view.Version)
			if !errors.Is(err, ErrPublishValidationFailed) {
				t.Fatalf("publish must fail validation-closed, got %v", err)
			}
			if res.Report.Valid {
				t.Fatal("report must be invalid")
			}
			found := false
			for _, axis := range res.Report.Axes {
				if axis.Axis == tc.axis && !axis.Passed {
					found = true
				}
			}
			if !found {
				t.Fatalf("report must itemize failed axis %s, got %+v", tc.axis, res.Report.Axes)
			}
			current, err := svc.GetVersion(context.Background(), "pro", view.Version)
			if err != nil {
				t.Fatal(err)
			}
			if current.State != domain.PlanStateDraft {
				t.Fatalf("failed validation must keep the row draft, got %q", current.State)
			}
			if spy.callCount() != 0 {
				t.Fatalf("an invalid draft must NEVER reach the seam, got %d calls", spy.callCount())
			}
		})
	}
}

// TestPublishIdempotentAcrossReplays: (a) a second Publish after success
// returns the recorded receipt with no seam call; (b) response loss — the
// first submit fails unreachable, the row stays publishing, and the retry
// replays the SAME command key and payload to success with exactly one
// publication row; (c) a definitive invalid response is the closed
// publish_conflict and the row stays publishing.
func TestPublishIdempotentAcrossReplays(t *testing.T) {
	ctx := context.Background()

	t.Run("second publish is a no-op replay", func(t *testing.T) {
		svc, _, spy := testPlanVersionService(t, &spyPlatform{})
		mustCreateDraft(t, svc, "pro")
		first := mustPublish(t, svc, "pro", 1)
		calls := spy.callCount()
		second := mustPublish(t, svc, "pro", 1)
		if spy.callCount() != calls {
			t.Fatalf("the replay must not call the seam again: %d -> %d", calls, spy.callCount())
		}
		if second.Receipt.CommandKey != first.Receipt.CommandKey ||
			!second.Receipt.PublishedAt.Equal(first.Receipt.PublishedAt) {
			t.Fatalf("the replay must return the SAME receipt: %+v vs %+v", first.Receipt, second.Receipt)
		}
	})

	t.Run("response loss retries the same identity", func(t *testing.T) {
		spy := &spyPlatform{}
		spy.script = func(call int) (domain.CommandReceipt, error) {
			if call == 1 {
				return domain.CommandReceipt{}, fmt.Errorf("%w: transport lost", domain.ErrPlatformUnreachable)
			}
			return domain.CommandReceipt{
				Key:        domain.PublishCommandKey("pro", 1),
				ExternalID: domain.DeterministicPlanCode("pro", 1),
				RecordedAt: time.Date(2026, 9, 21, 12, 0, 0, 0, time.UTC),
			}, nil
		}
		svc, _, _ := testPlanVersionService(t, spy)
		mustCreateDraft(t, svc, "pro")
		_, err := svc.Publish(ctx, "operator-1", "r", "pro", 1)
		if !errors.Is(err, domain.ErrPlatformUnreachable) {
			t.Fatalf("first call must surface unreachable, got %v", err)
		}
		view, err := svc.GetVersion(ctx, "pro", 1)
		if err != nil {
			t.Fatal(err)
		}
		if view.State != domain.PlanStatePublishing {
			t.Fatalf("response loss must leave the row publishing (retry with the SAME key), got %q", view.State)
		}
		res := mustPublish(t, svc, "pro", 1)
		if !res.Receipt.Received {
			t.Fatal("retry must record the receipt")
		}
		cmds := spy.recorded()
		if len(cmds) != 2 {
			t.Fatalf("want 2 seam calls (lost + retry), got %d", len(cmds))
		}
		if cmds[0].Key != cmds[1].Key || cmds[0].Key != domain.PublishCommandKey("pro", 1) {
			t.Fatalf("the retry must replay the SAME key: %q vs %q", cmds[0].Key, cmds[1].Key)
		}
		if !reflect.DeepEqual(cmds[0].Payload, cmds[1].Payload) {
			t.Fatalf("the retry must replay a byte-equal payload:\n%+v\n%+v", cmds[0].Payload, cmds[1].Payload)
		}
		pubs, err := svc.ListPublications(ctx, "pro")
		if err != nil {
			t.Fatal(err)
		}
		if len(pubs) != 1 {
			t.Fatalf("exactly one publication row may exist, got %d", len(pubs))
		}
	})

	t.Run("definitive invalid response is a closed conflict", func(t *testing.T) {
		spy := &spyPlatform{}
		spy.script = func(int) (domain.CommandReceipt, error) {
			return domain.CommandReceipt{}, fmt.Errorf("%w: content differs", domain.ErrPlatformInvalidResponse)
		}
		svc, _, _ := testPlanVersionService(t, spy)
		mustCreateDraft(t, svc, "pro")
		_, err := svc.Publish(ctx, "operator-1", "r", "pro", 1)
		if !errors.Is(err, domain.ErrPlatformInvalidResponse) {
			t.Fatalf("want the closed conflict (ErrPlatformInvalidResponse), got %v", err)
		}
		view, err := svc.GetVersion(ctx, "pro", 1)
		if err != nil {
			t.Fatal(err)
		}
		if view.State != domain.PlanStatePublishing {
			t.Fatalf("a conflict keeps the row publishing for operator attention, got %q", view.State)
		}
		if _, err := svc.GetPublication(ctx, "pro", 1); !errors.Is(err, repocommercial.ErrPublicationNotFound) {
			t.Fatalf("no publication may be fabricated, got %v", err)
		}
	})
}

// TestNewVersionNewCodeAndOldImmutable: v2 gets its own plan code, versions
// strictly increase, and the published v1 cannot be edited in place.
func TestNewVersionNewCodeAndOldImmutable(t *testing.T) {
	svc, _, _ := testPlanVersionService(t, &spyPlatform{})
	ctx := context.Background()

	mustCreateDraft(t, svc, "pro")
	mustPublish(t, svc, "pro", 1)

	second, err := svc.CreateDraft(ctx, "operator-1", validDraft("pro"))
	if err != nil {
		t.Fatal(err)
	}
	if second.Version != 2 {
		t.Fatalf("second draft version = %d, want 2", second.Version)
	}
	mustPublish(t, svc, "pro", 2)

	pubs, err := svc.ListPublications(ctx, "pro")
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, p := range pubs {
		codes[p.PlanCode] = true
	}
	if len(codes) != 2 {
		t.Fatalf("each version must have its OWN plan code, got %v", codes)
	}
	if !codes[domain.DeterministicPlanCode("pro", 1)] || !codes[domain.DeterministicPlanCode("pro", 2)] {
		t.Fatalf("codes must be the deterministic per-version codes, got %v", codes)
	}

	if _, err := svc.UpdateDraft(ctx, "pro", 1, validDraft("pro")); !errors.Is(err, repocommercial.ErrPublishedPlanImmutable) {
		t.Fatalf("UpdateDraft on published v1 must be immutable, got %v", err)
	}
}

// TestPublishNeverTouchesSubscriptions: the publish path performs NO writes
// to commercial_subscriptions — the seeded row is byte-identical afterwards
// and no other rows appeared.
func TestPublishNeverTouchesSubscriptions(t *testing.T) {
	svc, db, _ := testPlanVersionService(t, &spyPlatform{})
	ctx := context.Background()

	anchor := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	seeded := repocommercial.Subscription{
		ID: "sub-seed", TenantID: 7, PlanKey: "pro", PlanVersion: 1,
		PlanSnapshotJSON: `{"Key":"pro","Version":1}`, Anchor: anchor,
		PaidUntil: anchor.AddDate(0, 1, 0), FutureIntervalJSON: "{}", Version: 3,
		ProjectionPlanJSON: "", DowngradeReason: "",
	}
	if err := repocommercial.NewSubscriptionStore(db).SaveSubscription(ctx, &seeded); err != nil {
		t.Fatal(err)
	}
	before, err := db.Raw(`SELECT * FROM commercial_subscriptions WHERE id = 'sub-seed'`).Rows()
	if err != nil {
		t.Fatal(err)
	}
	cols, _ := before.Columns()
	var beforeRows [][]any
	for before.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := before.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		beforeRows = append(beforeRows, values)
	}
	before.Close()

	draft := mustCreateDraft(t, svc, "pro")
	if _, err := svc.Validate(ctx, "pro", draft.Version); err != nil {
		t.Fatal(err)
	}
	mustPublish(t, svc, "pro", draft.Version)

	after, err := db.Raw(`SELECT * FROM commercial_subscriptions`).Rows()
	if err != nil {
		t.Fatal(err)
	}
	var afterRows [][]any
	rowCount := 0
	for after.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := after.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		afterRows = append(afterRows, values)
		rowCount++
	}
	after.Close()
	if rowCount != 1 {
		t.Fatalf("no subscription rows may appear or disappear, got %d", rowCount)
	}
	if !reflect.DeepEqual(fmt.Sprint(beforeRows[0]), fmt.Sprint(afterRows[0])) {
		t.Fatalf("the seeded subscription must be byte-identical:\nbefore=%v\nafter=%v", beforeRows[0], afterRows[0])
	}
}

// TestNilPlatformFailsClosed: with no platform wired (blocked-env),
// draft/validate/list keep working and publish fails closed unconfigured —
// no state flip, no fabricated receipt.
func TestNilPlatformFailsClosed(t *testing.T) {
	svc, _, _ := testPlanVersionService(t, nil)
	ctx := context.Background()

	view := mustCreateDraft(t, svc, "pro")
	report, err := svc.Validate(ctx, "pro", view.Version)
	if err != nil || !report.Valid {
		t.Fatalf("draft/validate must work without a platform: %v %+v", err, report)
	}
	list, err := svc.ListVersions(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("list must work without a platform: %v %d", err, len(list))
	}
	_, err = svc.Publish(ctx, "operator-1", "r", "pro", 1)
	if !errors.Is(err, domain.ErrPlatformUnconfigured) {
		t.Fatalf("publish must fail closed unconfigured, got %v", err)
	}
	current, err := svc.GetVersion(ctx, "pro", 1)
	if err != nil {
		t.Fatal(err)
	}
	if current.State != domain.PlanStateDraft {
		t.Fatalf("nil platform must not flip state, got %q", current.State)
	}
	if _, err := svc.GetPublication(ctx, "pro", 1); !errors.Is(err, repocommercial.ErrPublicationNotFound) {
		t.Fatalf("no receipt may be fabricated, got %v", err)
	}
}

// TestConcurrentCreateDraftVersionsMonotonic: two concurrent draft creators
// land distinct, monotonic versions (PK retry inside the service).
func TestConcurrentCreateDraftVersionsMonotonic(t *testing.T) {
	svc, _, _ := testPlanVersionService(t, &spyPlatform{})
	const goroutines = 2
	var wg sync.WaitGroup
	versions := make([]int64, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			view, err := svc.CreateDraft(context.Background(), "operator-1", validDraft("pro"))
			if err == nil {
				versions[slot] = view.Version
			}
		}(i)
	}
	wg.Wait()
	seen := map[int64]bool{}
	for _, v := range versions {
		if v == 0 {
			t.Fatalf("a concurrent creator failed: %v", versions)
		}
		if seen[v] {
			t.Fatalf("versions must be distinct, got %v", versions)
		}
		seen[v] = true
	}
	if !seen[1] || !seen[2] {
		t.Fatalf("two creators must land v1 and v2 (gap-free), got %v", versions)
	}
}
