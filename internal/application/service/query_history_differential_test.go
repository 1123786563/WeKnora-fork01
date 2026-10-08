package service

// Application/worker-layer old-vs-new differential gate (Wave 1, Task 9,
// brief Step 5). The LEGACY export service (this package, real code) and the
// NEW module application (queryhistory/application, real code) run the same
// scenario against separate but identically-behaving fake stores; the
// resulting observations — created job rows, enqueued tasks (type, queue,
// max-retry, timeout, payload bytes), stored CSV bytes, and the normalized
// error class of the returned error — must be identical (testkit.Compare).
//
// The enqueue contract is observed end to end: both stacks enqueue through
// the same kind of recording enqueuer backed by a REAL asynq client over an
// isolated miniredis instance, so the queue/max-retry/timeout options that
// ride the task are read from the task the framework actually accepted, not
// from either stack's source code.
//
// Scenario list (brief Step 5, verbatim):
//
//	created pending job
//	enqueued task type/queue/max-retry/timeout/payload
//	enqueue failure transition
//	normal and anonymized CSV bytes
//	disabled-after-enqueue failure with no file write
//	storage failure and 1,024-byte stored error truncation
//	done-job idempotent rerun
//	missing job drop
//	malformed and tenantless payload permanent failure

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/application/repository"
	qhadapters "github.com/Tencent/WeKnora/internal/conversation/queryhistory/adapters"
	qhapplication "github.com/Tencent/WeKnora/internal/conversation/queryhistory/application"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/testkit"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// diffWorkerClock is the fixture clock: every timestamp that reaches an
// observation (row times rendered into the CSV) comes from it.
var diffWorkerClock = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

// diffExportRows serves BOTH stacks (types.QueryHistoryExportRow is the
// Wave 1 alias of domain.ExportRow): two sessions with tallies, fixed times.
func diffExportRows() []types.QueryHistoryExportRow {
	return []types.QueryHistoryExportRow{
		{
			SessionID: "diff-a", Title: "Alpha", UserID: "u1", Source: "web",
			EngineType:   "builtin",
			CreatedAt:    diffWorkerClock,
			UpdatedAt:    diffWorkerClock.Add(5 * time.Minute),
			MessageCount: 2, LikeCount: 1, DislikeCount: 0,
		},
		{
			SessionID: "diff-b", Title: "Beta", UserID: "u2", Source: "api",
			EngineType:   "builtin",
			CreatedAt:    diffWorkerClock.Add(22*time.Hour + 30*time.Minute),
			UpdatedAt:    diffWorkerClock.Add(23 * time.Hour),
			MessageCount: 1, LikeCount: 0, DislikeCount: 1,
		},
	}
}

// diffTenantRow builds the tenant fixture ("" leaves the policy unset).
func diffTenantRow(mode string) *types.Tenant {
	tenant := &types.Tenant{ID: 1}
	if mode != "" {
		tenant.QueryHistoryConfig = &types.QueryHistoryConfig{Mode: mode}
	}
	return tenant
}

// diffRedisEnqueuer records what each stack enqueues through a REAL asynq
// client over its own isolated miniredis. The returned TaskInfo carries the
// merged task options (queue, max-retry, timeout) the framework applied, so
// the observation is the true enqueue contract. err simulates a broker
// outage without touching redis.
type diffRedisEnqueuer struct {
	client *asynq.Client
	err    error
	infos  []asynq.TaskInfo
}

func newDiffRedisEnqueuer(t *testing.T) *diffRedisEnqueuer {
	t.Helper()
	mini := miniredis.RunT(t)
	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mini.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return &diffRedisEnqueuer{client: client}
}

// compile-time enqueuer conformance.
var _ interfaces.TaskEnqueuer = (*diffRedisEnqueuer)(nil)

func (e *diffRedisEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if e.err != nil {
		return nil, e.err
	}
	info, err := e.client.Enqueue(task, opts...)
	if err != nil {
		return nil, err
	}
	e.infos = append(e.infos, *info)
	return info, nil
}

// taskObservations projects the recorded tasks. The broker-assigned task id
// is deliberately not recorded.
func (e *diffRedisEnqueuer) taskObservations() []testkit.TaskObservation {
	out := make([]testkit.TaskObservation, 0, len(e.infos))
	for _, info := range e.infos {
		out = append(out, testkit.TaskObservation{
			Type:     info.Type,
			Queue:    info.Queue,
			MaxRetry: info.MaxRetry,
			Timeout:  info.Timeout,
			Payload:  append([]byte(nil), info.Payload...),
		})
	}
	return out
}

