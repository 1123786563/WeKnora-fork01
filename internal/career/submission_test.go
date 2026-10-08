package career

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newSubmissionOffice opens a file-backed office with the hermetic export
// storage bound: submission tests need submittable exports to bind versions.
func newSubmissionOffice(t *testing.T, user string, tenant uint64) (*Office, *gorm.DB, *mapExportStorage, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "career-submission.db")), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	store := newMapExportStorage()
	office.SetExportStorage(store)
	office.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, office.ClaimSpace(ctx))
	return office, db, store, ctx
}

// submissionFixture is one durable application plus a submittable material
// export under the seeding scope — the minimum a submission binds to.
type submissionFixture struct {
	ApplicationID string
	OpportunityID string
	SnapshotID    string
	MaterialID    string
	ExportID      string
	Version       uint64
	ContentDigest string
	Revision      uint64 // profile head revision (stable across the chain)
}

// seedSubmissionFixture drives the whole chain under one scope: confirmed
// year, JD import, evaluation, application, material draft, confirmed
// version 1, and its verified submittable export.
func seedSubmissionFixture(t *testing.T, o *Office, ctx context.Context, seedID string) submissionFixture {
	t.Helper()
	o.SetApplicationTaskLinker(&fakeCareerApplicationLinker{})
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "education.graduation_year", "2027", seedID+"-year", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = o.Open(ctx)
	require.NoError(t, err)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: seedID + "-job", RawText: "仅限2027届。Go 服务端工程师。"})
	require.NoError(t, err)
	evaluation, err := o.EvaluateOpportunity(ctx, EvaluateInput{
		RequestID:     seedID + "-eval",
		OpportunityID: job.OpportunityID,
		SnapshotID:    job.SnapshotID,
	})
	require.NoError(t, err)
	application, err := o.CreateApplication(ctx, CreateApplicationInput{
		RequestID:        seedID + "-app",
		OpportunityID:    job.OpportunityID,
		SnapshotID:       job.SnapshotID,
		EvaluationID:     evaluation.EvaluationID,
		BatchIdentity:    seedID + "-batch",
		ExpectedRevision: view.Revision,
	})
	require.NoError(t, err)
	created, err := o.EditMaterial(ctx, editMaterialInput(
		materialSeed{OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID, Revision: view.Revision},
		seedID+"-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: seedID + "-confirm", MaterialID: created.MaterialID, ExpectedRevision: view.Revision})
	require.NoError(t, err)
	export, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: seedID + "-publish", MaterialID: created.MaterialID, Version: 1, ExpectedRevision: view.Revision})
	require.NoError(t, err)
	require.True(t, export.Submittable, "the fixture must publish a submittable export")
	return submissionFixture{
		ApplicationID: application.ApplicationID,
		OpportunityID: job.OpportunityID,
		SnapshotID:    job.SnapshotID,
		MaterialID:    created.MaterialID,
		ExportID:      export.ExportID,
		Version:       export.Version,
		ContentDigest: export.ContentDigest,
		Revision:      view.Revision,
	}
}

func submissionMoment() time.Time {
	moment, err := time.Parse(time.RFC3339, "2026-09-22T08:30:00Z")
	if err != nil {
		panic(err)
	}
	return moment
}

// submissionInput builds one record intent. versionUnknown=false binds the
// fixture's submittable export; versionUnknown=true records the explicit
// unknown marker.
func submissionInput(fx submissionFixture, requestID, channel string, versionUnknown bool, revision uint64) RecordSubmissionInput {
	moment := submissionMoment()
	input := RecordSubmissionInput{
		RequestID:        requestID,
		ApplicationID:    fx.ApplicationID,
		Channel:          channel,
		OccurredAt:       &moment,
		VersionUnknown:   versionUnknown,
		ExpectedRevision: revision,
	}
	if !versionUnknown {
		input.MaterialID = fx.MaterialID
		input.ExportID = fx.ExportID
	}
	return input
}

