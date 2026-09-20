# SP1 · Onyx 对齐清债接线 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 craft 受控模型网关、验证式 Stop、confluence/dingtalk 前端入口、幽灵连接器清理四件事全部接入生产装配。

**Architecture:** 全部沿用仓库两段式装配模式（container 构造 → handler 包级 `Register*` 注入 → router 非 nil 才挂，fail-closed）。模型网关走两种认证面（凭据签发挂 Auth 后、Forward/ListModels 挂 Auth 前靠 cmg1 HMAC 自认证）；Stop 通过 `SetExecutor` 后置注入破 provider 环；前端按现有 form/connectorDefs 声明式 schema 补条目。

**Tech Stack:** Go 1.26（gin + dig + GORM）、SQL 迁移（PG versioned + sqlite 双轨）、React 19（apps/web，node --test）、Vue 3（frontend/，vue-tsc）、pnpm workspace 共享包（@weknora/api-client、views、i18n）。

**Spec:** `docs/superpowers/specs/2026-09-19-sp1-onyx-parity-wiring-design.md`（执行者需同时读 spec 与本计划）

## Global Constraints

- fail-closed 纪律：secret 缺失/过短不静默降级；网关/Stop 未装配时路由不存在（404），禁止 503 shim；Upstream resolver 禁止环境变量回退（`internal/handler/craft_model_gateway.go:37-40` 红线注释）
- gin 通配符纪律：GET 树通配符名必须是 `:id`、POST 树是 `:session_id`（`internal/handler/session/craft.go:122-124` 注释；handler 用 `craftScope` 的双参数名回退）
- 迁移双轨：PG `migrations/versioned/`、SQLite `migrations/sqlite/`，各带 up/down
- `release.go` 的 Billing 轴缺口**不得翻转**（G4 其余 4 张 AutoMigrate-only 表不在本计划）
- 测试命令：Go `go build ./...` + `go test <目标包> -count=1`；共享包 `pnpm test:craft:shared`（Task 7 另跑 `pnpm test:web`）；Vue `cd frontend && npm run type-check && npm run check-i18n`
- 提交纪律：每个 Task 结束提交一次；提交前 `git status --porcelain` 核对暂存区只含本 Task 文件（仓库有并行 lane，勿吞他人改动）

---

### Task 1: G4 owner 列迁移（PG 000158 + SQLite 000079）

**Files:**
- Create: `migrations/versioned/000158_commercial_reservations_owner.up.sql`
- Create: `migrations/versioned/000158_commercial_reservations_owner.down.sql`
- Create: `migrations/sqlite/000079_commercial_reservations_owner.up.sql`
- Create: `migrations/sqlite/000079_commercial_reservations_owner.down.sql`
- Test: `internal/application/repository/commercial/budget_migration_test.go`

**Interfaces:**
- Consumes: `ReservationRow.Owner`（`internal/application/repository/commercial/budget.go:89`，`gorm:"column:owner;not null;default:''"`）；`BudgetStore.Reserve`（`budget_reservation.go:22`）
- Produces: 迁移后的 `commercial_reservations` 表含 `owner` 列；后续 Task 3 的网关预算路径依赖它

**背景**：建表迁移 `000116`（PG）/`000036`（SQLite）没建 `owner` 列，但 GORM `ReservationRow` 有该字段——生产 PG 上 `Reserve` 的 INSERT 报"column does not exist"（错误形态的失败，而非干净的预算拒绝）。

- [ ] **Step 1: 写失败测试**

```go
// internal/application/repository/commercial/budget_migration_test.go
package commercial

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	domain "github.com/Tencent/WeKnora/internal/commercial"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestReserveWorksOnMigratedSchema proves the sqlite migration track builds a
// commercial_reservations table the GORM rows can actually write to: run the
// 000036 budget DDL followed by the 000079 owner-column ALTER, then take one
// reservation through Reserve on that schema (no AutoMigrate anywhere).
func TestReserveWorksOnMigratedSchema(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	exec := func(db *gorm.DB, rel string) {
		raw, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		for _, stmt := range strings.Split(string(raw), ";") {
			if trimmed := strings.TrimSpace(stmt); trimmed != "" {
				if err := db.Exec(trimmed).Error; err != nil {
					t.Fatalf("exec %s: %v", rel, err)
				}
			}
		}
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "budget.db")),
		&gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	exec(db, "migrations"+string(filepath.Separator)+"sqlite"+string(filepath.Separator)+"000036_commercial_budgets.up.sql")
	exec(db, "migrations"+string(filepath.Separator)+"sqlite"+string(filepath.Separator)+"000079_commercial_reservations_owner.up.sql")

	store := NewBudgetStore(db)
	// Seed one account so Reserve passes the account guard.
	db.Exec(`INSERT INTO commercial_budget_accounts
		(tenant_id, verified_micro, unreflected_micro, held_micro, refund_locked_micro, verified_until, version)
		VALUES (42, 1000000000, 0, 0, 0, ?, 0)`, time.Now().Add(time.Hour).UTC())
	if _, err := store.Reserve(context.Background(), domain.BudgetRequest{
		TenantID: 42, RunID: "run_migration", Key: "key_migration", Upper: 1000,
		Deadline: time.Now().Add(time.Hour).UTC(),
	}); err != nil {
		t.Fatalf("reserve on migrated schema failed (owner column missing?): %v", err)
	}
}
```

注意：`commercial_budget_accounts` 的列集以 `000036_commercial_budgets.up.sql` 实际 DDL 为准——打开该文件核对 INSERT 列名后再定稿测试（若列名不同，按 DDL 调整 INSERT；这是核对，不是设计自由度）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/repository/commercial/ -run TestReserveWorksOnMigratedSchema -count=1`
Expected: FAIL —— 000079 文件不存在（read 报错）。

- [ ] **Step 3: 写迁移文件**

`migrations/versioned/000158_commercial_reservations_owner.up.sql`：
```sql
-- SP1/G4: ReservationRow carries an owner column (budget.go) used by lease
-- takeover (budget_lease.go SET owner = ?), but the 000116 DDL never created
-- it — Reserve INSERTs failed on production Postgres. Additive, backfilled ''.
ALTER TABLE commercial_reservations ADD COLUMN IF NOT EXISTS owner VARCHAR(255) NOT NULL DEFAULT '';
```
`migrations/versioned/000158_commercial_reservations_owner.down.sql`：
```sql
ALTER TABLE commercial_reservations DROP COLUMN IF EXISTS owner;
```
`migrations/sqlite/000079_commercial_reservations_owner.up.sql`：
```sql
ALTER TABLE commercial_reservations ADD COLUMN owner TEXT NOT NULL DEFAULT '';
```
`migrations/sqlite/000079_commercial_reservations_owner.down.sql`（SQLite 老版本不支持 DROP COLUMN 的话按仓库既有 down 惯例；若既有 down 均为空操作注释，照做并注明）：
```sql
-- SQLite cannot drop columns portably; the down is a no-op by convention.
```
（定稿前先看 `migrations/sqlite/000076_browser_authorization.down.sql` 的既有惯例，保持一致。）

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/application/repository/commercial/ -count=1`
Expected: PASS（含既有全部测试）。

- [ ] **Step 5: 提交**

```bash
git status --porcelain   # 核对只有本 Task 文件
git add migrations/versioned/000158_commercial_reservations_owner.up.sql migrations/versioned/000158_commercial_reservations_owner.down.sql migrations/sqlite/000079_commercial_reservations_owner.up.sql migrations/sqlite/000079_commercial_reservations_owner.down.sql internal/application/repository/commercial/budget_migration_test.go
git commit -m "fix(commercial): add the missing commercial_reservations owner column (G4, PG 000158 + sqlite 000079)"
```

---

### Task 2: Upstream resolver 生产实现

**Files:**
- Create: `internal/container/craft_model_gateway.go`
- Test: `internal/container/craft_model_gateway_test.go`

**Interfaces:**
- Consumes: `handler.CraftUpstreamResolver = func(ctx context.Context, model string) (handler.CraftUpstreamTarget, error)`；`CraftUpstreamTarget{BaseURL, Path, APIKey string}`（`internal/handler/craft_model_gateway.go:66-78`）；`types.Model`（`GetModelByID` 返回，`Parameters.ModelParameters` 的 `Scan` 已自动 AES-GCM 解密 `APIKey`，`internal/types/model.go:240-270`）
- Produces: `craftUpstreamResolver(models craftModelSource) handler.CraftUpstreamResolver`（Task 3 使用）

