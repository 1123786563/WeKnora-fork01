package career

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newCareerExportOffice opens one file-backed office per test. Deletion tests
// are destructive by design, so every task gets its own isolated temporary
// SQLite database; no shared or persistent database is ever touched.
func newCareerExportOffice(t *testing.T, user string, tenant uint64) (*Office, *gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "career-export-delete.db")), &gorm.Config{})
	require.NoError(t, err)
	office, err := NewOffice(db)
	require.NoError(t, err)
	office.SetExportStorage(newMapExportStorage())
	office.SetExportSigningKey([]byte("0123456789abcdef0123456789abcdef"))
	ctx := WithScope(context.Background(), Scope{UserID: user, TenantID: tenant})
	require.NoError(t, office.ClaimSpace(ctx))
	return office, db, ctx
}

// fakeCareerTaskRemover stands in for the Workbench projection remover. It
// records every call and can be told to fail so partial-failure recovery can
// be driven deterministically.
type fakeCareerTaskRemover struct {
	failErr   error
	calls     int
	scopes    []ensureScope
	removed   []interfaces.CareerApplicationTaskProjection
	available []interfaces.CareerApplicationTaskProjection
}

func (f *fakeCareerTaskRemover) RemoveCareerApplicationTaskProjections(
	_ context.Context, tenantID uint64, ownerID string,
) ([]interfaces.CareerApplicationTaskProjection, error) {
	f.calls++
	f.scopes = append(f.scopes, ensureScope{tenantID: tenantID, ownerID: ownerID})
	if f.failErr != nil {
		return nil, f.failErr
	}
	removed := f.available
	f.available = nil
	return removed, nil
}

// seedExportChain drives the full durable chain under one scope: confirmed
// fact, JD snapshot, evaluation, application (with a Workbench task link),
// material version 1, its submittable export, one progress event, and one
// recorded submission.
func seedExportChain(t *testing.T, o *Office, ctx context.Context, seedID string) submissionFixture {
	t.Helper()
	o.SetApplicationTaskLinker(&fakeCareerApplicationLinker{})
	fx := seedSubmissionFixture(t, o, ctx, seedID)
	input := submissionInput(fx, seedID+"-submission", SubmissionChannelEmail, false, fx.Revision)
	input.Note = "已在官网投递"
	_, err := o.RecordSubmission(ctx, input)
	require.NoError(t, err)
	return fx
}

func countScopeRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where("tenant_id = ? AND user_id = ?", uint64(1951), "owner-1").Count(&count).Error)
	return count
}

func TestExportCareerIncludesProfileSnapshotsEventsAndMaterialVersions(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	fx := seedExportChain(t, o, ctx, "chain")

	view, err := o.Open(ctx)
	require.NoError(t, err)
	receipt, err := o.ExportCareer(ctx, CareerExportInput{RequestID: "export-1", ExpectedRevision: view.Revision})
	require.NoError(t, err)
	require.Equal(t, CareerKindExported, receipt.Kind)
	require.Equal(t, "export-1", receipt.RequestID)
	require.Equal(t, view.Revision, receipt.Revision)
	require.NotEmpty(t, receipt.ExportID)

	// 档案：confirmed facts travel in the archive.
	var graduation *Fact
	for i := range receipt.Archive.Profile.Facts {
		if receipt.Archive.Profile.Facts[i].Key == "education.graduation_year" {
			graduation = &receipt.Archive.Profile.Facts[i]
		}
	}
	require.NotNil(t, graduation, "archive must carry the profile facts")
	require.Equal(t, "2027", graduation.Value)

	// 原岗位快照：the pinned JD snapshot is exported with its raw text.
	require.Len(t, receipt.Archive.Opportunities, 1)
	require.Equal(t, fx.OpportunityID, receipt.Archive.Opportunities[0].OpportunityID)
	require.Len(t, receipt.Archive.Opportunities[0].Snapshots, 1)
	require.Equal(t, fx.SnapshotID, receipt.Archive.Opportunities[0].Snapshots[0].SnapshotID)
	require.Contains(t, receipt.Archive.Opportunities[0].Snapshots[0].RawText, "仅限2027届")

	// 申请事件：the immutable progress events ride with the application.
	require.Len(t, receipt.Archive.Applications, 1)
	application := receipt.Archive.Applications[0]
	require.Equal(t, fx.ApplicationID, application.ApplicationID)
	require.Len(t, application.ProgressEvents, 1)
	require.Equal(t, ProgressEventSubmitted, application.ProgressEvents[0].EventType)
	require.Equal(t, "owner-1", application.ProgressEvents[0].Confirmer)

	// 材料版本：the immutable version body is exported.
	require.Len(t, receipt.Archive.Materials, 1)
	require.Equal(t, fx.MaterialID, receipt.Archive.Materials[0].MaterialID)
	require.Len(t, receipt.Archive.Materials[0].Versions, 1)
	require.Equal(t, fx.Version, receipt.Archive.Materials[0].Versions[0].Version)
	require.Contains(t, receipt.Archive.Materials[0].Versions[0].VersionBody, "2027")

	// 投递记录：the user-confirmed submission is exported.
	require.Len(t, receipt.Archive.Submissions, 1)
	require.Equal(t, SubmissionChannelEmail, receipt.Archive.Submissions[0].Channel)
	require.Equal(t, fx.ExportID, receipt.Archive.Submissions[0].ExportID)

	// 导出文件 digest 可校验：recomputing sha256 over the archived payload
	// must reproduce the receipt digest.
	payload, err := json.Marshal(receipt.Archive)
	require.NoError(t, err)
	sum := sha256.Sum256(payload)
	require.Equal(t, hex.EncodeToString(sum[:]), receipt.Digest)

	var stored int64
	require.NoError(t, db.Table("career_data_exports").Where("tenant_id = ? AND user_id = ?", uint64(1951), "owner-1").Count(&stored).Error)
	require.Equal(t, int64(1), stored)
}