type storedSubmissionRow struct {
	ID               string
	TenantID         uint64
	UserID           string
	ApplicationID    string
	RequestID        string
	Fingerprint      string
	Channel          string
	OccurredAt       time.Time
	VersionConfirmed bool
	MaterialID       string
	ExportID         string
	Version          uint64
	ContentDigest    string
	Note             string
	Confirmer        string
	ReceiptBody      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func readSubmissionRows(t *testing.T, db *gorm.DB) []storedSubmissionRow {
	t.Helper()
	var rows []storedSubmissionRow
	require.NoError(t, db.Table("career_submissions").Order("created_at ASC").Find(&rows).Error)
	return rows
}

// explodingFetchTransport records any transport call; record_submission must
// never reach the network seam.
type explodingFetchTransport struct {
	called *bool
}

func (e explodingFetchTransport) Fetch(_ context.Context, _ ApprovedSource, _ string) (SourceFetchResult, error) {
	*e.called = true
	return SourceFetchResult{}, errors.New("record_submission must never fetch")
}

func TestRecordSubmissionBindsChannelTimeAndVersion(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1951)
	fx := seedSubmissionFixture(t, o, ctx, "bind")

	receipt, err := o.RecordSubmission(ctx, submissionInput(fx, "bind-1", SubmissionChannelEmail, false, fx.Revision))
	require.NoError(t, err)
	require.Equal(t, SubmissionKindRecorded, receipt.Kind)
	require.Equal(t, "bind-1", receipt.RequestID)
	require.Equal(t, fx.ApplicationID, receipt.ApplicationID)
	require.NotEmpty(t, receipt.SubmissionID)
	require.Equal(t, SubmissionChannelEmail, receipt.Channel)
	require.Equal(t, submissionMoment(), receipt.OccurredAt)
	require.True(t, receipt.VersionConfirmed)
	require.NotNil(t, receipt.BoundVersion)
	require.Equal(t, fx.MaterialID, receipt.BoundVersion.MaterialID)
	require.Equal(t, fx.ExportID, receipt.BoundVersion.ExportID)
	require.Equal(t, fx.Version, receipt.BoundVersion.Version)
	require.Equal(t, fx.ContentDigest, receipt.BoundVersion.ContentDigest)
	require.Equal(t, "owner-1", receipt.Confirmer)
	require.Equal(t, fx.Revision, receipt.Revision)

	rows := readSubmissionRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, receipt.SubmissionID, rows[0].ID)
	require.Equal(t, fx.ApplicationID, rows[0].ApplicationID)
	require.Equal(t, SubmissionChannelEmail, rows[0].Channel)
	require.Equal(t, submissionMoment(), rows[0].OccurredAt)
	require.True(t, rows[0].VersionConfirmed)
	require.Equal(t, fx.MaterialID, rows[0].MaterialID)
	require.Equal(t, fx.ExportID, rows[0].ExportID)
	require.EqualValues(t, fx.Version, rows[0].Version)
	require.Equal(t, fx.ContentDigest, rows[0].ContentDigest)
	require.Equal(t, "owner-1", rows[0].Confirmer)

	listed, err := o.ApplicationSubmissions(ctx, fx.ApplicationID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, receipt.SubmissionID, listed[0].SubmissionID)
}

func TestRecordSubmissionUnknownVersionRecordsExplicitUnconfirmed(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1952)
	fx := seedSubmissionFixture(t, o, ctx, "unk")

	receipt, err := o.RecordSubmission(ctx, submissionInput(fx, "unk-1", SubmissionChannelWeb, true, fx.Revision))
	require.NoError(t, err)
	require.False(t, receipt.VersionConfirmed, "unknown version must be recorded as explicitly unconfirmed")
	require.Nil(t, receipt.BoundVersion, "no version reference may be fabricated")

	rows := readSubmissionRows(t, db)
	require.Len(t, rows, 1)
	require.False(t, rows[0].VersionConfirmed)
	require.Empty(t, rows[0].MaterialID)
	require.Empty(t, rows[0].ExportID)
	require.Zero(t, rows[0].Version)
	require.Empty(t, rows[0].ContentDigest)

	// The unknown marker is durable: the stored receipt replays unchanged.
	replayed, err := o.FindSubmissionReceipt(ctx, "unk-1")
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
}