- [ ] **Step 1: 写失败测试**

```go
// internal/container/craft_model_gateway_test.go
package container

import (
	"context"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

type fakeModelSource struct {
	model *types.Model
	err   error
}

func (f fakeModelSource) GetModelByID(ctx context.Context, id string) (*types.Model, error) {
	return f.model, f.err
}

func withChatModel(id, baseURL, apiKey string) *types.Model {
	m := &types.Model{ID: id}
	m.Parameters.BaseURL = baseURL
	m.Parameters.APIKey = apiKey
	return m
}

func TestCraftUpstreamResolverResolvesManagedUpstream(t *testing.T) {
	resolver := craftUpstreamResolver(fakeModelSource{model: withChatModel("m1", "https://api.example.com/", "sk-managed")})
	target, err := resolver(context.Background(), "m1")
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if target.BaseURL != "https://api.example.com" || target.Path != "/v1/chat/completions" || target.APIKey != "sk-managed" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestCraftUpstreamResolverFailsClosedWithoutKey(t *testing.T) {
	resolver := craftUpstreamResolver(fakeModelSource{model: withChatModel("m1", "https://api.example.com", "")})
	if _, err := resolver(context.Background(), "m1"); err == nil || !strings.Contains(err.Error(), "no configured api key") {
		t.Fatalf("expected fail-closed no-key error, got %v", err)
	}
}

func TestCraftUpstreamResolverFailsClosedWithoutBaseURL(t *testing.T) {
	resolver := craftUpstreamResolver(fakeModelSource{model: withChatModel("m1", "", "sk-managed")})
	if _, err := resolver(context.Background(), "m1"); err == nil || !strings.Contains(err.Error(), "no base url") {
		t.Fatalf("expected fail-closed no-base-url error, got %v", err)
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/container/ -run TestCraftUpstreamResolver -count=1`
Expected: FAIL —— `craftUpstreamResolver` 未定义。

- [ ] **Step 3: 最小实现**

```go
// internal/container/craft_model_gateway.go
package container

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/handler"
	"github.com/Tencent/WeKnora/internal/types"
)

// craftGatewaySecretEnv enables the craft controlled model gateway (O02).
// Setting a secret is the enablement intent: a non-empty secret shorter than
// the handler's 16-byte minimum fails the boot (fail-closed, never a silent
// downgrade). Empty keeps the gateway unassembled.
const craftGatewaySecretEnv = "WEKNORA_CRAFT_GATEWAY_SECRET"

// craftGatewayBaseURLEnv optionally names the externally reachable gateway
// base URL stamped into issued credentials (default: relative path).
const craftGatewayBaseURLEnv = "WEKNORA_CRAFT_GATEWAY_BASE_URL"

// craftModelSource is the narrow consumer interface of the upstream
// resolver (consumer-defined interface, Go idiom): interfaces.ModelService
// satisfies it without taking the whole model surface here.
type craftModelSource interface {
	GetModelByID(ctx context.Context, id string) (*types.Model, error)
}

// craftUpstreamChatPath is the OpenAI-compatible path the gateway's Forward
// joins onto the resolved base URL (Forward does TrimSuffix(baseURL,"/")+Path).
const craftUpstreamChatPath = "/v1/chat/completions"

// craftUpstreamResolver resolves one model's managed upstream from the
// server's credential store ONLY (the model row's AES-GCM-encrypted api_key
// is decrypted transparently by ModelParameters.Scan). It fails closed on a
// missing model, a missing key or a missing base URL — the gateway header
// forbids any environment fallback.
func craftUpstreamResolver(models craftModelSource) handler.CraftUpstreamResolver {
	return func(ctx context.Context, model string) (handler.CraftUpstreamTarget, error) {
		m, err := models.GetModelByID(ctx, model)
		if err != nil {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s: %w", model, err)
		}
		if m == nil {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s not found", model)
		}
		if m.Parameters.APIKey == "" {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s has no configured api key", model)
		}
		if m.Parameters.BaseURL == "" {
			return handler.CraftUpstreamTarget{}, fmt.Errorf("craft gateway upstream: model %s has no base url", model)
		}
		return handler.CraftUpstreamTarget{
			BaseURL: m.Parameters.BaseURL,
			Path:    craftUpstreamChatPath,
			APIKey:  m.Parameters.APIKey,
		}, nil
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `go test ./internal/container/ -run TestCraftUpstreamResolver -count=1`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git status --porcelain
git add internal/container/craft_model_gateway.go internal/container/craft_model_gateway_test.go
git commit -m "feat(craft): production upstream resolver for the model gateway (server credential store only, fail-closed)"
```

---

### Task 3: 模型网关容器装配 + 双认证面路由

**Files:**
- Modify: `internal/container/craft_model_gateway.go`（追加装配函数）
- Modify: `internal/container/container.go`（Provide + Invoke 注册，参照 `container.go:542-563` craft 区）
- Modify: `internal/handler/craft_model_gateway.go`（追加包级注册变量与函数）
- Modify: `internal/router/router.go`（pre-auth 区挂 Forward/ListModels，插在 `router.go:216-236` craft preview 块之后、`r.Use(middleware.Auth...)` 之前）
- Modify: `internal/router/routes_chat.go`（craft 区后挂 IssueCredential/RevokeCredential）
- Test: `internal/container/craft_model_gateway_test.go`（追加装配测试）

**Interfaces:**
- Consumes: `handler.NewCraftModelGateway(cfg handler.CraftModelGatewayConfig) (*handler.CraftModelGateway, error)`（`craft_model_gateway.go:138-186`，校验 Secret≥16 / Budget 非 nil 且实现 `CraftCallAuthorizer` / Recorder、Upstream 非 nil）；`service.NewCraftBudgetService(db, store, policy)`（`craft_budget.go:164`，store 传 nil 自建）；`service.CraftBudgetPolicy{GrantWindow time.Duration; MaxCalls int; CallUpper, TaskLimit commercial.Credits}`（全正校验）；`*service.CraftUsageService.RecordPhysicalCall` 满足 `handler.CraftPhysicalCallRecorder`；Task 2 的 `craftUpstreamResolver`
- Produces: `handler.RegisteredCraftModelGateway() *handler.CraftModelGateway`（router 两处消费）；`newCraftModelGatewayHandler(db *gorm.DB, models interfaces.ModelService, usage *service.CraftUsageService) (*handler.CraftModelGateway, error)`（dig provider；禁用时返回 `(nil, nil)`）；`registerCraftModelGatewayHTTPHandlers(gw *handler.CraftModelGateway)`（Invoke 注册）

- [ ] **Step 1: 写失败测试（追加到 craft_model_gateway_test.go）**

```go
func TestCraftModelGatewayAssemblyFailsClosedOnShortSecret(t *testing.T) {
	t.Setenv(craftGatewaySecretEnv, "short")
	t.Setenv(craftOpenCodeBaseURLEnv, "http://127.0.0.1:9090")
	if _, err := newCraftModelGatewayHandler(
		nil, fakeModelSource{}, nil,
	); err == nil {
		t.Fatal("expected fail-closed error for a secret shorter than 16 bytes")
	}
}

func TestCraftModelGatewayAssemblyDisabledByDefault(t *testing.T) {
	t.Setenv(craftGatewaySecretEnv, "")
	if gw, err := newCraftModelGatewayHandler(nil, fakeModelSource{}, nil); err != nil || gw != nil {
		t.Fatalf("expected (nil, nil) when no secret is set, got (%v, %v)", gw, err)
	}
}
```

注意：这两个测试走的是"构造函数内部的 env 判定"，不触库——实现里把 secret/OC 判定放在**构造 gateway 之前**，budget service 仅在两者都启用后才构造，这样禁用路径不会碰 DB（也解释了为什么测试能传 nil db）。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/container/ -run TestCraftModelGatewayAssembly -count=1`
Expected: FAIL —— `newCraftModelGatewayHandler` 未定义。

- [ ] **Step 3: 实现装配函数（追加到 internal/container/craft_model_gateway.go）**

```go
// defaultCraftBudgetPolicy is the non-configurable default admission policy
// of the controlled gateway. Credit values are micro-credits (the same unit
// as ReservationRow.UpperMicro): CallUpper 1 credit per forwarded call,
// TaskLimit 10 credits per admitted run, GrantWindow 24h, MaxCalls 1000.
func defaultCraftBudgetPolicy() service.CraftBudgetPolicy {
	return service.CraftBudgetPolicy{
		GrantWindow: 24 * time.Hour,
		MaxCalls:    1000,
		CallUpper:   commercial.Credits(1_000_000),
		TaskLimit:   commercial.Credits(10_000_000),
	}
}

