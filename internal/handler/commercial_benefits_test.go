package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	commercialsvc "github.com/Tencent/WeKnora/internal/application/service/commercial"
	commercial "github.com/Tencent/WeKnora/internal/commercial"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newQuotaTestDB builds one shared-cache SQLite with the counter table and
// seeds (tenant, dimension) counters.
func newQuotaTestDB(t *testing.T, seed map[string][2]int64 /* used, limit; limit<0 = NULL */) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	if err := db.Exec(`CREATE TABLE IF NOT EXISTS commercial_resource_counters (
		tenant_id INTEGER NOT NULL, resource TEXT NOT NULL, used INTEGER NOT NULL DEFAULT 0,
		hard_limit INTEGER NULL, PRIMARY KEY (tenant_id, resource)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1) // SQLite single-writer: the CAS harness convention
	}
	for key, seedRow := range seed {
		var limit any
		if seedRow[1] >= 0 {
			limit = seedRow[1]
		}
		if err := db.Exec(`INSERT INTO commercial_resource_counters (tenant_id, resource, used, hard_limit)
			VALUES (?, ?, ?, ?)`, parseCounterKey(key).tenant, parseCounterKey(key).dim, seedRow[0], limit).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

type counterKey struct {
	tenant uint64
	dim    string
}

func parseCounterKey(s string) counterKey {
	tenant, dim, _ := strings.Cut(s, ":")
	var t uint64
	for _, c := range tenant {
		t = t*10 + uint64(c-'0')
	}
	return counterKey{tenant: t, dim: dim}
}

// counterRow reads one counter as (used, limit, hasLimit).
func counterRow(t *testing.T, db *gorm.DB, tenantID uint64, dim string) (used, limit int64, hasLimit bool) {
	t.Helper()
	var row struct {
		Used      *int64
		HardLimit *int64
	}
	if err := db.Raw(`SELECT used, hard_limit FROM commercial_resource_counters
		WHERE tenant_id = ? AND resource = ?`, tenantID, dim).Scan(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Used != nil {
		used = *row.Used
	}
	if row.HardLimit != nil {
		limit, hasLimit = *row.HardLimit, true
	}
	return
}

// TestMemberQuotaOverLimitBlocksAddOnly: with members at the hard limit,
// AddMember answers 4xx quota_exceeded while ListMembers, UpdateRole and
// RemoveMember still succeed (超限状态: block-add, keep read/update/clean);
// removal frees the slot and a re-add passes.
func TestMemberQuotaOverLimitBlocksAddOnly(t *testing.T) {
	db := newQuotaTestDB(t, map[string][2]int64{"1:members": {2, 2}})
	guard := commercialsvc.NewQuotaGuard(db)

	ms := &stubMemberService{
		add: func(_ context.Context, userID string, tenantID uint64, role types.TenantRole, _ *string) (*types.TenantMember, error) {
			return &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()}, nil
		},
		updateRole: func(_ context.Context, _ string, _ uint64, _ types.TenantRole) error { return nil },
		remove:     func(_ context.Context, _ string, _ uint64) error { return nil },
	}
	us := &stubMemberUserService{
		getByEmail: func(_ context.Context, email string) (*types.User, error) {
			return &types.User{ID: "u-bob", Email: email, Username: "bob"}, nil
		},
	}
	h := newTestMemberHandler(ms, us)
	h.SetQuotaGuard(guard)
	rt := memberTestRouter(h)

	// Over limit: add is blocked with the closed token.
	body := map[string]any{"email": "bob@x.com", "role": "contributor"}
	w := doJSON(t, rt, http.MethodPost, "/tenants/1/members", body, "u-owner")
	if w.Code < 400 || w.Code >= 500 {
		t.Fatalf("over-limit add must answer 4xx, got %d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "quota_exceeded") {
		t.Fatalf("the refusal must carry the closed token quota_exceeded: %s", w.Body.String())
	}

	// Read / update / remove stay open by construction.
	if w := doJSON(t, rt, http.MethodGet, "/tenants/1/members", nil, "u-owner"); w.Code != http.StatusOK {
		t.Fatalf("list must stay open, got %d", w.Code)
	}
	if w := doJSON(t, rt, http.MethodPut, "/tenants/1/members/u-1", map[string]any{"role": "viewer"}, "u-owner"); w.Code != http.StatusOK {
		t.Fatalf("role update must stay open, got %d body=%s", w.Code, w.Body.String())
	}
	if w := doJSON(t, rt, http.MethodDelete, "/tenants/1/members/u-1", nil, "u-owner"); w.Code != http.StatusOK {
		t.Fatalf("removal must stay open, got %d body=%s", w.Code, w.Body.String())
	}
	// The removal decremented the counter — the re-add fits again.
	if used, _, _ := counterRow(t, db, 1, "members"); used != 1 {
		t.Fatalf("counter after removal = %d, want 1", used)
	}
	if w := doJSON(t, rt, http.MethodPost, "/tenants/1/members", body, "u-owner"); w.Code != http.StatusCreated {
		t.Fatalf("re-add after removal must pass, got %d body=%s", w.Code, w.Body.String())
	}
	if used, _, _ := counterRow(t, db, 1, "members"); used != 2 {
		t.Fatalf("counter after re-add = %d, want 2", used)
	}
}

// TestConcurrentAddsAtBoundary: exactly one slot left, four parallel adds —
// the CAS admits exactly one; the others answer quota_exceeded; the counter
// lands on the limit.
func TestConcurrentAddsAtBoundary(t *testing.T) {
	db := newQuotaTestDB(t, map[string][2]int64{"1:members": {1, 2}})
	guard := commercialsvc.NewQuotaGuard(db)
	ms := &stubMemberService{
		add: func(_ context.Context, userID string, tenantID uint64, role types.TenantRole, _ *string) (*types.TenantMember, error) {
			return &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()}, nil
		},
	}
	us := &stubMemberUserService{
		getByEmail: func(_ context.Context, email string) (*types.User, error) {
			return &types.User{ID: "u-" + email, Email: email}, nil
		},
	}
	h := newTestMemberHandler(ms, us)
	h.SetQuotaGuard(guard)
	rt := memberTestRouter(h)

	const racers = 4
	var okCount, refused int64
	started := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-started
			w := doJSON(t, rt, http.MethodPost, "/tenants/1/members",
				map[string]any{"email": fmt.Sprintf("racer-%d@x.com", i), "role": "viewer"}, "u-owner")
			switch {
			case w.Code == http.StatusCreated:
				atomic.AddInt64(&okCount, 1)
			case w.Code >= 400 && w.Code < 500 && strings.Contains(w.Body.String(), "quota_exceeded"):
				atomic.AddInt64(&refused, 1)
			default:
				t.Errorf("unexpected racer outcome: %d %s", w.Code, w.Body.String())
			}
		}(i)
	}
	close(started)
	wg.Wait()
	if okCount != 1 || refused != racers-1 {
		t.Fatalf("exactly one add may land at the boundary: ok=%d refused=%d", okCount, refused)
	}
	if used, limit, _ := counterRow(t, db, 1, "members"); used != limit {
		t.Fatalf("counter must land exactly on the limit: used=%d limit=%d", used, limit)
	}
}

// TestGuardNotWiredFailsOpen: a nil guard (older deployments, commercial
// stack unwired) never blocks member adds.
func TestGuardNotWiredFailsOpen(t *testing.T) {
	ms := &stubMemberService{
		add: func(_ context.Context, userID string, tenantID uint64, role types.TenantRole, _ *string) (*types.TenantMember, error) {
			return &types.TenantMember{UserID: userID, TenantID: tenantID, Role: role, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()}, nil
		},
	}
	us := &stubMemberUserService{
		getByEmail: func(_ context.Context, email string) (*types.User, error) {
			return &types.User{ID: "u-bob", Email: email}, nil
		},
	}
	h := newTestMemberHandler(ms, us) // no SetQuotaGuard — nil guard
	w := doJSON(t, memberTestRouter(h), http.MethodPost, "/tenants/1/members",
		map[string]any{"email": "bob@x.com", "role": "contributor"}, "u-owner")
	if w.Code != http.StatusCreated {
		t.Fatalf("nil guard must fail open, got %d body=%s", w.Code, w.Body.String())
	}
}

// TestAddMemberFailureReleasesReservation: a member insert failure after a
// successful reserve runs the compensating decrement — the counter cannot
// leak a phantom hold.
func TestAddMemberFailureReleasesReservation(t *testing.T) {
	db := newQuotaTestDB(t, map[string][2]int64{"1:members": {0, 5}})
	guard := commercialsvc.NewQuotaGuard(db)
	ms := &stubMemberService{
		add: func(_ context.Context, _ string, _ uint64, _ types.TenantRole, _ *string) (*types.TenantMember, error) {
			return nil, errors.New("insert failed")
		},
	}
	us := &stubMemberUserService{
		getByEmail: func(_ context.Context, email string) (*types.User, error) {
			return &types.User{ID: "u-bob", Email: email}, nil
		},
	}
	h := newTestMemberHandler(ms, us)
	h.SetQuotaGuard(guard)
	w := doJSON(t, memberTestRouter(h), http.MethodPost, "/tenants/1/members",
		map[string]any{"email": "bob@x.com", "role": "contributor"}, "u-owner")
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("insert failure must surface 500, got %d", w.Code)
	}
	if used, _, _ := counterRow(t, db, 1, "members"); used != 0 {
		t.Fatalf("the compensating decrement must release the hold, used=%d", used)
	}
}

// countingGuard proves read/export paths never touch the growth gate.
type countingGuard struct {
	mu     sync.Mutex
	calls  []string
	refuse bool
}

func (g *countingGuard) ReserveGrowth(_ context.Context, _ uint64, dimension string, delta int64) (func(), error) {
	g.mu.Lock()
	g.calls = append(g.calls, fmt.Sprintf("%s:%d", dimension, delta))
	refuse := g.refuse
	g.mu.Unlock()
	if refuse && delta > 0 {
		return nil, commercial.ErrQuotaGrowthRefused
	}
	return func() {}, nil
}

func (g *countingGuard) snapshot() []string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return append([]string(nil), g.calls...)
}

// sanity: countingGuard implements the seam interface.
var _ commercial.ResourceQuotaGuard = (*countingGuard)(nil)
