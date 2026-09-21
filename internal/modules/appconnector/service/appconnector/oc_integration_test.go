//go:build integration

// T17 acceptance: multi-space and fault-injection integration over REAL
// PostgreSQL and the REAL open-connector execution client.
//
// Environment contract (plan T17 / coordinator ruling 1):
//
//	OC_TEST_DATABASE_URL  PostgreSQL DSN of the DISPOSABLE acceptance database
//	                      (R8 container pattern: weknora-oc-t17-pg-<n>, free
//	                      port, never 5432; migrations 000117→000124 applied).
//	                      Missing → TestOCIntegrationEnvironment FAILS with
//	                      "blocked-env" — it never Skip-passes.
//
// The suite applies the REAL versioned PG migrations (000117..000124) into an
// isolated per-run schema of that database, so every constraint and FOR UPDATE
// / SKIP LOCKED behavior below is the production one. tenant_members (the
// membership table the A02 guard reads, migration 000043) is materialized from
// the frozen types.TenantMember model — the same AutoMigrate convention the
// repository suites use for parent tables.
//
// Two spaces, each Owner+Member, one cross-space member, one shared runtime,
// same Provider under two accounts (ruling 2). The PROVIDER is an HTTP fake
// with operation counters only — grants, OAuth correlation, approval digests,
// claims, idempotency keys and recovery all run through the REAL service,
// repository, recovery and container code paths (fake provider ≠ fake OC).
//
// Two service wirings are exercised deliberately:
//
//   - stack.prod — container.NewOCArmedActionService, the PRODUCTION
//     composition (F-1 ordering, FileBackedOCTokenSource, GatedOCClaims,
//     default slot limits). Used for the authorization surface (scenarios
//     1-4), the credentials scan (12) and the wiring-parity probe.
//   - stack.t12 — the same pieces with the RAW repository claim store wired
//     as the settle face (what T12 designed). This wiring predates the R20
//     fix (GatedOCClaims used to mask the settle face — T17 finding 1); the
//     wrapper now forwards it transparently, both wirings settle
//     identically, and TestOCIntegrationWiringSettleParity pins that parity.
//
// Every scenario appends structured evidence to an acceptance journal printed
// as OC17-EVIDENCE JSON lines (consumed by scripts/open-connector/
// acceptance-cases.json and docs/integrations/open-connector-acceptance.md).
package appconnector_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	appconn "github.com/Tencent/WeKnora/internal/appconnector"
	"github.com/Tencent/WeKnora/internal/appconnector/openconnector"
	"github.com/Tencent/WeKnora/internal/application/repository"
	repoappconn "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/application/service/appconnector"
	"github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/connectorcontrol"
	"github.com/Tencent/WeKnora/internal/container"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Environment gate — VERBATIM plan sketch (ruling 1: Fatal, never Skip)
// ---------------------------------------------------------------------------

func TestOCIntegrationEnvironment(t *testing.T) {
	dsn := os.Getenv("OC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("blocked-env: OC_TEST_DATABASE_URL required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// acceptance journal (ruling 2: per-scenario evidence recording)
// ---------------------------------------------------------------------------

type oc17Journal struct {
	mu     sync.Mutex
	events []map[string]any
}

func (j *oc17Journal) record(t *testing.T, scenario, event string, kv ...any) {
	t.Helper()
	entry := map[string]any{"scenario": scenario, "event": event, "at": time.Now().UTC().Format(time.RFC3339Nano)}
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok {
			entry[k] = kv[i+1]
		}
	}
	raw, _ := json.Marshal(entry)
	j.mu.Lock()
	j.events = append(j.events, entry)
	j.mu.Unlock()
	t.Logf("OC17-EVIDENCE %s", raw)
}

func (j *oc17Journal) dump(t *testing.T) {
	j.mu.Lock()
	defer j.mu.Unlock()
	raw, _ := json.MarshalIndent(j.events, "", " ")
	_ = raw // the per-event lines above are the machine-readable feed
}

// ---------------------------------------------------------------------------
// HTTP fake PROVIDER — op counters only; envelope shapes are the FROZEN T01
// wire contract (success/data/meta.executionId/meta.auditPersisted; failure
// errorCode). It never stands in for OC-side logic.
// ---------------------------------------------------------------------------

type oc17Op struct {
	Seq      int    `json:"seq"`
	Action   string `json:"action"`
	Alias    string `json:"alias"`
	Key      string `json:"key"`
	Auth     string `json:"auth"`
	Body     string `json:"body"`
	At       string `json:"at"`
	Broken   bool   `json:"broken,omitempty"`
	Released bool   `json:"released,omitempty"`
}

type oc17Provider struct {
	srv *httptest.Server

	mu       sync.Mutex
	mode     string
	ops      []oc17Op // SIDE EFFECTS: one per unique idempotency key
	replays  int      // arrivals absorbed by the upstream key contract
	byKey    map[string]oc17Op
	inflight int32
	peak     int32

	holdOnce sync.Once
	holdCh   chan struct{}
	arrived  chan struct{}
}

func newOC17Provider() *oc17Provider {
	p := &oc17Provider{mode: "ok", byKey: map[string]oc17Op{}, holdCh: make(chan struct{}), arrived: make(chan struct{}, 64)}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/actions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		action := strings.TrimPrefix(r.URL.Path, "/v1/actions/")
		p.mu.Lock()
		mode := p.mode
		seq := len(p.ops) + 1
		op := oc17Op{
			Seq: seq, Action: action, Alias: r.Header.Get("x-oo-connector-alias"),
			Key: r.Header.Get("Idempotency-Key"), Auth: r.Header.Get("Authorization"),
			Body: string(body), At: time.Now().UTC().Format(time.RFC3339Nano),
		}
		// Upstream idempotency contract (T01 fixtures/key_replay): a second
		// arrival under the SAME key REPLAYS the recorded outcome — never a
		// second side effect. Go's transport can re-send a request over a
		// fresh connection after the server cut the previous one; the frozen
		// upstream absorbs exactly this with the key.
		if stored, seen := p.byKey[op.Key]; seen {
			p.replays++
			p.mu.Unlock()
			if stored.Broken {
				hj, ok := w.(http.Hijacker)
				if ok {
					if conn, _, err := hj.Hijack(); err == nil {
						_ = conn.Close()
					}
				}
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"success":true,"message":"OK (replay)","data":{"seq":%d},"meta":{"executionId":"exec-%d","actionId":%q,"auditPersisted":true}}`, stored.Seq, stored.Seq, action)))
			return
		}
		p.mu.Unlock()

		cur := atomic.AddInt32(&p.inflight, 1)
		for {
			peak := atomic.LoadInt32(&p.peak)
			if cur <= peak || atomic.CompareAndSwapInt32(&p.peak, peak, cur) {
				break
			}
		}
		defer atomic.AddInt32(&p.inflight, -1)

		switch mode {
		case "break":
			// Provider performs the WRITE (records the op) then breaks the
			// response: hijack + close, exactly the T01 "write-then-cut"
			// window an unknown outcome models.
			p.mu.Lock()
			op.Broken = true
			p.ops = append(p.ops, op)
			p.byKey[op.Key] = op
			p.mu.Unlock()
			hj, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "no hijack", http.StatusInternalServerError)
				return
			}
			conn, _, err := hj.Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		case "hold":
			p.mu.Lock()
			op.Released = true // recorded ONCE at arrival; the release only unblocks
			p.ops = append(p.ops, op)
			p.byKey[op.Key] = op
			p.mu.Unlock()
			select {
			case p.arrived <- struct{}{}:
			default:
			}
			<-p.holdCh
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{"success":true,"message":"OK","data":{"seq":%d},"meta":{"executionId":"exec-%d","actionId":%q,"auditPersisted":true}}`, seq, seq, action)))
			return
		}

		p.mu.Lock()
		p.ops = append(p.ops, op)
		p.byKey[op.Key] = op
		p.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch mode {
		case "throttle":
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"success":false,"message":"rate limited","data":null,"errorCode":"rate_limited"}`))
		case "invalid":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"success":false,"message":"invalid input","data":null,"errorCode":"invalid_input"}`))
		default:
			_, _ = w.Write([]byte(fmt.Sprintf(`{"success":true,"message":"OK","data":{"seq":%d},"meta":{"executionId":"exec-%d","actionId":%q,"auditPersisted":true}}`, seq, seq, action)))
		}
	})
	p.srv = httptest.NewServer(mux)
	return p
}

func (p *oc17Provider) setMode(mode string) { p.mu.Lock(); p.mode = mode; p.mu.Unlock() }
func (p *oc17Provider) URL() string         { return p.srv.URL }
func (p *oc17Provider) close()              { p.srv.Close() }

// total counts SIDE EFFECTS — one per unique idempotency key (the frozen
// upstream contract replays the recorded outcome for a repeated key and
// never performs a second side effect).
func (p *oc17Provider) total() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.ops)
}

// replayCount counts arrivals absorbed by the upstream idempotency contract
// (e.g. Go's transport re-send after the server cut the connection).
func (p *oc17Provider) replayCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.replays
}

func (p *oc17Provider) opsSnapshot() []oc17Op {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]oc17Op, len(p.ops))
	copy(out, p.ops)
	return out
}

func (p *oc17Provider) peakInflight() int32 { return atomic.LoadInt32(&p.peak) }

func (p *oc17Provider) releaseHold() {
	// One-shot close: the hold channel is immutable after construction, so
	// the handler's read and this release cannot race.
	p.holdOnce.Do(func() { close(p.holdCh) })
}

func (p *oc17Provider) lookupByKey(key string) (oc17Op, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, op := range p.ops {
		// The side effect happened even when the response was cut (Broken):
		// a provider query must report the write, not the response.
		if op.Key == key {
			return op, true
		}
	}
	return oc17Op{}, false
}

// ---------------------------------------------------------------------------
// instrumented commercial gate (the declared U05 seam; the billing boundary
// itself stays the frozen commercial.ExecutionGate interface)
// ---------------------------------------------------------------------------

type oc17Gate struct {
	mu          sync.Mutex
	begins      int
	finishCalls int
	finishFails int
	failFinishN int // first N Finish calls fail (local billing failure)
	facts       []commercial.UsageFact
	finishedOK  map[string]int
}

func newOC17Gate() *oc17Gate { return &oc17Gate{finishedOK: map[string]int{}} }

func (g *oc17Gate) Begin(ctx context.Context, req commercial.BudgetRequest) (commercial.Reservation, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.begins++
	return commercial.Reservation{ID: fmt.Sprintf("res-%d", g.begins), Upper: req.Upper}, nil
}

func (g *oc17Gate) Finish(ctx context.Context, reservationID string, fact commercial.UsageFact) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.finishCalls++
	if g.failFinishN > 0 {
		g.failFinishN--
		g.finishFails++
		return errors.New("oc17: simulated settlement outage")
	}
	g.facts = append(g.facts, fact)
	g.finishedOK[fact.CallID]++
	return nil
}

func (g *oc17Gate) stats() (begins, calls, fails int, facts []commercial.UsageFact) {
	g.mu.Lock()
	defer g.mu.Unlock()
	facts = append([]commercial.UsageFact(nil), g.facts...)
	return g.begins, g.finishCalls, g.finishFails, facts
}