// newCraftModelGatewayHandler assembles the O02 controlled model gateway.
// Enablement requires BOTH the OpenCode runtime dial (the gateway's only
// callers live inside the sandbox runtime) and a signing secret; a set-but
// short secret fails the boot (fail-closed). The budget service is built
// inline — it stays a private dependency of the gateway, not a container
// surface other consumers could accidentally depend on.
func newCraftModelGatewayHandler(
	db *gorm.DB,
	models interfaces.ModelService,
	usage *service.CraftUsageService,
) (*handler.CraftModelGateway, error) {
	secret := strings.TrimSpace(os.Getenv(craftGatewaySecretEnv))
	if secret == "" {
		return nil, nil
	}
	if strings.TrimSpace(os.Getenv(craftOpenCodeBaseURLEnv)) == "" {
		logger.Warnf(context.Background(),
			"[CraftModelGateway] %s set but %s is not: the gateway stays unassembled (no runtime caller)",
			craftGatewaySecretEnv, craftOpenCodeBaseURLEnv)
		return nil, nil
	}
	budget, err := service.NewCraftBudgetService(db, nil, defaultCraftBudgetPolicy())
	if err != nil {
		return nil, fmt.Errorf("craft model gateway budget service: %w", err)
	}
	gw, err := handler.NewCraftModelGateway(handler.CraftModelGatewayConfig{
		Secret:         []byte(secret),
		Budget:         budget,
		Recorder:       usage,
		Upstream:       craftUpstreamResolver(models),
		GatewayBaseURL: strings.TrimSpace(os.Getenv(craftGatewayBaseURLEnv)),
	})
	if err != nil {
		return nil, fmt.Errorf("craft model gateway: %w", err)
	}
	return gw, nil
}

// registerCraftModelGatewayHTTPHandlers installs the gateway for routing.
func registerCraftModelGatewayHTTPHandlers(gw *handler.CraftModelGateway) {
	if gw == nil {
		return
	}
	handler.RegisterCraftModelGateway(gw)
}
```

同文件补 import：`os`、`strings`、`time`、`commercial "github.com/Tencent/WeKnora/internal/commercial"`、`"github.com/Tencent/WeKnora/internal/application/service"`、`"github.com/Tencent/WeKnora/internal/logger"`、`"github.com/Tencent/WeKnora/internal/types/interfaces"`、`"gorm.io/gorm"`。

handler 侧（`internal/handler/craft_model_gateway.go` 追加）：

```go
// registeredCraftModelGateway holds the production gateway for route
// mounting (two auth planes: credential issuance behind the global Auth,
// Forward/ListModels pre-auth on the cmg1 HMAC credential).
var registeredCraftModelGateway *CraftModelGateway

// RegisterCraftModelGateway installs the gateway for route mounting.
func RegisterCraftModelGateway(g *CraftModelGateway) { registeredCraftModelGateway = g }

// RegisteredCraftModelGateway returns the registered gateway (nil keeps
// every gateway route unmounted — fail-closed, no 503 shims).
func RegisteredCraftModelGateway() *CraftModelGateway { return registeredCraftModelGateway }
```

- [ ] **Step 4: 容器注册（container.go）**

在既有 craft provider 注册区（`initConnectorRegistry` / craft provider 们附近）加：

```go
must(container.Provide(newCraftModelGatewayHandler))
```

在 Invoke 区（`container.go:542-563` craft 块，`registerCraftUsageHTTPHandlers` 之后）加：

```go
// O02: the controlled model gateway (credential issuance + HMAC forward),
// assembled only when the runtime dial and signing secret are both set.
must(container.Invoke(registerCraftModelGatewayHTTPHandlers))
```

- [ ] **Step 5: 路由挂载（两个认证面）**

`internal/router/router.go` —— 在 `handler.RegisterArtifactPreviewRoutes(...)` 之后、`r.Use(middleware.Auth(...))` 之前（先例注释同 craft preview/sandbox terminal）：

```go
// Craft controlled model gateway forward plane (O02): the sandbox runtime
// authenticates with its short-lived cmg1 HMAC credential (no user JWT),
// so this must precede the global Auth middleware. A nil gateway (craft
// runtime or signing secret not configured) mounts nothing.
if gw := handler.RegisteredCraftModelGateway(); gw != nil {
	r.POST("/api/v1/craft/model-gateway/v1/chat/completions", gw.Forward)
	r.GET("/api/v1/craft/model-gateway/v1/models", gw.ListModels)
}
```

`internal/router/routes_chat.go` —— 在 `RegisterCraftInteractionRoutes` 调用之后追加：

```go
// O02 credential issuance plane: behind the sessions group's auth chain
// (needs the tenant context); mounted only when the gateway is assembled.
if gw := handlerapi.RegisteredCraftModelGateway(); gw != nil {
	v1.POST("/craft/model-gateway/credentials", gw.IssueCredential)
	v1.POST("/craft/model-gateway/credentials/revoke", gw.RevokeCredential)
}
```

（核对 `routes_chat.go` 里 v1 组与 handler 导入别名的实际变量名——该文件的 gin group 变量与 import 别名以现有代码为准，通常 `v1` 与 `handlerapi`；若名字不同照改。）

- [ ] **Step 6: 运行测试与构建**

Run: `go test ./internal/container/ ./internal/handler/... -count=1 && go build ./...`
Expected: PASS / 构建成功。

- [ ] **Step 7: 提交**

```bash
git status --porcelain
git add internal/container/craft_model_gateway.go internal/container/craft_model_gateway_test.go internal/container/container.go internal/handler/craft_model_gateway.go internal/router/router.go internal/router/routes_chat.go
git commit -m "feat(craft): assemble the O02 model gateway in production (dual auth plane, fail-closed secret)"
```

---

### Task 4: craft 验证式 Stop 后端（SetExecutor + 路由）

**Files:**
- Modify: `internal/application/service/craft_control.go`（executor 字段 → `atomic.Pointer`；加 `SetExecutor`；改全部 `s.executor` 读点）
- Modify: `internal/container/container.go`（`wireCraftInteractionRegistrar` :2349-2360 扩展 SetExecutor）
- Modify: `internal/handler/session/craft_interaction.go`（接口扩展 + 2 个 handler 方法 + 路由）
- Test: `internal/application/service/craft_control_executor_test.go`（新建）
- Test: `internal/handler/session/craft_interaction_stop_test.go`（新建）

**Interfaces:**
- Consumes: `CraftControlService.Stop(ctx, CraftStopRequest{Scope craft.Scope; RunKey agentruntime.RunKey; TaskID string}) (CraftStopStatus{Phase string; Result *craft.Result; Note string}, error)`（`craft_control.go:401`）；`DelegationStatus(ctx, scope, RunKey, taskID)`（`:510`）；`agentruntime.RunKey{TenantID uint64; RunID string}`（`internal/agent/runtime/contracts.go:27`）
- Produces: `(*CraftControlService).SetExecutor(executor craft.Executor)`（container 后置注入）；HTTP `POST /api/v1/sessions/:session_id/craft/runs/:run_id/stop`（body `{"task_id"}`，响应 `{phase, note}`）与 `GET /api/v1/sessions/:id/craft/runs/:run_id/delegations/:task_id/status`（GET 树用 `:id`）

- [ ] **Step 1: 写失败测试（service 层）**

```go
// internal/application/service/craft_control_executor_test.go
package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/craft"
)

