# Backend Modularization Wave 0–1 Implementation Plan

> **SUPERSEDED:** Replaced by `docs/plans/2026-09-21-backend-modularization-foundation-pass-a.md` after approval of the two-pass Move First / Refactor Second Spec at commit `f81f132ca`. Do not execute this plan.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish enforceable backend-module ownership and migrate Conversation/Query History as the first behavior-compatible vertical module.

**Architecture:** Wave 0 adds a machine-readable ownership inventory and a Go architecture guard that covers server routes, workers, lifecycle hooks, migrations, and forbidden imports. Wave 1 adds reusable route/worker composition contracts, then moves Query History policy, snapshot, CSV export, HTTP transport, persistence, and worker registration under `internal/modules/conversation/queryhistory`, leaving explicit adapters around the still-legacy Session/Tenant code.

**Tech Stack:** Go 1.26, Gin, GORM, dig, asynq, YAML v3, Testify, SQLite test database.

**Spec:** `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`

## Global Constraints

- Scope is `cmd/server` plus the root module's `internal/`; do not modify `cmd/desktop`, `docreader`, `client`, or standalone SDK/contract modules.
- Preserve every Query History route, response body, status code, RBAC/API-key requirement, CSV column order/BOM/file name, database table, task type, queue, retry, timeout, and idempotency behavior.
- One business fact has one writer. Do not introduce dual writes or a second Session/Tenant repository.
- Domain and application code do not depend on Gin, GORM, Redis, asynq servers, or concrete external SDK clients.
- Migration adapters may depend on legacy Session/Tenant types only when the ownership manifest records the exact exception and its Wave 5 removal condition.
- No package-level mutable registration is added. A route, worker, or lifecycle hook must be registered exactly once through an explicit module instance.
- Existing failing tests must be recorded in the Wave 0 baseline; never delete or weaken assertions to make the migration green.
- Keep the repository's globally ordered migrations unchanged in Wave 0–1; this plan creates no schema migration.
- Use RED → GREEN → REFACTOR for each behavior-bearing task and commit only that task's owned files.

## Review Focus

- Duplicate route or worker registration must fail deterministically before serving requests; it must never silently overwrite a handler.
- A missing or foreign-tenant export job must remain indistinguishable and return 404 without leaking the owning tenant.
- A policy changed to `disabled` after enqueue must be rechecked by the worker, mark the job failed, and write no CSV.
- Malformed or tenantless worker payloads must be permanent/skip-retry failures and must not update an unrelated job.
- Redis/asynq and Lite/synchronous execution must register the same Query History task type and call the same module worker.

---

## File Structure

### Wave 0 ownership and guard files

| File | Responsibility |
|---|---|
| `docs/architecture/backend-modules.yaml` | Canonical list of 16 modules, Platform, current route/worker/hook/migration ownership, target package prefixes, and temporary dependency exceptions. |
| `docs/architecture/backend-baseline.md` | Exact pre-migration build/test commands, results, known failures, and inventory-generation date/SHA. |
| `tools/architectureguard/manifest.go` | Strict YAML schema and loader; rejects unknown fields, duplicate IDs, missing owners, and expired exceptions. |
| `tools/architectureguard/discovery.go` | Go AST/filesystem discovery of server route registrations, worker task registrations, lifecycle invokes, migration files, and imports. |
| `tools/architectureguard/check.go` | Coverage and dependency-rule evaluation with stable diagnostics. |
| `tools/architectureguard/main.go` | `go run ./tools/architectureguard -manifest ... -root ...` entry point. |
| `tools/architectureguard/*_test.go` | Fixture-driven schema, coverage, import, duplicate, and scope tests. |
| `Makefile` | Adds `check-backend-architecture` without changing existing targets. |

### Composition contracts

| File | Responsibility |
|---|---|
| `internal/bootstrap/routes.go` | Minimal authenticated route-module contract; router supplies a pre-guarded Gin group. |
| `internal/bootstrap/workers.go` | Duplicate-safe `WorkerRegistry` contract and asynq adapter. |
| `internal/bootstrap/workers_test.go` | Duplicate and dispatch behavior. |
| `internal/router/sync_task.go` | Makes the Lite executor implement the same duplicate-safe registry contract. |
| `internal/router/sync_task_test.go` | Lite duplicate registration and parity tests. |

### Query History module

| File | Responsibility |
|---|---|
| `internal/modules/conversation/queryhistory/module.go` | Module construction, route registration, worker registration, and exported module surface. |
| `internal/modules/conversation/queryhistory/domain/model.go` | Mode, config, export job/status/filter/row/payload, normalization, GORM value/scan, and stable errors. |
| `internal/modules/conversation/queryhistory/ports/ports.go` | Audit reader, policy reader, job store, file store, task queue, clock, and tracing ports. |
| `internal/modules/conversation/queryhistory/application/policy.go` | Access decision and anonymization. |
| `internal/modules/conversation/queryhistory/application/snapshot.go` | Tenant-scoped snapshot orchestration through the audit port. |
| `internal/modules/conversation/queryhistory/application/export.go` | Submit/status/download metadata and CSV export orchestration. |
| `internal/modules/conversation/queryhistory/application/worker.go` | Payload validation, policy recheck, idempotency, retries, CSV generation, and job transitions. |
| `internal/modules/conversation/queryhistory/adapters/legacy_audit.go` | Temporary adapter over existing Session/Message/Feedback repositories. |
| `internal/modules/conversation/queryhistory/adapters/tenant_policy.go` | Temporary adapter over TenantRepository. |
| `internal/modules/conversation/queryhistory/adapters/gorm_export.go` | Export-job persistence; every status update is tenant-scoped. |
| `internal/modules/conversation/queryhistory/adapters/platform.go` | FileService, TaskEnqueuer, clock, and tracing adapters. |
| `internal/modules/conversation/queryhistory/transport/http/handler.go` | Gin endpoint methods, DTO parsing, AppError mapping, CSV streaming. |
| `internal/modules/conversation/queryhistory/transport/http/routes.go` | Relative `/admin/sessions` route registration on a pre-guarded group. |
| `internal/modules/conversation/queryhistory/testkit/observation.go` | Test-only normalization and comparison of legacy/new HTTP, job, file, and task observations. |
| `internal/modules/conversation/queryhistory/README.md` | Responsibilities, non-responsibilities, ports, owned data, routes, worker, legacy exceptions, and removal wave. |
| `internal/modules/conversation/queryhistory/**/*_test.go` | Domain, application, adapter, transport, module, Redis/Lite parity, and compatibility tests. |

