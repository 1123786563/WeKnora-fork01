package adapters

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"github.com/Tencent/WeKnora/internal/tracing/langfuse"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
)

// PlatformFileStore satisfies ports.FileStore with the narrow slice of the
// platform file service the export worker needs. It is a deliberate
// pass-through: the storage backend keeps generating the stored path (the
// tenant-scoped, temp-aware convention) and keeps handing back its own
// reader, un-wrapped, so streaming and expiry behavior stay exactly the
// legacy ones.
type PlatformFileStore struct {
	files interfaces.FileService
}

// NewPlatformFileStore builds the file adapter over the platform file service.
func NewPlatformFileStore(files interfaces.FileService) *PlatformFileStore {
	return &PlatformFileStore{files: files}
}

// compile-time port conformance.
var _ ports.FileStore = (*PlatformFileStore)(nil)

// SaveBytes writes data under the tenant and returns the platform-generated
// storage path; temp marks derived artifacts that ride the expiry policy.
func (s *PlatformFileStore) SaveBytes(
	ctx context.Context, data []byte, tenantID uint64, fileName string, temp bool,
) (string, error) {
	return s.files.SaveBytes(ctx, data, tenantID, fileName, temp)
}

// Open returns the ORIGINAL platform reader over a stored path.
func (s *PlatformFileStore) Open(ctx context.Context, path string) (io.ReadCloser, error) {
	return s.files.GetFile(ctx, path)
}

// AsynqTaskQueue satisfies ports.TaskQueue over the platform enqueuer. It
// reproduces the legacy enqueue byte for byte: task type, queue, retry, and
// timeout options attached to the task itself, and a payload whose wire
// format is exactly the legacy types.QueryHistoryExportPayload.
type AsynqTaskQueue struct {
	enqueuer interfaces.TaskEnqueuer
}

// NewAsynqTaskQueue builds the queue adapter over the platform enqueuer
// (*asynq.Client in Redis mode, the synchronous executor in Lite mode).
func NewAsynqTaskQueue(enqueuer interfaces.TaskEnqueuer) *AsynqTaskQueue {
	return &AsynqTaskQueue{enqueuer: enqueuer}
}

// compile-time port conformance.
var _ ports.TaskQueue = (*AsynqTaskQueue)(nil)

// queryHistoryExportTaskOptions pins the legacy enqueue's processing options:
// the maintenance queue (physical "low"), three retries, and a ten-minute
// timeout — identical to the legacy QueryHistoryExportService.StartExport.
func queryHistoryExportTaskOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(types.QueueMaintenance),
		asynq.MaxRetry(3),
		asynq.Timeout(10 * time.Minute),
	}
}

// EnqueueQueryHistoryExport enqueues one async query-history export.
//
// lf_* tracing (CRITICAL Task 4 interface note): domain.ExportPayload is a
// pure mirror of types.QueryHistoryExportPayload and does NOT embed
// types.TracingContext, so it lacks the SetLangfuseTracing /
// GetLangfuseTracing carrier methods the legacy enqueue path relied on for
// auto-injection. The adapter therefore manages the tracing fields MANUALLY:
// it seeds a legacy types.TracingContext from the domain payload's lf_*
// mirror fields, runs the very same langfuse.InjectTracing primitive the
// legacy enqueue ran (against that TracingContext — the primitive is enabled
// only when the Langfuse manager is on, and then OVERWRITES the five lf_
// fields from the request context), and embeds the result into a
// types.QueryHistoryExportPayload copy of the domain fields. When Langfuse is
// off the payload's own lf_* seeds survive, exactly as they did before. The
// marshaled bytes are therefore byte-identical to the legacy enqueue's.
func (q *AsynqTaskQueue) EnqueueQueryHistoryExport(
	ctx context.Context, payload domain.ExportPayload,
) error {
	wire := types.QueryHistoryExportPayload{
		TracingContext: types.TracingContext{
			LangfuseTraceID:             payload.LangfuseTraceID,
			LangfuseParentObservationID: payload.LangfuseParentObservationID,
			LangfuseTraceparent:         payload.LangfuseTraceparent,
			LangfuseUserID:              payload.LangfuseUserID,
			LangfuseSessionID:           payload.LangfuseSessionID,
		},
		JobID:          payload.JobID,
		TenantID:       payload.TenantID,
		RequestedBy:    payload.RequestedBy,
		UserID:         payload.UserID,
		StartTimeMs:    payload.StartTimeMs,
		EndTimeMs:      payload.EndTimeMs,
		FeedbackRating: payload.FeedbackRating,
	}
	langfuse.InjectTracing(ctx, &wire.TracingContext)

	payloadJSON, err := json.Marshal(&wire)
	if err != nil {
		return err
	}
	task := asynq.NewTask(types.TypeQueryHistoryExport, payloadJSON,
		queryHistoryExportTaskOptions()...)
	_, err = q.enqueuer.Enqueue(task)
	return err
}

// SystemClock satisfies ports.Clock with the real wall clock. It is the
// production time provider the module wiring hands to the application layer;
// the application layer's own tests keep injecting frozen fakes, so this
// adapter stays a plain delegation with no behavior of its own.
type SystemClock struct{}

// NewSystemClock builds the wall-clock provider.
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// compile-time port conformance.
var _ ports.Clock = (*SystemClock)(nil)

// Now reads the real wall clock.
func (c *SystemClock) Now() time.Time {
	return time.Now()
}
