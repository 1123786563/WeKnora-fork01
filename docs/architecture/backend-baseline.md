base_sha: 7b1d3cf017a50cc8061da166d67befc6b51773a0

# Backend Baseline — Task F0 (Pass A)

- Branch: `backend-mod-passa` (worktree `.worktrees/bm-passa-main`)
- Recorded: 2026-09-21
- Go: 1.26.3; golangci-lint: 2.12.2 (`/opt/homebrew/bin/golangci-lint`)
- Scope: `cmd/server` + `internal/...` ONLY. `cmd/desktop`, `cmd/docreader`, `cmd/client`,
  `cmd/connector-control`, `cmd/download`, `cmd/saas-migrate` are out of scope and are NOT
  inventoried in `backend-modules.yaml`.
- Companion inventory: `docs/architecture/backend-modules.yaml`
- Environment at record time: postgres :5432, redis :6379, docreader :50051 running (dev compose).

## 1. Summary counts (diff targets for later tasks)

| Asset | Count | Source of truth |
|---|---|---|
| Server Go packages (`go list` ∩ cmd/server + internal/...) | 128 | §3.1 |
| Route registrations (gin method calls, non-test) | 542 (538 `internal/router` + 4 handler-side) | §3.2 |
| Route entry points (Register*/serve* functions) | 85 (67 Register* + 16 serve*/embedFrame* in router pkg + 2 handler pkg) | §3.2 |
| Redis-mode worker pools (asynq servers) | 6 (core, postprocess, enrichment, maintenance, shared, wiki) | §3.3 |
| Redis-mode worker task types (`mux.HandleFunc`) | 23 | §3.3 |
| Lite-mode worker task types (`SyncTaskExecutor`) | 23 (same set as Redis mode) | §3.3 |
| `container.Invoke` lifecycle hooks | 58 | §3.4 |
| Migration files | 537 (mysql 1, paradedb 2, sqlite 188, versioned 346) | §3.5 |
| Migration assets (up/down paired) | 270 (94 sqlite + 173 versioned pairs + 3 single files) | §3.5 |
| `go build ./...` | exit 0 | §2.1 |
| `go test ./internal/...` (per run) | run 1: 118 ok / 0 FAIL; run 2: 116 ok / 2 FAIL (flaky — see §2.4) | §2.2, §2.4 |
| `golangci-lint run --new-from-rev=HEAD~1` | exit 0, 0 findings | §2.3 |

## 2. Baseline command records

### 2.1 `go build ./...`

- Command: `go build ./...` (repo root, worktree bm-passa-main, base_sha above)
- Exit status: 0
- Output: two linker warnings only, no errors:
  - `# github.com/Tencent/WeKnora/cmd/desktop` / `# github.com/Tencent/WeKnora/cmd/server`:
    `ld: warning: ignoring duplicate libraries: '-lc++'`

### 2.2 `go test ./internal/... -count=1 -timeout=25m` (run twice)

- Command (verbatim, both runs): `go test ./internal/... -count=1 -timeout=25m`
- Run 1: exit 0. Results:
  - 118 packages `ok`
  - 0 packages `FAIL`
  - 9 packages `[no test files]` (118 + 9 = 127 internal packages):
    internal/application/repository/retriever/elasticsearch, .../neo4j, .../postgres,
    internal/assets, internal/errors, internal/im/wechat, internal/models/utils,
    internal/models/utils/ollama, internal/types/interfaces
  - Slowest packages: application/repository 269.5s, application/service 229.1s,
    handler/session 62.3s, infrastructure/commercialplatform 70.7s, agent/recoverytest 69.3s,
    service/workbench 37.5s, agent/opencode 31.4s
- Run 2: exit 1. Results:
  - 116 packages `ok`, **2 packages `FAIL`**, 9 `[no test files]` (same 9 as run 1)
  - Both failures are flaky (run 1 passed the same packages); details in §2.4.
