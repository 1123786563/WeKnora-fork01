package career

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type materialSeed struct {
	OpportunityID string
	SnapshotID    string
	Revision      uint64
}

func newMaterialOffice(t *testing.T, user string, tenant uint64) (*Office, *gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "career-material.db")), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, office.ClaimSpace(ctx))
	return office, db, ctx
}

func seedMaterialEvidence(t *testing.T, o *Office, ctx context.Context, year, seedID string) materialSeed {
	t.Helper()
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "education.graduation_year", year, seedID+"-year", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = o.Open(ctx)
	require.NoError(t, err)
	job, err := o.ImportJD(ctx, ImportJDInput{RequestID: seedID + "-job", RawText: "仅限" + year + "届。"})
	require.NoError(t, err)
	return materialSeed{OpportunityID: job.OpportunityID, SnapshotID: job.SnapshotID, Revision: view.Revision}
}

func materialBody(claims ...MaterialClaim) MaterialBody {
	return MaterialBody{Sections: []MaterialSection{{Heading: "summary", Content: "结构化正文", Claims: claims}}}
}

func graduated(claimID string) MaterialClaim {
	return MaterialClaim{ClaimID: claimID, Text: "2027 届毕业生", FactKey: "education.graduation_year"}
}

func editMaterialInput(seed materialSeed, requestID string, body MaterialBody) EditMaterialInput {
	return EditMaterialInput{
		RequestID:        requestID,
		OpportunityID:    seed.OpportunityID,
		SnapshotID:       seed.SnapshotID,
		Body:             body,
		ExpectedRevision: seed.Revision,
	}
}

func TestEditMaterialFreezesOpportunityAndProfileEvidence(t *testing.T) {
	o, db, ctx := newMaterialOffice(t, "owner-1", 1801)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "freeze")

	receipt, err := o.EditMaterial(ctx, editMaterialInput(seed, "freeze-1", materialBody(graduated("c1"))))
	require.NoError(t, err)
	require.Equal(t, MaterialKindEdited, receipt.Kind)
	require.Equal(t, seed.OpportunityID, receipt.PinnedEvidence.OpportunityID)
	require.Equal(t, seed.SnapshotID, receipt.PinnedEvidence.SnapshotID)
	require.Equal(t, seed.Revision, receipt.PinnedEvidence.ProfileRevision)
	require.NotEmpty(t, receipt.PinnedEvidence.SnapshotSHA256, "frozen evidence must pin the snapshot digest")

	// Profile moves forward and a newer opportunity snapshot exists; neither may
	// rewrite the frozen material evidence.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.go", "Go", "freeze-2", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	newer, err := o.ImportJD(ctx, ImportJDInput{RequestID: "freeze-job-2", RawText: "仅限2028届。"})
	require.NoError(t, err)

	stored, err := o.Material(ctx, receipt.MaterialID)
	require.NoError(t, err)
	require.Equal(t, seed.OpportunityID, stored.PinnedEvidence.OpportunityID)
	require.Equal(t, seed.SnapshotID, stored.PinnedEvidence.SnapshotID)
	require.Equal(t, seed.Revision, stored.PinnedEvidence.ProfileRevision)
	require.Equal(t, receipt.PinnedEvidence.SnapshotSHA256, stored.PinnedEvidence.SnapshotSHA256)
	require.NotEqual(t, newer.SnapshotID, stored.PinnedEvidence.SnapshotID)

	// An edit cannot redirect the frozen evidence onto another snapshot.
	redirect := editMaterialInput(seed, "freeze-3", materialBody(graduated("c1")))
	redirect.MaterialID = receipt.MaterialID
	redirect.OpportunityID = newer.OpportunityID
	redirect.SnapshotID = newer.SnapshotID
	view, err = o.Open(ctx)
	require.NoError(t, err)
	redirect.ExpectedRevision = view.Revision
	_, err = o.EditMaterial(ctx, redirect)
	require.ErrorIs(t, err, ErrInvalidRequest)
	after, err := o.Material(ctx, receipt.MaterialID)
	require.NoError(t, err)
	require.Equal(t, seed.SnapshotID, after.PinnedEvidence.SnapshotID)
	var rows int64
	require.NoError(t, db.Table("career_materials").Where("id=?", receipt.MaterialID).Count(&rows).Error)
	require.EqualValues(t, 1, rows)
}

