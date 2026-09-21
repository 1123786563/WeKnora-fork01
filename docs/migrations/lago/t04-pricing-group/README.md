# Lago T04 — Task Pricing Group 可核对计价批次实验证据

Sanitized, reviewable evidence that Task/Pricing Group usage events against a
**real pinned Lago Community `v1.53.0`** stack form exactly reconcilable
pricing batches (GitHub ticket #76, Wave 2 of the Lago billing migration).
Captured on worktree `lago-76-pricing-group-lab` with the ticket-isolated
Compose instance `weknora-lago-76` (API `127.0.0.1:48895`, frontend
`127.0.0.1:48896`). Plan: `docs/plans/2026-09-21-lago-t04-pricing-group.md`;
spec: `docs/specs/2026-09-20-lago-billing-migration-design.md` (§Usage,
rating, and reconciliation; objective 11); ADR-0012.

## Task → Lago 对象映射（显式声明）

| WeKnora 概念 | Lago v1.53.0 对象 | 关联键 |
|---|---|---|
| Billing Account（空间） | 1 Customer，`external_id = weknora-t04-<uuid>`（每 run 一个合成 Customer） | `external_id` |
| **Task** | **1 个专属 Subscription**，调用方自选 `external_id = weknora-t04-<uuid>-task-<k>` | `external_subscription_id` |
| **Pricing Group** | 挂在该订阅上的不可变计价定义：1 plan（月付、base `amount_cents: 0`、CNY）+ 2 个 pay-in-arrears SUM charge（`*-model-units` standard `"7"` 分/单位；`*-tool-calls` package `"1500"` 分、`package_size 100`、`free_units 0`） | `external_subscription_id` |
| Settlement Batch（本实验范围） | 该订阅当前 billing period 的 `current_usage` 读数（每 charge `units`/`events_count`/`amount_cents` + 总额） | 与 fixture 全字段 exact 相等 |
| Usage Fact 的 transaction identity | 事件 `transaction_id` | DB 唯一索引 `(organization_id, external_subscription_id, transaction_id)` |

Rejected alternative（记录，不采纳）：把 task_id 放进 event properties 并用
charge filters 分组——v1.53.0 的 `current_usage` 无法在不枚举每个分组的情况下
产出 per-task 金额，也无法承载 Settlement Batch 边界；subscription-per-task
是唯一能形成"一个 Task 的全部事件聚合成一个可核对批次"的原生分组键。

Plan deviation（已在实现 ledger 记录）：plan 接口草图将 `*-tool-calls` 标注为
COUNT 聚合，但其绑定的参考数学（5 个事件共 250 units → ceil(250/100)=3 bundle
→ 4500 分；per-charge events_count 为 10/5）要求 250 units，COUNT 只能产出 5。
实现采用 SUM 聚合整型 `calls` 字段，plan 的精确期望值（7000/4500/11500 分）原样保留。

## 实测契约修正（v1.53.0 真实栈上验证，写回实现与测试）

本次真实运行发现并修正了 4 处"按新文档想象旧版本"的契约偏差，全部先实测
（422/404 → 200）再改代码，每处一个独立 commit：

1. **billable metric 聚合枚举是 `sum_agg`**，不是 `sum`（`aggregation_type:
   "sum"` → 422 `value_is_invalid`；`"sum_agg"` → 200）。
2. **plan charge 用 `billable_metric_id`（metric 的 lago_id）引用**，不是
   `billable_metric_code`（给 code → 404 `billable_metric_not_found`）。
3. **事件 `timestamp` 是 Unix epoch 秒**（int），ISO-8601 字符串（含
   `Z`/offset/微秒各变体）一律 422 `invalid_format`；读回时 Lago 返回 ISO。
4. **charge `properties.amount` 十进制串是货币单位，Lago 自行 ×100 折分**
   （units=100 × amount `"7"` → 70000 分）。要得到"每单位 7 分"须写 `"0.07"`；
   "每 bundle 1500 分"须写 `"15"`。fixture 期望仍为 integer cents，由同一常量
   除以 100 派生线上值，杜绝漂移。

## 每个工件证明什么、如何产生

| 文件 | 证明什么 | 如何产生 |
|---|---|---|
| `t04-run.txt` | 全程相位时间线（UTC 时间戳 + 关键事件行） | `run_phase.py` 各相位自动追加 |
| `t04-acceptance-vs-rating.json` | **验收①**：event acceptance 只代表接收——15 个事件全部 2xx 且逐个可 `GET` 读回；第 1 个事件接受后的即时读数（快照 A0，必然未计价）、最后接受后的快照；异步 settle 时间线到首次 exact 匹配 t1；负对照事件（合法 metric + 不存在订阅）被接收、可查询、但永不计价（settle 后 3 次读数恒等于 fixture 金额） | `LAGO_API_KEY=<key> python3 deploy/lago-lab/pricing-group/run_phase.py acceptance` |
| `t04-idempotency-conflict.json` | **验收②**：byte-identical 重发 → HTTP 422 `value_already_exist`；同 `transaction_id` 不同 properties 重发 → 同样显式 422，响应体与重试不可区分；`GET` 读回**原始** properties（未覆盖）；settle 后金额/events_count 与基线逐字段一致（未双计） | `... run_phase.py idempotency` |
| `t04-reconciliation.json` | **验收③**：reference task（15 事件）与全部 K=64 个 load task 的 `current_usage` 与 fixture **全字段精确相等**（每 charge `amount_cents`/`units`/`events_count` 与 `total_amount_cents`，integer cents，1 分漂移即 fail） | `... run_phase.py reconcile`（依赖 load 相位状态） |
| `t04-latency-throughput.json` | **验收④**：event-to-reconcilable-batch p50/p95/max 延迟（时间戳推导：每 task 最后事件 2xx 时刻 → 首次 exact 匹配时刻）、ingest 延迟分位数、发送窗口吞吐与峰值 events/sec、Pricing Group 基数（并发 task subscription 数）、p95 vs 60 s 判定 | `... run_phase.py reconcile`（聚合 load 相位记录） |
| `t04-load.json` | load 相位原始记录：K×20 事件经 batch 端点 8 并发投递的逐请求状态/时间 | `... run_phase.py load` |
| `t04-state.json` | run 的对象身份与相位间状态（RunSpec + 创建结果 + load 计时）。全部为 `weknora-t04-` 前缀合成 id，无任何 secret | `run_phase.py` setup/load/acceptance 写入 |

全部 JSON 由相位脚本生成（stdout 与文件同文），经 `dumps_sanitized` 处理；
HTTP 响应体原样保留（其本身不含 secret）。

## 判读 "acceptance ≠ rating"

三个互相独立的证据，任一成立即证明"接收"与"计价完成"是两个事件：

1. **快照 A0**：第 1 个事件 2xx 之后立即读取 `current_usage` —— 此时其余 14
   个事件尚未发送，该读数在物理上不可能是完整的计价批次（必然 reconcile fail）。
2. **异步 settle 时间线**：从最后事件 2xx 到首次 exact 匹配存在非零间隔
   （Sidekiq `PostProcessJob` 在 default 队列异步失效 charge 缓存），
   `event_to_reconcilable_latency_seconds` 即该间隔的实测值。
3. **负对照**：合法 metric code + 不存在的 `external_subscription_id` 的事件
   返回 2xx 且可 `GET` 查询，但任何 `current_usage` 永远不含它 —— "已接收"
   与"已被计价"是可以分离的状态。

任何"2xx 即已计价"的断言都是缺陷；本实验把 2xx、可查询、可精确核对三层
证据分开记录。

## 重复/冲突判读（含移交 #87/#88 的 findings）

- 重复投递不双计：byte-identical 重发得 HTTP 422
  `{"status":422,"code":"validation_errors","error_details":{"transaction_id":["value_already_exist"]}}`，
  且 settle 后 `events_count`/`amount_cents` 与基线完全一致。
- **Finding（移交 #87/#88）**：422 响应体对"同内容重试"与"同 id 不同内容冲突"
  **完全相同**（本实验实测两响应体逐字节相等）——冲突检测只能靠
  `GET /api/v1/events/{transaction_id}` 读回已存内容比对。
- **Finding（移交 #87/#88）**：唯一性作用域是
  `(organization_id, external_subscription_id, transaction_id)` 三元组，即同一
  `transaction_id` 在不同订阅下不冲突——跨 Task 复用 id 时必须以 Task 为作用域
  生成。

## blocked-env 语义

`LAGO_API_KEY` 缺失（或 setup/teardown 时 Docker 不可用）→ 相位输出
`{"status":"blocked-env",...}`、退出码 2、零 API 调用。blocked-env 是环境
缺口的诚实证据，不是合同通过；修复环境后重跑。setup 阶段的 API key 优先取
调用方环境 `LAGO_API_KEY`；首次全新栈可回退到它刚写入 gitignored
`deploy/lago/.env` 的 `LAGO_ORG_API_KEY` 种子值（T01 README 记载的种子路径，
仅驻留内存，不进日志/证据）。后续相位一律只认环境变量。

## 无 secret 契约

- `LAGO_API_KEY` 只从调用方环境注入；绝不写入任何被跟踪文件、日志或证据。
  `deploy/lago/.env`（含种子）是 gitignored、mode 600 的本机文件，T01 机制。
- 提交前对全部待提交文件 grep 真实 key 与 `.env` 值，零命中是门槛。
- 隔离前缀：一切 external id 以 `weknora-t04-<uuid>` 开头，每 run 新 uuid；
  remote 对象只存在于本 ticket 的隔离实例中，teardown 不强制清理（一次性数据，
  `lago.sh down` 保留 named volumes）。

## AGPL 提示

Lago 为 AGPL-3.0。本证据切片只证明运行时合同行为，**不构成法律批准**；AGPL
审查仍是生产门前置项，由 billing migration 的完成标准持有。

## 实测结果摘要（2026-09-20 run，run_id 见各工件）

- **验收①**：15/15 事件 2xx 且逐个读回内容一致；首事件接受后的快照 A0 仅含
  1 个事件（100 units/700 分）——物理上不可能是完整批次；末事件后快照 A1 已
  完整计价（本机 Sidekiq 极快）；settle 首读即 exact（进程内实测
  event→reconcilable 0.112 s）；负对照接收+可查询、settle 后 3 次读数恒 11500 分。
- **验收②**：byte-identical 重发 422 `value_already_exist`；冲突重发同样 422
  且**响应体逐字节相同**；读回原始 properties（units=100，非变体 301）；
  前后金额/计数零漂移（1000u/10e/7000 分 + 250u/5e/4500 分 = 11500 分）。
- **验收③**：reference task 11500 分精确匹配；64/64 load task 全部 exact
  （0 unresolved），覆盖 package ceil 边界（k=10 恰 100 calls→1 bundle）。
- **验收④**：latency p50=6.678 s / p95=6.893 s / max=6.917 s（64 任务，
  全部 resolved，p95 ≤ 60 s → **pass**）；ingest p50=0.127 s / p95=0.215 s；
  发送窗口 1.134 s 内 1280/1280 accepted → 1128.49 events/s（峰值单秒
  960）；Pricing Group 基数 = 65 个并发 task subscription（64 load + 1 reference）。

延迟方法学诚实说明：reconcile 相位的 per-task latency 定义是"该任务最后事件
2xx 时刻 → 首次 exact 匹配读数时刻"，两个时刻分别由 load 与 reconcile 两个
独立进程记录，因此该 latency 含相位间进程启动间隔（约 6.5 s），是真实
event-to-reconcilable 延迟的**上界**；acceptance 相位的进程内测量（15 事件
0.112 s settle）显示纯 rating 稳定时间在本机为亚秒级。两组数字均按定义如实
记录，不改判。

## 一次性实例残留说明

隔离实例（`weknora-lago-76`，volumes 保留）中存在本次实验过程的对象残留：
契约探测期间的 `weknora-t04-probe-*` metric/plan、timestamp 探测事件，以及两
次因契约修正而废弃的 run（`*-6f1bf338-*`、`*-0602b7bf-*`，各 65 订阅）。全部
为 `weknora-t04-` 前缀的一次性合成数据，仅存在于本 ticket 的隔离实例内，不影
响任何有效证据（有效 run 为 `*-8ed2a126-*`，见 `t04-state.json`）。

## 复现

```bash
./deploy/lago/lago.sh init                                  # 若 .env 不存在（setup 也会做）
python3 deploy/lago-lab/pricing-group/run_phase.py setup --k 64
export LAGO_API_KEY=$(grep '^LAGO_ORG_API_KEY=' deploy/lago/.env | tail -1 | cut -d= -f2)
python3 deploy/lago-lab/pricing-group/run_phase.py acceptance
python3 deploy/lago-lab/pricing-group/run_phase.py idempotency
python3 deploy/lago-lab/pricing-group/run_phase.py load
python3 deploy/lago-lab/pricing-group/run_phase.py reconcile
python3 deploy/lago-lab/pricing-group/run_phase.py teardown
```

静态检查（Docker-free）：`python3 -m unittest discover -s deploy/lago-lab/pricing-group -p 'test_*.py' -v`。

## 验收标准 ↔ 工件映射

| 验收标准（ticket #76） | 相位 | 工件 |
|---|---|---|
| ① event acceptance 只代表接收，不当作 rating 完成 | acceptance | `t04-acceptance-vs-rating.json`（快照 A0/settle 时间线/负对照） |
| ② 重复事件不重复计量；相同身份不同内容被显式拒绝 | idempotency | `t04-idempotency-conflict.json`（422×2、读回原始内容、零漂移） |
| ③ current usage 金额与固定 fixture 精确核对 | reconcile（+Task 1 复算单测） | `t04-reconciliation.json`（逐 charge/总额 exact） |
| ④ p50/p95 延迟、峰值吞吐、Pricing Group 基数 | load + reconcile | `t04-latency-throughput.json`（含 p95 vs 60 s 判定） |