### Compatibility and production wiring

| File | Responsibility |
|---|---|
| `internal/types/query_history.go` | Temporary aliases for moved Query History domain types; retains shared-session compatibility types until Conversation Wave 5. |
| `internal/types/interfaces/query_history_export.go` | Deleted after consumers use the module port. |
| `internal/container/container.go` | Provides one Query History module instead of legacy repo/service/handler dependencies. |
| `internal/router/routes_query_history.go` | Small RBAC bridge that delegates to the module; no Query History handler logic. |
| `internal/router/router.go` | Router params depend on the module rather than the Session handler for Query History routes. |
| `internal/router/task.go` | Registers module workers through `WorkerRegistry` in Redis mode. |
| `internal/router/sync_task.go` | Registers the same module workers in Lite mode. |
| Legacy Query History files | Deleted only after all compatibility tests pass and imports are zero. |

---

### Task 1: Create the Canonical Backend Ownership Inventory

**Files:**
- Create: `docs/architecture/backend-modules.yaml`
- Create: `docs/architecture/backend-baseline.md`
- Test: manual schema validation in Task 2

**Interfaces:**
- Consumes: module names and ownership rules from the approved Spec.
- Produces: YAML root keys `schema_version`, `scope`, `modules`, `platform`, `assets`, and `temporary_exceptions`; stable module IDs used by the guard and later module READMEs.

- [ ] **Step 1: Capture the current baseline without changing code**

Run from a clean execution worktree created from the approved Spec commit `597c642c1` or a current descendant. Capture the actual execution baseline, because later unrelated mainline commits must not be attributed to this project:

```bash
ARCH_BASE_SHA=$(git rev-parse HEAD)
echo "$ARCH_BASE_SHA"
git rev-parse HEAD
go build ./...
go test ./internal/... -count=1 -timeout=25m
go test ./tools/... -count=1
```

Start `docs/architecture/backend-baseline.md` with `base_sha: <the printed SHA>`, using the actual 40-character value rather than the angle-bracket notation. Record every command, exit status, failing test name, and first actionable error. A missing external service or opt-in integration test is recorded as `blocked-env`, not as PASS.

- [ ] **Step 2: Write the inventory header and complete owner list**

Use this exact schema and IDs:

```yaml
schema_version: 1
scope:
  include: [cmd/server, internal]
  exclude: [cmd/desktop, docreader, client]
modules:
  - {id: identity, target_prefix: internal/modules/identity}
  - {id: knowledge, target_prefix: internal/modules/knowledge}
  - {id: conversation, target_prefix: internal/modules/conversation}
  - {id: agentcatalog, target_prefix: internal/modules/agentcatalog}
  - {id: agentruntime, target_prefix: internal/modules/agentruntime}
  - {id: workbench, target_prefix: internal/modules/workbench}
  - {id: craft, target_prefix: internal/modules/craft}
  - {id: execution, target_prefix: internal/modules/execution}
  - {id: datasource, target_prefix: internal/modules/datasource}
  - {id: appconnector, target_prefix: internal/modules/appconnector}
  - {id: channels, target_prefix: internal/modules/channels}
  - {id: airesource, target_prefix: internal/modules/airesource}
  - {id: commercial, target_prefix: internal/modules/commercial}
  - {id: insights, target_prefix: internal/modules/insights}
  - {id: system, target_prefix: internal/modules/system}
  - {id: policy, target_prefix: internal/modules/policy}
platform:
  id: platform
  target_prefixes: [internal/platform, internal/bootstrap]
```

- [ ] **Step 3: Inventory every current server asset**

Add one `assets` entry per discovered item with `kind`, `id`, `owner`, `source`, and `target`. Use kinds `route`, `worker`, `lifecycle`, `migration`, and `package`. At minimum include every `Register*Routes` call reachable from `internal/router/router.go`, every `RegisterHandler`/`HandleFunc` task type in `internal/router/task.go` and `internal/router/sync_task.go`, every `container.Invoke` startup/cleanup function, every migration pair under `internal/database/migrations`, and every production package under `internal/`.

Query History entries must be exact:

```yaml
assets:
  - kind: route
    id: queryhistory.admin-routes
    owner: conversation
    source: internal/router/routes_query_history.go
    symbol: RegisterQueryHistoryAdminRoutes
    target: internal/modules/conversation/queryhistory/transport/http
  - kind: worker
    id: queryhistory.export
    owner: conversation
    source: internal/router/task.go
    task_type: TypeQueryHistoryExport
    target: internal/modules/conversation/queryhistory/application
  - kind: package
    id: queryhistory.legacy-service
    owner: conversation
    source: internal/application/service/query_history_export.go
    target: internal/modules/conversation/queryhistory
```

- [ ] **Step 4: Record only the two intentional Wave 1 legacy exceptions**

```yaml
temporary_exceptions:
  - from: internal/modules/conversation/queryhistory/adapters
    to: internal/application/repository
    reason: legacy Session/Tenant persistence adapter during Wave 1
    remove_in_wave: 5
  - from: internal/modules/conversation/queryhistory/application
    to: internal/types
    reason: snapshot wire compatibility until Conversation owns Session and Message types
    remove_in_wave: 5
```

Do not add wildcard exceptions.

- [ ] **Step 5: Review inventory coverage manually**

Run:

```bash
rg -n 'Register[A-Za-z0-9]+Routes\(' internal/router/router.go
rg -n 'RegisterHandler\(|HandleFunc\(' internal/router/task.go internal/router/sync_task.go
rg -n 'container\.Invoke\(' internal/container
find internal/database -type f | sort
```

Check every result has an asset entry. Record counts by kind in the baseline document.

- [ ] **Step 6: Commit the inventory**