func TestMaterialClaimsLinkOnlyConfirmedFacts(t *testing.T) {
	o, db, ctx := newMaterialOffice(t, "owner-1", 1802)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "claims")

	// A pending proposal is not a confirmed fact; a claim referencing it must be
	// refused before any material body persists.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Propose(ctx, "experience.internship", "某公司三个月实习", "claims-propose", view.Revision, Source{Kind: "resume_extraction", ReferenceID: "resume-1"})
	require.NoError(t, err)

	unconfirmed := editMaterialInput(seed, "claims-1", materialBody(MaterialClaim{ClaimID: "c1", Text: "曾在某公司实习三个月", FactKey: "experience.internship"}))
	_, err = o.EditMaterial(ctx, unconfirmed)
	require.ErrorIs(t, err, ErrMaterialClaimUnconfirmed)
	var created int64
	require.NoError(t, db.Table("career_materials").Count(&created).Error)
	require.Zero(t, created, "no material may persist with a claim on an unconfirmed fact")

	// Once the user confirms the fact, the same claim may enter the body:
	// confirmation is what gates a claim, and claims keep linking the fact.
	view, err = o.Open(ctx)
	require.NoError(t, err)
	confirmedLater, err := o.Confirm(ctx, "experience.internship", "某公司三个月实习", "claims-confirm", view.Revision, Source{Kind: "user_confirmation"})
	require.NoError(t, err)
	require.Greater(t, confirmedLater.Revision, seed.Revision)
	nowConfirmed := editMaterialInput(seed, "claims-1b", materialBody(MaterialClaim{ClaimID: "c1", Text: "曾在某公司实习三个月", FactKey: "experience.internship"}))
	nowConfirmed.ExpectedRevision = confirmedLater.Revision
	confirmedReceipt, err := o.EditMaterial(ctx, nowConfirmed)
	require.NoError(t, err)
	require.Equal(t, "experience.internship", confirmedReceipt.Body.Sections[0].Claims[0].FactKey)
	// The frozen creation evidence pins the revision observed at creation.
	require.Equal(t, confirmedLater.Revision, confirmedReceipt.PinnedEvidence.ProfileRevision)

	// A claim linked to a fact confirmed before the freeze is accepted too.
	valid := editMaterialInput(seed, "claims-2", materialBody(graduated("c1"), MaterialClaim{ClaimID: "c2", Text: "曾在某公司实习三个月", FactKey: "education.graduation_year"}))
	valid.ExpectedRevision = confirmedLater.Revision
	receipt, err := o.EditMaterial(ctx, valid)
	require.NoError(t, err)
	require.Equal(t, "education.graduation_year", receipt.Body.Sections[0].Claims[0].FactKey)
	require.Equal(t, "education.graduation_year", receipt.Body.Sections[0].Claims[1].FactKey)
}

func TestMaterialDoesNotFabricateMissingExperienceCertificatesNumbers(t *testing.T) {
	o, _, ctx := newMaterialOffice(t, "owner-1", 1803)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "fabricate")

	// An asserted claim with numbers and no confirmed fact behind it must be
	// refused: missing internships, certificates, or metrics are never invented.
	fabricated := editMaterialInput(seed, "fab-1", materialBody(
		MaterialClaim{ClaimID: "c1", Text: "拥有 3 段实习经历"},
		MaterialClaim{ClaimID: "c2", Text: "持有 CET-6 证书，成绩 620 分"},
	))
	_, err := o.EditMaterial(ctx, fabricated)
	require.ErrorIs(t, err, ErrMaterialClaimUnconfirmed)

	// Explicit placeholders marked needs_review are the only honest way for a
	// missing item to appear in the structured body.
	honest := editMaterialInput(seed, "fab-2", materialBody(
		MaterialClaim{ClaimID: "c1", Text: "实习经历：缺失（待补充）", NeedsReview: true},
		MaterialClaim{ClaimID: "c2", Text: "语言证书：缺失（待补充）", NeedsReview: true},
		MaterialClaim{ClaimID: "c3", Text: "绩点：待补充", NeedsReview: true},
		graduated("c4"),
	))
	receipt, err := o.EditMaterial(ctx, honest)
	require.NoError(t, err)
	require.Len(t, receipt.ReviewRisks, 3)
	codes := map[string]string{}
	for _, risk := range receipt.ReviewRisks {
		codes[risk.ClaimID] = risk.Code
	}
	require.Equal(t, MaterialRiskMissingPlaceholder, codes["c1"])
	require.Equal(t, MaterialRiskMissingPlaceholder, codes["c2"])
	require.Equal(t, MaterialRiskMissingPlaceholder, codes["c3"])

	stored, err := o.Material(ctx, receipt.MaterialID)
	require.NoError(t, err)
	require.Len(t, stored.ReviewRisks, 3)
	require.True(t, stored.Body.Sections[0].Claims[0].NeedsReview)
	require.Empty(t, stored.Body.Sections[0].Claims[0].FactKey)
}