// diffLegacyJobRepo is the LEGACY stack's store: an in-memory
// interfaces.QueryHistoryExportJobRepository whose observable behavior
// matches the real GORM repository for the flows under test (sequential ids,
// pending default, tenant-scoped reads answering the shared not-found
// sentinel, both transition columns rewritten).
type diffLegacyJobRepo struct {
	jobs   map[uint64]*types.QueryHistoryExportJob
	nextID uint64
	rows   []types.QueryHistoryExportRow
}

func (r *diffLegacyJobRepo) Create(_ context.Context, job *types.QueryHistoryExportJob) error {
	r.nextID++
	job.ID = r.nextID
	if job.Status == "" {
		job.Status = types.QueryHistoryExportPending
	}
	stored := *job
	r.jobs[job.ID] = &stored
	return nil
}

func (r *diffLegacyJobRepo) GetByID(
	_ context.Context, tenantID uint64, id uint64,
) (*types.QueryHistoryExportJob, error) {
	job, ok := r.jobs[id]
	if !ok || job.TenantID != tenantID {
		return nil, repository.ErrQueryHistoryExportJobNotFound
	}
	stored := *job
	return &stored, nil
}

func (r *diffLegacyJobRepo) UpdateStatus(
	_ context.Context, id uint64, status, filePath, errMsg string,
) error {
	if job, ok := r.jobs[id]; ok {
		job.Status, job.FilePath, job.ErrorMessage = status, filePath, errMsg
	}
	return nil
}

func (r *diffLegacyJobRepo) ExportSessionRows(
	context.Context, *types.SessionListQuery,
) ([]types.QueryHistoryExportRow, error) {
	return append([]types.QueryHistoryExportRow(nil), r.rows...), nil
}

// diffNewJobStore is the NEW stack's store: the same behavior through
// ports.ExportJobStore (tenant-scoped transitions, per the port contract).
type diffNewJobStore struct {
	jobs   map[uint64]*domain.ExportJob
	nextID uint64
}

func (s *diffNewJobStore) Create(_ context.Context, job *domain.ExportJob) error {
	s.nextID++
	job.ID = s.nextID
	if job.Status == "" {
		job.Status = domain.ExportPending
	}
	stored := *job
	s.jobs[job.ID] = &stored
	return nil
}

func (s *diffNewJobStore) Get(
	_ context.Context, tenantID uint64, jobID uint64,
) (*domain.ExportJob, error) {
	job, ok := s.jobs[jobID]
	if !ok || job.TenantID != tenantID {
		return nil, repository.ErrQueryHistoryExportJobNotFound
	}
	stored := *job
	return &stored, nil
}

func (s *diffNewJobStore) UpdateStatus(
	_ context.Context, tenantID uint64, jobID uint64,
	status domain.ExportStatus, filePath string, errMsg string,
) error {
	if job, ok := s.jobs[jobID]; ok && job.TenantID == tenantID {
		job.Status, job.FilePath, job.ErrorMessage = status, filePath, errMsg
	}
	return nil
}

// diffNewAuditReader serves the module's ExportRows port from the same
// fixture rows the legacy fake serves.
type diffNewAuditReader struct{ rows []domain.ExportRow }

func (a *diffNewAuditReader) Snapshot(context.Context, uint64, string) (*types.QueryHistorySnapshot, error) {
	return nil, errors.New("snapshot is not part of the worker differential")
}

func (a *diffNewAuditReader) ExportRows(
	context.Context, uint64, domain.ExportFilter,
) ([]domain.ExportRow, error) {
	return append([]domain.ExportRow(nil), a.rows...), nil
}

// diffNewFileStore is the NEW stack's file store: mirrors the legacy
// stubExportFileService behavior (same path scheme, same capture).
type diffNewFileStore struct {
	err       error
	saveCalls int
	savedName string
	savedData []byte
}

func (f *diffNewFileStore) SaveBytes(
	_ context.Context, data []byte, _ uint64, fileName string, _ bool,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.saveCalls++
	f.savedName = fileName
	f.savedData = append([]byte(nil), data...)
	return "local://temp/" + fileName, nil
}

