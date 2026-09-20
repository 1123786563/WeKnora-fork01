package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────────────
// SP2-a Task 8 fakes: the purge worker drives the REAL batched delete
// pipeline (planKnowledgeDelete → executeKnowledgeDelete) against a sqlite
// database, so only the out-of-process dependencies (graph engine, KB
// lookup, chunk repo, tenant repo) are faked, mirroring how
// ProcessKnowledgeListDelete tests stub the same seams.
// ──────────────────────────────────────────────────────────────────────

// purgeGraphRepo absorbs the graph cleanup step (nothing to assert there).
type purgeGraphRepo struct {
	interfaces.RetrieveGraphRepository
}

func (purgeGraphRepo) DelGraph(context.Context, []types.NameSpace) error { return nil }

// purgeKBLookup serves the one KB the delete plan resolves.
type purgeKBLookup struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (l *purgeKBLookup) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return l.kb, nil
}

// purgeChunkRepo fakes the two chunk-repo calls executeKnowledgeDelete makes.
type purgeChunkRepo struct {
	interfaces.ChunkRepository
}

func (r *purgeChunkRepo) ListImageInfoByKnowledgeIDs(
	context.Context, uint64, []string,
) ([]interfaces.ChunkImageInfo, error) {
	return nil, nil
}

func (r *purgeChunkRepo) DeleteByKnowledgeList(context.Context, uint64, []string) error {
	return nil
}

// purgeTenantRepo satisfies withKBWriteTenantInfo / AdjustStorageUsed without
// a tenants table.
type purgeTenantRepo struct{ interfaces.TenantRepository }

func (purgeTenantRepo) GetTenantByID(_ context.Context, id uint64) (*types.Tenant, error) {
	return &types.Tenant{ID: id}, nil
}

func (purgeTenantRepo) AdjustStorageUsed(context.Context, uint64, int64) error { return nil }

// purgeEnqueuer records every enqueued task so tests can assert the purge
// task's type and payload, then replay it through the real worker entrypoint.
type purgeEnqueuer struct {
	tasks []*asynq.Task
}

func (e *purgeEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	e.tasks = append(e.tasks, task)
	return &asynq.TaskInfo{ID: fmt.Sprintf("purge-task-%d", len(e.tasks)), Payload: task.Payload()}, nil
}

// purgeAuditSink records the KB-activity audit entries recordKBActivity emits
// so tests can assert the purge_documents / purged_documents detail fields.
type purgeAuditSink struct {
	interfaces.AuditLogService
	entries []*types.AuditLog
}

func (a *purgeAuditSink) Log(_ context.Context, entry *types.AuditLog) error {
	a.entries = append(a.entries, entry)
	return nil
}

func (a *purgeAuditSink) findByAction(action types.AuditAction) []*types.AuditLog {
	var found []*types.AuditLog
	for _, e := range a.entries {
		if e.Action == action {
			found = append(found, e)
		}
	}
	return found
}

// datasourcePurgeFixture is a sqlite-backed SP2-a Task 8 fixture: one data
// source ("Purge Source", ds) with synced documents + auto-tag + binding, one
// sibling data source with its own document, and one manual document that can
// carry the same-named tag.
type datasourcePurgeFixture struct {
	db         *gorm.DB
	dsRepo     interfaces.DataSourceRepository
	syncLog    interfaces.SyncLogRepository
	scheduler  *datasource.Scheduler
	enqueuer   *purgeEnqueuer
	audit      *purgeAuditSink
	svc        *DataSourceService
	kbID       string
	ds         *types.DataSource
	otherDS    *types.DataSource
	dsDocs     []string
	otherDocs  []string
	manualDocs []string
	autoTagID  string
}

const purgeTenantID uint64 = 1

