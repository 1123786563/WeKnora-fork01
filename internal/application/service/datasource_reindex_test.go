package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/datasource"
	"github.com/Tencent/WeKnora/internal/datasource/connector/ima"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── scoped-reindex fixtures ──────────────────────────────────────────────

const scopedReindexConnectorType = "test-scoped-reindex"

// scopedReindexConnector is a targeted-fetcher connector whose per-external-id
// behaviour is table-driven: errs wins over items, and an id in neither table
// reads as not-found (ErrItemNotFound) like the real connectors.
type scopedReindexConnector struct {
	items         map[string]*types.FetchedItem
	errs          map[string]error
	fetched       []string
	fetchAllCalls int
}

func (c *scopedReindexConnector) Type() string { return scopedReindexConnectorType }
func (c *scopedReindexConnector) Validate(context.Context, *types.DataSourceConfig) error {
	return nil
}
func (c *scopedReindexConnector) ListResources(
	context.Context, *types.DataSourceConfig, string,
) ([]types.Resource, error) {
	return nil, nil
}
func (c *scopedReindexConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}

// FetchAll must never run for a scoped payload: the scoped branch executes
// before the batch fetch. The counter lets tests prove it.
func (c *scopedReindexConnector) FetchAll(
	context.Context, *types.DataSourceConfig, []string,
) ([]types.FetchedItem, error) {
	c.fetchAllCalls++
	return nil, nil
}

func (c *scopedReindexConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	return nil, nil, nil
}

func (c *scopedReindexConnector) FetchByExternalID(
	_ context.Context, _ *types.DataSourceConfig, externalID string,
) (*types.FetchedItem, error) {
	c.fetched = append(c.fetched, externalID)
	if err, ok := c.errs[externalID]; ok {
		return nil, err
	}
	if item, ok := c.items[externalID]; ok {
		return item, nil
	}
	return nil, fmt.Errorf("%w: %s", datasource.ErrItemNotFound, externalID)
}

const unsupportedTargetedConnectorType = "test-reindex-unsupported"

// unsupportedTargetedConnector implements only the base Connector interface,
// modelling a connector that has not adopted TargetedFetcher yet.
type unsupportedTargetedConnector struct {
	fetchAllCalls int
}

func (c *unsupportedTargetedConnector) Type() string { return unsupportedTargetedConnectorType }
func (c *unsupportedTargetedConnector) Validate(context.Context, *types.DataSourceConfig) error {
	return nil
}
func (c *unsupportedTargetedConnector) ListResources(
	context.Context, *types.DataSourceConfig, string,
) ([]types.Resource, error) {
	return nil, nil
}
func (c *unsupportedTargetedConnector) ResolveResourceAncestors(
	context.Context, *types.DataSourceConfig, []string,
) ([]string, error) {
	return nil, nil
}
func (c *unsupportedTargetedConnector) FetchAll(
	context.Context, *types.DataSourceConfig, []string,
) ([]types.FetchedItem, error) {
	c.fetchAllCalls++
	return nil, nil
}
func (c *unsupportedTargetedConnector) FetchIncremental(
	context.Context, *types.DataSourceConfig, *types.SyncCursor,
) ([]types.FetchedItem, *types.SyncCursor, error) {
	return nil, nil, nil
}

// scopedRecordingDSRepo counts every data-source write so the double-write
// convergence ruling (scoped runs never touch ds state) can be asserted.
type scopedRecordingDSRepo struct {
	kbDeleteDSRepo
	syncStateWrites int
	rowWrites       int
}

func (r *scopedRecordingDSRepo) UpdateSyncState(_ context.Context, _ *types.DataSource) error {
	r.syncStateWrites++
	return r.kbDeleteDSRepo.UpdateSyncState(nil, nil)
}

func (r *scopedRecordingDSRepo) Update(_ context.Context, _ *types.DataSource) error {
	r.rowWrites++
	return nil
}

// scopedReindexRepo builds on the sweep fixture: the parent lookup returns a
// configurable row (the update path's delete-before-create), and hard deletes
// are recorded.
type scopedReindexRepo struct {
	sweepFakeRepo
	parent       *types.Knowledge
	hardDeleted  []string
	hardDeletedL []string
}

