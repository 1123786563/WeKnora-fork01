package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/modules/conversation/queryhistory"
	"github.com/Tencent/WeKnora/internal/modules/knowledge"
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
// Compatibility wrapper that delegates to Register and ignores the error,
// kept only until old fire-and-forget callers are migrated to the
// error-returning contract. No existing caller registers the same pattern
// twice, so behavior is unchanged.
func (e *SyncTaskExecutor) RegisterHandler(pattern string, handler func(context.Context, *asynq.Task) error) {
	_ = e.Register(pattern, handler)
}

// Enqueue satisfies interfaces.TaskEnqueuer.
// Instead of queuing to Redis, it dispatches the task to a goroutine.
// Supports ProcessIn (delay) and MaxRetry options for parity with asynq.
func (e *SyncTaskExecutor) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	e.mu.RLock()
	handler, ok := e.handlers[task.Type()]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("sync task executor: no handler registered for type %q", task.Type())
	}

	var delay time.Duration
	maxRetry := 25 // asynq default
	maxRetrySet := false
	for _, opt := range opts {
		switch opt.Type() {
		case asynq.ProcessInOpt:
			if d, ok := opt.Value().(time.Duration); ok {
				delay = d
			}
		case asynq.MaxRetryOpt:
			if n, ok := opt.Value().(int); ok {
				maxRetry = n
				maxRetrySet = true
			}
		}
	}
	// Callers that explicitly pass MaxRetry(0) want no retries.
	// Without the flag we can't distinguish "not set" from "set to 0".
	if maxRetrySet && maxRetry < 0 {
		maxRetry = 0
	}

	taskID := uuid.New().String()
	info := &asynq.TaskInfo{
		ID:    taskID,
		Queue: "sync",
		Type:  task.Type(),
	}

	go func() {
		if delay > 0 {
			time.Sleep(delay)
		}

		// Tag as a background worker execution so the per-model concurrency
		// governor throttles Lite-mode ingestion/enrichment LLM calls, mirroring
		// the asynq backgroundTaskMiddleware in the Redis path.
		ctx := types.WithBackgroundTask(context.Background())
		start := time.Now()
		logger.Infof(ctx, "[SyncTask] Executing task type=%s id=%s", task.Type(), taskID)

		var lastErr error
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

			attemptCtx := types.WithTaskRetryMetadata(ctx, attempt, maxRetry)
			lastErr = handler(attemptCtx, task)
			if lastErr == nil {
				logger.Infof(ctx, "[SyncTask] Task completed type=%s id=%s elapsed=%v",
					task.Type(), taskID, time.Since(start))
				return
			}
		}

		logger.Errorf(ctx, "[SyncTask] Task failed (exhausted retries) type=%s id=%s elapsed=%v err=%v",
			task.Type(), taskID, time.Since(start), lastErr)
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
	ChunkExtractor       interfaces.TaskHandler `name:"chunkExtractor"`
	DataTableSummary     interfaces.TaskHandler `name:"dataTableSummary"`
	ImageMultimodal      interfaces.TaskHandler `name:"imageMultimodal"`
	KnowledgePostProcess interfaces.TaskHandler `name:"knowledgePostProcess"`
	KnowledgeAutoTag     interfaces.TaskHandler `name:"knowledgeAutoTag"`
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