func TestConfirmBodyCreatesImmutableVersionAndOldVersionsComparable(t *testing.T) {
	o, _, ctx := newMaterialOffice(t, "owner-1", 1804)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "versions")

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "ver-1", materialBody(graduated("c1"))))
	require.NoError(t, err)

	confirmed, err := o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "ver-confirm-1", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, MaterialKindConfirmed, confirmed.Kind)
	require.Equal(t, uint64(1), confirmed.Version)
	require.Equal(t, MaterialStatusConfirmed, confirmed.Status)
	require.Equal(t, graduated("c1"), confirmed.Body.Sections[0].Claims[0])

	// A new fact is confirmed and the draft evolves; the user then confirms a
	// second immutable version.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.go", "Go", "ver-skill", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = o.Open(ctx)
	require.NoError(t, err)
	evolved := editMaterialInput(seed, "ver-2", materialBody(graduated("c1"), MaterialClaim{ClaimID: "c2", Text: "掌握 Go", FactKey: "skill.go"}))
	evolved.MaterialID = created.MaterialID
	evolved.ExpectedRevision = view.Revision
	edited, err := o.EditMaterial(ctx, evolved)
	require.NoError(t, err)
	require.Equal(t, MaterialStatusDraft, edited.Status)

	confirmed2, err := o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "ver-confirm-2", MaterialID: created.MaterialID, ExpectedRevision: view.Revision})
	require.NoError(t, err)
	require.Equal(t, uint64(2), confirmed2.Version)

	versions, err := o.MaterialVersions(ctx, created.MaterialID)
	require.NoError(t, err)
	require.Len(t, versions, 2)
	require.Equal(t, uint64(1), versions[0].Version)
	require.Equal(t, uint64(2), versions[1].Version)

	first, err := o.MaterialVersion(ctx, created.MaterialID, 1)
	require.NoError(t, err)
	require.Equal(t, uint64(1), first.Version)
	require.Len(t, first.Body.Sections[0].Claims, 1, "version 1 keeps the original body")
	require.Equal(t, graduated("c1"), first.Body.Sections[0].Claims[0])

	// Old versions stay comparable against the newest one.
	comparison, err := o.CompareMaterialVersions(ctx, created.MaterialID, 1, 2)
	require.NoError(t, err)
	require.Equal(t, created.MaterialID, comparison.MaterialID)
	require.Equal(t, uint64(1), comparison.Baseline.Version)
	require.Equal(t, uint64(2), comparison.Target.Version)
	kinds := map[string]bool{}
	for _, change := range comparison.Changes {
		kinds[change.Kind] = true
	}
	require.True(t, kinds[MaterialChangeClaimAdded])
	require.False(t, kinds[MaterialChangeClaimRemoved])
}

func TestMaterialEditNeverOverwritesExistingVersions(t *testing.T) {
	o, db, ctx := newMaterialOffice(t, "owner-1", 1805)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "immutable")

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "imm-1", materialBody(graduated("c1"))))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "imm-confirm-1", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)

	var versionOneBody string
	require.NoError(t, db.Table("career_material_versions").Select("version_body").Where("material_id=? AND version=1", created.MaterialID).Scan(&versionOneBody).Error)
	require.NotEmpty(t, versionOneBody)

	// A later draft edit and a second confirm must append, never overwrite.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.go", "Go", "imm-skill", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	view, err = o.Open(ctx)
	require.NoError(t, err)
	next := editMaterialInput(seed, "imm-2", materialBody(MaterialClaim{ClaimID: "c2", Text: "掌握 Go", FactKey: "skill.go"}))
	next.MaterialID = created.MaterialID
	next.ExpectedRevision = view.Revision
	_, err = o.EditMaterial(ctx, next)
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "imm-confirm-2", MaterialID: created.MaterialID, ExpectedRevision: view.Revision})
	require.NoError(t, err)

	var versionRows []struct {
		Version     uint64
		VersionBody string
	}
	require.NoError(t, db.Table("career_material_versions").Where("material_id=?", created.MaterialID).Order("version ASC").Find(&versionRows).Error)
	require.Len(t, versionRows, 2)
	require.Equal(t, versionOneBody, versionRows[0].VersionBody, "version 1 must stay byte-identical")
	require.Equal(t, uint64(1), versionRows[0].Version)
	require.Equal(t, uint64(2), versionRows[1].Version)
	require.NotEqual(t, versionOneBody, versionRows[1].VersionBody)
}