func (r *scopedReindexRepo) FindByDataSourceExternalID(
	context.Context, uint64, string, string, string,
) (*types.Knowledge, error) {
	return r.parent, nil
}

func (r *scopedReindexRepo) HardDeleteKnowledge(_ context.Context, _ uint64, id string) error {
	r.hardDeleted = append(r.hardDeleted, id)
	return nil
}

func (r *scopedReindexRepo) HardDeleteKnowledgeList(_ context.Context, _ uint64, ids []string) error {
	r.hardDeletedL = append(r.hardDeletedL, ids...)
	return nil
}

const scopedReindexLastSyncResult = `{"total":99,"created":50,"failed":1}`

// scopedReindexHarness wires a DataSourceService around a connector and one
// running sync log, with sentinel ds state (LastSyncResult/ErrorMessage) left
// by a previous whole-source run.
type scopedReindexHarness struct {
	ds          *types.DataSource
	syncLog     *types.SyncLog
	syncLogRepo *processSyncSyncLogRepo
	dsRepo      *scopedRecordingDSRepo
	repo        *scopedReindexRepo
	ks          *sweepFakeKS
	svc         *DataSourceService
}

func newScopedReindexHarness(
	t *testing.T, connector datasource.Connector, repo *scopedReindexRepo,
) *scopedReindexHarness {
	t.Helper()
	configJSON, err := (&types.DataSourceConfig{Type: connector.Type()}).ToJSON()
	require.NoError(t, err)

	ds := &types.DataSource{
		ID:              "ds-reindex",
		TenantID:        7,
		KnowledgeBaseID: "kb-reindex",
		Name:            "Scoped Source",
		Type:            connector.Type(),
		Config:          configJSON,
		SyncMode:        types.SyncModeIncremental,
		Status:          types.DataSourceStatusActive,
		LastSyncResult:  types.JSON(scopedReindexLastSyncResult),
		ErrorMessage:    "prior whole-source failure",
	}
	syncLog := &types.SyncLog{
		ID:           "log-reindex",
		DataSourceID: ds.ID,
		TenantID:     ds.TenantID,
		Status:       types.SyncLogStatusRunning,
		StartedAt:    time.Now().UTC(),
	}
	syncLogRepo := &processSyncSyncLogRepo{logs: map[string]*types.SyncLog{syncLog.ID: syncLog}}

	registry := datasource.NewConnectorRegistry()
	require.NoError(t, registry.Register(connector))

	if repo == nil {
		repo = &scopedReindexRepo{}
	}
	ks := &sweepFakeKS{repo: repo}
	dsRepo := &scopedRecordingDSRepo{
		kbDeleteDSRepo: kbDeleteDSRepo{
			byKB:    map[string][]*types.DataSource{"kb-reindex": {ds}},
			deleted: map[string]bool{},
		},
	}

	return &scopedReindexHarness{
		ds:          ds,
		syncLog:     syncLog,
		syncLogRepo: syncLogRepo,
		dsRepo:      dsRepo,
		repo:        repo,
		ks:          ks,
		svc: &DataSourceService{
			dsRepo:            dsRepo,
			syncLogRepo:       syncLogRepo,
			knowledgeService:  ks,
			kbService:         &processSyncKBService{kb: &types.KnowledgeBase{ID: ds.KnowledgeBaseID, TenantID: ds.TenantID}},
			connectorRegistry: registry,
			tenantRepo:        &processSyncTenantRepo{tenant: &types.Tenant{ID: ds.TenantID}},
			tagService:        &processSyncTagService{},
		},
	}
}

func (h *scopedReindexHarness) run(t *testing.T, ids ...string) (*types.SyncLog, error) {
	t.Helper()
	payload, err := json.Marshal(types.DataSourceSyncPayload{
		DataSourceID: h.ds.ID,
		TenantID:     h.ds.TenantID,
		SyncLogID:    h.syncLog.ID,
		Trigger:      "manual_reindex",
		Scope:        &types.SyncScope{ExternalIDs: ids},
	})
	require.NoError(t, err)

	err = h.svc.ProcessSync(context.Background(), asynq.NewTask(types.TypeDataSourceSync, payload))
	updated := h.syncLogRepo.logs[h.syncLog.ID]
	require.NotNil(t, updated)
	return updated, err
}

