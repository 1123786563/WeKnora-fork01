package career

import (
	"context"
	"encoding/json"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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
	replayed, e := o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 1, Source{Kind: "user_confirmation"})
	require.NoError(t, e)
	confirmedJSON, err := json.Marshal(confirmed)
	require.NoError(t, err)
	replayedJSON, err := json.Marshal(replayed)
	require.NoError(t, err)
	require.JSONEq(t, string(confirmedJSON), string(replayedJSON))
	_, e = o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 1, Source{Kind: "edited_confirmation"})
	require.ErrorIs(t, e, ErrIdempotencyConflict)
	_, e = o.Act(ctx, "confirm_proposal", p.Proposal.ID, "", "", "c1", 2, Source{Kind: "user_confirmation"})
	require.ErrorIs(t, e, ErrIdempotencyConflict)
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
	require.Equal(t, set.Revision, set.Changes[len(set.Changes)-1].Revision)
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
	require.Equal(t, secondClientChanges.Revision, secondClientChanges.Changes[0].Revision)
	_, err = o.Changes(ctx, 3)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, uint64(2), conflict.CurrentRevision)
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

func TestConcurrentSameRequestIDReplaysCommittedReceiptAcrossConnections(t *testing.T) {
	dsn := "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "career.db")) + "?_busy_timeout=5000&_journal_mode=WAL"
	open := func() *gorm.DB {
		db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
		require.NoError(t, err)
		sqlDB, err := db.DB()
		require.NoError(t, err)
		sqlDB.SetMaxOpenConns(1)
		return db
	}
	db1 := open()
	o1, err := NewOffice(db1)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{UserID: "u1", TenantID: 1})
	require.NoError(t, o1.ClaimSpace(ctx))
	db2 := open()
	o2, err := NewOffice(db2)
	require.NoError(t, err)
	t.Cleanup(func() {
		if raw, e := db1.DB(); e == nil {
			_ = raw.Close()
		}
		if raw, e := db2.DB(); e == nil {
			_ = raw.Close()
		}
	})

	newBarrier := func() func() {
		var mu sync.Mutex
		arrived := 0
		bothMissed := make(chan struct{})
		return func() {
			mu.Lock()
			arrived++
			if arrived == 2 {
				close(bothMissed)
			}
			mu.Unlock()
			<-bothMissed
		}
	}
	barrier := newBarrier()
	o1.afterReceiptMiss = barrier
	o2.afterReceiptMiss = barrier
	type result struct {
		receipt Receipt
		err     error
	}
	results := make(chan result, 2)
	for _, office := range []*Office{o1, o2} {
		go func(office *Office) {
			r, e := office.Confirm(ctx, "degree", "bachelor", "concurrent-1", 0, Source{Kind: "user"})
			results <- result{r, e}
		}(office)
	}
	r1, r2 := <-results, <-results
	require.NoError(t, r1.err)
	require.NoError(t, r2.err)
	j1, err := json.Marshal(r1.receipt)
	require.NoError(t, err)
	j2, err := json.Marshal(r2.receipt)
	require.NoError(t, err)
	require.JSONEq(t, string(j1), string(j2))
	view, err := o1.Open(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(1), view.Revision)
	history, err := o1.History(ctx, "degree")
	require.NoError(t, err)
	require.Len(t, history, 1)

	// Both calls now overlap with the same request ID but different content.
	// Exactly one payload can own the receipt; the other must conflict.
	barrier = newBarrier()
	o1.afterReceiptMiss = barrier
	o2.afterReceiptMiss = barrier
	results = make(chan result, 2)
	for _, request := range []struct {
		office *Office
		value  string
	}{{o1, "master"}, {o2, "doctorate"}} {
		request := request
		go func() {
			r, e := request.office.Confirm(ctx, "degree", request.value, "concurrent-2", 1, Source{Kind: "user"})
			results <- result{r, e}
		}()
	}
	r1, r2 = <-results, <-results
	if r1.err == nil {
		require.ErrorIs(t, r2.err, ErrIdempotencyConflict)
	} else {
		require.ErrorIs(t, r1.err, ErrIdempotencyConflict)
		require.NoError(t, r2.err)
	}

	// Different request IDs still obey the revision CAS and must not be
	// converted into receipt replay when one concurrent mutation wins.
	barrier = newBarrier()
	o1.afterReceiptMiss = barrier
	o2.afterReceiptMiss = barrier
	results = make(chan result, 2)
	go func() {
		r, e := o1.Confirm(ctx, "city", "Beijing", "distinct-1", 2, Source{Kind: "user"})
		results <- result{r, e}
	}()
	go func() {
		r, e := o2.Confirm(ctx, "city", "Shanghai", "distinct-2", 2, Source{Kind: "user"})
		results <- result{r, e}
	}()
	r1, r2 = <-results, <-results
	conflicts := 0
	for _, result := range []result{r1, r2} {
		if result.err == nil {
			continue
		}
		var revisionConflict *RevisionConflictError
		require.ErrorAs(t, result.err, &revisionConflict)
		require.Equal(t, uint64(3), revisionConflict.CurrentRevision)
		conflicts++
	}
	require.Equal(t, 1, conflicts)
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
