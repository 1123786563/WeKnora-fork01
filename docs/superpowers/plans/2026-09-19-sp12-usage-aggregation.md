# SP12 用量聚合（Onyx 平台运营面对齐）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新建 `user_usage` 日桶表、chat/craft 双写入点 upsert 累加 token/成本、三个用量 API（个人/全员/CSV 导出）、settings 用量分区与 analytics 页用量 tab（GAP-MATRIX P-1/P-2）。

**Architecture:** 日桶表 `(tenant_id, user_id, window_start, model, flow)` 唯一 + OnConflict gorm.Expr 累加；chat 写入点挂 `completeAssistantMessage`（终态 defer 恰好一次），craft 写入点在 `CraftUsageStore.record` 事务内 JOIN agent_runs 补 user_id；计价经 `WEKNORA_USAGE_RATES` env JSON 加载为 `PriceVersionRates`（缺省全 0，复用 commercial 微积分算法）。

**Tech Stack:** Go（gin+GORM+golang-migrate）、React 19 + Tailwind v4、node:test + tsx。

**Spec:** `docs/superpowers/specs/2026-09-19-onyx-platform-parity-design.md` 第 4 节 SP12 部分；GAP-MATRIX P-1/P-2。

## Global Constraints

- 成功响应 `gin.H{"success": true, "data": ...}`；错误 `c.Error(errors.NewXxx)` 交 ErrorHandler。
- 迁移成对 up/down：versioned 下一个 **000160**、sqlite 下一个 **000081**（sqlite 头注释 `-- SQLite dialect of PG 000160`）。
- 新路由必须经 `g.apiKeyGroup(...)` 声明；时间参数 `start_time/end_time`（复用 `parseFilterTime`，纯日期 end 含全天语义照 SP11 `analyticsIsDateOnly` 先例）。
- repository 测试用 `openRunTestDB(t)`（真迁移；postgres 子测试名 `/postgres` 需 DSN 否则 Skip）。
- 前端测试 node:test；api-client/contracts 模式照 SP11 已建域（analytics/feedback）。
- commit 前核对 porcelain 只暂存本任务文件；**绝不 git add -A / git add - .**（并行会话整树提交前科，只 add 显式路径）。
- 每回合恰好一次记账：chat 侧写入前判 `assistantMessage.IsCompleted`（true=重复调用跳过）；craft 侧继承 UsageKey 幂等。

---

### Task 1: user_usage 迁移 + 模型 + UsageRepository

**Files:**
- Create: `migrations/versioned/000160_user_usage.up.sql` / `.down.sql`、`migrations/sqlite/000081_user_usage.up.sql` / `.down.sql`
- Create: `internal/types/user_usage.go`、`internal/types/interfaces/user_usage.go`
- Create: `internal/application/repository/user_usage.go`
- Test: `internal/application/repository/user_usage_test.go`
- Modify: `internal/container/container.go`（Provide 一行）

**Interfaces:**
- Produces（Task 3/4/5 依赖）:
```go
// types
type UserUsage struct {
    ID              uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
    TenantID        uint64    `json:"tenant_id" gorm:"index"`
    UserID          string    `json:"user_id" gorm:"type:varchar(512);not null;default:''"`
    WindowStart     time.Time `json:"window_start" gorm:"not null"` // UTC 日界
    Model           string    `json:"model" gorm:"type:varchar(128);not null;default:''"`
    Flow            string    `json:"flow" gorm:"type:varchar(32);not null;default:''"` // chat|craft|search
    InputTokens     int64     `json:"input_tokens" gorm:"not null;default:0"`
    OutputTokens    int64     `json:"output_tokens" gorm:"not null;default:0"`
    CacheReadTokens int64     `json:"cache_read_tokens" gorm:"not null;default:0"`
    CacheWriteTokens int64    `json:"cache_write_tokens" gorm:"not null;default:0"`
    CostMicrocredits int64    `json:"cost_microcredits" gorm:"not null;default:0"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
func (UserUsage) TableName() string { return "user_usage" }
const UsageFlowChat = "chat"; const UsageFlowCraft = "craft"