func TestMaterialFailurePreservesDraftAndReason(t *testing.T) {
	o, _, ctx := newMaterialOffice(t, "owner-1", 1806)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "failure")

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "fail-1", materialBody(graduated("c1"))))
	require.NoError(t, err)

	// A review failure (claim on a fact that is only pending, never confirmed)
	// refuses the write, keeps the previous draft, and persists the reason.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Propose(ctx, "experience.internship", "某公司实习", "fail-pending", view.Revision, Source{Kind: "resume_extraction", ReferenceID: "resume-9"})
	require.NoError(t, err)
	broken := editMaterialInput(seed, "fail-2", materialBody(MaterialClaim{ClaimID: "c1", Text: "曾在某公司实习", FactKey: "experience.internship"}))
	broken.MaterialID = created.MaterialID
	broken.ExpectedRevision = view.Revision
	_, err = o.EditMaterial(ctx, broken)
	require.ErrorIs(t, err, ErrMaterialClaimUnconfirmed)

	preserved, err := o.Material(ctx, created.MaterialID)
	require.NoError(t, err)
	require.Equal(t, MaterialStatusFailed, preserved.Status)
	require.Equal(t, MaterialFailureClaimUnconfirmed, preserved.FailureCode)
	require.Contains(t, preserved.FailureMessage, "experience.internship")
	require.Equal(t, graduated("c1"), preserved.Body.Sections[0].Claims[0], "the previous draft must survive the failure")

	// The failed request produced no receipt; recovery is a fresh request ID.
	_, err = o.FindMaterialReceipt(ctx, "fail-2")
	require.ErrorIs(t, err, ErrReceiptNotFound)

	view, err = o.Open(ctx)
	require.NoError(t, err)
	recovered := editMaterialInput(seed, "fail-3", materialBody(graduated("c1"), MaterialClaim{ClaimID: "c2", Text: "曾在某公司实习", FactKey: "education.graduation_year"}))
	recovered.MaterialID = created.MaterialID
	recovered.ExpectedRevision = view.Revision
	fixed, err := o.EditMaterial(ctx, recovered)
	require.NoError(t, err)
	require.Equal(t, MaterialStatusDraft, fixed.Status)
	after, err := o.Material(ctx, created.MaterialID)
	require.NoError(t, err)
	require.Empty(t, after.FailureCode)
	require.Empty(t, after.FailureMessage)
}

func TestMaterialExactReplayAndChangedIntentConflict(t *testing.T) {
	o, _, ctx := newMaterialOffice(t, "owner-1", 1807)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "replay")

	input := editMaterialInput(seed, "rep-1", materialBody(graduated("c1")))
	first, err := o.EditMaterial(ctx, input)
	require.NoError(t, err)

	// The profile moves on; an exact replay still returns the stored receipt.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.go", "Go", "rep-bump", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	second, err := o.EditMaterial(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first, second)

	// The same request ID with different content is refused.
	changed := editMaterialInput(seed, "rep-1", materialBody(graduated("c1"), MaterialClaim{ClaimID: "c2", Text: "掌握 Go", FactKey: "skill.go"}))
	_, err = o.EditMaterial(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	// Confirm replays exactly as well; the confirm pins the current head.
	view, err = o.Open(ctx)
	require.NoError(t, err)
	confirmInput := ConfirmMaterialInput{RequestID: "rep-confirm-1", MaterialID: first.MaterialID, ExpectedRevision: view.Revision}
	confirmed, err := o.ConfirmMaterial(ctx, confirmInput)
	require.NoError(t, err)
	replayed, err := o.ConfirmMaterial(ctx, confirmInput)
	require.NoError(t, err)
	require.Equal(t, confirmed, replayed)
	mutated := confirmInput
	mutated.ExpectedRevision = confirmInput.ExpectedRevision + 1
	_, err = o.ConfirmMaterial(ctx, mutated)
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}

