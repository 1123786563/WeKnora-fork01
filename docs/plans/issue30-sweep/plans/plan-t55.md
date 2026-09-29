# T25 #55：代码交付部分成功、未知结果与凭据隔离 — 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为代码交付补齐 T25 三件缺失：部分成功（推送成功/PR 失败）经最高稳定 Interface 的端到端恢复证据与移动端恢复闭环、未知结果的远端核对闭环、以及「通用 Shell 无远端写凭据」的结构性隔离钉死。

**Architecture:** 服务端部分成功状态机（`pushed`/`unknown`）已由 #52/#54 交付，本计划零新迁移、零生产代码路径新增，交付物是：①修复 #54 集成时破坏的 #53 HTTP e2e 装配（缺 `Providers`）；②新建 HTTP wire 级 T25 恢复 e2e（真实全量迁移 sqlite + 真实 handler/路由 + 真实 A03 审批链 + 契约钉死的 GitHub stub，含凭据探针断言）；③沙箱 exec 环境白名单单测；④移动端恢复写通道（api-client → mobile-core `createDeliveryRecovery` 状态自动路由 → TaskDetailScreen 恢复操作）；⑤移动 Terminal 无输入通道的结构性防护测试；⑥opt-in 集成证据扩展与真实 Provider blocked-env loop。

**Tech Stack:** Go 1.26（gin + gorm + testify，`go test`）、TypeScript（`packages/api-client`、`packages/mobile-core`、`apps/mobile` Expo RN）、node:test + tsx（TS 测试运行器）。所有测试命令在 worktree 根（`.worktrees/issue30-sweep`）执行；前置 `pnpm install` 已就绪。

**Spec:** `docs/plans/issue30-sweep/issues/issue-55.md`（验收标准原文）；`docs/specs/2026-09-20-mobile-ai-office-design.md`（140 行 Developer 边界、US 39/US 47、163 行集成测试要求）；`docs/specs/2026-09-20-mobile-module-seams.md`（220 行只读 Terminal、233 行终端日志分页）；`CONTEXT.md`「代码交付」「代码交付审批」术语。

## Global Constraints

以下约束逐字引自 Issue #55 验收标准与批准 Spec，每个任务的要求隐含本节：

- **AC1（原文）**：「推送成功/PR失败可恢复且不重复推送。」
- **AC2（原文）**：「移动 Terminal 底层无法发送输入或绕过 Delivery。」
- **AC3（原文）**：「端到端行为通过最高稳定 Interface 验证；底层单测、静态检查或 mock 不冒充真实集成证据。」
- **What to build（原文）**：「交付部分成功时只补做未完成步骤；未知结果核对远端；通用 Shell 无远端写凭据。」
- Spec `2026-09-20-mobile-ai-office-design.md:140`（原文）：「Developer supports personal and Tenant GitHub/GitLab connections, task branches and draft PR/MR only. It does not merge automatically or expose remote credentials to Shell.」
- Spec 同文件 `:76`（US 47 原文）：「As a developer, I want push-success and PR-creation failure represented as partial completion, so that recovery does not repeat the push.」
- Spec 同文件 `:68`（US 39 原文）：「As a Task Owner, I want every external action to retain an independent result, so that partial success can be reconciled without repeating successful actions.」
- Spec 同文件 `:163`（原文）：「Go integration tests verify authorization, Tenant/Owner/Collaborator predicates, revision and digest CAS, idempotency, partial external success, unknown outcomes and durable checkpoints.」
- `CONTEXT.md`「代码交付」避免项（原文）：「首版不直接写入受保护分支，也不自动合并。」
- **测试归属**：AC1 → Task 2（wire e2e）/ Task 5（mobile-core 状态路由）/ Task 6（移动 UI）/ Task 8（集成证据）；AC2 → Task 3（沙箱 env 白名单）/ Task 4（api-client terminal 面钉死）/ Task 6（移动源级防护），既有只读证据（`MATERIAL_TERMINAL_READ_ONLY`，`packages/mobile-core/src/material/task-material.ts:227`）引用不重复；AC3 → Task 2（真实迁移 DB + 真实 handler + wire stub 的最高稳定 Interface）/ Task 7（真实 Provider blocked-env）/ Task 8（opt-in 真实部署）。
- **凭据纪律**：测试中的 token 全部是伪探针值（如 `ghp_T25PROBE_7f3a9c1e`），不是可用凭据；真实凭据只从环境变量读取（`WEKNORA_GITHUB_TEST_TOKEN` 等既有键），缺 env 即 `t.Skip` 如实声明 blocked-env，绝不伪造通过。
- **数据库纪律**：所有 DB 访问经 gorm 参数绑定（既有范式，本计划零裸 SQL 拼接）。
- **并行集成约束**：本计划与同批（B6：#63/#64）并行实施。共享文件修改清单（全部为最小增量、位置明确）：`internal/application/repository/delivery_collaboration_http_test.go`（Task 1 一行装配）、`packages/api-client/src/mobile/code-delivery.ts` + `code-delivery.test.ts`（Task 4）、`packages/mobile-core/src/index.ts`（Task 5 追加导出）、`apps/mobile/src/composition.ts`（Task 6 追加工厂）、`apps/mobile/src/screens/TaskDetailScreen.tsx`（Task 6 追加 props/区块）、`apps/mobile/src/app/tasks/detail.tsx`（Task 6 追加回调）、`apps/mobile/src/app-smoke.test.tsx`（Task 6 追加一个 test 块）、`apps/mobile/src/delivery-integration-smoke.ts` + `.test.ts`（Task 8）、`internal/modules/codedelivery/github_real_test.go`（Task 7 追加测试）。codedelivery 域与 delivery 移动域是本计划独占领域（#52/#54/#46 已完成，无在途计划竞争）。
- **零迁移**：本计划无新表无新列，不触碰迁移轨道（`internal/database` 的 `TestMigrationVersionsUniquePerTrack` 常驻守卫不受影响）。
- **blocked-env 声明（AC3 边界）**：真实 GitHub/GitLab「部分成功→恢复」闭环需要平台侧 PR 创建失败注入，真实环境不可控。本地等价证据 = Task 2 的 wire 契约 stub e2e（stub 的 HTTP 形状由 `codedelivery/github_wire_test.go` 与真实 API 钉合——此范式已由 #52 计划差异记录与最终报告确立）；真实环境证据 = Task 7 的 delivered 后重复派发被拒 loop（env 门控）。移动端真实部署恢复证据 = Task 8（opt-in）。

## Review Focus

以下五类输入/失效模式是 Spec 隐含但没有任何任务的测试自然覆盖、最可能伤害使用者的，按可能性排序；每行后标注钉死它的测试与所属任务：

1. **恢复半程的重入/并发**（用户双击、网络层重试同时发出两次 dispatch-on-pushed）：一个合理的预期是第二次请求不产生第二次推送且状态机不撕裂。→ Task 2 Step 1 测试 1 的第三次 dispatch 409 + 计数零变化断言；Task 5 Step 1 的 delivered 幂等早退测试（零写请求）。
2. **移动面混入 Terminal 输入通道的回归**（未来有人在移动端接线 WS 终端或给 terminal API 加输入方法，破坏「移动 Terminal 无法发送输入」）：预期是移动装配图与 terminal API 面上结构性无输入方法。→ Task 4 Step 1 的 `terminal-surface.test.ts`（API 面恰为 `['issueTicket']`）；Task 6 Step 4 的 `terminal-purity.test.ts`（apps/mobile/src 源级扫描零命中 `createSandboxTerminalApi|issueTicket|sandbox/terminal-ticket`）。
3. **凭据经新恢复路径泄漏**（恢复请求的响应、日志或工作区把 token 或凭据引用带出）：预期是 token 仅出现在出网 Authorization 头。→ Task 2 Step 1 测试 3 的探针断言（Authorization 头唯一出现点 + 响应 body/工作区文件/code_deliveries 全表列不含探针，覆盖恢复请求自身的响应）。
4. **恢复写通道在 scope 撤销后迟到发出**（切租户/登出后 in-flight 恢复仍落网）：预期是 lease 前后双守卫拒绝。→ Task 5 Step 1 的无 lease 与写后撤销两个测试。
5. **沙箱 exec 环境白名单被绕过**（新代码往 exec env 塞凭据类键，破坏「通用 Shell 无远端写凭据」）：预期是环境注入函数只补 `WEKNORA_` 工作区路径键。→ Task 3 Step 1 的白名单测试（`withWorkspaceEnvDefaults` 对任意输入永不新增非 `WEKNORA_` 键）。

---

### Task 1: Go 前置修复——#53 HTTP e2e 装配补 `Providers`（修复 #54 集成回归）

**Files:**
- Modify: `internal/application/repository/delivery_collaboration_http_test.go:235-242`（`NewCodeDeliveryService(CodeDeliveryDeps{...})` 装配块）

**Interfaces:**
- Consumes: `appconnectorrepo.NewInstallationStore(db *gorm.DB) *InstallationStore`（`internal/modules/appconnector/repository/appconnector/install.go:66`）与 `GetInstallationByID(ctx, tenant, installationID)`（`install.go:225`）；#53 既有夹具 `openTaskGrantDB`/`taskGrantAdmission`（同包 `task_grant_store_test.go:27` / `task_grant_read_test.go:16`）。
- Produces: `TestDeliveryCollaboration*` 三个 e2e 恢复 PASS——它们是 Task 2 wire e2e 的宿主范式（真实全量迁移 sqlite + 真实 handler + 真实 A03）。

**背景（代码现状，实跑取证）**：当前 HEAD 上 `go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1` FAIL（3 个测试全部 `400 code_delivery_unsupported_provider`）。根因：#54 给 `CodeDeliveryService.platformProvider` 引入 fail-closed 的 `Providers` 必填（`internal/modules/codedelivery/service.go:477-489`：`Providers == nil ⇒ ErrUnsupportedProvider`，回归测试 `TestUnwiredProvidersFailsClosedEvenForGitHub` 钉死），但 #53 的 e2e 装配（`delivery_collaboration_http_test.go:235-242`）未同步传入 `Providers`。这是 #54 合入时对 #53 证据面的既有破坏，属本 Issue 邻域基础设施，前置修复。

- [x] **Step 1: 运行既有失败测试确认 RED（失败已现成）**

Run: `go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1`
Expected: FAIL，3 个测试均报 `{"code":"code_delivery_unsupported_provider","error":"the connection is not a code platform connection","success":false}`（计划作者已在当前 HEAD 实跑复现此输出）。

- [x] **Step 2: 装配块补 `Providers` 一行**

在 `delivery_collaboration_http_test.go` 的 `svc := codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{...})` 装配块（`:235-242`）中，`GitHub: factory,` 行后追加：

```go
		Providers:   appconnectorrepo.NewInstallationStore(db),
```

`appconnectorrepo` 已在该文件 import（`:31`）。夹具已在 `:214` 创建了 `InstallationRow{ID: "inst-gh", TenantID: 1, AppID: "github", ...}`，`GetInstallationByID` 将解析出 `github`。

- [x] **Step 3: 运行测试验证 GREEN**

Run: `go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1`
Expected: PASS（ok）。

- [x] **Step 4: Commit**

