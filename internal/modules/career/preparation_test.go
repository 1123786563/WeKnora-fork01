package career

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newPreparationOffice opens a file-backed office with the hermetic export
// storage bound: preparation tests need a submittable export chain.
func newPreparationOffice(t *testing.T, user string, tenant uint64) (*Office, *gorm.DB, *mapExportStorage, context.Context) {
	t.Helper()
	return newSubmissionOffice(t, user, tenant)
}

// confirmNextVersion edits the fixture material with a marker body, confirms
// the next immutable version, and publishes its export.
func confirmNextVersion(t *testing.T, o *Office, ctx context.Context, fx submissionFixture, marker, requestID string) ExportReceipt {
	t.Helper()
	body := exportFixtureBody()
	body.Sections[0].Claims[0].Text = "2027 届毕业生（" + marker + "）"
	_, err := o.EditMaterial(ctx, EditMaterialInput{RequestID: requestID + "-edit", MaterialID: fx.MaterialID, Body: body, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: requestID + "-confirm", MaterialID: fx.MaterialID, ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.EqualValues(t, fx.Revision, view.Revision, "the fixture keeps a stable profile head")
	return publishVersion(t, o, ctx, fx, view.Revision, requestID)
}

func publishVersion(t *testing.T, o *Office, ctx context.Context, fx submissionFixture, revision uint64, requestID string) ExportReceipt {
	t.Helper()
	versions, err := o.MaterialVersions(ctx, fx.MaterialID)
	require.NoError(t, err)
	next := uint64(len(versions))
	export, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: requestID + "-publish", MaterialID: fx.MaterialID, Version: next, ExpectedRevision: revision})
	require.NoError(t, err)
	require.True(t, export.Submittable, "the fixture must publish a submittable export")
	return export
}

// failingPreparationGenerator is the injected seam failure.
type failingPreparationGenerator struct{ err error }

func (f failingPreparationGenerator) GeneratePreparation(_ context.Context, _ PreparationGenerationRequest) (MaterialBody, error) {
	return MaterialBody{}, f.err
}

// fixedBodyPreparationGenerator returns a canned body regardless of input.
type fixedBodyPreparationGenerator struct{ body MaterialBody }

func (f fixedBodyPreparationGenerator) GeneratePreparation(_ context.Context, _ PreparationGenerationRequest) (MaterialBody, error) {
	return f.body, nil
}

func preparationInput(fx submissionFixture, requestID, focus string, revision uint64) GeneratePreparationInput {
	return GeneratePreparationInput{RequestID: requestID, ApplicationID: fx.ApplicationID, Focus: focus, ExpectedRevision: revision}
}

func readPreparationRows(t *testing.T, db *gorm.DB) []preparationRecord {
	t.Helper()
	var rows []preparationRecord
	require.NoError(t, db.Table("career_preparations").Order("created_at ASC").Find(&rows).Error)
	return rows
}

func countMaterials(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table("career_materials").Count(&count).Error)
	return count
}

