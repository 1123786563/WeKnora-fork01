package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOwnerScopedStoreRejectsMissingScopeAndSeparatesTenantOwner(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	if err := store.Put(ctx, Scope{}, Record{ID: "profile"}); err != ErrUnauthorized {
		t.Fatalf("unauthenticated write = %v", err)
	}
	owner := Scope{TenantID: 1, OwnerID: "u1"}
	if err := store.Put(ctx, owner, Record{ID: "profile", Payload: []byte("private")}); err != nil {
		t.Fatal(err)
	}
	for _, scope := range []Scope{{TenantID: 2, OwnerID: "u1"}, {TenantID: 1, OwnerID: "u2"}} {
		if _, err := store.Get(ctx, scope, "profile"); err != ErrNotFound {
			t.Fatalf("foreign scope %+v read = %v", scope, err)
		}
	}
	if _, err := store.Get(ctx, owner, "missing"); err != ErrNotFound {
		t.Fatalf("missing resource = %v", err)
	}
}

func TestScopeFromContextRequiresAuthenticatedMatchingPrincipal(t *testing.T) {
	ctx := context.Background()
	if _, err := ScopeFromContext(ctx); err != ErrUnauthorized {
		t.Fatalf("empty context = %v", err)
	}
	ctx = types.WithCaller(ctx, types.Caller{TenantID: 1, UserID: "u1"})
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "u2"})
	if _, err := ScopeFromContext(ctx); err != ErrUnauthorized {
		t.Fatalf("mismatched principal = %v", err)
	}
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "u1"})
	scope, err := ScopeFromContext(ctx)
	if err != nil || scope != (Scope{TenantID: 1, OwnerID: "u1"}) {
		t.Fatalf("scope = %+v, err=%v", scope, err)
	}
}

