package router

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/middleware/asynqdl"
	"github.com/Tencent/WeKnora/internal/modules/conversation/queryhistory"
	"github.com/Tencent/WeKnora/internal/knowledge"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/dig"
)

// SyncTaskExecutor executes tasks synchronously (in a goroutine) without Redis.
// Used in Lite mode as a drop-in replacement for *asynq.Client.
type SyncTaskExecutor struct {
	mu       sync.RWMutex
	handlers map[string]func(context.Context, *asynq.Task) error
	// onFinalFailure mirrors the asynq dead-letter callback: it runs once a
	// task exhausts its retries, so a document whose task gave up is marked
	// failed instead of spinning until the housekeeping sweep.
	onFinalFailure func(context.Context, *asynq.Task, error)
}

func NewSyncTaskExecutor() *SyncTaskExecutor {
	return &SyncTaskExecutor{
		handlers: make(map[string]func(context.Context, *asynq.Task) error),
	}
}

// Register registers a task handler for a given type pattern. First
// registration wins: registering an already-installed pattern returns an
// error and keeps the original handler. SyncTaskExecutor therefore
// satisfies bootstrap.TaskHandlerRegistry.
func (e *SyncTaskExecutor) Register(pattern string, handler bootstrap.TaskHandler) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, exists := e.handlers[pattern]; exists {
		return fmt.Errorf("worker %q already registered", pattern)
	}
	e.handlers[pattern] = handler
	return nil
}

// RegisterHandler registers a handler for a given task type pattern.
//
// Upstream parity: the plain map assignment (last registration wins). The
// bootstrap.TaskHandlerRegistry duplicate detection lives in Register; this
// fire-and-forget entry point must stay an overwrite — asynq's ServeMux
// HandleFunc is one too, and tests re-register a pattern to swap in a
// done-signalling handler (sync_task_options_test.go). Routing it through
// the first-wins Register silently ignored the second registration and
// stalled the swap.
func (e *SyncTaskExecutor) RegisterHandler(pattern string, handler func(context.Context, *asynq.Task) error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers[pattern] = handler
}

// SetFinalFailureHook installs the callback run after a task's last failed
// attempt. It receives the error of that attempt, including asynq.SkipRetry.
func (e *SyncTaskExecutor) SetFinalFailureHook(fn func(context.Context, *asynq.Task, error)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onFinalFailure = fn
}

// syncTaskOptions is the subset of asynq options the executor honours.
type syncTaskOptions struct {
	delay    time.Duration
	maxRetry int
	timeout  time.Duration
	deadline time.Time
}

// resolveSyncTaskOptions merges the options given to asynq.NewTask with the
// ones given to Enqueue, the latter winning, as asynq.Client does.
//
// Unlike asynq, a task with neither Timeout nor Deadline gets no default
// timeout: Lite has always run those unbounded (large imports on a laptop),
// and the document pipeline sets its own timeouts explicitly.
func resolveSyncTaskOptions(task *asynq.Task, opts []asynq.Option) syncTaskOptions {
	resolved := syncTaskOptions{maxRetry: 25} // asynq default
	all := append(append([]asynq.Option{}, asynqTaskOptions(task)...), opts...)
	for _, opt := range all {
		switch opt.Type() {
		case asynq.ProcessInOpt:
			if d, ok := opt.Value().(time.Duration); ok {
				resolved.delay = d
			}
		case asynq.ProcessAtOpt:
			if at, ok := opt.Value().(time.Time); ok {
				resolved.delay = time.Until(at)
			}
		case asynq.MaxRetryOpt:
			if n, ok := opt.Value().(int); ok {
				resolved.maxRetry = n
			}
		case asynq.TimeoutOpt:
			if d, ok := opt.Value().(time.Duration); ok {
				resolved.timeout = d
			}
		case asynq.DeadlineOpt:
			if at, ok := opt.Value().(time.Time); ok {
				resolved.deadline = at
			}
		}
	}
	if resolved.maxRetry < 0 {
		resolved.maxRetry = 0
	}
	return resolved
}

