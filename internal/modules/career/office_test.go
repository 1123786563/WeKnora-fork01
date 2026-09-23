package career

import (
	"context"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"testing"
)

func testOffice(t *testing.T) (*Office, context.Context) {
	t.Helper()
	db, e := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, e)
	o, e := NewOffice(db)
	require.NoError(t, e)
	ctx := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 1})
	require.NoError(t, o.ClaimSpace(ctx))
	return o, ctx
}
func TestUnconfirmedProposalIsNotConfirmedFact(t *testing.T) {
	o, ctx := testOffice(t)
	r, e := o.Propose(ctx, "graduation_year", "2027", "req-1", 0, Source{Kind: "resume_extraction", ReferenceID: "file-1"})
	require.NoError(t, e)
	require.Equal(t, "proposed", r.Kind)
	require.NotEmpty(t, r.Proposal.ID)
	v, e := o.Open(ctx)
	require.NoError(t, e)
	require.Empty(t, v.Facts)
	require.Len(t, v.Proposals, 1)
}
func TestConfirmProposalResolvesAndRetainsConfirmationHistory(t *testing.T) {
	o, ctx := testOffice(t)
	p, e := o.Propose(ctx, "graduation_year", "2027", "p1", 0, Source{Kind: "resume_extraction", ReferenceID: "file-1"})
	require.NoError(t, e)
	confirmed, e := o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 1, Source{Kind: "user_confirmation"})
	require.NoError(t, e)
	require.Equal(t, "confirmed", confirmed.Kind)
	view, e := o.Open(ctx)
	require.NoError(t, e)
	require.Len(t, view.Facts, 1)
	require.Empty(t, view.Proposals)
	require.Equal(t, "resume_extraction", view.Facts[0].Source.Kind)
	require.Equal(t, "u1", view.Facts[0].Confirmation.UserID)
	history, e := o.History(ctx, "graduation_year")
	require.NoError(t, e)
	require.Len(t, history, 1)
	require.Equal(t, "file-1", history[0].Source.ReferenceID)
	require.Equal(t, uint64(2), history[0].Revision)
	again, e := o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c2", 2, Source{Kind: "user_confirmation"})
	require.ErrorIs(t, e, ErrProposalResolved)
	_ = again
}
func TestDismissProposalAndChangesAreDurableOrderedEvents(t *testing.T) {
	o, ctx := testOffice(t)
	p, e := o.Propose(ctx, "degree", "bachelor", "p1", 0, Source{Kind: "interview"})
	require.NoError(t, e)
	_, e = o.Act(ctx, "dismiss", p.Proposal.ID, "", "", "d1", 1, Source{Kind: "user_confirmation"})
	require.NoError(t, e)
	set, e := o.Changes(ctx, 0)
	require.NoError(t, e)
	require.Equal(t, uint64(2), set.Revision)
	require.Len(t, set.Changes, 2)
	require.Equal(t, uint64(1), set.Changes[0].Revision)
	require.Equal(t, "proposed", set.Changes[0].Kind)
	require.Equal(t, "dismissed", set.Changes[1].Kind)
	require.Equal(t, "u1", set.Changes[1].Proposal.Confirmation.UserID)
	require.Equal(t, "user_confirmation", set.Changes[1].Proposal.ResolutionSource.Kind)
	view, e := o.Open(ctx)
	require.NoError(t, e)
	require.Empty(t, view.Proposals)
}
func TestConfirmedFactVersionsAreImmutable(t *testing.T) {
	o, ctx := testOffice(t)
	_, e := o.Confirm(ctx, "graduation_year", "2027", "c1", 0, Source{Kind: "user"})
	require.NoError(t, e)
	_, e = o.Confirm(ctx, "graduation_year", "2028", "c2", 1, Source{Kind: "user"})
	require.NoError(t, e)
	history, e := o.History(ctx, "graduation_year")
	require.NoError(t, e)
	require.Len(t, history, 2)
	require.Equal(t, "2027", history[0].Value)
	require.Equal(t, "2028", history[1].Value)
	changes, err := o.Changes(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, uint64(2), changes.Revision)
	require.Len(t, changes.Changes, 2)
	secondClientChanges, err := o.Changes(ctx, changes.Changes[0].Revision)
	require.NoError(t, err)
	require.Len(t, secondClientChanges.Changes, 1)
	require.Equal(t, "2028", secondClientChanges.Changes[0].Fact.Value)
}
func TestRevisionConflictHasCurrentValueAndIdempotency(t *testing.T) {
	o, ctx := testOffice(t)
	r, e := o.Confirm(ctx, "degree", "bachelor", "r1", 0, Source{Kind: "user"})
	require.NoError(t, e)
	again, e := o.Confirm(ctx, "degree", "bachelor", "r1", 0, Source{Kind: "user"})
	require.NoError(t, e)
	require.Equal(t, r.Kind, again.Kind)
	_, e = o.Confirm(ctx, "degree", "master", "r1", 0, Source{Kind: "user"})
	require.ErrorIs(t, e, ErrIdempotencyConflict)
	_, e = o.Confirm(ctx, "city", "Beijing", "r2", 0, Source{Kind: "user"})
	var conflict *RevisionConflictError
	require.ErrorAs(t, e, &conflict)
	require.Equal(t, uint64(1), conflict.CurrentRevision)
}
func TestOfficeRejectsUnclaimedOrCrossOwnerCareerSpace(t *testing.T) {
	o, _ := testOffice(t)
	other := WithScope(context.Background(), Scope{UserID: "u2", TenantID: 1})
	_, e := o.Open(other)
	require.ErrorIs(t, e, ErrUnauthorized)
	require.ErrorIs(t, o.ClaimSpace(other), ErrUnauthorized)
	tenant := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 2})
	_, e = o.Open(tenant)
	require.ErrorIs(t, e, ErrUnauthorized)
}

func TestCareerFactsStayIsolatedAcrossUsersAndTenants(t *testing.T) {
	o, ctx := testOffice(t)
	other := WithScope(context.Background(), Scope{UserID: "u2", TenantID: 2})
	require.NoError(t, o.ClaimSpace(other))
	_, err := o.Confirm(other, "degree", "master", "other-fact", 0, Source{Kind: "user"})
	require.NoError(t, err)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Empty(t, view.Facts)
	otherView, err := o.Open(other)
	require.NoError(t, err)
	require.Len(t, otherView.Facts, 1)
}
