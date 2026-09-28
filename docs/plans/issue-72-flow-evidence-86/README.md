# Issue #86 真实流程验证证据（[Lago 14] 套餐额度与充值额度按到期顺序消费）

验证日期：2026-09-28。验证人：流程验证员（动态工作流 subagent）。

## 环境（全部真实运行）

| 组件 | 来源 | 地址 | 说明 |
|---|---|---|---|
| Lago 栈 | `deploy/lago` compose（运行中实例 `weknora-lago-82r5-*`） | API 127.0.0.1:48889 / front :48890 | v1.53.0，`GET /health` OK；org `weknora-r5` |
| WeKnora 后端 | worktree `.worktrees-issue72/lago-int`（集成分支 `codex/issue-72-lago` HEAD `dd089662b`）`./scripts/dev.sh app`（air + go run） | 127.0.0.1:8080 | env：`WEKNORA_COMMERCIAL_PLATFORM_PROVIDER=lago`、`PLATFORM_URL=http://127.0.0.1:48889`、`OUTBOUND_ALLOW_LOOPBACK=true`；`DB_DRIVER=postgres` 指向 WeKnora-postgres-dev 独立库 **WeKnora86**（避免污染共享库：共享 WeKnora 库已被 main 分支迁移到 193 > 集成分支 188，降级失败；独立库迁移到 188 后由后端建全部 commercial 表） |
| WeKnora 前端 | 同 worktree `pnpm --filter @weknora/web dev` | http://localhost:5173（IPv6 [::1]） | /api 代理到 :8080 |
| 浏览器 | Playwright（MCP 插件，headless 真实渲染） | — | 登录→操作→断言→截图 |

测试数据：全新空间 `flow86@weknora.local`（租户 ID **10000**，customer `weknora-tenant-10000`，Lago customer lago_id `682daaca-…`），不触碰其他 Issue 的租户。后续审计发现本目录的本地环境快照曾包含凭据；快照中的值现已全部 redacted，且快照路径已加入 ignore。本文不复现任何凭据值。

## 用户流程与断言结果（全部真实验证）

### ① 余额分解区块（截图 01/02/06）

浏览器注册/登录新空间 → 首次打开 `/platform/billing` 触发 lazy ensure（customer + subscription + base plan + 月度 grant 1.00）：

- 月度批次：`套餐月度 2026-09 · 发放日 2026-09-28 · 到期 2026-10-01 · 余额 1.00`（01-monthly-only.png）
- Lago 侧月度钱包 `weknora-tenant-10000-2026-09`：`priority=1`（#86 创建初值编码落线）、`balance_cents=100`、`expiration_at=2026-10-01T00:00:00Z`（lago-baseline-tenant10000.json）
- 种两个充值形状钱包（#85 到账命令的对象形状：无 `weknora_period` 键、仅 `weknora_tenant` 锚）后 Reload：总余额 11.00 = 1+5+5、批次表三行（充值 A 到期 2027-02-01、充值 B 到期 2027-09-28）（02-with-topup.png）
- **刷新链权威重排**：B 创建初值 priority=2 被刷新链 rebalance 校准为 3 → 三钱包 priority 序 M=1、A=2、B=3 **严格等于到期序**（lago-after-rebalance.txt；PUT 由 `rebalance_credits_order` 命令发出）
- **可用口径**：向 `commercial_budget_accounts` 插入 `held_micro=200000`（0.20）→ 页面显示 总余额 4.00 / 预占（进行中任务）0.20 / 退款锁定 0.00 / **可用 3.80 = 4.00−0.20−0.00**（06-held-breakdown.png）；验证后删除该行
- account API 原文：weknora-account-api.json（消费前）、weknora-account-api-final.json（终态），字段 `credits.{balance,held,refund_locked,available,projected_at}_micro + batches[].{source,period,granted_at,balance_micro,expires_at}` 全部十进制数字字符串

### ② 到期语义：月度归零不结转、充值保留（截图 04）

真时间跨月不可等（计划 known-limit：`api-clock` 为真实小时钟，`docker inspect` 无时钟偏移变量），按计划边界处理并以三面证据合围：