// interfaces
type UsageAggregate struct { // 聚合返回行（日/模型或日/模型/用户）
    WindowStart string `json:"window_start"`
    Model       string `json:"model"`
    UserID      string `json:"user_id,omitempty"`
    InputTokens int64  `json:"input_tokens"`
    OutputTokens int64 `json:"output_tokens"`
    CacheReadTokens int64 `json:"cache_read_tokens"`
    CacheWriteTokens int64 `json:"cache_write_tokens"`
    CostMicrocredits int64 `json:"cost_microcredits"`
}
type UsageRepository interface {
    AddUsage(ctx context.Context, u *types.UserUsage) error // OnConflict(dims) gorm.Expr 累加
    AggregateByUser(ctx context.Context, tenantID uint64, userID string, from, to time.Time) ([]UsageAggregate, error)
    AggregateAllUsers(ctx context.Context, tenantID uint64, from, to time.Time, limit, offset int) ([]UsageAggregate, error)
    ExportRows(ctx context.Context, tenantID uint64, from, to time.Time) ([]UsageAggregate, error) // user 维度汇总行
}
```

- [ ] **Step 1: PG 迁移** `migrations/versioned/000160_user_usage.up.sql`：

```sql
DO $$ BEGIN RAISE NOTICE '[Migration 000160] user_usage'; END $$;
CREATE TABLE IF NOT EXISTS user_usage (
    id BIGSERIAL PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    user_id VARCHAR(512) NOT NULL DEFAULT '',
    window_start DATE NOT NULL,
    model VARCHAR(128) NOT NULL DEFAULT '',
    flow VARCHAR(32) NOT NULL DEFAULT '',
    input_tokens BIGINT NOT NULL DEFAULT 0,
    output_tokens BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens BIGINT NOT NULL DEFAULT 0,
    cache_write_tokens BIGINT NOT NULL DEFAULT 0,
    cost_microcredits BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uq_user_usage_dims UNIQUE (tenant_id, user_id, window_start, model, flow)
);
CREATE INDEX IF NOT EXISTS idx_user_usage_tenant_window ON user_usage (tenant_id, window_start);
```

down：`DROP TABLE IF EXISTS user_usage;`。sqlite 方言版 000081（BIGSERIAL→INTEGER AUTOINCREMENT、TIMESTAMPTZ→DATETIME、DATE 保持 DATE，其余同构）。

- [ ] **Step 2: 失败测试** `user_usage_test.go`：种子两次 AddUsage 同维度（1+2 token）断言一行累加 3；不同 model/flow 分行；AggregateByUser 按 UTC 日分组返回 `window_start` 为 `YYYY-MM-DD` 字符串（PG `to_char(window_start,'YYYY-MM-DD')`、sqlite `strftime`/直接 cast——sqlite DATE 列直接 string；双方言分支照 SP11 dayExpr 模式）；AggregateAllUsers limit/offset 分页；ExportRows 按 user 汇总。
- [ ] **Step 3: 跑测确认失败** `go test ./internal/application/repository/ -run TestUserUsage -v`
- [ ] **Step 4: 实现**：AddUsage 照 feedback.go OnConflict 模式，DoUpdates 用 map：

```go
DoUpdates: map[string]interface{}{
    "input_tokens":      gorm.Expr("user_usage.input_tokens + ?", u.InputTokens),
    "output_tokens":     gorm.Expr("user_usage.output_tokens + ?", u.OutputTokens),
    "cache_read_tokens": gorm.Expr("user_usage.cache_read_tokens + ?", u.CacheReadTokens),
    "cache_write_tokens": gorm.Expr("user_usage.cache_write_tokens + ?", u.CacheWriteTokens),
    "cost_microcredits": gorm.Expr("user_usage.cost_microcredits + ?", u.CostMicrocredits),
    "updated_at":        time.Now().UTC(),
},
```

（表名限定 `user_usage.` 防 PG 歧义；sqlite 同构支持。）
- [ ] **Step 5: 绿 + DI Provide + commit** `feat(usage): user_usage 日桶表与 repository（SP12 P-1）`

### Task 2: 计价管道（env 费率加载器）

**Files:**
- Create: `internal/usage/pricing.go`（新包 internal/usage）
- Test: `internal/usage/pricing_test.go`

**Interfaces:**
- Produces:
```go
package usage
// RatesFromEnv 解析 WEKNORA_USAGE_RATES（JSON: {"<model_id>":{"rate_micro":1200,"units":1000000}}）
// 返回 func(model string) int64（该模型一次调用的 cost 由调用方乘数量；这里给单价 helper）
func RatesFromEnv(raw string) (ModelRates, error)
type ModelRates map[string]Rate
type Rate struct{ RateMicro int64 `json:"rate_micro"`; Units int64 `json:"units"` }
func (m ModelRates) CostMicro(model string, inputTokens, outputTokens int64) int64
// CostMicro 语义：无该模型费率→0；有→big.Rat 精确算 (rate_micro/units)*(input+output) 四舍五入一次
// （维度单价 input/output 同价起步——YAGNI，split 维度留待真有分维费率时）
```

- [ ] **Step 1: 失败测试**：合法 JSON 解析、坏 JSON 报错、无费率模型返回 0、费率计算用 big.Rat 精度断言（如 rate_micro=1500,units=1000000, tokens=3333 → 5（1500*3333/1e6=4.9995 四舍五入→5））、空 env（""）返回空 rates 不报错。
- [ ] **Step 2: 确认失败 → 实现**（复用 `internal/commercial` 的 `roundHalfAwayFromZero` 若导出，未导出则本包内同语义实现并注释对齐）。
- [ ] **Step 3: 绿 + commit** `feat(usage): env 驱动模型费率与微积分计价（SP12 P-1）`

### Task 3: chat 写入点（UsageRecorder service + completeAssistantMessage 挂钩）

**Files:**
- Create: `internal/application/service/usage_recorder.go`
- Test: `internal/application/service/usage_recorder_test.go`
- Modify: `internal/handler/session/qa.go`（completeAssistantMessage 内挂调用）、`internal/container/container.go`（Provide）

**Interfaces:**
- Consumes: Task 1 UsageRepository、Task 2 ModelRates。
- Produces:
```go
// interfaces 加：
type UsageRecorderService interface {
    RecordChatTurn(ctx context.Context, tenantID uint64, userID, model string, usage *types.TokenUsage) error
}
// 实现：userID 空/model 空/usage nil → 静默跳过（返回 nil）；WindowStart=usage 时间 UTC 日界（用 time.Now().UTC() 截断）；
// flow=chat；CacheReadTokens=usage.CacheReadTokens（legacy CachedTokens 兜底：read 为 0 且 CachedTokens>0 时用它）、CacheWrite=usage.CacheWriteTokens；
// cost=ModelRates.CostMicro(model, Prompt, Completion)
```

- [ ] **Step 1: 失败测试**（stub UsageRepository 收参断言维度映射/nil usage 跳过/空 model 跳过/cache 兜底逻辑）。
- [ ] **Step 2: 实现 + container Provide。**
- [ ] **Step 3: qa.go 挂钩**——`completeAssistantMessage`（qa.go:1665）开头，UpdateMessage 之前：

```go
if !assistantMessage.IsCompleted && assistantMessage.Usage != nil {
    bg := context.WithoutCancel(ctx)
    if err := h.usageRecorder.RecordChatTurn(bg, tenantIDFromCtx(ctx), userIDForMessage, assistantMessage.ModelID, assistantMessage.Usage); err != nil {
        logger.WarnWithFields(bg, err, nil) // 记账失败不影响消息完成
    }
}
```

tenant/user 的获取：completeAssistantMessage 现签名无这两个值——查调用链（qa.go:1212-1243 reqCtx 有 session.TenantID 与用户上下文），最小改法=给 completeAssistantMessage 加两个参数（tenantID uint64, userID string）并更新两处调用点（qa.go:1243 defer、helpers.go:336 stop 路径）+ 相关测试签名。userID 从 reqCtx 的 gin context（`c.Get(types.UserIDContextKey.String())`）或 session.UserID 取（两调用点哪个可得用哪个，grep 后定）。
- [ ] **Step 4: 集成验证**：`go build ./...` + handler/session 包既有测试不破（grep completeAssistantMessage 的测试 mock 改签名）。
- [ ] **Step 5: commit** `feat(usage): chat 回合终态日桶记账（SP12 P-1）`

### Task 4: craft 写入点（record 事务内 JOIN agent_runs）

**Files:**
- Modify: `internal/application/repository/craft_usage.go`（record 事务内追加日桶累加）
- Test: `internal/application/repository/craft_usage_test.go`（追加用例）

**Interfaces:**
- Consumes: Task 1 的 user_usage 累加 SQL（此处直接 tx 内 Raw SQL，不走 UsageRepository——保持同事务）。
- 设计：record 事务内、`OnConflict DoNothing` 建 fact 行成功后：

```sql
INSERT INTO user_usage (tenant_id, user_id, window_start, model, flow, input_tokens, output_tokens, cache_read_tokens, cache_write_tokens, cost_microcredits, created_at, updated_at)
SELECT f.tenant_id, COALESCE(r.user_id, ''), date(f.observed_at), f.model_id, 'craft', f.input_tokens, f.output_tokens, f.cached_tokens, 0, 0, now-ish, now-ish
FROM (VALUES ...)  -- 或用已插入 row 的变量绑定
LEFT JOIN agent_runs r ON r.id = f.run_id  -- agent_runs 的 user_id 列名以实际 schema 为准（执行期 grep）
ON CONFLICT (tenant_id, user_id, window_start, model, flow) DO UPDATE SET input_tokens = user_usage.input_tokens + excluded.input_tokens, ... , updated_at = excluded.updated_at
```

用 GORM 变量绑定而非 VALUES 行（把 row 字段直接做参数；`date(?)` sqlite 与 `?::date` PG 双方言分支照 dayExpr 模式）。cost 先 0（craft 计价已有 commercial_usage 管道，不重复计价——spec 说 craft 汇入的是 token 维度）。幂等天然继承：record 开头"内容相等 no-op return"发生在日桶累加之前。

- [ ] **Step 1: 失败测试**：craft_usage_test.go 追加——种 agent_runs（含 user_id）+ Append fact 两次（同 key 重放）→ user_usage 一行且 token 只累加一次；无匹配 run → user_id=''；correction 路径（Correct）不双计（若 Correct 走同一 record，内容相等分支会挡——测试钉住）。
- [ ] **Step 2: 确认失败 → 实现 → 绿**（跑 `go test ./internal/application/repository/ -run TestCraftUsage -v` 含既有用例不破）。
- [ ] **Step 3: commit** `feat(usage): craft usage fact 同事务汇入日桶（SP12 P-1）`

### Task 5: 三端点 API + 路由 + container

**Files:**
- Create: `internal/handler/usage.go`
- Test: `internal/handler/usage_test.go`
- Create: `internal/router/routes_usage.go`
- Modify: `internal/router/router.go`（params 字段+调用）、`internal/container/container.go`

**Interfaces:**
- Produces:
  - `GET /api/v1/usage/me?start_time&end_time`（g.Viewer；从 ctx 取 tenant+user）→ data = UsageAggregate[]（按日/模型）
  - `GET /api/v1/admin/usage/by-user?page&page_size&start_time&end_time`（g.Admin；offset 分页）→ data = UsageAggregate[]（含 user_id）
  - `GET /api/v1/admin/usage/export?start_time&end_time`（g.Admin；CSV：照 faq.go header+BOM 模式，列 user_id,model,flow,window_start,input,output,cache_read,cache_write,cost_microcredits；用手拼 escapeCSVField 语义或 encoding/csv.Writer→bytes.Buffer 一次性返回）
- apiKeyGroup 声明（usage 组对 API-key：`/usage/me` 归 apiKeyChat 档、admin 两端点 apiKeyFullAccess——照 routes_chat/routes_analytics 模式）。
- 时间解析复用 SP11 `parseAnalyticsRange`（同包 internal/handler/analytics.go，直接调用；纯日期 end 含当日全天语义复用 `analyticsIsDateOnly`——注意它当前是小写私有，同包可用）。

- [ ] **Step 1: 失败测试**（stub repo：me 只传 ctx 用户、by-user 分页透传、export 断言 Content-Type/Disposition/BOM/表头行）。
- [ ] **Step 2: 实现 + 三处接线（routes_usage.go/router.go/container.go）→ build + 测试绿。**
- [ ] **Step 3: commit** `feat(usage): 个人/全员/导出三端点（SP12 P-2）`

### Task 6: contracts + api-client usage 域

**Files:**
- Create: `packages/contracts/src/usage.ts` + `usage.test.ts`（照 analytics.ts 模式：UsageAggregateRow 接口 + parseMyUsageResponse/parseUsageByUserResponse）
- Create: `packages/api-client/src/usage/index.ts` + `index.test.ts`（createUsageApi：my(params)/byUser(params)/exportCsv(params)→触发浏览器下载 blob 或返回 Blob——照仓库现有下载先例 grep `requestBinary`/`download` 用法，若有 requestBinary 则 exportCsv 用它返回 Blob 由调用方处理）
- Modify: `packages/contracts/src/index.ts`（re-export）、`packages/api-client/src/client.ts`（装配顶层 usage）

- [ ] **Step 1: contracts 失败测试→实现→绿**（字段与 Task 1 UsageAggregate JSON tag 对齐：window_start/model/user_id/input_tokens/output_tokens/cache_read_tokens/cache_write_tokens/cost_microcredits）。
- [ ] **Step 2: api-client 失败测试→实现→绿**（stub 断言 path/query 映射）。
- [ ] **Step 3: commit** `feat(contracts,api-client): usage 契约与客户端域（SP12 P-2）`

### Task 7: settings 用量分区（用户级页面）

**Files:**
- Create: `apps/web/src/settings/UsagePanel.tsx`
- Modify: `packages/views/src/settings/registry.ts`（`{ key: 'usage', viewId: 'UsageSettings', apiDomain: 'usage', scope: 'user', minRole: 'viewer', operations: ['read'], ported: true }`）、`apps/web/src/settings/surface.ts`（descriptions 加一条）、`apps/web/src/settings/SettingsPage.tsx`（readSettingsSection case（面板自拉可不加）/renderSectionPanel 分支）、`packages/i18n`（settings.usage 标题+表格列名等键，五 locale）
- Test: `apps/web/src/settings/usage-panel.test.tsx`（纯函数：聚合行→按模型汇总/总计计算的格式化函数）

**面板内容**：时间范围（复用 analytics-range.ts 的 defaultAnalyticsRange/clamp）+ 按模型表格（行=模型，列=input/output/cache/cost，末行合计）+ 预算余量卡片（`client.commercial.summary()` 已有——失败静默隐藏该卡）+ 数据来自 `client.usage.my()`。空态/loading/error 三态照 ConfigSettingsPanel 的 stats 块模式。

- [ ] **Step 1: 纯函数测试（聚合计算/格式化）→实现→绿。**
- [ ] **Step 2: 四处注册 + Panel 实现 → typecheck 绿。**
- [ ] **Step 3: commit** `feat(web): settings 用量分区（SP12 P-2）`

### Task 8: analytics 页用量 tab

**Files:**
- Modify: `apps/web/src/analytics/AnalyticsPage.tsx`（用 `@weknora/ui` 的 Tabs 包两个面板：现有四图 grid 为"图表"tab；新增"用量"tab= byUser 表格+CSV 导出按钮（调 client.usage.exportCsv 触发下载）+同页日期范围联动）
- Modify: `packages/i18n`（analytics.usageTab 等键五 locale）
- Test: `apps/web/src/analytics/analytics-usage-tab.test.tsx`（纯函数：byUser 行排序/汇总）

- [ ] **Step 1: 测试→实现→typecheck 绿。**
- [ ] **Step 2: commit** `feat(web): analytics 用量 tab 与 CSV 导出入口（SP12 P-2）`

### Task 9: 回归、冒烟、证据与登记

**Files:**
- Create: `docs/migrations/react/evidence/onyx-parity/2026-09-19-sp12-usage-aggregation.md`
- Modify: `docs/onyx-parity/ROADMAP.md`（SP12 ✅）、`docs/onyx-parity/GAP-MATRIX.md`（P-1/P-2 ✅）

- [ ] 后端全量回归（判归属：本线 usage 相关必须全绿）+ 前端回归。
- [ ] 冒烟（共享 dev 栈若在）：发一轮聊天 → `GET /api/v1/usage/me` 出数；craft 一轮 → flow=craft 行出现；`/platform/settings?section=usage` 表格渲染；analytics 用量 tab+CSV 下载。起不动则记欠账。
- [ ] evidence：交付清单/回归归属/冒烟/遗留（已知遗留预填：craft cost 列暂 0（commercial 管道已有计价，日桶不重复计）；input/output 同价（分维费率留待真需要）；DB 无费率表（env 配置驱动）；debug 重放 completeAssistantMessage 依赖 IsCompleted 防御）。
- [ ] commit `docs(parity): SP12 用量聚合验收与登记（P-1/P-2 ✅）`

---

## Self-Review 记录

- Spec 覆盖：P-1（表/双写入点/计价管道）→ Task 1-4；P-2（三 API+两处前端）→ Task 5-8；登记 → Task 9 ✓
- 类型一致性：UsageAggregate 字段在 Task 1（Go）/Task 6（contracts）/Task 7-8（消费）间对齐；flow 常量 chat/craft 一致 ✓
- 执行期确认点（已写 grep 指引）：agent_runs 的 user_id 列名、requestBinary 下载先例、completeAssistantMessage 调用点的 user 值来源、roundHalfAwayFromZero 导出性
- 与 SP11 的复用：OnConflict 模式（feedback.go）、dayExpr 双方言（analytics.go）、parseAnalyticsRange/analyticsIsDateOnly、faq CSV header 模式、analytics-range.ts