func TestExportCareerIsIdempotentByRequestId(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	seedExportChain(t, o, ctx, "idem")

	view, err := o.Open(ctx)
	require.NoError(t, err)
	first, err := o.ExportCareer(ctx, CareerExportInput{RequestID: "export-1", ExpectedRevision: view.Revision})
	require.NoError(t, err)
	second, err := o.ExportCareer(ctx, CareerExportInput{RequestID: "export-1", ExpectedRevision: view.Revision})
	require.NoError(t, err)
	require.Equal(t, first.ExportID, second.ExportID)
	require.Equal(t, first.Digest, second.Digest)
	require.Equal(t, first.CreatedAt, second.CreatedAt, "replay must return the stored receipt, not a fresh export")

	var stored int64
	require.NoError(t, db.Table("career_data_exports").Where("request_id = ?", "export-1").Count(&stored).Error)
	require.Equal(t, int64(1), stored, "one request ID writes exactly one export row")

	// 同一 request ID 内容变化拒绝：a different expected revision is a
	// different intent and must be rejected, not replayed.
	_, err = o.ExportCareer(ctx, CareerExportInput{RequestID: "export-1", ExpectedRevision: view.Revision + 1})
	require.ErrorIs(t, err, ErrIdempotencyConflict)
}

func TestDeleteCareerExplainsInSpaceVersusExternalBoundary(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	seedExportChain(t, o, ctx, "boundary")

	boundary, err := o.CareerDeletionBoundary(ctx)
	require.NoError(t, err)

	sections := map[string]CareerDeletionSection{}
	for _, section := range boundary.InSpace {
		sections[section.Section] = section
	}
	for _, name := range []string{"profile", "opportunities", "applications", "materials", "material_exports", "submissions", "workbench_tasks"} {
		require.Containsf(t, sections, name, "boundary must explain in-space section %s", name)
		require.NotEmptyf(t, sections[name].Description, "section %s must carry a description", name)
	}
	require.Equal(t, 1, sections["opportunities"].Count)
	require.Equal(t, 1, sections["applications"].Count)
	require.Equal(t, 1, sections["material_exports"].Count)
	require.Equal(t, 1, sections["submissions"].Count)
	require.Equal(t, 1, sections["workbench_tasks"].Count)

	// 外部平台资料：the system can only delete in-space data; external
	// submissions and already-sent material copies are not revocable here.
	require.NotEmpty(t, boundary.External)
	external := map[string]CareerExternalBoundaryItem{}
	for _, item := range boundary.External {
		external[item.Item] = item
	}
	require.Contains(t, external, "external_platform_submissions")
	require.Contains(t, external, "external_email_copies")
	for _, item := range boundary.External {
		require.False(t, item.Revocable, "external platform data is never revocable by this system")
		require.NotEmpty(t, item.Description)
	}

	// 延迟/例外保留：retention rows are disclosed up front, not hidden.
	require.NotEmpty(t, boundary.Retention)
	for _, item := range boundary.Retention {
		require.NotEmpty(t, item.Holder)
		require.NotEmpty(t, item.Reason)
	}

	// The boundary request itself must not mutate anything.
	view, err := o.Open(ctx)
	require.NoError(t, err)
	require.Len(t, view.Facts, 1)
	require.Equal(t, int64(1), countScopeRows(t, db, "career_applications"))
}

