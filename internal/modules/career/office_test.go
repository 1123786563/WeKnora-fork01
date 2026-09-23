package career

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func testOffice(t *testing.T) *Office {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	return office
}
func TestUnconfirmedProposalIsNotConfirmedFact(t *testing.T) {
	o := testOffice(t)
	ctx := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 1})
	_, err := o.Propose(ctx, "graduation_year", "2027", "req-1", 0)
	require.NoError(t, err)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Empty(t, view.Facts)
}
func TestConfirmIdempotencyAndRevisionConflict(t *testing.T) {
	o := testOffice(t)
	ctx := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 1})
	r, err := o.Confirm(ctx, "graduation_year", "2027", "req-1", 0)
	require.NoError(t, err)
	require.Equal(t, uint64(1), r.Revision)
	again, err := o.Confirm(ctx, "graduation_year", "2027", "req-1", 0)
	require.NoError(t, err)
	require.Equal(t, r.RequestID, again.RequestID)
	_, err = o.Confirm(ctx, "graduation_year", "2028", "req-1", 0)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	_, err = o.Confirm(ctx, "degree", "bachelor", "req-2", 0)
	require.ErrorIs(t, err, ErrRevisionConflict)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, uint64(1), conflict.CurrentRevision)
}
func TestOfficeIsScopedByUserAndTenant(t *testing.T) {
	o := testOffice(t)
	a := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 1})
	b := WithScope(context.Background(), Scope{UserID: "u2", TenantID: 1})
	_, err := o.Confirm(a, "degree", "bachelor", "r1", 0)
	require.NoError(t, err)
	v, err := o.Open(b)
	require.NoError(t, err)
	require.Empty(t, v.Facts)
	c := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 2})
	otherTenant, err := o.Open(c)
	require.NoError(t, err)
	require.Empty(t, otherTenant.Facts)
}