func (f *diffNewFileStore) Open(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("open is not part of the worker differential")
}

// diffWorkerEnv runs both stacks side by side over separate but identical
// fakes. Both policy lookups run the real adapter/helper over their own
// tenant-repo instance with the same fixture row.
type diffWorkerEnv struct {
	legacy      *QueryHistoryExportService
	modern      *qhapplication.ExportService
	legacyJobs  *diffLegacyJobRepo
	newJobs     *diffNewJobStore
	legacyFiles *stubExportFileService
	newFiles    *diffNewFileStore
	legacyEnq   *diffRedisEnqueuer
	newEnq      *diffRedisEnqueuer
}

func newDiffWorkerEnv(t *testing.T, mode string) *diffWorkerEnv {
	t.Helper()
	rows := diffExportRows()
	env := &diffWorkerEnv{
		legacyJobs:  &diffLegacyJobRepo{jobs: map[uint64]*types.QueryHistoryExportJob{}, rows: rows},
		newJobs:     &diffNewJobStore{jobs: map[uint64]*domain.ExportJob{}},
		legacyFiles: &stubExportFileService{},
		newFiles:    &diffNewFileStore{},
		legacyEnq:   newDiffRedisEnqueuer(t),
		newEnq:      newDiffRedisEnqueuer(t),
	}
	env.legacy = NewQueryHistoryExportService(
		env.legacyJobs,
		&stubTenantRepoForHistory{tenant: diffTenantRow(mode)},
		env.legacyFiles,
		env.legacyEnq,
	)
	env.modern = qhapplication.NewExportService(
		qhadapters.NewTenantPolicy(&stubTenantRepoForHistory{tenant: diffTenantRow(mode)}),
		&diffNewAuditReader{rows: rows},
		env.newJobs,
		env.newFiles,
		qhadapters.NewAsynqTaskQueue(env.newEnq),
	)
	return env
}

// legacyObservation captures the legacy stack's state after a scenario.
func (e *diffWorkerEnv) legacyObservation(err error) testkit.Observation {
	obs := testkit.Observation{
		ErrorClass: testkit.NormalizeError(err),
		Enqueued:   e.legacyEnq.taskObservations(),
	}
	for _, job := range e.legacyJobs.jobs {
		obs.Jobs = append(obs.Jobs, testkit.JobObservation{
			ID: job.ID, TenantID: job.TenantID, RequestedBy: job.RequestedBy,
			Status: job.Status, FilePath: job.FilePath, ErrorMessage: job.ErrorMessage,
		})
	}
	if e.legacyFiles.saveCalls > 0 {
		obs.StoredFiles = map[string][]byte{
			"local://temp/" + e.legacyFiles.savedName: e.legacyFiles.savedData,
		}
	}
	return obs
}

// modernObservation captures the new stack's state after a scenario.
func (e *diffWorkerEnv) modernObservation(err error) testkit.Observation {
	obs := testkit.Observation{
		ErrorClass: testkit.NormalizeError(err),
		Enqueued:   e.newEnq.taskObservations(),
	}
	for _, job := range e.newJobs.jobs {
		obs.Jobs = append(obs.Jobs, testkit.JobObservation{
			ID: job.ID, TenantID: job.TenantID, RequestedBy: job.RequestedBy,
			Status: job.Status, FilePath: job.FilePath, ErrorMessage: job.ErrorMessage,
		})
	}
	if e.newFiles.saveCalls > 0 {
		obs.StoredFiles = map[string][]byte{
			"local://temp/" + e.newFiles.savedName: e.newFiles.savedData,
		}
	}
	return obs
}

// diffCheck fails the scenario on any divergence.
func diffWorkerCheck(t *testing.T, scenario string, legacy, modern testkit.Observation) {
	t.Helper()
	if err := testkit.Compare(legacy, modern); err != nil {
		t.Fatalf("differential %s: legacy/new divergence: %v", scenario, err)
	}
}

// runWorkerPayload drives both worker bodies with the SAME payload bytes.
func (e *diffWorkerEnv) runWorkerPayload(ctx context.Context, payload []byte) (error, error) {
	legacyErr := e.legacy.ProcessExport(ctx, asynq.NewTask(types.TypeQueryHistoryExport, payload))
	modernErr := e.modern.Process(ctx, payload)
	return legacyErr, modernErr
}