```bash
git add docs/architecture/backend-modules.yaml docs/architecture/backend-baseline.md
git commit -m "docs: inventory backend module ownership"
```

---

### Task 2: Add the Architecture Guard

**Files:**
- Create: `tools/architectureguard/manifest.go`
- Create: `tools/architectureguard/discovery.go`
- Create: `tools/architectureguard/check.go`
- Create: `tools/architectureguard/main.go`
- Create: `tools/architectureguard/testdata/valid.yaml`
- Create: `tools/architectureguard/testdata/duplicate-owner.yaml`
- Create: `tools/architectureguard/testdata/missing-route.yaml`
- Create: `tools/architectureguard/testdata/forbidden-import.yaml`
- Test: `tools/architectureguard/manifest_test.go`
- Test: `tools/architectureguard/check_test.go`
- Modify: `Makefile`

**Interfaces:**
- Consumes: Task 1 manifest schema.
- Produces: `LoadManifest(path string) (*Manifest, error)`, `Discover(root string) (*Discovery, error)`, `Check(manifest *Manifest, discovery *Discovery) []Violation`, and CLI exit code 1 on violations.

- [ ] **Step 1: Write failing strict-loader tests**

```go
func TestLoadManifestRejectsUnknownAndDuplicateOwners(t *testing.T) {
    _, err := LoadManifest("testdata/duplicate-owner.yaml")
    require.ErrorContains(t, err, "duplicate asset id")
}

func TestLoadManifestRejectsWildcardException(t *testing.T) {
    _, err := LoadManifest("testdata/forbidden-import.yaml")
    require.ErrorContains(t, err, "exception paths must be exact prefixes")
}
```

- [ ] **Step 2: Run loader tests and verify RED**

Run: `go test ./tools/architectureguard -run TestLoadManifest -count=1 -v`

Expected: FAIL because `LoadManifest` is undefined.

- [ ] **Step 3: Implement the strict manifest loader**

Use `yaml.Decoder.KnownFields(true)`. Model the schema with explicit structs; normalize paths with `path.Clean`; reject absolute paths, `..`, duplicate module IDs, duplicate asset IDs, unknown owners, empty sources/targets, wildcard exceptions, and exceptions whose `remove_in_wave` is less than 1.

```go
func LoadManifest(filename string) (*Manifest, error) {
    f, err := os.Open(filename)
    if err != nil { return nil, err }
    defer f.Close()
    dec := yaml.NewDecoder(f)
    dec.KnownFields(true)
    var m Manifest
    if err := dec.Decode(&m); err != nil { return nil, err }
    if err := m.Validate(); err != nil { return nil, err }
    return &m, nil
}
```

- [ ] **Step 4: Write failing discovery and coverage tests**

Tests create a temporary miniature repository containing a route registration, Redis worker, Lite worker, lifecycle hook, migration, and an illegal import. Assert `Discover` returns stable IDs and `Check` reports:

```text
unowned route: router.RegisterFooRoutes
worker parity mismatch: task.type.foo missing from lite
forbidden import: internal/modules/a -> internal/modules/b/adapters
expired exception: ... remove_in_wave=1 current_wave=2
```

- [ ] **Step 5: Run checker tests and verify RED**

Run: `go test ./tools/architectureguard -run 'Test(Discover|Check)' -count=1 -v`

Expected: FAIL because discovery/checking functions are undefined.

- [ ] **Step 6: Implement AST discovery and stable diagnostics**

Use `go/parser`/`go/ast`, not regex, for imports and Go call expressions. Discover route registration calls in the server router, worker task expressions in Redis/Lite registration functions, and `container.Invoke` function arguments. Use filesystem enumeration for migrations and packages. Sort all discovered assets and violations before comparison/output.

`Check` must enforce complete ownership, Redis/Lite worker parity, target-prefix validity, no duplicate asset IDs, and these dependency rules:

```go
var forbiddenTargetSegments = []string{"/transport/", "/adapters/"}
// A module may import another module only through that module's root public
// package or explicit contracts package. Domain may import no other module.
```

- [ ] **Step 7: Implement CLI and Make target**

```make
.PHONY: check-backend-architecture
check-backend-architecture:
	go run ./tools/architectureguard -root . -manifest docs/architecture/backend-modules.yaml -wave 1
```

The CLI prints one violation per line to stderr and exits 1; it prints `backend architecture guard: ok` and exits 0 on success.

- [ ] **Step 8: Run the real guard and reconcile inventory only**

Run:

```bash
go test ./tools/architectureguard -count=1
make check-backend-architecture
```

Expected: PASS. Fix missing ownership in YAML; do not add broad exceptions or weaken discovery.

- [ ] **Step 9: Commit the guard**

```bash
git add tools/architectureguard Makefile docs/architecture/backend-modules.yaml
git commit -m "test: enforce backend module ownership"
```

---

### Task 3: Introduce Duplicate-Safe Module Composition Contracts

**Files:**
- Create: `internal/bootstrap/routes.go`
- Create: `internal/bootstrap/workers.go`
- Test: `internal/bootstrap/workers_test.go`
- Modify: `internal/router/sync_task.go`
- Test: `internal/router/sync_task_test.go`

**Interfaces:**
- Consumes: Gin route groups and asynq handlers.
- Produces: `type RouteModule interface { RegisterRoutes(*gin.RouterGroup) }`; `type TaskHandler func(context.Context, *asynq.Task) error`; `type WorkerRegistry interface { Register(string, TaskHandler) error }`; `NewAsynqWorkerRegistry(*asynq.ServeMux) WorkerRegistry`; Lite `SyncTaskExecutor.Register(string, bootstrap.TaskHandler) error`.

- [ ] **Step 1: Write failing duplicate-registration tests**

```go
func TestAsynqWorkerRegistryRejectsDuplicate(t *testing.T) {
    r := NewAsynqWorkerRegistry(asynq.NewServeMux())
    require.NoError(t, r.Register("task:x", func(context.Context, *asynq.Task) error { return nil }))
    require.ErrorContains(t, r.Register("task:x", func(context.Context, *asynq.Task) error { return nil }), "already registered")
}
```

Add an equivalent Lite test and assert the first handler remains installed after the rejected duplicate.

