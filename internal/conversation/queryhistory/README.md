# Query History module (`internal/conversation/queryhistory`)

The Admin+ query-history audit feature of the **conversation** module: the
per-session audit snapshot, the tenant privacy policy that gates it, and the
async CSV export (submit / poll / download) with its background worker.
Wave 1, Tasks 4–10 built this module out of the legacy
`internal/handler/session` query-history admin surface and the legacy
`service.QueryHistoryExportService`; since Task 10 it is the production
wiring (routes and workers no longer route through the legacy stack).

## Layout

| Package | Role |
| --- | --- |
| `domain` | Pure model: visibility `Mode`/`Config`, `ExportJob`, `ExportRow`, `ExportFilter`, `ExportPayload`, `ErrPermanentPayload` |
| `ports` | Outbound interfaces: `PolicyReader`, `AuditReader`, `ExportJobStore`, `FileStore`, `TaskQueue`, `Clock` |
| `application` | Use cases: `AuditService` (policy gate + snapshot), `ExportService` (`Start`/`Job`/`Open` + the `Process` worker body). Logging-free by design |
| `adapters` | Port implementations over legacy repos and platform services (GORM, asynq, `interfaces.FileService`, `interfaces.TaskEnqueuer`) |
| `transport/http` | Gin handlers + `RegisterRoutes` for the four audit routes |
| `testkit` | Old-vs-new differential observation helpers (Task 9 gate) |
| `module.go` | Assembly root: `NewModule(Dependencies)`, `RegisterRoutes`, `RegisterWorkers` |

## What this module owns

**Routes** (all mounted on a pre-guarded `Admin+` group built by the RBAC
bridge in `internal/router/routes_query_history.go` — JWT callers need the
Admin role, API keys need full tenant access):

| Method | Path (relative to the guarded group) |
| --- | --- |
| GET | `/:session_id/snapshot` |
| POST | `/export` |
| GET | `/export/:job_id/status` |
| GET | `/export/:job_id/download` |

**Task**: `types.TypeQueryHistoryExport` (`query_history:export`) — the async
CSV export worker. The queue adapter pins the legacy processing options:
queue `low` (maintenance), `MaxRetry(3)`, `Timeout(10m)`.

**Table**: `query_history_export_jobs` (via `domain.ExportJob` /
`GormExportJobStore`).

## What this module does NOT own

The audit **data** belongs to other parts of the codebase; this module only
reads it through adapters:

- `sessions`, `messages`, `message_feedback` rows — read through the
  `LegacyAudit` adapter over the temporary
  `repository.SessionAuditRepository` seam (owned by the legacy session
  stack).
- `tenants.query_history_config` — read through the `TenantPolicy` adapter
  over `interfaces.TenantRepository` (owned by identity). The container
  always wires the real `TenantPolicy` adapter; there is no nil-repo
  fallback behind the port in production wiring (the adapter itself
  normalizes a nil repo / missing config to `normal`, mirroring legacy
  behavior).

## Dependency ports

`ports.PolicyReader`, `ports.AuditReader`, `ports.ExportJobStore`,
`ports.FileStore`, `ports.TaskQueue` (and `ports.Clock`, provided by
`adapters.NewSystemClock`; the export lifecycle currently relies on
DB-side timestamps, so no clock is injected at assembly yet). Production
adapters: `adapters.TenantPolicy`, `adapters.LegacyAudit`,
`adapters.GormExportJobStore`, `adapters.PlatformFileStore`,
`adapters.AsynqTaskQueue`. Assembly also takes an `ErrorLogger` boundary
(`logger.ErrorWithFields`-compatible) — the application layers are
logging-free, and worker-side error emissions happen at the module
boundary.

## Worker modes

`(*Module).RegisterWorkers(bootstrap.WorkerRegistry) error` is the single
registration path for both execution modes:

- **Redis**: `internal/router/task.go` wraps the shared asynq mux in
  `bootstrap.NewAsynqWorkerRegistry` and calls `RegisterWorkers`; the
  handler therefore still runs behind the mux's dead-letter,
  background-task, and Langfuse middlewares.
- **Lite**: `internal/router/sync_task.go` calls `RegisterWorkers` on the
  `SyncTaskExecutor`, which satisfies the same registry contract.

Both registries are duplicate-safe: registering the export task type twice
fails, and the registration error propagates during startup (no
log-and-continue). The private asynq handler passes `task.Payload()` to
`application.ExportService.Process` and maps `domain.ErrPermanentPayload`
to `asynq.SkipRetry`.

## Wave 5 legacy exceptions (manifest-recorded)

1. `adapters → internal/application/repository` — the adapters wrap the
   legacy Session/Tenant persistence seams. Remove in Wave 5 when the
   conversation module owns its persistence.
2. `application → internal/types` — `AuditReader.Snapshot` still answers
   the legacy `*types.QueryHistorySnapshot` (Session/Message/
   MessageFeedback serializations). Remove in Wave 5 once Conversation
   owns the Session and Message types.

Related Wave 1 compatibility notes: `domain.Mode` and `domain.ExportStatus`
are string aliases of the legacy `types` constants so existing call sites
compile unchanged; `domain.ExportPayload` is a byte-compatible mirror of
`types.QueryHistoryExportPayload` so payloads enqueued before the switch
keep parsing.