// assertDSUntouched pins the double-write convergence ruling (Ruling P-4): a
// scoped reindex must leave the data source row byte-identical — no writes at
// all, LastSyncResult/Status/ErrorMessage exactly as the last whole-source run
// left them.
func (h *scopedReindexHarness) assertDSUntouched(t *testing.T) {
	t.Helper()
	assert.Zero(t, h.dsRepo.syncStateWrites, "scoped reindex must never UpdateSyncState the data source")
	assert.Zero(t, h.dsRepo.rowWrites, "scoped reindex must never write the data source row")
	assert.Equal(t, types.DataSourceStatusActive, h.ds.Status, "ds.Status must not flip")
	assert.JSONEq(t, scopedReindexLastSyncResult, string(h.ds.LastSyncResult),
		"LastSyncResult must keep the last whole-source run's value")
	assert.Equal(t, "prior whole-source failure", h.ds.ErrorMessage, "ErrorMessage must not be rewritten")
}

func decodeSyncResult(t *testing.T, log *types.SyncLog) types.SyncResult {
	t.Helper()
	var result types.SyncResult
	require.NoError(t, json.Unmarshal(log.Result, &result))
	return result
}

// ── ProcessSync scoped branch ────────────────────────────────────────────

// TestProcessSync_ScopedReindexHappyPath runs the full scoped chain: each
// external id is refetched through TargetedFetcher and ingested via the shared
// applyFetchedItem core, converging into this run's own SyncLog only.
func TestProcessSync_ScopedReindexHappyPath(t *testing.T) {
	connector := &scopedReindexConnector{items: map[string]*types.FetchedItem{
		"doc-a": {ExternalID: "doc-a", Title: "Doc A", Content: []byte("alpha body"), FileName: "a.md"},
		"doc-b": {ExternalID: "doc-b", Title: "Doc B", Content: []byte("beta body"), FileName: "b.md"},
	}}
	h := newScopedReindexHarness(t, connector, nil)

	log, err := h.run(t, "doc-a", "doc-b")
	require.NoError(t, err)

	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	require.NotNil(t, log.FinishedAt)
	assert.Equal(t, 2, log.ItemsTotal)
	assert.Equal(t, 2, log.ItemsCreated)
	assert.Equal(t, 0, log.ItemsFailed)
	assert.Empty(t, log.ErrorMessage)

	result := decodeSyncResult(t, log)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 2, result.Created)
	assert.Empty(t, result.Errors)

	assert.Equal(t, []string{"doc-a", "doc-b"}, connector.fetched,
		"every scoped id must be refetched, in request order")
	assert.Zero(t, connector.fetchAllCalls, "scoped runs must not walk the whole source")

	assert.Equal(t, []string{"create:a.md", "create:b.md"}, h.ks.events)
	h.assertDSUntouched(t)
}

