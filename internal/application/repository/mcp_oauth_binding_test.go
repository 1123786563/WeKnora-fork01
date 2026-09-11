package repository

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	appconnector "github.com/Tencent/WeKnora/internal/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/application/repository/appconnector"
	"github.com/Tencent/WeKnora/internal/types"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newBindingStore(t *testing.T) *MCPOAuthBindingStore {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// Single pooled connection, mirroring commercial/budget_test.go: without
	// it, concurrent multi-statement transactions over separate shared-cache
	// connections fail with SQLITE_LOCKED ("database table is locked"), which
	// busy_timeout does not retry. SQLite is single-writer anyway, and the
	// conditional used=false UPDATE — never a process mutex — still decides
	// the single redemption winner.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&MCPOAuthBindingRow{},
		&appconnectorrepo.AppVersion{},
		&appconnectorrepo.InstallationRow{},
		&appconnectorrepo.ConnectionRow{},
		&types.TenantMember{},
		&types.MCPOAuthToken{},
	); err != nil {
		t.Fatal(err)
	}
	return NewMCPOAuthBindingStore(db)
}

func seedBindingFixture(t *testing.T, s *MCPOAuthBindingStore, tenant uint64, actor string) {
	t.Helper()
	if err := s.db.Create(&appconnectorrepo.InstallationRow{
		ID: "inst-1", TenantID: tenant, AppID: "app-x", AppVersion: "1.0.0",
		State: appconnector.InstallationActive, Version: 1,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.db.Create(&types.TenantMember{
		UserID: actor, TenantID: tenant, Role: types.TenantRoleContributor,
		Status: types.TenantMemberStatusActive, JoinedAt: time.Now(),
	}).Error; err != nil {
		t.Fatal(err)
	}
}

func issueBinding(t *testing.T, s *MCPOAuthBindingStore, tenant uint64, actor, state string, ttl time.Duration) {
	t.Helper()
	if err := s.IssueBindingState(context.Background(), appconnector.OAuthBinding{
		State: state, InstallationID: "inst-1", ActorID: actor, TenantID: tenant,
		ExpiresAt: time.Now().Add(ttl),
	}, "svc-1"); err != nil {
		t.Fatal(err)
	}
}

func exchangedToken() *types.MCPOAuthToken {
	return &types.MCPOAuthToken{AccessToken: "at-secret-value", RefreshToken: "rt-secret-value", TokenType: "Bearer"}
}

func countConnections(t *testing.T, s *MCPOAuthBindingStore) int64 {
	t.Helper()
	var n int64
	if err := s.db.Model(&appconnectorrepo.ConnectionRow{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// TestMCPOAuthBindingCallbackHappyPathBindsConnection pins the happy path:
// the state is consumed, the token is persisted encrypted at rest via the
// existing helpers, and the connection stores ONLY the reference.
func TestMCPOAuthBindingCallbackHappyPathBindsConnection(t *testing.T) {
	t.Setenv("SYSTEM_AES_KEY", strings.Repeat("k", 32))
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	issueBinding(t, s, 7, "u1", "st-1", time.Minute)

	conn, err := s.CompleteBinding(ctx, 7, "st-1", "u1", exchangedToken())
	if err != nil {
		t.Fatal(err)
	}
	if conn.Kind != appconnector.ConnectionKindPersonal || conn.OwnerID != "u1" ||
		conn.State != appconnector.ConnectionActive || conn.TenantID != 7 || conn.AuthVersion != 1 {
		t.Fatalf("unexpected connection: %+v", conn)
	}
	if conn.CredentialRef != CredentialRefPrefix+"svc-1" {
		t.Fatalf("connection must store only the reference, got %q", conn.CredentialRef)
	}
	// The stored token must be ciphertext, not plaintext.
	var raw map[string]interface{}
	if err := s.db.Table("mcp_oauth_tokens").Take(&raw).Error; err != nil {
		t.Fatal(err)
	}
	at, _ := raw["access_token"].(string)
	if !strings.HasPrefix(at, "enc:v1:") || strings.Contains(at, "at-secret-value") {
		t.Fatalf("access token not encrypted at rest: %q", at)
	}
}

// TestMCPOAuthBindingStateReplayRejected pins one-time semantics: the same
// state presented twice binds exactly one connection.
func TestMCPOAuthBindingStateReplayRejected(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	issueBinding(t, s, 7, "u1", "st-1", time.Minute)

	if _, err := s.CompleteBinding(ctx, 7, "st-1", "u1", exchangedToken()); err != nil {
		t.Fatal(err)
	}
	_, err := s.CompleteBinding(ctx, 7, "st-1", "u1", exchangedToken())
	if !errors.Is(err, ErrOAuthBindingInvalid) {
		t.Fatalf("replay accepted: %v", err)
	}
	if n := countConnections(t, s); n != 1 {
		t.Fatalf("replay bound a second connection: %d", n)
	}
}

// TestMCPOAuthBindingExpiryRejected pins that an expired state is refused
// and binds nothing.
func TestMCPOAuthBindingExpiryRejected(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	issueBinding(t, s, 7, "u1", "st-old", time.Minute)
	if err := s.db.Model(&MCPOAuthBindingRow{}).Where("state = ?", "st-old").
		Update("expires_at", time.Now().Add(-time.Minute)).Error; err != nil {
		t.Fatal(err)
	}

	_, err := s.CompleteBinding(ctx, 7, "st-old", "u1", exchangedToken())
	if !errors.Is(err, ErrOAuthBindingInvalid) {
		t.Fatalf("expired state accepted: %v", err)
	}
	if n := countConnections(t, s); n != 0 {
		t.Fatalf("expired state bound a connection: %d", n)
	}
}

// TestMCPOAuthBindingWrongTenantActorRejected pins the cross-space and
// wrong-actor boundaries at the persistence layer.
func TestMCPOAuthBindingWrongTenantActorRejected(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	issueBinding(t, s, 7, "u1", "st-1", time.Minute)

	if _, err := s.CompleteBinding(ctx, 8, "st-1", "u1", exchangedToken()); !errors.Is(err, ErrOAuthBindingInvalid) {
		t.Fatalf("cross-tenant callback accepted: %v", err)
	}
	if _, err := s.CompleteBinding(ctx, 7, "st-1", "attacker", exchangedToken()); !errors.Is(err, ErrOAuthBindingInvalid) {
		t.Fatalf("wrong actor accepted: %v", err)
	}
	if n := countConnections(t, s); n != 0 {
		t.Fatalf("rejected callback still bound: %d", n)
	}
}

// TestMCPOAuthBindingRemovedMemberRejected pins the same-transaction
// membership check: a removed member's callback fails with a distinct error
// and leaves the state unconsumed.
func TestMCPOAuthBindingRemovedMemberRejected(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	// Only some other user holds a membership; the binding's initiator
	// "u1" was removed before their callback arrived.
	seedBindingFixture(t, s, 7, "someone-else")
	issueBinding(t, s, 7, "u1", "st-1", time.Minute)

	_, err := s.CompleteBinding(ctx, 7, "st-1", "u1", exchangedToken())
	if !errors.Is(err, ErrOAuthBindingActorNotMember) {
		t.Fatalf("removed member accepted: %v", err)
	}
	var row MCPOAuthBindingRow
	if err := s.db.Where("state = ?", "st-1").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Used {
		t.Fatal("failed callback consumed the state")
	}
	if n := countConnections(t, s); n != 0 {
		t.Fatalf("removed member bound a connection: %d", n)
	}
}

// TestMCPOAuthBindingFailedCallbackLeavesStateUnused pins transactional
// rollback: when the post-consume step fails (installation disabled), the
// Used flag does not stick.
func TestMCPOAuthBindingFailedCallbackLeavesStateUnused(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	issueBinding(t, s, 7, "u1", "st-1", time.Minute)
	if err := s.db.Model(&appconnectorrepo.InstallationRow{}).
		Where("id = ?", "inst-1").
		Update("state", appconnector.InstallationDisabled).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := s.CompleteBinding(ctx, 7, "st-1", "u1", exchangedToken()); !errors.Is(err, ErrOAuthBindingInvalid) {
		t.Fatalf("disabled installation accepted: %v", err)
	}
	var row MCPOAuthBindingRow
	if err := s.db.Where("state = ?", "st-1").Take(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Used {
		t.Fatal("rolled-back callback consumed the state")
	}
}

// TestMCPOAuthMemberLeaveRevokesPersonalNotSpace pins the member-removal
// hook semantics: personal connections are revoked with an auth_version
// bump; space connections are untouched and never default to personal.
func TestMCPOAuthMemberLeaveRevokesPersonalNotSpace(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	conns := []appconnectorrepo.ConnectionRow{
		{TenantID: 7, ID: "c-personal", InstallationID: "inst-1", Kind: appconnector.ConnectionKindPersonal,
			OwnerID: "u1", CredentialRef: CredentialRefPrefix + "svc-1", State: appconnector.ConnectionActive, AuthVersion: 1},
		{TenantID: 7, ID: "c-space", InstallationID: "inst-1", Kind: appconnector.ConnectionKindSpace,
			OwnerID: "u1", CredentialRef: CredentialRefPrefix + "svc-2", State: appconnector.ConnectionActive, AuthVersion: 1},
		{TenantID: 8, ID: "c-other", InstallationID: "inst-8", Kind: appconnector.ConnectionKindPersonal,
			OwnerID: "u1", CredentialRef: CredentialRefPrefix + "svc-3", State: appconnector.ConnectionActive, AuthVersion: 1},
	}
	for i := range conns {
		if err := s.db.Create(&conns[i]).Error; err != nil {
			t.Fatal(err)
		}
	}

	n, err := s.RevokePersonalConnections(ctx, 7, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("revoked %d connections, want exactly the tenant-7 personal one", n)
	}
	personal, err := s.FindConnectionByID(ctx, "c-personal")
	if err != nil {
		t.Fatal(err)
	}
	if personal.State != appconnector.ConnectionRevoked || personal.AuthVersion != 2 {
		t.Fatalf("personal connection not revoked with version bump: %+v", personal)
	}
	space, err := s.FindConnectionByID(ctx, "c-space")
	if err != nil {
		t.Fatal(err)
	}
	if space.State != appconnector.ConnectionActive || space.AuthVersion != 1 || space.Kind != appconnector.ConnectionKindSpace {
		t.Fatalf("space connection disturbed by member leave: %+v", space)
	}
	other, err := s.FindConnectionByID(ctx, "c-other")
	if err != nil {
		t.Fatal(err)
	}
	if other.State != appconnector.ConnectionActive {
		t.Fatalf("other tenant's connection disturbed: %+v", other)
	}
}

// TestMCPOAuthBindingConsumeConcurrentSingleWinner pins that two concurrent
// callbacks presenting the same state yield exactly one bound connection.
func TestMCPOAuthBindingConsumeConcurrentSingleWinner(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	issueBinding(t, s, 7, "u1", "st-race", time.Minute)

	var wg sync.WaitGroup
	successes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.CompleteBinding(ctx, 7, "st-race", "u1", exchangedToken()); err == nil {
				successes <- 1
			}
		}()
	}
	wg.Wait()
	close(successes)
	won := 0
	for range successes {
		won++
	}
	if won != 1 {
		t.Fatalf("concurrent state redemption had %d winners, want 1", won)
	}
	if n := countConnections(t, s); n != 1 {
		t.Fatalf("concurrent redemption bound %d connections, want 1", n)
	}
}

// TestMCPOAuthBindingRefreshLeaseSingleWinner pins that the EXISTING refresh
// lease yields exactly one winner across concurrent refreshers.
func TestMCPOAuthBindingRefreshLeaseSingleWinner(t *testing.T) {
	s := newBindingStore(t)
	ctx := context.Background()
	seedBindingFixture(t, s, 7, "u1")
	tok := exchangedToken()
	tok.TenantID = 7
	tok.PrincipalType = types.PrincipalWebUser
	tok.PrincipalID = "u1"
	tok.ServiceID = "svc-1"
	if err := s.db.Create(tok).Error; err != nil {
		t.Fatal(err)
	}
	c := appconnector.Connection{TenantID: 7, OwnerID: "u1", CredentialRef: CredentialRefPrefix + "svc-1"}

	const workers = 8
	until := time.Now().Add(time.Minute)
	var wg sync.WaitGroup
	wins := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			won, err := s.TryAcquireRefreshLease(ctx, c, "lease-"+string(rune('a'+i)), until)
			if err != nil {
				t.Errorf("lease attempt: %v", err)
			}
			wins <- won
		}(i)
	}
	wg.Wait()
	close(wins)
	winners := 0
	for w := range wins {
		if w {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("refresh lease had %d winners, want exactly 1", winners)
	}
}