func TestPreparationAnchorsToActuallySubmittedVersion(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1971)
	fx := seedSubmissionFixture(t, o, ctx, "anchor")

	// Version 2 exists and is the one actually submitted.
	second := confirmNextVersion(t, o, ctx, fx, "V2", "anchor-v2")
	submission, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "anchor-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: second.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	// Version 3 appears afterwards: it is the latest but was never submitted.
	confirmNextVersion(t, o, ctx, fx, "V3", "anchor-v3")

	receipt, err := o.GeneratePreparation(ctx, preparationInput(fx, "anchor-1", PreparationFocusInterview, fx.Revision))
	require.NoError(t, err)
	require.Equal(t, PreparationKindGenerated, receipt.Kind)
	require.Equal(t, "anchor-1", receipt.RequestID)
	require.Equal(t, fx.ApplicationID, receipt.ApplicationID)
	require.Equal(t, PreparationFocusInterview, receipt.Focus)
	require.Equal(t, PreparationStatusDraft, receipt.Status)
	require.NotEmpty(t, receipt.PreparationID)

	// The anchor is exactly the submitted V2 binding — never V1 and never the
	// latest V3.
	require.Equal(t, submission.SubmissionID, receipt.Anchor.SubmissionID)
	require.Equal(t, fx.MaterialID, receipt.Anchor.MaterialID)
	require.Equal(t, second.ExportID, receipt.Anchor.ExportID)
	require.EqualValues(t, 2, receipt.Anchor.Version)
	require.Equal(t, second.ContentDigest, receipt.Anchor.ContentDigest)

	// The sources chain cites the application's frozen snapshot and the
	// confirmed fact keys the draft references.
	require.Equal(t, fx.OpportunityID, receipt.Sources.Snapshot.OpportunityID)
	require.Equal(t, fx.SnapshotID, receipt.Sources.Snapshot.SnapshotID)
	var snapshotDigest string
	require.NoError(t, db.Table("career_opportunity_snapshots").Select("raw_sha256").
		Where("id=?", fx.SnapshotID).Scan(&snapshotDigest).Error)
	require.Equal(t, snapshotDigest, receipt.Sources.Snapshot.SnapshotSHA256)
	require.Contains(t, receipt.Sources.FactKeys, "education.graduation_year")

	// The composed draft carries the submitted V2 claims, not the V3 rewrite.
	joined := ""
	for _, section := range receipt.Body.Sections {
		for _, claim := range section.Claims {
			joined += claim.Text
		}
	}
	require.Contains(t, joined, "（V2）")
	require.NotContains(t, joined, "（V3）")

	// The draft is durable as a material pinned to the application's frozen
	// snapshot.
	require.NotEmpty(t, receipt.MaterialID)
	material, err := o.Material(ctx, receipt.MaterialID)
	require.NoError(t, err)
	require.Equal(t, MaterialStatusDraft, material.Status)
	require.Equal(t, fx.OpportunityID, material.PinnedEvidence.OpportunityID)
	require.Equal(t, fx.SnapshotID, material.PinnedEvidence.SnapshotID)

	require.Len(t, readPreparationRows(t, db), 1)
}

func TestPreparationUnknownVersionRequiresPromptNotLatestGuess(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1972)
	fx := seedSubmissionFixture(t, o, ctx, "unk")
	confirmNextVersion(t, o, ctx, fx, "V2", "unk-v2")
	confirmNextVersion(t, o, ctx, fx, "V3", "unk-v3")
	// The submission is recorded with the explicit unknown marker: V3 is the
	// latest but must never be guessed.
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "unk-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelWeb,
		VersionUnknown: true, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	materialsBefore := countMaterials(t, db)

	_, err = o.GeneratePreparation(ctx, preparationInput(fx, "unk-1", PreparationFocusCoverLetter, fx.Revision))
	require.ErrorIs(t, err, ErrPreparationVersionUnknown)
	var prompt *PreparationVersionUnknownError
	require.ErrorAs(t, err, &prompt)
	require.Equal(t, fx.ApplicationID, prompt.ApplicationID)
	require.True(t, prompt.SubmissionRecorded, "the unknown marker was recorded, still not a version")

	require.Empty(t, readPreparationRows(t, db), "no preparation may persist without an anchored version")
	require.Equal(t, materialsBefore, countMaterials(t, db), "no draft material may be created from a guess")
	_, err = o.FindPreparationReceipt(ctx, "unk-1")
	require.ErrorIs(t, err, ErrReceiptNotFound)

	// An application with no submission at all answers the same prompt state.
	fresh := seedSubmissionFixture(t, o, ctx, "unk2")
	_, err = o.GeneratePreparation(ctx, preparationInput(fresh, "unk-2", PreparationFocusCoverLetter, fresh.Revision))
	require.ErrorIs(t, err, ErrPreparationVersionUnknown)
	require.ErrorAs(t, err, &prompt)
	require.Equal(t, fresh.ApplicationID, prompt.ApplicationID)
	require.False(t, prompt.SubmissionRecorded)
	require.Empty(t, readPreparationRows(t, db))
}

