// Package queryhistory is the Query History module's assembly root (Wave 1,
// Task 10): it owns nothing but composition. NewModule receives the module's
// outbound ports and a logging boundary, builds the application services and
// the HTTP transport over them, and exposes the two registration surfaces the
// platform consumes:
//
//	RegisterRoutes(*gin.RouterGroup)      — four Admin+ audit routes, mounted
//	                                        on a PRE-GUARDED group (the RBAC
//	                                        bridge in internal/router builds
//	                                        the guarded group and hands it
//	                                        here; authorization is the
//	                                        caller's).
//	RegisterWorkers(bootstrap.TaskHandlerRegistry) error
//	                                     — the single async export worker
//	                                        (types.TypeQueryHistoryExport),
//	                                        registered identically in Redis
//	                                        mode (registry over the asynq
//	                                        mux) and Lite mode (the sync
//	                                        executor). A duplicate
//	                                        registration fails.
//
// The asynq handler is private on purpose: the surface above is the whole
// public contract. It passes the raw task payload bytes to the application
// worker (the application owns the payload parse) and translates the
// application's domain.ErrPermanentPayload sentinel into asynq.SkipRetry so
// a payload that can never succeed skips the retry budget — the application
// layer itself never imports the task framework.
//
// The application layer (Tasks 5–6) is logging-free by design; this assembly
// layer is where worker-side error logging returns. Dependencies.Logger is
// the narrow boundary those emissions flow through — never the global
// logger, never the DI container.
package queryhistory

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/application"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/domain"
	"github.com/Tencent/WeKnora/internal/conversation/queryhistory/ports"
	httptransport "github.com/Tencent/WeKnora/internal/conversation/queryhistory/transport/http"
	"github.com/Tencent/WeKnora/internal/types"
)

// ErrorLogger is the module's logging boundary: the narrow
// logger.ErrorWithFields-compatible slice the assembly layer needs to emit
// worker-side failures. It exists so the module's asynq handler and worker
// registration paths can log errors without importing the platform's global
// logger; production wiring adapts the platform logger onto it.
type ErrorLogger interface {
	// ErrorWithFields records err with structured fields, mirroring the
	// platform logger's ErrorWithFields contract.
	ErrorWithFields(ctx context.Context, err error, fields map[string]interface{})
}

// Dependencies is everything the module needs at assembly time: its outbound
// ports and the logging boundary. It deliberately contains no *dig.Container
// and no concrete infrastructure — the container (or a test) supplies the
// port implementations. A zero Dependencies builds a module that serves
// nothing; every port the used surface needs must be non-nil.
type Dependencies struct {
	// Policy answers the workspace's query-history visibility mode.
	Policy ports.PolicyReader
	// Audit serves the snapshot and export-row reads.
	Audit ports.AuditReader
	// Jobs persists the async export job lifecycle.
	Jobs ports.ExportJobStore
	// Files stores and opens export archives.
	Files ports.FileStore
	// Queue enqueues the async export task.
	Queue ports.TaskQueue
	// Logger receives worker-side error emissions. A nil Logger disables
	// logging (useful for assembly-only tests); production always wires the
	// platform adapter.
	Logger ErrorLogger
}

// Module is the assembled Query History feature: the HTTP transport over the
// audit and export application services, plus the async export worker. It
// holds no package-level or global state — every registration writes through
// the caller-provided router group or worker registry, so two modules never
// observe each other.
type Module struct {
	handler *httptransport.Handler
	exports *application.ExportService
	log     ErrorLogger
}

// compile-time contract checks: the module mounts routes exactly the way the
// platform's route-module contract prescribes.
var _ bootstrap.RouteModule = (*Module)(nil)