func TestRecordSubmissionOnlyAcceptsSubmittableVersions(t *testing.T) {
	o, _, _, ctx := newSubmissionOffice(t, "owner-1", 1953)
	fx := seedSubmissionFixture(t, o, ctx, "sub")

	// Neither a bound version nor the explicit unknown marker is invalid.
	unmarked := submissionInput(fx, "sub-none", SubmissionChannelEmail, false, fx.Revision)
	unmarked.MaterialID, unmarked.ExportID = "", ""
	_, err := o.RecordSubmission(ctx, unmarked)
	require.ErrorIs(t, err, ErrInvalidRequest)

	// A missing export never binds.
	missing := submissionInput(fx, "sub-missing", SubmissionChannelEmail, false, fx.Revision)
	missing.ExportID = "00000000-0000-0000-0000-0000000000ff"
	_, err = o.RecordSubmission(ctx, missing)
	require.ErrorIs(t, err, ErrExportNotFound)

	// A staged export (one format failed verification) is not submittable.
	stagedMaterial, err := o.EditMaterial(ctx, editMaterialInput(
		materialSeed{OpportunityID: fx.OpportunityID, SnapshotID: fx.SnapshotID, Revision: fx.Revision},
		"sub-edit-2", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "sub-confirm-2", MaterialID: stagedMaterial.MaterialID, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	o.failExportVerify = func(format string) error {
		if format == ExportFormatDOCX {
			return errors.New("staged on purpose")
		}
		return nil
	}
	staged, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "sub-publish-2", MaterialID: stagedMaterial.MaterialID, Version: 1, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, ExportStatusStaged, staged.Status)
	stagedBinding := submissionInput(fx, "sub-staged", SubmissionChannelEmail, false, fx.Revision)
	stagedBinding.MaterialID, stagedBinding.ExportID = stagedMaterial.MaterialID, staged.ExportID
	_, err = o.RecordSubmission(ctx, stagedBinding)
	require.ErrorIs(t, err, ErrExportNotSubmittable)

	// A revoked export is no longer submittable either.
	revoked, err := o.RevokeMaterialExport(ctx, RevokeMaterialExportInput{RequestID: "sub-revoke-1", MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, ExportStatusRevoked, revoked.Status)
	_, err = o.RecordSubmission(ctx, submissionInput(fx, "sub-revoked", SubmissionChannelEmail, false, fx.Revision))
	require.ErrorIs(t, err, ErrExportNotSubmittable)

	// The explicit unknown marker still records after those refusals.
	_, err = o.RecordSubmission(ctx, submissionInput(fx, "sub-unknown", SubmissionChannelOther, true, fx.Revision))
	require.NoError(t, err)
}

func TestRecordSubmissionRepeatConfirmationDoesNotDuplicate(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1954)
	fx := seedSubmissionFixture(t, o, ctx, "dup")

	first, err := o.RecordSubmission(ctx, submissionInput(fx, "dup-1", SubmissionChannelWeb, false, fx.Revision))
	require.NoError(t, err)

	// A second confirmation under a different request ID is a typed conflict:
	// the application keeps exactly one submission record and the original
	// receipt stays the replay source for its own request ID.
	_, err = o.RecordSubmission(ctx, submissionInput(fx, "dup-2", SubmissionChannelWeb, false, fx.Revision))
	require.ErrorIs(t, err, ErrSubmissionAlreadyConfirmed)
	_, err = o.RecordSubmission(ctx, submissionInput(fx, "dup-3", SubmissionChannelEmail, true, fx.Revision))
	require.ErrorIs(t, err, ErrSubmissionAlreadyConfirmed)

	rows := readSubmissionRows(t, db)
	require.Len(t, rows, 1, "repeated confirmation must not create a second submission record")
	require.Equal(t, first.SubmissionID, rows[0].ID)
	require.Equal(t, "dup-1", rows[0].RequestID)

	// The original request still replays its stored receipt.
	replayed, err := o.FindSubmissionReceipt(ctx, "dup-1")
	require.NoError(t, err)
	require.Equal(t, first, replayed)
}

func TestRecordSubmissionImmutableVersionReferenceResolvesExactly(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1955)
	fx := seedSubmissionFixture(t, o, ctx, "imm")

	receipt, err := o.RecordSubmission(ctx, submissionInput(fx, "imm-1", SubmissionChannelEmail, false, fx.Revision))
	require.NoError(t, err)
	snapshot := readSubmissionRows(t, db)[0]

	// A second version and export exist afterwards; the recorded reference
	// must still resolve to exactly version 1 of the moment of recording.
	_, err = o.EditMaterial(ctx, EditMaterialInput{RequestID: "imm-edit-2", MaterialID: fx.MaterialID, Body: exportFixtureBody(), ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "imm-confirm-2", MaterialID: fx.MaterialID, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	second, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "imm-publish-2", MaterialID: fx.MaterialID, Version: 2, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.True(t, second.Submittable)

	binding, err := o.SubmissionBoundVersion(ctx, receipt.SubmissionID)
	require.NoError(t, err)
	require.Equal(t, fx.MaterialID, binding.MaterialID)
	require.Equal(t, fx.ExportID, binding.ExportID)
	require.EqualValues(t, fx.Version, binding.Version)
	require.Equal(t, fx.ContentDigest, binding.ContentDigest)

	// The material version itself still resolves exactly.
	version, err := o.MaterialVersion(ctx, fx.MaterialID, fx.Version)
	require.NoError(t, err)
	require.EqualValues(t, fx.Version, version.Version)

	// Revoking the bound export changes download rights, never the recorded
	// reference: the row stays byte-identical.
	_, err = o.RevokeMaterialExport(ctx, RevokeMaterialExportInput{RequestID: "imm-revoke-1", MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	binding, err = o.SubmissionBoundVersion(ctx, receipt.SubmissionID)
	require.NoError(t, err)
	require.EqualValues(t, fx.Version, binding.Version)
	require.Equal(t, fx.ContentDigest, binding.ContentDigest)
	require.Equal(t, snapshot, readSubmissionRows(t, db)[0], "the submission row must never be rewritten")
}

func TestRecordSubmissionNeverPerformsOrInfersExternalAction(t *testing.T) {
	o, db, store, ctx := newSubmissionOffice(t, "owner-1", 1956)
	fx := seedSubmissionFixture(t, o, ctx, "pure")

	// Redeeming download grants never implies a submission.
	grant, err := o.MaterialExportGrant(ctx, fx.MaterialID, fx.ExportID, ExportFormatPDF, time.Minute)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		_, err = o.DownloadMaterialExport(ctx, fx.MaterialID, fx.ExportID, ExportFormatPDF,
			itoa(uint64(grant.ExpiresAt)), grant.Signature)
		require.NoError(t, err)
	}
	require.Empty(t, readSubmissionRows(t, db), "downloads must never infer a submission")

	// Recording must not touch any external seam: the transport fails the
	// test on any fetch, and a fresh linker counts every external call.
	transportCalled := false
	o.sourceTransport = explodingFetchTransport{called: &transportCalled}
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)
	filesBefore := len(store.files)

	receipt, err := o.RecordSubmission(ctx, submissionInput(fx, "pure-1", SubmissionChannelWeb, true, fx.Revision))
	require.NoError(t, err)
	require.False(t, transportCalled, "record_submission must not perform any external fetch")
	linker.mutex.Lock()
	ensureCalls, findCalls := linker.ensureCalls, linker.findCalls
	linker.mutex.Unlock()
	require.Zero(t, ensureCalls, "record_submission must not create or query external tasks")
	require.Zero(t, findCalls)
	require.Len(t, store.files, filesBefore, "record_submission must not render or store any file")

	progress, err := o.ApplicationProgress(ctx, fx.ApplicationID)
	require.NoError(t, err)
	require.Equal(t, ProgressStageSubmitted, progress.Stage)
	require.Len(t, progress.Events, 1)
	require.Equal(t, ProgressEventSubmitted, progress.Events[0].EventType)
	require.Equal(t, receipt.SubmissionID, progress.Events[0].SubmissionID)
	require.Len(t, readSubmissionRows(t, db), 1)
	require.NotEmpty(t, receipt.SubmissionID)
}

func TestRecordSubmissionProjectsLinkedProgressAndReplayOnce(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "known", true: "unknown"}[unknown], func(t *testing.T) {
			o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1970+uint64(boolInt(unknown)))
			fx := seedSubmissionFixture(t, o, ctx, "projection")
			input := submissionInput(fx, "projection-1", SubmissionChannelWeb, unknown, fx.Revision)
			first, err := o.RecordSubmission(ctx, input)
			require.NoError(t, err)
			replayed, err := o.RecordSubmission(ctx, input)
			require.NoError(t, err)
			require.Equal(t, first, replayed)
			view, err := o.ApplicationProgress(ctx, fx.ApplicationID)
			require.NoError(t, err)
			require.Equal(t, ProgressStageSubmitted, view.Stage)
			require.Len(t, view.Events, 1)
			event := view.Events[0]
			require.Equal(t, ProgressEventSubmitted, event.EventType)
			require.Equal(t, first.SubmissionID, event.SubmissionID)
			require.Equal(t, first.OccurredAt, event.OccurredAt)
			require.Equal(t, SubmissionChannelWeb, event.SubmissionChannel)
			require.NotNil(t, event.VersionConfirmed)
			require.Equal(t, !unknown, *event.VersionConfirmed)
			require.Len(t, readSubmissionRows(t, db), 1)
		})
	}
}

