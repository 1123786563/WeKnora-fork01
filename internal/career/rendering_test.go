package career

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// mapExportStorage is the hermetic storage seam used by office tests: keys
// follow the local:// object-key pattern without touching the real backend.
type mapExportStorage struct {
	files map[string][]byte
}

type blockingExportStorage struct {
	*mapExportStorage
	started chan struct{}
	release chan struct{}
}

func (b *blockingExportStorage) SaveExport(ctx context.Context, tenantID uint64, name string, data []byte) (string, error) {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.release
	return b.mapExportStorage.SaveExport(ctx, tenantID, name, data)
}

func newMapExportStorage() *mapExportStorage {
	return &mapExportStorage{files: map[string][]byte{}}
}

func (m *mapExportStorage) SaveExport(_ context.Context, tenantID uint64, name string, data []byte) (string, error) {
	key := "local://" + itoa(uint64(tenantID)) + "/" + name
	m.files[key] = append([]byte(nil), data...)
	return key, nil
}

func (m *mapExportStorage) ReadExport(_ context.Context, objectKey string) ([]byte, error) {
	data, ok := m.files[objectKey]
	if !ok {
		return nil, ErrExportNotFound
	}
	return append([]byte(nil), data...), nil
}

func (m *mapExportStorage) DeleteExport(_ context.Context, objectKey string) error {
	delete(m.files, objectKey)
	return nil
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var digits []byte
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	return string(digits)
}

func newExportOffice(t *testing.T, user string, tenant uint64) (*Office, *mapExportStorage, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "career-export.db")), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	store := newMapExportStorage()
	office.SetExportStorage(store)
	office.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, office.ClaimSpace(ctx))
	return office, store, ctx
}

// publishConfirmedVersion creates a material, confirms one immutable version,
// and publishes it. The body mixes CJK and ASCII on purpose: both encodings
// must survive rendering and independent verification.
func publishConfirmedVersion(t *testing.T, o *Office, ctx context.Context, seed materialSeed, requestID string) ExportReceipt {
	t.Helper()
	created, err := o.EditMaterial(ctx, editMaterialInput(seed, requestID+"-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: requestID + "-confirm", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	receipt, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: requestID + "-publish", MaterialID: created.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	return receipt
}

func exportFixtureBody() MaterialBody {
	return MaterialBody{Sections: []MaterialSection{
		{
			Heading: "个人总结",
			Content: "结构化正文：具备 Go 服务端开发经验，参与过检索与存储系统。Backend engineer with Go experience.",
			Claims: []MaterialClaim{
				{ClaimID: "c1", Text: "2027 届毕业生", FactKey: "education.graduation_year"},
				{ClaimID: "c2", Text: "实习经历：缺失（待补充）", NeedsReview: true},
			},
		},
	}}
}

func exportFile(receipt ExportReceipt, format string) ExportedFile {
	for _, file := range receipt.Files {
		if file.Format == format {
			return file
		}
	}
	t := &testing.T{}
	t.Fatalf("receipt must carry a %s file record", format)
	return ExportedFile{}
}

func TestPublishMaterialBindsSameDigestAndVersionToPDFAndDOCX(t *testing.T) {
	o, store, ctx := newExportOffice(t, "export-owner", 1901)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "bind")

	receipt := publishConfirmedVersion(t, o, ctx, seed, "bind-1")

	// Both file records bind the same material version and content digest.
	bodyDigest := sha256.Sum256(mustJSON(receiptBodyOf(t, o, ctx, receipt)))
	require.Equal(t, hex.EncodeToString(bodyDigest[:]), receipt.ContentDigest)
	require.Len(t, receipt.Files, 2)
	pdf, docx := exportFile(receipt, ExportFormatPDF), exportFile(receipt, ExportFormatDOCX)
	for _, file := range []ExportedFile{pdf, docx} {
		require.Equal(t, receipt.MaterialID, file.MaterialID)
		require.Equal(t, receipt.Version, file.Version)
		require.Equal(t, receipt.ContentDigest, file.ContentDigest)
		require.True(t, file.Verified)
		require.NotEmpty(t, file.ObjectKey)
		require.NotEmpty(t, file.FileDigest)
		require.Greater(t, file.Size, int64(0))
	}

	// Stored bytes match the recorded file digests.
	for _, file := range []ExportedFile{pdf, docx} {
		data := store.files[file.ObjectKey]
		require.NotNil(t, data)
		sum := sha256.Sum256(data)
		require.Equal(t, hex.EncodeToString(sum[:]), file.FileDigest)
	}
	require.True(t, strings.HasPrefix(string(store.files[pdf.ObjectKey]), "%PDF-"))
	require.True(t, strings.HasPrefix(string(store.files[docx.ObjectKey]), "PK"))

	// Deterministic rendering: the same confirmed version published again
	// produces byte-identical files and the same content digest.
	again, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "bind-2-publish", MaterialID: receipt.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, receipt.ContentDigest, again.ContentDigest)
	require.Equal(t, exportFile(receipt, ExportFormatPDF).FileDigest, exportFile(again, ExportFormatPDF).FileDigest)
	require.Equal(t, exportFile(receipt, ExportFormatDOCX).FileDigest, exportFile(again, ExportFormatDOCX).FileDigest)
}