```bash
git add internal/application/repository/delivery_collaboration_http_test.go
git commit -m "fix(codedelivery): wire Providers into the T23 delivery e2e rig — restore #53 evidence broken by the #54 fail-closed provider gate"
```

### Task 2: Go——T25 恢复 HTTP wire e2e：部分成功恰一次恢复、重复派发拒、unknown 远端收敛、凭据探针隔离

**Files:**
- Test: `internal/application/repository/delivery_recovery_http_test.go`（新建）

**Interfaces:**
- Consumes: Task 1 修复后的宿主范式与同包助手：`openTaskGrantDB(t)`、`taskGrantAdmission()`、seed 种子（tenant 1 / owner u1 / session s1）；生产构造函数 `codedelivery.NewCodeDeliveryService` / `NewDeliveryDispatcher` / `NewGitHubClientFactory`（`NewGitHubClientFactory(httpClient, baseURL)`，`github_client.go`）、`appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)`、`appconnectorrepo.NewActionStore(db)` / `NewInstallationStore(db)`、`deliveryrepo.NewDeliveryStore(db)`、`codedelivery.NewLocalWorkspaceSource(root)`；HTTP 面 `session.NewWorkbenchDeliveryHandler(runs, runs, svc)` 的 `DispatchDelivery`/`ResolveDeliveryUnknown`/`GetDelivery`/`PrepareDelivery`/`MaterializeBaseline`（`internal/handler/session/workbench_delivery.go:17-23` 接口，构造器 `:34`）与 `handler.NewAppActionHandler(db)` + `SetActionService(actions)` 的 `POST /apps/actions/:id/approve`。
- Produces: `TestT25PartialPushPRFailureRecoversExactlyOnceOverHTTP` / `TestT25UnknownResolvesFromRemoteFactsOverHTTP` / `TestT25CredentialsNeverLeaveTheDispatchBoundary` 三个 e2e（AC1+AC3+凭据隔离的服务端最高稳定 Interface 证据）；文件内助手 `recoveryEnv` / `recoveryGitHubStub` 可被后续批次（如 #71 证据矩阵）镜像复用。

**设计要点**：stub 镜像 `delivery_collaboration_http_test.go:47-145` 的 `githubStub`（同一最小 GitHub REST 子集），追加三类能力：①按方法+路径的调用计数（`calls map[string]int`，键形如 `"POST /git/blobs"`）；②`failPRCreations int`（>0 时 `POST /pulls` 返回 422，制造确定性 PR 失败 → `pushed`）；③`failPRTransport bool`（`POST /pulls` 时 `panic(http.ErrAbortHandler)` 掐断连接，`http.Client` 收到 EOF = 传输不可观测 → `unknown`）。`GET /pulls` 按 `r.URL.Query().Get("head")` 过滤（`github_client.go:261` 以 `?head=` 查询，ResolveUnknown 的 `QueryProvider` 经它找 PR——`dispatcher.go:279`）。

- [x] **Step 1: 写失败测试（三个 e2e，完整代码）**

新建 `internal/application/repository/delivery_recovery_http_test.go`：

```go
package repository_test

// T25 (#55) recovery evidence over the highest-stable interface: the real
// HTTP delivery surface (routing + auth context + real handlers), a REAL
// fully-migrated sqlite database, the real A03 approval chain, and a
// GitHub stub speaking the wire contract pinned by
// codedelivery/github_wire_test.go. The stub is the only double; the
// provider itself is credential-gated (blocked-env, Task 7 covers the
// real-provider loop). This file mirrors delivery_collaboration_http_test.go.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/handler/session"
	appconnector "github.com/Tencent/WeKnora/internal/modules/appconnector"
	appconnectorrepo "github.com/Tencent/WeKnora/internal/modules/appconnector/repository/appconnector"
	appconnectorsvc "github.com/Tencent/WeKnora/internal/modules/appconnector/service/appconnector"
	"github.com/Tencent/WeKnora/internal/modules/codedelivery"
	deliveryrepo "github.com/Tencent/WeKnora/internal/modules/codedelivery/repository/codedelivery"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// The credential probe: a FAKE token (never a usable credential). Every
// assertion below proves this byte string exists ONLY in the outbound
// Authorization header — nowhere in responses, workspace files or the
// delivery ledger (spec: "does not ... expose remote credentials to Shell").
const t25ProbeToken = "ghp_T25PROBE_7f3a9c1e"
const t25BaseSHA = "b000000000000000000000000000000000000000"

// recoveryGitHubStub: the wire contract of githubStub (T23) plus call
// counters, deterministic PR failure injection (422 → partial `pushed`),
// transport-failure injection (connection abort → `unknown`), and
// head-filtered PR listing for the unknown resolver.
type recoveryGitHubStub struct {
	srv *httptest.Server

	mu              sync.Mutex
	blobs           map[string][]byte
	refs            map[string]string
	prs             []map[string]any
	prSeq           int64
	calls           map[string]int
	failPRCreations int // >0: every PR creation answers 422 (definite failure)
	failPRTransport bool
}

func newRecoveryGitHubStub(t *testing.T) *recoveryGitHubStub {
	s := &recoveryGitHubStub{
		blobs: map[string][]byte{"blob-main": []byte("package main\n")},
		refs:  map[string]string{},
		calls: map[string]int{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.serve)
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	return s
}

func (s *recoveryGitHubStub) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *recoveryGitHubStub) count(key string) {
	s.mu.Lock()
	s.calls[key]++
	s.mu.Unlock()
}

func (s *recoveryGitHubStub) snapshotCalls() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]int{}
	for k, v := range s.calls {
		out[k] = v
	}
	return out
}

func (s *recoveryGitHubStub) serve(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("Authorization"); got != "Bearer "+t25ProbeToken {
		s.writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "bad credentials"})
		return
	}
	path := r.URL.Path
	switch {
	case r.Method == http.MethodGet && path == "/user":
		s.writeJSON(w, http.StatusOK, map[string]any{"login": "octocat-remote"})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello":
		s.writeJSON(w, http.StatusOK, map[string]any{"default_branch": "main", "private": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/branches/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"protected": false})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/commits/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"tree": "tree-baseline"})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/trees/"):
		s.writeJSON(w, http.StatusOK, map[string]any{"truncated": false, "tree": []map[string]any{
			{"path": "main.go", "sha": "blob-main", "type": "blob"},
		}})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/blobs/"):
		sha := filepath.Base(path)
		content, ok := s.blobs[sha]
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "no such blob"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{
			"encoding": "base64", "content": base64.StdEncoding.EncodeToString(content),
		})
	case r.Method == http.MethodGet && strings.HasPrefix(path, "/repos/octocat/hello/git/ref/heads/"):
		branch := strings.TrimPrefix(path, "/repos/octocat/hello/git/ref/heads/")
		s.mu.Lock()
		sha, ok := s.refs[branch]
		s.mu.Unlock()
		if !ok {
			s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "no such ref"})
			return
		}
		s.writeJSON(w, http.StatusOK, map[string]any{"object": map[string]any{"sha": sha}})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/blobs":
		s.count("POST /git/blobs")
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "blob-changed"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/trees":
		s.count("POST /git/trees")
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "tree-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/commits":
		s.count("POST /git/commits")
		s.writeJSON(w, http.StatusCreated, map[string]any{"sha": "commit-delivery"})
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/git/refs":
		s.count("POST /git/refs")
		var body struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		s.mu.Lock()
		s.refs[strings.TrimPrefix(body.Ref, "refs/heads/")] = body.SHA
		s.mu.Unlock()
		s.writeJSON(w, http.StatusCreated, map[string]any{"ref": body.Ref})
	case r.Method == http.MethodPatch && strings.HasPrefix(path, "/repos/octocat/hello/git/refs/"):
		s.count("PATCH /git/refs")
		s.writeJSON(w, http.StatusOK, map[string]any{"object": map[string]any{"sha": "commit-delivery"}})
	case r.Method == http.MethodGet && path == "/repos/octocat/hello/pulls":
		head := r.URL.Query().Get("head")
		s.mu.Lock()
		out := []map[string]any{}
		for _, pr := range s.prs {
			if head == "" || pr["head_ref"] == strings.TrimPrefix(head, "octocat:") {
				out = append(out, pr)
			}
		}
		s.mu.Unlock()
		s.writeJSON(w, http.StatusOK, out)
	case r.Method == http.MethodPost && path == "/repos/octocat/hello/pulls":
		s.count("POST /pulls")
		s.mu.Lock()
		failCreations := s.failPRCreations
		failTransport := s.failPRTransport
		s.mu.Unlock()
		if failTransport {
			// Unobservable outcome: the connection dies mid-request, the
			// client sees a transport error, the delivery settles unknown.
			panic(http.ErrAbortHandler)
		}
		if failCreations > 0 {
			s.mu.Lock()
			s.failPRCreations--
			s.mu.Unlock()
			s.writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"message": "Validation Failed"})
			return
		}
		s.mu.Lock()
		s.prSeq++
		pr := map[string]any{"number": s.prSeq, "html_url": fmt.Sprintf("https://github.com/octocat/hello/pull/%d", s.prSeq), "draft": true, "state": "open", "head_ref": "weknora/task/s1"}
		s.prs = append(s.prs, pr)
		s.mu.Unlock()
		s.writeJSON(w, http.StatusCreated, pr)
	default:
		s.writeJSON(w, http.StatusNotFound, map[string]any{"message": "unexpected " + r.Method + " " + path})
	}
}

// t25CredentialSource: same dual role as the T23 rig (connection reader +
// credential resolver), but LoadCredential/Resolve hand out the PROBE token
// so the containment assertions observe the real dispatch path.
type t25CredentialSource struct {
	db      *gorm.DB
	members map[string]bool
}

func (s *t25CredentialSource) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("tenant_id = ? AND id = ?", 1, id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, CredentialRef: row.CredentialRef,
		State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *t25CredentialSource) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte(t25ProbeToken), nil
}

func (s *t25CredentialSource) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.members[userID], nil
}

func (s *t25CredentialSource) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

func (s *t25CredentialSource) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte(t25ProbeToken), nil
}

// recoveryEnv mirrors newDeliveryCollabEnv WITH the Providers wiring (the
// regression Task 1 fixed) and the counting/injecting stub.
type recoveryEnv struct {
	db     *gorm.DB
	engine *gin.Engine
	wsRoot string
	github *recoveryGitHubStub
}

func newRecoveryEnv(t *testing.T) *recoveryEnv {
	t.Helper()
	db := openTaskGrantDB(t)
	runs := repository.NewAgentRunStore(db)
	_, err := runs.Admit(context.Background(), taskGrantAdmission())
	require.NoError(t, err)
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", TenantID: 1, AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, Version: 1}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{TenantID: 1, ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal, OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive, AuthVersion: 1}).Error)

	github := newRecoveryGitHubStub(t)
	wsRoot := t.TempDir()
	workspace, err := codedelivery.NewLocalWorkspaceSource(wsRoot)
	require.NoError(t, err)

	actionStore := appconnectorrepo.NewActionStore(db)
	store := deliveryrepo.NewDeliveryStore(db)
	connections := &t25CredentialSource{db: db, members: map[string]bool{"u1": true}}
	guard := appconnectorsvc.NewSubjectGuard(connections)
	factory := codedelivery.NewGitHubClientFactory(http.DefaultClient, github.srv.URL)
	dispatcher := codedelivery.NewDeliveryDispatcher(codedelivery.DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guard,
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: runs,
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	svc := codedelivery.NewCodeDeliveryService(codedelivery.CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, Providers: appconnectorrepo.NewInstallationStore(db),
		Workspace: workspace, Runs: runs, Dispatcher: dispatcher,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	v1 := r.Group("/api/v1")
	deliveryHandler := session.NewWorkbenchDeliveryHandler(runs, runs, svc)
	v1.GET("/workbench/executions/:run_id/delivery", deliveryHandler.GetDelivery)
	v1.POST("/workbench/executions/:run_id/baseline", deliveryHandler.MaterializeBaseline)
	v1.POST("/workbench/executions/:run_id/delivery", deliveryHandler.PrepareDelivery)
	v1.POST("/workbench/executions/:run_id/delivery/:delivery_id/dispatch", deliveryHandler.DispatchDelivery)
	v1.POST("/workbench/executions/:run_id/delivery/:delivery_id/resolve", deliveryHandler.ResolveDeliveryUnknown)
	actionHandler := handler.NewAppActionHandler(db)
	actionHandler.SetActionService(actions)
	v1.POST("/apps/actions/:id/approve", actionHandler.ApproveAction)
	return &recoveryEnv{db: db, engine: r, wsRoot: wsRoot, github: github}
}

func (e *recoveryEnv) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, uint64(1))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u1")
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleContributor)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// seedApprovedDelivery drives the owner's real baseline→edit→prepare→approve
// loop and returns the delivery id. Request shapes mirror the T23 rig exactly
// (delivery_collaboration_http_test.go:289-326): repo is the STRING
// "owner/name" (deliveryBaselineInput.Repo is a string, workbench_delivery.go:52),
// baseline answers 201, and approve carries the action's CURRENT fence as
// expected_version.
func (e *recoveryEnv) seedApprovedDelivery(t *testing.T) string {
	t.Helper()
	w := e.do(t, "POST", "/api/v1/workbench/executions/r1/baseline",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+t25BaseSHA+`"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	target := filepath.Join(e.wsRoot, "octocat/hello")
	require.NoError(t, os.MkdirAll(target, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(target, "main.go"), []byte("package main\n\nfunc main() { println(\"t25\") }\n"), 0o644))
	w = e.do(t, "POST", "/api/v1/workbench/executions/r1/delivery",
		`{"connection_id":"conn-gh","repo":"octocat/hello","baseline_sha":"`+t25BaseSHA+`","commit_message":"fix: t25 recovery","pr_title":"WeKnora t25 recovery"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var prepared struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &prepared))
	require.Equal(t, "u1", prepared.Data.Delivery.Initiator)
	require.Equal(t, "prepared", prepared.Data.Delivery.State)
	// Approve over the REAL A03 endpoint with the persisted fence — same as
	// the T23 approveThroughHTTP (delivery_collaboration_http_test.go:323-332).
	var fence int64
	require.NoError(t, e.db.Raw(`SELECT fence FROM app_actions WHERE id = ?`, prepared.Data.Delivery.ActionID).Scan(&fence).Error)
	w = e.do(t, "POST", "/api/v1/apps/actions/"+prepared.Data.Delivery.ActionID+"/approve",
		fmt.Sprintf(`{"digest":%q,"expected_version":%d}`, prepared.Data.Delivery.Digest, fence))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return prepared.Data.Delivery.ID
}

func (e *recoveryEnv) dispatchState(t *testing.T, deliveryID string) (int, codedelivery.DeliveryView) {
	t.Helper()
	w := e.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/dispatch", "")
	var out struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		}
	}
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	}
	return w.Code, out.Data.Delivery
}

// AC1 e2e：推送成功 + PR 确定性失败 = pushed；恢复只补 PR 恰一次；
// delivered 之后的重复派发被状态机拒绝（409）且零远端副作用。
func TestT25PartialPushPRFailureRecoversExactlyOnceOverHTTP(t *testing.T) {
	env := newRecoveryEnv(t)
	env.github.mu.Lock()
	env.github.failPRCreations = 1
	env.github.mu.Unlock()
	deliveryID := env.seedApprovedDelivery(t)

	code, view := env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "pushed", view.State, "push-success + definite PR failure = partial completion")
	require.NotEmpty(t, view.CommitSHA, "the pushed commit is already on the ledger")

	pushed := env.github.snapshotCalls()
	for _, key := range []string{"POST /git/blobs", "POST /git/trees", "POST /git/commits", "POST /git/refs"} {
		require.Equal(t, 1, pushed[key], "%s must have happened exactly once during the push half", key)
	}
	require.Equal(t, 1, pushed["POST /pulls"], "the PR creation failed exactly once")

	// The recovery: same approval, PR-only.
	code, view = env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "delivered", view.State)
	require.NotZero(t, view.PRNumber)
	require.NotEmpty(t, view.PRURL)

	recovered := env.github.snapshotCalls()
	require.Equal(t, pushed["POST /git/blobs"], recovered["POST /git/blobs"], "recovery must not re-send blobs")
	require.Equal(t, pushed["POST /git/trees"], recovered["POST /git/trees"], "recovery must not re-send trees")
	require.Equal(t, pushed["POST /git/commits"], recovered["POST /git/commits"], "recovery must not re-send commits")
	require.Equal(t, pushed["POST /git/refs"], recovered["POST /git/refs"], "recovery must not re-push the branch")
	require.Equal(t, pushed["POST /pulls"]+1, recovered["POST /pulls"], "recovery retries ONLY the PR creation")

	// Duplicate dispatch after delivery: refused by the state machine, zero
	// new remote side effects (double-tap / network-retry re-entry).
	code, _ = env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusConflict, code)
	final := env.github.snapshotCalls()
	require.Equal(t, recovered, final, "the rejected duplicate dispatch must not touch the provider")

	w := env.do(t, "GET", "/api/v1/workbench/executions/r1/delivery", "")
	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"state":"delivered"`)
}