// diffPayloadFor marshals the shared payload of one job run.
func diffPayloadFor(t *testing.T, jobID uint64) []byte {
	t.Helper()
	payload, err := json.Marshal(&types.QueryHistoryExportPayload{
		JobID: jobID, TenantID: 1, RequestedBy: "admin-1", UserID: "u1",
	})
	require.NoError(t, err)
	return payload
}

// TestQueryHistoryDifferentialStart pins the submission surface: the created
// pending job row and the enqueued task's type/queue/max-retry/timeout/
// payload, plus the enqueue-failure transition and the zero-tenant guard.
func TestQueryHistoryDifferentialStart(t *testing.T) {
	ctx := context.Background()

	t.Run("created pending job and enqueued task contract", func(t *testing.T) {
		env := newDiffWorkerEnv(t, types.QueryHistoryModeNormal)
		start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

		legacyJobID, legacyErr := env.legacy.StartExport(ctx, 1, "admin-1", types.SessionListQuery{
			UserID: "u1", StartTime: start, FeedbackRating: types.FeedbackRatingLike,
			// Noise the export must strip: the export always spans the audit view.
			Source: "web", Keyword: "k", AgentID: "a1", Page: 3, PageSize: 10,
		})
		modernJobID, modernErr := env.modern.Start(ctx, 1, "admin-1", domain.ExportFilter{
			UserID: "u1", StartTime: start, FeedbackRating: types.FeedbackRatingLike,
		})

		require.NoError(t, legacyErr)
		require.NoError(t, modernErr)
		require.Equal(t, legacyJobID, modernJobID, "both stores admit the first job as id 1")

		diffWorkerCheck(t, "start creates pending job and enqueues",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))

		// The task contract details the observation comparison already
		// pinned; these assertions spell them out for the reader.
		require.Len(t, env.legacyEnq.infos, 1)
		require.Equal(t, types.TypeQueryHistoryExport, env.legacyEnq.infos[0].Type)
		require.Equal(t, types.QueueMaintenance, env.legacyEnq.infos[0].Queue)
		require.Equal(t, 3, env.legacyEnq.infos[0].MaxRetry)
		require.Equal(t, 10*time.Minute, env.legacyEnq.infos[0].Timeout)
	})

	t.Run("enqueue failure marks the job failed", func(t *testing.T) {
		env := newDiffWorkerEnv(t, "")
		env.legacyEnq.err = errors.New("redis unavailable")
		env.newEnq.err = errors.New("redis unavailable")

		_, legacyErr := env.legacy.StartExport(ctx, 1, "admin-1", types.SessionListQuery{})
		_, modernErr := env.modern.Start(ctx, 1, "admin-1", domain.ExportFilter{})

		require.Error(t, legacyErr)
		require.Error(t, modernErr)
		diffWorkerCheck(t, "enqueue failure transition",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
		require.Len(t, env.legacyJobs.jobs, 1)
		require.Equal(t, types.QueryHistoryExportFailed,
			env.legacyJobs.jobs[1].Status, "legacy job is failed with the reason")
		require.Contains(t, env.legacyJobs.jobs[1].ErrorMessage, "redis unavailable")
	})

	t.Run("zero tenant is rejected before any store or queue touch", func(t *testing.T) {
		env := newDiffWorkerEnv(t, "")
		_, legacyErr := env.legacy.StartExport(ctx, 0, "admin-1", types.SessionListQuery{})
		_, modernErr := env.modern.Start(ctx, 0, "admin-1", domain.ExportFilter{})

		require.EqualError(t, legacyErr, "workspace id is required")
		require.EqualError(t, modernErr, "workspace id is required")
		diffWorkerCheck(t, "zero tenant", env.legacyObservation(legacyErr), env.modernObservation(modernErr))
	})
}