func TestPreparationClaimsOnlyConfirmedFactsAndSnapshot(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1973)
	fx := seedSubmissionFixture(t, o, ctx, "claims")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "claims-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)

	// The default composer only carries confirmed claims and snapshot
	// citations: generation succeeds.
	receipt, err := o.GeneratePreparation(ctx, preparationInput(fx, "claims-1", PreparationFocusCoverLetter, fx.Revision))
	require.NoError(t, err)
	for _, section := range receipt.Body.Sections {
		for _, claim := range section.Claims {
			if claim.NeedsReview {
				continue
			}
			require.NotEmpty(t, claim.FactKey, "an asserted claim must cite a confirmed fact")
		}
	}
	require.Equal(t, fx.SnapshotID, receipt.Sources.Snapshot.SnapshotID)

	// A generator that emits an unconfirmed claim is refused: typed failure,
	// no draft material, and the request stays recoverable.
	o.preparationGenerator = fixedBodyPreparationGenerator{body: materialBody(
		MaterialClaim{ClaimID: "bad-1", Text: "虚构的实习承诺", FactKey: "experience.internship"},
	)}
	materialsBefore := countMaterials(t, db)
	_, err = o.GeneratePreparation(ctx, preparationInput(fx, "claims-2", PreparationFocusCoverLetter, fx.Revision))
	require.ErrorIs(t, err, ErrPreparationGenerationFailed)
	require.Equal(t, materialsBefore, countMaterials(t, db), "an unconfirmed claim must not become a draft material")

	rows := readPreparationRows(t, db)
	require.Len(t, rows, 2, "the successful claim-cited draft and the refused request both persist")
	require.Equal(t, PreparationStatusFailed, rows[1].Status)
	require.Equal(t, MaterialFailureClaimUnconfirmed, rows[1].FailureCode)

	// The failed request is readable as a typed failure state, never a blank
	// success product.
	failed, err := o.FindPreparationReceipt(ctx, "claims-2")
	require.NoError(t, err)
	require.Equal(t, PreparationStatusFailed, failed.Status)
	require.Equal(t, MaterialFailureClaimUnconfirmed, failed.FailureCode)
	require.Empty(t, failed.MaterialID)
	require.Empty(t, failed.Body.Sections)
}

func TestPreparationDraftReviewableRevisionableWithSources(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1974)
	fx := seedSubmissionFixture(t, o, ctx, "revise")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "revise-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)

	receipt, err := o.GeneratePreparation(ctx, preparationInput(fx, "revise-1", PreparationFocusCoverLetter, fx.Revision))
	require.NoError(t, err)
	require.Equal(t, PreparationStatusDraft, receipt.Status)
	require.NotEmpty(t, receipt.MaterialID)
	originalBody := receipt.Body

	// The user revises the draft through the material seam: the revision is a
	// new draft on top, never an overwrite of the generated original.
	revised := originalBody
	revised.Sections = append([]MaterialSection(nil), originalBody.Sections...)
	revised.Sections[0].Content = "用户修订后的求职信正文"
	_, err = o.EditMaterial(ctx, EditMaterialInput{
		RequestID: "revise-2", MaterialID: receipt.MaterialID,
		OpportunityID: fx.OpportunityID, SnapshotID: fx.SnapshotID,
		Body: revised, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "revise-3", MaterialID: receipt.MaterialID, ExpectedRevision: fx.Revision})
	require.NoError(t, err)

	// A second revision confirms version 2; version 1 stays immutable and
	// readable with its sources.
	again := revised
	again.Sections[0].Content = "第二次修订"
	_, err = o.EditMaterial(ctx, EditMaterialInput{
		RequestID: "revise-4", MaterialID: receipt.MaterialID,
		OpportunityID: fx.OpportunityID, SnapshotID: fx.SnapshotID,
		Body: again, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "revise-5", MaterialID: receipt.MaterialID, ExpectedRevision: fx.Revision})
	require.NoError(t, err)

	versionOne, err := o.MaterialVersion(ctx, receipt.MaterialID, 1)
	require.NoError(t, err)
	require.Equal(t, "用户修订后的求职信正文", versionOne.Body.Sections[0].Content)
	versionTwo, err := o.MaterialVersion(ctx, receipt.MaterialID, 2)
	require.NoError(t, err)
	require.Equal(t, "第二次修订", versionTwo.Body.Sections[0].Content)

	// The generation receipt keeps its sources chain through replay.
	stored, err := o.FindPreparationReceipt(ctx, "revise-1")
	require.NoError(t, err)
	require.Equal(t, receipt, stored)
	require.EqualValues(t, fx.Version, stored.Anchor.Version)
	require.Equal(t, fx.SnapshotID, stored.Sources.Snapshot.SnapshotID)
	require.Len(t, readPreparationRows(t, db), 1, "revisions never create a second preparation record")
}