// TestSetExecutorIsVisibleToConcurrentStop proves the post-construction
// injection (which breaks the provider cycle) is safe to read from a stop
// that runs after the setter: the R06 semantic — nil executor keeps the
// recorded-intent degrade, an injected executor is used for the abort.
func TestSetExecutorIsVisibleToConcurrentStop(t *testing.T) {
	// Build the same fixture shape the existing craft_control_test.go uses
	// for its stop tests (see TestControlStopStaysStoppingWhenAbortAccepted-
	// ButStillRunning, craft_control_test.go:508-528): runs controller, store,
	// nil executor at construction, interactions store.
	fixture := newControlStopFixture(t) // helper defined below from the existing test fixtures
	svc := NewCraftControlService(fixture.runs, fixture.store, nil, fixture.interactions, nil)

	if got := svc.currentExecutor(); got != nil {
		t.Fatalf("expected nil executor before injection, got %T", got)
	}
	var injected craft.Executor = fixture.abortExecutor // an executor fake whose Abort records the call
	svc.SetExecutor(injected)
	if got := svc.currentExecutor(); got != injected {
		t.Fatalf("expected injected executor after SetExecutor, got %T", got)
	}
}
```

实施提示（不是占位）：打开 `craft_control_test.go` 找 `TestControlStopStaysStoppingWhenAbortAcceptedButStillRunning`（:508 起）复用它的 fixture 构造方式（runs/store/interactions/abort-executor fake 都是现成的），把上面 `newControlStopFixture` 展开成同样的构造代码——测试要能独立编译运行。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/application/service/ -run TestSetExecutorIsVisibleToConcurrentStop -count=1`
Expected: FAIL —— `SetExecutor`/`currentExecutor` 未定义。

- [ ] **Step 3: 实现（craft_control.go）**

结构体字段改造（`:132-138`）：

```go
type CraftControlService struct {
	runs         CraftRunController
	store        craft.Store
	executor     atomic.Pointer[craft.Executor] // post-construction injection (wireCraftInteractionRegistrar)
	interactions CraftInteractionStore
	reply        CraftOpenCodeReplier
}
```

构造函数（`:143-154`）改为：

```go
func NewCraftControlService(
	runs CraftRunController,
	store craft.Store,
	executor craft.Executor,
	interactions CraftInteractionStore,
	reply CraftOpenCodeReplier,
) *CraftControlService {
	svc := &CraftControlService{
		runs: runs, store: store,
		interactions: interactions, reply: reply,
	}
	svc.SetExecutor(executor)
	return svc
}

// SetExecutor installs the craft executor post-construction (the provider
// cycle is broken exactly like the interaction emitter: the runtime is
// assembled after this service). Nil keeps the recorded-intent degrade.
func (s *CraftControlService) SetExecutor(executor craft.Executor) {
	if executor == nil {
		return
	}
	s.executor.Store(&executor)
}

// currentExecutor returns the live executor or nil (recorded-intent stop).
func (s *CraftControlService) currentExecutor() craft.Executor {
	if held := s.executor.Load(); held != nil {
		return *held
	}
	return nil
}
```

然后 `grep -n "s.executor" internal/application/service/craft_control.go`，把每一处 `s.executor` 的**读**替换为 `s.currentExecutor()`（nil 判断语义不变——原代码本来就处理 nil executor 的降级分支 `craft_control.go:476-479`）。补 import `sync/atomic`。

- [ ] **Step 4: 破环注入（container.go wireCraftInteractionRegistrar :2349-2360）**

```go
func wireCraftInteractionRegistrar(executor craft.Executor, assembly *CraftInteractionAssembly) {
	if assembly == nil {
		return
	}
	if runtime, ok := executor.(*localCraftRuntime); ok {
		// BASE wrapped the executor's emission path with the registrar at
		// construction using the executor's OWN opencode client; the
		// post-construction install reuses that same client (assembly.Client
		// may legitimately be nil when its own dial failed).
		runtime.setInteractionEmitter(craftInteractionRegistrar(
			runtime.client, runtime.store, assembly.Store, assembly.Runs, runtime.emit))
		// R06: the same post-construction seam now carries the verifiable
		// stop surface — the control service can abort the real runtime.
		// The fail-closed executor (no runtime dial) is NOT injected: stop
		// keeps its honest "no executor available" degrade there.
		assembly.Control.SetExecutor(executor)
	}
}
```

- [ ] **Step 5: handler + 路由（craft_interaction.go）**

接口扩展（`:35-38`）：

```go
type CraftInteractionAPI interface {
	ListInteractions(context.Context, craft.Scope, string) ([]service.CraftInteractionRecord, error)
	Decide(context.Context, service.CraftDecisionRequest) (service.CraftDecisionOutcome, error)
	Stop(context.Context, service.CraftStopRequest) (service.CraftStopStatus, error)
	DelegationStatus(context.Context, craft.Scope, agentruntime.RunKey, string) (service.CraftStopStatus, error)
}
```

（补 import `agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"`；`var _ CraftInteractionAPI = (*service.CraftControlService)(nil)` 会自动校验签名正确。）

路由（`RegisterCraftInteractionRoutes` :59-64 内追加；遵守 GET `:id` / POST `:session_id` 通配符约定）：

```go
	sessions.POST("/:session_id/craft/runs/:run_id/stop", h.StopCraftRun)
	sessions.GET("/:id/craft/runs/:run_id/delegations/:task_id/status", h.GetCraftDelegationStatus)
```

handler 方法（追加到文件尾部、`craftOperator` 之前）：

```go
type stopRequestBody struct {
	TaskID string `json:"task_id"`
}

// StopCraftRun serves POST /api/v1/sessions/:session_id/craft/runs/:run_id/
// stop: the R06 verifiable stop. The response keeps the honest phase —
// "stopping" is a real answer (abort not yet confirmed), never folded into
// a boolean.
func (h *CraftInteractionHandler) StopCraftRun(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft control is unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	var body stopRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	runID := strings.TrimSpace(c.Param("run_id"))
	if runID == "" || strings.TrimSpace(body.TaskID) == "" {
		c.Error(apperrors.NewBadRequestError("run_id and task_id are required"))
		return
	}
	status, err := h.svc.Stop(c.Request.Context(), service.CraftStopRequest{
		Scope:  scope,
		RunKey: agentruntime.RunKey{TenantID: scope.TenantID, RunID: runID},
		TaskID: strings.TrimSpace(body.TaskID),
	})
	if err != nil {
		craftInteractionHTTPError(c, err)
		return
	}
	data := gin.H{"phase": status.Phase, "note": status.Note}
	if status.Result != nil {
		data["result_status"] = status.Result.Status
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

// GetCraftDelegationStatus serves GET /api/v1/sessions/:id/craft/runs/
// :run_id/delegations/:task_id/status: the read-only poll the client runs
// after a stop answered "stopping". It writes nothing.
func (h *CraftInteractionHandler) GetCraftDelegationStatus(c *gin.Context) {
	if h == nil || h.svc == nil {
		c.Error(apperrors.NewServiceUnavailableError("craft control is unavailable"))
		return
	}
	scope, ok := craftScope(c)
	if !ok || scope.SessionID == "" {
		craftUnauthorized(c)
		return
	}
	runID := strings.TrimSpace(c.Param("run_id"))
	taskID := strings.TrimSpace(c.Param("task_id"))
	if runID == "" || taskID == "" {
		c.Error(apperrors.NewBadRequestError("run_id and task_id are required"))
		return
	}
	status, err := h.svc.DelegationStatus(c.Request.Context(), scope,
		agentruntime.RunKey{TenantID: scope.TenantID, RunID: runID}, taskID)
	if err != nil {
		craftInteractionHTTPError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"phase": status.Phase, "note": status.Note}})
}
```

（补 import `strings`；`craftUnauthorized`/`craftScope`/`craftInteractionHTTPError` 已在本包。）

- [ ] **Step 6: handler 测试（新建 craft_interaction_stop_test.go）**

仿照本包既有 handler 测试的 fake API 模式（若包内无现成 fake，用最小结构实现四个接口方法）：

