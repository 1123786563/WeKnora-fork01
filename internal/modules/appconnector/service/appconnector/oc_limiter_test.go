package appconnector

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/modules/appconnector"
	repoappconn "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/commercial"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// T10: distributed dispatch slots — defaults, DB leases, fair space rotation,
// provider Retry-After — and the Execute integration (nil dispatcher,
// slots-before-budget, shared reservation on lost claim races).
// ---------------------------------------------------------------------------

func ocLimiterMigrationSQL(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("../../../../../migrations/sqlite/000043_open_connector_dispatch.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func ocLimiterDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000&_foreign_keys=1"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&repoappconn.ActionRow{}, &repoappconn.ApprovalRow{}, &repoappconn.AppVersion{}, &repoappconn.InstallationRow{}, &repoappconn.ConnectionRow{}); err != nil {
		t.Fatal(err)
	}
	bindings, err := os.ReadFile("../../../../../migrations/sqlite/000041_open_connector_bindings.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(bindings)).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(ocLimiterMigrationSQL(t)).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

// TestOCSlotsDefaultsArePositiveAndValidate pins ruling 6: the phase-one
// defaults tenant=4, connection=1, provider=16, global=32, and ANY non-positive
// limit is rejected before the limiter is ever built.
func TestOCSlotsDefaultsArePositiveAndValidate(t *testing.T) {
	def := DefaultOCSlotLimits()
	if def.PerTenant != 4 || def.PerConnection != 1 || def.PerProvider != 16 || def.Global != 32 {
		t.Fatalf("defaults = %+v", def)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("defaults rejected: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(*OCSlotLimits)
	}{
		{"zero tenant", func(c *OCSlotLimits) { c.PerTenant = 0 }},
		{"negative tenant", func(c *OCSlotLimits) { c.PerTenant = -1 }},
		{"zero connection", func(c *OCSlotLimits) { c.PerConnection = 0 }},
		{"zero provider", func(c *OCSlotLimits) { c.PerProvider = 0 }},
		{"zero global", func(c *OCSlotLimits) { c.Global = 0 }},
	} {
		c := def
		tc.mutate(&c)
		if err := c.Validate(); err == nil {
			t.Fatalf("%s: invalid config accepted", tc.name)
		}
	}
	db := ocLimiterDB(t)
	if _, err := NewOCSlotLimiter(repoappconn.NewOCStore(db), OCSlotLimits{PerTenant: 0}, "r1"); err == nil {
		t.Fatal("limiter built with invalid limits")
	}
	if _, err := NewOCSlotLimiter(repoappconn.NewOCStore(db), DefaultOCSlotLimits(), ""); err == nil {
		t.Fatal("limiter built without an owner")
	}
}

// TestOCSlotsAcquireReleaseAroundLimits: the connection limit (default 1)
// serializes dispatches per connection; a busy slot surfaces ErrOCSlotsBusy on
// context timeout and a release makes capacity available again.
func TestOCSlotsAcquireReleaseAroundLimits(t *testing.T) {
	db := ocLimiterDB(t)
	lim, err := NewOCSlotLimiter(repoappconn.NewOCStore(db), DefaultOCSlotLimits(), "replica-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	until := time.Now().Add(time.Minute)

	release, err := lim.AcquireOCSlots(ctx, 7, "conn-1", "github", "act-1", until)
	if err != nil {
		t.Fatal(err)
	}
	// Same connection, second dispatch: the slot is held, the caller times out.
	quick, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if _, err := lim.AcquireOCSlots(quick, 7, "conn-1", "github", "act-2", until); !errors.Is(err, ErrOCSlotsBusy) {
		t.Fatalf("second acquisition err = %v, want ErrOCSlotsBusy", err)
	}
	// A different connection on the same tenant still fits (tenant limit 4).
	releaseOther, err := lim.AcquireOCSlots(ctx, 7, "conn-2", "github", "act-3", until)
	if err != nil {
		t.Fatal(err)
	}
	if err := releaseOther(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := release(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := lim.AcquireOCSlots(ctx, 7, "conn-1", "github", "act-4", until); err != nil {
		t.Fatalf("released slot not reusable: %v", err)
	}
}

// TestOCSlotsQueueRotatesSpaces: the in-process waiting queue serves spaces
// round-robin — one waiter per space per rotation, never FIFO-bursts of a
// single space.
func TestOCSlotsQueueRotatesSpaces(t *testing.T) {
	var q ocSlotQueue
	wA1 := q.enter("7")
	wA2 := q.enter("7")
	wB1 := q.enter("8")
	if !q.anyWaiting() {
		t.Fatal("queue reports no waiters")
	}
	var order []string
	for i := 0; i < 3; i++ {
		w, ok := q.next()
		if !ok {
			t.Fatalf("wake %d: no waiter", i)
		}
		order = append(order, w.space)
	}
	if len(order) != 3 || order[0] != "7" || order[1] != "8" || order[2] != "7" {
		t.Fatalf("serve order = %v, want 7,8,7 (round-robin by space)", order)
	}
	q.leave(wA1)
	q.leave(wA2)
	q.leave(wB1)
	if q.anyWaiting() {
		t.Fatal("queue not empty after leave")
	}
}

// TestOCSlotsFairProgressAcrossSpacesUnderContention: with a shared provider
// scope of one slot and two spaces continuously contending, both spaces
// complete and neither starves; grants alternate while both are waiting.
func TestOCSlotsFairProgressAcrossSpacesUnderContention(t *testing.T) {
	db := ocLimiterDB(t)
	lim, err := NewOCSlotLimiter(repoappconn.NewOCStore(db), OCSlotLimits{PerTenant: 4, PerConnection: 1, PerProvider: 1, Global: 32}, "replica-1")
	if err != nil {
		t.Fatal(err)
	}
	lim.retryEvery = 10 * time.Second // release-driven wakes only: deterministic rotation

	var mu sync.Mutex
	var grants []uint64
	hold := make(chan struct{})
	acquired := make(chan uint64, 4)

	run := func(tenant uint64, conn string) {
		release, err := lim.AcquireOCSlots(context.Background(), tenant, conn, "github", fmt.Sprintf("act-%d", tenant), time.Now().Add(time.Minute))
		if err != nil {
			t.Errorf("space %d starved: %v", tenant, err)
			return
		}
		mu.Lock()
		grants = append(grants, tenant)
		mu.Unlock()
		acquired <- tenant
		<-hold // hold the only provider slot until driven
		if err := release(context.Background()); err != nil {
			t.Errorf("release: %v", err)
		}
	}
	go run(7, "conn-7")
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("first grant never happened")
	}
	// Two contenders queue, the OTHER space first so the queue order is
	// deterministic: ring = [8, 7]. Both are parked before any wake.
	go run(8, "conn-8")
	time.Sleep(100 * time.Millisecond)
	go run(7, "conn-7b")
	time.Sleep(200 * time.Millisecond)
	close(hold) // release the held slot; the rotation serves 8 then 7

	deadline := time.After(5 * time.Second)
	for served := 1; served < 3; served++ {
		select {
		case <-acquired:
		case <-deadline:
			mu.Lock()
			order := append([]uint64(nil), grants...)
			mu.Unlock()
			t.Fatalf("only %d grants after rotation, order so far %v", served, order)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(grants) != 3 {
		t.Fatalf("grants = %v, want 3 acquisitions", grants)
	}
	// The two QUEUED waiters (grants[1:]) must alternate spaces even though
	// the holding space queued its own second request — round-robin by
	// space, never a burst of one tenant.
	if grants[1] == grants[2] {
		t.Fatalf("queued grants did not alternate: %v", grants)
	}
}

// TestOCSlotsProviderThrottleBlocksAcquire: a stored provider Retry-After
// fails new acquisitions closed (no queueing behind a provider that said
// stop) until the window passes.
func TestOCSlotsProviderThrottleBlocksAcquire(t *testing.T) {
	db := ocLimiterDB(t)
	store := repoappconn.NewOCStore(db)
	lim, err := NewOCSlotLimiter(store, DefaultOCSlotLimits(), "replica-1")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now()
	if err := store.NoteOCProviderRetryAfter(ctx, "github", now.Add(30*time.Second), now); err != nil {
		t.Fatal(err)
	}
	if _, err := lim.AcquireOCSlots(ctx, 7, "conn-1", "github", "act-1", now.Add(time.Minute)); !errors.Is(err, ErrOCProviderThrottled) {
		t.Fatalf("throttled acquire err = %v", err)
	}
	lim.now = func() time.Time { return now.Add(31 * time.Second) }
	if _, err := lim.AcquireOCSlots(ctx, 7, "conn-1", "github", "act-1", now.Add(time.Minute)); err != nil {
		t.Fatalf("acquire after cooldown: %v", err)
	}
}

// TestParseOCRetryAfter: delay-seconds and HTTP-date forms, bounded, with
// garbage rejected.
func TestParseOCRetryAfter(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	if _, err := ParseOCRetryAfter("", now); err == nil {
		t.Fatal("empty header accepted")
	}
	if _, err := ParseOCRetryAfter("soon", now); err == nil {
		t.Fatal("garbage accepted")
	}
	at, err := ParseOCRetryAfter("30", now)
	if err != nil || !at.Equal(now.Add(30*time.Second)) {
		t.Fatalf("seconds form = %v %v", at, err)
	}
	httpDate := now.Add(time.Hour).Format("Mon, 02 Jan 2006 15:04:05 GMT")
	at, err = ParseOCRetryAfter(httpDate, now)
	if err != nil || !at.Equal(now.Add(time.Hour)) {
		t.Fatalf("http-date form = %v %v", at, err)
	}
	// A date in the past clamps to "now", an absurd horizon is bounded.
	at, err = ParseOCRetryAfter("Mon, 02 Jan 2006 15:04:05 GMT", now)
	if err != nil || at.Before(now) {
		t.Fatalf("past date = %v %v", at, err)
	}
	at, err = ParseOCRetryAfter("8640000", now)
	if err != nil || at.After(now.Add(25*time.Hour)) {
		t.Fatalf("unbounded retry-after = %v %v", at, err)
	}
}

// ---------------------------------------------------------------------------
// Execute integration
// ---------------------------------------------------------------------------

type stableGate struct {
	mu           sync.Mutex
	begins       int
	finishes     int
	reservations map[string]string // budget key -> reservation id (stable per Action ID)
	order        []string
}

func (g *stableGate) Begin(ctx context.Context, req commercial.BudgetRequest) (commercial.Reservation, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.reservations == nil {
		g.reservations = map[string]string{}
	}
	id, ok := g.reservations[req.Key]
	if !ok {
		g.begins++
		id = fmt.Sprintf("res-%s", req.Key)
		g.reservations[req.Key] = id
	}
	g.order = append(g.order, id)
	return commercial.Reservation{ID: id, Upper: req.Upper}, nil
}

func (g *stableGate) Finish(ctx context.Context, reservationID string, fact commercial.UsageFact) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.finishes++
	return nil
}

func (g *stableGate) stats() (begins, finishes int, distinct []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	seen := map[string]bool{}
	for _, id := range g.order {
		if !seen[id] {
			seen[id] = true
			distinct = append(distinct, id)
		}
	}
	return g.begins, g.finishes, distinct
}

func ocBindingJSONForExecute() string {
	return "{\"RuntimeID\":\"rt-1\",\"Provider\":\"github\",\"ExternalID\":\"ext-42\",\"Alias\":\"alias-1\",\"ActionID\":\"send_message\",\"SchemaDigest\":\"sd-1\",\"BindingVersion\":1}"
}

// newOCExecuteEnv wires a full OC Execute stack over one sqlite database:
// action store, OC store (claims + leases), dispatcher, resolver, gate and
// guard, with the tenant's installation/connection/binding seeded.
func newOCExecuteEnv(t *testing.T) (*ActionService, *stubDispatcher, *stableGate, *repoappconn.OCStore, *gorm.DB) {
	t.Helper()
	db := ocLimiterDB(t)
	inst := repoappconn.NewInstallationStore(db)
	store := repoappconn.NewOCStore(db)
	ctx := context.Background()
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: "app-oc", Version: "1.0.0", State: appconn.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	if err := inst.SaveConnection(ctx, appconn.Connection{
		ID: "conn-oc", InstallationID: "inst-7", Kind: appconn.ConnectionKindSpace,
		OwnerID: "owner", CredentialRef: "cred/x", State: appconn.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveBinding(ctx, appconn.OCBinding{
		TenantID: 7, ConnectionID: "conn-oc", RuntimeID: "rt-1", Provider: "github",
		ExternalID: "ext-42", Alias: "alias-1", AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive,
	}); err != nil {
		t.Fatal(err)
	}
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "ok"}}
	gate := &stableGate{}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, gate, disp, nil)
	svc.UseOCDispatchClaims(store)
	return svc, disp, gate, store, db
}

// ocSeedAuthorizedOCAction inserts one authorized open-connector action with
// a live approval, exactly like a PrepareOC + Approve pair would have.
func ocSeedAuthorizedOCAction(t *testing.T, db *gorm.DB, id string) {
	t.Helper()
	row := repoappconn.ActionRow{
		ID: id, TenantID: 7, ActorID: "alice", ConnectionID: "conn-oc",
		AppVersion: "1.0.0", Target: "send_message", Risk: appconn.RiskSend,
		AuthVersion: 1, ArgsSnapshot: "{\"body\":\"hi\"}", ArgsDigest: "dg-" + id,
		State: appconn.ActionAuthorized, OCBindingJSON: ocBindingJSONForExecute(), DigestVersion: 2,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	ap := repoappconn.ApprovalRow{ArgsDigest: row.ArgsDigest, ActionID: id, Actor: "alice", Expiry: time.Now().Add(time.Hour).UTC(), Remaining: 5}
	if err := db.Create(&ap).Error; err != nil {
		t.Fatal(err)
	}
}

// TestExecuteWithoutDispatcherFailsClosed is the T04-QF-6 carry: with no
// dispatcher wired, Execute fails closed BEFORE consuming anything — no
// approval, no budget, no state change, and above all no panic.
func TestExecuteWithoutDispatcherFailsClosed(t *testing.T) {
	db := ocLimiterDB(t)
	inst := repoappconn.NewInstallationStore(db)
	ctx := context.Background()
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: "app-oc", Version: "1.0.0", State: appconn.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	gate := &stableGate{}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, gate, nil, nil) // nil dispatcher, nil resolver
	ocSeedAuthorizedOCAction(t, db, "act-nil")

	err := svc.Execute(ctx, "act-nil")
	if err == nil {
		t.Fatal("execute without dispatcher returned nil")
	}
	if !errors.Is(err, ErrNoDispatcher) {
		t.Fatalf("err = %v, want ErrNoDispatcher", err)
	}
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", "act-nil").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionAuthorized {
		t.Fatalf("state = %s, want authorized (nothing consumed)", row.State)
	}
	var ap repoappconn.ApprovalRow
	if err := db.Where("action_id = ?", "act-nil").First(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if ap.Remaining != 5 {
		t.Fatalf("remaining = %d, want untouched", ap.Remaining)
	}
	if begins, _, _ := gate.stats(); begins != 0 {
		t.Fatalf("budget begins = %d, want 0", begins)
	}
}

// TestExecuteOCSlotsAcquiredBeforeBudgetBegin: when the slot lease cannot be
// acquired, Execute stops BEFORE Begin — a throttled dispatch consumes no
// approval and no budget.
func TestExecuteOCSlotsAcquiredBeforeBudgetBegin(t *testing.T) {
	svc, _, gate, store, db := newOCExecuteEnv(t)
	lim, err := NewOCSlotLimiter(store, DefaultOCSlotLimits(), "replica-1")
	if err != nil {
		t.Fatal(err)
	}
	svc.UseOCSlotLimiter(lim)
	ctx := context.Background()
	ocSeedAuthorizedOCAction(t, db, "act-slots")

	// Saturate the connection scope (limit 1) so Execute's own acquisition fails.
	block, err := lim.AcquireOCSlots(ctx, 7, "conn-oc", "github", "act-other", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	defer block(context.Background())

	quick, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if err := svc.Execute(quick, "act-slots"); err == nil {
		t.Fatal("execute succeeded without a slot")
	}
	if begins, _, _ := gate.stats(); begins != 0 {
		t.Fatalf("budget consumed without a slot: begins = %d", begins)
	}
	var ap repoappconn.ApprovalRow
	if err := db.Where("action_id = ?", "act-slots").First(&ap).Error; err != nil {
		t.Fatal(err)
	}
	if ap.Remaining != 5 {
		t.Fatalf("approval consumed without a slot: %d", ap.Remaining)
	}
}

// TestExecuteOCClaimRacingRequestsShareReservation: two racing Executes on
// one action produce ONE dispatch and ONE settlement; the loser fails with
// the claim error, reads the winner's reservation and never cancels it (the
// Begin key is Action-ID-stable, so both hold the SAME reservation).
func TestExecuteOCClaimRacingRequestsShareReservation(t *testing.T) {
	svc, disp, gate, _, db := newOCExecuteEnv(t)
	ctx := context.Background()
	ocSeedAuthorizedOCAction(t, db, "act-race")

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs[i] = svc.Execute(ctx, "act-race")
		}(i)
	}
	close(start)
	wg.Wait()

	var success int
	for _, err := range errs {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successes = %d, want exactly 1 (errs = %v / %v)", success, errs[0], errs[1])
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("dispatch calls = %d, want exactly 1", calls)
	}
	begins, finishes, distinct := gate.stats()
	if begins != 1 || len(distinct) != 1 {
		t.Fatalf("begins = %d distinct = %v, want one stable reservation", begins, distinct)
	}
	if finishes != 1 {
		t.Fatalf("finishes = %d, want exactly 1 (loser must not cancel the shared reservation)", finishes)
	}
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", "act-race").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionSucceeded {
		t.Fatalf("final state = %s, want succeeded", row.State)
	}
	var rec int64
	db.Table("connector_dispatch_records").Where("tenant_id = ? AND action_id = ?", 7, "act-race").Count(&rec)
	if rec != 1 {
		t.Fatalf("dispatch records = %d, want 1", rec)
	}
}

// TestExecuteOCWithoutClaimsFallsBackToFrozenPath: with the claim store not
// wired (transitional wiring), an OC action keeps the frozen claim path — it
// dispatches exactly once but leaves NO durable idempotency record; the
// durable claim activates with UseOCDispatchClaims.
func TestExecuteOCWithoutClaimsFallsBackToFrozenPath(t *testing.T) {
	db := ocLimiterDB(t)
	inst := repoappconn.NewInstallationStore(db)
	ctx := context.Background()
	if err := inst.ApplyInstallation(ctx, appconn.Installation{ID: "inst-7", AppID: "app-oc", Version: "1.0.0", State: appconn.InstallationActive, TenantID: 7}, 0); err != nil {
		t.Fatal(err)
	}
	disp := &stubDispatcher{outcome: DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "ok"}}
	svc := NewActionService(repoappconn.NewActionStore(db), &stubGuard{}, &stableGate{}, disp, nil)
	ocSeedAuthorizedOCAction(t, db, "act-nowire")

	if err := svc.Execute(ctx, "act-nowire"); err != nil {
		t.Fatal(err)
	}
	if calls, _ := disp.stats(); calls != 1 {
		t.Fatalf("dispatch calls = %d, want 1", calls)
	}
	var row repoappconn.ActionRow
	if err := db.Where("id = ?", "act-nowire").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.State != appconn.ActionSucceeded {
		t.Fatalf("state = %s, want succeeded", row.State)
	}
	var recs int64
	db.Table("connector_dispatch_records").Where("tenant_id = ?", 7).Count(&recs)
	if recs != 0 {
		t.Fatalf("unwired path wrote durable records: %d", recs)
	}
}

// TestExecuteOCRecordsDurableKeyAndSettles: the happy OC path writes the
// durable dispatch record with the operation-scoped key and settles once.
func TestExecuteOCRecordsDurableKeyAndSettles(t *testing.T) {
	svc, _, gate, store, db := newOCExecuteEnv(t)
	ctx := context.Background()
	ocSeedAuthorizedOCAction(t, db, "act-ok")

	if err := svc.Execute(ctx, "act-ok"); err != nil {
		t.Fatal(err)
	}
	rec, err := store.GetOCDispatch(ctx, 7, "act-ok")
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Key) < len("wk-oc-") || rec.Key[:len("wk-oc-")] != "wk-oc-" {
		t.Fatalf("key = %q, want wk-oc- prefixed", rec.Key)
	}
	if rec.RuntimeID != "rt-1" || rec.ReservationID == "" {
		t.Fatalf("record = %+v", rec)
	}
	if begins, finishes, _ := gate.stats(); begins != 1 || finishes != 1 {
		t.Fatalf("begins = %d finishes = %d, want 1/1", begins, finishes)
	}
}