func TestPreparationNeverAutoSendsOrPromisesFacts(t *testing.T) {
	o, db, store, ctx := newPreparationOffice(t, "owner-1", 1975)
	fx := seedSubmissionFixture(t, o, ctx, "pure")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "pure-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)

	transportCalled := false
	o.sourceTransport = explodingFetchTransport{called: &transportCalled}
	linker := &fakeCareerApplicationLinker{}
	o.SetApplicationTaskLinker(linker)
	filesBefore := len(store.files)
	submissionsBefore := len(readSubmissionRows(t, db))

	receipt, err := o.GeneratePreparation(ctx, preparationInput(fx, "pure-1", PreparationFocusCoverLetter, fx.Revision))
	require.NoError(t, err)

	// Pure generation: no external fetch, no task calls, no rendered files,
	// and the recorded submission fact is untouched.
	require.False(t, transportCalled, "generation must not perform any external fetch")
	linker.mutex.Lock()
	ensureCalls, findCalls := linker.ensureCalls, linker.findCalls
	linker.mutex.Unlock()
	require.Zero(t, ensureCalls, "generation must not create or query external tasks")
	require.Zero(t, findCalls)
	require.Len(t, store.files, filesBefore, "generation must not render or store files")
	require.Len(t, readSubmissionRows(t, db), submissionsBefore, "generation must not touch submissions")

	// The body never promises facts: every asserted claim cites a confirmed
	// fact key and missing items stay explicit placeholders.
	for _, section := range receipt.Body.Sections {
		for _, claim := range section.Claims {
			if claim.NeedsReview {
				require.Equal(t, MaterialRiskMissingPlaceholder, reviewRiskCode(claim))
				continue
			}
			require.NotEmpty(t, claim.FactKey, "generation must not assert without a confirmed fact")
		}
	}
	require.NotEmpty(t, receipt.ReviewRisks, "missing items surface as explicit review risks")
}

func TestPreparationGenerationFailurePreservesRecoverableRequest(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1976)
	fx := seedSubmissionFixture(t, o, ctx, "fail")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "fail-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	materialsBefore := countMaterials(t, db)

	o.preparationGenerator = failingPreparationGenerator{err: errors.New("model unavailable")}
	_, err = o.GeneratePreparation(ctx, preparationInput(fx, "fail-1", PreparationFocusInterview, fx.Revision))
	require.ErrorIs(t, err, ErrPreparationGenerationFailed)

	// The request survives as a readable failed state; no blank success.
	rows := readPreparationRows(t, db)
	require.Len(t, rows, 1)
	require.Equal(t, PreparationStatusFailed, rows[0].Status)
	require.Equal(t, PreparationFailureGeneration, rows[0].FailureCode)
	require.Equal(t, materialsBefore, countMaterials(t, db), "a failed generation must not create a draft material")
	failed, err := o.FindPreparationReceipt(ctx, "fail-1")
	require.NoError(t, err)
	require.Equal(t, PreparationStatusFailed, failed.Status)
	require.Equal(t, PreparationFailureGeneration, failed.FailureCode)
	require.Empty(t, failed.Body.Sections)

	// Recovery reuses the SAME request ID: the retry succeeds and leaves
	// exactly one preparation record with one draft material.
	o.preparationGenerator = deterministicPreparationGenerator{}
	recovered, err := o.GeneratePreparation(ctx, preparationInput(fx, "fail-1", PreparationFocusInterview, fx.Revision))
	require.NoError(t, err)
	require.Equal(t, PreparationStatusDraft, recovered.Status)
	require.Equal(t, materialsBefore+1, countMaterials(t, db))
	require.Len(t, readPreparationRows(t, db), 1)

	replayed, err := o.FindPreparationReceipt(ctx, "fail-1")
	require.NoError(t, err)
	require.Equal(t, recovered, replayed)
}