- [ ] **Step 2: Run tests and verify RED**

Run: `go test ./internal/bootstrap ./internal/router -run 'WorkerRegistry|RegisterDuplicate' -count=1 -v`

Expected: FAIL because the contracts do not exist and Lite silently overwrites.

- [ ] **Step 3: Implement composition contracts**

`AsynqWorkerRegistry` owns a mutex and `map[string]struct{}`; it checks/records before `mux.HandleFunc`. `SyncTaskExecutor.Register` performs the same check under its existing mutex. Keep `RegisterHandler` temporarily as a delegating compatibility wrapper only until Task 10 removes old callers.

```go
func (e *SyncTaskExecutor) Register(pattern string, h bootstrap.TaskHandler) error {
    e.mu.Lock()
    defer e.mu.Unlock()
    if _, exists := e.handlers[pattern]; exists {
        return fmt.Errorf("worker %q already registered", pattern)
    }
    e.handlers[pattern] = h
    return nil
}
```

- [ ] **Step 4: Run focused tests**

Run: `go test ./internal/bootstrap ./internal/router -run 'WorkerRegistry|RegisterDuplicate' -count=1`

Expected: PASS.

- [ ] **Step 5: Run existing task executor tests**

Run: `go test ./internal/router -run 'SyncTask|Asynq' -count=1`

Expected: PASS with no behavior change.

- [ ] **Step 6: Commit composition contracts**

```bash
git add internal/bootstrap internal/router/sync_task.go internal/router/sync_task_test.go
git commit -m "refactor: add module composition registries"
```

---

### Task 4: Move Query History Domain Types and Define Ports

**Files:**
- Create: `internal/modules/conversation/queryhistory/domain/model.go`
- Test: `internal/modules/conversation/queryhistory/domain/model_test.go`
- Create: `internal/modules/conversation/queryhistory/ports/ports.go`
- Modify: `internal/types/query_history.go`
- Test: `internal/types/query_history_test.go`

**Interfaces:**
- Consumes: legacy Session/Message/Feedback snapshot DTO only as the manifest-recorded compatibility seam.
- Produces: `domain.Mode`, `domain.Config`, `domain.ExportStatus`, `domain.ExportJob`, `domain.ExportFilter`, `domain.ExportRow`, `domain.ExportPayload`; ports `PolicyReader`, `AuditReader`, `ExportJobStore`, `FileStore`, `TaskQueue`, `Clock`.

- [ ] **Step 1: Write failing domain tests**

Cover normalization of empty/unknown modes to normal, JSON/GORM round trip, pending default, stable table name, and status constants.

```go
func TestNormalizeMode(t *testing.T) {
    assert.Equal(t, Normal, NormalizeMode("unknown"))
    assert.Equal(t, Anonymized, NormalizeMode(" anonymized "))
}
```

- [ ] **Step 2: Run domain tests and verify RED**

Run: `go test ./internal/modules/conversation/queryhistory/domain -count=1 -v`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement domain types without framework imports**

Move Query History mode/config/export types and their `Value`, `Scan`, and `TableName` behavior. `domain` may import only Go standard library packages.

- [ ] **Step 4: Define ports with exact signatures**

```go
type PolicyReader interface { Mode(context.Context, uint64) (domain.Mode, error) }
type AuditReader interface {
    Snapshot(context.Context, uint64, string) (*types.QueryHistorySnapshot, error)
    ExportRows(context.Context, uint64, domain.ExportFilter) ([]domain.ExportRow, error)
}
type ExportJobStore interface {
    Create(context.Context, *domain.ExportJob) error
    Get(context.Context, uint64, uint64) (*domain.ExportJob, error)
    UpdateStatus(context.Context, uint64, uint64, domain.ExportStatus, string, string) error
}
type FileStore interface {
    SaveBytes(context.Context, []byte, uint64, string, bool) (string, error)
    Open(context.Context, string) (io.ReadCloser, error)
}
type TaskQueue interface { EnqueueQueryHistoryExport(context.Context, domain.ExportPayload) error }
type Clock interface { Now() time.Time }
```

The `types.QueryHistorySnapshot` dependency is the exact Wave 5 exception. No other application/domain file may import legacy repositories.

- [ ] **Step 5: Add legacy aliases and run compatibility tests**

In `internal/types/query_history.go`, alias moved config/export types and constants to the new domain package. Retain `QueryHistorySnapshot` and `SharedSessionSnapshot` locally.

Run:

```bash
go test ./internal/modules/conversation/queryhistory/domain ./internal/types -run QueryHistory -count=1
```

Expected: PASS with unchanged JSON/GORM behavior.

- [ ] **Step 6: Commit domain and ports**

```bash
git add internal/modules/conversation/queryhistory/domain internal/modules/conversation/queryhistory/ports internal/types/query_history.go internal/types/query_history_test.go
git commit -m "refactor(queryhistory): define domain and ports"
```

---

### Task 5: Move Policy and Snapshot Application Logic

**Files:**
- Create: `internal/modules/conversation/queryhistory/application/policy.go`
- Create: `internal/modules/conversation/queryhistory/application/snapshot.go`
- Test: `internal/modules/conversation/queryhistory/application/policy_test.go`
- Test: `internal/modules/conversation/queryhistory/application/snapshot_test.go`

**Interfaces:**
- Consumes: `ports.PolicyReader`, `ports.AuditReader`.
- Produces: `NewAuditService(policy ports.PolicyReader, audit ports.AuditReader) *AuditService`; `CheckAccess(context.Context, uint64) (domain.Mode, error)`; `Snapshot(context.Context, uint64, string) (*types.QueryHistorySnapshot, error)`.

- [ ] **Step 1: Write failing policy tests**

Port current normal/anonymized/disabled coverage and add missing/zero tenant tests. Disabled returns the existing forbidden AppError text exactly: `query history is disabled for this tenant`.

- [ ] **Step 2: Write failing snapshot tests**

Port normal, anonymized, disabled, not-found, 200-message truncation, and cross-tenant scope tests. Add a test that `AuditReader.Snapshot` is not called when policy is disabled.

- [ ] **Step 3: Run tests and verify RED**