- **对象语义**（权威表达）：月度钱包 `expiration_at=2026-10-01T00:00:00Z`（月期末/下月 1 日 00:00Z）；充值 D `expiration_at=2027-09-28`（= 发放日 +12 个月）；C 为「老化充值」等效形状（2027-02-01）
- **到期终止等价模拟**：DELETE 月度钱包（= `TerminateWalletsJob` 到期动作等价，t03 实测同为 wallet terminated）→ 页面月度批次行显示 **0.00**（registry overlay 归零展示，`TestExpiredBatchSurfacesZero` 语义），总余额 9.00=4+5 **不含终止批次**（不结转），充值 C/D 行保留（04-monthly-expired-no-carryover.png；lago-after-terminate-monthly.txt）
- **本地 lot 不复活**（Task 3 真实链路）：刷新后 `commercial_budget_lots` 中 active 批次 remaining 与权威一致（C=4,000,000µ、D=5,000,000µ），**terminated 批次（含曾有 300c/500c 余额的 A/B）remaining 全部塌缩为 0**；`issued_at`=钱包 `created_at`（tie-break 列就位）
- 守护单测实跑：`TestExpiredBatchSurfacesZero`、`TestSyncLotsNeverResurrectsExpiredBatch`、`TestReserveSameExpiryPicksEarliestIssuedLot` 全部 ok
- Reload 后本期不重发月度（period 幂等），且 rebalance 再次收敛（M 终止后 C=1、D=2）

### ③ 消费顺序：月度（最早到期）先减，跨过月度后充值按到期先后减（截图 03）

消费触发器沿用 t03 lab 实测方法（billable_metric + plan + subscription + `POST /api/v1/events` → 发票从钱包按 `priority ASC, created_at ASC` 真实扣减——正是 #86 priority 编码/重排所服务的 Lago 原生消费路径）：

- 首跑（USD 种子）：消费 2.00 全部落在充值 A、月度未扣——**种子数据缺陷**（充值钱包币种用了计划示例的 USD，而 WeKnora `createWallet` 硬编码 `CurrencyCNY`（lago.go:996），USD 发票跳过 CNY 钱包）。删除 USD 钱包重建 CNY 种子（consume-leg1-run.txt 留档；**非产品缺陷**）
- 正式跑（consume-cny-run.txt / consume-cny.json）：消费 2.00 CNY → 3 秒结算，**月度 M（priority 1，到期 2026-10-01）100→0 先扣空；充值 C（priority 2，到期 2027-02-01）500→400 承接跨过月度的 1.00；充值 D（priority 3，到期 2027-09-28）不动** → ORDER VERDICT PASS
- 页面刷新同步：总余额 9.00、月度 0.00、C 4.00、D 5.00（03-after-consume-monthly-drained.png）

### ④ 管理员对照 Lago 控制台对账（截图 05 + reconcile ×2 PASS）

- 浏览器登录 Lago front（:48890，org `weknora-r5`）→ 该 customer wallets 视图：`topup-d Active CN¥5.00 5 credits`、`topup-c Active CN¥4.00 4 credits`、月度 `Terminated CN¥0.00 0 credits`、terminated 批次（A/B）不进页面（05-lago-console-wallets.png）
- `reconcile.py` 两轮（消费前 9.00 状态 + 终态 4.00 状态）全部 PASS（reconcile-output.txt / reconcile-final-output.txt）：① Σ active balance_cents×10⁴ == 页面 balance_micro；② 每 active 钱包与页面批次 1:1（余额/到期/发放时间）；③ 无 active 钱包对应的页面行余额必为 0（不结转/不复活）；④ available == balance − held − refund_locked
- 运行：`LAGO_API_KEY=$KEY python3 reconcile.py --output-dir "/tmp/weknora-86-reconcile-$(date +%s)"`（可用 `WK_ACCOUNT_JSON=` 换输入快照；目录必须是全新路径）

### ⑤ 并发消费无重复扣减（consumption-concurrent-run.txt + replay-idempotency.txt）