// asynqTaskOptions returns the options passed to asynq.NewTask. asynq keeps
// them in an unexported field and only merges them inside Client.Enqueue, so
// an executor standing in for the client has to read the field itself.
// Without this, every Lite task ran with 25 retries and no timeout, since
// callers put MaxRetry / Timeout on NewTask. TestAsynqTaskOptionsReadsNewTaskOptions
// breaks if an asynq upgrade renames the field.
func asynqTaskOptions(task *asynq.Task) []asynq.Option {
	if task == nil {
		return nil
	}
	field := reflect.ValueOf(task).Elem().FieldByName("opts")
	if !field.IsValid() || field.Type() != reflect.TypeOf([]asynq.Option(nil)) {
		return nil
	}
	return *(*[]asynq.Option)(unsafe.Pointer(field.UnsafeAddr()))
}

// attemptContext bounds one attempt the way asynq does: by the task's
// Timeout, and by its Deadline when that comes first.
func (o syncTaskOptions) attemptContext(ctx context.Context) (context.Context, context.CancelFunc) {
	var deadline time.Time
	if o.timeout > 0 {
		deadline = time.Now().Add(o.timeout)
	}
	if !o.deadline.IsZero() && (deadline.IsZero() || o.deadline.Before(deadline)) {
		deadline = o.deadline
	}
	if deadline.IsZero() {
		return context.WithCancel(ctx)
	}
	return context.WithDeadline(ctx, deadline)
}

// Enqueue satisfies interfaces.TaskEnqueuer.
// Instead of queuing to Redis, it dispatches the task to a goroutine.
// Supports ProcessIn / ProcessAt (delay), MaxRetry, Timeout and Deadline for
// parity with asynq, whether they were given here or to asynq.NewTask.
func (e *SyncTaskExecutor) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	e.mu.RLock()
	handler, ok := e.handlers[task.Type()]
	onFinalFailure := e.onFinalFailure
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("sync task executor: no handler registered for type %q", task.Type())
	}

	options := resolveSyncTaskOptions(task, opts)
	maxRetry := options.maxRetry

	taskID := uuid.New().String()
	info := &asynq.TaskInfo{
		ID:    taskID,
		Queue: "sync",
		Type:  task.Type(),
	}

	go func() {
		if options.delay > 0 {
			time.Sleep(options.delay)
		}

		// Tag as a background worker execution so the per-model concurrency
		// governor throttles Lite-mode ingestion/enrichment LLM calls, mirroring
		// the asynq backgroundTaskMiddleware in the Redis path.
		ctx := types.WithBackgroundTask(context.Background())
		start := time.Now()
		logger.Infof(ctx, "[SyncTask] Executing task type=%s id=%s", task.Type(), taskID)

		var lastErr error
		var attemptCtx context.Context
		for attempt := 0; attempt <= maxRetry; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(attempt) * 5 * time.Second
				if backoff > 30*time.Second {
					backoff = 30 * time.Second
				}
				logger.Infof(ctx, "[SyncTask] Retrying task type=%s id=%s attempt=%d/%d backoff=%s",
					task.Type(), taskID, attempt, maxRetry, backoff)
				time.Sleep(backoff)
			}

			var cancel context.CancelFunc
			attemptCtx, cancel = options.attemptContext(types.WithTaskRetryMetadata(ctx, attempt, maxRetry))
			// An unrecovered panic here would take down the whole process.
			lastErr = asynqdl.CallRecovered(attemptCtx, task, handler)
			cancel()
			if lastErr == nil {
				logger.Infof(ctx, "[SyncTask] Task completed type=%s id=%s elapsed=%v",
					task.Type(), taskID, time.Since(start))
				return
			}
			if errors.Is(lastErr, asynq.SkipRetry) {
				break
			}
		}

		logger.Errorf(ctx, "[SyncTask] Task failed (exhausted retries) type=%s id=%s elapsed=%v err=%v",
			task.Type(), taskID, time.Since(start), lastErr)
		if onFinalFailure != nil {
			// The attempt's own context may be past its deadline; the
			// callback only writes status, so give it a fresh bound.
			cbCtx, cancel := context.WithTimeout(context.WithoutCancel(attemptCtx), 30*time.Second)
			defer cancel()
			func() {
				defer func() {
					if r := recover(); r != nil {
						logger.Errorf(ctx, "[SyncTask] final-failure hook panicked for type=%s id=%s: %v",
							task.Type(), taskID, r)
					}
				}()
				onFinalFailure(cbCtx, task, lastErr)
			}()
		}
	}()

	return info, nil
}