func TestRecordSubmissionProgressFailureRollsBackBothRecords(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1972)
	fx := seedSubmissionFixture(t, o, ctx, "rollback")
	o.afterSubmissionProgressPersist = func() error { return errors.New("injected progress write failure") }
	_, err := o.RecordSubmission(ctx, submissionInput(fx, "rollback-1", SubmissionChannelEmail, false, fx.Revision))
	require.ErrorContains(t, err, "injected progress write failure")
	require.Empty(t, readSubmissionRows(t, db))
	progress, err := o.ApplicationProgress(ctx, fx.ApplicationID)
	require.NoError(t, err)
	require.Empty(t, progress.Events)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func TestRecordSubmissionExactReplayAndChangedIntentConflict(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1957)
	fx := seedSubmissionFixture(t, o, ctx, "replay")

	receipt, err := o.RecordSubmission(ctx, submissionInput(fx, "replay-1", SubmissionChannelEmail, false, fx.Revision))
	require.NoError(t, err)

	replayed, err := o.RecordSubmission(ctx, submissionInput(fx, "replay-1", SubmissionChannelEmail, false, fx.Revision))
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	require.Len(t, readSubmissionRows(t, db), 1, "exact replay must not create a second record")

	// The same request ID with changed content is a typed conflict.
	changed := submissionInput(fx, "replay-1", SubmissionChannelOther, true, fx.Revision)
	_, err = o.RecordSubmission(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	require.Len(t, readSubmissionRows(t, db), 1)

	// The stored receipt replays byte-for-byte through the receipt seam.
	stored, err := o.FindSubmissionReceipt(ctx, "replay-1")
	require.NoError(t, err)
	require.Equal(t, receipt, stored)
}

func TestRecordSubmissionScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1958)
	fx := seedSubmissionFixture(t, o, ctx, "scope")

	// A different owner in their own tenant may never see owner-1's
	// application: the lookup answers not-found, never a foreign write.
	intruder := WithScope(context.Background(), Scope{UserID: "owner-2", TenantID: 1960})
	require.NoError(t, o.ClaimSpace(intruder))
	_, err := o.RecordSubmission(intruder, submissionInput(fx, "scope-1", SubmissionChannelEmail, true, 0))
	require.ErrorIs(t, err, ErrApplicationNotFound)
	_, err = o.ApplicationSubmissions(intruder, fx.ApplicationID)
	require.ErrorIs(t, err, ErrApplicationNotFound)
	_, err = o.FindSubmissionReceipt(intruder, "scope-1")
	require.ErrorIs(t, err, ErrReceiptNotFound)

	// A second user of the same tenant has no personal space at all.
	sameTenant := WithScope(context.Background(), Scope{UserID: "owner-3", TenantID: 1958})
	_, err = o.RecordSubmission(sameTenant, submissionInput(fx, "scope-2", SubmissionChannelEmail, true, 0))
	require.ErrorIs(t, err, ErrUnauthorized)

	// The owner's own record still resolves; a foreign submission ID is
	// invisible to them too.
	receipt, err := o.RecordSubmission(ctx, submissionInput(fx, "scope-3", SubmissionChannelEmail, false, fx.Revision))
	require.NoError(t, err)
	_, err = o.SubmissionBoundVersion(ctx, receipt.SubmissionID)
	require.NoError(t, err)
	_, err = o.SubmissionBoundVersion(intruder, receipt.SubmissionID)
	require.ErrorIs(t, err, ErrSubmissionNotFound)

	require.Len(t, readSubmissionRows(t, db), 1, "only the owner's confirmation may persist")
}