Run: `go test ./internal/modules/conversation/queryhistory/application -run 'Policy|Snapshot' -count=1 -v`

Expected: FAIL because `AuditService` does not exist.

- [ ] **Step 4: Implement minimal policy and snapshot service**

```go
func (s *AuditService) CheckAccess(ctx context.Context, tenantID uint64) (domain.Mode, error) {
    if tenantID == 0 { return "", errors.New("workspace id is required") }
    mode, err := s.policy.Mode(ctx, tenantID)
    if err != nil { return "", err }
    if mode == domain.Disabled { return mode, apperrors.NewForbiddenError("query history is disabled for this tenant") }
    return mode, nil
}
```

`Snapshot` checks policy first, calls the tenant-scoped reader once, and masks Session/Feedback user IDs only for anonymized mode.

- [ ] **Step 5: Run focused and legacy characterization tests**

```bash
go test ./internal/modules/conversation/queryhistory/application -run 'Policy|Snapshot' -count=1
go test ./internal/application/service -run 'QueryHistoryAccess|QueryHistorySnapshot' -count=1
```

Expected: new tests PASS; legacy tests remain PASS until deletion.

- [ ] **Step 6: Commit policy and snapshot logic**

```bash
git add internal/modules/conversation/queryhistory/application
git commit -m "refactor(queryhistory): move audit policy and snapshot"
```

---

### Task 6: Move Export Submission and Worker Logic

**Files:**
- Create: `internal/modules/conversation/queryhistory/application/export.go`
- Create: `internal/modules/conversation/queryhistory/application/worker.go`
- Test: `internal/modules/conversation/queryhistory/application/export_test.go`
- Test: `internal/modules/conversation/queryhistory/application/worker_test.go`

**Interfaces:**
- Consumes: domain/ports from Task 4 and `AuditService` from Task 5.
- Produces: `NewExportService(...) *ExportService`; `Start(context.Context, uint64, string, domain.ExportFilter) (uint64, error)`; `Job(context.Context, uint64, uint64) (*domain.ExportJob, error)`; `Open(context.Context, uint64, uint64) (io.ReadCloser, string, error)`; `Process(context.Context, []byte) error`; `domain.ErrPermanentPayload` for invalid non-retryable payloads.

- [ ] **Step 1: Write failing submit/status/download tests**

Port job creation, tenant-required, enqueue failure marks failed, tenant-scoped status, not-ready download, and completed filename behavior. Assert enqueue receives the same filter values and task options through the queue adapter.

- [ ] **Step 2: Write failing worker tests**

Port full-chain success, anonymized user IDs, storage failure, idempotent done rerun, missing job drop, disabled policy, malformed payload skip retry, and retry behavior. Add Review Focus cases for tenantless payload and policy changed after enqueue.

- [ ] **Step 3: Run tests and verify RED**

Run: `go test ./internal/modules/conversation/queryhistory/application -run 'Export|Worker' -count=1 -v`

Expected: FAIL because export/worker types do not exist.

- [ ] **Step 4: Implement export service and worker**

Keep this exact CSV contract:

```go
var exportColumns = []string{
    "session_id", "title", "user_id", "source", "engine_type",
    "created_at", "updated_at", "message_count", "like_count", "dislike_count",
}
```

Worker order is: validate payload → tenant-scoped job read → done short-circuit → policy recheck → running transition → audit rows → anonymize → CSV encode → file write → done transition. Every status update includes both `tenantID` and `jobID`. On failure, truncate stored error to 1,024 bytes, mark failed, and return the original error for retry. Malformed/tenantless payload returns `domain.ErrPermanentPayload` and never touches a job; the module's asynq adapter in Task 10 translates that error to `asynq.SkipRetry`, keeping application code independent of asynq.

- [ ] **Step 5: Run focused tests**

Run: `go test ./internal/modules/conversation/queryhistory/application -run 'Export|Worker' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit export application logic**

```bash
git add internal/modules/conversation/queryhistory/application
git commit -m "refactor(queryhistory): move export workflow"
```

---

### Task 7: Implement Legacy and Platform Adapters

**Files:**
- Create: `internal/modules/conversation/queryhistory/adapters/legacy_audit.go`
- Create: `internal/modules/conversation/queryhistory/adapters/tenant_policy.go`
- Create: `internal/modules/conversation/queryhistory/adapters/gorm_export.go`
- Create: `internal/modules/conversation/queryhistory/adapters/platform.go`
- Create: `internal/application/repository/session_audit.go`
- Test: `internal/application/repository/session_audit_test.go`
- Test: `internal/modules/conversation/queryhistory/adapters/legacy_audit_test.go`
- Test: `internal/modules/conversation/queryhistory/adapters/tenant_policy_test.go`
- Test: `internal/modules/conversation/queryhistory/adapters/gorm_export_test.go`
- Test: `internal/modules/conversation/queryhistory/adapters/platform_test.go`

**Interfaces:**
- Consumes: existing Session/Message/Feedback/Tenant repositories, `interfaces.FileService`, `interfaces.TaskEnqueuer`, GORM DB, asynq.
- Produces: implementations of every Task 4 port; narrow legacy `SessionAuditRepository` used only by the adapter; `ErrExportJobNotFound` maps missing and cross-tenant jobs to the same existing 404 AppError.

- [ ] **Step 1: Write failing policy and audit adapter tests**

Port the current snapshot repository tests. Assert tenant scope is applied before Session lookup, feedback/messages are capped exactly as before, nil/missing Tenant config normalizes to normal, and a foreign session is not distinguishable from missing.

- [ ] **Step 2: Write failing GORM export adapter tests**

Port CRUD, aggregation/filters, source classification, 10,000-row cap, and cross-tenant not-found tests. Run against SQLite; preserve PostgreSQL-specific SQL branching.

- [ ] **Step 3: Write failing queue/file adapter tests**

Assert queue adapter creates `types.TypeQueryHistoryExport`, `types.QueueMaintenance`, `MaxRetry(3)`, and `Timeout(10*time.Minute)`. Assert file adapter preserves the current generated path convention and returns the original reader.

- [ ] **Step 4: Run adapter tests and verify RED**

Run: `go test ./internal/modules/conversation/queryhistory/adapters -count=1 -v`

Expected: FAIL because adapters do not exist.

- [ ] **Step 5: Implement adapters without duplicating Session audit SQL**

Move `applySessionAuditFilters`, snapshot assembly, and `ExportSessionRows` into `internal/application/repository/session_audit.go` behind this temporary Wave 5 interface:

```go
type SessionAuditRepository interface {
    Snapshot(context.Context, uint64, string) (*types.QueryHistorySnapshot, error)
    ExportRows(context.Context, uint64, domain.ExportFilter) ([]domain.ExportRow, error)
}
```

The legacy Session listing delegates to the same private filter builder in `session_audit.go`; it does not import the new module. `adapters.LegacyAudit` wraps `SessionAuditRepository` and satisfies `ports.AuditReader`. There is one predicate implementation, no raw `*gorm.DB` crosses the port, and the manifest records `adapters -> internal/application/repository` for removal in Wave 5.

- [ ] **Step 6: Run adapter and legacy repository tests**

```bash
go test ./internal/modules/conversation/queryhistory/adapters -count=1
go test ./internal/application/repository -run 'QueryHistory|SessionAudit' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit adapters**

