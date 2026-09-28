package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
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