func TestRecordSubmissionRevisionConflictReturnsCurrentRevision(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1959)
	fx := seedSubmissionFixture(t, o, ctx, "rev")

	// A stale expected revision returns the current revision, not a blank 409.
	_, err := o.RecordSubmission(ctx, submissionInput(fx, "rev-1", SubmissionChannelEmail, true, fx.Revision+5))
	require.ErrorIs(t, err, ErrRevisionConflict)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.EqualValues(t, fx.Revision, conflict.CurrentRevision)
	require.Empty(t, readSubmissionRows(t, db), "a losing revision must not record")

	// The profile moving forward raises the head the conflict reports.
	_, err = o.Confirm(ctx, "skill.go", "Go", "rev-year-2", fx.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.RecordSubmission(ctx, submissionInput(fx, "rev-2", SubmissionChannelEmail, true, fx.Revision))
	require.ErrorAs(t, err, &conflict)
	require.EqualValues(t, fx.Revision+1, conflict.CurrentRevision)

	// Recording against the true head succeeds and reports that revision.
	receipt, err := o.RecordSubmission(ctx, submissionInput(fx, "rev-3", SubmissionChannelEmail, true, fx.Revision+1))
	require.NoError(t, err)
	require.EqualValues(t, fx.Revision+1, receipt.Revision)
	require.Len(t, readSubmissionRows(t, db), 1)
}

func TestRecordSubmissionUnknownOutcomeRecoversViaReceipt(t *testing.T) {
	o, db, _, ctx := newSubmissionOffice(t, "owner-1", 1961)
	fx := seedSubmissionFixture(t, o, ctx, "unknown")

	// Cancel the operation right after the row insert: the outcome of the
	// write is unknown to the caller, never a blind retry with a new ID.
	writeCtx, cancel := context.WithCancel(ctx)
	o.afterSubmissionPersist = func() error {
		cancel()
		return writeCtx.Err()
	}
	_, err := o.RecordSubmission(writeCtx, submissionInput(fx, "unknown-1", SubmissionChannelEmail, true, fx.Revision))
	require.ErrorIs(t, err, ErrOutcomeUnknown)
	var unknown *OutcomeUnknownError
	require.ErrorAs(t, err, &unknown)
	require.Equal(t, "unknown-1", unknown.RequestID)

	// The original request ID decides the truth: nothing committed here.
	_, err = o.FindSubmissionReceipt(ctx, "unknown-1")
	require.ErrorIs(t, err, ErrReceiptNotFound)

	// Retrying the SAME request ID is safe and produces exactly one record.
	o.afterSubmissionPersist = nil
	recovered, err := o.RecordSubmission(ctx, submissionInput(fx, "unknown-1", SubmissionChannelEmail, true, fx.Revision))
	require.NoError(t, err)
	require.Len(t, readSubmissionRows(t, db), 1)

	// The retry outcome and the receipt replay agree.
	replayed, err := o.FindSubmissionReceipt(ctx, "unknown-1")
	require.NoError(t, err)
	require.Equal(t, recovered, replayed)
	require.Equal(t, recovered.SubmissionID, readSubmissionRows(t, db)[0].ID)
}
