package service

// Pass B test-support shim (Ruling 2026-09-24-TEST-SUPPORT-SHIM).
//
// datasource_service.go and its test fixtures migrated to
// internal/modules/datasource/service in 26-datasource B2-DS.3. Three staying
// K-owner test files still consume unexported symbols that used to live in the
// host package and cannot be edited by this node:
//
//   - knowledge_replace_test.go:223  → bytesToFileHeader (was a production
//     helper in datasource_service.go; verbatim copy below)
//   - knowledge_write_access_test.go:419 → processSyncTenantRepo (was a
//     fixture type in datasource_service_test.go; verbatim copy below)
//   - knowledge_write_access_test.go:431 → newSyncDeletionHarness /
//     syncDeletionHarness / processSyncTagService (were fixtures in
//     datasource_service_test.go; minimal re-implementation below, pinned by
//     re-running TestDataSourceTagCreationReceivesOnlyItsTaskKBGrant)
//
// Ledger: 「临时测试装置垫片（B5 清理范围）」 in the B2-DS.3 report.
// remove_at: ib2 (first-come-first-delete, at the latest B5) — collapses when
// the K-side tests migrate in the K4 supplementary window.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/modules/datasource"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

// bytesToFileHeader is a verbatim copy of the helper that used to live in
// datasource_service.go (consumed by knowledge_replace_test.go).
func bytesToFileHeader(data []byte, filename string) (*multipart.FileHeader, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Create a form file part
	partHeader := make(textproto.MIMEHeader)
	partHeader.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	partHeader.Set("Content-Type", "application/octet-stream")

	part, err := writer.CreatePart(partHeader)
	if err != nil {
		return nil, fmt.Errorf("create multipart part: %w", err)
	}

	if _, err := part.Write(data); err != nil {
		return nil, fmt.Errorf("write data to part: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	// Parse the multipart data to get a FileHeader
	reader := multipart.NewReader(&buf, writer.Boundary())
	form, err := reader.ReadForm(int64(len(data)) + 1024)
	if err != nil {
		return nil, fmt.Errorf("read multipart form: %w", err)
	}

	files := form.File["file"]
	if len(files) == 0 {
		return nil, fmt.Errorf("no file in multipart form")
	}

	return files[0], nil
}

// processSyncTenantRepo is a verbatim copy of the fixture that used to live in
// datasource_service_test.go (consumed by knowledge_write_access_test.go).
type processSyncTenantRepo struct {
	interfaces.TenantRepository
	tenant *types.Tenant
}

func (r *processSyncTenantRepo) GetTenantByID(context.Context, uint64) (*types.Tenant, error) {
	return r.tenant, nil
}

// processSyncTagService is a verbatim copy of the fixture that used to live in
// datasource_service_test.go (consumed by knowledge_write_access_test.go).
type processSyncTagService struct {
	interfaces.KnowledgeTagService
	ctx context.Context
}

func (s *processSyncTagService) FindOrCreateTagByName(
	ctx context.Context,
	_ string,
	_ string,
) (*types.KnowledgeTag, error) {
	s.ctx = ctx
	return nil, nil
}

// ── minimal re-implementation of the sync-deletion harness ──
//
// The migrated harness wires a full ProcessSync round over a one-deleted-item
// connector; the staying K-owner test (TestDataSourceTagCreationReceivesOnlyIts-
// TaskKBGrant) only exercises that single flow, so the shim reproduces exactly
// it: same deletedItemConnector semantics, same service wiring (compat
// constructor), same run() helper.

const shimDeletionConnectorType = "test-sync-deletion"

type shimDeletionConnector struct{}

func (shimDeletionConnector) Type() string { return shimDeletionConnectorType }
func (shimDeletionConnector) Validate(context.Context, *types.DataSourceConfig) error {
	return nil
}
func (shimDeletionConnector) ListResources(context.Context, *types.DataSourceConfig, string) ([]types.Resource, error) {
	return nil, nil
}
func (shimDeletionConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}
func (shimDeletionConnector) FetchAll(context.Context, *types.DataSourceConfig, []string) ([]types.FetchedItem, error) {
	return []types.FetchedItem{{
		ExternalID:       "file:gone",
		SourceResourceID: "folder:1",
		IsDeleted:        true,
	}}, nil
}
func (shimDeletionConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	items, err := (shimDeletionConnector{}).FetchAll(context.Background(), nil, nil)
	return items, nil, err
}

type shimKnowledgeRepo struct {
	interfaces.KnowledgeRepository
	knowledge *types.Knowledge
}

func (r *shimKnowledgeRepo) FindByDataSourceExternalID(
	context.Context, uint64, string, string, string,
) (*types.Knowledge, error) {
	return r.knowledge, nil
}

func (r *shimKnowledgeRepo) HardDeleteKnowledge(context.Context, uint64, string) error { return nil }
func (r *shimKnowledgeRepo) HardDeleteKnowledgeList(context.Context, uint64, []string) error {
	return nil
}

type shimKS struct {
	interfaces.KnowledgeService
	repo    interfaces.KnowledgeRepository
	deleted []string
}

func (k *shimKS) GetRepository() interfaces.KnowledgeRepository { return k.repo }
func (k *shimKS) DeleteKnowledge(context.Context, string) error { return nil }

type shimSyncLogRepo struct {
	logs map[string]*types.SyncLog
}

func (r *shimSyncLogRepo) Create(_ context.Context, log *types.SyncLog) error {
	r.logs[log.ID] = log
	return nil
}
func (r *shimSyncLogRepo) FindByID(_ context.Context, id string) (*types.SyncLog, error) {
	log, ok := r.logs[id]
	if !ok {
		return nil, errors.New("sync log not found")
	}
	return log, nil
}
func (r *shimSyncLogRepo) FindByDataSource(context.Context, string, int, int) ([]*types.SyncLog, error) {
	return nil, nil
}
func (r *shimSyncLogRepo) FindLatest(context.Context, string) (*types.SyncLog, error) {
	return nil, nil
}
func (r *shimSyncLogRepo) HasRunningSync(context.Context, string) (bool, error) {
	return false, nil
}
func (r *shimSyncLogRepo) Update(_ context.Context, log *types.SyncLog) error {
	r.logs[log.ID] = log
	return nil
}
func (r *shimSyncLogRepo) UpdateResult(_ context.Context, log *types.SyncLog) error {
	return r.Update(context.Background(), log)
}
func (r *shimSyncLogRepo) CancelPendingByDataSource(context.Context, string) error { return nil }
func (r *shimSyncLogRepo) CleanupOldLogs(context.Context, int) error               { return nil }
func (r *shimSyncLogRepo) UpdateHeartbeat(context.Context, string, time.Time) error {
	return nil
}
func (r *shimSyncLogRepo) UpdateAsynqTaskID(context.Context, string, string) error { return nil }
func (r *shimSyncLogRepo) RequestCancel(context.Context, string) error             { return nil }

var _ interfaces.SyncLogRepository = (*shimSyncLogRepo)(nil)

// syncDeletionHarness mirrors the migrated harness surface the staying K-owner
// test touches: h.run(t), h.ds, h.svc.tagService.(*processSyncTagService).
// h.svc is a shim wrapper because the migrated type's tagService field is an
// unexported field of the module-side type and no longer reachable in-package.
type syncDeletionHarness struct {
	ds        *types.DataSource
	syncLogID string
	svc       *shimDeletionSvc
}

type shimDeletionSvc struct {
	*DataSourceService
	tagService interfaces.KnowledgeTagService
}

// newSyncDeletionHarness rebuilds the full-sync ProcessSync fixture. The repo /
// ks parameters keep the migrated signature shape (callers pass nil, nil).
func newSyncDeletionHarness(
	t *testing.T, syncDeletions bool, dsID, logID string,
	repo interface{}, ks interface{},
) *syncDeletionHarness {
	t.Helper()
	configJSON, err := (&types.DataSourceConfig{Type: shimDeletionConnectorType}).ToJSON()
	require.NoError(t, err)

	ds := &types.DataSource{
		ID:              dsID,
		TenantID:        1,
		KnowledgeBaseID: "kb-1",
		Name:            "Sync Deletion",
		Type:            shimDeletionConnectorType,
		Config:          configJSON,
		SyncMode:        types.SyncModeFull,
		Status:          types.DataSourceStatusActive,
		SyncDeletions:   syncDeletions,
	}
	syncLog := &types.SyncLog{
		ID:           logID,
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}
	syncLogRepo := &shimSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}
	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(shimDeletionConnector{}))

	tags := &processSyncTagService{}
	svc := NewDataSourceService(
		newKBDeleteDSRepo(ds.KnowledgeBaseID, ds), syncLogRepo,
		&shimKS{repo: &shimKnowledgeRepo{knowledge: &types.Knowledge{ID: "knowledge-gone"}}},
		newShimKBService(&types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID}),
		nil, registry, nil, &processSyncTenantRepo{tenant: &types.Tenant{ID: ds.TenantID}},
		tags, nil,
	).(*DataSourceService)

	return &syncDeletionHarness{ds: ds, syncLogID: syncLog.ID, svc: &shimDeletionSvc{DataSourceService: svc, tagService: tags}}
}

// run executes a full sync round (same contract as the migrated harness).
func (h *syncDeletionHarness) run(t *testing.T) (*types.SyncLog, error) {
	t.Helper()
	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: h.ds.ID,
		TenantID:     h.ds.TenantID,
		SyncLogID:    h.syncLogID,
		ForceFull:    true,
	})
	require.NoError(t, err)
	err = h.svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload))
	return nil, err
}

// shimKBService is the minimal KnowledgeBaseService the harness ProcessSync
// round needs (GetKnowledgeBaseByID only; other methods inherit the embedded
// interface's nil panic, never called on this path).
type shimKBService struct {
	interfaces.KnowledgeBaseService
	kb *types.KnowledgeBase
}

func (s *shimKBService) GetKnowledgeBaseByID(context.Context, string) (*types.KnowledgeBase, error) {
	return s.kb, nil
}

func newShimKBService(kb *types.KnowledgeBase) interfaces.KnowledgeBaseService {
	return &shimKBService{kb: kb}
}