// AC1 e2e（unknown half）：传输不可观测 → unknown；resolve 只读远端事实
// 收敛为 pushed（分支在、PR 缺席），再走 PR-only 恢复到 delivered。
func TestT25UnknownResolvesFromRemoteFactsOverHTTP(t *testing.T) {
	env := newRecoveryEnv(t)
	env.github.mu.Lock()
	env.github.failPRTransport = true
	env.github.mu.Unlock()
	deliveryID := env.seedApprovedDelivery(t)

	code, view := env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "unknown", view.State, "an unobservable PR transport failure settles unknown, never a replay")

	w := env.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/resolve", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resolved struct {
		Data struct {
			Delivery codedelivery.DeliveryView
		}
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resolved))
	require.Equal(t, "pushed", resolved.Data.Delivery.State, "remote facts: branch pushed, draft PR absent")

	env.github.mu.Lock()
	env.github.failPRTransport = false
	env.github.mu.Unlock()
	code, view = env.dispatchState(t, deliveryID)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "delivered", view.State)
	require.NotZero(t, view.PRNumber)
}

// AC2/凭据隔离 e2e（spec :140 "does not ... expose remote credentials to
// Shell"）：探针 token 只允许出现在出网 Authorization 头——不出现在任何
// 响应 body、工作区文件（Shell 能看到的世界）或交付台账的任何列。
func TestT25CredentialsNeverLeaveTheDispatchBoundary(t *testing.T) {
	env := newRecoveryEnv(t)
	env.github.mu.Lock()
	env.github.failPRCreations = 1
	env.github.mu.Unlock()
	deliveryID := env.seedApprovedDelivery(t)

	bodies := []string{}
	capture := func(w *httptest.ResponseRecorder) { bodies = append(bodies, w.Body.String()) }
	capture(env.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/dispatch", ""))
	capture(env.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/dispatch", ""))
	capture(env.do(t, "GET", "/api/v1/workbench/executions/r1/delivery", ""))
	capture(env.do(t, "POST", "/api/v1/workbench/executions/r1/delivery/"+deliveryID+"/resolve", ""))

	for i, body := range bodies {
		require.NotContains(t, body, t25ProbeToken, "response %d must not carry credential material", i)
	}
	// The workspace IS the Shell's world: no dispatched file may embed the probe.
	require.NoError(t, filepath.WalkDir(env.wsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		content, rerr := os.ReadFile(path)
		require.NoError(t, rerr)
		require.NotContains(t, string(content), t25ProbeToken, "workspace file %s must not embed the credential (Shell isolation)", path)
		return nil
	}))
	// The delivery ledger: no string column may embed the probe.
	var rows []deliveryrepo.DeliveryRow
	require.NoError(t, env.db.Where("tenant_id = ?", 1).Find(&rows).Error)
	for _, row := range rows {
		for _, field := range []string{row.ID, row.TaskID, row.RunID, row.OwnerID, row.ActionID,
			row.ConnectionID, row.Repo, row.BaselineSHA, row.Branch, row.CommitSHA, row.PRURL,
			row.RemoteLogin, row.State, row.Failure} {
			require.NotContains(t, field, t25ProbeToken, "the delivery ledger must not carry credential material")
		}
	}
	// Positive control: the probe DID leave as the Authorization header —
	// every provider call carried it (otherwise the stub 401s and the run
	// above would not have reached delivered/pushed states).
	require.NotZero(t, env.github.snapshotCalls()["POST /git/blobs"])
}
```

注意：`e.db.Raw(...).Scan(&fence)` 是 #53 `approveThroughHTTP` 同款参数绑定读（`delivery_collaboration_http_test.go:328`）；approve 不带 `expected_version` 会被 A03 的 fence 检查拒绝，这是镜像而非新设计。

- [x] **Step 2: 运行测试确认失败形态**

Run: `go test ./internal/application/repository/ -run 'TestT25' -count=1`
Expected: 首跑为编译期或断言期 FAIL 均可接受（例如 import 遗漏、`seedApprovedDelivery` 的 baseline 请求体与服务端 `deliveryPrepareInput`/`MaterializeBaseline` 的绑定字段不匹配导致的 4xx）。若 baseline 请求被 400 拒绝，对照 `workbench_delivery.go:76-100` 的 `deliveryPrepareInput` 字段名（`connection_id`/`repo`/`baseline_sha`）与 `#53 prepareDelivery`（`delivery_collaboration_http_test.go:289-321`）的实际请求体修正——以两处源码为准。

- [x] **Step 3: 最小修正至通过**

按 Step 2 的失败信息修正（不做超出断言需要的实现改动；本任务零生产代码改动——任何需要动生产代码才能绿的断言都是信号，停下核对语义而不是放宽断言）。

Run: `go test ./internal/application/repository/ -run 'TestT25' -count=1`
Expected: PASS（3 个测试）。

- [x] **Step 4: 回归邻近面**

Run: `go test ./internal/application/repository/ -run 'TestDeliveryCollaboration|TestT25' -count=1`
Expected: PASS。

- [x] **Step 5: Commit**

```bash
git add internal/application/repository/delivery_recovery_http_test.go
git commit -m "test(codedelivery): T25 recovery e2e over the real HTTP delivery surface — partial-push PR-only recovery with zero re-push, duplicate dispatch refused, unknown resolved from remote facts, credential probe containment"
```

### Task 3: Go——沙箱 exec 环境白名单单测（通用 Shell 无远端写凭据的函数级钉死）

**Files:**
- Test: `internal/modules/execution/sandbox/workspace_env_test.go`（新建）

**Interfaces:**
- Consumes: 包内私有 `withWorkspaceEnvDefaults(env map[string]string) map[string]string`（`session_manager.go:368-378`）、常量 `skillOutputEnvVar = "WEKNORA_SKILL_OUTPUT_DIR"` / `sessionInputEnvVar = "WEKNORA_SESSION_INPUT_DIR"`（`session_manager.go:51-57`）与 `SessionOutputRoot`/`SessionInputRoot`。
- Produces: 沙箱环境注入纪律的回归钉（AC2/凭据隔离在 exec env 维度的证据）。

**背景（代码现状）**：沙箱进程环境的唯一函数级注入点是 `withWorkspaceEnvDefaults`（`session_manager.go:1492` `envVars := withWorkspaceEnvDefaults(cloneMetadata(cfg.EnvVars))`——env 仅来自租户沙箱配置 + 工作区路径默认）；交互终端的 `RemoteTerminalOptions` 由 WS handler 构造且**不设 `Envs` 字段**（`internal/handler/session/sandbox_terminal_ws.go:209-213` 仅 `Cols/Rows/AttachPID`，客户端不可注入环境——该文件在 handler/session 包，非本任务所在 sandbox 包）。远端写凭据的唯一出口是 `CredentialResolver.Resolve` → 适配器闭包（`internal/modules/appconnector/service/appconnector/credentials.go:17-23` 注释契约 + Task 2 探针 e2e 的运行时证据）。本任务把函数级纪律钉进测试，防止后续改动向 exec env 塞凭据类键。

- [x] **Step 1: 写失败测试**

新建 `internal/modules/execution/sandbox/workspace_env_test.go`：

```go
package sandbox

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// T25 (#55)：沙箱 Shell 环境的函数级注入纪律——withWorkspaceEnvDefaults 是
// exec env 的唯一函数级注入点（session_manager.go:1492），它只允许补两个
// WEKNORA_ 工作区路径键，永不新增任何其它键（凭据隔离的白名单面）。
func TestWithWorkspaceEnvDefaultsStampsOnlyWorkspacePaths(t *testing.T) {
	got := withWorkspaceEnvDefaults(nil)
	require.Len(t, got, 2)
	require.Equal(t, SessionOutputRoot, got[skillOutputEnvVar])
	require.Equal(t, SessionInputRoot, got[sessionInputEnvVar])
}

func TestWithWorkspaceEnvDefaultsPreservesConfiguredValuesAndAddsNothingElse(t *testing.T) {
	input := map[string]string{
		skillOutputEnvVar: "/workspace/custom-out",
		"PATH":            "/usr/local/bin:/usr/bin",
		"EXISTING_FLAG":   "1",
	}
	got := withWorkspaceEnvDefaults(input)
	require.Equal(t, "/workspace/custom-out", got[skillOutputEnvVar], "a configured workspace path is never overridden")
	require.Equal(t, SessionInputRoot, got[sessionInputEnvVar])
	require.Equal(t, "/usr/local/bin:/usr/bin", got["PATH"])
	require.Equal(t, "1", got["EXISTING_FLAG"])
	// 白名单断言：新增键只允许是两个 WEKNORA_ 工作区键。
	for key := range got {
		if _, configured := input[key]; configured {
			continue
		}
		require.Equal(t, sessionInputEnvVar, key, "the only key this function may add is %s; anything else is a credential-leak surface (T25)", sessionInputEnvVar)
	}
}
```

- [x] **Step 2: 运行测试确认现状**

Run: `go test ./internal/modules/execution/sandbox/ -run 'TestWithWorkspaceEnvDefaults' -count=1`
Expected: 现有实现已满足语义——两测试直接 PASS 也符合 TDD 钉死意图（此任务是回归钉，不是缺陷修复；计划作者已读实现 `session_manager.go:368-378` 确认语义）。若 FAIL，说明实现与所读源码不符——停下按 `session_manager.go` 实际行为重新核对断言，不得为绿改生产代码（除非发现真实凭据泄漏面，那属于本 Issue 必修项，须在差异记录中写明）。

- [x] **Step 3: Commit**

```bash
git add internal/modules/execution/sandbox/workspace_env_test.go
git commit -m "test(sandbox): pin the workspace env whitelist — the exec env injection point may only stamp WEKNORA_ workspace paths (T25 credential isolation)"
```

### Task 4: TS api-client——terminal API 面只读钉死 + code-delivery 恢复写方法

**Files:**
- Modify: `packages/api-client/src/mobile/code-delivery.ts`
- Test: `packages/api-client/src/sandbox/terminal-surface.test.ts`（新建）
- Test: `packages/api-client/src/mobile/code-delivery.test.ts`（追加）

**Interfaces:**
- Consumes: 既有 `createSandboxTerminalApi(request)`（`sandbox/terminal.ts:36-47`，面恰为 `issueTicket`）；既有 `createMobileCodeDeliveryRemote` 的 envelope 解析与 `parseCodeDeliveryRecord`；服务端路由 `POST /api/v1/workbench/executions/:run_id/delivery/:delivery_id/dispatch|resolve`（`routes_workbench.go:288-289`）。
- Produces（Task 5/6/8 消费的精确签名）：

```ts
export interface MobileCodeDeliveryRemote {
  delivery(runId: string): Promise<CodeDeliveryRecord | null>;
  dispatchDelivery(input: { runId: string; deliveryId: string }): Promise<CodeDeliveryRecord>;
  resolveDelivery(input: { runId: string; deliveryId: string }): Promise<CodeDeliveryRecord>;
}
```

错误形态约定（Task 5 翻译消费）：409 以 ApiError 双形状抛出（顶层 `.status`/`.code`，或替身 `.body.code`），`code === 'code_delivery_state_conflict'`。

- [x] **Step 1: 写失败测试（terminal 面钉死）**

新建 `packages/api-client/src/sandbox/terminal-surface.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createSandboxTerminalApi } from './terminal.ts';

// T25 (#55) AC2：terminal API 面是门票签发 ONLY——不存在任何输入发送或
// 终端写方法。未来任何给该面加输入通道的改动都会在此破防。
test('the terminal api surface is ticket-issue only — no input or write channel exists (T25 #55 AC2)', () => {
  const api = createSandboxTerminalApi(async () => {
    throw new Error('no request may leave this probe');
  });
  assert.deepEqual(Object.keys(api), ['issueTicket']);
});
```

Run: `pnpm exec tsx --test packages/api-client/src/sandbox/terminal-surface.test.ts`
Expected: PASS（现状即满足——钉死性测试，同 Task 3 语义；若 FAIL 说明面已被污染，停下核查）。

- [x] **Step 2: 写失败测试（恢复写方法）**

在 `packages/api-client/src/mobile/code-delivery.test.ts` 追加（复用文件内既有 `okDeliveryWire`/`fakeRequest`）：

```ts
test('dispatchDelivery posts the PR-only recovery half and maps the record', async () => {
  const transport = fakeRequest(() => ({ status: 200, body: { delivery: okDeliveryWire } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: transport.request });
  const record = await remote.dispatchDelivery({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(record.state, 'pushed');
  assert.equal(transport.seen[0].method, 'POST');
  assert.equal(transport.seen[0].path, '/api/v1/workbench/executions/run-1/delivery/dlv-1/dispatch');
});

test('resolveDelivery posts the unknown-resolution half and maps the record', async () => {
  const transport = fakeRequest(() => ({ status: 200, body: { delivery: okDeliveryWire } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: transport.request });
  const record = await remote.resolveDelivery({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(record.state, 'pushed');
  assert.equal(transport.seen[0].path, '/api/v1/workbench/executions/run-1/delivery/dlv-1/resolve');
});

test('a 409 state conflict rejects with the ApiError shape intact for the recovery module to translate', async () => {
  const conflict = fakeRequest(() => ({ status: 409, body: { code: 'code_delivery_state_conflict' } }));
  const remote = createMobileCodeDeliveryRemote({ origin: 'https://weknora.example.com', request: conflict.request });
  await assert.rejects(
    () => remote.dispatchDelivery({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: any) => error.status === 409 && error.body?.code === 'code_delivery_state_conflict',
  );
});
```

Run: `pnpm exec tsx --test packages/api-client/src/mobile/code-delivery.test.ts`
Expected: FAIL（`remote.dispatchDelivery is not a function`）。

- [x] **Step 3: 实现两个恢复方法**

在 `packages/api-client/src/mobile/code-delivery.ts` 中：接口 `MobileCodeDeliveryRemote` 追加上述两方法；把既有 `delivery()` 内的 envelope→record 解析提取为私有 helper `deliveryRecordOf(response: unknown): CodeDeliveryRecord`（success 信封 + `data.delivery` + `parseCodeDeliveryRecord`，逻辑逐字搬移）；两个新方法 POST 对应路径后经同一 helper 返回（404→null 语义**不适用**于恢复面——恢复目标不存在是错误，照常 reject）。请求体为空对象 `{}`（服务端 handler 不读 body）。

- [x] **Step 4: 运行测试验证 GREEN**

Run: `pnpm exec tsx --test packages/api-client/src/sandbox/terminal-surface.test.ts packages/api-client/src/mobile/code-delivery.test.ts`
Expected: 全部 PASS——`code-delivery.test.ts` 既有 4 项 + 新 3 项，加 `terminal-surface.test.ts` 1 项，两文件合计 8 项。

- [x] **Step 5: Commit**

```bash
git add packages/api-client/src/sandbox/terminal-surface.test.ts packages/api-client/src/mobile/code-delivery.ts packages/api-client/src/mobile/code-delivery.test.ts
git commit -m "feat(api-client): delivery recovery write methods (dispatch/resolve) + pin the terminal api surface to ticket-issue only (T25 #55)"
```

### Task 5: TS mobile-core——`createDeliveryRecovery` 状态自动路由恢复模块

**Files:**
- Create: `packages/mobile-core/src/delivery/delivery-recovery.ts`
- Test: `packages/mobile-core/src/delivery/delivery-recovery.test.ts`（新建）
- Modify: `packages/mobile-core/src/index.ts:103-107`（delivery 导出段追加）

**Interfaces:**
- Consumes: `DeliveryRemote`/`DeliveryRemoteRecord`/`deliveryViewOf`/`DeliveryReceiptView`（`delivery-view.ts`）、`leaseActive`（`runtime/scope-lease.ts`）、`ScopeLease`（`runtime/types.ts`）；Task 4 的 `MobileCodeDeliveryRemote`（同一 remote 结构结构性地满足本端口——`delivery/dispatchDelivery/resolveDelivery` 三方法同名同参）。
- Produces（Task 6/8 消费）：

```ts
export interface DeliveryRecoveryRemote {
  dispatchDelivery(input: { runId: string; deliveryId: string }): Promise<DeliveryRemoteRecord>;
  resolveDelivery(input: { runId: string; deliveryId: string }): Promise<DeliveryRemoteRecord>;
}
export type DeliveryRecoveryErrorCode = 'DELIVERY_SCOPE_CHANGED' | 'DELIVERY_STATE_CONFLICT' | 'DELIVERY_INVALID_INPUT' | 'DELIVERY_BACKEND';
export class DeliveryRecoveryError extends Error { readonly code: DeliveryRecoveryErrorCode }
export interface DeliveryRecovery {
  recover(input: { runId: string; deliveryId: string }): Promise<DeliveryReceiptView>;
}
export function createDeliveryRecovery(ports: {
  remote: DeliveryRecoveryRemote & Pick<DeliveryRemote, 'delivery'>;
  lease(): ScopeLease | undefined;
}): DeliveryRecovery;
```

`recover` 语义（固定顺序，写进实现注释）：①lease 前置守卫；②`remote.delivery(runId)` 读现状——`null` ⇒ `DELIVERY_INVALID_INPUT`（无交付可恢复）；③状态路由：`delivered` ⇒ 直接返回视图（**零写请求**，幂等重入）；`pushed` ⇒ `dispatchDelivery`（PR-only 半程）；`unknown` ⇒ `resolveDelivery`（远端事实收敛）；`prepared`/`dispatched`/`failed` ⇒ `DELIVERY_STATE_CONFLICT`（消息含 state 名——prepared 待审批、dispatched 派发在途均不是恢复窗口）；④写调用异常：ApiError 双形状（顶层 `.status === 409` 或 `.code`/`.body?.code` 等于 `code_delivery_state_conflict`）⇒ `DELIVERY_STATE_CONFLICT`，其余 ⇒ `DELIVERY_BACKEND`；⑤写返回后 lease 复验（在途撤销 ⇒ `DELIVERY_SCOPE_CHANGED`，迟到结果丢弃）；⑥`deliveryViewOf(record)` 返回。409 双形状判定写成模块内私有 helper（mobile-core 不 import api-client——`app-smoke.test.tsx:592-595` 的 Interface-only 纪律），形态镜像 `delivery-reader.ts` 的 `DeliveryRemote` 依赖方向。

- [x] **Step 1: 写失败测试**

新建 `packages/mobile-core/src/delivery/delivery-recovery.test.ts`（lease 铸造范式复用 `delivery-reader.test.ts:16-21` 的 `RuntimeScopeLease`）：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { createDeliveryRecovery, DeliveryRecoveryError, type DeliveryRecoveryRemote } from './delivery-recovery.ts';
import type { DeliveryRemote, DeliveryRemoteRecord } from './delivery-reader.ts';
import { RuntimeScopeLease } from '../runtime/scope-lease.ts';
import type { ScopeLease } from '../runtime/types.ts';

function mintLease(): { lease: ScopeLease; revoke: () => void } {
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  return { lease: internal.asScopeLease(), revoke: () => internal.revoke() };
}

function recordWith(state: DeliveryRemoteRecord['state']): DeliveryRemoteRecord {
  return {
    id: 'dlv-1', taskId: 's-1', runId: 'run-1', state,
    repo: 'octocat/hello', baselineSha: 'b'.repeat(40), branch: 'weknora/task/s-1',
    commitSha: 'c1', actionId: 'act-1', actionState: 'dispatched', digest: 'd1', files: 1,
    createdAt: '2026-09-24T00:00:00Z', updatedAt: '2026-09-24T00:00:30Z',
  };
}

function harness(state: DeliveryRemoteRecord['state']) {
  const calls: string[] = [];
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { calls.push('delivery'); return recordWith(state); },
    async dispatchDelivery() { calls.push('dispatchDelivery'); return recordWith('delivered'); },
    async resolveDelivery() { calls.push('resolveDelivery'); return recordWith('pushed'); },
  };
  const { lease, revoke } = mintLease();
  return {
    calls,
    revoke,
    recover: createDeliveryRecovery({ remote, lease: () => (revoked ? undefined : lease) }).recover,
    revoked: false as boolean,
    setRevoked(value: boolean) { revoked = value; },
  };
}