func (g *oc17Gate) okFactsFor(callID string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.finishedOK[callID]
}

// ---------------------------------------------------------------------------
// map-backed SpaceGrantSource — ONLY for the scenario-3 control case (prove
// the space-connection denial is grant-driven); the production container
// wiring passes nil grants (space connections fail closed), which the main
// guard mirrors.
// ---------------------------------------------------------------------------

type mapGrants struct {
	granted map[string]bool // "tenant|connection|actor"
}

func (m mapGrants) SpaceConnectionGranted(ctx context.Context, tenantID uint64, connectionID, actorID string) (bool, error) {
	return m.granted[fmt.Sprintf("%d|%s|%s", tenantID, connectionID, actorID)], nil
}

// ---------------------------------------------------------------------------
// PG harness: isolated schema + the REAL versioned migrations 000117..000124
// ---------------------------------------------------------------------------

var oc17Migrations = []string{
	"000117_app_installations.up.sql",
	"000118_mcp_oauth_binding_states.up.sql",
	"000119_app_actions.up.sql",
	"000120_app_datasource_bindings.up.sql",
	"000121_open_connector_bindings.up.sql",
	"000122_open_connector_actions.up.sql",
	"000123_open_connector_dispatch.up.sql",
	"000124_open_connector_tool_bindings.up.sql",
}