// newDataSourcePurgeFixture wires the full purge chain. When
// manualDocSharesAutoTag is true the manual document also carries the data
// source's auto-tag (simulating a user manually tagging a doc with the same
// name), so the tag must survive the purge as a non-orphan.
func newDataSourcePurgeFixture(t *testing.T, manualDocSharesAutoTag bool) *datasourcePurgeFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "weknora.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&types.DataSource{}, &types.SyncLog{}, &types.Knowledge{}, &types.Chunk{},
		&types.KnowledgeTag{}, &types.KnowledgeTagRelation{},
		&repository.AppDataSourceBindingRow{},
	))

	dsRepo := repository.NewDataSourceRepository(db)
	syncLogRepo := repository.NewSyncLogRepository(db)

	kbID := "kb-purge"
	ds := &types.DataSource{
		ID:              "ds-purge",
		TenantID:        purgeTenantID,
		KnowledgeBaseID: kbID,
		Name:            "Purge Source",
		Type:            types.ConnectorTypeFeishu,
		Status:          types.DataSourceStatusActive,
	}
	otherDS := &types.DataSource{
		ID:              "ds-other",
		TenantID:        purgeTenantID,
		KnowledgeBaseID: kbID,
		Name:            "Other Source",
		Type:            types.ConnectorTypeFeishu,
		Status:          types.DataSourceStatusActive,
	}
	require.NoError(t, dsRepo.Create(context.Background(), ds))
	require.NoError(t, dsRepo.Create(context.Background(), otherDS))

	// insertDoc creates one knowledge row through GORM (not raw SQL) so
	// created_at/updated_at are written the same way the CAS in
	// UpdateKnowledgeForTransfer reads them back.
	insertDoc := func(id, dsID, title string) {
		t.Helper()
		metadata := "{}"
		if dsID != "" {
			metadata = fmt.Sprintf(`{"datasource_id":%q}`, dsID)
		}
		require.NoError(t, db.Create(&types.Knowledge{
			ID:              id,
			TenantID:        purgeTenantID,
			KnowledgeBaseID: kbID,
			Type:            "document",
			Title:           title,
			Source:          "manual",
			ParseStatus:     "completed",
			Metadata:        types.JSON(metadata),
			CustomMetadata:  types.JSON("{}"),
		}).Error)
	}

	f := &datasourcePurgeFixture{db: db, dsRepo: dsRepo, syncLog: syncLogRepo, kbID: kbID, ds: ds, otherDS: otherDS}
	for _, id := range []string{"doc-purge-1", "doc-purge-2", "doc-purge-3"} {
		insertDoc(id, ds.ID, id)
		f.dsDocs = append(f.dsDocs, id)
	}
	insertDoc("doc-other-1", otherDS.ID, "doc-other-1")
	f.otherDocs = []string{"doc-other-1"}
	insertDoc("doc-manual-1", "", "doc-manual-1")
	f.manualDocs = []string{"doc-manual-1"}

	// Per-ds auto-tag (resolveAutoTagIDs names it after the data source) plus
	// relations to every synced document.
	autoTag := &types.KnowledgeTag{
		ID: "tag-auto-purge", TenantID: purgeTenantID, KnowledgeBaseID: kbID, Name: ds.Name,
	}
	require.NoError(t, db.Create(autoTag).Error)
	f.autoTagID = autoTag.ID
	for _, id := range f.dsDocs {
		require.NoError(t, db.Create(&types.KnowledgeTagRelation{KnowledgeID: id, TagID: autoTag.ID}).Error)
	}
	if manualDocSharesAutoTag {
		require.NoError(t, db.Create(&types.KnowledgeTagRelation{
			KnowledgeID: "doc-manual-1", TagID: autoTag.ID,
		}).Error)
	}

	// app_datasource_bindings row for the deleted source (and one sibling row
	// that must survive).
	require.NoError(t, db.Create(&repository.AppDataSourceBindingRow{
		ID: "bind-purge", TenantID: purgeTenantID, DataSourceID: ds.ID,
		InstallationID: "inst-1", ConnectionID: "conn-1", AuthVersion: 2,
	}).Error)
	require.NoError(t, db.Create(&repository.AppDataSourceBindingRow{
		ID: "bind-other", TenantID: purgeTenantID, DataSourceID: otherDS.ID,
		InstallationID: "inst-2", ConnectionID: "conn-2", AuthVersion: 1,
	}).Error)

	kb := &types.KnowledgeBase{ID: kbID, TenantID: purgeTenantID, Name: "Purge KB"}
	knowledgeService := &knowledgeService{
		repo:        repository.NewKnowledgeRepository(db),
		kbService:   &purgeKBLookup{kb: kb},
		tenantRepo:  purgeTenantRepo{},
		chunkRepo:   &purgeChunkRepo{},
		graphEngine: purgeGraphRepo{},
	}
	tagService := &knowledgeTagService{
		kbService: &purgeKBLookup{kb: kb},
		repo:      repository.NewKnowledgeTagRepository(db),
	}

	f.enqueuer = &purgeEnqueuer{}
	f.audit = &purgeAuditSink{}
	f.scheduler = datasource.NewScheduler(dsRepo, syncLogRepo, nil)
	f.svc = &DataSourceService{
		dsRepo:           dsRepo,
		syncLogRepo:      syncLogRepo,
		knowledgeService: knowledgeService,
		tagService:       tagService,
		taskEnqueuer:     f.enqueuer,
		scheduler:        f.scheduler,
		audit:            f.audit,
	}
	return f
}