func TestPreparationExactReplayAndChangedIntentConflict(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1977)
	fx := seedSubmissionFixture(t, o, ctx, "replay")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "replay-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)

	receipt, err := o.GeneratePreparation(ctx, preparationInput(fx, "replay-1", PreparationFocusCoverLetter, fx.Revision))
	require.NoError(t, err)

	replayed, err := o.GeneratePreparation(ctx, preparationInput(fx, "replay-1", PreparationFocusCoverLetter, fx.Revision))
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	require.Len(t, readPreparationRows(t, db), 1, "exact replay must not create a second record")

	changed := preparationInput(fx, "replay-1", PreparationFocusInterview, fx.Revision)
	_, err = o.GeneratePreparation(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
	require.Len(t, readPreparationRows(t, db), 1)

	stored, err := o.FindPreparationReceipt(ctx, "replay-1")
	require.NoError(t, err)
	require.Equal(t, receipt, stored)
}

func TestPreparationScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1978)
	fx := seedSubmissionFixture(t, o, ctx, "scope")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "scope-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	receipt, err := o.GeneratePreparation(ctx, preparationInput(fx, "scope-1", PreparationFocusCoverLetter, fx.Revision))
	require.NoError(t, err)

	// A different owner in their own tenant may never generate for or read
	// owner-1's application: lookups answer not-found.
	intruder := WithScope(context.Background(), Scope{UserID: "owner-2", TenantID: 1980})
	require.NoError(t, o.ClaimSpace(intruder))
	_, err = o.GeneratePreparation(intruder, preparationInput(fx, "scope-2", PreparationFocusCoverLetter, fx.Revision))
	require.ErrorIs(t, err, ErrApplicationNotFound)
	_, err = o.ApplicationPreparations(intruder, fx.ApplicationID)
	require.ErrorIs(t, err, ErrApplicationNotFound)
	_, err = o.FindPreparationReceipt(intruder, "scope-1")
	require.ErrorIs(t, err, ErrReceiptNotFound)

	// A second user of the same tenant has no personal space at all.
	sameTenant := WithScope(context.Background(), Scope{UserID: "owner-3", TenantID: 1978})
	_, err = o.GeneratePreparation(sameTenant, preparationInput(fx, "scope-3", PreparationFocusCoverLetter, 0))
	require.ErrorIs(t, err, ErrUnauthorized)

	// The owner's own preparations still resolve.
	listed, err := o.ApplicationPreparations(ctx, fx.ApplicationID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Equal(t, receipt.PreparationID, listed[0].PreparationID)
	require.Len(t, readPreparationRows(t, db), 1, "only the owner's preparation may persist")
}

// TestPreparationReserveRaceReconcilesThroughStoredRow pins ocr3-129: the
// losing side of a same-request-ID reserve race (unique-index violation on
// career_preparations) reconciles through the winner's durable row instead
// of leaking the raw database error as a 500.
func TestPreparationReserveRaceReconcilesThroughStoredRow(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1979)
	fx := seedSubmissionFixture(t, o, ctx, "race")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "race-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)
	input := preparationInput(fx, "race-1", PreparationFocusCoverLetter, fx.Revision)
	receipt, err := o.GeneratePreparation(ctx, input)
	require.NoError(t, err)

	var row preparationRecord
	require.NoError(t, db.Where("request_id = ?", "race-1").First(&row).Error)
	scope, err := getScope(ctx)
	require.NoError(t, err)

	// The losing twin replays the winner's stored receipt.
	stored, resolved, resolveErr := o.resolvePreparationReserveError(ctx, scope, input, row.Fingerprint, gorm.ErrDuplicatedKey)
	require.NoError(t, resolveErr)
	require.True(t, resolved)
	require.Equal(t, receipt.RequestID, stored.RequestID)
	require.Equal(t, receipt.MaterialID, stored.MaterialID)

	// The same request ID under different content stays a typed conflict.
	_, resolved, resolveErr = o.resolvePreparationReserveError(ctx, scope, input, "different-fingerprint", gorm.ErrDuplicatedKey)
	require.ErrorIs(t, resolveErr, ErrIdempotencyConflict)
	require.False(t, resolved)

	// No durable decision (the winner rolled back): the outcome stays
	// unknown, never a raw database error.
	missing := input
	missing.RequestID = "race-missing"
	var unknown *OutcomeUnknownError
	_, resolved, resolveErr = o.resolvePreparationReserveError(ctx, scope, missing, row.Fingerprint, gorm.ErrDuplicatedKey)
	require.ErrorAs(t, resolveErr, &unknown)
	require.False(t, resolved)

	// Non-race errors pass through untouched.
	boom := errors.New("boom")
	_, resolved, resolveErr = o.resolvePreparationReserveError(ctx, scope, input, row.Fingerprint, boom)
	require.Same(t, boom, resolveErr)
	require.False(t, resolved)
}