type SyncTaskParams struct {
	dig.In

	Executor             *SyncTaskExecutor
	KnowledgeService     interfaces.KnowledgeService
	KnowledgeBaseService interfaces.KnowledgeBaseService
	TagService           interfaces.KnowledgeTagService
	DataSourceService    interfaces.DataSourceService
	SpanTracker          service.SpanTracker
	ChunkExtractor       interfaces.TaskHandler `name:"chunkExtractor"`
	DataTableSummary     interfaces.TaskHandler `name:"dataTableSummary"`
	ImageMultimodal      interfaces.TaskHandler `name:"imageMultimodal"`
	KnowledgePostProcess interfaces.TaskHandler `name:"knowledgePostProcess"`
	KnowledgeAutoTag     interfaces.TaskHandler `name:"knowledgeAutoTag"`
	KnowledgeBaseProfile interfaces.TaskHandler `name:"knowledgeBaseProfile"`
	WikiIngest           interfaces.TaskHandler `name:"wikiIngest"`
	TemporaryDocument    interfaces.TemporaryDocumentService
	MemoryService        interfaces.MemoryService
	// QueryHistory serves the Admin+ async query-history CSV export
	// (SP13 Task 4); the Lite executor dispatches it inline like the rest.
	QueryHistoryExport *service.QueryHistoryExportService
	QueryHistory       *queryhistory.Module

	// KnowledgeModule 是 knowledge 模块装配门面（IB2，K5 Brief (b)）：18 个
	// knowledge 任务处理器的双栈注册经门面执行（RegisterSyncHandlers 内装配点，
	// 见 workers_knowledge.go）。knowledge 分发面字段保留为注入源。
	KnowledgeModule *knowledge.Module
}

// RegisterSyncHandlers registers all task handlers on the SyncTaskExecutor.
// Used in Lite mode instead of RunAsynqServer.
func RegisterSyncHandlers(params SyncTaskParams) {
	// Same callback the asynq dead-letter middleware runs in standard mode.
	if failer := newDeadLetterKnowledgeFailer(params.KnowledgeService, params.SpanTracker); failer != nil {
		params.Executor.SetFinalFailureHook(failer)
	}
	// Register the 18 knowledge task handlers via the module facade
	// (IB2, K5 Brief (b)): one-for-one swap of the former 18 knowledge
	// RegisterHandler lines (same task types, same method values).
	if err := RegisterKnowledgeWorkersLite(params.Executor, params.KnowledgeModule); err != nil {
		logger.Errorf(context.Background(), "register knowledge workers (lite): %v", err)
		panic(err)
	}
	params.Executor.RegisterHandler(types.TypeQueryHistoryExport, params.QueryHistory.HandleExportTask)
	params.Executor.RegisterHandler(types.TypeTemporaryDocumentProcess, params.TemporaryDocument.Process)
	params.Executor.RegisterHandler(types.TypeDataSourceSync, params.DataSourceService.ProcessSync)
	params.Executor.RegisterHandler(types.TypeDataSourcePurge, params.DataSourceService.ProcessDataSourcePurge)
	params.Executor.RegisterHandler(types.TypeMemoryExtract, params.MemoryService.Handle)
	logger.Infof(context.Background(), "[SyncTask] All task handlers registered (Lite mode, no Redis)")
}