// TestQueryHistoryDifferentialWorkerCSV pins the worker body: identical CSV
// bytes in normal and anonymized modes, the done transition with the recorded
// file path, and the idempotent rerun of a done job.
func TestQueryHistoryDifferentialWorkerCSV(t *testing.T) {
	ctx := context.Background()

	runToDone := func(t *testing.T, mode string) *diffWorkerEnv {
		t.Helper()
		env := newDiffWorkerEnv(t, mode)
		legacyJob := &types.QueryHistoryExportJob{TenantID: 1, RequestedBy: "admin-1"}
		require.NoError(t, env.legacyJobs.Create(ctx, legacyJob))
		modernJob := &domain.ExportJob{TenantID: 1, RequestedBy: "admin-1"}
		require.NoError(t, env.newJobs.Create(ctx, modernJob))
		require.Equal(t, legacyJob.ID, modernJob.ID)

		payload := diffPayloadFor(t, legacyJob.ID)
		legacyErr, modernErr := env.runWorkerPayload(ctx, payload)
		require.NoError(t, legacyErr)
		require.NoError(t, modernErr)

		diffWorkerCheck(t, "worker csv "+mode,
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
		return env
	}

	t.Run("normal mode keeps owner identities in the CSV bytes", func(t *testing.T) {
		env := runToDone(t, types.QueryHistoryModeNormal)
		require.Equal(t, 1, env.legacyFiles.saveCalls)
		require.Equal(t, 1, env.newFiles.saveCalls)
		require.Equal(t, env.legacyFiles.savedName, env.newFiles.savedName)
		require.Contains(t, string(env.legacyFiles.savedData), "u1",
			"normal mode keeps the owner id in the CSV")
	})

	t.Run("anonymized mode masks owner identities in the CSV bytes", func(t *testing.T) {
		env := runToDone(t, types.QueryHistoryModeAnonymized)
		require.Contains(t, string(env.legacyFiles.savedData), "anonymous")
		require.NotContains(t, string(env.legacyFiles.savedData), "u1",
			"anonymized mode must not leak the owner id")
		require.Equal(t, env.legacyFiles.savedData, env.newFiles.savedData,
			"the stored CSV bytes are identical")
	})

	t.Run("done job rerun is idempotent", func(t *testing.T) {
		env := runToDone(t, types.QueryHistoryModeNormal)
		doneLegacy := env.legacyJobs.jobs[1].Status
		require.Equal(t, types.QueryHistoryExportDone, doneLegacy)

		payload := diffPayloadFor(t, 1)
		legacyErr, modernErr := env.runWorkerPayload(ctx, payload)
		require.NoError(t, legacyErr)
		require.NoError(t, modernErr)
		diffWorkerCheck(t, "done rerun",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))

		require.Equal(t, 1, env.legacyFiles.saveCalls, "legacy rerun must not rebuild the file")
		require.Equal(t, 1, env.newFiles.saveCalls, "new rerun must not rebuild the file")
		require.Equal(t, types.QueryHistoryExportDone, env.legacyJobs.jobs[1].Status)
		require.Equal(t, domain.ExportDone, env.newJobs.jobs[1].Status)
	})
}