// knowledgeRowUnscoped fetches one knowledge row including tombstones; a nil
// row means fully gone (neither soft- nor hard-deleted state remains).
func (f *datasourcePurgeFixture) knowledgeRowUnscoped(t *testing.T, id string) *types.Knowledge {
	t.Helper()
	var row types.Knowledge
	err := f.db.Unscoped().Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	require.NoError(t, err)
	return &row
}

func (f *datasourcePurgeFixture) countLiveKnowledge(t *testing.T, ids []string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.db.Model(&types.Knowledge{}).
		Where("id IN ?", ids).Count(&n).Error)
	return n
}

func (f *datasourcePurgeFixture) tagRow(t *testing.T, id string) *types.KnowledgeTag {
	t.Helper()
	var tag types.KnowledgeTag
	err := f.db.Where("id = ?", id).First(&tag).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	require.NoError(t, err)
	return &tag
}

func (f *datasourcePurgeFixture) bindingRows(t *testing.T, dsID string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.db.Table("app_datasource_bindings").
		Where("datasource_id = ?", dsID).Count(&n).Error)
	return n
}

func (f *datasourcePurgeFixture) relationCount(t *testing.T, knowledgeID, tagID string) int64 {
	t.Helper()
	var n int64
	require.NoError(t, f.db.Model(&types.KnowledgeTagRelation{}).
		Where("knowledge_id = ? AND tag_id = ?", knowledgeID, tagID).Count(&n).Error)
	return n
}

// replayPurgeTask feeds the recorded enqueue payload through the real asynq
// worker entrypoint (unmarshal + guards + drain loop), exactly what a worker
// process would do with the queued task.
func (f *datasourcePurgeFixture) replayPurgeTask(t *testing.T, ctx context.Context) error {
	t.Helper()
	require.Len(t, f.enqueuer.tasks, 1, "exactly one purge task must have been enqueued")
	task := asynq.NewTask(f.enqueuer.tasks[0].Type(), f.enqueuer.tasks[0].Payload())
	return f.svc.ProcessDataSourcePurge(ctx, task)
}

// ──────────────────────────────────────────────────────────────────────
// Tests
// ──────────────────────────────────────────────────────────────────────

// purge=true enqueues exactly one datasource:purge task scoped to the deleted
// source, and the worker then removes every synced document (soft AND hard
// rows), the binding row, while sibling-source and manual documents, their
// tags and relations stay untouched. The auto-tag survives here because the
// manual document still references it (not an orphan).
func TestDeleteDataSourcePurgeEnqueuesTaskAndWorkerDrainsDocuments(t *testing.T) {
	f := newDataSourcePurgeFixture(t, true)

	require.NoError(t, f.svc.DeleteDataSource(context.Background(), f.ds.ID, true))

	// The delete-time audit carries purge_documents=true.
	deleted := f.audit.findByAction(types.AuditActionDataSourceDeleted)
	require.Len(t, deleted, 1)
	assert.Contains(t, string(deleted[0].Details), `"purge_documents":true`)

	// Enqueue assertions: exactly one task, right type, right scope.
	require.Len(t, f.enqueuer.tasks, 1)
	task := f.enqueuer.tasks[0]
	assert.Equal(t, types.TypeDataSourcePurge, task.Type())
	var payload types.DataSourcePurgePayload
	require.NoError(t, json.Unmarshal(task.Payload(), &payload))
	assert.Equal(t, purgeTenantID, payload.TenantID)
	assert.Equal(t, f.kbID, payload.KnowledgeBaseID)
	assert.Equal(t, f.ds.ID, payload.DataSourceID)
	assert.Equal(t, f.ds.Name, payload.TagName)

	// The delete itself stays async: documents still exist right after.
	assert.Equal(t, int64(3), f.countLiveKnowledge(t, f.dsDocs))

	// Run the worker with the recorded payload.
	require.NoError(t, f.replayPurgeTask(t, context.Background()))

	// Synced documents fully gone — no tombstone rows either.
	for _, id := range f.dsDocs {
		assert.Nil(t, f.knowledgeRowUnscoped(t, id), "document %s must be fully removed", id)
	}
	// Sibling source's document and the manual document survive.
	assert.NotNil(t, f.knowledgeRowUnscoped(t, "doc-other-1"))
	assert.NotNil(t, f.knowledgeRowUnscoped(t, "doc-manual-1"))

	// Binding rows: deleted source's row gone, sibling's kept.
	assert.Equal(t, int64(0), f.bindingRows(t, f.ds.ID))
	assert.Equal(t, int64(1), f.bindingRows(t, f.otherDS.ID))

	// Auto-tag still referenced by the manual document → not an orphan, kept,
	// and the manual document's relation is intact.
	assert.NotNil(t, f.tagRow(t, f.autoTagID))
	assert.Equal(t, int64(1), f.relationCount(t, "doc-manual-1", f.autoTagID))

	// Replaying the worker is idempotent (drain finds nothing, cleanup no-ops).
	require.NoError(t, f.replayPurgeTask(t, context.Background()))
	assert.NotNil(t, f.knowledgeRowUnscoped(t, "doc-manual-1"))

	// The worker's completion audit reports the total purged count.
	completed := f.audit.findByAction(types.AuditActionDataSourceDeleted)
	require.Len(t, completed, 2)
	assert.Contains(t, string(completed[1].Details), `"purged_documents":3`)
	assert.Contains(t, string(completed[1].Details), `"purge_completed":true`)
}