- Run 2 was niced (background `nohup`, SN state) and overlapped heavy machine load
  (load average up to ~6); this is a plausible contributor to the opencode hang below, but
  per F0 resolution the differing signatures are recorded as unstable blockers regardless.
- Skips: this command does not use `-v`, so individual `t.Skip` outcomes are not visible in the
  output; nothing is printed for skipped tests and no package fails because of skips. For
  honesty: 65 test files under `internal/` contain `t.Skip` guards, typical reasons are
  missing external credentials/endpoints (FEISHU_APP_ID/SECRET, ALIPAY_SANDBOX_*, TRPC_RECOVERY_*,
  BROWSERSKILL_TEST_BINARY, DOCKER_INTEGRATION_IMAGE) — these tests SKIP, they are not PASSes.
  We deliberately do NOT claim them as green; they are environment-gated (`blocked-env` class).
  The two `Skip("blocked-dependency: W04 SQLite migration ...")` sites are pre-existing
  blocked-dependency skips in `internal/container` tests.

### 2.3 `golangci-lint run --new-from-rev=HEAD~1 ./...`

- Command (verbatim): `/opt/homebrew/bin/golangci-lint run --new-from-rev=HEAD~1 ./...`
- Exit status: 0
- Findings: `0 issues.`
- Context: `HEAD~1..HEAD` = commit 7b1d3cf01 ("docs: plan two-pass backend modularization
  foundation"), which changed 2 Markdown files under `docs/` and 0 `.go` files
  (`git diff --name-only HEAD~1..HEAD | grep -c '\.go$'` → 0). The empty findings result is
  therefore a legitimate baseline: the changed range contains no Go code. Non-fatal linter
  warnings during the run (not findings): one processor warning referencing a file path from a
  sibling worktree (`.worktrees/bm-t3/...`) via shared analysis state, and one
  `nolint_filter` note about an unknown linter name in an existing `//nolint` directive.

### 2.4 Flaky comparison (run 1 vs run 2)

Per F0 resolution: same failures in both runs = stable pre-existing; differing signatures =
unstable blockers. Run 1 had 0 FAILs; run 2 had 2 — **both are unstable blockers** (differing,
since run 1 passed the same packages):

1. `github.com/Tencent/WeKnora/internal/agent/opencode`
   - Run 1: `ok ... 31.440s`. Run 2: `FAIL ... 1502.280s`.
   - Error signature: `panic: test timed out after 25m0s` — the package's test binary hit the
     per-package 25m timeout. Goroutine dump shows goroutines blocked ~24 minutes on a
     `net/http` connection: `net/http.(*persistConn).readLoop` `[IO wait, 24 minutes]` and
     `net/http.(*persistConn).writeLoop` `[select, 24 minutes]` (a test talks to a local HTTP
     endpoint that never answers). Classification: **unstable blocker (environment/load-sensitive
     hang)** — needs attention before tasks that touch `internal/agent/opencode`.
2. `github.com/Tencent/WeKnora/internal/application/repository`
   - Run 1: `ok ... 269.458s`. Run 2: `FAIL ... 398.102s` with one failing test.
   - Error signature:
     `--- FAIL: TestAgentRunDecisionConcurrentOnlyOneRevision (7.81s)` —
     `internal/application/repository/agent_run_decisions_test.go:90: Not equal: expected: 2, actual: 1`.
     The test fires two concurrent `store.ApplyDecision` calls and requires both to succeed
     (the second as an idempotent replay, `require.Equal(t, 2, success)`); in run 2 only one
     succeeded, i.e. the replay path returned `ErrConflict` instead. Classification:
     **unstable blocker (genuine concurrency flake in decision-apply/replay race)**.
3. The other 116 packages report `ok` in both runs with no differing signatures → stable at
   package granularity (individual `-v`-level flakes inside passing packages are not visible
   at this verbosity).

### 2.5 Known failures register (F0-known failures)

No stable pre-existing failures (run 1: zero). The two §2.4 items are recorded as **F0-known
unstable blockers** (pre-existing on base_sha — the range up to this commit introduced no Go
changes; not to be fixed by F0, not blockers for F0 itself, but must be watched by later tasks
that touch `internal/agent/opencode` and `internal/application/repository` agent-run code):

| Test / package | Signature | Class |
|---|---|---|
| internal/agent/opencode (package) | `panic: test timed out after 25m0s`; goroutines stuck 24m on `net/http` persistConn read/write loops | unstable blocker (hang) |
| internal/application/repository TestAgentRunDecisionConcurrentOnlyOneRevision | `agent_run_decisions_test.go:90: Not equal: expected: 2, actual: 1` (concurrent ApplyDecision idempotent-replay returns ErrConflict) | unstable blocker (concurrency flake) |

Environment-gated skips are described in §2.2 (they skip, do not fail; recorded as
`blocked-env`, never counted as PASS).

## 3. Inventory enumerations (detail)

### 3.1 Packages

- `go list ./...` full repo → 132 packages under `cmd/` + `internal/`; after keeping only
  `cmd/server` + `internal/...` → **128 packages** (raw list preserved in this worktree at
  inventory time; regenerate with `go list ./cmd/server/... ./internal/...`).
- Ownership: every package has exactly one owner in `backend-modules.yaml` (16 business modules
  + `platform` + `bootstrap`). Package counts by owner:
  agentruntime 21, platform 20, knowledge 19, datasource 12, airesource 12, channels 10,
  commercial 7, bootstrap 6, policy 5, appconnector 5, workbench 4, execution 3,
  conversation 2, craft 1, insights 1, identity 0, agentcatalog 0, system 0.
  identity/agentcatalog/system own no dedicated package today — their code lives inside the
  horizontal packages (`internal/application/service`, `internal/handler`, ...) whose
  file-level split is deferred to Pass B (documented per-entry with `split:` notes).

### 3.2 Routes

- Total gin route registrations (`.GET/.POST/.PUT/.DELETE/.PATCH(` in non-test files):
  - `internal/router/*.go`: 538 — by method: GET 221, POST 201, PUT 53, DELETE 60, PATCH 3.
    (Line-based `grep -c` undercounts; the totals use occurrence-based `grep -o` and include
    2 chained registrations `X.With(...).GET/DELETE(...)` in routes_auth_tenant.go:252 and
    routes_knowledge.go:83.)
  - `internal/handler/session/craft_preview.go`: 2 — `sessions.POST(.../craft/versions/:version_id/preview)`
    (authenticated group, line 45) + `r.GET /p/:cap/*filepath` (W02 isolated preview origin, line 56)
  - `internal/handler/artifact_preview.go`: 2 — `sessions.POST(.../artifact-versions/:version_id/preview-ticket)`
    (authenticated group, line 275) + `r.GET /ap/:token` (W27 isolated preview origin, line 288)
  - **Total: 542** (method split across all: GET 223, POST 203, PUT 53, DELETE 60, PATCH 3)
- Route entry points: **85** = 67 `Register*` functions (HTTP) + 16 `serve*`/`embedFrame*`
  route-mounting helpers in `internal/router` (worker registration `RegisterSyncHandlers`
  excluded) + 2 handler-side `Register*` functions
  (`internal/handler/artifact_preview.go:284`, `internal/handler/session/craft_preview.go:55`).
- Special pre-auth groups (registered before global Auth middleware): IM callbacks, embed
  public routes, resource grants, sandbox terminal WS, workbench artifact grant download,
  local browser extension gateway, craft preview origin, artifact preview origin, craft model
  gateway, semantic internal route, presigned files.
- Per-file registration counts (non-test, `internal/router`, occurrence-based):
  routes_infra.go 103, routes_knowledge.go 92, routes_agent.go 88, routes_chat.go 58,
  routes_auth_tenant.go 53, routes_workbench.go 39, routes_commercial.go 20,
  routes_app_connectors.go 17, routes_memory.go 16, routes_native_archive.go 7,
  routes_skill_market.go 5, routes_persona.go 5, router.go 5, routes_query_history.go 4,
  routes_analytics.go 4, files.go 4, routes_usage.go 3, routes_tenant_skill_market.go 3,
  routes_tenant_expert_market.go 3, routes_expert.go 3, rbac.go 3, routes_subagent.go 2,
  routes_agent_marketplace.go 1. (task.go/sync_task.go and the remaining files register no
  HTTP routes.)

### 3.3 Workers

- Redis mode (`internal/router/task.go`, `RunAsynqServer`): one shared `asynq.ServeMux` with
  **23 `mux.HandleFunc` registrations**, run on **6 asynq server pools** (core, postprocess,
  enrichment, maintenance, shared, wiki; `runPool` at task.go:330-343). Plus 3 mux middlewares:
  asynqdl dead-letter (`MiddlewareWithCallback` + knowledge-failer callback),
  `backgroundTaskMiddleware`, `langfuse.AsynqMiddleware`.
- Lite mode (`internal/router/sync_task.go`, `RegisterSyncHandlers`): in-memory
  `SyncTaskExecutor` with the **same 23 task types** (no Redis). Mode selection:
  container.go:1044 (Redis) vs container.go:1046 (Lite).
- Task type → handler owner (both modes): knowledge 18 (ChunkExtract, DataTableSummary,
  DocumentProcess, ManualProcess, FAQImport, QuestionGeneration, SummaryGeneration, KBClone,
  KnowledgeMove, KnowledgeListDelete, KnowledgeListReparse, IndexDelete, KBDelete,
  ImageMultimodal, KnowledgePostProcess, KnowledgeAutoTag, WikiIngest, WikiFinalize),
  conversation 2 (TemporaryDocumentProcess, QueryHistoryExport), datasource 2 (DataSourceSync,
  DataSourcePurge), agentruntime 1 (MemoryExtract).
- Other periodic/daemon workers are `container.Invoke` hooks (see §3.4): datasource sync
  scheduler, temporary-document cleanup, audit-log retention, housekeeping, tenant-skill reaper,
  craft decision delivery, commercial fulfillment runner, open-connector recovery runner,
  pending-wiki-task recovery.

### 3.4 `container.Invoke` lifecycle hooks (58 total, internal/container/container.go)

Line → hook (owner):

| Line | Hook | Owner |
|---|---|---|
| 166 | registerLangfuseCleanup | platform |
| 169 | registerPoolCleanup | platform |
| 257 | registerAgentRunResourceProtection | agentruntime |
| 439 | registerWebSearchProviders | airesource |
| 604 | registerModelConcurrencyLimiter (Redis mode) | airesource |
| 615 | registerLiteModelConcurrencyLimiter (Lite mode) | airesource |
| 618 | startTemporaryDocumentCleanup | conversation |
| 636 | injectDataSourceTaskInspector | datasource |
| 637 | startDataSourceScheduler | datasource |
| 639 | startAuditLogRetention | identity |
| 642 | startHousekeepingService | system |
| 645-661 | chatpipeline plugin registration ×17 (Search, Rerank, WebFetch, Merge, DataAnalysis, IntoChatMessage, ChatCompletion, ChatCompletionStream, FilterTopK, QueryUnderstand, LoadHistory, MemoryRecall, ExtractEntity, SearchEntity, SearchParallel, WikiBoost, MemoryAffinity) | conversation |
| 665 | registerCraftHTTPHandlers | craft |
| 668 | registerCraftScheduledHTTPHandlers | craft |
| 671 | registerArtifactVersionHTTPHandlers | workbench |
| 676 | registerArtifactPreviewHTTPHandlers | workbench |
| 680 | wireCraftInteractionRegistrar | craft |
| 681 | registerCraftInteractionHTTPHandlers | craft |
| 682 | startCraftDecisionDelivery | craft |
| 684 | validateCraftKnowledgeAssembly | craft |
| 686 | registerCraftUsageHTTPHandlers | craft |
| 689 | wireCraftLifecycleIntegration | craft |
| 693 | inline: AgentRunStore + FileService + ResourceCleaner cleanup | agentruntime |
| 702 | startTenantSkillReaper | agentcatalog |
| 751 | inline: AuthHandler + MobileExchangeStore | identity |
| 863 | registerIMService | channels |
| 881 | inline: CommercialHandler + CommercialPlatform | commercial |
| 892 | inline: CommercialHandler + BillingAccountService | commercial |
| 902 | inline: CommercialHandler + PlanVersionService | commercial |
| 916 | inline: QuotaGuard → Commercial/TenantMember/Knowledge | commercial (split note) |
| 944 | startCommercialFulfillment | commercial |
| 980 | inline: AppActionHandler + ActionService | appconnector |
| 983 | inline: AppActionHandler + OCPreparer | appconnector |
| 986 | inline: AppActionHandler + OCConnectionService | appconnector |
| 989 | startOCRecoveryRunner | appconnector |
| 994 | inline: AppConnectionHandler | appconnector |
| 1015 | inline: CommercialHandler + OrderService | commercial |
| 1034 | registerChatLocalImageResolver | conversation |
| 1038 | wireCraftSessionTombstone | craft |
| 1044 | RunAsynqServer (Redis mode worker bootstrap) | bootstrap |
| 1046 | RegisterSyncHandlers (Lite mode worker bootstrap) | bootstrap |
| 1052 | recoverPendingWikiTasks | knowledge |

Owner totals: conversation 19, craft 9, commercial 6, appconnector 5, airesource 3,
platform 2, agentruntime 2, datasource 2, identity 2, bootstrap 2, workbench 2, system 1,
agentcatalog 1, channels 1, knowledge 1 = **58**.

### 3.5 Migrations

- Directory: `migrations/` at repo root. Runner: `internal/database/migration.go`.
- **537 files** = mysql 1 (`00-init-db.sql`) + paradedb 2 + sqlite 188 (94 paired migrations)
  + versioned 346 (173 paired migrations).
- Every migration pair has exactly one owner in `backend-modules.yaml` (owner → count of
  paired assets, plus singles): knowledge 42, execution 29, commercial 26, agentruntime 23,
  agentcatalog 21, airesource 20, workbench 20, conversation 19, craft 18, identity 16,
  appconnector 14, channels 8, platform 4 (2 pairs + 2 paradedb), datasource 3, bootstrap 3
  (2 init pairs + mysql init), insights 2, system 2. Total assets: 270.
- The two `000000_init` baseline schemas (sqlite + versioned) and `mysql/00-init-db.sql` are
  assigned `bootstrap` with an explicit note: they create cross-module baseline tables in one
  batch; per-table ownership split is flagged for design revision per §5.18's rule on
  omissions/one-to-many ownership.
- sqlite tree = Lite-mode schema; versioned tree = Postgres schema. Numbering gaps
  (e.g. sqlite 000017/000018/000085/000086/000088-000090/000101; versioned 000100-000109,
  000164/000165/000167-000169/000180) are pre-existing, not inventory errors.

## 4. Method and verification notes

- Route count = textual count of gin method registrations on non-test files; every receiver
  was verified to be a gin `*gin.Engine`/`*gin.RouterGroup` variable (spot-checked all
  ambiguous receivers). A route added/moved later will change these counts.
- Hook inventory = exact `container.Invoke(` call-site scan of `internal/container/container.go`
  (the only file with `container.Invoke` outside tests).
- Worker inventory = `mux.HandleFunc` scan of `internal/router/task.go` and
  `RegisterHandler` scan of `internal/router/sync_task.go` (23 identical task-type sets).
- Ownership decisions follow docs/specs/2026-09-21-backend-domain-module-reorganization-design.md
  §5.18 (覆盖矩阵). Horizontal packages are assigned to the plurality content owner with a
  `split:` note (permitted by F0 resolution #4); Pass B must split files.