// oc17OpenDB opens the acceptance PostgreSQL, provisions an isolated schema,
// applies the REAL versioned migrations from the repo checkout and registers
// the fixture-manifest cleanup (drop THIS schema only).
func oc17OpenDB(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	dsn := os.Getenv("OC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("blocked-env: OC_TEST_DATABASE_URL required")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard, NowFunc: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("open admin: %v", err)
	}
	schema := fmt.Sprintf("oc17_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Errorf("drop isolated schema %s: %v", schema, err)
		}
		sqlDB, _ := admin.DB()
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})
	dsnSchema := dsn
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		if strings.Contains(dsn, "?") {
			dsnSchema = dsn + "&search_path=" + schema
		} else {
			dsnSchema = dsn + "?search_path=" + schema
		}
	} else {
		// pgx keyword/value DSN: space-separated settings.
		dsnSchema = dsn + " search_path=" + schema
	}
	db, err := gorm.Open(postgres.Open(dsnSchema), &gorm.Config{Logger: gormlogger.Discard, NowFunc: func() time.Time { return time.Now().UTC() }})
	if err != nil {
		t.Fatalf("open isolated schema: %v", err)
	}
	for _, m := range oc17Migrations {
		raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "migrations", "versioned", m))
		if err != nil {
			t.Fatalf("read migration %s: %v", m, err)
		}
		if err := db.Exec(string(raw)).Error; err != nil {
			t.Fatalf("apply migration %s: %v", m, err)
		}
	}
	// Parent membership table (000043): materialize from the frozen model —
	// the same AutoMigrate convention the repository suites use.
	if err := db.AutoMigrate(&types.TenantMember{}); err != nil {
		t.Fatalf("tenant_members: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(20)
	return db, schema
}

// ---------------------------------------------------------------------------
// stack assembly
// ---------------------------------------------------------------------------

const (
	oc17TenantA      = uint64(91701)
	oc17TenantB      = uint64(91702)
	oc17Runtime      = "rt-oc17-shared"
	oc17ProviderName = "github"
	oc17ActionID     = "github.send_demo"
	oc17AppVer       = "1.0.0"
)

var (
	oc17OwnerA  = appconn.OCSubject{TenantID: oc17TenantA, ActorID: "owner-a"}
	oc17MemberA = appconn.OCSubject{TenantID: oc17TenantA, ActorID: "member-a1"}
	oc17AdminA  = appconn.OCSubject{TenantID: oc17TenantA, ActorID: "admin-a"}
	oc17OwnerB  = appconn.OCSubject{TenantID: oc17TenantB, ActorID: "owner-b"}
	oc17MemberB = appconn.OCSubject{TenantID: oc17TenantB, ActorID: "member-b1"}
)

type oc17Stack struct {
	db          *gorm.DB
	schema      string
	ocStore     *repoappconn.OCStore
	installs    *repoappconn.InstallationStore
	actionStore *repoappconn.ActionStore
	catalog     *appconnectorsvc.OCCatalog
	preparer    *appconnectorsvc.OCPreparer
	connSvc     *appconnectorsvc.OCConnectionService
	prod        *appconnectorsvc.ActionService // container.NewOCArmedActionService wiring
	t12         *appconnectorsvc.ActionService // raw-claims wiring (settle-face parity cross-check)
	gate        *oc17Gate
	provider    *oc17Provider
	tokenDir    string
	journal     *oc17Journal
}

// oc17SeedTokenFile writes one restricted runtime-token file the way the
// control worker's FileSecretSink does (sha256(ref).secret, plaintext legacy
// sink — the container falls back to it when no key file is provisioned).
func oc17SeedTokenFile(dir string, tenant uint64, connectionID string, authVersion int64, material string) error {
	ref := container.OCTokenRef(tenant, connectionID, authVersion)
	sum := sha256.Sum256([]byte(ref))
	name := hex.EncodeToString(sum[:]) + ".secret"
	return os.WriteFile(filepath.Join(dir, name), []byte(material), 0o600)
}

// oc17Sink builds the plaintext legacy secret sink the container falls back
// to when no encryption key file is provisioned (dev/test read-compat).
func oc17Sink(t *testing.T, dir string) *connectorcontrol.FileSecretSink {
	t.Helper()
	sink, err := connectorcontrol.NewFileSecretSink(dir)
	if err != nil {
		t.Fatal(err)
	}
	return sink
}

func oc17BuildStack(t *testing.T) *oc17Stack {
	t.Helper()
	db, schema := oc17OpenDB(t)
	ocStore := repoappconn.NewOCStore(db)
	installs := repoappconn.NewInstallationStore(db)
	actionStore := repoappconn.NewActionStore(db)
	provider := newOC17Provider()
	tokenDir := t.TempDir()
	gate := newOC17Gate()
	j := &oc17Journal{}
	t.Cleanup(func() { provider.close(); j.dump(t) })

	// Production credential source (repository.MCPOAuthBindingStore): the
	// guard's membership/connection reads are the production code path.
	src := repository.NewMCPOAuthBindingStore(db)
	installations := appconnectorsvc.NewInstallationStateSource(installs)
	// Production parity: nil grants (container.go wiring) — space connections
	// fail closed until a grant source exists.
	guard := appconnectorsvc.NewOCSubjectGuard(src, installations, nil, ocStore)

	// PRODUCTION composition (F-1 ordering, token source, gated claims,
	// default slot limits) — the exact constructor the container registers.
	prod, _, err := container.NewOCArmedActionService(
		actionStore, guard, gate, ocStore,
		container.OCConfig{
			Enabled:     true,
			RuntimeAddr: provider.URL(),
			TokenDir:    tokenDir,
			SlotOwner:   "oc17-replica-prod",
		},
		&http.Client{Timeout: 30 * time.Second},
	)
	if err != nil {
		t.Fatalf("production wiring: %v", err)
	}

	// T12-parity wiring: identical pieces with the RAW claim store wired as
	// the settle face (kept from the T17-F1 era as a standing cross-check —
	// the R20 fix made GatedOCClaims forward the settle face, so both
	// wirings settle identically; see TestOCIntegrationWiringSettleParity).
	// Built in the same order as PrepareOpenConnector: token source →
	// executor → dispatcher → limiter.
	tokens, err := container.NewFileBackedOCTokenSource(oc17Sink(t, tokenDir), ocStore)
	if err != nil {
		t.Fatal(err)
	}
	exec, err := openconnector.NewClient(provider.URL(), &http.Client{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	inner := appconnectorsvc.NewOCDispatcher(exec, ocStore, tokens, ocStore)
	inner.UseOCRetryAfterSink(ocStore)
	slots, err := appconnectorsvc.NewOCSlotLimiter(ocStore, appconnectorsvc.DefaultOCSlotLimits(), "oc17-replica-t12")
	if err != nil {
		t.Fatal(err)
	}
	t12 := appconnectorsvc.NewActionService(actionStore, guard, gate, inner, nil)
	t12.UseOCDispatchClaims(ocStore)
	t12.UseOCSlotLimiter(slots)

	catalog := appconnectorsvc.NewOCCatalog(ocStore, ocStore, installs, installs)
	preparer := appconnectorsvc.NewOCPreparer(t12, catalog, ocStore)
	connSvc := appconnectorsvc.NewOCConnectionService(src, installs, ocStore)

	return &oc17Stack{
		db: db, schema: schema, ocStore: ocStore, installs: installs,
		actionStore: actionStore, catalog: catalog, preparer: preparer, connSvc: connSvc,
		prod: prod, t12: t12,
		gate: gate, provider: provider, tokenDir: tokenDir, journal: j,
	}
}

// oc17SeedFixture materializes the two-space topology (ruling 2):
// two spaces, each Owner+Member, one cross-space member, one shared runtime,
// same Provider under two accounts.
func oc17SeedFixture(t *testing.T, s *oc17Stack) {
	t.Helper()
	ctx := context.Background()
	members := []types.TenantMember{
		{UserID: "owner-a", TenantID: oc17TenantA, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()},
		{UserID: "member-a1", TenantID: oc17TenantA, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()},
		{UserID: "admin-a", TenantID: oc17TenantA, Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()},
		{UserID: "owner-b", TenantID: oc17TenantB, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()},
		{UserID: "member-b1", TenantID: oc17TenantB, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()},
		// ONE cross-space member: member-a1 also belongs to space B.
		{UserID: "member-a1", TenantID: oc17TenantB, Role: types.TenantRoleContributor, Status: types.TenantMemberStatusActive, JoinedAt: time.Now().UTC()},
	}
	for i := range members {
		if err := s.db.Create(&members[i]).Error; err != nil {
			t.Fatalf("seed member %+v: %v", members[i], err)
		}
	}
	// Shared runtime registry row (the T05/T16 registry the control plane owns).
	if err := s.db.Create(&repoappconn.OCRuntimeRow{ID: oc17Runtime, Address: s.provider.URL(), Enabled: true, Version: "v1.5.0", ImageDigest: "sha256:" + strings.Repeat("a", 64)}).Error; err != nil {
		t.Fatal(err)
	}
	for _, inst := range []appconn.Installation{
		{ID: "inst-a", AppID: oc17ProviderName, Version: oc17AppVer, State: appconn.InstallationActive, TenantID: oc17TenantA},
		{ID: "inst-b", AppID: oc17ProviderName, Version: oc17AppVer, State: appconn.InstallationActive, TenantID: oc17TenantB},
	} {
		if err := s.installs.ApplyInstallation(ctx, inst, 0); err != nil {
			t.Fatalf("seed installation %+v: %v", inst, err)
		}
	}
	for _, c := range []appconn.Connection{
		{ID: "conn-a1", InstallationID: "inst-a", Kind: appconn.ConnectionKindPersonal, OwnerID: "owner-a", State: appconn.ConnectionActive, TenantID: oc17TenantA, AuthVersion: 1},
		{ID: "conn-a-space", InstallationID: "inst-a", Kind: appconn.ConnectionKindSpace, OwnerID: "owner-a", State: appconn.ConnectionActive, TenantID: oc17TenantA, AuthVersion: 1},
		{ID: "conn-b1", InstallationID: "inst-b", Kind: appconn.ConnectionKindPersonal, OwnerID: "owner-b", State: appconn.ConnectionActive, TenantID: oc17TenantB, AuthVersion: 1},
	} {
		if err := s.installs.SaveConnection(ctx, c); err != nil {
			t.Fatalf("seed connection %+v: %v", c, err)
		}
	}
	// Same Provider, two accounts: distinct external ids + aliases on the one
	// shared runtime.
	for _, b := range []appconn.OCBinding{
		{TenantID: oc17TenantA, ConnectionID: "conn-a1", RuntimeID: oc17Runtime, Provider: oc17ProviderName, ExternalID: "ext-account-a", Alias: "alias-account-a", AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive},
		{TenantID: oc17TenantA, ConnectionID: "conn-a-space", RuntimeID: oc17Runtime, Provider: oc17ProviderName, ExternalID: "ext-aspace", Alias: "alias-aspace", AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive},
		{TenantID: oc17TenantB, ConnectionID: "conn-b1", RuntimeID: oc17Runtime, Provider: oc17ProviderName, ExternalID: "ext-account-b", Alias: "alias-account-b", AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive},
	} {
		if err := s.ocStore.SaveBinding(ctx, b); err != nil {
			t.Fatalf("seed binding %+v: %v", b, err)
		}
	}
	// The reviewed, PUBLISHED definition (platform control-plane operation).
	schema := `{"type":"object","properties":{"target":{"type":"string"},"body":{"type":"string"}},"required":["body"],"additionalProperties":false}`
	if err := s.catalog.PublishOCDefinition(ctx, appconn.OCDefinition{
		AppID: oc17ProviderName, AppVersion: oc17AppVer, ActionID: oc17ActionID,
		Provider: oc17ProviderName, Risk: appconn.RiskSend, InputSchema: json.RawMessage(schema),
		RequiredScopes: []string{"demo:send"}, Published: true,
	}); err != nil {
		t.Fatalf("publish definition: %v", err)
	}
	// Restricted runtime tokens (as the control worker minted them) + ONE
	// admin-secret file that must never reach a dispatch.
	for _, tk := range []struct {
		tenant uint64
		conn   string
		val    string
	}{
		{oc17TenantA, "conn-a1", "tok-A1-v1-restricted"},
		{oc17TenantA, "conn-a-space", "tok-AS-v1-restricted"},
		{oc17TenantB, "conn-b1", "tok-B1-v1-restricted"},
	} {
		if err := oc17SeedTokenFile(s.tokenDir, tk.tenant, tk.conn, 1, tk.val); err != nil {
			t.Fatal(err)
		}
	}
	// Control-plane-only material under a NON-dispatch ref (wrong connection).
	if err := oc17SeedTokenFile(s.tokenDir, oc17TenantA, "conn-controlplane-only", 1, "oc17-admin-secret-do-not-dispatch"); err != nil {
		t.Fatal(err)
	}
}

// oc17ConnVersion reads the fixture connection's CURRENT authorization
// generation (the revoke/restore scenarios move it forward).
func oc17ConnVersion(t *testing.T, s *oc17Stack) int64 {
	t.Helper()
	var v int64
	if err := s.db.Raw("SELECT auth_version FROM connections WHERE tenant_id=? AND id='conn-a1'", oc17TenantA).Scan(&v).Error; err != nil {
		t.Fatal(err)
	}
	return v
}

// oc17RestoreConnA1 re-arms the shared fixture connection (active, v1) for
// the scenarios that follow the revoke-based ones.
func oc17RestoreConnA1(t *testing.T, s *oc17Stack) {
	t.Helper()
	// A FRESH authorization generation (well clear of every prior revoke
	// generation, whose deterministic cleanup-outbox ids must never collide)
	// plus its restricted token file.
	var cur int64
	if err := s.db.Raw("SELECT auth_version FROM connections WHERE tenant_id=? AND id='conn-a1'", oc17TenantA).Scan(&cur).Error; err != nil {
		t.Fatal(err)
	}
	nv := cur + 10
	if err := s.db.Exec("UPDATE connections SET state='active', auth_version=? WHERE tenant_id=? AND id='conn-a1'", nv, oc17TenantA).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Exec("UPDATE connector_connection_bindings SET state='active', auth_version=?, binding_version=binding_version+1 WHERE tenant_id=? AND connection_id='conn-a1'", nv, oc17TenantA).Error; err != nil {
		t.Fatal(err)
	}
	if err := oc17SeedTokenFile(s.tokenDir, oc17TenantA, "conn-a1", nv, fmt.Sprintf("tok-A1-v%d-restricted", nv)); err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func oc17Args(body string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"target":"demo://%s","body":%q}`, body, body))
}

// oc17PrepareApprove runs the trusted prepare + a human approval and returns
// the action id (the "request ID" of the acceptance evidence).
func oc17PrepareApprove(t *testing.T, s *oc17Stack, subject appconn.OCSubject, connectionID, body string) string {
	t.Helper()
	ctx := context.Background()
	id, err := s.preparer.PrepareOC(ctx, subject, connectionID, oc17ActionID, oc17Args(body))
	if err != nil {
		t.Fatalf("PrepareOC: %v", err)
	}
	row, err := s.actionStore.FindAction(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.t12.Approve(ctx, id, subject.ActorID, row.ArgsDigest); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	return id
}

func oc17Exec(s *oc17Stack, ctx context.Context, actionID string) error {
	return s.t12.Execute(ctx, actionID)
}

// oc17ActionDBState captures the before/after DB evidence of one action.
func oc17ActionDBState(t *testing.T, s *oc17Stack, actionID string) map[string]any {
	t.Helper()
	ctx := context.Background()
	row, err := s.actionStore.FindAction(ctx, actionID)
	if err != nil {
		return map[string]any{"action": "missing"}
	}
	state := map[string]any{
		"action_state": row.State, "action_fence": row.Fence,
		"actor": row.ActorID, "connection": row.ConnectionID,
		"auth_version": row.AuthVersion, "digest": row.ArgsDigest,
		"digest_version": row.DigestVersion,
	}
	if rec, err := s.ocStore.GetOCDispatch(ctx, row.TenantID, actionID); err == nil {
		state["record_state"] = rec.State
		state["record_key"] = rec.Key
		state["record_fence"] = rec.Fence
		state["record_runtime"] = rec.RuntimeID
		state["record_execution"] = rec.ExecutionID
		state["record_reservation"] = rec.ReservationID
		state["record_first_sent_at"] = rec.FirstSentAt.UTC().Format(time.RFC3339Nano)
		state["record_replay_until"] = rec.ReplayUntil.UTC().Format(time.RFC3339Nano)
	} else {
		state["record_state"] = "none"
	}
	var ap repoappconn.ApprovalRow
	if err := s.db.Where("action_id = ?", actionID).First(&ap).Error; err == nil {
		state["approval_remaining"] = ap.Remaining
		state["approval_expiry"] = ap.Expiry.UTC().Format(time.RFC3339Nano)
	}
	return state
}

func oc17BindingState(t *testing.T, s *oc17Stack, tenant uint64, connectionID string) map[string]any {
	t.Helper()
	b, err := s.ocStore.GetBinding(context.Background(), tenant, connectionID)
	if err != nil {
		return map[string]any{"binding": "missing"}
	}
	return map[string]any{"state": b.State, "auth_version": b.AuthVersion, "binding_version": b.BindingVersion, "alias": b.Alias, "external": b.ExternalID}
}

func oc17WaitFor(t *testing.T, what string, tick func() bool, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if tick() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", what)
}

// ---------------------------------------------------------------------------
// TestOCIntegration — the 14-scenario matrix (ruling 2). Scenario 14
// (regression) is the recorded command evidence in the acceptance doc/JSON.
// ---------------------------------------------------------------------------

func TestOCIntegration(t *testing.T) {
	s := oc17BuildStack(t)
	oc17SeedFixture(t, s)
	ctx := context.Background()
	s.journal.record(t, "meta", "fixture", "schema", s.schema, "runtime", oc17Runtime,
		"provider_url", s.provider.URL(), "migrations", strings.Join(oc17Migrations, ","))

	// --- 1. cross-space: B uses A's connection/action/attempt id -----------
	t.Run("cross_space", func(t *testing.T) {
		before := s.provider.total()
		actionID := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "cross-space-probe")

		// B's member prepares against A's connection: no binding in tenant B
		// for that connection id → unreachable (denied before anything else).
		_, err := s.preparer.PrepareOC(ctx, oc17MemberB, "conn-a1", oc17ActionID, oc17Args("b-uses-a-conn"))
		s.journal.record(t, "cross_space", "prepare_B_on_A_connection", "actionRef", actionID,
			"err", fmt.Sprintf("%v", err), "denied", err != nil)
		if !errors.Is(err, appconnectorsvc.ErrOCDefinitionUnreachable) {
			t.Fatalf("PrepareOC by space-B member on A connection: err=%v, want ErrOCDefinitionUnreachable", err)
		}

		// B's member starts an authorization attempt on A's connection: the
		// connection belongs to another tenant → forbidden, no attempt row.
		_, err = s.connSvc.Begin(ctx, oc17MemberB, "conn-a1")
		s.journal.record(t, "cross_space", "begin_B_on_A_connection", "err", fmt.Sprintf("%v", err), "denied", err != nil)
		if !errors.Is(err, appconnectorsvc.ErrConnectionForbidden) {
			t.Fatalf("Begin by space-B member on A connection: err=%v, want ErrConnectionForbidden", err)
		}

		// A's OWN attempt, then B tries to complete it: the frozen completion
		// gate requires the persisted subject to match.
		attemptID, err := s.connSvc.Begin(ctx, oc17OwnerA, "conn-a1")
		if err != nil {
			t.Fatal(err)
		}
		att, err := s.ocStore.GetOCAttemptByID(ctx, attemptID)
		if err != nil {
			t.Fatal(err)
		}
		stolen := appconnectorsvc.CanCompleteOCAttempt(att, oc17MemberB, att.Alias, att.AuthVersion, time.Now())
		s.journal.record(t, "cross_space", "attempt_completion_by_B", "attempt_id", attemptID, "completed_by_B", stolen)
		if stolen {
			t.Fatal("space-B member completed space-A authorization attempt")
		}

		// B claims A's action id through the durable claim store: tenant-scoped
		// predicate → not found for tenant B (indistinguishable from missing).
		_, err = s.ocStore.ClaimOCDispatch(ctx, oc17MemberB, actionID, "res-cross")
		s.journal.record(t, "cross_space", "claim_B_on_A_action", "actionRef", actionID, "err", fmt.Sprintf("%v", err), "denied", err != nil)
		if err == nil {
			t.Fatal("space-B member claimed space-A action")
		}

		// The A02 guard itself rejects B's subject on A's connection before
		// any credential or token is even considered.
		guard := appconnectorsvc.NewOCSubjectGuard(repository.NewMCPOAuthBindingStore(s.db),
			appconnectorsvc.NewInstallationStateSource(s.installs), nil, s.ocStore)
		err = guard.Check(ctx, oc17MemberB, "conn-a1", 1)
		s.journal.record(t, "cross_space", "a02_B_on_A_connection", "err", fmt.Sprintf("%v", err), "denied", err != nil)
		if !errors.Is(err, appconnectorsvc.ErrConnectionVersionStale) && !errors.Is(err, appconnectorsvc.ErrConnectionForbidden) {
			t.Fatalf("guard.Check for B on A connection: err=%v, want tenant-scope denial", err)
		}

		after := s.provider.total()
		s.journal.record(t, "cross_space", "provider_counters", "before", before, "after", after)
		if after != before {
			t.Fatalf("provider saw %d ops during denied cross-space attempts", after-before)
		}
	})

	// --- 2. personal connection: same-space non-owner ----------------------
	t.Run("personal_connection", func(t *testing.T) {
		before := s.provider.total()
		// member-a1 (same space, NOT the owner) drives the full pipeline on
		// the owner's personal connection: prepare is reachable (catalog
		// chain is tenant-scoped), but Execute's A02 re-check rejects.
		id, err := s.preparer.PrepareOC(ctx, oc17MemberA, "conn-a1", oc17ActionID, oc17Args("personal-member"))
		if err != nil {
			t.Fatalf("PrepareOC by member: %v", err)
		}
		row, _ := s.actionStore.FindAction(ctx, id)
		if err := s.t12.Approve(ctx, id, "owner-a", row.ArgsDigest); err != nil {
			t.Fatalf("Approve: %v", err)
		}
		dbBefore := oc17ActionDBState(t, s, id)
		err = oc17Exec(s, ctx, id)
		dbAfter := oc17ActionDBState(t, s, id)
		s.journal.record(t, "personal_connection", "member_execute", "action_id", id,
			"db_before", dbBefore, "db_after", dbAfter, "err", fmt.Sprintf("%v", err))
		if !errors.Is(err, appconnectorsvc.ErrConnectionForbidden) {
			t.Fatalf("member execute on owner's personal connection: err=%v, want ErrConnectionForbidden", err)
		}
		if dbAfter["action_state"] != appconn.ActionAuthorized {
			t.Fatalf("rejected action left state=%v, want authorized (untouched)", dbAfter["action_state"])
		}
		if ap := s.provider.total(); ap != before {
			t.Fatalf("provider saw %d ops on denied personal-connection execute", ap-before)
		}

		// LIST VISIBILITY FINDING (T17-F2): the connection list projection
		// returns every tenant connection to every member (metadata only —
		// no credential_ref); "list hiding" is currently a UI concern.
		var rows []repoappconn.ConnectionRow
		if err := s.db.Where("tenant_id = ?", oc17TenantA).Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		ids := []string{}
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		sort.Strings(ids)
		s.journal.record(t, "personal_connection", "list_visibility_observation", "tenant_rows_visible_to_any_member", ids,
			"note", "handler returns metadata projections for all tenant connections; hiding of others' personal connections is frontend-side (finding T17-F2)")
	})

	// --- 3. space connection without explicit grant ------------------------
	t.Run("space_connection_grant", func(t *testing.T) {
		before := s.provider.total()
		// Member without grant: denied. Billing-capable admin without grant:
		// also denied (role never substitutes for a grant).
		for _, who := range []struct {
			name    string
			subject appconn.OCSubject
		}{{"member", oc17MemberA}, {"admin_billing", oc17AdminA}} {
			id, err := s.preparer.PrepareOC(ctx, who.subject, "conn-a-space", oc17ActionID, oc17Args("space-"+who.name))
			if err != nil {
				t.Fatalf("PrepareOC by %s: %v", who.name, err)
			}
			row, _ := s.actionStore.FindAction(ctx, id)
			if err := s.t12.Approve(ctx, id, "owner-a", row.ArgsDigest); err != nil {
				t.Fatal(err)
			}
			err = oc17Exec(s, ctx, id)
			s.journal.record(t, "space_connection_grant", who.name+"_execute_denied", "action_id", id, "err", fmt.Sprintf("%v", err))
			if !errors.Is(err, appconnectorsvc.ErrConnectionForbidden) {
				t.Fatalf("%s without grant: err=%v, want ErrConnectionForbidden", who.name, err)
			}
		}
		// Control case: with an EXPLICIT grant the same subject passes the
		// guard — proving the denial above is grant-driven, not actor-driven.
		grants := mapGrants{granted: map[string]bool{fmt.Sprintf("%d|%s|%s", oc17TenantA, "conn-a-space", "member-a1"): true}}
		grantedGuard := appconnectorsvc.NewOCSubjectGuard(repository.NewMCPOAuthBindingStore(s.db),
			appconnectorsvc.NewInstallationStateSource(s.installs), grants, s.ocStore)
		err := grantedGuard.Check(ctx, oc17MemberA, "conn-a-space", 1)
		prodGuard := appconnectorsvc.NewOCSubjectGuard(repository.NewMCPOAuthBindingStore(s.db),
			appconnectorsvc.NewInstallationStateSource(s.installs), nil, s.ocStore)
		errNil := prodGuard.Check(ctx, oc17OwnerA, "conn-a-space", 1)
		s.journal.record(t, "space_connection_grant", "control_cases",
			"member_with_explicit_grant", err == nil, "err", fmt.Sprintf("%v", err),
			"owner_under_production_nil_grants", fmt.Sprintf("%v", errNil),
			"note", "production wiring has no grant source yet: space connections fail closed for everyone (container.go:506)")
		if err != nil {
			t.Fatalf("explicit-grant control case failed: %v", err)
		}
		if after := s.provider.total(); after != before {
			t.Fatalf("provider saw %d ops during denied space-connection executes", after-before)
		}
	})

	// --- 4. account switch on the same provider ----------------------------
	t.Run("account_switch", func(t *testing.T) {
		before := s.provider.total()
		// Owner approves under account A (ext-account-a).
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "pre-switch")
		dbBefore := oc17ActionDBState(t, s, id)

		// Switch accounts the T03/T08 way: revoke the connection+binding,
		// create a NEW connection+binding under the new external identity
		// (binding external identity is immutable per SaveBinding contract).
		if err := s.ocStore.RevokeOCConnection(ctx, oc17OwnerA, "conn-a1", 1); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		if err := s.installs.SaveConnection(ctx, appconn.Connection{
			ID: "conn-a2", InstallationID: "inst-a", Kind: appconn.ConnectionKindPersonal,
			OwnerID: "owner-a", State: appconn.ConnectionActive, TenantID: oc17TenantA, AuthVersion: 1,
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.ocStore.SaveBinding(ctx, appconn.OCBinding{
			TenantID: oc17TenantA, ConnectionID: "conn-a2", RuntimeID: oc17Runtime,
			Provider: oc17ProviderName, ExternalID: "ext-account-a2", Alias: "alias-account-a2",
			AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive,
		}); err != nil {
			t.Fatal(err)
		}
		if err := oc17SeedTokenFile(s.tokenDir, oc17TenantA, "conn-a2", 1, "tok-A2-v1-restricted"); err != nil {
			t.Fatal(err)
		}

		// The OLD approval must not execute: the connection it was approved
		// against is revoked (and at a new authorization generation).
		err := oc17Exec(s, ctx, id)
		dbAfter := oc17ActionDBState(t, s, id)
		bindingAfter := oc17BindingState(t, s, oc17TenantA, "conn-a1")
		s.journal.record(t, "account_switch", "old_approval_execute", "action_id", id,
			"db_before", dbBefore, "db_after", dbAfter, "old_binding", bindingAfter, "err", fmt.Sprintf("%v", err))
		if err == nil {
			t.Fatal("old approval executed after account switch")
		}
		if !errors.Is(err, appconnectorsvc.ErrConnectionVersionStale) && !errors.Is(err, appconnectorsvc.ErrConnectionRevoked) && !errors.Is(err, appconnectorsvc.ErrConnectionForbidden) {
			t.Fatalf("old approval after switch: err=%v, want revocation denial", err)
		}
		if after := s.provider.total(); after != before {
			t.Fatalf("provider saw %d ops during post-switch old-approval execute", after-before)
		}

		// NO DEFAULT FALLBACK: the frozen client refuses an empty alias, so a
		// dispatch can never silently ride upstream's "default" connection.
		client, cerr := openconnector.NewClient(s.provider.URL(), &http.Client{Timeout: 5 * time.Second})
		if cerr != nil {
			t.Fatal(cerr)
		}
		_, err = client.Execute(ctx, "tok-A1-v1-restricted", openconnector.Call{Alias: "", ActionID: oc17ActionID, Key: "wk-oc-probe-empty-alias", Input: json.RawMessage(`{"body":"x"}`)})
		s.journal.record(t, "account_switch", "empty_alias_refused", "err", fmt.Sprintf("%v", err))
		if !errors.Is(err, openconnector.ErrAliasRequired) {
			t.Fatalf("empty alias: err=%v, want ErrAliasRequired (no default fallback)", err)
		}

		// The NEW account works and the provider sees the NEW alias — the
		// approval cannot ride the old identity either.
		id2 := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a2", "post-switch")
		if err := oc17Exec(s, ctx, id2); err != nil {
			t.Fatalf("execute on switched account: %v", err)
		}
		ops := s.provider.opsSnapshot()
		var last oc17Op
		for _, op := range ops {
			if op.Alias != "" {
				last = op
			}
		}
		s.journal.record(t, "account_switch", "new_account_execute", "action_id", id2, "alias_seen", last.Alias)
		if last.Alias != "alias-account-a2" {
			t.Fatalf("provider alias=%q, want alias-account-a2 (no fallback to old account)", last.Alias)
		}
		// Re-arm the shared fixture connection for the scenarios that follow.
		oc17RestoreConnA1(t, s)
	})

	// --- 5. revoke race: revoke before claim / claim before revoke ---------
	t.Run("revoke_race", func(t *testing.T) {
		// (a) revoke BEFORE claim: zero sends, approval NOT consumed.
		before := s.provider.total()
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "revoke-first")
		curVer := oc17ConnVersion(t, s)
		if err := s.ocStore.RevokeOCConnection(ctx, oc17OwnerA, "conn-a1", curVer); err != nil {
			t.Fatalf("revoke: %v", err)
		}
		dbBefore := oc17ActionDBState(t, s, id)
		err := oc17Exec(s, ctx, id)
		dbAfter := oc17ActionDBState(t, s, id)
		s.journal.record(t, "revoke_race", "revoke_before_claim", "action_id", id,
			"db_before", dbBefore, "db_after", dbAfter, "err", fmt.Sprintf("%v", err),
			"provider_delta", s.provider.total()-before)
		if err == nil {
			t.Fatal("execute succeeded after revoke-before-claim")
		}
		if got := dbAfter["approval_remaining"]; got != int64(1) {
			t.Fatalf("approval remaining after pre-claim revoke = %v, want 1 (unconsumed)", got)
		}
		if s.provider.total() != before {
			t.Fatalf("provider saw %d ops after revoke-before-claim", s.provider.total()-before)
		}
		// Restore the fixture connection for the reverse order (new generation).
		oc17RestoreConnA1(t, s)

		// (b) claim (in-flight) BEFORE revoke: exactly the original in-flight
		// completes — one provider write, terminal settle, audit-only.
		id2 := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "claim-first")
		curVer2 := oc17ConnVersion(t, s)
		s.provider.setMode("hold")
		done := make(chan error, 1)
		go func() { done <- oc17Exec(s, ctx, id2) }()
		// QR-2: bounded wait — if the dispatch failed before reaching the
		// provider, release the hold (so the provider server can close
		// cleanly) and fail instead of hanging on the arrival channel.
		select {
		case <-s.provider.arrived: // the outbound call is at the provider
		case <-time.After(30 * time.Second):
			s.provider.releaseHold()
			t.Fatal("in-flight dispatch never reached the provider")
		}
		if err := s.ocStore.RevokeOCConnection(ctx, oc17OwnerA, "conn-a1", curVer2); err != nil {
			t.Errorf("revoke during in-flight: %v", err)
		}
		s.provider.releaseHold()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("in-flight dispatch failed after concurrent revoke: %v", err)
			}
		case <-time.After(30 * time.Second):
			t.Fatal("in-flight dispatch did not finish")
		}
		s.provider.setMode("ok")
		dbAfter2 := oc17ActionDBState(t, s, id2)
		ops := s.provider.total() - before
		s.journal.record(t, "revoke_race", "claim_before_revoke", "action_id", id2,
			"db_after", dbAfter2, "provider_ops_for_scenario", ops)
		if dbAfter2["action_state"] != appconn.ActionSucceeded {
			t.Fatalf("in-flight action state=%v, want succeeded (original completes)", dbAfter2["action_state"])
		}
		if ops != 1 {
			t.Fatalf("provider ops during scenario = %d, want exactly the one in-flight write", ops)
		}
		// Re-arm the shared fixture connection for the scenarios that follow.
		oc17RestoreConnA1(t, s)
	})

	// --- 6. approval tampering: risk/schema/args/version --------------------
	t.Run("approval_tamper", func(t *testing.T) {
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "tamper-base")
		row, _ := s.actionStore.FindAction(ctx, id)

		// (a) ARGS: a second action with modified args has a different digest;
		// the OLD approval digest cannot approve it.
		id2, err := s.preparer.PrepareOC(ctx, oc17OwnerA, "conn-a1", oc17ActionID, oc17Args("tamper-modified"))
		if err != nil {
			t.Fatal(err)
		}
		err = s.t12.Approve(ctx, id2, "owner-a", row.ArgsDigest)
		s.journal.record(t, "approval_tamper", "modified_args_old_digest", "action", id2, "err", fmt.Sprintf("%v", err))
		if !errors.Is(err, appconnectorsvc.ErrActionDigestMismatch) {
			t.Fatalf("old digest approved modified args: err=%v", err)
		}

		// (b) DB tampering of args/risk/version: the persisted digest no longer
		// matches a recomputation over the row → the only path forward is a
		// NEW Prepare (reject or re-prepare).
		for _, col := range []string{"args_snapshot", "risk", "app_version"} {
			tampered := "tampered-value"
			if col == "args_snapshot" {
				tampered = `{"body":"tampered"}`
			}
			if err := s.db.Exec("UPDATE app_actions SET "+col+" = ? WHERE id = ?", tampered, id).Error; err != nil {
				t.Fatal(err)
			}
			trow, _ := s.actionStore.FindAction(ctx, id)
			recomputed, rerr := appconn.ActionDigest(appconn.Action{
				TenantID: trow.TenantID, ActorID: trow.ActorID, ConnectionID: trow.ConnectionID,
				Version: trow.AppVersion, Target: trow.Target, Risk: trow.Risk,
				AuthVersion: trow.AuthVersion, Args: []byte(trow.ArgsSnapshot),
				OC: nil, DigestVersion: int(trow.DigestVersion),
			})
			detectable := rerr != nil || recomputed != trow.ArgsDigest
			s.journal.record(t, "approval_tamper", "db_tamper_"+col, "detectable", detectable,
				"stored_digest_prefix", trow.ArgsDigest[:12], "oc_binding_json_len", len(trow.OCBindingJSON))
			if !detectable {
				t.Fatalf("tampering %s left the digest binding intact", col)
			}
			if err := s.db.Exec("ROLLBACK").Error; err != nil {
				_ = err
			}
			// restore the row for the next probe
			if col == "args_snapshot" {
				_ = s.db.Exec("UPDATE app_actions SET args_snapshot = ? WHERE id = ?", row.ArgsSnapshot, id).Error
			} else if col == "risk" {
				_ = s.db.Exec("UPDATE app_actions SET risk = ? WHERE id = ?", row.Risk, id).Error
			} else {
				_ = s.db.Exec("UPDATE app_actions SET app_version = ? WHERE id = ?", row.AppVersion, id).Error
			}
		}

		// (c) SCHEMA: the frozen catalog refuses a re-publish of the same
		// (app, version, action) with different schema bytes.
		err = s.catalog.PublishOCDefinition(ctx, appconn.OCDefinition{
			AppID: oc17ProviderName, AppVersion: oc17AppVer, ActionID: oc17ActionID,
			Provider: oc17ProviderName, Risk: appconn.RiskSend,
			InputSchema:    json.RawMessage(`{"type":"object","properties":{"body":{"type":"string"},"extra":{"type":"string"}},"required":["body"]}`),
			RequiredScopes: []string{"demo:send"}, Published: true,
		})
		s.journal.record(t, "approval_tamper", "schema_republish_conflict", "err", fmt.Sprintf("%v", err))
		if err == nil {
			t.Fatal("frozen catalog accepted a changed schema for the same version")
		}

		// (d) DIGEST GENERATION: a v1-generation authorized row must re-Prepare.
		if err := s.db.Exec("UPDATE app_actions SET digest_version = 1 WHERE id = ?", id).Error; err != nil {
			t.Fatal(err)
		}
		err = s.t12.Approve(ctx, id, "owner-a", row.ArgsDigest)
		s.journal.record(t, "approval_tamper", "v1_digest_approve", "err", fmt.Sprintf("%v", err))
		if !errors.Is(err, appconnectorsvc.ErrActionRePrepareRequired) {
			t.Fatalf("v1 digest approval: err=%v, want ErrActionRePrepareRequired", err)
		}
		err = oc17Exec(s, ctx, id)
		s.journal.record(t, "approval_tamper", "v1_digest_execute", "err", fmt.Sprintf("%v", err))
		if !errors.Is(err, appconnectorsvc.ErrActionRePrepareRequired) {
			t.Fatalf("v1 digest execute: err=%v, want ErrActionRePrepareRequired", err)
		}
	})

	// --- 7. concurrent duplicates: 20 requests, one claim/write/usage -----
	t.Run("concurrent_duplicate", func(t *testing.T) {
		before := s.provider.total()
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "concurrent-once")
		dbBefore := oc17ActionDBState(t, s, id)
		beginsBefore, _, _, _ := s.gate.stats() // scenario-relative baseline (the gate counter is suite-cumulative)

		const n = 20
		start := make(chan struct{})
		errs := make(chan error, n)
		var wg sync.WaitGroup
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				ectx, cancel := context.WithTimeout(ctx, 60*time.Second)
				defer cancel()
				errs <- oc17Exec(s, ectx, id)
			}()
		}
		close(start)
		wg.Wait()
		close(errs)
		wins, losses := 0, 0
		for err := range errs {
			if err == nil {
				wins++
			} else {
				losses++
			}
		}
		dbAfter := oc17ActionDBState(t, s, id)
		begins, _, _, facts := s.gate.stats()
		okFacts := s.gate.okFactsFor(id)
		s.journal.record(t, "concurrent_duplicate", "execute_race", "action_id", id,
			"requests", n, "winners", wins, "losers", losses,
			"provider_ops", s.provider.total()-before,
			"db_before", dbBefore, "db_after", dbAfter,
			"gate_begins_during_race", begins-beginsBefore, "terminal_facts_for_action", okFacts)
		if wins != 1 || losses != n-1 {
			t.Fatalf("winners=%d losers=%d, want exactly 1/%d", wins, losses, n-1)
		}
		if got := s.provider.total() - before; got != 1 {
			t.Fatalf("provider ops = %d, want exactly 1 side effect", got)
		}
		if dbAfter["action_state"] != appconn.ActionSucceeded {
			t.Fatalf("action state=%v", dbAfter["action_state"])
		}
		if okFacts != 1 {
			t.Fatalf("terminal usage facts for action = %d, want exactly 1 (facts=%d)", okFacts, len(facts))
		}
		if rem := dbAfter["approval_remaining"]; rem != int64(0) {
			t.Fatalf("approval remaining = %v, want exactly one consume", rem)
		}

		// CARRY T14-QI-1: the tool-binding insert race on REAL PostgreSQL —
		// 20 identical PrepareForTool calls produce ONE action + ONE binding.
		toolStore := appconnectorsvc.NewOCToolBindingStore(s.db)
		toolSvc := appconnectorsvc.NewOCToolBindingService(s.actionStore, s.catalog, s.ocStore, toolStore)
		tsub := appconn.OCSubject{TenantID: oc17TenantA, ActorID: "owner-a"}
		terr := make(chan error, n)
		tids := make(chan string, n)
		for i := 0; i < n; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				aid, err := toolSvc.PrepareForTool(ctx, tsub, "sess-17", "call-17", "conn-a1", oc17ActionID, oc17Args("tool-bind-race"))
				terr <- err
				if aid != "" {
					tids <- aid
				}
			}()
		}
		// The tool-binding racers gate on the ALREADY-CLOSED start channel
		// (a receive from a closed channel returns immediately).
		wg.Wait()
		close(terr)
		close(tids)
		tbWins, tbErrs := 0, 0
		uniqueActions := map[string]bool{}
		for err := range terr {
			if err == nil {
				tbWins++
			} else {
				tbErrs++
			}
		}
		for aid := range tids {
			uniqueActions[aid] = true
		}
		var bindRows int64
		if err := s.db.Raw("SELECT COUNT(*) FROM connector_tool_bindings WHERE tenant_id = ? AND session_id = 'sess-17'", oc17TenantA).Scan(&bindRows).Error; err != nil {
			t.Fatal(err)
		}
		var actRows int64
		if err := s.db.Raw("SELECT COUNT(*) FROM app_actions WHERE id LIKE 'ocact_%' AND tenant_id = ?", oc17TenantA).Scan(&actRows).Error; err != nil {
			t.Fatal(err)
		}
		s.journal.record(t, "concurrent_duplicate", "tool_binding_race_pg", "requests", n,
			"ok", tbWins, "readback_winners", tbErrs, "unique_action_ids", len(uniqueActions),
			"binding_rows", bindRows)
		if len(uniqueActions) != 1 || bindRows != 1 {
			t.Fatalf("tool binding race: unique actions=%d binding rows=%d, want 1/1", len(uniqueActions), bindRows)
		}
	})

	// --- 8. unknown outcome: provider writes then breaks the response ------
	t.Run("unknown_outcome", func(t *testing.T) {
		before := s.provider.total()
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "unknown-write-cut")
		s.provider.setMode("break")
		err := oc17Exec(s, ctx, id)
		s.provider.setMode("ok")
		dbAfter := oc17ActionDBState(t, s, id)
		s.journal.record(t, "unknown_outcome", "write_then_break", "action_id", id,
			"err", fmt.Sprintf("%v", err), "provider_writes", s.provider.total()-before,
			"db_after", dbAfter)
		if !errors.Is(err, appconnectorsvc.ErrDispatchUnknown) {
			t.Fatalf("write-then-break: err=%v, want ErrDispatchUnknown", err)
		}
		if s.provider.total()-before != 1 {
			tail := s.provider.opsSnapshot()
			if len(tail) > 3 {
				tail = tail[len(tail)-3:]
			}
			for _, op := range tail {
				t.Logf("OC17-DEBUG op seq=%d alias=%s key_prefix=%s body_prefix=%s broken=%v", op.Seq, op.Alias, op.Key[:12], op.Body[:min(60, len(op.Body))], op.Broken)
			}
			t.Fatalf("provider writes = %d, want exactly 1 (the cut write)", s.provider.total()-before)
		}
		if dbAfter["action_state"] != appconn.ActionUnknown || dbAfter["record_state"] != appconn.ActionUnknown {
			t.Fatalf("unknown parking: action=%v record=%v", dbAfter["action_state"], dbAfter["record_state"])
		}

		// NO RESEND: recovery without a resolver never re-POSTs; a manual
		// Execute is refused (unknown never re-enters the dispatch path).
		recov, rerr := appconnectorsvc.NewOCRecovery(s.ocStore, s.t12)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if err := recov.RunOnce(ctx); err != nil {
			t.Logf("recovery pass note: %v", err)
		}
		err = oc17Exec(s, ctx, id)
		s.journal.record(t, "unknown_outcome", "no_resend", "provider_ops_after_recovery", s.provider.total()-before,
			"manual_execute_err", fmt.Sprintf("%v", err))
		if s.provider.total()-before != 1 {
			t.Fatalf("provider ops after recovery = %d, want still 1 (no resend)", s.provider.total()-before)
		}
		if !errors.Is(err, appconnectorsvc.ErrActionState) {
			t.Fatalf("manual execute on unknown: err=%v, want ErrActionState (no resend path)", err)
		}
	})

	// --- 9. local failure after provider success: compensate, never re-send
	t.Run("local_failure_settlement", func(t *testing.T) {
		before := s.provider.total()
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "settle-outage")
		s.gate.mu.Lock()
		s.gate.failFinishN = 1 // the FIRST settlement delivery fails
		s.gate.mu.Unlock()
		err := oc17Exec(s, ctx, id)
		dbAfterFail := oc17ActionDBState(t, s, id)
		_, calls, fails, _ := s.gate.stats()
		s.journal.record(t, "local_failure_settlement", "delivery_failure", "action_id", id,
			"err", fmt.Sprintf("%v", err), "provider_ops", s.provider.total()-before,
			"db_after", dbAfterFail, "finish_calls", calls, "finish_failures", fails)
		if err == nil {
			t.Fatal("settlement outage did not surface from Execute")
		}
		// The ORIGINAL outcome is already durable: record + action terminal.
		if dbAfterFail["action_state"] != appconn.ActionSucceeded || dbAfterFail["record_state"] != appconn.ActionSucceeded {
			t.Fatalf("original outcome not preserved: action=%v record=%v", dbAfterFail["action_state"], dbAfterFail["record_state"])
		}

		// Recovery retries the settlement ALONE with the identical fact — no
		// provider re-call, exactly one successful delivery.
		recov, rerr := appconnectorsvc.NewOCRecovery(s.ocStore, s.t12)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if err := recov.RunOnce(ctx); err != nil {
			t.Logf("recovery pass note: %v", err)
		}
		oc17WaitFor(t, "settlement delivered", func() bool { return s.gate.okFactsFor(id) == 1 }, 10*time.Second)
		dbAfter := oc17ActionDBState(t, s, id)
		_, calls2, fails2, facts := s.gate.stats()
		s.journal.record(t, "local_failure_settlement", "compensation_retry", "action_id", id,
			"provider_ops", s.provider.total()-before, "finish_calls", calls2,
			"finish_failures", fails2, "ok_facts", s.gate.okFactsFor(id), "db_after", dbAfter)
		if s.provider.total()-before != 1 {
			t.Fatalf("provider ops = %d, want 1 (never a second external write)", s.provider.total()-before)
		}
		if s.gate.okFactsFor(id) != 1 {
			t.Fatalf("successful settlement facts = %d, want exactly 1", s.gate.okFactsFor(id))
		}
		var revs []int64
		for _, f := range facts {
			if f.CallID == id {
				revs = append(revs, f.Revision)
			}
		}
		if len(revs) != 1 || revs[0] != 1 {
			t.Fatalf("delivered fact revisions for action = %v, want the single stable fact [1]", revs)
		}
	})

	// --- 10. crash: claim then kill the worker, restart, converge ----------
	t.Run("crash_restart", func(t *testing.T) {
		before := s.provider.total()
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "crash-after-claim")
		// The worker claims (one transaction: approval consumed, key minted,
		// action dispatched) and THEN dies before the outbound call. Claiming
		// directly through the store leaves the exact durable state a killed
		// process leaves (T12 precedent); the "restart" is a fresh recovery
		// loop over the same database.
		rec, err := s.ocStore.ClaimOCDispatch(ctx, oc17OwnerA, id, "res-crash-17")
		if err != nil {
			t.Fatalf("claim: %v", err)
		}
		keyBefore, runtimeBefore := rec.Key, rec.RuntimeID
		// Isolated clock advance: backdate the heartbeat past the 90s bound.
		if err := s.db.Exec("UPDATE connector_dispatch_records SET updated_at = ? WHERE tenant_id = ? AND action_id = ?",
			time.Now().UTC().Add(-2*time.Minute), oc17TenantA, id).Error; err != nil {
			t.Fatal(err)
		}
		dbBefore := oc17ActionDBState(t, s, id)

		recov, rerr := appconnectorsvc.NewOCRecovery(s.ocStore, s.t12)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if err := recov.RunOnce(ctx); err != nil {
			t.Logf("recovery pass note: %v", err)
		}
		dbAfter := oc17ActionDBState(t, s, id)
		s.journal.record(t, "crash_restart", "stale_sweep", "action_id", id,
			"key_before", keyBefore, "key_after", dbAfter["record_key"],
			"runtime_before", runtimeBefore, "runtime_after", dbAfter["record_runtime"],
			"db_before", dbBefore, "db_after", dbAfter, "provider_ops", s.provider.total()-before)
		if dbAfter["action_state"] != appconn.ActionUnknown || dbAfter["record_state"] != appconn.ActionUnknown {
			t.Fatalf("crash convergence: action=%v record=%v, want unknown/unknown", dbAfter["action_state"], dbAfter["record_state"])
		}
		if dbAfter["record_key"] != keyBefore {
			t.Fatal("stale sweep replaced the idempotency key")
		}
		if dbAfter["record_runtime"] != runtimeBefore {
			t.Fatal("stale sweep replaced the runtime")
		}
		if s.provider.total() != before {
			t.Fatalf("provider saw %d ops during crash recovery", s.provider.total()-before)
		}
		// A re-claim of the swept action is refused (no re-claim path).
		if _, err := s.ocStore.ClaimOCDispatch(ctx, oc17OwnerA, id, "res-crash-17-retry"); err == nil {
			t.Fatal("re-claim of swept action succeeded")
		}

		// CARRY T08-F-2/T12 (stale-attempt hygiene): authorization attempts
		// stuck in non-terminal states with the window passed are OBSERVED —
		// no sweeper exists for them yet (report-only, no fix).
		expiredID, aerr := s.connSvc.Begin(ctx, oc17OwnerA, "conn-a1")
		if aerr != nil {
			t.Fatalf("begin attempt: %v", aerr)
		}
		if err := s.db.Exec("UPDATE connector_authorization_attempts SET expires_at = ? WHERE id = ?",
			time.Now().UTC().Add(-time.Hour), expiredID).Error; err != nil {
			t.Fatal(err)
		}
		var stuck int64
		if err := s.db.Raw("SELECT COUNT(*) FROM connector_authorization_attempts WHERE state IN ('pending','authorizing','verifying') AND expires_at < ?", time.Now().UTC()).Scan(&stuck).Error; err != nil {
			t.Fatal(err)
		}
		s.journal.record(t, "crash_restart", "stale_attempt_hygiene_observation",
			"expired_attempt_id", expiredID, "stuck_non_terminal_attempts", stuck,
			"note", "no attempt sweeper exists in phase one (T08 Q-2/Q-3 carry): expired attempts persist until T18")
	})

	// --- 11. replay deadline: isolated clock at the cutoff ------------------
	t.Run("replay_deadline", func(t *testing.T) {
		id := oc17PrepareApprove(t, s, oc17OwnerA, "conn-a1", "deadline-window")
		if err := oc17Exec(s, ctx, id); err != nil {
			t.Fatalf("execute: %v", err)
		}
		// Baseline AFTER the scenario's own legitimate dispatch: everything
		// from here on must be provably free of automatic replay.
		before := s.provider.total()
		db := oc17ActionDBState(t, s, id)
		first, err := time.Parse(time.RFC3339Nano, db["record_first_sent_at"].(string))
		if err != nil {
			t.Fatal(err)
		}
		until, err := time.Parse(time.RFC3339Nano, db["record_replay_until"].(string))
		if err != nil {
			t.Fatal(err)
		}
		rec := appconn.OCDispatchRecord{
			TenantID: oc17TenantA, ActionID: id, RuntimeID: oc17Runtime,
			Key: db["record_key"].(string), FirstSentAt: first, ReplayUntil: until,
		}
		atStart := appconnectorsvc.ReplayAllowed(rec, first)
		justInside := appconnectorsvc.ReplayAllowed(rec, until.Add(-time.Nanosecond))
		atCutoff := appconnectorsvc.ReplayAllowed(rec, until)
		clockBackwards := appconnectorsvc.ReplayAllowed(rec, first.Add(-time.Second))
		corrupt := rec
		corrupt.ReplayUntil = first.Add(25 * time.Hour)
		corruptAllowed := appconnectorsvc.ReplayAllowed(corrupt, first.Add(24*time.Hour))

		// Isolated clock ADVANCE TO CUTOFF: push the stored window into the
		// past (keeping it inside the 24h plausibility bound), run recovery —
		// no automatic replay may happen and the original runtime survives.
		if err := s.db.Exec("UPDATE connector_dispatch_records SET first_sent_at = ?, replay_until = ?, updated_at = ? WHERE tenant_id = ? AND action_id = ?",
			time.Now().UTC().Add(-26*time.Hour), time.Now().UTC().Add(-2*time.Hour),
			time.Now().UTC().Add(-2*time.Hour), oc17TenantA, id).Error; err != nil {
			t.Fatal(err)
		}
		recov, rerr := appconnectorsvc.NewOCRecovery(s.ocStore, s.t12)
		if rerr != nil {
			t.Fatal(rerr)
		}
		if err := recov.RunOnce(ctx); err != nil {
			t.Logf("recovery pass note: %v", err)
		}
		after := oc17ActionDBState(t, s, id)
		s.journal.record(t, "replay_deadline", "boundaries_and_advance",
			"action_id", id,
			"allowed_at_first_sent", atStart, "allowed_just_inside", justInside,
			"allowed_at_cutoff", atCutoff, "allowed_clock_backwards", clockBackwards,
			"allowed_corrupt_window", corruptAllowed,
			"provider_ops_after_advance", s.provider.total()-before,
			"record_after", after)
		if !atStart || !justInside {
			t.Fatal("in-window replayability is not granted inside the window")
		}
		if atCutoff {
			t.Fatal("replay allowed AT the cutoff (original deadline must never extend)")
		}
		if clockBackwards {
			t.Fatal("replay allowed before FirstSentAt")
		}
		if corruptAllowed {
			t.Fatal("replay allowed for a corrupt >24h window")
		}
		if s.provider.total() != before {
			tail := s.provider.opsSnapshot()
			if len(tail) > 3 {
				tail = tail[len(tail)-3:]
			}
			for _, op := range tail {
				bp := op.Body
				if len(bp) > 60 {
					bp = bp[:60]
				}
				t.Logf("OC17-DEBUG op seq=%d alias=%s body_prefix=%s broken=%v", op.Seq, op.Alias, bp, op.Broken)
			}
			t.Fatalf("auto-replay fired: provider ops +%d", s.provider.total()-before)
		}
		if after["record_runtime"] != oc17Runtime || after["record_key"] != db["record_key"] {
			t.Fatal("original runtime/key not preserved at the deadline")
		}
		if after["record_state"] != appconn.ActionSucceeded {
			// the record is terminal (succeeded) — the sweep must not touch it
			t.Fatalf("terminal record disturbed at deadline: %v", after["record_state"])
		}
	})

	// --- 12. credentials scan: no admin token/secret/raw credentials -------
	t.Run("credentials_scan", func(t *testing.T) {
		before := s.provider.total()
		// A clean dispatch through the PRODUCTION token path.
		prodPreparer := appconnectorsvc.NewOCPreparer(s.prod, s.catalog, s.ocStore)
		id, err := prodPreparer.PrepareOC(ctx, oc17OwnerA, "conn-a1", oc17ActionID, oc17Args("credential-scan"))
		if err != nil {
			t.Fatal(err)
		}
		row, _ := s.actionStore.FindAction(ctx, id)
		if err := s.prod.Approve(ctx, id, "owner-a", row.ArgsDigest); err != nil {
			t.Fatal(err)
		}
		if err := s.prod.Execute(ctx, id); err != nil {
			t.Fatalf("production-path execute: %v", err)
		}
		ops := s.provider.opsSnapshot()
		var seenAuth []string
		for _, op := range ops {
			if op.Body != "" && strings.Contains(op.Body, "credential-scan") {
				seenAuth = append(seenAuth, strings.TrimPrefix(op.Auth, "Bearer "))
			}
		}
		s.journal.record(t, "credentials_scan", "dispatch_auth_headers", "action_id", id,
			"auth_headers", seenAuth, "expected", fmt.Sprintf("tok-A1-v%d-restricted", oc17ConnVersion(t, s)))

		// The scoped token for EXACTLY (tenant, connection, CURRENT version) —
		// and never the admin material, never another generation/tenant.
		wantToken := fmt.Sprintf("tok-A1-v%d-restricted", oc17ConnVersion(t, s))
		if len(seenAuth) == 0 || seenAuth[len(seenAuth)-1] != wantToken {
			t.Fatalf("provider Authorization = %v, want the scoped restricted token %q", seenAuth, wantToken)
		}
		tokens, err := container.NewFileBackedOCTokenSource(oc17Sink(t, s.tokenDir), s.ocStore)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := tokens.Token(ctx, oc17TenantA, "conn-a1", 2); err == nil {
			t.Fatal("token issued for a stale authorization generation")
		}
		if _, err := tokens.Token(ctx, oc17TenantB, "conn-a1", 1); err == nil {
			t.Fatal("token issued across tenants")
		}

		// Scan the recorded evidence and the persisted rows for secret
		// material: none of the restricted/admin values may appear anywhere
		// outside the Authorization header of its own dispatch.
		forbidden := []string{"oc17-admin-secret-do-not-dispatch", "tok-B1-v1-restricted"}
		blob, _ := json.Marshal(ops)
		for _, secret := range forbidden {
			if strings.Contains(string(blob), secret) {
				t.Fatalf("secret material %q leaked into provider-visible evidence", secret[:10])
			}
		}
		var rows []repoappconn.ConnectionRow
		if err := s.db.Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if strings.Contains(r.CredentialRef, "tok-") || strings.Contains(r.CredentialRef, "secret") && strings.Contains(r.CredentialRef, "do-not") {
				t.Fatalf("connection %s carries credential material in its ref", r.ID)
			}
		}
		var actionsRaw string
		if err := s.db.Raw("SELECT COALESCE(string_agg(id::text || oc_binding_json, ' '), '') FROM app_actions").Scan(&actionsRaw).Error; err != nil {
			t.Fatal(err)
		}
		for _, secret := range append(forbidden, "tok-A1-v1-restricted") {
			if strings.Contains(actionsRaw, secret) {
				t.Fatalf("secret material %q persisted on action rows", secret[:10])
			}
		}
		// The transient API-key path seals material before persistence.
		cipher, err := appconnectorsvc.NewTransientCipherFromKey("oc17-transient-key-material")
		if err != nil {
			t.Fatal(err)
		}
		sealed, err := cipher.Seal("ghp_raw_provider_key_123", "attempt-uuid-17")
		if err != nil {
			t.Fatal(err)
		}
		leaks := strings.Contains(sealed, "ghp_raw_provider_key_123")
		s.journal.record(t, "credentials_scan", "apikey_sealed_and_row_scan", "sealed_prefix", sealed[:8],
			"plaintext_in_sealed", leaks, "db_actions_blob_len", len(actionsRaw))
		if leaks {
			t.Fatal("transient cipher leaked plaintext")
		}
		if after := s.provider.total(); after-before != 1 {
			t.Fatalf("credentials scan dispatch op count = %d", after-before)
		}
	})

	// --- 13. multi-replica: two API workers, shared global limits ----------
	t.Run("multi_replica", func(t *testing.T) {
		before := s.provider.total()
		// Two replica wirings over the SAME store, TUNED limits (the default
		// global of 32 cannot be observed with a bounded scenario): the
		// four-scope DB leases are the authority across processes.
		limits := appconnectorsvc.OCSlotLimits{PerTenant: 2, PerConnection: 1, PerProvider: 8, Global: 3}
		mkReplica := func(owner string) *appconnectorsvc.ActionService {
			exec, err := openconnector.NewClient(s.provider.URL(), &http.Client{Timeout: 30 * time.Second})
			if err != nil {
				t.Fatal(err)
			}
			tokens, err := container.NewFileBackedOCTokenSource(oc17Sink(t, s.tokenDir), s.ocStore)
			if err != nil {
				t.Fatal(err)
			}
			disp := appconnectorsvc.NewOCDispatcher(exec, s.ocStore, tokens, s.ocStore)
			slots, err := appconnectorsvc.NewOCSlotLimiter(s.ocStore, limits, owner)
			if err != nil {
				t.Fatal(err)
			}
			src := repository.NewMCPOAuthBindingStore(s.db)
			guard := appconnectorsvc.NewOCSubjectGuard(src, appconnectorsvc.NewInstallationStateSource(s.installs), nil, s.ocStore)
			svc := appconnectorsvc.NewActionService(s.actionStore, guard, s.gate, disp, nil)
			svc.UseOCDispatchClaims(s.ocStore)
			svc.UseOCSlotLimiter(slots)
			return svc
		}
		replicaA := mkReplica("oc17-replica-1")
		replicaB := mkReplica("oc17-replica-2")

		// 6 approved actions: 3 per space, spread across BOTH replicas.
		type job struct {
			svc    *appconnectorsvc.ActionService
			action string
			space  uint64
		}
		var jobs []job
		perSpace := map[uint64]int{}
		for i := 0; i < 6; i++ {
			var subject appconn.OCSubject
			var conn string
			if i%2 == 0 {
				subject, conn = oc17OwnerA, "conn-a1"
				perSpace[oc17TenantA]++
				if perSpace[oc17TenantA] > 1 {
					conn = "conn-a-space-owner"
				}
			} else {
				subject, conn = oc17OwnerB, "conn-b1"
			}
			preparer := appconnectorsvc.NewOCPreparer(s.t12, s.catalog, s.ocStore)
			id, err := preparer.PrepareOC(ctx, subject, conn, oc17ActionID, oc17Args(fmt.Sprintf("replica-%02d", i)))
			if err != nil {
				// space-A second+ actions need a granted space connection;
				// fall back to a second personal connection for tenant A.
				if conn == "conn-a-space-owner" {
					_ = s.installs.SaveConnection(ctx, appconn.Connection{ID: "conn-a-p2", InstallationID: "inst-a", Kind: appconn.ConnectionKindPersonal, OwnerID: "owner-a", State: appconn.ConnectionActive, TenantID: oc17TenantA, AuthVersion: 1})
					_ = s.ocStore.SaveBinding(ctx, appconn.OCBinding{TenantID: oc17TenantA, ConnectionID: "conn-a-p2", RuntimeID: oc17Runtime, Provider: oc17ProviderName, ExternalID: "ext-a-p2", Alias: "alias-a-p2", AuthVersion: 1, BindingVersion: 1, State: appconn.OCBindingActive})
					_ = oc17SeedTokenFile(s.tokenDir, oc17TenantA, "conn-a-p2", 1, "tok-AP2-v1-restricted")
					id, err = preparer.PrepareOC(ctx, subject, "conn-a-p2", oc17ActionID, oc17Args(fmt.Sprintf("replica-%02d", i)))
				}
				if err != nil {
					t.Fatalf("prepare replica job %d: %v", i, err)
				}
			}
			row, _ := s.actionStore.FindAction(ctx, id)
			if err := s.t12.Approve(ctx, id, subject.ActorID, row.ArgsDigest); err != nil {
				t.Fatal(err)
			}
			svc := replicaA
			if i%2 == 1 {
				svc = replicaB
			}
			jobs = append(jobs, job{svc: svc, action: id, space: subject.TenantID})
		}

		// The provider holds each op briefly so overlap is observable.
		s.provider.setMode("ok")
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, j := range jobs {
			wg.Add(1)
			go func(j job) {
				defer wg.Done()
				<-start
				ectx, cancel := context.WithTimeout(ctx, 90*time.Second)
				defer cancel()
				if err := j.svc.Execute(ectx, j.action); err != nil {
					t.Errorf("replica execute %s: %v", j.action, err)
				}
			}(j)
		}
		close(start)
		wg.Wait()
		peak := s.provider.peakInflight()
		ops := s.provider.total() - before
		spacesDone := map[uint64]int{}
		for _, j := range jobs {
			st := oc17ActionDBState(t, s, j.action)
			if st["action_state"] == appconn.ActionSucceeded {
				spacesDone[j.space]++
			}
		}
		s.journal.record(t, "multi_replica", "two_replica_concurrency",
			"limits", limits, "jobs", len(jobs), "provider_ops", ops,
			"peak_inflight", peak, "succeeded_by_space", fmt.Sprintf("%v", spacesDone))
		if ops != len(jobs) {
			t.Fatalf("provider ops = %d, want %d (all schedulable)", ops, len(jobs))
		}
		if int(peak) > limits.Global {
			t.Fatalf("peak provider concurrency %d exceeded global limit %d", peak, limits.Global)
		}
		if spacesDone[oc17TenantA] < 3 || spacesDone[oc17TenantB] < 3 {
			t.Fatalf("space starvation: succeeded %v", spacesDone)
		}
	})

	// --- end of suite: settle intentional unknowns, then scan ---------------
	t.Run("final_settlement_scan", func(t *testing.T) {
		// Wire the provider-query resolver (phase-one production has none)
		// and settle the scenarios that intentionally parked unknown.
		tokens, err := container.NewFileBackedOCTokenSource(oc17Sink(t, s.tokenDir), s.ocStore)
		if err != nil {
			t.Fatal(err)
		}
		exec, err := openconnector.NewClient(s.provider.URL(), &http.Client{Timeout: 30 * time.Second})
		if err != nil {
			t.Fatal(err)
		}
		disp := appconnectorsvc.NewOCDispatcher(exec, s.ocStore, tokens, s.ocStore)
		resolver := keyResolver{p: s.provider, store: s.ocStore}
		src := repository.NewMCPOAuthBindingStore(s.db)
		guard := appconnectorsvc.NewOCSubjectGuard(src, appconnectorsvc.NewInstallationStateSource(s.installs), nil, s.ocStore)
		settleSvc := appconnectorsvc.NewActionService(s.actionStore, guard, s.gate, disp, resolver)
		settleSvc.UseOCDispatchClaims(s.ocStore)
		recov, err := appconnectorsvc.NewOCRecovery(s.ocStore, settleSvc)
		if err != nil {
			t.Fatal(err)
		}
		before := s.provider.total()
		// unexplainedOpen counts open records that are NOT the known finding-1
		// residue (records left dispatched by the PRODUCTION wiring because
		// GatedOCClaims masks the settle face — their action rows are already
		// terminal; see TestOCIntegrationWiringSettleParity).
		openExceptFinding1 := func() []struct {
			ActionID  string    `gorm:"column:action_id"`
			State     string    `gorm:"column:state"`
			UpdatedAt time.Time `gorm:"column:updated_at"`
		} {
			var open []struct {
				ActionID  string    `gorm:"column:action_id"`
				State     string    `gorm:"column:state"`
				UpdatedAt time.Time `gorm:"column:updated_at"`
			}
			_ = s.db.Raw(`SELECT d.action_id, d.state, d.updated_at FROM connector_dispatch_records d
				LEFT JOIN app_actions a ON a.tenant_id = d.tenant_id AND a.id = d.action_id
				WHERE d.state IN ('dispatched','unknown')
				AND (a.id IS NULL OR a.state IN ('dispatched','unknown'))`).Scan(&open).Error
			return open
		}
		oc17WaitFor(t, "unknown resolution", func() bool {
			if err := recov.RunOnce(ctx); err != nil {
				t.Logf("settlement pass note: %v", err)
			}
			return len(openExceptFinding1()) == 0
		}, 30*time.Second)

		var openRecords []struct {
			TenantID    uint64  `gorm:"column:tenant_id"`
			ActionID    string  `gorm:"column:action_id"`
			State       string  `gorm:"column:state"`
			ActionState *string `gorm:"column:action_state"`
		}
		_ = s.db.Raw(`SELECT d.tenant_id, d.action_id, d.state, a.state AS action_state
			FROM connector_dispatch_records d
			LEFT JOIN app_actions a ON a.tenant_id = d.tenant_id AND a.id = d.action_id
			WHERE d.state IN ('dispatched','unknown')`).Scan(&openRecords).Error
		var unsettled []struct {
			TenantID uint64 `gorm:"column:tenant_id"`
			ActionID string `gorm:"column:action_id"`
		}
		_ = s.db.Raw(`SELECT d.tenant_id, d.action_id FROM connector_dispatch_records d
			WHERE d.state IN ('succeeded','failed') AND d.reservation_id <> ''
			AND d.fence <= COALESCE((SELECT a.fence FROM app_actions a WHERE a.tenant_id = d.tenant_id AND a.id = d.action_id), d.fence)`).Scan(&unsettled).Error
		var staleAttempts int64
		_ = s.db.Raw("SELECT COUNT(*) FROM connector_authorization_attempts WHERE state IN ('pending','authorizing','verifying')").Scan(&staleAttempts).Error
		// Partition the remaining open records: finding-1 residue (record open,
		// action TERMINAL — the production wiring never settled the record) is
		// KNOWN, REPORTED evidence; anything else genuinely blocks cleanup.
		var finding1Residue, unexplained []string
		for _, r := range openRecords {
			if r.ActionState != nil && *r.ActionState != appconn.ActionDispatched && *r.ActionState != appconn.ActionUnknown {
				finding1Residue = append(finding1Residue, fmt.Sprintf("%s(record=%s,action=%s)", r.ActionID, r.State, *r.ActionState))
			} else {
				unexplained = append(unexplained, fmt.Sprintf("%s(record=%s)", r.ActionID, r.State))
			}
		}
		s.journal.record(t, "final_settlement_scan", "end_state",
			"open_records", openRecords, "unsettled_settlements", unsettled,
			"finding1_residue", finding1Residue, "unexplained_open", unexplained,
			"non_terminal_attempts_observed", staleAttempts,
			"provider_ops_during_settlement", s.provider.total()-before,
			"key_replays_absorbed", s.provider.replayCount(),
			"cleanup_blocked", len(unexplained) > 0 || len(unsettled) > 0)
		if len(unexplained) > 0 || len(unsettled) > 0 {
			t.Fatalf("unsettled state remains: %v / settlements=%v — environment must be PRESERVED (cleanup-blocked)", unexplained, unsettled)
		}
	})
}

// keyResolver resolves unknown records by querying the fake provider's
// recorded operations with the dispatch record's idempotency key (the
// provider-side correlation identity of the write).
type keyResolver struct {
	p     *oc17Provider
	store *repoappconn.OCStore
}

func (r keyResolver) QueryProvider(ctx context.Context, snap appconnectorsvc.ActionSnapshot, providerKey string) (appconnectorsvc.DispatchOutcome, error) {
	rec, err := r.store.GetOCDispatch(ctx, snap.TenantID, snap.ID)
	if err != nil {
		return appconnectorsvc.DispatchOutcome{}, err
	}
	if op, ok := r.p.lookupByKey(rec.Key); ok {
		return appconnectorsvc.DispatchOutcome{Status: appconn.ActionSucceeded, ProviderResult: "provider query: write confirmed; execution " + op.Key}, nil
	}
	return appconnectorsvc.DispatchOutcome{Status: appconn.ActionFailed, ProviderResult: "provider query: no such execution — write never landed"}, nil
}

// ---------------------------------------------------------------------------
// T17 finding 1 — production wiring parity probe (GatedOCClaims masks the
// T12 settle face). This is a REPORTED production defect; the probe is its
// executable reproduction and stays red until the one-line fix lands.
// ---------------------------------------------------------------------------

func TestOCIntegrationWiringSettleParity(t *testing.T) {
	dsn := os.Getenv("OC_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("blocked-env: OC_TEST_DATABASE_URL required")
	}
	db, _ := oc17OpenDB(t)
	s := &oc17Stack{db: db, ocStore: repoappconn.NewOCStore(db), installs: repoappconn.NewInstallationStore(db), actionStore: repoappconn.NewActionStore(db), gate: newOC17Gate(), provider: newOC17Provider(), tokenDir: t.TempDir(), journal: &oc17Journal{}}
	s.catalog = appconnectorsvc.NewOCCatalog(s.ocStore, s.ocStore, s.installs, s.installs)
	t.Cleanup(func() { s.provider.close() })
	oc17SeedFixture(t, s)
	src := repository.NewMCPOAuthBindingStore(db)
	guard := appconnectorsvc.NewOCSubjectGuard(src, appconnectorsvc.NewInstallationStateSource(s.installs), nil, s.ocStore)
	prod, prodWire, err := container.NewOCArmedActionService(s.actionStore, guard, s.gate, s.ocStore,
		container.OCConfig{Enabled: true, RuntimeAddr: s.provider.URL(), TokenDir: s.tokenDir, SlotOwner: "oc17-parity"},
		&http.Client{Timeout: 30 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	// Direct wiring-level assertion (R20): the claims the production
	// composition wires MUST satisfy the T12 settle face — ActionService
	// detects it by type assertion (ocDispatchSettleOf), so this is the
	// exact check that failed before the GatedOCClaims forwarding fix.
	settleFace, ok := prodWire.Claims().(appconnectorsvc.OCDispatchSettleSource)
	if !ok || settleFace == nil {
		t.Fatal("production wiring claims do not carry the T12 settle face (GatedOCClaims must forward FinishOCDispatch/MarkOCDispatchSettled)")
	}
	ctx := context.Background()
	catalog := appconnectorsvc.NewOCCatalog(s.ocStore, s.ocStore, s.installs, s.installs)
	preparer := appconnectorsvc.NewOCPreparer(prod, catalog, s.ocStore)
	id, err := preparer.PrepareOC(ctx, oc17OwnerA, "conn-a1", oc17ActionID, oc17Args("parity-probe"))
	if err != nil {
		t.Fatal(err)
	}
	row, _ := s.actionStore.FindAction(ctx, id)
	if err := prod.Approve(ctx, id, "owner-a", row.ArgsDigest); err != nil {
		t.Fatal(err)
	}
	if err := prod.Execute(ctx, id); err != nil {
		t.Fatalf("production-path execute: %v", err)
	}
	arow, _ := s.actionStore.FindAction(ctx, id)
	rec, rerr := s.ocStore.GetOCDispatch(ctx, oc17TenantA, id)
	if rerr != nil {
		t.Fatalf("dispatch record missing: %v", rerr)
	}
	t.Logf("OC17-EVIDENCE parity probe: action_state=%s record_state=%s settle_face_forwarded=true", arow.State, rec.State)
	// T12 contract: the durable record IS the linearization point and the
	// settlement outbox — a successful dispatch must settle the RECORD too,
	// or the 90s stale sweep parks every successful dispatch unknown (T16
	// alert noise) and the settlement outbox never drains. (Before the R20
	// fix this probe was the red T17-F1 reproduction: record stayed
	// "dispatched" while the action was "succeeded".)
	if arow.State == appconn.ActionSucceeded && rec.State != appconn.ActionSucceeded {
		t.Fatalf("T17-F1 regression: action %s is succeeded but its durable dispatch record is %q — the GatedOCClaims settle-face forwarding (internal/container/open_connector.go) is broken again", id, rec.State)
	}
	// The settled record must also carry the delivery mark (fence raised
	// past the action row's) so the settlement outbox does not re-deliver.
	marked, err := s.ocStore.GetOCDispatch(ctx, oc17TenantA, id)
	if err != nil {
		t.Fatal(err)
	}
	if marked.Fence <= arow.Fence {
		t.Fatalf("settlement delivery not marked: record fence %d <= action fence %d", marked.Fence, arow.Fence)
	}
}