test('recover routes pushed to the PR-only dispatch half exactly once', async () => {
  const h = harness('pushed');
  const view = await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(view.state, 'delivered');
  assert.deepEqual(h.calls, ['delivery', 'dispatchDelivery']);
});

test('recover routes unknown to the remote-facts resolve half', async () => {
  const h = harness('unknown');
  const view = await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.deepEqual(h.calls, ['delivery', 'resolveDelivery']);
  assert.equal(view.state, 'pushed');
});

test('recover on delivered is idempotent — zero write requests (re-entry safety)', async () => {
  const h = harness('delivered');
  const view = await h.recover({ runId: 'run-1', deliveryId: 'dlv-1' });
  assert.equal(view.state, 'delivered');
  assert.deepEqual(h.calls, ['delivery']);
});

test('prepared/dispatched/failed are not recovery windows — DELIVERY_STATE_CONFLICT', async () => {
  for (const state of ['prepared', 'dispatched', 'failed'] as const) {
    const h = harness(state);
    await assert.rejects(
      () => h.recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
      (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_STATE_CONFLICT' && error.message.includes(state),
    );
    assert.deepEqual(h.calls, ['delivery'], `${state} must not trigger any write`);
  }
});

test('no delivery to recover fails closed with DELIVERY_INVALID_INPUT', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return null; },
    async dispatchDelivery() { throw new Error('must not be called'); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  const { lease } = mintLease();
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_INVALID_INPUT',
  );
});

