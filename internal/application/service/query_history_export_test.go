package service

// Tests for the async query-history export service (SP13 Task 4). The job
// repository runs against a real migrated SQLite database so ExportSessionRows,
// the job lifecycle transitions, and the CSV pipeline are exercised end to
// end; only the task enqueue and the file storage are stubs.

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// recordingEnqueuer captures the task StartExport would dispatch.
type recordingEnqueuer struct {
	task *asynq.Task
	opts []asynq.Option
	err  error
}

func (e *recordingEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if e.err != nil {
		return nil, e.err
	}
	e.task = task
	e.opts = opts
	return &asynq.TaskInfo{ID: "t1", Queue: "low", Type: task.Type()}, nil
}

// stubExportFileService captures the stored CSV bytes without touching a
// storage backend.
type stubExportFileService struct {
	interfaces.FileService
	saveCalls int
	savedName string
	savedData []byte
	err       error
}

func (f *stubExportFileService) SaveBytes(
	_ context.Context, data []byte, _ uint64, fileName string, _ bool,
) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	f.saveCalls++
	f.savedName = fileName
	f.savedData = data
	return "local://temp/" + fileName, nil
}

// newExportServiceForTest wires the export service over a real migrated
// SQLite DB. tenantMode seeds the tenant's query-history policy ("" = unset).
func newExportServiceForTest(
	t *testing.T, tenantMode string,
) (*QueryHistoryExportService, *recordingEnqueuer, *stubExportFileService, interfaces.QueryHistoryExportJobRepository, *gorm.DB) {
	t.Helper()
	db := openDurableRunTestDB(t)
	jobRepo := repository.NewQueryHistoryExportJobRepository(db)
	enqueuer := &recordingEnqueuer{}
	files := &stubExportFileService{}

	tenantRow := &types.Tenant{ID: 1}
	if tenantMode != "" {
		tenantRow.QueryHistoryConfig = &types.QueryHistoryConfig{Mode: tenantMode}
	}
	svc := NewQueryHistoryExportService(
		jobRepo,
		&stubTenantRepoForHistory{tenant: tenantRow},
		files,
		enqueuer,
	)
	return svc, enqueuer, files, jobRepo, db
}