// When no live document references the auto-tag anymore, the purge tail
// removes it; an unrelated tag (also unreferenced by this source) stays.
func TestPurgeWorkerRemovesOrphanAutoTag(t *testing.T) {
	f := newDataSourcePurgeFixture(t, false)
	keepTag := &types.KnowledgeTag{
		ID: "tag-keep", TenantID: purgeTenantID, KnowledgeBaseID: f.kbID, Name: "Keep Me",
	}
	require.NoError(t, f.db.Create(keepTag).Error)
	require.NoError(t, f.db.Create(&types.KnowledgeTagRelation{
		KnowledgeID: "doc-manual-1", TagID: keepTag.ID,
	}).Error)

	require.NoError(t, f.svc.DeleteDataSource(context.Background(), f.ds.ID, true))
	require.NoError(t, f.replayPurgeTask(t, context.Background()))

	assert.Nil(t, f.tagRow(t, f.autoTagID), "orphaned auto-tag must be deleted")
	assert.NotNil(t, f.tagRow(t, keepTag.ID), "still-referenced unrelated tag must survive")
	assert.Equal(t, int64(1), f.relationCount(t, "doc-manual-1", keepTag.ID))
}

// purge=false must be byte-identical with the pre-SP2-a behavior: no enqueue,
// documents/tag/binding all stay.
func TestDeleteDataSourceWithoutPurgeKeepsDocuments(t *testing.T) {
	f := newDataSourcePurgeFixture(t, false)

	require.NoError(t, f.svc.DeleteDataSource(context.Background(), f.ds.ID, false))

	assert.Empty(t, f.enqueuer.tasks, "no purge task may be enqueued without the flag")
	assert.Equal(t, int64(3), f.countLiveKnowledge(t, f.dsDocs))
	assert.NotNil(t, f.tagRow(t, f.autoTagID))
	assert.Equal(t, int64(1), f.bindingRows(t, f.ds.ID))

	// The delete-time audit carries no purge marker at all.
	deleted := f.audit.findByAction(types.AuditActionDataSourceDeleted)
	require.Len(t, deleted, 1)
	assert.NotContains(t, string(deleted[0].Details), "purge_documents")

	// And the source itself is gone (the legacy delete path still ran).
	_, err := f.dsRepo.FindByID(context.Background(), f.ds.ID)
	require.EqualError(t, err, "data source not found")
}

// More documents than one batch: the drain loop keeps fetching batches until
// the query comes back empty.
func TestPurgeWorkerDrainsAcrossBatches(t *testing.T) {
	f := newDataSourcePurgeFixture(t, false)
	var allIDs []string
	for i := 0; i < 205; i++ {
		id := fmt.Sprintf("doc-batch-%03d", i)
		require.NoError(t, f.db.Create(&types.Knowledge{
			ID: id, TenantID: purgeTenantID, KnowledgeBaseID: f.kbID,
			Type: "document", Title: id, Source: "manual", ParseStatus: "completed",
			Metadata:       types.JSON(fmt.Sprintf(`{"datasource_id":%q}`, f.ds.ID)),
			CustomMetadata: types.JSON("{}"),
		}).Error)
		allIDs = append(allIDs, id)
	}

	require.NoError(t, f.svc.PurgeDataSourceDocuments(context.Background(), types.DataSourcePurgePayload{
		TenantID: purgeTenantID, KnowledgeBaseID: f.kbID, DataSourceID: f.ds.ID, TagName: f.ds.Name,
	}))

	var n int64
	require.NoError(t, f.db.Unscoped().Model(&types.Knowledge{}).
		Where("id IN ?", allIDs).Count(&n).Error)
	assert.Equal(t, int64(0), n, "every batched document must be purged, tombstones included")
}

