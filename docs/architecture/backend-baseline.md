# Backend Modularization — Pre-Migration Baseline (Wave 0)

base_sha: 20d2350dce5e4ce97f7f3e8a6a1bea7f91228b2a

This document records the execution baseline for the backend modularization
program (Spec: `docs/specs/2026-09-21-backend-domain-module-reorganization-design.md`).
It was captured in a clean worktree on branch `sdd-bm-t1` whose HEAD is a
current descendant of the approved Spec commit `597c642c1`. No code was changed.
The companion inventory is `docs/architecture/backend-modules.yaml`.

Environment: go1.26.3 darwin/arm64 (macOS, Apple Silicon).

## 1. Baseline commands and results

Exit codes were captured by redirecting output to a file and checking `$?`
explicitly; no pipe status was trusted.

### 1.1 HEAD SHA

Command:

```bash
ARCH_BASE_SHA=$(git rev-parse HEAD)
echo "$ARCH_BASE_SHA"
git rev-parse HEAD
```

Output (exit 0):

```
ARCH_BASE_SHA=20d2350dce5e4ce97f7f3e8a6a1bea7f91228b2a
20d2350dce5e4ce97f7f3e8a6a1bea7f91228b2a
```

Worktree status at capture time: clean (`nothing to commit, working tree clean`).

### 1.2 go build ./...

Command: `go build ./...` — **exit 0 (PASS)**.

Full output (warnings only, no errors):

```
# github.com/Tencent/WeKnora/cmd/server
ld: warning: ignoring duplicate libraries: '-lc++'
# github.com/Tencent/WeKnora/cmd/desktop
ld: warning: ignoring duplicate libraries: '-lc++'
```

The two `ld` warnings are macOS linker warnings about duplicate `-lc++` and do
not affect build success.

### 1.3 go test ./internal/... -count=1 -timeout=25m

Command: `go test ./internal/... -count=1 -timeout=25m` — **exit 1**.

Result summary (full output captured to a file; per-package results below):

- 127 Go packages under `./internal/...` (per `go list ./internal/...`)
- 117 packages: `ok`
- 9 packages: no test files
- 1 package: **FAIL** — pre-existing known failure, first actionable error below

Failing test (the only failure in the suite):

- Package: `github.com/Tencent/WeKnora/internal/agent/recoverytest`
- Test: `TestCrashMatrixSQLite` (167.25s)
- Subtest: `TestCrashMatrixSQLite/unknown_result_user_retry` (126.29s)
- First actionable error:

```
recovery provider: wait after retry: run did not complete after retry, status=waiting_user wait_reason=call-matrix-1
--- FAIL: TestCrashMatrixSQLite (167.25s)
    --- FAIL: TestCrashMatrixSQLite/unknown_result_user_retry (126.29s)
        matrix_test.go:125: resume provider after "unknown_result_user_retry": exit status 1
```

Interpretation: the crash-matrix harness resumes a helper provider process
after an injected crash in the `unknown_result_user_retry` scenario; the
provider subprocess exits with status 1, so `matrix_test.go:125` reports the
failure. This failure executed and failed on its own (not an environment
blocker): it is recorded as a **known failure in the pre-migration ledger**,
not as PASS and not as blocked-env.

Slowest packages (wall time): `internal/application/repository` 382.801s,
`internal/application/service` 326.107s, `internal/handler/session` 102.255s,
`internal/application/service/workbench` 99.387s,
`internal/infrastructure/commercialplatform` 70.556s.

No test hit the 25m package timeout; no `blocked-env` condition was hit (no
test was skipped for missing external services in this run).

Per-package results:

| Package | Result |
|---|---|
| internal/agent | ok (5.057s) |
| internal/agent/approval | ok (6.110s) |
| internal/agent/compaction | ok (4.967s) |
| internal/agent/experts | ok (3.855s) |
| internal/agent/native | ok (5.265s) |
| internal/agent/nativecontract | ok (3.148s) |
| internal/agent/nativeprobe | ok (3.207s) |
| internal/agent/opencode | ok (39.017s) |
| internal/agent/persona | ok (3.076s) |
| internal/agent/recoverytest | FAIL (200.536s) |
| internal/agent/recoverytest/provider | ok (1.016s) |
| internal/agent/runtime | ok (0.885s) |
| internal/agent/skills | ok (2.444s) |
| internal/agent/skills/skillhub | ok (0.885s) |
| internal/agent/subagents | ok (1.130s) |
| internal/agent/token | ok (1.029s) |
| internal/agent/tools | ok (16.639s) |
| internal/agent/trpc | ok (1.427s) |
| internal/appconnector | ok (0.134s) |
| internal/appconnector/openconnector | ok (0.217s) |
| internal/application/access | ok (1.116s) |
| internal/application/repository | ok (382.801s) |
| internal/application/repository/appconnector | ok (0.422s) |
| internal/application/repository/commercial | ok (0.263s) |
| internal/application/repository/retriever/doris | ok (0.897s) |
| internal/application/repository/retriever/elasticsearch | no test files |
| internal/application/repository/retriever/elasticsearch/v7 | ok (1.099s) |
| internal/application/repository/retriever/elasticsearch/v8 | ok (1.254s) |
| internal/application/repository/retriever/milvus | ok (1.165s) |
| internal/application/repository/retriever/neo4j | no test files |
| internal/application/repository/retriever/opensearch | ok (1.056s) |
| internal/application/repository/retriever/postgres | no test files |
| internal/application/repository/retriever/qdrant | ok (1.263s) |
| internal/application/repository/retriever/sqlite | ok (1.278s) |
| internal/application/repository/retriever/tencentvectordb | ok (2.144s) |
| internal/application/repository/retriever/weaviate | ok (4.404s) |
| internal/application/service | ok (326.107s) |
| internal/application/service/appconnector | ok (5.019s) |
| internal/application/service/chat_pipeline | ok (2.468s) |
| internal/application/service/commercial | ok (1.973s) |
| internal/application/service/file | ok (3.153s) |
| internal/application/service/memory | ok (7.538s) |
| internal/application/service/metric | ok (1.677s) |
| internal/application/service/retriever | ok (1.758s) |
| internal/application/service/workbench | ok (99.387s) |
| internal/assets | no test files |
| internal/browserskill | ok (0.404s) |
| internal/commercial | ok (0.101s) |
| internal/common | ok (0.984s) |
| internal/common/redislock | ok (0.293s) |
| internal/config | ok (1.118s) |
| internal/connectorcontrol | ok (3.646s) |
| internal/container | ok (7.915s) |
| internal/craft | ok (2.728s) |
| internal/database | ok (30.589s) |
| internal/datasource | ok (5.341s) |
| internal/datasource/connector/confluence | ok (1.253s) |
| internal/datasource/connector/dingtalk | ok (1.270s) |
| internal/datasource/connector/feishu/core | ok (4.478s) |
| internal/datasource/connector/feishu/drive | ok (3.865s) |
| internal/datasource/connector/feishu/wiki | ok (14.032s) |
| internal/datasource/connector/gitlab | ok (2.689s) |
| internal/datasource/connector/ima | ok (1.588s) |
| internal/datasource/connector/moauth | ok (2.452s) |
| internal/datasource/connector/notion | ok (3.307s) |
| internal/datasource/connector/rss | ok (2.612s) |
| internal/datasource/connector/yuque | ok (10.392s) |
| internal/embedpolicy | ok (0.397s) |
| internal/errors | no test files |
| internal/event | ok (1.696s) |
| internal/execution | ok (1.717s) |
| internal/filetransport | ok (1.052s) |
| internal/handler | ok (4.495s) |
| internal/handler/dto | ok (2.401s) |
| internal/handler/session | ok (102.255s) |
| internal/im | ok (4.537s) |
| internal/im/dingtalk | ok (3.253s) |
| internal/im/feishu | ok (3.759s) |
| internal/im/mattermost | ok (3.384s) |
| internal/im/qqbot | ok (2.573s) |
| internal/im/slack | ok (1.890s) |
| internal/im/telegram | ok (1.872s) |
| internal/im/wechat | no test files |
| internal/im/wecom | ok (2.362s) |
| internal/im/yunzhijia | ok (2.225s) |
| internal/infrastructure/chunker | ok (2.235s) |
| internal/infrastructure/commercialplatform | ok (70.556s) |
| internal/infrastructure/docparser | ok (2.334s) |
| internal/infrastructure/docparser/anydoc | ok (0.430s) |
| internal/infrastructure/openmeter | ok (0.546s) |
| internal/infrastructure/semantic | ok (4.185s) |
| internal/infrastructure/web_fetch | ok (1.555s) |
| internal/infrastructure/web_search | ok (1.827s) |
| internal/ipclass | ok (0.377s) |
| internal/logger | ok (1.981s) |
| internal/mcp | ok (1.875s) |
| internal/metrics | ok (0.418s) |
| internal/middleware | ok (5.740s) |
| internal/middleware/asynqdl | ok (6.084s) |
| internal/modelcontext | ok (4.888s) |
| internal/models/asr | ok (3.319s) |
| internal/models/chat | ok (3.498s) |
| internal/models/embedding | ok (4.726s) |
| internal/models/limiter | ok (6.092s) |
| internal/models/provider | ok (6.433s) |
| internal/models/rerank | ok (3.114s) |
| internal/models/utils | no test files |
| internal/models/utils/ollama | no test files |
| internal/models/vlm | ok (2.634s) |
| internal/notification | ok (0.621s) |
| internal/payment | ok (20.675s) |
| internal/ratelimit | ok (0.518s) |
| internal/router | ok (6.283s) |
| internal/runtime | ok (3.143s) |
| internal/sandbox | ok (8.578s) |
| internal/searchutil | ok (3.266s) |
| internal/storageallowlist | ok (0.677s) |
| internal/storageurl | ok (2.994s) |
| internal/stream | ok (2.485s) |
| internal/textconv | ok (0.445s) |
| internal/tracing/langfuse | ok (2.445s) |
| internal/types | ok (3.677s) |
| internal/types/interfaces | no test files |
| internal/usage | ok (0.431s) |
| internal/utils | ok (0.556s) |
| internal/voice | ok (0.808s) |
| internal/workbench | ok (0.613s) |