// TestProcessSync_ScopedReindexPerItemFailureCodes verifies every per-item
// failure classification: not_found (ErrItemNotFound), fetch_failed with the
// raw cause text (so later tasks can tell transient from permanent), and the
// ima degradation (targeted_unsupported with its remedy text). Per the status
// ruling the run still converges success: failures live in the result, not in
// the status — and never in ds state.
func TestProcessSync_ScopedReindexPerItemFailureCodes(t *testing.T) {
	connector := &scopedReindexConnector{
		items: map[string]*types.FetchedItem{
			"ok": {ExternalID: "ok", Title: "Fine", Content: []byte("still here"), FileName: "ok.md"},
		},
		errs: map[string]error{
			"gone":      fmt.Errorf("%w: wiki node no longer in space", datasource.ErrItemNotFound),
			"transient": errors.New("upstream 503: rate limited"),
			"ima-hash":  fmt.Errorf("%w (external_id=%q)", ima.ErrTargetedRefetchUnsupported, "ima-hash"),
		},
	}
	h := newScopedReindexHarness(t, connector, nil)

	log, err := h.run(t, "gone", "ok", "transient", "ima-hash")
	require.NoError(t, err)

	assert.Equal(t, types.SyncLogStatusSuccess, log.Status,
		"per-item failures must not flip the scoped run's status")
	assert.Equal(t, 4, log.ItemsTotal)
	assert.Equal(t, 1, log.ItemsCreated)
	assert.Equal(t, 3, log.ItemsFailed)

	result := decodeSyncResult(t, log)
	require.Len(t, result.Errors, 3)
	byID := map[string]types.SyncItemError{}
	for _, e := range result.Errors {
		byID[e.ExternalID] = e
	}
	assert.Equal(t, "not_found", byID["gone"].Code)
	assert.Equal(t, "fetch_failed", byID["transient"].Code)
	assert.Contains(t, byID["transient"].Message, "rate limited",
		"fetch_failed must carry the raw cause text")
	assert.Equal(t, "targeted_unsupported", byID["ima-hash"].Code)
	assert.Contains(t, byID["ima-hash"].Message, "incremental",
		"the ima degradation must surface its remedy text")

	assert.Equal(t, []string{"gone", "ok", "transient", "ima-hash"}, connector.fetched)
	h.assertDSUntouched(t)
}

// TestProcessSync_ScopedReindexUnsupportedConnector covers a connector that
// has not adopted TargetedFetcher: every requested item records a
// targeted_unsupported failure with the "run a normal sync" hint, and the run
// never falls through to the batch fetch.
func TestProcessSync_ScopedReindexUnsupportedConnector(t *testing.T) {
	connector := &unsupportedTargetedConnector{}
	h := newScopedReindexHarness(t, connector, nil)

	log, err := h.run(t, "x", "y")
	require.NoError(t, err)

	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Equal(t, 2, log.ItemsTotal)
	assert.Equal(t, 2, log.ItemsFailed)

	result := decodeSyncResult(t, log)
	require.Len(t, result.Errors, 2)
	for _, e := range result.Errors {
		assert.Equal(t, "targeted_unsupported", e.Code)
		assert.Contains(t, e.Message, "normal sync", "the hint must tell the user the working remedy")
		assert.NotEmpty(t, e.ExternalID)
	}

	assert.Zero(t, connector.fetchAllCalls, "unsupported must degrade per-item, not via FetchAll")
	h.assertDSUntouched(t)
}

// TestProcessSync_ScopedReindexSubtreeKeepContract pins the SubtreeKeep
// contract through the scoped path (spec §5.3): refetching one docx node runs
// the same delete-before-create ingest as a whole sync, the sweep removes only
// the stale child, and the child still listed in SubtreeKeep keeps its
// previously-synced copy.
func TestProcessSync_ScopedReindexSubtreeKeepContract(t *testing.T) {
	connector := &scopedReindexConnector{items: map[string]*types.FetchedItem{
		"doc-parent": {
			ExternalID:       "doc-parent",
			Title:            "Parent Doc",
			Content:          []byte("rebuilt body"),
			FileName:         "parent.docx",
			ReplacesSubtree:  true,
			SubtreeKeep:      []string{"doc-parent#file#1"},
			SourceResourceID: "space-1",
		},
	}}
	repo := &scopedReindexRepo{
		sweepFakeRepo: sweepFakeRepo{prefixReturn: []*types.Knowledge{
			childWithExternalID("child-keep", "doc-parent#file#1", "ds-reindex"),
			childWithExternalID("child-stale", "doc-parent#file#2", "ds-reindex"),
		}},
		parent: &types.Knowledge{ID: "parent-old"},
	}
	h := newScopedReindexHarness(t, connector, repo)

	log, err := h.run(t, "doc-parent")
	require.NoError(t, err)

	assert.Equal(t, types.SyncLogStatusSuccess, log.Status)
	assert.Equal(t, 1, log.ItemsTotal)
	assert.Equal(t, 1, log.ItemsUpdated, "an existing parent is deleted and rebuilt, not re-created")

	// Delete-before-create for the parent, then the subtree sweep.
	assert.Equal(t,
		[]string{"delete:parent-old", "create:parent.docx", "delete:child-stale"},
		h.ks.events)
	assert.Contains(t, repo.hardDeleted, "parent-old")
	assert.Equal(t, []string{"child-stale"}, repo.hardDeletedL,
		"only the vanished child is hard-deleted; the kept child survives")
	h.assertDSUntouched(t)
}