// A canceled context interrupts the drain BETWEEN batches: the first batch
// completes fully, the loop-top ctx.Err() check stops the run before the
// second batch is even fetched, and the remainder survives for the asynq
// retry.
func TestPurgeWorkerStopsBetweenBatchesWhenContextCanceled(t *testing.T) {
	f := newDataSourcePurgeFixture(t, false)
	var allIDs []string
	for i := 0; i < 205; i++ {
		id := fmt.Sprintf("doc-cancel-%03d", i)
		require.NoError(t, f.db.Create(&types.Knowledge{
			ID: id, TenantID: purgeTenantID, KnowledgeBaseID: f.kbID,
			Type: "document", Title: id, Source: "manual", ParseStatus: "completed",
			Metadata:       types.JSON(fmt.Sprintf(`{"datasource_id":%q}`, f.ds.ID)),
			CustomMetadata: types.JSON("{}"),
		}).Error)
		allIDs = append(allIDs, id)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel right after the first batch's DeleteKnowledgeList returns — the
	// exact boundary the drain loop's ctx.Err() check guards.
	svc := &DataSourceService{
		dsRepo:           f.dsRepo,
		knowledgeService: &cancelAfterBatchKS{KnowledgeService: f.svc.knowledgeService, cancel: cancel},
	}

	err := svc.PurgeDataSourceDocuments(ctx, types.DataSourcePurgePayload{
		TenantID: purgeTenantID, KnowledgeBaseID: f.kbID, DataSourceID: f.ds.ID, TagName: f.ds.Name,
	})
	require.ErrorIs(t, err, context.Canceled)

	var remaining int64
	require.NoError(t, f.db.Model(&types.Knowledge{}).Where("id IN ?", allIDs).Count(&remaining).Error)
	assert.Equal(t, int64(5), remaining, "the un-drained remainder must survive for the retry")
}

// cancelAfterBatchKS cancels the run context as soon as one batch delete
// returns, simulating an asynq cancellation landing exactly on the batch
// boundary.
type cancelAfterBatchKS struct {
	interfaces.KnowledgeService
	cancel context.CancelFunc
}

func (w *cancelAfterBatchKS) DeleteKnowledgeList(ctx context.Context, ids []string) error {
	if err := w.KnowledgeService.DeleteKnowledgeList(ctx, ids); err != nil {
		return err
	}
	w.cancel()
	return nil
}

// A context canceled before the run deletes nothing at all.
func TestPurgeWorkerNoopOnPreCanceledContext(t *testing.T) {
	f := newDataSourcePurgeFixture(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := f.svc.PurgeDataSourceDocuments(ctx, types.DataSourcePurgePayload{
		TenantID: purgeTenantID, KnowledgeBaseID: f.kbID, DataSourceID: f.ds.ID, TagName: f.ds.Name,
	})
	require.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int64(3), f.countLiveKnowledge(t, f.dsDocs))
	assert.Equal(t, int64(1), f.bindingRows(t, f.ds.ID), "tail cleanup must not run on cancel")
}

// Invalid payloads fail fast with SkipRetry so asynq drops them instead of
// retrying forever.
func TestProcessDataSourcePurgeRejectsInvalidPayload(t *testing.T) {
	f := newDataSourcePurgeFixture(t, false)
	for name, payload := range map[string]types.DataSourcePurgePayload{
		"missing tenant":     {KnowledgeBaseID: f.kbID, DataSourceID: f.ds.ID},
		"missing kb":         {TenantID: purgeTenantID, DataSourceID: f.ds.ID},
		"missing datasource": {TenantID: purgeTenantID, KnowledgeBaseID: f.kbID},
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(payload)
			require.NoError(t, err)
			task := asynq.NewTask(types.TypeDataSourcePurge, data)
			require.ErrorIs(t, f.svc.ProcessDataSourcePurge(context.Background(), task), asynq.SkipRetry)
		})
	}
}

// The purge task is declared on the maintenance queue topology so producers
// and worker pools agree on where it runs.
func TestDataSourcePurgeQueueTopology(t *testing.T) {
	queue, ok := types.QueueForTaskType(types.TypeDataSourcePurge)
	require.True(t, ok, "datasource:purge must be registered in the queue topology")
	assert.Equal(t, types.QueueMaintenance, queue)
}