### 1.4 go test ./tools/... -count=1

Command: `go test ./tools/... -count=1` — **exit 1**, output verbatim:

```
go: warning: "./tools/..." matched no packages
no packages to test
```

Recorded as-is (not a test failure): `tools/` exists but contains only
standalone nested Go modules — `tools/trpc-native-p1/sessionprobe`,
`tools/trpc-native-p1/memoryprobe`, `tools/trpc-native-p1/identityprobe` —
each with its own `go.mod`, so the main module's `./tools/...` pattern matches
no packages. These probe modules are outside `scope.include` of the inventory.

## 2. Asset inventory counts (backend-modules.yaml)

549 asset entries in total, by kind:

| Kind | Count | Discovery source |
|---|---|---|
| route | 72 | 69 `Register*Routes` calls in `internal/router/router.go` + 3 indirect craft registrations reachable via `RegisterSessionRoutes` (`internal/router/routes_chat.go` calls `session.RegisterCraftSessionRoutes`, `session.RegisterCraftInteractionRoutes`, `session.RegisterCraftScheduledTaskRoutes`) |
| worker | 23 | 23 distinct task types registered in `internal/router/task.go` (`mux.HandleFunc`) and the same 23 in `internal/router/sync_task.go` (`SyncTaskExecutor.RegisterHandler`); one worker asset per task type, source recorded as `internal/router/task.go` |
| lifecycle | 58 | 58 `container.Invoke` calls under `internal/container` (47 named functions + 11 inline closures) |
| migration | 267 | 173 `.up/.down` pairs in `migrations/versioned` (PostgreSQL) + 94 pairs in `migrations/sqlite` (SQLite) |
| package | 129 | 127 packages from `go list ./internal/...` + `cmd/server` (per `scope.include`) |

Assets by owner: knowledge 76, conversation 53, airesource 48, agentruntime 43,
execution 40, commercial 39, craft 38, agentcatalog 37, platform 33,
workbench 30, appconnector 26, channels 23, identity 23, datasource 21,
policy 7, system 7, insights 5.

## 3. Coverage check (Step 5 commands and output)

Commands run exactly as specified in the task brief; output captured to files.
Cross-check result: every match has an asset entry in
`docs/architecture/backend-modules.yaml` (verified programmatically against the
rg outputs; 0 unmatched).

### 3.1 rg -n 'Register[A-Za-z0-9]+Routes\(' internal/router/router.go

69 matching lines. All 69 symbols have `kind: route` asset entries. The three
additional craft route functions (defined in `internal/handler/session/`) are
not called in `router.go` directly but are reachable from it through
`RegisterSessionRoutes` (`internal/router/routes_chat.go:202,208,230`); they
are included as route assets (72 total). Note:
`handlerapi.RegisterArtifactPreviewIssueRoute` (`routes_chat.go:93`, singular
"Route") is a sub-registration inside `RegisterSessionRoutes` and is covered by
the `conversation.session-routes` / `craft.artifact-preview-routes` entries.