```go
package session

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/craft"
	agentruntime "github.com/Tencent/WeKnora/internal/agent/runtime"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
)

type stopFakeAPI struct {
	stopErr   error
	stopGot   service.CraftStopRequest
	stopReply service.CraftStopStatus
}

func (f *stopFakeAPI) ListInteractions(context.Context, craft.Scope, string) ([]service.CraftInteractionRecord, error) { return nil, nil }
func (f *stopFakeAPI) Decide(context.Context, service.CraftDecisionRequest) (service.CraftDecisionOutcome, error) {
	return service.CraftDecisionOutcome{}, nil
}
func (f *stopFakeAPI) Stop(_ context.Context, req service.CraftStopRequest) (service.CraftStopStatus, error) {
	f.stopGot = req
	return f.stopReply, f.stopErr
}
func (f *stopFakeAPI) DelegationStatus(context.Context, craft.Scope, agentruntime.RunKey, string) (service.CraftStopStatus, error) {
	return service.CraftStopStatus{Phase: "stopping"}, nil
}

func stopTestRouter(api CraftInteractionAPI) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/sessions/:session_id/craft/runs/:run_id/stop", NewCraftInteractionHandler(api).StopCraftRun)
	r.GET("/api/v1/sessions/:id/craft/runs/:run_id/delegations/:task_id/status", NewCraftInteractionHandler(api).GetCraftDelegationStatus)
	return r
}

// craftScope needs tenant+user in the context (mirrors the authenticated chain).
func withScopeContext(req *http.Request, tenant uint64, user, session string) *http.Request {
	ctx := context.WithValue(req.Context(), types.TenantIDContextKey, tenant)
	// SessionOwnerIDFromContext reads the owner key; use the same one the
	// production middleware sets (see craftScope and types context keys).
	ctx = context.WithValue(ctx, types.SessionOwnerContextKey, user)
	return req.WithContext(ctx)
}

func TestStopCraftRunKeepsHonestPhase(t *testing.T) {
	api := &stopFakeAPI{stopReply: service.CraftStopStatus{Phase: "stopping", Note: "abort accepted, still running"}}
	r := stopTestRouter(api)
	body := `{"task_id":"dlg_1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s1/craft/runs/run_1/stop", strings.NewReader(body))
	req = withScopeContext(req, 42, "u1", "s1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"phase":"stopping"`) {
		t.Fatalf("expected 200 with honest stopping phase, got %d %s", w.Code, w.Body.String())
	}
	if api.stopGot.RunKey.RunID != "run_1" || api.stopGot.TaskID != "dlg_1" || api.stopGot.Scope.TenantID != 42 {
		t.Fatalf("unexpected stop request: %+v", api.stopGot)
	}
}

func TestStopCraftRunRejectsMissingTaskID(t *testing.T) {
	r := stopTestRouter(&stopFakeAPI{})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/s1/craft/runs/run_1/stop", strings.NewReader(`{}`))
	req = withScopeContext(req, 42, "u1", "s1")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing task_id, got %d", w.Code)
	}
}
```

实施提示：`withScopeContext` 里的 context key 名以 `types` 包实际导出为准（`types.TenantIDContextKey` 是自定义 string 类型，context key 用法照 `internal/handler/session/craft.go` 的 `craftScope`（:176-196）——它会 `c.Get(types.TenantIDContextKey.String())`，即 gin 的 `c.Set` 注入而非 request context；那么测试里应该用 `c.Set`，改用中间件形式：包一层 `func(ctx *gin.Context) { ctx.Set(types.TenantIDContextKey.String(), uint64(42)); ctx.Next() }` 挂 `r.Use(...)`，同时 owner 键照 `types.SessionOwnerIDFromContext` 的读取源设置。以 `craftScope` 实际代码为准修正测试注入方式。

- [ ] **Step 7: 运行全部相关测试**

Run: `go test ./internal/application/service/ ./internal/handler/... ./internal/container/ -count=1 && go build ./...`
Expected: PASS（含既有 craft_control 保序/竞态测试不回归）。

- [ ] **Step 8: 提交**

```bash
git status --porcelain
git add internal/application/service/craft_control.go internal/application/service/craft_control_executor_test.go internal/container/container.go internal/handler/session/craft_interaction.go internal/handler/session/craft_interaction_stop_test.go
git commit -m "feat(craft): R06 verifiable stop HTTP surface + post-construction executor injection"
```

---

### Task 5: api-client stop/status 方法

**Files:**
- Modify: `packages/api-client/src/craft/index.ts`
- Test: `packages/api-client/src/craft/index.test.ts`（已有文件，追加）

**Interfaces:**
- Consumes: `createCraftApi(request)` 的返回对象与 `unwrap`（同文件）；Task 4 的两个 HTTP 端点
- Produces: `CraftStopStatusView{phase: string; note: string}`；`api.stop(sessionId, runId, taskId, signal?)`；`api.delegationStatus(sessionId, runId, taskId, signal?)`（Task 6 消费）

- [ ] **Step 1: 写失败测试（追加到 index.test.ts，沿用该文件既有的 request fake 模式）**

```ts
test('stop posts task_id and returns the honest phase', async () => {
  const calls: Array<{ method: string; path: string; body?: unknown }> = [];
  const api = createCraftApi(async (input) => {
    calls.push({ method: input.method, path: input.path, body: input.body });
    return { success: true, data: { phase: 'stopping', note: 'abort accepted, still running' } };
  });
  const out = await api.stop('s1', 'run_1', 'dlg_1');
  if (out.phase !== 'stopping' || out.note !== 'abort accepted, still running') throw new Error('bad view: ' + JSON.stringify(out));
  if (calls[0]?.method !== 'POST' || calls[0]?.path !== '/api/v1/sessions/s1/craft/runs/run_1/stop') throw new Error('bad request');
  if (JSON.stringify(calls[0]?.body) !== JSON.stringify({ task_id: 'dlg_1' })) throw new Error('bad body');
});

test('delegationStatus polls the read-only endpoint', async () => {
  const api = createCraftApi(async (input) => {
    if (input.method !== 'GET' || input.path !== '/api/v1/sessions/s1/craft/runs/run_1/delegations/dlg_1/status') {
      throw new Error('unexpected request');
    }
    return { success: true, data: { phase: 'canceled', note: '' } };
  });
  const out = await api.delegationStatus('s1', 'run_1', 'dlg_1');
  if (out.phase !== 'canceled') throw new Error('bad phase');
});
```

（`test` 的引入方式与 fake request 的形状照该测试文件既有用例的模式对齐。）

- [ ] **Step 2: 运行确认失败**

Run: `pnpm test:craft:shared`
Expected: FAIL —— `api.stop is not a function`。

- [ ] **Step 3: 实现（index.ts，createCraftApi 返回对象内追加）**

```ts
export interface CraftStopStatusView {
  phase: string;
  note: string;
}

function parseStopStatus(value: unknown, label: string): CraftStopStatusView {
  const data = unwrap(value, label) as Record<string, unknown>;
  return {
    phase: typeof data.phase === 'string' ? data.phase : '',
    note: typeof data.note === 'string' ? data.note : '',
  };
},
```
（`parseStopStatus` 放在 `createCraftApi` 之外，与 `craftDownloadPath` 同级；以下方法加入返回对象）：
```ts
    /** POST /craft/runs/:run_id/stop — R06 verifiable stop; "stopping" is an honest phase, poll delegationStatus until terminal. */
    async stop(sessionId: string, runId: string, taskId: string, signal?: AbortSignal): Promise<CraftStopStatusView> {
      return parseStopStatus(await request({
        method: 'POST',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs/' + encodeURIComponent(runId) + '/stop',
        body: { task_id: taskId },
        signal,
      }), 'stop');
    },
    /** GET /craft/runs/:run_id/delegations/:task_id/status — read-only poll after a stop answered stopping. */
    async delegationStatus(sessionId: string, runId: string, taskId: string, signal?: AbortSignal): Promise<CraftStopStatusView> {
      return parseStopStatus(await request({
        method: 'GET',
        path: '/api/v1/sessions/' + encodeURIComponent(sessionId) + '/craft/runs/' + encodeURIComponent(runId) + '/delegations/' + encodeURIComponent(taskId) + '/status',
        signal,
      }), 'delegation status');
    },
```

- [ ] **Step 4: 运行确认通过**

Run: `pnpm test:craft:shared && pnpm typecheck:shared`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git status --porcelain
git add packages/api-client/src/craft/index.ts packages/api-client/src/craft/index.test.ts
git commit -m "feat(api-client): craft stop + delegation status methods"
```

---

### Task 6: 工作台停止按钮（workbench prop + routes 装配）

**Files:**
- Modify: `packages/views/src/craft/workbench.tsx`（props + STOP_STRINGS + 头部按钮）
- Modify: `apps/web/src/features/craft/routes.tsx`（activeDelegation 追踪 + onStopRun 装配 + stopPhase）
- Test: `packages/views/src/craft/workbench.stop.test.tsx`（新建）