func TestDeleteCareerRemovesCareerDataAndWorkbenchProjection(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	remover := &fakeCareerTaskRemover{available: []interfaces.CareerApplicationTaskProjection{{
		TaskID: "task-1", RunID: "run-1", ApplicationID: "app-any",
	}}}
	o.SetApplicationTaskRemover(remover)
	fx := seedExportChain(t, o, ctx, "remove")

	// T19 的准备记录也属于 Career 域数据，必须随完整删除一起清除（集成修复 F1）。
	require.NoError(t, db.Exec(`INSERT INTO career_preparations
		(id, tenant_id, user_id, application_id, request_id, fingerprint, focus, status,
		 submission_id, submitted_material_id, submitted_export_id, submitted_version, submitted_digest,
		 snapshot_id, snapshot_sha256, profile_revision, material_id, failure_code, failure_message,
		 receipt_body, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"prep-1", uint64(1951), "owner-1", "app-any", "prep-req-1", "fp", "cover_letter", "succeeded",
		"", "", "", 0, "", "", "", 0, "", "", "", "{}", "2026-09-26 00:00:00", "2026-09-26 00:00:00").Error)

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-1", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
	require.NotNil(t, receipt.CompletedAt)

	// Career 域数据全部删除。
	for _, table := range []string{
		"career_facts", "career_fact_versions", "career_proposals", "career_source_revisions",
		"career_opportunities", "career_opportunity_observations", "career_opportunity_snapshots",
		"career_opportunity_receipts", "career_evaluations", "career_applications",
		"career_searches", "career_search_results", "career_materials", "career_material_versions",
		"career_material_receipts", "career_material_exports", "career_progress_events",
		"career_search_rules", "career_search_rule_receipts", "career_search_rule_runs",
		"career_search_discovery_todos", "career_submissions", "career_preparations",
		"career_receipts", "career_data_exports",
	} {
		require.Zerof(t, countScopeRows(t, db, table), "%s must be empty after deletion", table)
	}

	// Workbench 投影经删除端口移除，且只带认证作用域。
	require.Equal(t, 1, remover.calls)
	require.Equal(t, uint64(1951), remover.scopes[0].tenantID)
	require.Equal(t, "owner-1", remover.scopes[0].ownerID)

	// 审计行保留且可重放。
	var audit int64
	require.NoError(t, db.Table("career_data_deletions").Where("tenant_id = ? AND user_id = ? AND request_id = ?", uint64(1951), "owner-1", "delete-1").Count(&audit).Error)
	require.Equal(t, int64(1), audit)
	replay, err := o.FindCareerDeletion(ctx, "delete-1")
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, replay.Status)
	require.Equal(t, receipt.RequestID, replay.RequestID)
}

func TestDeleteCareerRevokesOldExportAndArtifactGrants(t *testing.T) {
	o, _, ctx := newCareerExportOffice(t, "owner-1", 1951)
	fx := seedExportChain(t, o, ctx, "revoke")
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})

	grant, err := o.MaterialExportGrant(ctx, fx.MaterialID, fx.ExportID, ExportFormatPDF, time.Minute)
	require.NoError(t, err)
	_, err = o.DownloadMaterialExport(ctx, fx.MaterialID, fx.ExportID, ExportFormatPDF,
		fmt.Sprintf("%d", grant.ExpiresAt), grant.Signature)
	require.NoError(t, err, "grant must work before deletion")

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-1", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)

	// 旧导出授权不可再兑换。
	_, err = o.DownloadMaterialExport(ctx, fx.MaterialID, fx.ExportID, ExportFormatPDF,
		fmt.Sprintf("%d", grant.ExpiresAt), grant.Signature)
	require.ErrorIs(t, err, ErrExportGrantInvalid)

	// 旧 Artifact（材料版本本体）不可再访问。
	_, err = o.MaterialVersion(ctx, fx.MaterialID, fx.Version)
	require.ErrorIs(t, err, ErrMaterialNotFound)
	_, err = o.Material(ctx, fx.MaterialID)
	require.ErrorIs(t, err, ErrMaterialNotFound)
}

func TestDeleteCareerRemovesPreviouslyRevokedExportObjects(t *testing.T) {
	o, store, ctx := newExportOffice(t, "revoked-delete-owner", 1953)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "revoked-delete")
	published := publishConfirmedVersion(t, o, ctx, seed, "revoked-delete-publish")
	_, err := o.RevokeMaterialExport(ctx, RevokeMaterialExportInput{RequestID: "revoked-delete-revoke", MaterialID: published.MaterialID, ExportID: published.ExportID, ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.NotEmpty(t, store.files, "revoke may retain physical bytes for deletion recovery")
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "revoked-delete-final", ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
	require.Empty(t, store.files, "deleted receipt requires physical cleanup, including revoked exports")
}

func TestDeleteCareerFailsClosedWhenExportStorageUnavailable(t *testing.T) {
	o, store, ctx := newExportOffice(t, "missing-storage-delete-owner", 1954)
	seed := seedMaterialEvidence(t, o, ctx, "2027", "missing-storage-delete")
	published := publishConfirmedVersion(t, o, ctx, seed, "missing-storage-publish")
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	o.SetExportStorage(nil)
	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "missing-storage-delete-final", ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusPartial, receipt.Status, "missing storage must not be reported as deleted")
	require.NotEmpty(t, store.files)
	// Retry the same durable delete intent when storage returns; all original
	// object locators are still present for deterministic compensation.
	o.SetExportStorage(store)
	retry, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "missing-storage-delete-final", ExpectedRevision: seed.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, retry.Status)
	require.Empty(t, store.files)
	_ = published
}

func TestCareerDeletionBoundaryReturnsCountQueryErrors(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "boundary-count-owner", 1955)
	require.NoError(t, db.Migrator().DropTable("career_material_exports"))
	view, err := o.CareerDeletionBoundary(ctx)
	require.Error(t, err)
	require.Empty(t, view.InSpace)
}

func TestDeleteCareerDisclosesRetentionScopeAndStatus(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	fx := seedExportChain(t, o, ctx, "retain")

	boundary, err := o.CareerDeletionBoundary(ctx)
	require.NoError(t, err)
	retention := map[string]CareerRetentionItem{}
	for _, item := range boundary.Retention {
		retention[item.Holder] = item
	}
	require.Contains(t, retention, "career_data_deletions")
	require.Contains(t, retention, "career_changes")

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-1", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
	require.NotEmpty(t, receipt.Retention)
	for _, item := range receipt.Retention {
		require.Equal(t, "retained", item.Status)
		require.NotEmpty(t, item.Reason)
	}

	// 保留行确实保留：审计行 + deletion change 事件 + revision 计数器。
	var audit int64
	require.NoError(t, db.Table("career_data_deletions").Where("tenant_id = ? AND user_id = ?", uint64(1951), "owner-1").Count(&audit).Error)
	require.Equal(t, int64(1), audit)
	var changes int64
	require.NoError(t, db.Table("career_changes").Where("tenant_id = ? AND user_id = ?", uint64(1951), "owner-1").Count(&changes).Error)
	require.Equal(t, int64(1), changes, "only the deletion event survives in the changes stream")
	var profiles int64
	require.NoError(t, db.Table("career_profiles").Where("tenant_id = ? AND user_id = ?", uint64(1951), "owner-1").Count(&profiles).Error)
	require.Equal(t, int64(1), profiles)
}

func TestDeleteCareerPartialFailureKeepsRecoverableStateAndAudit(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	remover := &fakeCareerTaskRemover{failErr: errors.New("workbench unavailable")}
	o.SetApplicationTaskRemover(remover)
	fx := seedExportChain(t, o, ctx, "partial")

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-1", ExpectedRevision: fx.Revision})
	require.NoError(t, err, "partial failure is a receipt, not a transport error")
	require.Equal(t, DeletionStatusPartial, receipt.Status)
	require.Nil(t, receipt.CompletedAt, "partial deletion must never claim completion")
	require.Equal(t, 1, remover.calls)

	steps := map[string]CareerDeletionStep{}
	for _, step := range receipt.Steps {
		steps[step.Name] = step
	}
	require.Equal(t, DeletionStepStatusDone, steps["purge_career_data"].Status)
	require.Equal(t, DeletionStepStatusFailed, steps["remove_workbench_tasks"].Status)
	require.NotEmpty(t, steps["remove_workbench_tasks"].Detail)

	// 审计记录已存在，整体状态是 partial。
	var status string
	require.NoError(t, db.Table("career_data_deletions").
		Where("tenant_id = ? AND user_id = ? AND request_id = ?", uint64(1951), "owner-1", "delete-1").
		Select("status").Scan(&status).Error)
	require.Equal(t, DeletionStatusPartial, status)
	require.Zerof(t, countScopeRows(t, db, "career_applications"), "completed steps stay deleted")

	// 可恢复：重试同一 request ID 续删。
	remover.failErr = nil
	resumed, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-1", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, resumed.Status)
	require.Equal(t, 2, remover.calls)
	require.NotNil(t, resumed.CompletedAt)
	require.NoError(t, db.Table("career_data_deletions").
		Where("tenant_id = ? AND user_id = ? AND request_id = ?", uint64(1951), "owner-1", "delete-1").
		Select("status").Scan(&status).Error)
	require.Equal(t, DeletionStatusDeleted, status)
}

// TestDeleteCareerConcurrentRetryBumpsRevisionExactlyOnce pins ocr3-128:
// two concurrent DeleteCareer retries under the same request ID must not
// double-run finalize — the revision bumps exactly once and exactly one
// career_deleted event lands, because the finalize phase locks the deletion
// audit row and replays any already-finalized receipt.
func TestDeleteCareerConcurrentRetryBumpsRevisionExactlyOnce(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1980)
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})
	fx := seedExportChain(t, o, ctx, "double-run")

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-race", ExpectedRevision: fx.Revision})
		}(i)
	}
	wg.Wait()
	require.True(t, results[0] == nil || results[1] == nil,
		"at least one runner must produce a receipt: %v / %v", results[0], results[1])

	var revision uint64
	require.NoError(t, db.Table("career_profiles").
		Where("tenant_id = ? AND user_id = ?", uint64(1980), "owner-1").
		Select("revision").Scan(&revision).Error)
	require.Equal(t, fx.Revision+1, revision,
		"concurrent retries of the same request ID must bump the revision exactly once")

	var events int64
	require.NoError(t, db.Table("career_changes").
		Where("tenant_id = ? AND user_id = ? AND kind = ?", uint64(1980), "owner-1", ChangeKindCareerDeleted).
		Count(&events).Error)
	require.EqualValues(t, 1, events, "exactly one career_deleted event survives")

	var status string
	require.NoError(t, db.Table("career_data_deletions").
		Where("tenant_id = ? AND user_id = ? AND request_id = ?", uint64(1980), "owner-1", "delete-race").
		Select("status").Scan(&status).Error)
	require.Equal(t, DeletionStatusDeleted, status)
}

// TestDeleteCareerSweepRemovesRowsCommittedDuringPausedDeletion pins the
// honesty half of ocr3-128: a career row committed after the purge step ran
// (the deletion sits partial) must be swept inside the locked finalize
// transaction — the resumed receipt may only claim "deleted" over an empty
// space.
func TestDeleteCareerSweepRemovesRowsCommittedDuringPausedDeletion(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	remover := &fakeCareerTaskRemover{failErr: errors.New("workbench unavailable")}
	o.SetApplicationTaskRemover(remover)
	fx := seedExportChain(t, o, ctx, "sweep")

	partial, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-sweep", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusPartial, partial.Status)

	// While the deletion sits partial, a racing write commits a fresh career
	// row after the purge already ran. The public writers are fenced behind
	// the lifecycle gate now (H7), so the torn writer is simulated by a direct
	// row insert — the transaction of a writer whose admission passed before
	// the phase flipped.
	require.NoError(t, db.Create(&opportunity{
		ID: "sneaky-job-opp", TenantID: 1951, UserID: "owner-1", CreatedAt: time.Now().UTC(),
	}).Error)
	require.EqualValues(t, 1, countScopeRows(t, db, "career_opportunities"),
		"the sneaked row is committed after the purge step ran")

	// Resuming the same request ID finalizes under the profile lock with the
	// in-transaction sweep: the sneaked row must not survive into a receipt
	// that claims "deleted".
	remover.failErr = nil
	resumed, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-sweep", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, resumed.Status)
	require.Zerof(t, countScopeRows(t, db, "career_opportunities"),
		"rows committed after the purge must be swept before the deleted receipt")
}

func TestDeleteCareerInvalidatesClientVisibleScopeOrChanges(t *testing.T) {
	o, _, ctx := newCareerExportOffice(t, "owner-1", 1951)
	fx := seedExportChain(t, o, ctx, "scope")
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})

	grant, err := o.MaterialExportGrant(ctx, fx.MaterialID, fx.ExportID, ExportFormatPDF, time.Minute)
	require.NoError(t, err)

	before, err := o.Open(ctx)
	require.NoError(t, err)
	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-1", ExpectedRevision: before.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)

	// changes 流呈现 deletion 事件：old since values surface the deletion.
	changes, err := o.Changes(ctx, before.Revision)
	require.NoError(t, err)
	require.Equal(t, before.Revision+1, changes.Revision)
	found := false
	for _, change := range changes.Changes {
		if change.Kind == ChangeKindCareerDeleted {
			found = true
		}
	}
	require.True(t, found, "changes stream must present the deletion event")

	// 客户端缓存不可再访问：space 数据为空、revision 前进、旧授权失效。
	after, err := o.Open(ctx)
	require.NoError(t, err)
	require.Equal(t, before.Revision+1, after.Revision)
	require.Empty(t, after.Facts)
	_, err = o.DownloadMaterialExport(ctx, fx.MaterialID, fx.ExportID, ExportFormatPDF,
		fmt.Sprintf("%d", grant.ExpiresAt), grant.Signature)
	require.ErrorIs(t, err, ErrExportGrantInvalid)
}

func TestExportDeletionScopeRejectsOtherTenantAndOwner(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	fx := seedExportChain(t, o, ctx, "scope")
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})

	view, err := o.Open(ctx)
	require.NoError(t, err)

	otherTenant := WithScope(context.Background(), Scope{UserID: "owner-1", TenantID: 1952})
	_, err = o.ExportCareer(otherTenant, CareerExportInput{RequestID: "steal-1", ExpectedRevision: view.Revision})
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.DeleteCareer(otherTenant, CareerDeletionInput{RequestID: "steal-1", ExpectedRevision: view.Revision})
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.CareerDeletionBoundary(otherTenant)
	require.ErrorIs(t, err, ErrUnauthorized)

	otherOwner := WithScope(context.Background(), Scope{UserID: "owner-2", TenantID: 1951})
	_, err = o.ExportCareer(otherOwner, CareerExportInput{RequestID: "steal-2", ExpectedRevision: view.Revision})
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.DeleteCareer(otherOwner, CareerDeletionInput{RequestID: "steal-2", ExpectedRevision: view.Revision})
	require.ErrorIs(t, err, ErrUnauthorized)
	_, err = o.CareerDeletionBoundary(otherOwner)
	require.ErrorIs(t, err, ErrUnauthorized)

	// 拒绝后原 scope 数据完好。
	require.Equal(t, int64(1), countScopeRows(t, db, "career_facts"))
	require.Equal(t, int64(1), countScopeRows(t, db, "career_applications"))
	require.Equal(t, fx.Revision, view.Revision)
}

// ---- OCR round 1 fixes ---------------------------------------------------

// fakeSourceUploadReleaser stands in for the UploadAdapter release seam so
// the purge step can be observed without a file backend.
type fakeSourceUploadReleaser struct {
	calls []string
	fail  error
}

func (f *fakeSourceUploadReleaser) Release(_ context.Context, reference, sourceID string) error {
	f.calls = append(f.calls, reference+"|"+sourceID)
	return f.fail
}

// TestDeleteCareerRemovesExportObjectsAndReleasesSourceUploads pins the
// physical side of delete_career: rendered export objects are deleted from
// storage (not just revoked in the DB) and uploaded source originals are
// released through the catalog seam before their locator rows are purged.
func TestDeleteCareerRemovesExportObjectsAndReleasesSourceUploads(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	store := newMapExportStorage()
	o.SetExportStorage(store)
	fx := seedExportChain(t, o, ctx, "purge-files")
	require.Len(t, store.files, 2, "the seeded submittable export holds a PDF and a DOCX object")

	releaser := &fakeSourceUploadReleaser{}
	o.SetSourceUploadReleaser(releaser)
	require.NoError(t, db.Exec(`INSERT INTO career_source_revisions
		(id, tenant_id, user_id, revision, file_name, mime_type, size, digest, request_id,
		 resource_ref, status, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		"src-1", uint64(1951), "owner-1", 1, "resume.pdf", "application/pdf", 10, "digest", "src-req-1",
		"local://1951/career_source_src-1.pdf", "ready", "2026-09-26 00:00:00").Error)
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "delete-1", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)

	require.Empty(t, store.files, "physical export objects must not outlive the deleted space")
	require.Len(t, releaser.calls, 1, "every uploaded source with a resource ref is released exactly once")
	require.Equal(t, "local://1951/career_source_src-1.pdf|src-1", releaser.calls[0])
	require.Zerof(t, countScopeRows(t, db, "career_source_revisions"), "source rows are purged after release")
}