func TestPreparationRevisionConflictReturnsCurrentRevision(t *testing.T) {
	o, db, _, ctx := newPreparationOffice(t, "owner-1", 1979)
	fx := seedSubmissionFixture(t, o, ctx, "rev")
	_, err := o.RecordSubmission(ctx, RecordSubmissionInput{
		RequestID: "rev-sub", ApplicationID: fx.ApplicationID, Channel: SubmissionChannelEmail,
		MaterialID: fx.MaterialID, ExportID: fx.ExportID, ExpectedRevision: fx.Revision,
	})
	require.NoError(t, err)

	// A stale expected revision returns the current revision, not a blank 409.
	_, err = o.GeneratePreparation(ctx, preparationInput(fx, "rev-1", PreparationFocusCoverLetter, fx.Revision+5))
	require.ErrorIs(t, err, ErrRevisionConflict)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.EqualValues(t, fx.Revision, conflict.CurrentRevision)
	require.Empty(t, readPreparationRows(t, db), "a losing revision must not persist")

	// The profile moving forward raises the head the conflict reports.
	_, err = o.Confirm(ctx, "skill.go", "Go", "rev-year-2", fx.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	_, err = o.GeneratePreparation(ctx, preparationInput(fx, "rev-2", PreparationFocusCoverLetter, fx.Revision))
	require.ErrorAs(t, err, &conflict)
	require.EqualValues(t, fx.Revision+1, conflict.CurrentRevision)

	// Generating against the true head succeeds and reports that revision.
	receipt, err := o.GeneratePreparation(ctx, preparationInput(fx, "rev-3", PreparationFocusCoverLetter, fx.Revision+1))
	require.NoError(t, err)
	require.EqualValues(t, fx.Revision+1, receipt.Revision)
	require.Len(t, readPreparationRows(t, db), 1)
}

// reviewRiskCode derives the review risk code of one claim the way
// materialReviewRisks classifies placeholders.
func reviewRiskCode(claim MaterialClaim) string {
	if claim.FactKey == "" {
		return MaterialRiskMissingPlaceholder
	}
	return MaterialRiskNeedsReview
}

// The composed heading mixes a Chinese lead with CJK content: a raw
// heading[:256] byte cut splits a multi-byte rune and json.Marshal would
// persist U+FFFD into a heading frozen into immutable versions (ocr1-146).
func TestTruncatePreparationHeadingNeverSplitsRunes(t *testing.T) {
	heading := "面试准备（草稿）：" + strings.Repeat("深", 120)
	require.Greater(t, len(heading), maxMaterialHeadingBytes)

	truncated := truncatePreparationHeading(heading)
	require.LessOrEqual(t, len(truncated), maxMaterialHeadingBytes)
	require.True(t, utf8.ValidString(truncated), "truncation must stay on rune boundaries")
	require.NotEqual(t, heading[:maxMaterialHeadingBytes], truncated, "byte slicing would have split a rune")
	require.True(t, strings.HasPrefix(heading, truncated), "truncation keeps the longest valid prefix")

	encoded, err := json.Marshal(truncated)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "\ufffd", "no replacement character may be persisted")
}