```bash
git add internal/modules/conversation/queryhistory/adapters internal/application/repository
git commit -m "refactor(queryhistory): add persistence and platform adapters"
```

---

### Task 8: Move HTTP Transport and Preserve the API Contract

**Files:**
- Create: `internal/modules/conversation/queryhistory/transport/http/handler.go`
- Create: `internal/modules/conversation/queryhistory/transport/http/routes.go`
- Test: `internal/modules/conversation/queryhistory/transport/http/handler_test.go`
- Test: `internal/modules/conversation/queryhistory/transport/http/routes_test.go`

**Interfaces:**
- Consumes: Query History audit/export application interfaces.
- Produces: `NewHandler(audit AuditUseCases, exports ExportUseCases) *Handler`; `RegisterRoutes(group *gin.RouterGroup, h *Handler)` with four existing relative routes.

- [ ] **Step 1: Write failing route compatibility test**

Register on a test group `/api/v1/admin/sessions` and assert exactly:

```text
GET  /api/v1/admin/sessions/:session_id/snapshot
POST /api/v1/admin/sessions/export
GET  /api/v1/admin/sessions/export/:job_id/status
GET  /api/v1/admin/sessions/export/:job_id/download
```

Assert a second registration panics/fails in the test composition layer rather than shadowing a route.

- [ ] **Step 2: Write failing handler contract tests**

Port valid snapshot, empty session ID, missing tenant, invalid times, invalid feedback, invalid job ID, disabled policy, missing/foreign job, not-ready download, storage failure, completed CSV download, UTF-8 BOM, Content-Type, and Content-Disposition tests.

- [ ] **Step 3: Run tests and verify RED**

Run: `go test ./internal/modules/conversation/queryhistory/transport/http -count=1 -v`

Expected: FAIL because transport does not exist.

- [ ] **Step 4: Implement transport only**

Move request parsing and response mapping from `internal/handler/session/query_history_admin.go`. Do not log or expose raw tenant/job IDs beyond existing structured logging behavior. `Open` returns bytes without BOM; handler writes BOM once before `io.Copy`.

- [ ] **Step 5: Run transport and existing router tests**

```bash
go test ./internal/modules/conversation/queryhistory/transport/http -count=1
go test ./internal/router -run QueryHistory -count=1
```

Expected: new transport PASS; router characterization remains PASS before switch.

- [ ] **Step 6: Commit transport**

```bash
git add internal/modules/conversation/queryhistory/transport/http
git commit -m "refactor(queryhistory): move HTTP transport"
```

---

### Task 9: Add the Old-vs-New Differential Compatibility Gate

**Files:**
- Create: `internal/modules/conversation/queryhistory/testkit/observation.go`
- Test: `internal/modules/conversation/queryhistory/testkit/observation_test.go`
- Test: `internal/handler/session/query_history_differential_test.go`
- Test: `internal/application/service/query_history_differential_test.go`
- Test: `internal/application/repository/query_history_differential_test.go`
- Modify: `docs/architecture/backend-baseline.md`

**Interfaces:**
- Consumes: untouched legacy Query History handler/service/repository plus the new domain/application/adapters/transport from Tasks 4–8.
- Produces: `testkit.Observation`, `testkit.NormalizeHTTP`, `testkit.NormalizeError`, and `testkit.Compare`; an automated gate proving the two implementations produce identical observable results before production wiring changes.

- [ ] **Step 1: Write failing observation-normalizer tests**

Define the comparison record explicitly:

```go
type Observation struct {
    Status       int
    Headers      map[string][]string
    Body         []byte
    ErrorClass   string
    Jobs         []JobObservation
    StoredFiles  map[string][]byte
    Enqueued     []TaskObservation
}
```

Tests must prove JSON object key order is normalized while JSON array order is preserved; `Date`, request ID, trace ID, and generated transport timing headers are removed; CSV and non-JSON bodies remain byte-exact; AppError status/code/message remain significant; job/file/task observations sort only by their stable IDs.

- [ ] **Step 2: Run the normalizer tests and verify RED**

Run: `go test ./internal/modules/conversation/queryhistory/testkit -count=1 -v`

Expected: FAIL because the testkit does not exist.

- [ ] **Step 3: Implement the deterministic comparison testkit**

`Compare(want, got)` returns a field-specific error and never ignores a mismatch. The only normalization allowlist is the four transport-only headers named in Step 1. Do not normalize user IDs, timestamps stored in domain records, CSV ordering, errors, file paths, task options, or job transitions.

- [ ] **Step 4: Add HTTP differential scenarios**

In `internal/handler/session/query_history_differential_test.go`, the test package can construct the legacy handler using its private fields and independently construct the new transport with fake ports. For each scenario, send the same `httptest` request and compare normalized observations:

```text
snapshot normal
snapshot anonymized
snapshot disabled
snapshot missing and foreign tenant
export empty body
export filtered by user/time/feedback
invalid time, feedback, and job ID
status pending/running/done/failed
download not ready, missing file, and completed CSV
```