### 3.2 rg -n 'RegisterHandler\(|HandleFunc\(' internal/router/task.go internal/router/sync_task.go

47 matching lines = 23 `mux.HandleFunc` registrations in `task.go` +
23 `Executor.RegisterHandler` registrations in `sync_task.go` + 1 method
definition (`func (e *SyncTaskExecutor) RegisterHandler(...)` itself).
The 23 distinct `types.Type*` task types all have `kind: worker` asset
entries. The same task type is registered on both executors (asynq mux and
Lite-mode SyncTaskExecutor); the inventory records one worker per task type.

### 3.3 rg -n 'container\.Invoke\(' internal/container

58 matching lines, all in `internal/container/container.go` except the ones
whose invoked symbol is defined elsewhere (`StartExecutionCleanupSweep` in
`internal/container/cleanup.go`, `recoverPendingWikiTasks` in
`internal/container/recover_pending_wiki_tasks.go`). All 58 have
`kind: lifecycle` asset entries (47 named function references + 11 inline
closures recorded with `inline:`-prefixed symbol descriptions).

### 3.4 find internal/database -type f | sort

6 files:

```
internal/database/migration_sqlite_test.go
internal/database/migration_sqlite_versioned_schema_test.go
internal/database/migration.go
internal/database/semantic_migration_pg_test.go
internal/database/semantic_migration_test.go
internal/database/workbench_migration_test.go
```

**Important factual correction to the task brief / plan:** there is no
`internal/database/migrations` directory in this repository. The live
golang-migrate source sets are at the repository root, selected by
`RunMigrationsWithOptions` in `internal/database/migration.go`:

- `file://migrations/versioned` — PostgreSQL (173 pairs, versions
  000000–000187 with 16 version numbers unused in this set)
- `file://migrations/sqlite` — SQLite (94 pairs, versions 000000–000109 with
  its own independent numbering; the two dialects assign different version
  numbers to the same logical change)

Auxiliary directories found and intentionally NOT inventoried as migration
assets (recorded here for honesty):

- `migrations/mysql/` — 1 file (`drop tables` script), no code references
- `migrations/paradedb/` — 2 init scripts, referenced only in a code comment
  (`internal/application/service/wiki_ingest.go:2305`)
- `semantic/migrations/` — 2 SQL files owned by the Python semantic service
  (separate deployment, outside the Go server scope)
- `docs/migrations/` — documentation only

## 4. Ownership decisions and ambiguity ledger

The spec §5 module map and §5.18 coverage matrix are the authority for module
IDs and ownership. The following judgment calls were made during
classification; each is flagged for confirmation during later waves (per spec
§5.18: omissions or one-to-many mappings require a design revision):

1. **Migration location** differs from the brief's assumption (see §3.4).
   Migration assets carry `target: internal/modules/<owner>/migrations` (or
   `internal/platform/migrations`) to record ownership; golang-migrate
   requires a single global ordered source URL, so the physical end-state
   layout (central directory with owner labels vs. per-module with a manifest)
   is a Task 2 / Wave 2 design decision. Targets record ownership, not
   physical layout.
2. **Cross-cutting initial migrations**: `000000_init` (both dialects) is the
   full initial schema spanning all domains — recorded under `platform`
   (migration runner owner) as custodian. `000001_agent` (versioned) creates
   `users`/`auth_tokens` (identity) plus `knowledge_tags`/`mcp_services` —
   recorded under `identity` (dominant). `000041_task_queue_and_wiki_indexes`
   (task queue tables + wiki indexes) — under `platform`.
3. **housekeeping**: spec §5.18 maps "housekeeping" to System, but the current
   implementation (`internal/application/service/knowledge_housekeeping.go`)
   sweeps stuck knowledge rows. `system.housekeeping-start` lifecycle asset
   and `internal/application/service` package ownership follow the code's
   knowledge-row scope only where noted; the lifecycle asset is recorded
   owner `system` per the spec matrix — flag for design confirmation.
4. **`000156_paseo_control` / sqlite `000077_paseo_control`**: creates
   `execution_workspace_leases` (Execution) and also alters
   `workbench_interactions` — recorded under `execution` (new table dominant).
