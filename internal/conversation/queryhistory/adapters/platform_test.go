package adapters

// Platform adapter tests (Wave 1, Task 7, brief Step 3): the ports.TaskQueue,
// ports.FileStore, and ports.Clock contracts. The queue adapter must create the
// legacy asynq task (types.TypeQueryHistoryExport on types.QueueMaintenance with
// MaxRetry(3) and Timeout(10*time.Minute), options attached to the task
// exactly like the legacy enqueue) and keep the payload wire format
// byte-identical to the legacy types.QueryHistoryExportPayload, including the
// lf_* tracing fields the domain payload mirrors. The file adapter must
// preserve the platform's generated path convention and return the original
// platform reader. The clock adapter must satisfy ports.Clock and read the
// real wall clock.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/hibiken/asynq"
	"github.com/stretchr/testify/require"
)

// recordingEnqueuer captures the task the queue adapter dispatches.
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
	return &asynq.TaskInfo{ID: "t1", Queue: types.QueueMaintenance, Type: task.Type()}, nil
}

// stubPlatformFiles records the save/open calls the file adapter makes.
type stubPlatformFiles struct {
	interfaces.FileService
	saveData   []byte
	saveTenant uint64
	saveName   string
	saveTemp   *bool
	savePath   string
	saveErr    error
	openPath   string
	openReader io.ReadCloser
	openErr    error
}

func (f *stubPlatformFiles) SaveBytes(
	_ context.Context, data []byte, tenantID uint64, fileName string, temp bool,
) (string, error) {
	if f.saveErr != nil {
		return "", f.saveErr
	}
	f.saveData = data
	f.saveTenant = tenantID
	f.saveName = fileName
	f.saveTemp = &temp
	return f.savePath, nil
}

func (f *stubPlatformFiles) GetFile(_ context.Context, filePath string) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	f.openPath = filePath
	return f.openReader, nil
}

// parseQueryHistoryExportTaskOptions reads the queue/retry/timeout options the
// adapter attaches (same option-parsing pattern the service tests use).
func parseQueryHistoryExportTaskOptions(
	t *testing.T, opts []asynq.Option,
) (queue string, maxRetry int, timeout time.Duration) {
	t.Helper()
	for _, opt := range opts {
		switch opt.Type() {
		case asynq.QueueOpt:
			queue, _ = opt.Value().(string)
		case asynq.TimeoutOpt:
			timeout, _ = opt.Value().(time.Duration)
		case asynq.MaxRetryOpt:
			maxRetry, _ = opt.Value().(int)
		default:
			t.Fatalf("unexpected asynq option type %v", opt.Type())
		}
	}
	return queue, maxRetry, timeout
}

func TestAsynqTaskQueueEnqueueQueryHistoryExport(t *testing.T) {
	enq := &recordingEnqueuer{}
	queue := NewAsynqTaskQueue(enq)
	ctx := context.Background()

	payload := domain.ExportPayload{
		LangfuseTraceID:     "trace-1",
		LangfuseTraceparent: "00-trace-1-span-1-01",
		LangfuseUserID:      "admin-1",
		JobID:               7,
		TenantID:            1,
		RequestedBy:         "admin-1",
		UserID:              "u1",
		StartTimeMs:         time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		FeedbackRating:      "like",
	}
	require.NoError(t, queue.EnqueueQueryHistoryExport(ctx, payload))
	require.NotNil(t, enq.task)
	require.Equal(t, types.TypeQueryHistoryExport, enq.task.Type())
	require.Empty(t, enq.opts, "the legacy enqueue passes no options at Enqueue time; they ride on the task")

	// Wire parity: with no live trace on ctx (Langfuse disabled under tests,
	// so the injection primitive is a no-op) the enqueued payload must be
	// byte-identical to the marshaled domain payload — same keys, same order,
	// same omitempty behavior as the legacy types.QueryHistoryExportPayload.
	domainJSON, err := json.Marshal(&payload)
	require.NoError(t, err)
	require.Equal(t, string(domainJSON), string(enq.task.Payload()),
		"the enqueue wire format must stay byte-identical to the domain payload")

	// The payload still parses as the LEGACY struct (the worker's type), with
	// the lf_* mirror fields landing on the embedded TracingContext.
	var wire types.QueryHistoryExportPayload
	require.NoError(t, json.Unmarshal(enq.task.Payload(), &wire))
	require.Equal(t, uint64(7), wire.JobID)
	require.Equal(t, uint64(1), wire.TenantID)
	require.Equal(t, "admin-1", wire.RequestedBy)
	require.Equal(t, "u1", wire.UserID)
	require.Equal(t, payload.StartTimeMs, wire.StartTimeMs)
	require.Equal(t, int64(0), wire.EndTimeMs, "a zero end bound must stay open (omitempty)")
	require.Equal(t, "like", wire.FeedbackRating)
	require.Equal(t, "trace-1", wire.LangfuseTraceID)
	require.Equal(t, "00-trace-1-span-1-01", wire.LangfuseTraceparent)
	require.Equal(t, "admin-1", wire.LangfuseUserID)
}