Use fixed Tenant/User IDs and a fixed clock. Completed downloads compare status, headers, BOM, filename, and all CSV bytes.

- [ ] **Step 5: Add application/worker differential scenarios**

In `internal/application/service/query_history_differential_test.go`, run the legacy export service and new export application against separate but identical fake stores. Compare:

```text
created pending job
enqueued task type/queue/max-retry/timeout/payload
enqueue failure transition
normal and anonymized CSV bytes
disabled-after-enqueue failure with no file write
storage failure and 1,024-byte stored error truncation
done-job idempotent rerun
missing job drop
malformed and tenantless payload permanent failure
```

Normalize the legacy `asynq.SkipRetry` result and new `domain.ErrPermanentPayload` to the same semantic error class only in `NormalizeError`; all other error text and wrapping remain significant.

- [ ] **Step 6: Add repository differential scenarios**

In external package `repository_test`, initialize two independent in-memory SQLite databases with the same fixtures. Run the legacy repository against one and the new adapter against the other. Compare job CRUD, tenant-scoped missing/foreign reads, source classification, user/time/feedback filters, chronological tie-breaking, counts, soft-delete exclusion, skill-maintenance exclusion, and the 10,000-row cap.

- [ ] **Step 7: Run the complete differential gate**

```bash
go test ./internal/handler/session -run QueryHistoryDifferential -count=1
go test ./internal/application/service -run QueryHistoryDifferential -count=1
go test ./internal/application/repository -run QueryHistoryDifferential -count=1
```

Expected: PASS with zero ignored mismatches. Any mismatch blocks Task 10; update the new implementation, not the expected observation, unless the approved Spec explicitly changes behavior.

- [ ] **Step 8: Prove the gate detects drift**

The testkit unit test feeds observations differing in status, anonymized user ID, CSV column order, task retry count, and tenant-scoped job transition. Assert each comparison fails with the field name. This mutation-style check prevents an over-normalizing comparator from producing false confidence.

- [ ] **Step 9: Record and commit the gate**

Append the three differential commands and passing scenario counts to `docs/architecture/backend-baseline.md`.

```bash
git add internal/modules/conversation/queryhistory/testkit internal/handler/session/query_history_differential_test.go internal/application/service/query_history_differential_test.go internal/application/repository/query_history_differential_test.go docs/architecture/backend-baseline.md
git commit -m "test(queryhistory): add old-new differential gate"
```

---

### Task 10: Assemble the Module and Switch Production Wiring

**Files:**
- Create: `internal/modules/conversation/queryhistory/module.go`
- Test: `internal/modules/conversation/queryhistory/module_test.go`
- Create: `internal/modules/conversation/queryhistory/README.md`
- Modify: `internal/container/container.go`
- Modify: `internal/router/routes_query_history.go`
- Modify: `internal/router/router.go`
- Modify: `internal/router/task.go`
- Modify: `internal/router/sync_task.go`
- Test: `internal/router/routes_query_history_test.go`
- Test: `internal/router/task_processing_scope_test.go`
- Test: `internal/router/sync_task_retry_test.go`

**Interfaces:**
- Consumes: Tasks 3–9; production switching is forbidden until every Task 9 differential scenario passes.
- Produces: `queryhistory.NewModule(queryhistory.Dependencies) *queryhistory.Module`; `(*Module).RegisterRoutes(*gin.RouterGroup)`; `(*Module).RegisterWorkers(bootstrap.WorkerRegistry) error`; a private asynq handler that passes `task.Payload()` to application and maps `domain.ErrPermanentPayload` to `asynq.SkipRetry`.

- [ ] **Step 1: Write failing module tests**

Construct one module with fake ports. Assert it registers four routes and one worker, rejects duplicate worker registration, routes call the injected application service, and no package-level state is required.

- [ ] **Step 2: Run module tests and verify RED**

Run: `go test ./internal/modules/conversation/queryhistory -count=1 -v`

Expected: FAIL because `Module` does not exist.

- [ ] **Step 3: Implement module assembly and README**

`Dependencies` contains only ports and a logger-compatible boundary; it does not contain `*dig.Container`. README lists owned routes/task/table, non-owned Session/Tenant data, dependency ports, both worker modes, and both Wave 5 legacy exceptions.

- [ ] **Step 4: Switch the RBAC bridge**

Change `RegisterQueryHistoryAdminRoutes` to accept `*queryhistory.Module`, create the exact existing guarded group with `g.Admin()` and `apiKeyFullAccess()`, then call `module.RegisterRoutes(admin)`. `router.RouterParams` receives the module; it no longer routes Query History through `session.Handler`.

- [ ] **Step 5: Switch Redis and Lite workers**

Replace `QueryHistoryExport *service.QueryHistoryExportService` in both task parameter structs with `QueryHistory *queryhistory.Module`. Create the asynq registry around each mux, and call `RegisterWorkers`; call the same method on `SyncTaskExecutor`. Propagate registration errors during startup; do not log-and-continue.

- [ ] **Step 6: Switch container construction**

Replace legacy Query History repo/service providers with adapter providers plus one `queryhistory.NewModule`. Put the provider after TaskEnqueuer/FileService/Tenant/Session dependencies exist, preserving the documented dig ordering constraints. Do not add `Invoke`-time global registration.

- [ ] **Step 7: Run focused wiring and parity tests**

```bash
go test ./internal/modules/conversation/queryhistory/... -count=1
go test ./internal/router -run 'QueryHistory|SyncTask|TaskProcessing' -count=1
go test ./internal/container -run 'QueryHistory|BuildContainer' -count=1
make check-backend-architecture
```

Expected: PASS. The guard shows the route/worker assets at their new target and Redis/Lite parity intact.

- [ ] **Step 8: Commit production switch**

```bash
git add internal/modules/conversation/queryhistory internal/container/container.go internal/router docs/architecture/backend-modules.yaml
git commit -m "refactor(queryhistory): switch production to conversation module"
```

---

### Task 11: Remove Legacy Query History Paths and Verify Wave 0–1