func TestDeleteCareerWaitsForInFlightExportPublish(t *testing.T) {
	o, _, ctx := newCareerExportOffice(t, "owner-1", 1951)
	baseStore := newMapExportStorage()
	o.SetExportStorage(baseStore)
	fx := seedExportChain(t, o, ctx, "publish-delete-race")
	store := &blockingExportStorage{mapExportStorage: baseStore, started: make(chan struct{}, 1), release: make(chan struct{})}
	o.SetExportStorage(store)
	publishDone := make(chan error, 1)
	go func() {
		_, err := o.PublishMaterial(ctx, PublishMaterialInput{
			RequestID: "publish-delete-race-late", MaterialID: fx.MaterialID,
			Version: fx.Version, ExpectedRevision: fx.Revision,
		})
		publishDone <- err
	}()
	<-store.started

	deleteDone := make(chan error, 1)
	go func() {
		_, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "publish-delete-race-delete", ExpectedRevision: fx.Revision})
		deleteDone <- err
	}()
	select {
	case err := <-deleteDone:
		t.Fatalf("deletion crossed the in-flight exporter: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(store.release)
	require.NoError(t, <-publishDone)
	require.NoError(t, <-deleteDone)
	require.Empty(t, baseStore.files, "all files published before deletion finalization must be removed")
}

func receiptBodyOf(t *testing.T, o *Office, ctx context.Context, receipt ExportReceipt) MaterialBody {
	t.Helper()
	view, err := o.MaterialVersion(ctx, receipt.MaterialID, receipt.Version)
	require.NoError(t, err)
	return view.Body
}

func TestPublishMaterialVerifiesPDFTextAndLayoutIndependently(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1902)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "pdftext")

	receipt := publishConfirmedVersion(t, o, ctx, seed, "pdftext-1")
	pdf := exportFile(receipt, ExportFormatPDF)
	require.True(t, pdf.Verified, "PDF verification must pass for the honest render")

	// The verifier is a real parser: a code absent from the ToUnicode CMap
	// must fail text decoding.
	view, err := o.MaterialVersion(ctx, receipt.MaterialID, receipt.Version)
	require.NoError(t, err)
	raw := readStoredExport(t, o, ctx, receipt, ExportFormatPDF)
	unmapped := strings.Replace(string(raw), " Tj", "", -1)
	require.NotEqual(t, raw, []byte(unmapped), "fixture must contain Tj operators")
	var reTJ = regexp.MustCompile(`(?s)<([0-9A-F]{4,})> Tj`)
	first := reTJ.FindStringSubmatch(string(raw))
	require.NotNil(t, first, "fixture must contain a hex Tj string")
	tampered := strings.Replace(string(raw), first[0], "<FFFF> Tj", 1)
	require.Error(t, verifyMaterialPDF([]byte(tampered), view.Body), "an unmapped glyph code must fail independent text verification")

	// Wrong body content must fail verification.
	wrongBody := exportFixtureBody()
	wrongBody.Sections[0].Heading = "不存在的标题"
	require.Error(t, verifyMaterialPDF(raw, wrongBody))

	// Layout checks: a baseline y outside the MediaBox must fail.
	layoutBroken := regexp.MustCompile(`1 0 0 1 ([0-9.]+) ([0-9.]+) Tm`).ReplaceAllString(string(raw), "1 0 0 1 $1 99999 Tm")
	require.NotEqual(t, string(raw), layoutBroken)
	require.Error(t, verifyMaterialPDF([]byte(layoutBroken), view.Body), "text drawn outside the page box must fail layout verification")

	// Page count consistency: a corrupted /Count must fail.
	countBroken := regexp.MustCompile(`/Count \d+`).ReplaceAllString(string(raw), "/Count 99")
	require.NotEqual(t, string(raw), countBroken)
	require.Error(t, verifyMaterialPDF([]byte(countBroken), view.Body))
}