test('no lease fails closed with DELIVERY_SCOPE_CHANGED before any network', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { throw new Error('must not be called'); },
    async dispatchDelivery() { throw new Error('must not be called'); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => undefined }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED',
  );
});

test('a lease revoked while the write was in flight drops the late result — DELIVERY_SCOPE_CHANGED', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return recordWith('pushed'); },
    async dispatchDelivery() { revoke(); return recordWith('delivered'); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  const internal = new RuntimeScopeLease({ deploymentOrigin: 'https://weknora.example.com', userId: 'u1', tenantId: 't1' });
  const lease = internal.asScopeLease();
  const revoke = (): void => internal.revoke();
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_SCOPE_CHANGED',
  );
});

test('a 409 state conflict translates to DELIVERY_STATE_CONFLICT (both ApiError shapes)', async () => {
  for (const shape of [
    Object.assign(new Error('api error 409'), { status: 409, code: 'code_delivery_state_conflict' }),
    Object.assign(new Error('api error 409'), { status: 409, body: { code: 'code_delivery_state_conflict' } }),
  ]) {
    const remote: DeliveryRemote & DeliveryRecoveryRemote = {
      async delivery() { return recordWith('pushed'); },
      async dispatchDelivery() { throw shape; },
      async resolveDelivery() { throw new Error('must not be called'); },
    };
    const { lease } = mintLease();
    await assert.rejects(
      () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
      (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_STATE_CONFLICT',
    );
  }
});

test('any other backend failure translates to DELIVERY_BACKEND', async () => {
  const remote: DeliveryRemote & DeliveryRecoveryRemote = {
    async delivery() { return recordWith('pushed'); },
    async dispatchDelivery() { return Promise.reject(new Error('network down')); },
    async resolveDelivery() { throw new Error('must not be called'); },
  };
  const { lease } = mintLease();
  await assert.rejects(
    () => createDeliveryRecovery({ remote, lease: () => lease }).recover({ runId: 'run-1', deliveryId: 'dlv-1' }),
    (error: unknown) => error instanceof DeliveryRecoveryError && error.code === 'DELIVERY_BACKEND',
  );
});
```

Run: `pnpm exec tsx --test packages/mobile-core/src/delivery/delivery-recovery.test.ts`
Expected: FAIL（模块不存在，加载错误）。

- [x] **Step 2: 实现 `delivery-recovery.ts`**

按 Interfaces 段的签名与固定顺序语义实现；`DeliveryRecoveryError` 形态镜像 `delivery-reader.ts:11-17`（`readonly code` + constructor）。

- [x] **Step 3: 运行测试验证 GREEN + typecheck**

Run: `pnpm exec tsx --test packages/mobile-core/src/delivery/delivery-recovery.test.ts && pnpm --filter @weknora/mobile typecheck`
Expected: PASS + 类型检查通过。类型检查用 apps/mobile 的 tsc 而非 mobile-core 自身——`packages/mobile-core` 无独立 tsconfig（`pnpm --filter @weknora/mobile-core exec tsc --noEmit` 会因无输入文件只打印帮助文本，不是有效验证，勿用）；mobile-core 以 TS 源直出（`package.json` exports `"./src/index.ts"`），其类型经 apps/mobile 的 tsc 程序传递覆盖（`delivery-view.ts:6-7` 注释即声称由该路径证明同构；`pnpm --filter @weknora/mobile typecheck` 计划作者实跑 exit 0）。

- [x] **Step 4: barrel 导出**

`packages/mobile-core/src/index.ts:103-107` 的 delivery 导出段追加：

```ts
export { createDeliveryRecovery, DeliveryRecoveryError } from './delivery/delivery-recovery.ts';
export type { DeliveryRecovery, DeliveryRecoveryErrorCode, DeliveryRecoveryRemote } from './delivery/delivery-recovery.ts';
```

Run: `pnpm --filter @weknora/mobile typecheck`
Expected: 通过（理由同 Task 5 Step 3——mobile-core 无独立 tsconfig，类型检查经 apps/mobile 的 tsc 传递覆盖）。

- [x] **Step 5: Commit**

```bash
git add packages/mobile-core/src/delivery/delivery-recovery.ts packages/mobile-core/src/delivery/delivery-recovery.test.ts packages/mobile-core/src/index.ts
git commit -m "feat(mobile-core): createDeliveryRecovery — state-routed partial-success recovery (pushed→PR-only, unknown→remote facts, delivered→idempotent no-op) under scope-lease guards (T25 #55)"
```

### Task 6: TS apps/mobile——恢复装配、TaskDetailScreen 恢复操作与移动 Terminal 无输入通道防护

**Files:**
- Modify: `apps/mobile/src/composition.ts:334-350`（delivery 装配段）
- Modify: `apps/mobile/src/screens/TaskDetailScreen.tsx`（props、`DELIVERY_RECOVERY_COPY`、`DeliveryReceiptSection`）
- Modify: `apps/mobile/src/app/tasks/detail.tsx`（恢复回调与错误态）
- Test: `apps/mobile/src/terminal-purity.test.ts`（新建）
- Test: `apps/mobile/src/app-smoke.test.tsx`（追加一个 test 块）

**Interfaces:**
- Consumes: Task 5 的 `createDeliveryRecovery`/`DeliveryRecovery`；Task 4 的 `createMobileCodeDeliveryRemote`（composition 已 import，`composition.ts:17`）；既有 `cachePut`/`deploymentScopeKey`/`runtime()` 范式（`composition.ts:256-278`、`:334-350`）；既有 `DELIVERY_STATE_COPY`（`TaskDetailScreen.tsx:44-52`）。
- Produces: `activeDeliveryRecovery(): DeliveryRecovery | undefined`（composition 公共函数，Task 8 消费）；`TaskDetailScreenProps.onRecoverDelivery?: (input: { runId: string; deliveryId: string }) => Promise<DeliveryReceiptView>` 与 `recoveryError?: string`；导出常量 `DELIVERY_RECOVERY_COPY`。

- [ ] **Step 1: 写失败测试（terminal 纯度源级扫描）**

新建 `apps/mobile/src/terminal-purity.test.ts`：

```ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));

function tsFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = join(dir, name);
    if (statSync(full).isDirectory()) return name === 'node_modules' ? [] : tsFiles(full);
    return /\.(ts|tsx)$/.test(name) && !/\.test\./.test(name) ? [full] : [];
  });
}

// T25 (#55) AC2：移动装配图结构性无终端输入通道——src 源树（除测试）零命中
// terminal 门票签发与 WS 终端装配词。移动端的终端面只有材料域的只读投影
// （MATERIAL_TERMINAL_READ_ONLY，task-material.ts:227）。
test('the mobile source tree wires no terminal ticket or interactive terminal channel (T25 #55 AC2)', () => {
  for (const file of tsFiles(here)) {
    const source = readFileSync(file, 'utf8');
    for (const marker of ['createSandboxTerminalApi', 'issueTicket', 'sandbox/terminal-ticket']) {
      assert.equal(source.includes(marker), false, `${file} must not reference ${marker}: the mobile terminal is read-only by construction`);
    }
  }
});

test('the composition wires the delivery recovery interface (T25 #55 AC1)', () => {
  const composition = readFileSync(join(here, 'composition.ts'), 'utf8');
  assert.match(composition, /createDeliveryRecovery/, 'composition must wire the delivery recovery module');
  assert.match(composition, /activeDeliveryRecovery/, 'composition must expose activeDeliveryRecovery for the detail route');
});
```

Run: `pnpm exec tsx --test apps/mobile/src/terminal-purity.test.ts`
Expected: FAIL（第 2 个测试——composition 尚无 `createDeliveryRecovery`；第 1 个测试现状应已 PASS，它是钉死性断言）。

- [ ] **Step 2: 写失败测试（Screen 恢复操作渲染）**

在 `apps/mobile/src/app-smoke.test.tsx` 文件尾部追加一个 test 块（复用文件内既有 `render`/`descendants` helper 与 `:566-577` 的 `view` 构造范式）：

```tsx
test('the delivery receipt section offers the recovery action for partial and unknown states only (T25 #55 AC1)', async () => {
  const { TaskDetailScreen } = await import('./screens/TaskDetailScreen.tsx');
  const view: import('@weknora/mobile-core').TaskDetailView = {
    taskId: 'task-1', runId: 'run-1', title: '交付', lifecycle: 'active', runStatus: 'running', attention: 'required',
    executionStatus: 'running', settlementStatus: 'pending', revision: 1, cursor: 2, incomplete: false, connection: 'live',
    timeline: [], duplicateSeqs: [],
  };
  const pushedReceipt: import('@weknora/mobile-core').DeliveryReceiptView = {
    deliveryId: 'dlv-1', taskId: 'task-1', runId: 'run-1', state: 'pushed',
    repo: 'octocat/hello', branch: 'weknora/task/s-1', baselineSha: 'b'.repeat(40),
    commitSha: 'c1', attention: true, updatedAt: '2026-09-24T00:00:30Z',
  };
  const withRecovery = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {},
    delivery: pushedReceipt, onRecoverDelivery: async () => pushedReceipt,
  });
  const pushedJson = JSON.stringify(withRecovery);
  assert.ok(pushedJson.includes('已推送，等待草稿 PR/MR 恢复'), 'the honest partial-completion copy renders');
  assert.ok(pushedJson.includes('恢复创建草稿 PR/MR'), 'the pushed state offers the recovery action');
  const recovered: import('@weknora/mobile-core').DeliveryReceiptView = { ...pushedReceipt, state: 'delivered' };
  const settled = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {},
    delivery: recovered,
  });
  assert.equal(JSON.stringify(settled).includes('恢复创建草稿 PR/MR'), false, 'a delivered receipt offers no recovery action');
  const noCallback = render(TaskDetailScreen, {
    view, loading: false, error: undefined, onRefresh: () => {}, delivery: pushedReceipt,
  });
  assert.equal(JSON.stringify(noCallback).includes('恢复创建草稿 PR/MR'), false, 'without the callback the action stays hidden (fail closed)');
});
```

Run: `pnpm exec tsx --test apps/mobile/src/app-smoke.test.tsx`
Expected: 新测试 FAIL（`恢复创建草稿 PR/MR` 不存在/props 不被接受）。

- [ ] **Step 3: 实现 composition 装配**

`composition.ts` delivery 装配段（`:334-350`）追加（import 区补 `createDeliveryRecovery`、类型 `DeliveryRecovery` 从 `@weknora/mobile-core`——该 import 已存在，只加名）：

```ts
const deliveryRecoveries = new Map<string, DeliveryRecovery>();

/** Delivery recovery 按 deployment scope key 记忆化；同一 remote 结构实现读与恢复两端口
 * （镜像 taskOfficeFor 的 `backend: remote, detail: remote` 范式，T05）。 */
function deliveryRecoveryFor(activeRuntime: MobileRuntime, origin: string, tenantId: string): DeliveryRecovery {
  return cachePut(deliveryRecoveries, deploymentScopeKey(origin, tenantId), () => {
    const remote = createMobileCodeDeliveryRemote({ origin, request: (input) => activeRuntime.authorizedRequest(input) });
    return createDeliveryRecovery({ remote, lease: () => activeRuntime.scopeLease() });
  });
}

/** 详情路由经此取当前授权 scope 的交付恢复器（无授权面返回 undefined）。 */
export function activeDeliveryRecovery(): DeliveryRecovery | undefined {
  const activeRuntime = runtime();
  const snapshot = activeRuntime.snapshot();
  if (snapshot.surface !== 'authorized' || !snapshot.deployment || !snapshot.identity?.userId) return undefined;
  return deliveryRecoveryFor(activeRuntime, snapshot.deployment.origin, snapshot.identity.activeTenantId ?? '');
}
```

- [ ] **Step 4: 实现 Screen 恢复操作**

`TaskDetailScreen.tsx`：
1. `TaskDetailScreenProps` 追加 `onRecoverDelivery?: (input: { runId: string; deliveryId: string }) => Promise<DeliveryReceiptView>;` 与 `recoveryError?: string;`（JSDoc 注明：T25 恢复入口；未提供时操作不渲染，fail closed 的呈现面——镜像 `onAct` 的注释纪律）。
2. 导出常量：

```ts
/** T25 (#55) 恢复操作文案：按状态给恰一个动作，失败态错误不粉饰。 */
export const DELIVERY_RECOVERY_COPY = {
  pushed: '恢复创建草稿 PR/MR',
  unknown: '核对远端结果',
  failed: '当前状态无法恢复：请刷新后查看最新交付状态',
} as const;
```

3. `DeliveryReceiptSection` 签名扩为 `({ delivery, onRecover, recoveryError }: { delivery: DeliveryReceiptView; onRecover?: () => void; recoveryError?: string })`：`pushed` 态渲染 `<Button title={DELIVERY_RECOVERY_COPY.pushed} onPress={onRecover} />`、`unknown` 态渲染 `DELIVERY_RECOVERY_COPY.unknown` 按钮（同一 `onRecover`——服务端/模块按状态路由半程）；`recoveryError !== undefined` 时追加一行错误文案；其余状态零操作。`TaskDetailScreen` 主体把 `onAct` 同款受控转发接上（`onRecoverDelivery === undefined ? undefined : () => { void onRecoverDelivery({ runId: view.runId, deliveryId: delivery.deliveryId }); }`）。

`app/tasks/detail.tsx`：
1. state：`const [recoveryError, setRecoveryError] = useState<string | undefined>(undefined);`
2. 回调（镜像既有 `onAct` 的 then/catch 吞错范式，但恢复错误要呈现）：

```ts
const onRecoverDelivery = activeDeliveryRecovery() === undefined ? undefined : (input: { runId: string; deliveryId: string }) => {
  setRecoveryError(undefined);
  const recovery = activeDeliveryRecovery();
  if (recovery === undefined) return Promise.reject(new Error('unauthorized'));
  return recovery.recover(input).then((view) => { setDelivery(view); return view; }, (error: unknown) => {
    setRecoveryError(error instanceof DeliveryRecoveryError ? (DELIVERY_RECOVERY_COPY.failed + (error.message ? `（${error.message}）` : '')) : '恢复请求失败，请稍后重试');
    throw error;
  });
};
```

（`DeliveryRecoveryError` 从 `@weknora/mobile-core` import；`DELIVERY_RECOVERY_COPY` 从 `../../screens/TaskDetailScreen.tsx` import——同 app 内既有相互 import 范式。）3. 传参：`<TaskDetailScreen ... delivery={delivery} recoveryError={recoveryError} onRecoverDelivery={onRecoverDelivery} />`。

- [ ] **Step 5: 运行测试验证 GREEN**

Run: `pnpm exec tsx --test apps/mobile/src/terminal-purity.test.ts apps/mobile/src/app-smoke.test.tsx`
Expected: PASS（含既有全部断言——`app-smoke.test.tsx:592-595` 的 Interface-only 断言必须仍然成立：`TaskDetailScreen.tsx`/`task-detail-view.ts`/`app/tasks/detail.tsx` 不得 import `@weknora/api-client`，本任务的 import 全部走 `@weknora/mobile-core` 与 app 内相对路径）。

- [ ] **Step 6: 全量回归 + typecheck**

Run: `pnpm --filter @weknora/mobile test && pnpm --filter @weknora/mobile typecheck`
Expected: 0 fail；typecheck 通过。

- [ ] **Step 7: Commit**

```bash
git add apps/mobile/src/composition.ts apps/mobile/src/screens/TaskDetailScreen.tsx apps/mobile/src/app/tasks/detail.tsx apps/mobile/src/terminal-purity.test.ts apps/mobile/src/app-smoke.test.tsx
git commit -m "feat(mobile): delivery recovery action on the task detail receipt + structural guard that the mobile tree wires no terminal input channel (T25 #55)"
```

### Task 7: Go——真实 Provider 恢复收敛 blocked-env loop（env 门控）

**Files:**
- Test: `internal/modules/codedelivery/github_real_test.go`（追加）

**Interfaces:**
- Consumes: 既有 env 键 `WEKNORA_GITHUB_TEST_TOKEN`/`WEKNORA_GITHUB_TEST_REPO`（`github_real_test.go:15-16`）+ 新增第三键 `WEKNORA_GITHUB_TEST_WRITABLE=1`；包内夹具类型 `fixtureRun`/`fixtureConnections`（`service_prepare_test.go:30-77`）与 `GitHubAPIBaseURL`/`httpClientDefault()`（`github_real_test.go:22`）。
- Produces: `TestGitHubRealRecoveryLoopNoRepeatPush`——真实环境「delivered 后重复派发被拒 + 恢复读面幂等」证据（AC3 真实面；本环境 blocked-env skip）。

**边界声明（写进测试注释）**：真实平台无法注入「PR 创建确定性失败」，因此真实部分成功（pushed）闭环不可确定性制造——该分支的最高稳定 Interface 证据是 Task 2 的 wire 契约 e2e（`github_wire_test.go` 钉合真实 API 形状）；本测试覆盖真实环境可确定性的部分：完整真实交付→delivered→重复 dispatch 被服务端状态机拒绝（HTTP 409）→读面幂等。测试会在真实仓库留下任务分支与草稿 PR（`weknora/task/<session>` 前缀），**必须使用专用测试仓库**，注释中写明。

- [x] **Step 1: 写测试（追加到 `github_real_test.go`；文件头 import 补 `"context"`/`"fmt"`/`"path/filepath"`/`"time"` 与 `appconnectorrepo`/`appconnectorsvc`/`deliveryrepo`/`"gorm.io/driver/sqlite"`/`"gorm.io/gorm"`/`"gorm.io/gorm/logger"`——既有 import 仅 context/os/testing/require）**

```go
// realLoopConnections mirrors fixtureConnections (service_prepare_test.go:41)
// but hands out the REAL token from the environment — credentials are read
// from env only, never literals.
type realLoopConnections struct {
	db      *gorm.DB
	members map[string]bool
}

