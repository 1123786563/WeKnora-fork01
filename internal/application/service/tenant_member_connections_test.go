package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
)

// fakeConnectionRevoker records RevokePersonalConnections calls.
type fakeConnectionRevoker struct {
	calls []struct {
		tenant uint64
		user   string
	}
	err error
}

func (f *fakeConnectionRevoker) RevokePersonalConnections(ctx context.Context, tenantID uint64, userID string) (int64, error) {
	f.calls = append(f.calls, struct {
		tenant uint64
		user   string
	}{tenantID, userID})
	return int64(len(f.calls)), f.err
}

// TestMemberLeaveRevokesPersonalConnections pins the A02 hook: a successful
// RemoveMember fires RevokePersonalConnections exactly once for the removed
// (tenant, user) pair, and the hook failure never fails the removal itself.
func TestMemberLeaveRevokesPersonalConnections(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	if err := repo.Create(ctx, &types.TenantMember{
		UserID: "u1", TenantID: 7, Role: types.TenantRoleContributor,
		Status: types.TenantMemberStatusActive, JoinedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	revoker := &fakeConnectionRevoker{}
	svc := NewTenantMemberService(repo, nil, nil, nil, WithPersonalConnectionRevoker(revoker))

	if err := svc.RemoveMember(ctx, "u1", 7); err != nil {
		t.Fatal(err)
	}
	if len(revoker.calls) != 1 {
		t.Fatalf("hook called %d times, want 1", len(revoker.calls))
	}
	if revoker.calls[0].tenant != 7 || revoker.calls[0].user != "u1" {
		t.Fatalf("hook called with %+v", revoker.calls[0])
	}

	// Hook failures are logged, not propagated: the membership row is
	// already soft-deleted and the resolver's membership check is the hard
	// gate.
	repo2 := newFakeRepo()
	if err := repo2.Create(ctx, &types.TenantMember{
		UserID: "u2", TenantID: 7, Role: types.TenantRoleContributor,
		Status: types.TenantMemberStatusActive, JoinedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	failing := &fakeConnectionRevoker{err: errors.New("store down")}
	svc2 := NewTenantMemberService(repo2, nil, nil, nil, WithPersonalConnectionRevoker(failing))
	if err := svc2.RemoveMember(ctx, "u2", 7); err != nil {
		t.Fatalf("hook failure failed the removal: %v", err)
	}
}

// TestMemberLeaveWithoutRevokerOptionKeepsWorking pins that existing
// four-argument construction is unaffected.
func TestMemberLeaveWithoutRevokerOptionKeepsWorking(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepo()
	if err := repo.Create(ctx, &types.TenantMember{
		UserID: "u1", TenantID: 7, Role: types.TenantRoleContributor,
		Status: types.TenantMemberStatusActive, JoinedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	svc := NewTenantMemberService(repo, nil, nil, nil)
	if err := svc.RemoveMember(ctx, "u1", 7); err != nil {
		t.Fatal(err)
	}
}