func TestPublishMaterialVerifiesDOCXCompletenessAndEditability(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1903)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "docxcheck")

	receipt := publishConfirmedVersion(t, o, ctx, seed, "docxcheck-1")
	docx := exportFile(receipt, ExportFormatDOCX)
	require.True(t, docx.Verified)

	view, err := o.MaterialVersion(ctx, receipt.MaterialID, receipt.Version)
	require.NoError(t, err)
	raw := readStoredExport(t, o, ctx, receipt, ExportFormatDOCX)

	// Wrong body content must fail.
	wrongBody := exportFixtureBody()
	wrongBody.Sections[0].Claims[0].Text = "没有这条主张"
	require.Error(t, verifyMaterialDOCX(raw, wrongBody))

	// A truncated zip must fail completeness verification.
	require.Error(t, verifyMaterialDOCX(raw[:len(raw)/2], view.Body))

	// Removing the sectPr structural essential must fail editability
	// verification even though the text is still present. The package is
	// rebuilt entry by entry with document.xml edited.
	noSectPr, changed := rebuildDOCXWithoutSectPr(raw)
	require.True(t, changed, "fixture must contain a w:sectPr element")
	require.Error(t, verifyMaterialDOCX(noSectPr, view.Body))
}

// rebuildDOCXWithoutSectPr rewrites the OOXML package with the section
// properties stripped from word/document.xml.
func rebuildDOCXWithoutSectPr(data []byte) ([]byte, bool) {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, false
	}
	out := &bytes.Buffer{}
	writer := zip.NewWriter(out)
	changed := false
	for _, file := range reader.File {
		rc, openErr := file.Open()
		if openErr != nil {
			return nil, false
		}
		content, _ := io.ReadAll(rc)
		_ = rc.Close()
		if file.Name == "word/document.xml" {
			stripped := regexp.MustCompile(`(?s)<w:sectPr.*?</w:sectPr>`).ReplaceAll(content, nil)
			if len(stripped) == len(content) {
				return nil, false
			}
			content = stripped
			changed = true
		}
		header := &zip.FileHeader{Name: file.Name, Method: zip.Deflate}
		header.Modified = file.Modified
		target, createErr := writer.CreateHeader(header)
		if createErr != nil {
			return nil, false
		}
		if _, createErr = target.Write(content); createErr != nil {
			return nil, false
		}
	}
	if err = writer.Close(); err != nil {
		return nil, false
	}
	return out.Bytes(), changed
}