// seedExportFixtures inserts two web sessions: exp-a (owner u1, 2 messages,
// 1 like) and exp-b (owner u2, 1 message, 1 dislike). The DB helper's own
// seeded session is removed so exports see exactly these rows.
func seedExportFixtures(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(`DELETE FROM sessions WHERE id = 's1'`).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO sessions (id, tenant_id, title, user_id, engine_type) VALUES
		 ('exp-a', 1, 'Alpha', 'u1', 'builtin'),
		 ('exp-b', 1, 'Beta', 'u2', 'builtin')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO messages (id, request_id, session_id, role, content) VALUES
		 ('em-1', 'r1', 'exp-a', 'user', 'q'),
		 ('em-2', 'r2', 'exp-a', 'assistant', 'a'),
		 ('em-3', 'r3', 'exp-b', 'user', 'q')`,
	).Error)
	require.NoError(t, db.Exec(
		`INSERT INTO message_feedback (id, tenant_id, user_id, message_id, session_id, rating) VALUES
		 (1, 1, 'u1', 'em-1', 'exp-a', 'like'),
		 (2, 1, 'u2', 'em-3', 'exp-b', 'dislike')`,
	).Error)
}

func exportTaskFor(t *testing.T, jobID uint64) *asynq.Task {
	t.Helper()
	payload, err := json.Marshal(&types.QueryHistoryExportPayload{JobID: jobID, TenantID: 1})
	require.NoError(t, err)
	return asynq.NewTask(types.TypeQueryHistoryExport, payload)
}

func TestStartExportCreatesPendingJobAndEnqueues(t *testing.T) {
	svc, enqueuer, _, jobRepo, _ := newExportServiceForTest(t, types.QueryHistoryModeNormal)
	ctx := context.Background()
	start := parseExportTestTime(t, "2026-09-01T00:00:00Z")

	jobID, err := svc.StartExport(ctx, 1, "admin-1", types.SessionListQuery{
		UserID:         "u1",
		StartTime:      start,
		FeedbackRating: types.FeedbackRatingLike,
		// Noise the export must strip: it always spans the audit view.
		Source:   "web",
		Keyword:  "k",
		AgentID:  "a1",
		Page:     3,
		PageSize: 10,
	})
	require.NoError(t, err)
	require.NotZero(t, jobID)

	job, err := jobRepo.GetByID(ctx, 1, jobID)
	require.NoError(t, err)
	require.Equal(t, types.QueryHistoryExportPending, job.Status)
	require.Equal(t, "admin-1", job.RequestedBy)

	require.NotNil(t, enqueuer.task)
	require.Equal(t, types.TypeQueryHistoryExport, enqueuer.task.Type())

	var payload types.QueryHistoryExportPayload
	require.NoError(t, json.Unmarshal(enqueuer.task.Payload(), &payload))
	require.Equal(t, jobID, payload.JobID)
	require.Equal(t, uint64(1), payload.TenantID)
	require.Equal(t, "admin-1", payload.RequestedBy)
	require.Equal(t, "u1", payload.UserID)
	require.Equal(t, start.UnixMilli(), payload.StartTimeMs)
	require.Equal(t, int64(0), payload.EndTimeMs, "zero end time must stay open")
	require.Equal(t, types.FeedbackRatingLike, payload.FeedbackRating)
}

func parseExportTestTime(t *testing.T, raw string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, raw)
	require.NoError(t, err)
	return parsed
}

func TestStartExportEnqueueFailureMarksJobFailed(t *testing.T) {
	svc, enqueuer, _, _, db := newExportServiceForTest(t, "")
	enqueuer.err = errors.New("redis unavailable")

	jobID, err := svc.StartExport(context.Background(), 1, "admin-1", types.SessionListQuery{})
	require.Error(t, err)
	require.Zero(t, jobID, "the caller must not get a job id that will never run")

	var jobs []types.QueryHistoryExportJob
	require.NoError(t, db.Find(&jobs).Error)
	require.Len(t, jobs, 1, "exactly one job row is created even when enqueueing fails")
	require.Equal(t, types.QueryHistoryExportFailed, jobs[0].Status)
	require.Contains(t, jobs[0].ErrorMessage, "redis unavailable")
}

func TestStartExportRequiresTenant(t *testing.T) {
	svc, _, _, _, _ := newExportServiceForTest(t, "")
	_, err := svc.StartExport(context.Background(), 0, "admin-1", types.SessionListQuery{})
	require.Error(t, err)
}

func TestProcessExportFullChainDone(t *testing.T) {
	svc, _, files, jobRepo, db := newExportServiceForTest(t, types.QueryHistoryModeNormal)
	ctx := context.Background()
	seedExportFixtures(t, db)

	job := &types.QueryHistoryExportJob{TenantID: 1, RequestedBy: "admin-1"}
	require.NoError(t, jobRepo.Create(ctx, job))

	require.NoError(t, svc.ProcessExport(ctx, exportTaskFor(t, job.ID)))

	got, err := jobRepo.GetByID(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, types.QueryHistoryExportDone, got.Status)
	require.Equal(t, "local://temp/query_history_export_"+strconv.FormatUint(job.ID, 10)+".csv", got.FilePath)
	require.Empty(t, got.ErrorMessage)
	require.Equal(t, 1, files.saveCalls)
	require.Equal(t, "query_history_export_"+strconv.FormatUint(job.ID, 10)+".csv", files.savedName)

	// The CSV: header + one row per session, tallies included, real owner id.
	reader := csv.NewReader(strings.NewReader(string(files.savedData)))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 3)
	require.Equal(t, []string{
		"session_id", "title", "user_id", "source", "engine_type",
		"created_at", "updated_at", "message_count", "like_count", "dislike_count",
	}, records[0])
	require.Equal(t, "exp-a", records[1][0])
	require.Equal(t, "u1", records[1][2], "normal mode keeps the owner id")
	require.Equal(t, "2", records[1][7])
	require.Equal(t, "1", records[1][8])
	require.Equal(t, "0", records[1][9])
	require.Equal(t, "exp-b", records[2][0])
	require.Equal(t, "u2", records[2][2])
	require.Equal(t, "1", records[2][7])
	require.Equal(t, "0", records[2][8])
	require.Equal(t, "1", records[2][9])
}

func TestProcessExportAnonymizedMasksUserID(t *testing.T) {
	svc, _, files, jobRepo, db := newExportServiceForTest(t, types.QueryHistoryModeAnonymized)
	ctx := context.Background()
	seedExportFixtures(t, db)

	job := &types.QueryHistoryExportJob{TenantID: 1}
	require.NoError(t, jobRepo.Create(ctx, job))
	require.NoError(t, svc.ProcessExport(ctx, exportTaskFor(t, job.ID)))

	reader := csv.NewReader(strings.NewReader(string(files.savedData)))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.Equal(t, "anonymous", records[1][2], "anonymized mode masks the owner column")
	require.Equal(t, "anonymous", records[2][2])
}

func TestProcessExportFailureMarksFailedAndReturnsError(t *testing.T) {
	svc, _, files, jobRepo, db := newExportServiceForTest(t, types.QueryHistoryModeNormal)
	files.err = io.ErrClosedPipe
	ctx := context.Background()
	seedExportFixtures(t, db)

	job := &types.QueryHistoryExportJob{TenantID: 1}
	require.NoError(t, jobRepo.Create(ctx, job))

	err := svc.ProcessExport(ctx, exportTaskFor(t, job.ID))
	require.Error(t, err)

	got, err := jobRepo.GetByID(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, types.QueryHistoryExportFailed, got.Status)
	require.Empty(t, got.FilePath)
	require.Contains(t, got.ErrorMessage, "store export file")
}

func TestProcessExportIdempotentDoneRerun(t *testing.T) {
	svc, _, files, jobRepo, db := newExportServiceForTest(t, types.QueryHistoryModeNormal)
	ctx := context.Background()
	seedExportFixtures(t, db)

	job := &types.QueryHistoryExportJob{TenantID: 1}
	require.NoError(t, jobRepo.Create(ctx, job))
	task := exportTaskFor(t, job.ID)

	require.NoError(t, svc.ProcessExport(ctx, task))
	done, err := jobRepo.GetByID(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, types.QueryHistoryExportDone, done.Status)
	require.NotEmpty(t, done.FilePath)

	// A retry after the committed done must not rebuild the file.
	require.NoError(t, svc.ProcessExport(ctx, task))
	require.Equal(t, 1, files.saveCalls)
	again, err := jobRepo.GetByID(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, done.FilePath, again.FilePath)
}

func TestProcessExportMissingJobDropsTask(t *testing.T) {
	svc, _, files, _, _ := newExportServiceForTest(t, types.QueryHistoryModeNormal)

	// A stale task whose job row is gone must not error (retrying cannot fix
	// it) and must not write any file.
	require.NoError(t, svc.ProcessExport(context.Background(), exportTaskFor(t, 424242)))
	require.Equal(t, 0, files.saveCalls)
}

func TestProcessExportDisabledPolicyFailsJob(t *testing.T) {
	svc, _, files, jobRepo, db := newExportServiceForTest(t, types.QueryHistoryModeDisabled)
	ctx := context.Background()
	seedExportFixtures(t, db)

	job := &types.QueryHistoryExportJob{TenantID: 1}
	require.NoError(t, jobRepo.Create(ctx, job))

	err := svc.ProcessExport(ctx, exportTaskFor(t, job.ID))
	require.Error(t, err)
	require.Contains(t, err.Error(), "disabled")

	got, err := jobRepo.GetByID(ctx, 1, job.ID)
	require.NoError(t, err)
	require.Equal(t, types.QueryHistoryExportFailed, got.Status)
	require.Equal(t, 0, files.saveCalls, "no file may be written when the policy refuses")
}

func TestProcessExportMalformedPayloadSkipsRetry(t *testing.T) {
	svc, _, files, _, _ := newExportServiceForTest(t, types.QueryHistoryModeNormal)

	err := svc.ProcessExport(context.Background(), asynq.NewTask(types.TypeQueryHistoryExport, []byte("{not json")))
	require.Error(t, err)
	require.ErrorIs(t, err, asynq.SkipRetry)
	require.Equal(t, 0, files.saveCalls)
}