func TestAsynqTaskQueueMinimalPayloadCollapsesOmittableFields(t *testing.T) {
	enq := &recordingEnqueuer{}
	queue := NewAsynqTaskQueue(enq)

	require.NoError(t, queue.EnqueueQueryHistoryExport(context.Background(),
		domain.ExportPayload{JobID: 9, TenantID: 2}))
	require.NotNil(t, enq.task)
	// Only the two always-present keys survive; every lf_*/optional field is
	// omitempty exactly like the legacy payload.
	require.Equal(t, `{"job_id":9,"tenant_id":2}`, string(enq.task.Payload()))
}

func TestAsynqTaskQueueOptionsMatchLegacyEnqueue(t *testing.T) {
	queue, maxRetry, timeout := parseQueryHistoryExportTaskOptions(t, queryHistoryExportTaskOptions())
	require.Equal(t, types.QueueMaintenance, queue)
	require.Equal(t, 3, maxRetry)
	require.Equal(t, 10*time.Minute, timeout)
}

// TestAsynqTaskQueueAttachesOptionsToTheTask: asynq.Task exposes no accessor
// for the options baked into NewTask, so the test inspects the option slice's
// LENGTH via reflect (value inspection only, no mutation) to prove the three
// options ride ON the task itself, exactly like the legacy enqueue call.
func TestAsynqTaskQueueAttachesOptionsToTheTask(t *testing.T) {
	enq := &recordingEnqueuer{}
	queue := NewAsynqTaskQueue(enq)
	require.NoError(t, queue.EnqueueQueryHistoryExport(context.Background(),
		domain.ExportPayload{JobID: 1, TenantID: 1}))

	optsField := reflect.ValueOf(enq.task).Elem().FieldByName("opts")
	require.True(t, optsField.IsValid(), "asynq.Task still carries its NewTask options")
	require.Equal(t, 3, optsField.Len(), "queue, retry, and timeout must ride on the task")
}

func TestAsynqTaskQueuePropagatesEnqueueError(t *testing.T) {
	boom := errors.New("redis unavailable")
	queue := NewAsynqTaskQueue(&recordingEnqueuer{err: boom})
	require.ErrorIs(t, queue.EnqueueQueryHistoryExport(context.Background(),
		domain.ExportPayload{JobID: 1, TenantID: 1}), boom)
}

func TestPlatformFileStoreDelegatesAndPreservesGeneratedPath(t *testing.T) {
	files := &stubPlatformFiles{
		savePath:   "local://temp/query_history_export_7.csv",
		openReader: io.NopCloser(strings.NewReader("session_id,title\n")),
	}
	store := NewPlatformFileStore(files)
	ctx := context.Background()

	path, err := store.SaveBytes(ctx, []byte("csv-bytes"), 7, "query_history_export_7.csv", true)
	require.NoError(t, err)
	require.Equal(t, "local://temp/query_history_export_7.csv", path,
		"the platform-generated path is returned verbatim")
	require.Equal(t, []byte("csv-bytes"), files.saveData)
	require.Equal(t, uint64(7), files.saveTenant)
	require.Equal(t, "query_history_export_7.csv", files.saveName)
	require.NotNil(t, files.saveTemp)
	require.True(t, *files.saveTemp, "temp marks derived artifacts riding the expiry policy")

	// temp=false (permanent artifacts) passes through untouched too.
	_, err = store.SaveBytes(ctx, []byte("keep"), 1, "keep.bin", false)
	require.NoError(t, err)
	require.False(t, *files.saveTemp)

	reader, err := store.Open(ctx, "local://temp/query_history_export_7.csv")
	require.NoError(t, err)
	require.True(t, reader == files.openReader,
		"the ORIGINAL platform reader is returned, not a wrapper")
	defer func() { _ = reader.Close() }()
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "session_id,title\n", string(body))
	require.Equal(t, "local://temp/query_history_export_7.csv", files.openPath)
}

func TestPlatformFileStorePropagatesErrors(t *testing.T) {
	boom := errors.New("storage down")
	store := NewPlatformFileStore(&stubPlatformFiles{saveErr: boom, openErr: boom})
	_, err := store.SaveBytes(context.Background(), nil, 1, "f.csv", true)
	require.ErrorIs(t, err, boom)
	_, err = store.Open(context.Background(), "local://x")
	require.ErrorIs(t, err, boom)
}

// TestSystemClockSatisfiesPortAndReadsWallClock: SystemClock is the
// production ports.Clock provider the wiring hands the application layer;
// assigning it to the port here doubles the compile-time assertion. Beyond
// conformance it only delegates to time.Now, so the single runtime assertion
// is a non-zero reading — no frozen wall-clock pinning.
func TestSystemClockSatisfiesPortAndReadsWallClock(t *testing.T) {
	var clock ports.Clock = NewSystemClock()
	require.False(t, clock.Now().IsZero(), "the system clock reads the real wall clock")
}