func TestPublishMaterialMarksSubmittableOnlyAfterBothFormatsVerified(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1904)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "submittable")

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "submittable-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "submittable-confirm", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	publish := func(requestID string) ExportReceipt {
		receipt, publishErr := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: requestID, MaterialID: created.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
		require.NoError(t, publishErr)
		return receipt
	}

	// Both formats verified: submittable.
	honest := publish("submittable-publish-1")
	require.Equal(t, ExportStatusSubmittable, honest.Status)
	require.True(t, honest.Submittable)

	// Both formats failing verification: failed, never submittable.
	o.failExportVerify = func(format string) error { return errTestExportVerify }
	bothFailed := publish("submittable-publish-2")
	require.Equal(t, ExportStatusFailed, bothFailed.Status)
	require.False(t, bothFailed.Submittable)

	o.failExportVerify = nil
}

var errTestExportVerify = &exportVerificationError{format: ExportFormatPDF, cause: "injected verification failure"}

type exportVerificationError struct {
	format string
	cause  string
}

func (e *exportVerificationError) Error() string { return e.format + ": " + e.cause }

func TestPublishMaterialSingleFormatFailureStaysStagedWithError(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1905)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "staged")

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "staged-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "staged-confirm", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)

	// The DOCX half fails verification; the export must keep the staged
	// status, retain the error, and never expose the successful half as
	// deliverable.
	o.failExportVerify = func(format string) error {
		if format == ExportFormatDOCX {
			return errTestExportVerify
		}
		return nil
	}
	staged, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "staged-publish-1", MaterialID: created.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, ExportStatusStaged, staged.Status)
	require.False(t, staged.Submittable)
	pdfFile, docxFile := exportFile(staged, ExportFormatPDF), exportFile(staged, ExportFormatDOCX)
	require.True(t, pdfFile.Verified)
	require.NotEmpty(t, pdfFile.ObjectKey, "the verified half stays stored for forensics")
	require.False(t, docxFile.Verified)
	require.NotEmpty(t, docxFile.Error, "the failing half keeps its typed error")

	// No download grant may be issued for a not-submittable export.
	_, err = o.MaterialExportGrant(ctx, staged.MaterialID, staged.ExportID, ExportFormatPDF, time.Minute)
	require.ErrorIs(t, err, ErrExportNotSubmittable)

	// A later publish with both formats healthy is submittable.
	o.failExportVerify = nil
	healthy, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "staged-publish-2", MaterialID: created.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, ExportStatusSubmittable, healthy.Status)
	require.True(t, healthy.Submittable)
}

func TestPublishMaterialDownloadGrantFailsImmediatelyAfterRevoke(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1906)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "grant")

	receipt := publishConfirmedVersion(t, o, ctx, seed, "grant-1")
	require.Equal(t, ExportStatusSubmittable, receipt.Status)

	grant, err := o.MaterialExportGrant(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, 5*time.Minute)
	require.NoError(t, err)
	require.Equal(t, exportFile(receipt, ExportFormatPDF).FileDigest, grant.Digest)
	downloaded, err := o.DownloadMaterialExport(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, itoa(uint64(grant.ExpiresAt)), grant.Signature)
	require.NoError(t, err)
	require.Equal(t, readStoredExport(t, o, ctx, receipt, ExportFormatPDF), downloaded)

	// A tampered signature must fail.
	_, err = o.DownloadMaterialExport(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, itoa(uint64(grant.ExpiresAt)), strings.Repeat("0", len(grant.Signature)))
	require.ErrorIs(t, err, ErrExportGrantInvalid)

	// An expired grant must fail.
	base := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	o.exportNow = func() time.Time { return base }
	shortTTL, err := o.MaterialExportGrant(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatDOCX, time.Second)
	require.NoError(t, err)
	o.exportNow = func() time.Time { return base.Add(2 * time.Second) }
	_, err = o.DownloadMaterialExport(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatDOCX, itoa(uint64(shortTTL.ExpiresAt)), shortTTL.Signature)
	require.ErrorIs(t, err, ErrExportGrantInvalid)
	o.exportNow = nil

	// Revocation invalidates an already-issued grant immediately.
	revoked, err := o.RevokeMaterialExport(ctx, RevokeMaterialExportInput{RequestID: "grant-revoke-1", MaterialID: receipt.MaterialID, ExportID: receipt.ExportID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, ExportStatusRevoked, revoked.Status)
	_, err = o.DownloadMaterialExport(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, itoa(uint64(grant.ExpiresAt)), grant.Signature)
	require.Error(t, err, "an issued grant must fail immediately after revocation")
	_, err = o.MaterialExportGrant(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, time.Minute)
	require.ErrorIs(t, err, ErrExportNotSubmittable)
}