// TestFindCareerDeletionDuringExecutionWindowReportsInProgress pins the
// executing window: a deleting record still carrying the initial "{}" body
// must replay as a truthful in-progress receipt, never as an empty success.
func TestFindCareerDeletionDuringExecutionWindowReportsInProgress(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	fx := seedExportChain(t, o, ctx, "inprogress")
	require.NoError(t, db.Create(&careerDataDeletionRecord{
		ID: "del-inprogress", TenantID: 1951, UserID: "owner-1",
		RequestID: "delete-live", Fingerprint: "fp", ExpectedRevision: fx.Revision,
		Status: DeletionStatusDeleting, StateBody: string(mustJSON(newDeletionExecution())), ReceiptBody: "{}",
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	}).Error)

	receipt, err := o.FindCareerDeletion(ctx, "delete-live")
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleting, receipt.Status)
	require.Equal(t, CareerKindDeleted, receipt.Kind)
	require.Equal(t, "delete-live", receipt.RequestID)
	require.Equal(t, fx.Revision, receipt.Revision)
	require.Nil(t, receipt.CompletedAt)
	require.NotEmpty(t, receipt.Retention)
	require.Len(t, receipt.Steps, 4)
	for _, step := range receipt.Steps {
		require.Equal(t, DeletionStepStatusPending, step.Status, "durable state shows nothing completed yet")
	}
}