func TestMaterialRevisionConflictReturnsCurrentRevision(t *testing.T) {
	o, _, ctx := newMaterialOffice(t, "owner-1", 1808)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "revision")

	stale := editMaterialInput(seed, "rev-1", materialBody(graduated("c1")))
	stale.ExpectedRevision = seed.Revision - 1
	_, err := o.EditMaterial(ctx, stale)
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, seed.Revision, conflict.CurrentRevision)

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "rev-2", materialBody(graduated("c1"))))
	require.NoError(t, err)
	view, err := o.Open(ctx)
	require.NoError(t, err)
	_, err = o.Confirm(ctx, "skill.go", "Go", "rev-bump", view.Revision, Source{Kind: "manual"})
	require.NoError(t, err)
	moved, err := o.Open(ctx)
	require.NoError(t, err)
	require.Greater(t, moved.Revision, seed.Revision)

	next := editMaterialInput(seed, "rev-3", materialBody(graduated("c1")))
	next.MaterialID = created.MaterialID
	next.ExpectedRevision = seed.Revision
	_, err = o.EditMaterial(ctx, next)
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, moved.Revision, conflict.CurrentRevision)

	confirmStale := ConfirmMaterialInput{RequestID: "rev-confirm-1", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision}
	_, err = o.ConfirmMaterial(ctx, confirmStale)
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, moved.Revision, conflict.CurrentRevision)
}

func TestMaterialScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, db, ctx := newMaterialOffice(t, "owner-1", 1809)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "scope")
	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "scope-1", materialBody(graduated("c1"))))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "scope-confirm-1", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)

	otherTenantDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "career-material-other.db")), &gorm.Config{})
	require.NoError(t, err)
	otherOffice, err := NewOffice(otherTenantDB)
	require.NoError(t, err)
	otherCtx := WithScope(context.Background(), Scope{UserID: "owner-2", TenantID: 1899})
	require.NoError(t, otherOffice.ClaimSpace(otherCtx))

	_, err = otherOffice.Material(otherCtx, created.MaterialID)
	require.ErrorIs(t, err, ErrMaterialNotFound)
	_, err = otherOffice.MaterialVersion(otherCtx, created.MaterialID, 1)
	require.ErrorIs(t, err, ErrMaterialNotFound)
	_, err = otherOffice.MaterialVersions(otherCtx, created.MaterialID)
	require.ErrorIs(t, err, ErrMaterialNotFound)
	_, err = otherOffice.CompareMaterialVersions(otherCtx, created.MaterialID, 1, 1)
	require.ErrorIs(t, err, ErrMaterialNotFound)
	hijack := editMaterialInput(seed, "scope-hijack", materialBody(graduated("c1")))
	hijack.MaterialID = created.MaterialID
	_, err = otherOffice.EditMaterial(otherCtx, hijack)
	require.ErrorIs(t, err, ErrMaterialNotFound)
	_, err = otherOffice.ConfirmMaterial(otherCtx, ConfirmMaterialInput{RequestID: "scope-hijack-confirm", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.ErrorIs(t, err, ErrMaterialNotFound)

	// A different user in the same tenant has no Career space at all.
	sameTenantCtx := WithScope(context.Background(), Scope{UserID: "owner-1b", TenantID: 1809})
	_, err = o.Material(sameTenantCtx, created.MaterialID)
	require.ErrorIs(t, err, ErrUnauthorized)

	// The owning scope still reads everything.
	view, err := o.Material(ctx, created.MaterialID)
	require.NoError(t, err)
	require.Equal(t, uint64(1), view.VersionCount)
	var materialRows int64
	require.NoError(t, db.Table("career_materials").Count(&materialRows).Error)
	require.EqualValues(t, 1, materialRows)
}

// A finalized preparation owns its request ID in career_materials without
// leaving a career_material_receipts row: the edit_material creation path
// must refuse that occupied request as the typed 409 idempotency conflict
// instead of surfacing the raw unique-index failure as a 500 (ocr1-144).
func TestEditMaterialRefusesRequestIDOwnedByAnotherSeam(t *testing.T) {
	o, db, ctx := newMaterialOffice(t, "owner-1", 1804)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "occupy")

	now := time.Now().UTC()
	require.NoError(t, db.Create(&materialRecord{
		ID: "11111111-2222-4333-8444-555555555555", TenantID: 1804, UserID: "owner-1",
		RequestID: "shared-1", Fingerprint: "preparation-fingerprint",
		OpportunityID: seed.OpportunityID, SnapshotID: seed.SnapshotID, ProfileRevision: seed.Revision,
		EvidenceBody: "{}", DraftBody: "{}", ReceiptBody: "{}",
		Status: MaterialStatusDraft, CreatedAt: now, UpdatedAt: now,
	}).Error)

	_, err := o.EditMaterial(ctx, editMaterialInput(seed, "shared-1", materialBody(graduated("c1"))))
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}