func TestPublishMaterialOldVersionsRemainDownloadable(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1907)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "oldversions")

	first := publishConfirmedVersion(t, o, ctx, seed, "old-1")

	// Edit the draft and confirm a second version; the first version and its
	// export must stay untouched and downloadable.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Equal(t, seed.Revision, view.Revision)
	updated := exportFixtureBody()
	updated.Sections[0].Content = "更新后的结构化正文：新增分布式存储经验。"
	_, err = o.EditMaterial(ctx, EditMaterialInput{RequestID: "old-2-edit", MaterialID: first.MaterialID, Body: updated, ExpectedRevision: view.Revision})
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "old-2-confirm", MaterialID: first.MaterialID, ExpectedRevision: view.Revision})
	require.NoError(t, err)
	second, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "old-2-publish", MaterialID: first.MaterialID, Version: 2, ExpectedRevision: view.Revision})
	require.NoError(t, err)
	require.Equal(t, uint64(2), second.Version)
	require.NotEqual(t, first.ContentDigest, second.ContentDigest)

	for _, receipt := range []ExportReceipt{first, second} {
		for _, format := range []string{ExportFormatPDF, ExportFormatDOCX} {
			grant, grantErr := o.MaterialExportGrant(ctx, receipt.MaterialID, receipt.ExportID, format, time.Minute)
			require.NoError(t, grantErr)
			data, downloadErr := o.DownloadMaterialExport(ctx, receipt.MaterialID, receipt.ExportID, format, itoa(uint64(grant.ExpiresAt)), grant.Signature)
			require.NoError(t, downloadErr)
			sum := sha256.Sum256(data)
			require.Equal(t, exportFile(receipt, format).FileDigest, hex.EncodeToString(sum[:]))
		}
	}

	listed, err := o.MaterialExports(ctx, first.MaterialID)
	require.NoError(t, err)
	require.Len(t, listed, 2)
}

func TestPublishMaterialExactReplayAndChangedIntentConflict(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1908)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "replay")

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "replay-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "replay-confirm-1", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	_, err = o.EditMaterial(ctx, EditMaterialInput{RequestID: "replay-edit-2", MaterialID: created.MaterialID, Body: exportFixtureBody(), ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "replay-confirm-2", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)

	intent := PublishMaterialInput{RequestID: "replay-publish", MaterialID: created.MaterialID, Version: 2, ExpectedRevision: seed.Revision}
	receipt, err := o.PublishMaterial(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, uint64(2), receipt.Version)

	// Exact replay returns the stored receipt.
	replayed, err := o.PublishMaterial(ctx, intent)
	require.NoError(t, err)
	require.Equal(t, receipt.ExportID, replayed.ExportID)
	require.Equal(t, receipt.ContentDigest, replayed.ContentDigest)

	// The same request ID with changed content is an idempotency conflict.
	changed := intent
	changed.Version = 1
	_, err = o.PublishMaterial(ctx, changed)
	require.ErrorIs(t, err, ErrIdempotencyConflict)

	// A stale expected revision for a fresh request is a revision conflict.
	_, err = o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "replay-publish-stale", MaterialID: created.MaterialID, Version: 2, ExpectedRevision: seed.Revision - 1})
	require.ErrorIs(t, err, ErrRevisionConflict)

	listed, err := o.MaterialExports(ctx, created.MaterialID)
	require.NoError(t, err)
	require.Len(t, listed, 1, "replays and conflicts must not create extra exports")
}

func TestPublishMaterialScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, _, ctx := newExportOffice(t, "export-owner", 1909)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "scope")

	receipt := publishConfirmedVersion(t, o, ctx, seed, "scope-1")

	otherTenant := WithScope(context.Background(), Scope{UserID: "other-tenant-owner", TenantID: 2910})
	require.NoError(t, o.ClaimSpace(otherTenant))
	// One tenant is one personal career space: a second owner on the same
	// tenant cannot even claim a scope, so owner isolation holds by
	// construction.
	sameTenantSecondOwner := WithScope(context.Background(), Scope{UserID: "someone-else", TenantID: 1909})
	require.ErrorIs(t, o.ClaimSpace(sameTenantSecondOwner), ErrUnauthorized)

	foreign := map[string]context.Context{"other tenant": otherTenant}
	for name, ctx := range foreign {
		_, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "scope-" + name, MaterialID: receipt.MaterialID, Version: 1, ExpectedRevision: 0})
		require.ErrorIs(t, err, ErrMaterialNotFound, "%s must not publish another scope's material", name)
		_, err = o.MaterialExportGrant(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, time.Minute)
		require.ErrorIs(t, err, ErrExportNotFound, "%s must not be granted another scope's export", name)
		_, err = o.MaterialExports(ctx, receipt.MaterialID)
		require.ErrorIs(t, err, ErrMaterialNotFound, "%s must not list another scope's exports", name)
		_, err = o.RevokeMaterialExport(ctx, RevokeMaterialExportInput{RequestID: "scope-revoke-" + name, MaterialID: receipt.MaterialID, ExportID: receipt.ExportID, ExpectedRevision: 0})
		require.ErrorIs(t, err, ErrExportNotFound, "%s must not revoke another scope's export", name)
	}

	// A grant is bound to the issuing owner: redemption under a foreign scope
	// fails the signature check.
	grant, err := o.MaterialExportGrant(ctx, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, time.Minute)
	require.NoError(t, err)
	_, err = o.DownloadMaterialExport(otherTenant, receipt.MaterialID, receipt.ExportID, ExportFormatPDF, itoa(uint64(grant.ExpiresAt)), grant.Signature)
	require.ErrorIs(t, err, ErrExportGrantInvalid)
}

func readStoredExport(t *testing.T, o *Office, ctx context.Context, receipt ExportReceipt, format string) []byte {
	t.Helper()
	file := exportFile(receipt, format)
	data, err := o.exportStorage.ReadExport(ctx, file.ObjectKey)
	require.NoError(t, err)
	return data
}

// TestPublishMaterialFailureAndReplayLeaveNoOrphanObjects pins the storage
// gap: objects written before the durable transaction must be compensated
// away whenever that transaction fails or resolves to a replayed receipt —
// only committed exports keep their bytes.
func TestPublishMaterialFailureAndReplayLeaveNoOrphanObjects(t *testing.T) {
	o, store, ctx := newExportOffice(t, "export-owner", 1941)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "orphan")

	created, err := o.EditMaterial(ctx, editMaterialInput(seed, "orphan-edit", exportFixtureBody()))
	require.NoError(t, err)
	_, err = o.ConfirmMaterial(ctx, ConfirmMaterialInput{RequestID: "orphan-confirm", MaterialID: created.MaterialID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)

	// A stale expected revision fails inside the transaction AFTER the two
	// objects were written; the compensating delete must remove them again.
	_, err = o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "orphan-stale", MaterialID: created.MaterialID, Version: 1, ExpectedRevision: seed.Revision + 5})
	require.ErrorIs(t, err, ErrRevisionConflict)
	require.Empty(t, store.files, "a failed publish must not leave orphan objects behind")

	// A successful publish persists exactly its own two objects.
	receipt, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "orphan-ok", MaterialID: created.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Len(t, store.files, 2)

	// Exact replay of the same request ID resolves to the stored receipt;
	// the objects written by the replay attempt are compensated away.
	replayed, err := o.PublishMaterial(ctx, PublishMaterialInput{RequestID: "orphan-ok", MaterialID: created.MaterialID, Version: 1, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, receipt.ExportID, replayed.ExportID)
	require.Len(t, store.files, 2, "replay must not duplicate stored objects")
}