// TestExportCareerArchiveCarriesPreparationsSearchRulesAndReminders pins the
// completeness contract: every section the purge destroys — preparations,
// periodic search rules, reminders — travels in the one complete export.
func TestExportCareerArchiveCarriesPreparationsSearchRulesAndReminders(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	seedExportChain(t, o, ctx, "full-archive")
	preparationReceipt := PreparationReceipt{
		Kind: "career_preparation", RequestID: "prep-req-9", ApplicationID: "app-any",
		PreparationID: "prep-export-1", Focus: "cover_letter", Status: "succeeded",
		Anchor:   PreparationAnchor{SubmissionID: "submission-1", MaterialID: "material-1", ExportID: "export-1", Version: 3, ContentDigest: "digest-v3"},
		Body:     MaterialBody{Sections: []MaterialSection{{Heading: "Cover letter", Content: "Preserve this generated draft."}}},
		Sources:  PreparationSources{SubmittedVersion: PreparationAnchor{SubmissionID: "submission-1", MaterialID: "material-1", ExportID: "export-1", Version: 3, ContentDigest: "digest-v3"}, Snapshot: PreparationSnapshotRef{OpportunityID: "opp-1", SnapshotID: "snap-1", SnapshotSHA256: "sha256-snapshot"}, FactKeys: []string{"education.graduation_year"}, ProfileRevision: 7},
		Revision: 7, CreatedAt: time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC),
	}
	preparationReceiptJSON, err := json.Marshal(preparationReceipt)
	require.NoError(t, err)

	require.NoError(t, db.Exec(`INSERT INTO career_preparations
		(id, tenant_id, user_id, application_id, request_id, fingerprint, focus, status,
		 submission_id, submitted_material_id, submitted_export_id, submitted_version, submitted_digest,
		 snapshot_id, snapshot_sha256, profile_revision, material_id, failure_code, failure_message,
		 receipt_body, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		"prep-export-1", uint64(1951), "owner-1", "app-any", "prep-req-9", "fp", "cover_letter", "succeeded",
		"", "", "", 0, "", "", "", 0, "", "", "", string(preparationReceiptJSON), "2026-09-26 00:00:00", "2026-09-26 00:00:00").Error)
	require.NoError(t, db.Exec(`INSERT INTO career_search_rules
		(tenant_id, user_id, id, query, interval_minutes, status, revision, last_period, next_due_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		uint64(1951), "owner-1", "rule-export-1", "Go 后端", 720, "active", 3, 1,
		"2026-09-27 00:00:00", "2026-09-26 00:00:00", "2026-09-26 00:00:00").Error)
	require.NoError(t, db.Exec(`INSERT INTO career_reminders
		(id, tenant_id, user_id, source_kind, source_id, application_id, opportunity_id, notice_key, status, request_id, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		"rem-export-1", uint64(1951), "owner-1", "search_rule", "rule-export-1", "", "", "period_done", "open",
		"rem-req-9", "2026-09-26 00:00:00", "2026-09-26 00:00:00").Error)

	view, err := o.Open(ctx)
	require.NoError(t, err)
	receipt, err := o.ExportCareer(ctx, CareerExportInput{RequestID: "export-full", ExpectedRevision: view.Revision})
	require.NoError(t, err)

	require.Len(t, receipt.Archive.Preparations, 1)
	require.Equal(t, "prep-export-1", receipt.Archive.Preparations[0].PreparationID)
	require.Equal(t, "cover_letter", receipt.Archive.Preparations[0].Focus)
	require.Equal(t, "Preserve this generated draft.", receipt.Archive.Preparations[0].Receipt.Body.Sections[0].Content)
	require.Equal(t, []string{"education.graduation_year"}, receipt.Archive.Preparations[0].Receipt.Sources.FactKeys)
	require.Equal(t, "digest-v3", receipt.Archive.Preparations[0].Receipt.Anchor.ContentDigest)
	require.Len(t, receipt.Archive.SearchRules, 1)
	require.Equal(t, "rule-export-1", receipt.Archive.SearchRules[0].RuleID)
	require.Equal(t, uint64(720), receipt.Archive.SearchRules[0].IntervalMinutes)
	require.Equal(t, uint64(3), receipt.Archive.SearchRules[0].Revision)
	require.Len(t, receipt.Archive.Reminders, 1)
	require.Equal(t, "rem-export-1", receipt.Archive.Reminders[0].ReminderID)
	require.Equal(t, "search_rule", receipt.Archive.Reminders[0].SourceKind)

	// The archive digest still verifies over the extended payload.
	payload, err := json.Marshal(receipt.Archive)
	require.NoError(t, err)
	sum := sha256.Sum256(payload)
	require.Equal(t, hex.EncodeToString(sum[:]), receipt.Digest)
}

func TestCareerDeletionBoundaryDoesNotPromiseSearchRunsOrDiscoveryTodosInExport(t *testing.T) {
	o, _, ctx := newCareerExportOffice(t, "owner-1", 1951)
	view, err := o.CareerDeletionBoundary(ctx)
	require.NoError(t, err)
	var searchRules CareerDeletionSection
	for _, section := range view.InSpace {
		if section.Section == "search_rules" {
			searchRules = section
			break
		}
	}
	require.NotEmpty(t, searchRules.Section)
	require.Contains(t, searchRules.Description, "运行与发现待办不随导出携带")
	require.Contains(t, searchRules.Description, "删除后不可恢复")
}

// ---- OCR round 2 fixes ---------------------------------------------------

// TestExportCareerWithoutProfileRowSucceedsAtRevisionZero pins ocr2-020: a
// space that never wrote a profile row (only opened, or only imported
// sources) still exports whole — the epoch reads as revision 0 instead of a
// 500 — and a revision mismatch stays a typed conflict.
func TestExportCareerWithoutProfileRowSucceedsAtRevisionZero(t *testing.T) {
	o, _, ctx := newCareerExportOffice(t, "owner-1", 1961)

	receipt, err := o.ExportCareer(ctx, CareerExportInput{RequestID: "bare-export-1", ExpectedRevision: 0})
	require.NoError(t, err)
	require.Equal(t, CareerKindExported, receipt.Kind)
	require.Equal(t, uint64(0), receipt.Revision)
	require.Equal(t, CareerExportStatusComplete, receipt.Status)
	require.Empty(t, receipt.Archive.Profile.Facts)
	require.Empty(t, receipt.Archive.Opportunities)

	replay, err := o.FindCareerExport(ctx, "bare-export-1")
	require.NoError(t, err)
	require.Equal(t, receipt.ExportID, replay.ExportID)

	_, err = o.ExportCareer(ctx, CareerExportInput{RequestID: "bare-export-2", ExpectedRevision: 3})
	var conflict *RevisionConflictError
	require.ErrorAs(t, err, &conflict)
	require.Equal(t, uint64(0), conflict.CurrentRevision)
}

// TestDeleteCareerWithoutProfileRowCompletes pins ocr2-021: deletion is a
// data-sovereignty right — a user who never wrote a profile fact must still
// delete their whole space (no 500 at the pre-check, no partial stall at
// finalize; the epoch row is created so career_deleted lands on revision 1).
func TestDeleteCareerWithoutProfileRowCompletes(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1962)
	o.SetApplicationTaskRemover(&fakeCareerTaskRemover{})

	receipt, err := o.DeleteCareer(ctx, CareerDeletionInput{RequestID: "bare-delete-1", ExpectedRevision: 0})
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, receipt.Status)
	require.Equal(t, uint64(1), receipt.Revision, "finalize creates the epoch row and bumps to revision 1")

	var head profile
	require.NoError(t, db.Where("tenant_id=? AND user_id=?", uint64(1962), "owner-1").First(&head).Error)
	require.Equal(t, uint64(1), head.Revision)

	var changes []change
	require.NoError(t, db.Where("tenant_id=? AND user_id=?", uint64(1962), "owner-1").Find(&changes).Error)
	require.Len(t, changes, 1)
	require.Equal(t, ChangeKindCareerDeleted, changes[0].Kind)
}

// TestDeleteCareerBoundaryDisclosesPurgeListSections pins ocr2-145: the
// personal-data tables the purge physically sweeps — evaluations, uploaded
// source originals, whole-space export archives, and the idempotency receipt
// ledgers — must each be disclosed before the user confirms deletion.
func TestDeleteCareerBoundaryDisclosesPurgeListSections(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1951)
	fx := seedExportChain(t, o, ctx, "disclose")
	_, err := o.ExportCareer(ctx, CareerExportInput{RequestID: "disclose-export", ExpectedRevision: fx.Revision})
	require.NoError(t, err)
	require.NoError(t, db.Create(&sourceRevision{
		ID: "src-disclose-1", TenantID: 1951, UserID: "owner-1", Revision: 1,
		FileName: "resume.pdf", Status: SourceStatusComplete,
	}).Error)

	boundary, err := o.CareerDeletionBoundary(ctx)
	require.NoError(t, err)
	sections := map[string]CareerDeletionSection{}
	for _, section := range boundary.InSpace {
		sections[section.Section] = section
	}
	for _, name := range []string{"evaluations", "source_revisions", "data_exports", "receipts"} {
		require.Containsf(t, sections, name, "boundary must disclose purge section %s", name)
		require.NotEmptyf(t, sections[name].Description, "section %s must carry a description", name)
	}
	require.Equal(t, 1, sections["evaluations"].Count)
	require.Equal(t, 1, sections["source_revisions"].Count)
	require.Equal(t, 1, sections["data_exports"].Count)
	require.Positive(t, sections["receipts"].Count, "the fixture's confirmed facts wrote idempotency receipts")
}

// TestDeleteCareerCreateRaceRejectsDifferentFingerprint pins ocr2-146: the
// Create race fallback compares fingerprints — the same request ID with a
// different expected revision is a definite conflict and never receives the
// winner's deletion receipt.
func TestDeleteCareerCreateRaceRejectsDifferentFingerprint(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1963)
	scope, err := getScope(ctx)
	require.NoError(t, err)

	winnerFP, err := careerDeletionFingerprint("race-1", 3)
	require.NoError(t, err)
	execution := newDeletionExecution()
	winner := CareerDeletionReceipt{
		Kind: CareerKindDeleted, RequestID: "race-1", Status: DeletionStatusPartial,
		Retention: careerDeletionRetention(), Revision: 3,
	}
	now := time.Now().UTC()
	require.NoError(t, db.Create(&careerDataDeletionRecord{
		ID: uuid.NewString(), TenantID: scope.TenantID, UserID: scope.UserID,
		RequestID: "race-1", Fingerprint: winnerFP, ExpectedRevision: 3,
		Status: DeletionStatusPartial, StateBody: string(mustJSON(execution)),
		ReceiptBody: string(mustJSON(winner)), CreatedAt: now, UpdatedAt: now,
	}).Error)

	// The losing twin raced with a different expected revision under the
	// same request ID: a definite conflict, never the winner's receipt.
	loserFP, err := careerDeletionFingerprint("race-1", 4)
	require.NoError(t, err)
	_, resolved, raceErr := o.resolveDeletionCreateRace(ctx, scope, "race-1", loserFP)
	require.ErrorIs(t, raceErr, ErrIdempotencyConflict)
	require.False(t, resolved)

	// An exact fingerprint match replays the winner's current receipt.
	replay, resolved, raceErr := o.resolveDeletionCreateRace(ctx, scope, "race-1", winnerFP)
	require.NoError(t, raceErr)
	require.True(t, resolved)
	require.Equal(t, DeletionStatusPartial, replay.Status)
	require.Equal(t, "race-1", replay.RequestID)

	// A race that resolved nothing durable stays unresolved.
	_, resolved, raceErr = o.resolveDeletionCreateRace(ctx, scope, "race-none", winnerFP)
	require.NoError(t, raceErr)
	require.False(t, resolved)
}

// TestPersistDeletionOutcomeNeverRegressesDeletedState pins ocr2-147: a late
// partial write from a concurrent same-request runner must never regress a
// finalized "deleted" receipt — the conditional write is a no-op and the
// stored terminal receipt replays instead.
func TestPersistDeletionOutcomeNeverRegressesDeletedState(t *testing.T) {
	o, db, ctx := newCareerExportOffice(t, "owner-1", 1964)
	scope, err := getScope(ctx)
	require.NoError(t, err)

	fp, err := careerDeletionFingerprint("regress-1", 0)
	require.NoError(t, err)
	execution := newDeletionExecution()
	for i := range execution.Steps {
		execution.Steps[i].Status = DeletionStepStatusDone
	}
	terminal := CareerDeletionReceipt{
		Kind: CareerKindDeleted, RequestID: "regress-1", Status: DeletionStatusDeleted,
		Retention: careerDeletionRetention(), Revision: 1,
	}
	now := time.Now().UTC()
	require.NoError(t, db.Create(&careerDataDeletionRecord{
		ID: uuid.NewString(), TenantID: scope.TenantID, UserID: scope.UserID,
		RequestID: "regress-1", Fingerprint: fp, ExpectedRevision: 0,
		Status: DeletionStatusDeleted, StateBody: string(mustJSON(execution)),
		ReceiptBody: string(mustJSON(terminal)), CreatedAt: now, UpdatedAt: now,
	}).Error)

	// The late runner's partial outcome loses to the finalized state.
	partialExecution := newDeletionExecution()
	partial := CareerDeletionReceipt{
		Kind: CareerKindDeleted, RequestID: "regress-1", Status: DeletionStatusPartial,
		Retention: careerDeletionRetention(),
	}
	returned, err := o.persistDeletionOutcome(ctx, scope, "regress-1", DeletionStatusPartial, &partialExecution, partial)
	require.NoError(t, err)
	require.Equal(t, DeletionStatusDeleted, returned.Status, "the stored terminal receipt replays")

	var row careerDataDeletionRecord
	require.NoError(t, db.Where("request_id=?", "regress-1").First(&row).Error)
	require.Equal(t, DeletionStatusDeleted, row.Status, "the terminal state must not regress")
	var stored CareerDeletionReceipt
	require.NoError(t, json.Unmarshal([]byte(row.ReceiptBody), &stored))
	require.Equal(t, DeletionStatusDeleted, stored.Status)
}