**Files:**
- Delete: `internal/handler/session/query_history_admin.go`
- Delete: `internal/handler/session/query_history_admin_test.go`
- Delete: `internal/handler/session/query_history_export_test.go`
- Delete: `internal/application/service/query_history_policy.go`
- Delete: `internal/application/service/query_history_policy_test.go`
- Delete: `internal/application/service/query_history_export.go`
- Delete: `internal/application/service/query_history_export_test.go`
- Delete: `internal/application/service/query_history_snapshot_test.go`
- Delete: `internal/application/repository/query_history_export.go`
- Delete: `internal/application/repository/query_history_export_test.go`
- Delete: `internal/types/interfaces/query_history_export.go`
- Delete: `internal/handler/session/query_history_differential_test.go`
- Delete: `internal/application/service/query_history_differential_test.go`
- Delete: `internal/application/repository/query_history_differential_test.go`
- Delete: `internal/modules/conversation/queryhistory/testkit/observation.go`
- Delete: `internal/modules/conversation/queryhistory/testkit/observation_test.go`
- Modify: `internal/application/service/session.go`
- Modify: `internal/handler/session/handler.go`
- Modify: `docs/architecture/backend-modules.yaml`
- Modify: `docs/architecture/backend-baseline.md`

**Interfaces:**
- Consumes: production module from Task 10 and passing differential evidence from Task 9.
- Produces: zero runtime imports of deleted legacy Query History service/repository/handler; final Wave 0–1 evidence and clean guard.

- [ ] **Step 1: Re-run the differential gate after production switching**

```bash
go test ./internal/handler/session -run QueryHistoryDifferential -count=1
go test ./internal/application/service -run QueryHistoryDifferential -count=1
go test ./internal/application/repository -run QueryHistoryDifferential -count=1
```

Expected: PASS. This proves the wiring changes in Task 10 did not change the application, repository, or HTTP observations before the legacy comparator is removed.

- [ ] **Step 2: Prove production no longer imports legacy Query History symbols**

Run:

```bash
rg -n 'QueryHistoryExportService|NewQueryHistoryExportService|QueryHistoryExportJobRepository|GetQueryHistorySnapshot|queryHistoryExport' internal --glob '*.go'
```

Expected: only intentional module/compatibility results. Any old handler/service/repository consumer must be migrated before deletion.

- [ ] **Step 3: Move remaining snapshot code and delete legacy files**

Remove `sessionService.GetQueryHistorySnapshot` and the Query History fields/constructor argument from `session.Handler`. Delete old tests only after their assertions exist under the new module. Remove the old repository interface after adapters use module ports. Delete the three differential test files and their Query History testkit only after Step 1 passes; permanent new-module tests retain every scenario, while Task 9's commit and baseline document retain the old-vs-new evidence.

- [ ] **Step 4: Run formatting and focused tests**

```bash
gofmt -w internal/bootstrap internal/modules/conversation/queryhistory internal/router internal/container internal/types internal/application
go test ./internal/modules/conversation/queryhistory/... ./internal/router ./internal/container ./internal/types ./internal/application/repository ./internal/application/service -count=1 -timeout=15m
```

Expected: PASS.

- [ ] **Step 5: Run architecture and full server verification**

```bash
make check-backend-architecture
go build ./...
go test ./internal/... -count=1 -timeout=25m
ARCH_BASE_SHA=$(sed -n 's/^base_sha: //p' docs/architecture/backend-baseline.md | head -1)
test "${#ARCH_BASE_SHA}" -eq 40
golangci-lint run --new-from-rev="$ARCH_BASE_SHA" ./...
```

Expected: guard/build PASS; tests and lint PASS or match only failures documented in Task 1 baseline with identical signatures. New failures are blockers.

- [ ] **Step 6: Verify scope and external contract**

```bash
ARCH_BASE_SHA=$(sed -n 's/^base_sha: //p' docs/architecture/backend-baseline.md | head -1)
test "${#ARCH_BASE_SHA}" -eq 40
git diff --name-only "$ARCH_BASE_SHA"...HEAD
git diff --check "$ARCH_BASE_SHA"...HEAD
git diff "$ARCH_BASE_SHA"...HEAD -- cmd/desktop docreader client
```

Expected: last command has no output; diff check is clean; changed files match this plan.

- [ ] **Step 7: Update final evidence**

Append commands, results, known-baseline comparison, final route/worker counts, and the two remaining Wave 5 exceptions to `docs/architecture/backend-baseline.md`. Mark Query History assets as migrated in the YAML; do not mark other modules complete.

- [ ] **Step 8: Commit cleanup and evidence**

```bash
git add -A internal docs/architecture
git commit -m "refactor(queryhistory): complete first backend module slice"
```

- [ ] **Step 9: Request independent review**

Review must check Spec compliance, route/worker parity, tenant isolation, disabled-after-enqueue behavior, compatibility aliases, exception scope, no hidden globals, no unrelated refactor, and the final diff against the recorded `base_sha`.

---

## Plan Self-Review Record

- **Spec coverage:** Wave 0 inventory/guard is Tasks 1–2; composition contract is Task 3; Query History domain/application/adapters/transport is Tasks 4–8; old-vs-new equivalence is Task 9; production wiring is Task 10; old-path removal and evidence is Task 11. Wave 2–6 are deliberately excluded and require later plans after the pilot review.
- **Placeholder scan:** The plan contains no implementation placeholders; every task names exact files, interfaces, commands, expected results, and commit boundaries.
- **Type consistency:** `bootstrap.TaskHandler` and `WorkerRegistry.Register` are defined in Task 3 and used unchanged in Task 10. Query History domain and ports are defined in Task 4 and consumed unchanged by Tasks 5–10. `testkit.Observation` exists only for Task 9 differential evidence and is removed after the post-switch rerun in Task 11.
- **Review Focus coverage:** Duplicate registration is tested in Tasks 3/8/10; cross-tenant 404 in Tasks 7–9; disabled-after-enqueue and malformed payload in Tasks 6/9; Redis/Lite parity in Tasks 2/9/10. Task 9 directly compares old and new outputs and Task 11 reruns the comparison after wiring changes.
- **Scope check:** This plan yields independently testable software: architecture ownership is enforced and one complete production feature slice runs through the new module boundary. No later module migration is required for this deliverable to function.