// ── ReindexItems (enqueue side) ──────────────────────────────────────────

// reindexEnqueuer records every enqueued task plus its options, and can be
// armed to fail (e.g. with asynq.ErrTaskIDConflict).
type reindexEnqueuer struct {
	tasks []*asynq.Task
	opts  [][]asynq.Option
	err   error
}

func (e *reindexEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if e.err != nil {
		return nil, e.err
	}
	e.tasks = append(e.tasks, task)
	e.opts = append(e.opts, opts)
	return &asynq.TaskInfo{ID: "reindex-asynq-1", Payload: task.Payload()}, nil
}

// reindexSyncLogRepo assigns deterministic ids on Create (the uuid hook is a
// GORM-only concern) and records every created row.
type reindexSyncLogRepo struct {
	processSyncSyncLogRepo
	created []*types.SyncLog
}

func (r *reindexSyncLogRepo) Create(ctx context.Context, log *types.SyncLog) error {
	if log.ID == "" {
		log.ID = fmt.Sprintf("reindex-log-%d", len(r.created)+1)
	}
	r.created = append(r.created, log)
	return r.processSyncSyncLogRepo.Create(ctx, log)
}

func newReindexEnqueueFixture(t *testing.T, enq *reindexEnqueuer) (*scopedRecordingDSRepo, *reindexSyncLogRepo, *DataSourceService) {
	t.Helper()
	configJSON, err := (&types.DataSourceConfig{Type: scopedReindexConnectorType}).ToJSON()
	require.NoError(t, err)
	ds := &types.DataSource{
		ID:              "ds-reindex",
		TenantID:        7,
		KnowledgeBaseID: "kb-reindex",
		Type:            scopedReindexConnectorType,
		Config:          configJSON,
		Status:          types.DataSourceStatusActive,
		LastSyncResult:  types.JSON(scopedReindexLastSyncResult),
	}
	dsRepo := &scopedRecordingDSRepo{
		kbDeleteDSRepo: kbDeleteDSRepo{
			byKB:    map[string][]*types.DataSource{"kb-reindex": {ds}},
			deleted: map[string]bool{},
		},
	}
	logRepo := &reindexSyncLogRepo{}
	logRepo.logs = map[string]*types.SyncLog{}
	svc := &DataSourceService{dsRepo: dsRepo, syncLogRepo: logRepo, taskEnqueuer: enq}
	return dsRepo, logRepo, svc
}

// optionValue extracts the value of the first option of the given type, with
// presence reported separately so "no TaskID at all" is assertable.
func optionValue(opts []asynq.Option, want asynq.OptionType) (interface{}, bool) {
	for _, opt := range opts {
		if opt.Type() == want {
			return opt.Value(), true
		}
	}
	return nil, false
}

// TestReindexItems_BuildsScopedPayloadWithIdempotentTaskID verifies the
// enqueue contract: a manual_reindex-triggered scoped payload whose asynq
// TaskID embeds the caller's request_id, so a double click collides in Redis
// instead of queueing a second run.
func TestReindexItems_BuildsScopedPayloadWithIdempotentTaskID(t *testing.T) {
	enq := &reindexEnqueuer{}
	_, logRepo, svc := newReindexEnqueueFixture(t, enq)

	syncLogID, err := svc.ReindexItems(context.Background(), "ds-reindex",
		[]string{"doc-a", "doc-b"}, "req-42")
	require.NoError(t, err)
	require.NotEmpty(t, syncLogID)

	require.Len(t, enq.tasks, 1)
	var payload types.DataSourceSyncPayload
	require.NoError(t, json.Unmarshal(enq.tasks[0].Payload(), &payload))
	assert.Equal(t, "ds-reindex", payload.DataSourceID)
	assert.Equal(t, uint64(7), payload.TenantID)
	assert.Equal(t, syncLogID, payload.SyncLogID)
	assert.Equal(t, "manual_reindex", payload.Trigger)
	require.NotNil(t, payload.Scope)
	assert.Equal(t, []string{"doc-a", "doc-b"}, payload.Scope.ExternalIDs)
	assert.Equal(t, 2, payload.MaxItems, "the scoped item count is the run's item bound")

	queue, ok := optionValue(enq.opts[0], asynq.QueueOpt)
	require.True(t, ok)
	assert.Equal(t, types.QueueSync, queue)

	taskID, ok := optionValue(enq.opts[0], asynq.TaskIDOpt)
	require.True(t, ok, "a non-empty request_id must pin the asynq TaskID")
	assert.Equal(t, "dssync:7:ds-reindex:reindex:req-42", taskID)

	require.Len(t, logRepo.created, 1)
	assert.Equal(t, types.SyncLogStatusRunning, logRepo.created[0].Status)
	assert.Equal(t, syncLogID, logRepo.created[0].ID)
}

