package career

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// H7: claim-free Career writers (row writes that never take a lifecycle
// claim) must fail closed once a deletion has been requested, exactly like
// claimed writers already do through admitLifecycleClaimTx. Otherwise rows
// committed after the purge step — or after the terminal receipt, which never
// re-runs its steps — become permanent orphans behind a "deleted" receipt.

func TestClaimFreeWritersFailClosedDuringPausedDeletion(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	remover := &fakeCareerTaskRemover{failErr: errors.New("workbench unavailable")}
	o.SetApplicationTaskRemover(remover)
	fx := seedExportChain(t, o, ctx, "fence-paused")

	partial, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-fence", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusPartial, partial.Status)

	_, err = o.ImportJD(ctx, ImportJDInput{RequestID: "jd-late", RawText: "仅限2027届。Go 服务端工程师。"})
	require.ErrorIs(t, err, ErrCareerDeleting, "a fresh claim-free write must fail closed while deletion is in flight")

	_, err = o.Propose(ctx, "availability", "周末可面试", "fence-r", fx.Revision, Source{Kind: "user"})
	require.ErrorIs(t, err, ErrCareerDeleting)

	remover.failErr = nil
	resumed, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-fence", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, resumed.Status)
	require.Zerof(t, countScopeRows(t, db, "career_opportunities"), "no orphan rows may survive the deleted receipt")
	require.Zerof(t, countScopeRows(t, db, "career_proposals"), "no orphan rows may survive the deleted receipt")
}

func TestClaimFreeWritersFailClosedAfterDeletedReceipt(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	fx := seedExportChain(t, o, ctx, "fence-post")

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-post", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)

	_, err = o.ImportJD(ctx, ImportJDInput{RequestID: "jd-post", RawText: "仅限2027届。后端工程师。"})
	require.ErrorIs(t, err, ErrCareerDeleting, "a deleted space must not accept claim-free writes back")

	_, err = o.ExportCareer(ctx, CareerExportInput{RequestID: "export-post", ExpectedRevision: receipt.Revision})
	require.ErrorIs(t, err, ErrCareerDeleting)

	require.Zerof(t, countScopeRows(t, db, "career_opportunities"), "no orphan rows after the receipt")
	require.Zerof(t, countScopeRows(t, db, "career_data_exports"), "no orphan rows after the receipt")
}

// The ordering contract for CreateApplication: the fresh-create branch must
// observe the deletion gate BEFORE its first Career commit. Today the linking
// row commits and the claim acquisition fails afterwards, stranding a row the
// client believes was rejected.
func TestCreateApplicationFreshRequestFailsClosedWhenDeletionRequested(t *testing.T) {
	o, db, ctx := newApplicationOffice(t, "owner-1", 1951)
	seed := seedApplicationEvaluation(t, o, ctx, "Go backend engineer", "2027", "fence-app")
	o.SetApplicationTaskLinker(&fakeCareerApplicationLinker{})
	s, err := getScope(ctx)
	require.NoError(t, err)

	// One unresolved claim parks the deletion in the busy state: admission is
	// closed, but the purge has not run and the seeded rows still exist.
	require.NoError(t, o.admitLifecycleClaim(ctx, s, "source_upload", "hold-fence", "fp"))
	_, err = o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-app-fence", ExpectedRevision: seed.Revision})
	require.ErrorIs(t, err, ErrCareerOperationsBusy)

	_, err = o.CreateApplication(ctx, applicationInput(seed, "app-late", "batch-late"))
	require.ErrorIs(t, err, ErrCareerDeleting)
	require.Empty(t, readApplicationRows(t, db), "a fresh application must not commit its row once deletion is requested")
}