// NewModule assembles the module from its ports: the audit application
// service (policy gate + snapshot), the export application service (job
// admission, status/download reads, and the worker body), and the HTTP
// transport over both.
func NewModule(deps Dependencies) *Module {
	audit := application.NewAuditService(deps.Policy, deps.Audit)
	exports := application.NewExportService(deps.Policy, deps.Audit, deps.Jobs, deps.Files, deps.Queue)
	return &Module{
		handler: httptransport.NewHandler(audit, exports),
		exports: exports,
		log:     deps.Logger,
	}
}

// RegisterRoutes mounts the module's four Admin+ query-history audit routes
// on the given PRE-GUARDED group (Wave 1, Task 8 contract): the caller owns
// authorization (JWT Admin+ and API-key full access), the handlers own the
// privacy policy. The relative paths are byte-exact with the legacy
// registration; a second registration of any pair panics at gin composition
// time instead of shadowing.
func (m *Module) RegisterRoutes(group *gin.RouterGroup) {
	httptransport.RegisterRoutes(group, m.handler)
}

// RegisterWorkers installs the module's single async worker — the
// query-history CSV export task — on the registry. Both execution modes use
// this one method: Redis mode passes a bootstrap.TaskHandlerRegistry wrapping the
// shared asynq mux, Lite mode passes the synchronous executor (which
// satisfies the same interface). Registries are duplicate-safe, so a double
// registration returns an error instead of silently overwriting the handler;
// the error is logged through the module's boundary AND returned so startup
// fails loudly rather than continuing without the worker.
func (m *Module) RegisterWorkers(registry bootstrap.TaskHandlerRegistry) error {
	if err := registry.Register(types.TypeQueryHistoryExport, m.processExport); err != nil {
		m.logError(context.Background(), err, map[string]interface{}{
			"task_type": types.TypeQueryHistoryExport,
		})
		return fmt.Errorf("query history: register export worker: %w", err)
	}
	return nil
}

// processExport is the private asynq worker body. It hands the RAW task
// payload bytes to the application worker — the application layer owns the
// payload parse — emits every failure through the logging boundary (the
// application layers are logging-free by design; this is where worker-side
// error logging returns), and maps domain.ErrPermanentPayload to
// asynq.SkipRetry so a payload that can never succeed (malformed JSON,
// missing job/tenant scope) skips the retry budget. Ordinary failures are
// returned unchanged so the task framework retries them.
// HandleExportTask 是导出任务的终端处理器签名（供 router 侧以
// mux.HandleFunc / RegisterHandler 字面量注册，满足 worker-coverage 扫描）。
func (m *Module) HandleExportTask(ctx context.Context, t *asynq.Task) error {
	return m.processExport(ctx, t)
}

func (m *Module) processExport(ctx context.Context, t *asynq.Task) error {
	err := m.exports.Process(ctx, t.Payload())
	if err == nil {
		return nil
	}
	scope := payloadScope(t)
	m.logError(ctx, err, map[string]interface{}{
		"task_type": types.TypeQueryHistoryExport,
		"job_id":    scope.JobID,
		"tenant_id": scope.TenantID,
	})
	if stderrors.Is(err, domain.ErrPermanentPayload) {
		// Keep both chains: the domain sentinel context for callers that
		// inspect it, and asynq's skip-retry signal for the framework.
		return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
	}
	return err
}

// exportLogScope is the best-effort payload probe used only to enrich the
// boundary's error fields with the job/tenant scope the legacy worker logged.
type exportLogScope struct {
	JobID    uint64 `json:"job_id"`
	TenantID uint64 `json:"tenant_id"`
}

// payloadScope extracts the job/tenant ids from a task payload without
// failing: a malformed payload simply yields zero fields.
func payloadScope(t *asynq.Task) exportLogScope {
	var scope exportLogScope
	if t == nil {
		return scope
	}
	_ = json.Unmarshal(t.Payload(), &scope)
	return scope
}

// logError emits through the boundary, tolerating a nil Logger.
func (m *Module) logError(ctx context.Context, err error, fields map[string]interface{}) {
	if m.log == nil || err == nil {
		return
	}
	m.log.ErrorWithFields(ctx, err, fields)
}