**Interfaces:**
- Consumes: Task 5 的 `api.stop`/`api.delegationStatus`；`CraftWorkbenchProps`（`workbench.tsx:83-176`）；SSE craft 帧 payload `{kind, workspace_id, delegation_id, tool_call_id, data}`（`internal/container/craft_runtime.go:561-577` 的 emitter 序列化形状——`delegation_id` 就是 Stop 需要的 task_id）
- Produces: `CraftWorkbenchProps.onStopRun?(): Promise<void>`、`stopPhase?: 'idle' | 'stopping'`

- [ ] **Step 1: 写失败测试**

```tsx
// packages/views/src/craft/workbench.stop.test.tsx
// 沿用本目录既有 workbench 测试的 render 模式（从相邻 workbench *.test.tsx
// 复制其 harness：controller fake、messageLog fake、locale、最少 props）。
test('stop button appears while running and calls onStopRun', async () => {
  // harness 让 threadRunning=true（controller state 的 running 形状照相邻测试）
  const onStopRun = async () => { calls++; };
  render(<CraftWorkbench {...baseProps} onStopRun={onStopRun} />);
  const button = screen.getByRole('button', { name: '停止' }); // locale=zh
  await userEvent.click(button);
  if (calls !== 1) throw new Error('onStopRun not called once');
});

test('stop button hidden when no run is active', () => {
  render(<CraftWorkbench {...baseProps} onStopRun={async () => {}} />);
  // threadRunning=false 的 harness：查询不到停止按钮
});
```

（harness 的具体搭法从相邻 `workbench*.test.tsx` 现有 fake 复制——它们的 controller/messageLog fake 就是为此存在的；locale zh 时按钮文案为"停止"（STOP_STRINGS），这是本测试锚定的契约。）

- [ ] **Step 2: 运行确认失败**

Run: `pnpm test:craft:shared`
Expected: FAIL —— 按钮不存在。

- [ ] **Step 3: workbench 实现**

`workbench.tsx`，RESTORE_STRINGS 旁（:150-172 区域）加：

```tsx
// R06 stop wording: same feature-string pattern as RESTORE_STRINGS.
const STOP_STRINGS = {
  zh: { action: '停止', busy: '停止中…', failed: '停止失败' },
  en: { action: 'Stop', busy: 'Stopping…', failed: 'Stop failed' },
} as const;
```

props（`CraftWorkbenchProps` 内、`onRestoreVersion?` 之后加）：

```tsx
  /**
   * R06 verifiable stop entrance (optional until the assembly wires POST
   * /craft/runs/:run_id/stop): when provided and a run is active, the
   * header offers the stop. The assembly owns addressing (run id + the
   * active delegation id) and the stopping→terminal poll.
   */
  onStopRun?(): Promise<void>;
  /** Poll state of an accepted stop ('stopping' shows the busy label). */
  stopPhase?: 'idle' | 'stopping';
```

按钮渲染：定位 `isRunning={threadRunning}` 传递处（:495-510 附近，isRunning 流向 thread 组件的区域）所在的头部/工具条 JSX，在运行指示旁加：

```tsx
{props.canWrite && threadRunning && props.onStopRun && (
  <button
    type="button"
    disabled={props.stopPhase === 'stopping'}
    onClick={() => { void props.onStopRun?.().catch(() => { /* surfaced via syncError by the assembly */ }); }}
  >
    {props.stopPhase === 'stopping' ? strings.busy : strings.action}
  </button>
)}
```

（`strings = STOP_STRINGS[props.locale === 'en' ? 'en' : 'zh']`；按钮的 class/样式照同区域现有按钮。）

- [ ] **Step 4: routes.tsx 装配**

在 `apps/web/src/features/craft/routes.tsx`（641 行文件）：

1. workbench 状态区加：

```tsx
const [stopPhase, setStopPhase] = useState<'idle' | 'stopping'>('idle');
const activeDelegationRef = useRef<{ runId: string; taskId: string } | null>(null);
```

2. 定位现有 SSE 帧消费点（`createServerSentEventParser` 的 onEvent/tee 回调，routes.tsx 中订阅 `/runs/:run_id/events` 的那段），在每帧处理里追加：

```tsx
// R06: remember the active delegation so the stop button can address it.
try {
  const parsed = JSON.parse(frame.data) as { delegation_id?: string };
  const runId = /* 该订阅所属的 run id 变量（回调闭包里现成） */;
  if (typeof parsed.delegation_id === 'string' && parsed.delegation_id !== '') {
    activeDelegationRef.current = { runId, taskId: parsed.delegation_id };
  }
} catch { /* non-JSON frames pass through untouched */ }
```

（`runId` 用该 SSE 订阅闭包中既有的 run id 变量；若帧回调拿不到，则在发起订阅处把 runId 一并存进 ref。）

3. `CraftWorkbench` 装配处传 prop：

```tsx
stopPhase={stopPhase}
onStopRun={async () => {
  const active = activeDelegationRef.current;
  if (active === null) return;
  setStopPhase('stopping');
  try {
    const result = await craftApi.stop(sessionId, active.runId, active.taskId);
    // Poll the read-only status until the phase leaves "stopping".
    for (let i = 0; i < 60 && result.phase === 'stopping'; i++) {
      await new Promise((resolve) => setTimeout(resolve, 2000));
      const status = await craftApi.delegationStatus(sessionId, active.runId, active.taskId);
      if (status.phase !== 'stopping') break;
    }
  } finally {
    setStopPhase('idle');
    activeDelegationRef.current = null;
    // The controller's own snapshot reload reflects the terminal state —
    // same entry the existing error/cancel paths use (controller.reload()).
    await controller.reload();
  }
}}
```

（`controller.reload` 的准确方法名照 `@weknora/core/craft/controller` 现有 API——routes.tsx 里已有重载调用点，复用同一个。）

- [ ] **Step 5: 运行测试与类型检查**

Run: `pnpm test:craft:shared && pnpm typecheck:shared && pnpm typecheck:web`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git status --porcelain
git add packages/views/src/craft/workbench.tsx packages/views/src/craft/workbench.stop.test.tsx apps/web/src/features/craft/routes.tsx
git commit -m "feat(craft-web): workbench stop entrance with honest stopping poll"
```

---

### Task 7: React 数据源 confluence + dingtalk 创建入口

**Files:**
- Modify: `apps/web/src/data-sources/DataSourcesPage.tsx:15`（ORDER）
- Modify: `apps/web/src/data-sources/form.ts:142`（字段）、`:160`（引导）
- Modify: `packages/i18n/src/generated/dataSource.ts`（各语言段补键）
- Test: `apps/web/src/data-sources/form.confluence.test.ts`（新建）

**Interfaces:**
- Consumes: `CredentialField = {key, label, placeholder?, secret?, optional?, hint?}`（form.ts:140）；`VUE_CONNECTOR_GUIDES` 形状；后端必填（confluence `edition/base_url/username` + 按 edition `api_token|password`，`connector/confluence/types.go:40-78`；dingtalk `client_id/client_secret/operator_id` 全必填，`connector/dingtalk/client.go:54-61`）
- Produces: 创建向导出现 confluence/dingtalk 卡片（`createTypes = ORDER ∩ /datasource/types`，服务端已注册两类）

- [ ] **Step 1: 写失败测试**

```ts
// apps/web/src/data-sources/form.confluence.test.ts
import { strict as assert } from 'node:assert';
import { test } from 'node:test';
import { VUE_CREDENTIAL_FIELDS, VUE_CONNECTOR_GUIDES, buildDataSourceInput } from './form.ts';

test('confluence credential fields carry the edition-switched secret pair', () => {
  const fields = VUE_CREDENTIAL_FIELDS.confluence ?? [];
  const keys = fields.map((field) => field.key);
  assert.ok(keys.includes('edition'));
  assert.ok(keys.includes('base_url'));
  assert.ok(keys.includes('username'));
  // Both secret fields exist and are optional: which one is REQUIRED depends
  // on the edition — the backend's connection test reports the exact miss.
  const apiToken = fields.find((field) => field.key === 'api_token');
  const password = fields.find((field) => field.key === 'password');
  assert.ok(apiToken?.secret && apiToken.optional);
  assert.ok(password?.secret && password.optional);
});

