# t14 Credits Order 真实栈验收记录（Issue #86）

**日期**：2026-09-28 · **执行者**：实现员-86（worktree `.worktrees-issue72/issue-86`，分支 `codex/issue-72-lago-86`）

## 环境

| 组件 | 来源 | 端口 | 说明 |
|---|---|---|---|
| Lago 栈 | `deploy/lago`（`COMPOSE_PROJECT_NAME=weknora-lago-86v`） | 48897(api)/48898(front) | v1.53.0（images.lock.json pin）；`lago.sh init` 后改端口 + `LAGO_CREATE_ORG=true` 种 org/API key |
| WeKnora 后端 | worktree `cmd/server`（config 副本 sed 48086） | 48086 | env：provider=lago、URL=48897、API key source 自 `deploy/lago/.env`、`WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=true`；PG 独立库 `weknora_t86`（宿主 WeKnora-postgres-dev:5432） |
| 前端 | `pnpm --filter @weknora/web dev --port 5186`（代理 48086） | 5186 | |

端口选择遵循计划并行纪律（避开 48889/48890 `weknora-lago-82r5`、48895/48896 `weknora-lago-t11`；执行日 `docker ps` 复核 48897/48898 空闲）。

## 集成测试（`//go:build lago_integration`）

`TestLagoCreditsOrder`（`internal/commercial/commercialplatform/lago_credits_order_integration_test.go`）
四阶段全 PASS（真实运行时 v1.53.0）：

- **a** 月度钱包经 `EnsureBenefits` 创建后 `GET wallets` 断言 `priority == 1`（初值编码落线）。
- **b** 直建充值形状钱包（无 period 键、priority 2、+12 月）→ 快照列出 `source=topup`、余额合计含它（6.00）。
- **c** 新租户先种老化充值（到期 2026-09-29，早于期末 10-01）再首访 → 让位月月度钱包创建后经链尾 rebalance 收敛为 aging=1 / monthly=2（让位初值 3 是 grant→rebalance 之间的中间态，服务级 `TestMonthlyGrantEncodesYieldPriority` 证明；完整链上的可观察终态即此收敛序）。
- **d** 混合三族（aging 2 / monthly 3 / fresh 2）→ 一次 `rebalance_credits_order` → **A=1、M=2、B=3**（PUT priority 在 v1.53 真实生效——spec L132 runtime verification 闭环）；第二次 rebalance 零变更。

运行命令：
```
LAGO_INTEGRATION_BASE_URL=http://127.0.0.1:48897 LAGO_INTEGRATION_API_KEY=<org key> \
  go test -tags lago_integration ./internal/commercial/commercialplatform/ -run TestLagoCreditsOrder -count=1 -v
```
输出：`--- PASS: TestLagoCreditsOrder (1.80s)`（四阶段日志见测试 stdout）。

## 浏览器流程（Playwright MCP，页面 `http://localhost:5186/platform/billing`）

| 步骤 | 断言 | 结果 | 截图 |
|---|---|---|---|
| 1 注册 t86owner（租户 10000）→ 首次账单页 | 分解卡：总余额 1.00 / 预占 0.00 / 退款锁定 0.00 / 可用 1.00；批次一行「套餐月度 2026-09 · 到期 2026-10-01（期末）· 1.00」；对账时间非空 | PASS | `01-monthly-only.png` |
| 2 种充值钱包（granted 5、+12 月、priority 2）→ Reload | 总余额 6.00；两行：充值批次（到期 2027-09-27、5.00）+ 月度不变 | PASS | `02-with-topup.png` |
| 3 对账（AC5） | `reconcile.sh`：Lago Σ active balance_cents×10⁴ == 页面 balance_micro（6000000==6000000，wallets=2==batches=2） | PASS | `reconcile-output.txt`（`RECONCILE PASS`） |
| 4 让位 + 混合重排（租户 10002：先种老化再首访 → 再种新充值 → 账单访问触发刷新链） | curl 断言钱包 priority：老化=1（exp 09-29）、月度=2（exp 10-01，让位）、新充值=3（exp 2027-03）——r1 审查 High 判例真实栈闭环 | PASS | `03-yield-rebalance.png`（Lago front :48898 wallets 视图） |
| 5 可用扣减口径 | 插入 `held_micro=200000` 行 → Reload：预占 0.20、可用 6.00−0.20−0.00=**5.80**；随后删除该行 | PASS | `04-held.png` |
| 6 月度不结转 | 栈为真实时钟（api-clock 无冻结/快进 env，`docker inspect` 无 CLOCK 变量）——按计划边界处理：(a) API 断言月度钱包 `expiration_at`=期末、充值=+12 月（对象语义权威表达，步骤 2/4 已证）；(b) 过期归零由 `TestExpiredBatchSurfacesZero`（既有）+ `TestSyncLotsNeverResurrectsExpiredBatch`（Task 3）覆盖；(c) 真时间跨月联验待 #87/#88 admission 波次的真实消费一并补 | 边界记录 | — |

## 凭据纪律

API key/密码仅 source 自 `deploy/lago/.env` 注入环境；截图与文档不含 key 字面量（登录页截图不涉及；Lago front 截图为 wallets 数据区）。测试账号（t86owner/t86second/t86third @weknora.local）为本地一次性种子，栈已销毁。

## 附带修复（执行中发现）

- `BenefitsStore.EnsureSchema` 原为 SQLite 方言 DDL（`AUTOINCREMENT`），在 PostgreSQL 上语法错——改为 gorm AutoMigrate（跨方言、幂等）。这是基线缺陷（PG 部署下服务启动即 panic），非本计划任务项，但不修则真实栈验证（PG 后端）无法进行。
- `budget_pg_test.go` fixture 补 000161 owner 列（经 gorm Migrator，镜像 000161 DDL）——计划 Task 3 Step 4 的显式交付物；既有 2 个 FAIL 测试（issue inventory L180）随之转绿。