- **权威侧**：5 线程并发各发 1.00 消费事件 → 3 秒内结算：总额 900c→400c **恰好扣 500c**（无重复扣减/无超扣）、无负余额、扣减序 == 到期序（C 400→0 先扣空、D 500→400 承接 100）
- **重放幂等**：重放已消费的同一 `transaction_id` → HTTP 200（Lago 幂等去重）且余额分文不动（400 == 400）
- **WeKnora 本地 lot 面**（真实 Postgres WeKnora86 + sqlite）：`TestBudgetPGConcurrentReservation`、`TestBudgetPGTwoTasksCompeteForAccount`、`TestBudgetPGRefundLockCompetesWithReservation`、`TestSyncLotsConcurrentWithReserveNoOverAllocation`（`-tags commercial_integration` + `SAAS_TEST_PG_DSN`）4/4 PASS；sqlite `TestSyncLotsConcurrentSqlite` + `TestBudgetReserveConcurrent*` 3/3 PASS

## 脚本（本目录）

| 脚本 | 用途 | 运行 |
|---|---|---|
| `consume_86.py` | 单笔 2.00 跨月度消费（顺序断言） | `LAGO_API_KEY=$KEY python3 consume_86.py --output-dir "/tmp/weknora-86-consume-$(date +%s)"` |
| `concurrent_consumption_86.py` | 5 并发消费 + 重放幂等证据（无 pinned duplicate response contract 时 fail-closed） | `LAGO_API_KEY=$KEY python3 concurrent_consumption_86.py --output-dir "/tmp/weknora-86-concurrent-$(date +%s)"` |
| `reconcile.py` | WeKnora account API vs Lago wallets 两源对账 | `LAGO_API_KEY=$KEY python3 reconcile.py --output-dir "/tmp/weknora-86-reconcile-$(date +%s)"` |

每次运行都必须指定全新的 `--output-dir`；该目录不得已存在。以上路径直接位于已有的 `/tmp` 下。Reconciliation 同样要求非空 `LAGO_API_KEY`，缺失或为空时会在发出任何请求前停止；不读取 `.env`。所有示例使用 loopback Lago API，密钥仅通过环境变量提供。

消费触发器（metric/plan/subscription）按 t03 lab 形状创建，tag 前缀 `weknora-86-*`，Lago 侧留存（不影响 WeKnora commercial 面）。

## 已知边界与如实记录

1. **真时间跨月不可等**（计划 §6 known-limit）：月度批次「下一月访问」的真时间推进留待 #87/#88 admission 波次真实消费联验；本验证以对象语义（expires_at）+ 终止等价模拟 + 过期归零/不复活单测合围。
2. **USD 种子首跑失败**：验证脚本种子币种与 WeKnora 体系 CNY 不一致所致（计划示例文本照抄了 t03 的 USD 形状），重建 CNY 种子后通过；非产品缺陷。**建议**：#85 充值到账命令落地时沿用 `commercial.CurrencyCNY`（与 `createWallet` 一致），计划文档示例宜同步改 CNY。
3. **误发的 pending `paid_credits` 1.5 交易**（lago-consume-1p5.json）：流程③首驱动方式理解错误（paid_credits 是充值入账不是消费）；Lago 拒绝 void 未结算交易（422 `no_remaining_amount`，t03 已实证）。该交易无 payment provider 永不结算、不入余额，留存月度钱包交易列表（status pending）无害。
4. **本地 Reserve 的真实用户入口**（agent 付费任务）需要模型后端，本环境未配置；本地 lot 分配顺序以真实 SyncLots 落库（`commercial_budget_lots` 行序/issued_at）+ 顺序/并发单测（sqlite+PG）覆盖。`CommercialGateway.Settle` 当前仍为 openmeter legacy 装配（container.go:244），Lago wallet 消费交易的 WeKnora 出账命令属 #87/#88 范围。
5. 浏览器会话中两次 `fetch /api/v1/commercial/account` 401 为验证员用 stale localStorage token 的探测尝试（network #275/276），页面自身请求（#253/#266）均 200。

## 结论

原始 2026-09-28 真实环境运行记录及当时集成 HEAD 的结果属于历史证据，详见上文，不代表当前完整验收。当前集成检查点的 exact-hash replay/duplicate contract 与稳定 identity 仍未验证；因此 Issue #86 完整验收仍未通过。原始运行事实保留为历史记录，后续凭据处理更正见环境说明。