func (s *realLoopConnections) FindConnectionByID(ctx context.Context, id string) (appconnector.Connection, error) {
	var row appconnectorrepo.ConnectionRow
	if err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error; err != nil {
		return appconnector.Connection{}, err
	}
	return appconnector.Connection{
		ID: row.ID, InstallationID: row.InstallationID, Kind: row.Kind,
		OwnerID: row.OwnerID, CredentialRef: row.CredentialRef,
		State: row.State, TenantID: row.TenantID, AuthVersion: row.AuthVersion,
	}, nil
}

func (s *realLoopConnections) LoadCredential(ctx context.Context, c appconnector.Connection) ([]byte, error) {
	return []byte(os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")), nil
}

func (s *realLoopConnections) MemberActive(ctx context.Context, tenantID uint64, userID string) (bool, error) {
	return s.members[userID], nil
}

func (s *realLoopConnections) TryAcquireRefreshLease(ctx context.Context, c appconnector.Connection, leaseID string, until time.Time) (bool, error) {
	return true, nil
}

func (s *realLoopConnections) Resolve(ctx context.Context, connectionID string, expectedVersion int64) ([]byte, error) {
	return []byte(os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")), nil
}

// realLoopBaselineSHA anchors the delivery on the default branch's CURRENT
// head (a real 40-hex commit sha the real API accepts).
func realLoopBaselineSHA(t *testing.T, client GitHubClient) string {
	t.Helper()
	info, err := client.Repository(context.Background())
	require.NoError(t, err)
	head, exists, err := client.BranchHead(context.Background(), info.DefaultBranch)
	require.NoError(t, err)
	require.True(t, exists, "the default branch must exist on the real repo")
	require.Len(t, head, 40)
	return head
}

// TestGitHubRealRecoveryLoopNoRepeatPush 需要 WEKNORA_GITHUB_TEST_TOKEN（对
// WEKNORA_GITHUB_TEST_REPO 有写权限的 PAT）、WEKNORA_GITHUB_TEST_REPO（owner/name，
// 专用测试仓库——本测试会真实推送任务分支并创建草稿 PR）与
// WEKNORA_GITHUB_TEST_WRITABLE=1 三者同时在场。缺任一即 skip（blocked-env，
// 不伪造）。真实部分成功（pushed）需要平台侧 PR 失败注入、不可确定性制造：
// 该分支的等价证据是 delivery_recovery_http_test.go 的 wire 契约 e2e；本测试
// 覆盖真实环境可确定性的恢复收敛面：完整交付→delivered→重复 dispatch 被状态
// 机拒绝（不重复推送）→读面幂等。
func TestGitHubRealRecoveryLoopNoRepeatPush(t *testing.T) {
	token := os.Getenv("WEKNORA_GITHUB_TEST_TOKEN")
	repoRaw := os.Getenv("WEKNORA_GITHUB_TEST_REPO")
	if token == "" || repoRaw == "" || os.Getenv("WEKNORA_GITHUB_TEST_WRITABLE") != "1" {
		t.Skip("WEKNORA_GITHUB_TEST_TOKEN/WEKNORA_GITHUB_TEST_REPO/WEKNORA_GITHUB_TEST_WRITABLE not set (blocked-env)")
	}
	repo, err := ParseRepoRef(repoRaw)
	require.NoError(t, err)

	// 装配镜像 newDeliveryFixture（service_prepare_test.go:102-149），但 GitHub
	// factory 指向真实 API；workspace/DB/审批链全部真实（同包私有类型复用）。
	dsn := "file:" + filepath.Join(t.TempDir(), "real.db") + "?_foreign_keys=on&_busy_timeout=5000"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&appconnectorrepo.ActionRow{}, &appconnectorrepo.ApprovalRow{}, &appconnectorrepo.PreAuthorizationRow{},
		&appconnectorrepo.ConnectionRow{}, &appconnectorrepo.InstallationRow{}, &deliveryrepo.DeliveryRow{},
	))
	require.NoError(t, db.Create(&appconnectorrepo.InstallationRow{ID: "inst-gh", AppID: "github", AppVersion: "1", State: appconnector.InstallationActive, TenantID: 7}).Error)
	require.NoError(t, db.Create(&appconnectorrepo.ConnectionRow{
		ID: "conn-gh", InstallationID: "inst-gh", Kind: appconnector.ConnectionKindPersonal,
		OwnerID: "u1", CredentialRef: "mcp:conn-gh:github", State: appconnector.ConnectionActive,
		TenantID: 7, AuthVersion: 1,
	}).Error)

	root := t.TempDir()
	sessionID := fmt.Sprintf("s-real-%d", time.Now().UnixNano())
	dir := filepath.Join(root, repo.Owner, repo.Name)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# real loop\n"), 0o644))
	workspace, err := NewLocalWorkspaceSource(root)
	require.NoError(t, err)

	factory := NewGitHubClientFactory(httpClientDefault(), GitHubAPIBaseURL)
	client := factory(token, repo)
	baseline := realLoopBaselineSHA(t, client)

	actionStore := appconnectorrepo.NewActionStore(db)
	store := deliveryrepo.NewDeliveryStore(db)
	connections := &realLoopConnections{db: db, members: map[string]bool{"u1": true}}
	guard := appconnectorsvc.NewSubjectGuard(connections)
	dispatcher := NewDeliveryDispatcher(DispatcherDeps{
		Connections: connections, Creds: connections, Guard: guard,
		GitHub: factory, Workspace: workspace, Store: store,
		ActionRows: actionStore, Runs: fixtureRun{sessionID: sessionID},
	})
	actions := appconnectorsvc.NewActionService(actionStore, guard, nil, dispatcher, dispatcher)
	svc := NewCodeDeliveryService(CodeDeliveryDeps{
		Store: store, Actions: actions, ActionRows: actionStore,
		Connections: connections, Creds: connections,
		GitHub: factory, Providers: appconnectorrepo.NewInstallationStore(db),
		Workspace: workspace, Runs: fixtureRun{sessionID: sessionID}, Dispatcher: dispatcher,
	})

	ctx := context.Background()
	view, err := svc.PrepareDelivery(ctx, PrepareInput{
		TenantID: 7, CallerID: "u1", RunID: "run-1", ConnectionID: "conn-gh",
		Repo: repo, BaselineSHA: baseline, CommitMessage: "t25 real recovery loop", PRTitle: "t25 real recovery loop",
	})
	require.NoError(t, err)
	require.NoError(t, actions.Approve(ctx, view.ActionID, "u1", view.Digest))
	view, err = svc.DispatchDelivery(ctx, DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID})
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), view.State)
	require.NotEmpty(t, view.PRURL)

	// The duplicate dispatch must be refused by the state machine — no
	// second push leaves the process.
	_, err = svc.DispatchDelivery(ctx, DispatchInput{TenantID: 7, CallerID: "u1", RunID: "run-1", DeliveryID: view.ID})
	require.ErrorIs(t, err, ErrDeliveryState)
	again, err := svc.GetDelivery(ctx, 7, view.ID)
	require.NoError(t, err)
	require.Equal(t, string(DeliveryDelivered), again.State, "the read face stays idempotent after the refused duplicate")
}
```

- [x] **Step 2: 运行确认 blocked-env skip 形态**

Run: `go test ./internal/modules/codedelivery/ -run 'TestGitHubRealRecoveryLoopNoRepeatPush' -count=1 -v`
Expected: SKIP（`WEKNORA_GITHUB_TEST_TOKEN/.../WEKNORA_GITHUB_TEST_WRITABLE not set (blocked-env)`）——本环境无真实凭据，如实 skip；`go vet` 级编译通过。

- [x] **Step 3: 编译与邻近回归**

Run: `go test ./internal/modules/codedelivery/ -count=1`
Expected: PASS（既有全部测试 + 新增 1 skip）。

- [x] **Step 4: Commit**

```bash
git add internal/modules/codedelivery/github_real_test.go
git commit -m "test(codedelivery): real-GitHub recovery convergence loop (env-gated, blocked-env skip) — duplicate dispatch after delivery is refused with zero re-push"
```

### Task 8: TS——交付集成证据扩展恢复段（opt-in）

**Files:**
- Modify: `apps/mobile/src/delivery-integration-smoke.ts`
- Test: `apps/mobile/src/delivery-integration-smoke.test.ts`（追加）

**Interfaces:**
- Consumes: Task 5 的 `createDeliveryRecovery`（与 `createMobileCodeDeliveryRemote` 同 remote 结构装配，镜像 `composition.ts` 的 `deliveryRecoveryFor`）；既有 `deliveryIntegrationConfig(env)`（`delivery-integration-smoke.ts:27-44`）与 `MobileRuntime.authorizedRequest`（`packages/mobile-core/src/runtime/mobile-runtime.ts:383-388`）/`scopeLease`（同文件 `:413`）。
- Produces: `DeliveryIntegrationEvidence` 新字段 `recovery: 'skipped' | 'not-needed' | 'recovered' | 'failed'` 与 `recoveryState?: string`；config 新增可选段 `recover?: boolean`（env `WEKNORA_MOBILE_TEST_DELIVERY_RECOVER=1` 时 true）。

- [ ] **Step 1: 写失败测试**

`apps/mobile/src/delivery-integration-smoke.test.ts` 追加：

```ts
test('the recover flag is opt-in and defaults to off', () => {
  const base = {
    WEKNORA_MOBILE_TEST_DEPLOYMENT_URL: 'https://weknora.example.com',
    WEKNORA_MOBILE_TEST_EMAIL: 'a@b.c',
    WEKNORA_MOBILE_TEST_PASSWORD: 'pw',
  };
  // 收窄形态（enabled === true 先判）：DeliveryIntegrationConfig 是判别联合，
  // enabled:false 分支无 recover 字段——直接 .recover 访问会在 tsc 下报错。
  const off = deliveryIntegrationConfig(base);
  assert.equal(off.enabled === true && off.recover === false, true);
  const on = deliveryIntegrationConfig({ ...base, WEKNORA_MOBILE_TEST_DELIVERY_RECOVER: '1' });
  assert.equal(on.enabled === true && on.recover === true, true);
  assert.equal(deliveryIntegrationConfig({}).enabled, false);
});
```

Run: `pnpm exec tsx --test apps/mobile/src/delivery-integration-smoke.test.ts`
Expected: FAIL（`recover` 不存在）。

- [ ] **Step 2: 实现恢复段**

`delivery-integration-smoke.ts`：
1. config 返回类型 enabled-true 形态加 `recover: boolean`，解析处：`recover: env.WEKNORA_MOBILE_TEST_DELIVERY_RECOVER === '1'`。
2. `DeliveryIntegrationEvidence` 加 `recovery: 'skipped' | 'not-needed' | 'recovered' | 'failed'`（初始 `'skipped'`）与 `recoveryState?: string`。
3. `runDeliveryIntegration` 主流程（`:53-100`）在 `evidence.deliveryRead === 'read'` 分支内：`config.recover` 为 false 时保持 `'skipped'`；为 true 时以 Task 6 同款装配构造 recovery（`createDeliveryRecovery({ remote: createMobileCodeDeliveryRemote({ origin, request: (input) => runtime.authorizedRequest(input) }), lease: () => runtime.scopeLease() })`——remote 复用主流程已构造的实例），对读到的 record 调 `recover({ runId, deliveryId: record.id })`：成功 ⇒ `recovery: 'recovered'` + `recoveryState: 结果视图 state`；`DeliveryRecoveryError` 且 code 为 `DELIVERY_STATE_CONFLICT`/`DELIVERY_INVALID_INPUT` ⇒ `'not-needed'`（状态本就不在恢复窗口——如实记录，不算失败）；其余错误 ⇒ `'failed'` + `evidence.failure` 追记。
4. `emitDeliveryIntegrationEvidence`（`:107-109`）零改动（spread 已携带新字段）。

- [ ] **Step 3: 运行验证 GREEN**

Run: `pnpm exec tsx --test apps/mobile/src/delivery-integration-smoke.test.ts && pnpm --filter @weknora/mobile typecheck`
Expected: PASS + typecheck 通过。

- [ ] **Step 4: Commit**

```bash
git add apps/mobile/src/delivery-integration-smoke.ts apps/mobile/src/delivery-integration-smoke.test.ts
git commit -m "test(mobile): opt-in delivery recovery leg in the integration evidence — state-routed recovery against a real deployment (T25 #55)"
```

---

## 计划级验证命令

在 worktree 根（`.worktrees/issue30-sweep`）执行：

```bash
go build ./... && go test ./internal/application/repository/ -run 'TestDeliveryCollaboration|TestT25' -count=1 && go test ./internal/modules/execution/sandbox/ -run 'TestWithWorkspaceEnvDefaults' -count=1 && go test ./internal/modules/codedelivery/... -count=1 && pnpm exec tsx --test packages/api-client/src/sandbox/terminal-surface.test.ts packages/api-client/src/mobile/code-delivery.test.ts packages/mobile-core/src/delivery/delivery-recovery.test.ts packages/mobile-core/src/delivery/delivery-reader.test.ts && pnpm --filter @weknora/mobile typecheck && pnpm exec tsx --test apps/mobile/src/terminal-purity.test.ts apps/mobile/src/delivery-integration-smoke.test.ts && pnpm --filter @weknora/mobile test
```

覆盖：Task 1（`TestDeliveryCollaboration`）、Task 2（`TestT25`）、Task 3（`TestWithWorkspaceEnvDefaults`）、Task 4–5（api-client/mobile-core 测试 + tsc）、Task 6–8（apps/mobile 文件级 + 全量 test + typecheck）、codedelivery 全包回归（既有 #52/#54/R5 测试防破坏）。

## 基线证据（计划作者在当前 HEAD 实跑）

- `go build ./...`：exit 0（仅 ld duplicate-library warning）。
- `go test ./internal/modules/codedelivery/ -run 'TestPartialPushPRFailureRecoversWithoutRepush|TestUnknownOutcomeResolvesFromRemoteFacts' -count=1`：ok（0.559s）——服务面恢复语义已在场。
- `go test ./internal/handler/session/ -run 'TestDelivery' -count=1`：ok（0.816s）。
- `pnpm exec tsx --test packages/mobile-core/src/delivery/delivery-reader.test.ts packages/api-client/src/mobile/code-delivery.test.ts`：8 pass / 0 fail。
- `pnpm exec tsx --test apps/mobile/src/delivery-integration-smoke.test.ts`：2 pass / 0 fail。
- `go test ./internal/application/repository/ -run 'TestDeliveryCollaboration' -count=1`：**FAIL（3 个测试，`400 code_delivery_unsupported_provider`）**——既有破损，Task 1 修复（差异记录第 2 条）。

## 差异记录（调查结论与代码现状的冲突，以代码现状为准）

1. **调查摘要过时**：Issue 调查称「前置 #52、#54 均 absent」「Delivery 本身不存在」「apps/mobile 仅登录壳」。代码现状：#52/#54/#46 已全部交付并集成——`internal/modules/codedelivery/`（dispatcher/service/store/wire/gitlab 全链，worktree `git log` 可证）、`packages/api-client/src/mobile/code-delivery.ts`、`packages/mobile-core/src/delivery/`、`apps/mobile` 的 TaskDetailScreen 交付区块与 material terminal 只读面均在。本计划因此不是从零建交付，而是补 T25 三缺口：wire 级 e2e 证据、移动恢复闭环、凭据隔离钉死。
2. **既有 e2e 破损（Task 1）**：#53 的 `TestDeliveryCollaboration*` 在当前 HEAD FAIL——#54 引入 `Providers` fail-closed（`service.go:477-489`）后未同步 #53 的装配。实跑输出见「基线证据」。
3. **B5 OCR 已知缺口处置核实**：任务指引点名的 codedelivery 缺口（`gitlab_client.go:433` MR 复用缺 target 维度、`:128` 8MB 截断、`:351` 空收敛 unknown 无自愈、`:353` 全量增量膨胀）经核实已由 R5 修复轮处理——commits `6b6b398c7`（R5-F8/F10：MR source+target 分页解析、2xx 解码失败诚实分类）、`a8f817647`（R5-F6/F9：cap 超限检测与 blob 内容寻址校验）、`42c933a88`（R5-F11/F13：任务分支锚定基线、本地拒绝落 not-started）。`dispatcher.go:270-279` 的 QueryProvider 已带 target 维度（R5-F8 注释在场）。本计划零重复修复；`TestGitLabPartialPushMRFailureRecoversWithoutRepush`/`TestQueryProviderIgnoresForeignTargetMR` 等既有回归持续由 codedelivery 全包测试守卫。
4. **真实部分成功闭环的 blocked-env 边界**：真实平台无法确定性注入「PR 创建失败」，故 Task 7 覆盖真实环境可确定性的收敛面（delivered 后重复派发被拒），`pushed` 分支的最高稳定 Interface 证据由 Task 2 的 wire 契约 stub e2e 承载（范式与 #52 计划的 blocked-env 声明一致）。移动端真实部署恢复证据（Task 8）依赖真实部署上恰有 pushed/unknown 交付，opt-in 且如实记录 `not-needed`，不伪造。
5. **`issueTicket` 在 api-client 根导出**：`packages/api-client/src/index.ts` 导出 `createSandboxTerminalApi`（门票签发），移动集成冒烟链 import 根入口（`delivery-integration-smoke.ts:1`）。门票本身不是输入通道（输入需要 WS 终端传输，移动端结构性不存在）；AC2 的钉法是 Task 4 的 API 面断言 + Task 6 的源级扫描，而非移除根导出（Web 端合法使用该面）。

## 自我审查记录

1. **Spec 覆盖**：AC1 → Task 2（wire e2e：恰一次恢复/零重推/重复派发 409）+ Task 5（状态路由含 delivered 幂等）+ Task 6（移动操作入口）+ Task 8（部署证据）；AC2 → Task 3（exec env 白名单）+ Task 4（terminal API 面）+ Task 6（源级扫描）+ 既有 `MATERIAL_TERMINAL_READ_ONLY`（引用）；AC3 → Task 2（真实迁移 DB+真实 handler+真实 A03，唯一 double 是 wire 契约 stub）+ Task 7（真实 Provider，blocked-env 如实 skip）+ Task 8（opt-in 真实部署）。What to build 三句逐一有任务承接。无缺口。
2. **步骤扫描**：每个步骤一个可检查动作；Task 2/7 的测试代码是完整可运行文件（算法非签名可定者才给全文——e2e 夹具属此类）；实现步骤只给签名+语义顺序+文件位置。无 TBD/「处理边界情况」类空步骤。
3. **类型一致性**：`DeliveryRecoveryRemote`（Task 5 定义）与 `MobileCodeDeliveryRemote`（Task 4 扩展）三方法同名同参同返回（`DeliveryRemoteRecord` vs `CodeDeliveryRecord` 结构逐字一致——`delivery-view.ts:11-31` 注释已声明该同构由 apps/mobile typecheck 证明，Task 6 装配处同一 remote 实例喂两端口）；`DELIVERY_RECOVERY_COPY` 在 Task 6 定义、Task 6 Step 2 测试与 `detail.tsx` 消费同名；`activeDeliveryRecovery` 在 Task 6 定义、Task 8 与 terminal-purity 断言消费一致；错误码 `DELIVERY_*` 四值在 Task 5 定义、Task 8 消费一致。
4. **Review Focus**：五项均有对应任务测试（见各条标注），无空节。
5. **比例**：计划正文以任务/接口/断言为主，代码块集中在 e2e 测试文件（writing-plans 允许「测试步骤给断言代码」；两处 Go e2e 属算法不可由签名推出的夹具全文）。

### 独立计划审查修复轮（5 项全部修复）

1. 【阻断·验证命令不可运行】Task 5 Step 3/Step 4 与计划级验证命令原用 `pnpm --filter @weknora/mobile-core exec tsc --noEmit`——审查实跑 exit 1（`packages/mobile-core` 无 tsconfig.json，tsc 无输入只打印帮助）。计划作者本轮复现属实（`ls packages/mobile-core/` 仅 node_modules/package.json/src）。三处已替换为 `pnpm --filter @weknora/mobile typecheck`（实跑 exit 0；mobile-core 以 `"./src/index.ts"` TS 源直出，类型经 apps/mobile 的 tsc 程序传递覆盖），并在 Task 5 Step 3 注明勿用原因。
2. 【低·期望计数】Task 4 Step 4 Expected 由「既有 8 项 + 新 4 项」更正为「`code-delivery.test.ts` 既有 4 项 + 新 3 项，加 terminal-surface 1 项，合计 8 项」（审查实跑既有 4 pass；计划作者复核 `grep -c "^test("` = 4 属实）。
3. 【低·裸文件名歧义】Task 3 背景两处引用补全路径：`internal/handler/session/sandbox_terminal_ws.go:209-213`（并注明该文件在 handler/session 包）、`internal/modules/appconnector/service/appconnector/credentials.go:17-23`。
4. 【低·行号修正】五处：Task 2 Interfaces `workbench_delivery.go:17-23` 接口（构造器 `:34`）；Task 1 import `:31`、InstallationRow `:214`；Task 2 fence 查询 `:328`；Task 7 `github_real_test.go:22`；Task 8 `scopeLease` 同文件 `:413`（`authorizedRequest` 仍在 `:383-388`）——全部经本轮 grep/sed 复核实测。
5. 【低·预先规避 typecheck 红】Task 8 Step 1 测试改为收窄形态（`off.enabled === true && off.recover === false`）直写主断言，删除「失败后改写」注记。
