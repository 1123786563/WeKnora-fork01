package service

import (
	"context"
	"errors"
	"testing"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/require"
)

// stubKnowledgeLookup injects source-document lookup outcomes at the service
// boundary (GetKnowledgeByIDOnly), mirroring the fault injection described in
// #3714: not-found vs cancelled/deadline/DB failures must be classified
// differently by the wiki ingest pipeline.
type stubKnowledgeLookup struct {
	interfaces.KnowledgeService
	kn  *types.Knowledge
	err error
}

func (s *stubKnowledgeLookup) GetKnowledgeByIDOnly(context.Context, string) (*types.Knowledge, error) {
	return s.kn, s.err
}

func TestIsKnowledgeGoneClassification(t *testing.T) {
	live := &types.Knowledge{ID: "k-1", ParseStatus: types.ParseStatusCompleted}
	tests := []struct {
		name      string
		stub      *stubKnowledgeLookup
		wantGone  bool
		wantErr   bool // any error expected (DB failure has no sentinel)
		wantErrIs error
	}{
		{"explicit not-found counts as gone", &stubKnowledgeLookup{err: apprepo.ErrKnowledgeNotFound}, true, false, nil},
		{"nil knowledge counts as gone", &stubKnowledgeLookup{}, true, false, nil},
		{"deleting parse state counts as gone", &stubKnowledgeLookup{kn: &types.Knowledge{ID: "k-1", ParseStatus: types.ParseStatusDeleting}}, true, false, nil},
		{"cancelled parse state counts as gone", &stubKnowledgeLookup{kn: &types.Knowledge{ID: "k-1", ParseStatus: types.ParseStatusCancelled}}, true, false, nil},
		{"live knowledge is not gone", &stubKnowledgeLookup{kn: live}, false, false, nil},
		{"ctx cancelled is not gone", &stubKnowledgeLookup{err: context.Canceled}, false, true, context.Canceled},
		{"ctx deadline exceeded is not gone", &stubKnowledgeLookup{err: context.DeadlineExceeded}, false, true, context.DeadlineExceeded},
		{"database error is not gone", &stubKnowledgeLookup{err: errors.New("database unavailable")}, false, true, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &wikiIngestService{knowledgeSvc: tc.stub}
			gone, err := svc.isKnowledgeGone(context.Background(), "kb-1", "k-1")
			require.Equal(t, tc.wantGone, gone)
			if tc.wantErrIs != nil {
				require.ErrorIs(t, err, tc.wantErrIs)
			} else if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

	// Empty knowledge ID has no row to lose.
	svc := &wikiIngestService{}
	gone, err := svc.isKnowledgeGone(context.Background(), "kb-1", "")
	require.NoError(t, err)
	require.True(t, gone)
}

// TestMapOneDocumentPreservesLookupFailures pins the Map-side contract of
// #3714: a failed source lookup must surface as an error so the batch files
// the op into failedOps (durable pending row survives for retry). Returning
// (nil, nil, nil) here would look like "source deleted, skip" and the batch
// would trim the op without ever generating its pages.
func TestMapOneDocumentPreservesLookupFailures(t *testing.T) {
	for _, lookupErr := range []error{context.Canceled, context.DeadlineExceeded, errors.New("database unavailable")} {
		t.Run(lookupErr.Error(), func(t *testing.T) {
			svc := &wikiIngestService{knowledgeSvc: &stubKnowledgeLookup{err: lookupErr}}
			result, updates, err := svc.mapOneDocument(context.Background(), nil,
				WikiIngestPayload{TenantID: 7, KnowledgeBaseID: "kb-1"},
				WikiPendingOp{KnowledgeID: "k-1", Op: WikiOpIngest}, nil)
			require.ErrorIs(t, err, lookupErr)
			require.Nil(t, result)
			require.Nil(t, updates)
		})
	}
}

// TestMapOneDocumentSkipsDeletedKnowledge pins the legitimate deletion path:
// an actual not-found result still yields the terminal skip (nil, nil, nil),
// so deleted documents keep their cheap no-op behavior.
func TestMapOneDocumentSkipsDeletedKnowledge(t *testing.T) {
	svc := &wikiIngestService{knowledgeSvc: &stubKnowledgeLookup{err: apprepo.ErrKnowledgeNotFound}}
	result, updates, err := svc.mapOneDocument(context.Background(), nil,
		WikiIngestPayload{TenantID: 7, KnowledgeBaseID: "kb-1"},
		WikiPendingOp{KnowledgeID: "k-1", Op: WikiOpIngest}, nil)
	require.NoError(t, err)
	require.Nil(t, result)
	require.Nil(t, updates)
}

func TestFilterLiveUpdatesClassification(t *testing.T) {
	updates := []SlugUpdate{
		{Slug: "entity/example", Type: types.WikiPageTypeEntity, KnowledgeID: "live-doc"},
		{Slug: "entity/example", Type: types.WikiPageTypeEntity, KnowledgeID: "gone-doc"},
		{Slug: "entity/other", Type: "retract", KnowledgeID: "gone-doc"},
		{Slug: "summary/x", Type: "summary", KnowledgeID: ""},
	}
	t.Run("deletions drop additions but keep retracts", func(t *testing.T) {
		svc := &wikiIngestService{knowledgeSvc: &liveOnlyLookup{
			live: map[string]bool{"live-doc": true, "": true},
		}}
		filtered, err := svc.filterLiveUpdates(context.Background(), "kb-1", updates)
		require.NoError(t, err)
		// The live-doc addition and the empty-kid summary survive; the
		// gone-doc addition is dropped; the gone-doc retract is KEPT
		// (retracts actively clean up and never depend on liveness).
		require.Len(t, filtered, 3)
		for _, u := range filtered {
			require.NotEqual(t, types.WikiPageTypeEntity+"-gone", u.Type)
			if u.Type == types.WikiPageTypeEntity {
				require.Equal(t, "live-doc", u.KnowledgeID)
			}
		}
	})
	t.Run("lookup failure fails instead of dropping additions", func(t *testing.T) {
		svc := &wikiIngestService{knowledgeSvc: &stubKnowledgeLookup{err: context.Canceled}}
		filtered, err := svc.filterLiveUpdates(context.Background(), "kb-1", updates)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, filtered)
	})
}

// liveOnlyLookup answers liveness per knowledge id: true means live, false
// means "not found".
type liveOnlyLookup struct {
	interfaces.KnowledgeService
	live map[string]bool
}

func (s *liveOnlyLookup) GetKnowledgeByIDOnly(_ context.Context, id string) (*types.Knowledge, error) {
	if s.live[id] {
		return &types.Knowledge{ID: id, ParseStatus: types.ParseStatusCompleted}, nil
	}
	return nil, apprepo.ErrKnowledgeNotFound
}

// TestReduceSlugUpdatesPreservesLookupFailures pins the Reduce-side contract
// of #3714: a failed source lookup must fail the slug (the egReduce caller
// then records its knowledge ids as unapplied and the batch re-queues them)
// instead of dropping the additions and reporting a successful no-op.
func TestReduceSlugUpdatesPreservesLookupFailures(t *testing.T) {
	for _, lookupErr := range []error{context.Canceled, context.DeadlineExceeded, errors.New("database unavailable")} {
		t.Run(lookupErr.Error(), func(t *testing.T) {
			svc := &wikiIngestService{knowledgeSvc: &stubKnowledgeLookup{err: lookupErr}}
			changed, affectedType, additionFailed, err := svc.reduceSlugUpdates(context.Background(), nil, "kb-1",
				"entity/example",
				[]SlugUpdate{{Slug: "entity/example", Type: types.WikiPageTypeEntity, KnowledgeID: "k-1"}},
				7, nil, nil)
			require.ErrorIs(t, err, lookupErr)
			require.False(t, changed)
			require.Empty(t, affectedType)
			require.False(t, additionFailed)
		})
	}
}