// TestQueryHistoryDifferentialWorkerFailures pins the failure paths: the
// disabled-after-enqueue refusal (no file write), the storage failure with
// the 1,024-byte stored-error truncation, the missing-job drop, and the
// malformed / tenantless permanent-payload failures.
func TestQueryHistoryDifferentialWorkerFailures(t *testing.T) {
	ctx := context.Background()

	seedJob := func(t *testing.T, env *diffWorkerEnv) uint64 {
		t.Helper()
		legacyJob := &types.QueryHistoryExportJob{TenantID: 1, RequestedBy: "admin-1"}
		require.NoError(t, env.legacyJobs.Create(ctx, legacyJob))
		modernJob := &domain.ExportJob{TenantID: 1, RequestedBy: "admin-1"}
		require.NoError(t, env.newJobs.Create(ctx, modernJob))
		require.Equal(t, legacyJob.ID, modernJob.ID)
		return legacyJob.ID
	}

	t.Run("disabled after enqueue fails the job and writes no file", func(t *testing.T) {
		env := newDiffWorkerEnv(t, types.QueryHistoryModeDisabled)
		jobID := seedJob(t, env)

		legacyErr, modernErr := env.runWorkerPayload(ctx, diffPayloadFor(t, jobID))
		require.Error(t, legacyErr)
		require.Error(t, modernErr)
		require.Contains(t, legacyErr.Error(), "query history is disabled for this tenant")

		diffWorkerCheck(t, "disabled after enqueue",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
		require.Equal(t, 0, env.legacyFiles.saveCalls, "no legacy file write")
		require.Equal(t, 0, env.newFiles.saveCalls, "no new file write")
		require.Equal(t, types.QueryHistoryExportFailed, env.legacyJobs.jobs[jobID].Status)
		require.Equal(t, domain.ExportFailed, env.newJobs.jobs[jobID].Status)
	})

	t.Run("storage failure truncates the stored error at 1024 bytes", func(t *testing.T) {
		env := newDiffWorkerEnv(t, types.QueryHistoryModeNormal)
		jobID := seedJob(t, env)
		boom := fmt.Errorf("disk blown: %s", strings.Repeat("x", 2000))
		env.legacyFiles.err = boom
		env.newFiles.err = boom

		legacyErr, modernErr := env.runWorkerPayload(ctx, diffPayloadFor(t, jobID))
		require.Error(t, legacyErr)
		require.Error(t, modernErr)

		diffWorkerCheck(t, "storage failure truncation",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
		require.Len(t, env.legacyJobs.jobs[jobID].ErrorMessage, 1024,
			"legacy stored error is exactly the 1024-byte cap")
		require.Len(t, env.newJobs.jobs[jobID].ErrorMessage, 1024,
			"new stored error is exactly the 1024-byte cap")
		require.Equal(t, env.legacyJobs.jobs[jobID].ErrorMessage, env.newJobs.jobs[jobID].ErrorMessage)
	})

	t.Run("missing job drops the task without error", func(t *testing.T) {
		env := newDiffWorkerEnv(t, types.QueryHistoryModeNormal)
		payload, err := json.Marshal(&types.QueryHistoryExportPayload{JobID: 424242, TenantID: 1})
		require.NoError(t, err)

		legacyErr, modernErr := env.runWorkerPayload(ctx, payload)
		require.NoError(t, legacyErr)
		require.NoError(t, modernErr)
		diffWorkerCheck(t, "missing job drop",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
		require.Equal(t, 0, env.legacyFiles.saveCalls)
		require.Equal(t, 0, env.newFiles.saveCalls)
	})

	t.Run("malformed payload is a permanent failure", func(t *testing.T) {
		env := newDiffWorkerEnv(t, types.QueryHistoryModeNormal)
		legacyErr, modernErr := env.runWorkerPayload(ctx, []byte("{not json"))
		require.Error(t, legacyErr)
		require.Error(t, modernErr)
		require.ErrorIs(t, legacyErr, asynq.SkipRetry)
		require.ErrorIs(t, modernErr, domain.ErrPermanentPayload)
		require.Equal(t, testkit.ClassPermanent, testkit.NormalizeError(legacyErr))
		require.Equal(t, testkit.ClassPermanent, testkit.NormalizeError(modernErr))

		diffWorkerCheck(t, "malformed payload",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
		require.Empty(t, env.legacyJobs.jobs, "no job row may be touched")
		require.Empty(t, env.newJobs.jobs)
	})

	t.Run("tenantless payload is a permanent failure", func(t *testing.T) {
		env := newDiffWorkerEnv(t, types.QueryHistoryModeNormal)
		payload, err := json.Marshal(&types.QueryHistoryExportPayload{JobID: 5})
		require.NoError(t, err)

		legacyErr, modernErr := env.runWorkerPayload(ctx, payload)
		require.Error(t, legacyErr)
		require.Error(t, modernErr)
		require.Equal(t, testkit.ClassPermanent, testkit.NormalizeError(legacyErr))
		require.Equal(t, testkit.ClassPermanent, testkit.NormalizeError(modernErr))
		diffWorkerCheck(t, "tenantless payload",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
	})

	t.Run("jobless payload is a permanent failure", func(t *testing.T) {
		env := newDiffWorkerEnv(t, types.QueryHistoryModeNormal)
		payload, err := json.Marshal(&types.QueryHistoryExportPayload{TenantID: 1})
		require.NoError(t, err)

		legacyErr, modernErr := env.runWorkerPayload(ctx, payload)
		require.Error(t, legacyErr)
		require.Error(t, modernErr)
		require.Equal(t, testkit.ClassPermanent, testkit.NormalizeError(legacyErr))
		require.Equal(t, testkit.ClassPermanent, testkit.NormalizeError(modernErr))
		diffWorkerCheck(t, "jobless payload",
			env.legacyObservation(legacyErr), env.modernObservation(modernErr))
	})
}