func TestGormStoreSQLiteOwnerIsolationAndFoundationRoundTrip(t *testing.T) {
	db, migrationRoot := openCareerSQLite(t)
	up, err := os.ReadFile(filepath.Join(migrationRoot, "000124_career_foundation.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(up)).Error; err != nil {
		t.Fatalf("apply Career up migration: %v", err)
	}
	store := NewGormStore(db)
	ctx := context.Background()
	owner := Scope{TenantID: 12, OwnerID: "owner-a"}
	otherOwner := Scope{TenantID: 12, OwnerID: "owner-b"}
	otherTenant := Scope{TenantID: 99, OwnerID: "owner-a"}
	fact := Record{ID: "fact-1", Payload: []byte(`{"skill":"Go"}`)}
	if err := store.Put(ctx, owner, fact); err != nil {
		t.Fatal(err)
	}
	got, err := store.Get(ctx, owner, fact.ID)
	if err != nil || string(got.Payload) != string(fact.Payload) {
		t.Fatalf("fact=%+v err=%v", got, err)
	}
	for _, foreign := range []Scope{otherOwner, otherTenant} {
		if _, err := store.Get(ctx, foreign, fact.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign scope %+v read err=%v", foreign, err)
		}
	}
	receipt := Receipt{RequestID: "request-1", RequestHash: "hash-1", Status: "applied", Response: []byte(`{"revision":1}`)}
	if err := store.PutReceipt(ctx, owner, receipt); err != nil {
		t.Fatal(err)
	}
	gotReceipt, err := store.GetReceipt(ctx, owner, receipt.RequestID)
	if err != nil || string(gotReceipt.Response) != string(receipt.Response) {
		t.Fatalf("receipt=%+v err=%v", gotReceipt, err)
	}
	for _, foreign := range []Scope{otherOwner, otherTenant} {
		if _, err := store.GetReceipt(ctx, foreign, receipt.RequestID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign scope %+v read receipt err=%v", foreign, err)
		}
	}
	evidence := Evidence{ID: "e1", ResourceID: "resume", VersionID: "v2", Digest: "digest", Payload: []byte(`{"source":"confirmed"}`)}
	if err := store.AppendEvidence(ctx, owner, evidence); err != nil {
		t.Fatal(err)
	}
	gotEvidence, err := store.GetEvidence(ctx, owner, evidence.ID)
	if err != nil || string(gotEvidence.Payload) != string(evidence.Payload) {
		t.Fatalf("evidence=%+v err=%v", gotEvidence, err)
	}
	for _, foreign := range []Scope{otherOwner, otherTenant} {
		if _, err := store.GetEvidence(ctx, foreign, evidence.ID); !errors.Is(err, ErrNotFound) {
			t.Fatalf("foreign scope %+v read evidence err=%v", foreign, err)
		}
	}
	if err := db.Exec(`INSERT INTO career_profile_facts (tenant_id, owner_id, fact_id, payload) VALUES (700, 'missing', 'bad', '{}')`).Error; err == nil {
		t.Fatal("foreign-key violation accepted")
	}
	if err := db.Exec(`UPDATE career_evidence SET digest='changed' WHERE tenant_id=12 AND owner_id='owner-a' AND evidence_id='e1'`).Error; err == nil {
		t.Fatal("evidence update accepted")
	}
	if err := db.Exec(`DELETE FROM career_evidence WHERE tenant_id=12 AND owner_id='owner-a' AND evidence_id='e1'`).Error; err == nil {
		t.Fatal("evidence deletion accepted")
	}
	down, err := os.ReadFile(filepath.Join(migrationRoot, "000124_career_foundation.down.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(down)).Error; err != nil {
		t.Fatalf("apply Career down migration: %v", err)
	}
	var tableCount int64
	if err := db.Raw(`SELECT count(*) FROM sqlite_master WHERE type='table' AND name LIKE 'career_%'`).Scan(&tableCount).Error; err != nil || tableCount != 0 {
		t.Fatalf("Career tables after down=%d err=%v", tableCount, err)
	}
}

func openCareerSQLite(t *testing.T) (*gorm.DB, string) {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test source path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../.."))
	db, err := gorm.Open(sqlite.Open("file:"+filepath.Join(t.TempDir(), "career.db")+"?_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db, filepath.Join(root, "migrations/sqlite")
}

func TestScopeFromContextRejectsForeignExecutionTenant(t *testing.T) {
	ctx := types.WithCaller(context.Background(), types.Caller{TenantID: 12, UserID: "owner-a"})
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(12))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-a")
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "owner-a"})
	if scope, err := ScopeFromContext(ctx); err != nil || scope != (Scope{TenantID: 12, OwnerID: "owner-a"}) {
		t.Fatalf("matching caller scope = %+v, err=%v", scope, err)
	}
	switched := types.WithExecutionTenant(ctx, 99)
	if scope, err := ScopeFromContext(switched); err != ErrUnauthorized {
		t.Fatalf("foreign execution tenant returned scope %+v, err=%v; want ErrUnauthorized", scope, err)
	}
}

func TestScopeFromContextRejectsMissingAndMismatchedCaller(t *testing.T) {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(12))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "owner-a")
	ctx = types.WithPrincipal(ctx, types.Principal{Type: types.PrincipalWebUser, ID: "owner-a"})
	if _, err := ScopeFromContext(ctx); err != ErrUnauthorized {
		t.Fatalf("missing caller accepted: %v", err)
	}
	ctx = types.WithCaller(ctx, types.Caller{TenantID: 12, UserID: "owner-b"})
	if _, err := ScopeFromContext(ctx); err != ErrUnauthorized {
		t.Fatalf("caller/user mismatch accepted: %v", err)
	}
	ctx = types.WithCaller(ctx, types.Caller{TenantID: 0, UserID: ""})
	if _, err := ScopeFromContext(ctx); err != ErrUnauthorized {
		t.Fatalf("empty caller accepted: %v", err)
	}
}