test('dingtalk credential fields are the three required secrets', () => {
  const keys = (VUE_CREDENTIAL_FIELDS.dingtalk ?? []).map((field) => field.key);
  assert.deepEqual(keys.sort(), ['client_id', 'client_secret', 'operator_id']);
  assert.ok(VUE_CREDENTIAL_FIELDS.dingtalk?.every((field) => field.secret && !field.optional));
});

test('confluence credentials round-trip through the key=value protocol', () => {
  const input = buildDataSourceInput({
    name: 'cf', type: 'confluence', schedule: '0 0 */6 * * *', mode: 'incremental',
    conflict: 'overwrite', deletions: true,
    credentialsText: 'edition = cloud\nbase_url = https://x.atlassian.net\nusername = a@b.c\napi_token = t1',
    settingsText: '', resourceIds: [], authHeaders: [], gitlabProjects: [],
  });
  assert.equal(input.config?.credentials?.edition, 'cloud');
});

test('both new connectors have setup guides', () => {
  assert.ok(VUE_CONNECTOR_GUIDES.confluence?.docUrl);
  assert.ok(VUE_CONNECTOR_GUIDES.dingtalk?.docUrl);
});
```

（`DataSourceFormValues` 的完整字段集照 form.ts 类型定义——上面 credentialsText/settingsText 等字段名与 `buildDataSourceInput` 读取的一致。）

- [ ] **Step 2: 运行确认失败**

Run: `pnpm test:web`
Expected: FAIL —— confluence/dingtalk 键不存在。

- [ ] **Step 3: 实现**

`form.ts` `VUE_CREDENTIAL_FIELDS` 追加：

```ts
  confluence: [
    { key: 'edition', label: 'dataSource.field.confluenceEdition', placeholder: 'server' },
    { key: 'base_url', label: 'dataSource.field.baseUrl', placeholder: 'https://confluence.example.com' },
    { key: 'username', label: 'dataSource.field.confluenceUsername', placeholder: 'user@example.com' },
    { key: 'api_token', label: 'dataSource.field.confluenceApiToken', secret: true, optional: true, hint: 'dataSource.field.confluenceApiTokenHint' },
    { key: 'password', label: 'dataSource.field.confluencePassword', secret: true, optional: true, hint: 'dataSource.field.confluencePasswordHint' },
  ],
  dingtalk: [
    { key: 'client_id', label: 'dataSource.field.dingtalkClientId', secret: true },
    { key: 'client_secret', label: 'dataSource.field.dingtalkClientSecret', secret: true },
    { key: 'operator_id', label: 'dataSource.field.dingtalkOperatorId', secret: true },
  ],
```

`VUE_CONNECTOR_GUIDES` 追加：

```ts
  confluence: { docUrl: 'https://developer.atlassian.com/cloud/confluence/rest/intro/', permissionPageUrl: '', requiredPermissions: [] },
  dingtalk: { docUrl: 'https://open.dingtalk.com/document/orgapp/obtain-orgapp-exclusive-access-token', permissionPageUrl: '', requiredPermissions: [] },
```

`DataSourcesPage.tsx:15`：

```ts
const VUE_CREATE_CONNECTOR_ORDER = ['feishu', 'lark', 'feishu_drive', 'lark_drive', 'notion', 'yuque', 'ima', 'rss', 'gitlab', 'confluence', 'dingtalk'];
```

`packages/i18n/src/generated/dataSource.ts`：**每个语言段**（zh-CN/en-US 及文件中存在的其他语言段）补（非中英语言用英文文案）：

```ts
    'dataSource.connector.confluence': 'Confluence',
    'dataSource.connector.dingtalk': '钉钉（知识库）',          // en: 'DingTalk Knowledge Base'
    'dataSource.connectorDesc.confluence': '同步 Confluence 空间中的页面',   // en: 'Sync pages from Confluence spaces'
    'dataSource.connectorDesc.dingtalk': '同步钉钉知识库中的在线文档',       // en: 'Sync online documents from DingTalk knowledge bases'
    'dataSource.field.confluenceEdition': '版本（server / cloud）',          // en: 'Edition (server / cloud)'
    'dataSource.field.confluenceUsername': '用户名',                        // en: 'Username'
    'dataSource.field.confluenceApiToken': 'API Token',                     // en: 'API Token'
    'dataSource.field.confluenceApiTokenHint': 'cloud 版必填',               // en: 'Required for the cloud edition'
    'dataSource.field.confluencePassword': '密码',                          // en: 'Password'
    'dataSource.field.confluencePasswordHint': 'server 版必填',              // en: 'Required for the server edition'
    'dataSource.field.dingtalkClientId': 'Client ID（AppKey）',              // en: 'Client ID (AppKey)'
    'dataSource.field.dingtalkClientSecret': 'Client Secret（AppSecret）',   // en: 'Client Secret (AppSecret)'
    'dataSource.field.dingtalkOperatorId': '操作人 Operator ID',             // en: 'Operator ID'
```

（键插入位置：紧跟各语言段的 `dataSource.connector.gitlab` / `connectorDesc.gitlab` 之后；中文文案在 zh-CN 段、英文在其余段。）

- [ ] **Step 4: 运行确认通过**

Run: `pnpm test:web && pnpm typecheck:web && pnpm test:shared`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
git status --porcelain
git add apps/web/src/data-sources/DataSourcesPage.tsx apps/web/src/data-sources/form.ts apps/web/src/data-sources/form.confluence.test.ts packages/i18n/src/generated/dataSource.ts
git commit -m "feat(data-sources): confluence + dingtalk creation entries on React (fields, guides, i18n)"
```

---

### Task 8: Vue 数据源 confluence + dingtalk 入口

**Files:**
- Modify: `frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue:499-633`（connectorDefs 追加两源）
- Modify: `frontend/src/i18n/locales/zh-CN.ts`、`frontend/src/i18n/locales/en-US.ts`（补 `datasource.field.*` 等键；若 ja/ko/ru locales 也携带 datasource 块则同步补英文文案）
- Test: 无独立 Vue 测试基建——以 `type-check` + `check-i18n` 为验证门

**Interfaces:**
- Consumes: `ConnectorDef{type, available, docUrl, permissionDocUrl, permissionPageUrl, requiredPermissions, fields:[{key, labelKey, placeholder, secret?, optional?, hintKey?, multiline?, fieldType?}]}`（同文件 :499 样例）；Vue i18n 键前缀 `datasource.field.*`（小写 d，与 React 的 `dataSource.` 不同）
- Produces: Vue 创建向导出现两源卡片

- [ ] **Step 1: connectorDefs 追加（DataSourceEditorDialog.vue，gitlab 条目之后）**

```ts
  {
    type: 'confluence',
    available: true,
    docUrl: 'https://developer.atlassian.com/cloud/confluence/rest/intro/',
    permissionDocUrl: '',
    permissionPageUrl: '',
    requiredPermissions: [],
    fields: [
      { key: 'edition', labelKey: 'datasource.field.confluenceEdition', placeholder: 'server' },
      { key: 'base_url', labelKey: 'datasource.field.baseUrl', placeholder: 'https://confluence.example.com' },
      { key: 'username', labelKey: 'datasource.field.confluenceUsername', placeholder: 'user@example.com' },
      { key: 'api_token', labelKey: 'datasource.field.confluenceApiToken', placeholder: '', secret: true, optional: true, hintKey: 'datasource.field.confluenceApiTokenHint' },
      { key: 'password', labelKey: 'datasource.field.confluencePassword', placeholder: '', secret: true, optional: true, hintKey: 'datasource.field.confluencePasswordHint' },
    ],
  },
  {
    type: 'dingtalk',
    available: true,
    docUrl: 'https://open.dingtalk.com/document/orgapp/obtain-orgapp-exclusive-access-token',
    permissionDocUrl: '',
    permissionPageUrl: '',
    requiredPermissions: [],
    fields: [
      { key: 'client_id', labelKey: 'datasource.field.dingtalkClientId', placeholder: '', secret: true },
      { key: 'client_secret', labelKey: 'datasource.field.dingtalkClientSecret', placeholder: '', secret: true },
      { key: 'operator_id', labelKey: 'datasource.field.dingtalkOperatorId', placeholder: '', secret: true },
    ],
  },
```