// TestReindexItems_EmptyRequestIDOmitsTaskID: without a request_id there is
// nothing to be idempotent about, so no TaskID option is attached and every
// call enqueues freely.
func TestReindexItems_EmptyRequestIDOmitsTaskID(t *testing.T) {
	enq := &reindexEnqueuer{}
	_, _, svc := newReindexEnqueueFixture(t, enq)

	_, err := svc.ReindexItems(context.Background(), "ds-reindex", []string{"a"}, "")
	require.NoError(t, err)

	_, ok := optionValue(enq.opts[0], asynq.TaskIDOpt)
	assert.False(t, ok, "an empty request_id must not pin the asynq TaskID")
}

// TestReindexItems_DuplicateRequestIDIRejectedIdempotently: when the same
// request_id is still enqueued, the second call hits ErrTaskIDConflict and
// must surface ErrReindexDuplicateRequest, cancelling the log row it had just
// optimistically created — without touching ds state.
func TestReindexItems_DuplicateRequestIDIRejectedIdempotently(t *testing.T) {
	enq := &reindexEnqueuer{}
	dsRepo, logRepo, svc := newReindexEnqueueFixture(t, enq)

	firstID, err := svc.ReindexItems(context.Background(), "ds-reindex", []string{"a"}, "req-42")
	require.NoError(t, err)

	enq.err = asynq.ErrTaskIDConflict
	secondID, err := svc.ReindexItems(context.Background(), "ds-reindex", []string{"a"}, "req-42")
	require.ErrorIs(t, err, ErrReindexDuplicateRequest)
	assert.Empty(t, secondID)

	require.Len(t, logRepo.created, 2, "both calls create their log row first")
	assert.Equal(t, firstID, logRepo.created[0].ID)
	second := logRepo.created[1]
	assert.Equal(t, types.SyncLogStatusCanceled, second.Status,
		"the losing request's log row must reach a terminal state")
	require.NotNil(t, second.FinishedAt)
	assert.NotEmpty(t, second.ErrorMessage)

	assert.Zero(t, dsRepo.syncStateWrites)
	assert.Zero(t, dsRepo.rowWrites)
}

// TestReindexItems_EnqueueFailureIsolatedToLogRow: a genuine enqueue outage
// fails only the scoped run's own log row — unlike ManualSync it must not flip
// ds.Status to error, per the scoped double-write convergence ruling.
func TestReindexItems_EnqueueFailureIsolatedToLogRow(t *testing.T) {
	enq := &reindexEnqueuer{err: errors.New("redis down")}
	dsRepo, logRepo, svc := newReindexEnqueueFixture(t, enq)

	_, err := svc.ReindexItems(context.Background(), "ds-reindex", []string{"a"}, "")
	require.Error(t, err)

	require.Len(t, logRepo.created, 1)
	failed := logRepo.created[0]
	assert.Equal(t, types.SyncLogStatusFailed, failed.Status)
	require.NotNil(t, failed.FinishedAt)
	assert.NotEmpty(t, failed.ErrorMessage)

	assert.Zero(t, dsRepo.syncStateWrites, "enqueue failure must not write ds sync state")
	assert.Zero(t, dsRepo.rowWrites, "enqueue failure must not write the ds row")
}