5. **sqlite `000005_messages_attachments_and_invitation_fields`**: alters both
   `messages` (conversation) and `tenant_invitations` (identity) — recorded
   under `conversation`.
6. **Execution cleanup sweep lifecycle** (`craft.execution-cleanup-sweep-start`):
   O03/W33 craft lifecycle reclamation per the container comments, but it
   consumes `repository.AgentRunStore` (Agent Runtime data) — recorded under
   `craft`, mixed seam noted.
7. **Model concurrency governors** (`registerModelConcurrencyLimiter`,
   `registerLiteModelConcurrencyLimiter`, `internal/models/limiter`): recorded
   under `policy` per spec §5.16 (限流).
8. **Retriever driver packages** (`internal/application/repository/retriever/*`):
   recorded under `airesource` per spec §5.2 ("向量数据库驱动由 AI
   Resource/Platform 适配"); retrieval strategy (`internal/application/service/retriever`)
   stays `knowledge`.
9. **`internal/mcp`** (MCP client/manager/OAuth lifecycle): recorded under
   `airesource` per spec §5.18 matrix row for AI Resource ("MCP
   service/OAuth metadata"); MCP runtime tools remain Agent Runtime.
10. **Semantic family** (`RegisterSemanticInternalRoutes`,
    `RegisterSemanticModelPolicyRoutes`, `semantic_control`,
    `semantic_model_policy`, `semantic_model_invocations` migrations,
    `internal/infrastructure/semantic`): recorded under `knowledge` per the
    matrix ("semantic knowledge → Knowledge"); the routes are defined in
    `routes_knowledge.go`, which supports this reading.
11. **`RegisterKnowledgeBaseActivityRoutes`** is served by `AuditLogHandler`
    — recorded under `identity` (audit log family).
12. **`RegisterNativeArchiveRoutes`** (native protocol archive) — recorded
    under `agentruntime` (Native/tRPC/OpenCode protocols per spec §5.5).
13. **`RegisterWeKnoraCloudRoutes`** (cloud credentials/status) — recorded
    under `system` (deployment capability per spec §5.15).

### 4.1 Cross-cutting packages pending split

The spec matrix requires `types`, `interfaces`, `common`, `utils`, and `event`
to be attributed item by item ("按所有权拆分"); only pure technical primitives
may remain in Platform. These packages currently mix domains and have no
single dominant owner. They are recorded under `platform` as **current
custodian, not end-state owner**; each later wave must claim its slice:

| Package | Non-test files | Dominant families observed | Inventory owner (custodian) |
|---|---|---|---|
| internal/types | 101 | tenant, agent, knowledge, web, user, semantic (no majority) | platform |
| internal/types/interfaces | 60 | cross-domain ports | platform |
| internal/event | 5 | cross-domain events | platform |
| internal/common (+redislock) | 3 (+1) | technical | platform |
| internal/utils | 16 | technical | platform |
| internal/handler/dto | 8 | one file per domain | platform |
| internal/application/repository | 95 | agent_run (11), tenant (8), craft (6), knowledge (5), semantic (4) | agentruntime (dominant; split required) |
| internal/application/service | 182 | knowledge (32), tenant (25), craft (18), agent (15) | knowledge (dominant; split required) |
| internal/handler | 86 | tenant/auth (7), app (7), mcp (5) | identity (dominant; split required) |
| internal/handler/session | 36 | workbench (7), craft (5), session/chat core | conversation (session family; split required) |

Wave 6 deletes the global `handler/service/repository/types` paths; the
per-item attribution happens progressively in Waves 2–6.

## 5. Known-failure ledger (pre-existing)

| # | Package | Test | Error | Status |
|---|---|---|---|---|
| 1 | internal/agent/recoverytest | TestCrashMatrixSQLite/unknown_result_user_retry | `matrix_test.go:125: resume provider after "unknown_result_user_retry": exit status 1` (provider subprocess exited 1 after injected crash; preceding log: `run did not complete after retry, status=waiting_user wait_reason=call-matrix-1`) | Known failure at base_sha; must not be attributed to modularization work |

## 6. Reproduction

```bash
git -C <worktree> checkout 20d2350dce5e4ce97f7f3e8a6a1bea7f91228b2a
go build ./...
go test ./internal/... -count=1 -timeout=25m
go test ./tools/... -count=1
```

Expected: build PASS; internal tests 117 ok / 9 no-test-files /
1 known failure (`internal/agent/recoverytest`); tools "matched no packages".