（先确认 `datasource.field.baseUrl` 键在 Vue locales 已存在——feishu 条目在用，存在。）

- [ ] **Step 2: Vue locales 补键（zh-CN.ts / en-US.ts 的 datasource 块）**

zh（en-US 同键英文）：

```ts
  'datasource.connector.confluence': 'Confluence',
  'datasource.connector.dingtalk': '钉钉（知识库）',
  'datasource.connectorDesc.confluence': '同步 Confluence 空间中的页面',
  'datasource.connectorDesc.dingtalk': '同步钉钉知识库中的在线文档',
  'datasource.field.confluenceEdition': '版本（server / cloud）',
  'datasource.field.confluenceUsername': '用户名',
  'datasource.field.confluenceApiToken': 'API Token',
  'datasource.field.confluenceApiTokenHint': 'cloud 版必填',
  'datasource.field.confluencePassword': '密码',
  'datasource.field.confluencePasswordHint': 'server 版必填',
  'datasource.field.dingtalkClientId': 'Client ID（AppKey）',
  'datasource.field.dingtalkClientSecret': 'Client Secret（AppSecret）',
  'datasource.field.dingtalkOperatorId': '操作人 Operator ID',
```

- [ ] **Step 3: 验证**

Run: `cd frontend && npm run type-check && npm run check-i18n`
Expected: 两项全绿（check-i18n 的 localeKeyAudit 会抓缺失键——若 audit 报 ja/ko/ru 缺键，按其指引同步补英文文案后重跑）。

- [ ] **Step 4: 提交**

```bash
git status --porcelain
git add frontend/src/views/knowledge/settings/DataSourceEditorDialog.vue frontend/src/i18n/locales/zh-CN.ts frontend/src/i18n/locales/en-US.ts
# 若补了 ja/ko/ru 一并 add
git commit -m "feat(data-sources): confluence + dingtalk creation entries on Vue (connectorDefs + i18n)"
```

---

### Task 9: 幽灵连接器清理 + 台账登记

**Files:**
- Modify: `internal/datasource/connector.go:240-295`（删 6 条 registry 条目：github/google_drive/onedrive/web_crawler/slack/imap；dingtalk :264 保留）
- Test: `internal/datasource/connector_registry_test.go`（新建或并入既有 connector_test.go）
- Modify: `docs/onyx-parity/ROADMAP.md`（SP1 状态 → ✅）、`docs/onyx-parity/GAP-MATRIX.md`（C-25/C-3/K-26/K-5 行尾标注 SP1 完成）

**Interfaces:**
- Consumes: `ConnectorMetadataRegistry`（:175-312，17 条）；`ListAvailableConnectors()`（:314-331）
- Produces: `/api/v1/datasource/types` 只返回 11 个实装源（`initConnectorRegistry` `container.go:2021-2056` 注册集）

- [ ] **Step 1: 写失败测试**

```go
// internal/datasource/connector_registry_test.go
package datasource

import (
	"slices"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

// TestConnectorMetadataRegistryMatchesImplementedSet pins the registry to
// exactly the connector set the container registers (container.go
// initConnectorRegistry). A metadata entry without an implementation is a
// ghost: it surfaces fake options in the edit-mode type dropdown.
func TestConnectorMetadataRegistryMatchesImplementedSet(t *testing.T) {
	want := []string{
		types.ConnectorTypeFeishu, types.ConnectorTypeLark,
		types.ConnectorTypeFeishuDrive, types.ConnectorTypeLarkDrive,
		types.ConnectorTypeNotion, types.ConnectorTypeYuque,
		types.ConnectorTypeIMA, types.ConnectorTypeRSS,
		types.ConnectorTypeGitLab, types.ConnectorTypeConfluence,
		types.ConnectorTypeDingTalk,
	}
	got := make([]string, 0, len(ConnectorMetadataRegistry))
	for source := range ConnectorMetadataRegistry {
		got = append(got, source)
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Fatalf("registry/implementation drift:\n want %v\n got  %v", want, got)
	}
}
```

（`types.ConnectorType*` 常量名以 `internal/types/datasource.go:17-40` 实际导出名为准——先打开核对拼写再定稿。）

- [ ] **Step 2: 运行确认失败**

Run: `go test ./internal/datasource/ -run TestConnectorMetadataRegistryMatchesImplementedSet -count=1`
Expected: FAIL —— registry 还有 17 条（ghost 条目 drift）。

- [ ] **Step 3: 删除 6 条幽灵条目**

从 `ConnectorMetadataRegistry` 删除 `github:`、`google_drive:`、`onedrive:`、`web_crawler:`、`slack:`、`imap:` 六个 map 条目（连同各自注释块）。**保留** `types.ConnectorType*` 常量（存量数据源行与 SP7-9 复用）。在删除位置留一行注释：

```go
	// SP1 ghost cleanup: github/google_drive/onedrive/web_crawler/slack/imap
	// metadata entries were removed until their connectors are implemented
	// (they surfaced fake options in the edit-mode type dropdown). The
	// types.ConnectorType* constants stay for future registrations.
```

- [ ] **Step 4: 运行全部相关测试与构建**

Run: `go test ./internal/datasource/... -count=1 && go build ./...`
Expected: PASS（既有 connector_test.go:10 feishu 断言与 ima 测试不受影响）。

- [ ] **Step 5: 台账登记**

- `docs/onyx-parity/ROADMAP.md`：SP1 行状态 🔄 → ✅（附验证证据一句话：各测试命令绿 + 端到端冒烟见下）
- `docs/onyx-parity/GAP-MATRIX.md`：C-25、C-3、K-26、K-5 四行"定性"更新为 ✅（SP1）
- 变更记录追加一行：`2026-09-19：SP1 四项接线完成（commit 见 git log）`

- [ ] **Step 6: 提交**

```bash
git status --porcelain
git add internal/datasource/connector.go internal/datasource/connector_registry_test.go docs/onyx-parity/ROADMAP.md docs/onyx-parity/GAP-MATRIX.md
git commit -m "fix(datasource): remove ghost connector metadata entries (registry==implementation, SP1 closeout)"
```

---

## 端到端冒烟（Task 9 后一次性执行，不单独成任务）

源码栈（后端 8082 + React 5175 + Vue 5174，环境要点见记忆 `local-dev-ports-and-pitfalls`）：

1. 设 `CRAFT_OPENCODE_BASE_URL` + `WEKNORA_CRAFT_GATEWAY_SECRET`（≥16 字节）重启后端 → `curl -X POST .../api/v1/craft/model-gateway/v1/chat/completions`（无凭据）应 401；未设 secret 的常规启动下该路径应 404
2. React 与 Vue 各创建一个 dingtalk 数据源（假凭据）→ "测试连接"应返回后端的凭据校验错误（证明表单→config→Validate 链路通）
3. `/api/v1/datasource/types` 返回 11 类，无 github/slack 等假项
4. （有真 OC runtime 栈时）craft 运行中点停止 → phase stopping → 轮询收敛终态

---

## Self-Review 记录

- **Spec 覆盖**：spec §3.1→Task 2、§3.2/3.4/3.5→Task 3、§3.3→Task 1、§4.1→Task 4、§4.2→Task 4 Step 4、§4.3→Task 5+6、§5→Task 7+8、§6→Task 9、§8 测试清单→各 Task 步骤+端到端冒烟。spec §3.5 的"secret 缺失即装配报错"在本计划细化为"设了但过短→报错；未设→不装配"（默认关语义，避免空 secret 拦截所有部署）。
- **类型一致性**：`CraftUpstreamTarget`/`CraftModelGatewayConfig`（Task 2/3 一致）；`CraftStopRequest{Scope,RunKey,TaskID}`/`CraftStopStatus{Phase,Result,Note}`（Task 4/5 一致）；api-client `stop`/`delegationStatus` 与 workbench `onStopRun`/`stopPhase`（Task 5/6 一致）。
- **已知不确定点**（实施时以既有代码为准，均已给出核对方法而非空指令）：000036 DDL 的账户列名（Task 1 Step 1）、routes_chat.go 的 group/导入别名（Task 3 Step 5）、craft_control_test fixture 复用（Task 4 Step 1）、handler 测试的 context 注入方式（Task 4 Step 6 提示）、workbench 测试 harness（Task 6 Step 1）、`types.ConnectorType*` 常量拼写（Task 9 Step 1）。
