# Lago T04 — Task Pricing Group 可核对计价批次实验 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** 向本 worktree 自己的真实 Lago Community `v1.53.0` 投递带 Task/Pricing Group 身份的 Usage Event，证明四件事：event acceptance 只代表接收（不是 rating 完成）；重复事件不重复计量、相同身份不同内容被显式拒绝；Task/Pricing Group 的 current usage 金额可与固定 fixture 精确核对（integer cents）；并记录 p50/p95 延迟、峰值吞吐与 Pricing Group 基数。

**Architecture:** 纯证据切片（evidence-only lab），不改产品代码。`deploy/lago-lab/pricing-group/` 存放 fixture、Lago 客户端、相位脚本与 Docker-free 单测；真实相位通过 T01 的 `./deploy/lago/lago.sh`（只读复用）驱动本 ticket 专属 Compose 实例。证据写入 `docs/migrations/lago/t04-pricing-group/`。Settlement Batch 的 WeKnora 侧建模按下述 **Task → Lago 对象映射** 显式声明，映射依据是 spec §Usage/rating 与 ADR-0012。

**Tech Stack:** Python 3 标准库（unittest + `http.server` 假 Lago）、Bash、Docker Compose v2、Lago Community `v1.53.0`（digest 已锁，T01 产出）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（§Usage, rating, and reconciliation；§Testing Decisions——contract test 记录 pinned release、请求、响应、已知限制）；验收来源：GitHub Issue #76（blocked by #73，已合并）；父票 #72；ADR-0012。

## Task → Lago 对象映射（本实验的显式声明）

| WeKnora 概念（CONTEXT.md） | Lago v1.53.0 对象 | 说明 |
|---|---|---|
| Billing Account（空间） | 1 个 Customer，`external_id = weknora-t04-<uuid>` | 每 run 一个合成 Customer |
| **Task** | **1 个专属 Subscription**，调用方自选 `external_id = weknora-t04-<uuid>-task-<k>` | v1.53.0 订阅创建接受 caller-supplied `external_id`（`subscriptions_controller.rb` create_params 含 `:external_id`） |
| **Pricing Group** | 挂在该订阅上的不可变计价定义：1 个 plan（月付、base `amount_cents: 0`、CNY）+ 2 个 pay-in-arrears charge（2 个 billable metric） | group 关联键 = `external_subscription_id` |
| Settlement Batch（实验范围） | 该订阅当前 billing period 的 `current_usage` 读数（每 charge 的 units/events_count/amount_cents + 总额） | 可核对 = 与 fixture 期望 exact 相等 |
| Usage Fact 的 Lago transaction identity | event 的 **`transaction_id`** | 见下方契约事实：v1.53.0 幂等字段名是 `transaction_id`，不是旧文档的 `external_id` |

**为什么 Task→专属订阅**：spec 要求"按 Task／结算批次和 Pricing Group 读取可关联的权威聚合计价结果，yield 一个 determinate authoritative amount"。v1.53.0 的 `current_usage` 只能按 (customer, subscription) 聚合出"每 charge 一个确定金额"；properties/charge filters 无法在不枚举每个分组的情况下产出 per-task 金额，也无法承载 Settlement Batch 边界。因此 subscription-per-task 是唯一能形成"一个 Task 的全部事件聚合成一个可核对批次"的原生分组键，且与 ADR-0012"按 Task／结算批次……读取权威聚合计价结果"一致。备选（task_id 放 properties + filters）在实验报告中记录为 rejected alternative 及理由。

## Pinned-release 契约事实（2026-09-21 源码核验，getlago/lago-api @ 591ae900）

实现前必须按这些事实编码；若真实运行观测与之一致，写入证据；不一致则作为 finding 记录，不得静默改判。

1. **事件幂等字段是 `transaction_id`**（`events_controller.rb` create_params 只 permit `transaction_id, code, timestamp, external_subscription_id, precise_total_amount_cents, properties`）。Ticket 行文中的 "external_id" 是旧称。
2. **重复事件 → HTTP 422，非 409、非静默丢弃**。DB 唯一索引 `index_unique_transaction_id` 建在 `(organization_id, external_subscription_id, transaction_id)`；`Events::CreateService` 捕获 `ActiveRecord::RecordNotUnique` → `single_validation_failure!(field: :transaction_id, error_code: "value_already_exist")` → `ApiErrors#validation_errors` 渲染 422 `{"status":422,"code":"validation_errors","error_details":{"transaction_id":["value_already_exist"]}}`（request spec 断言 `:unprocessable_content`）。**响应不区分"同内容重试"与"不同内容冲突"**——冲突检测只能靠 `GET /api/v1/events/{transaction_id}` 读回已存内容比对。
3. **acceptance ≠ rating**。`POST /api/v1/events` 同步只做 `save!` + `PostProcessJob.perform_later`（Sidekiq），随后立即返回（request spec 断言 `:ok`，即 HTTP 200；ticket 里的 "202" 按实际观测记录）。charge-cache 失效、pay-in-advance fee、wallet 刷新全部异步。未知 `code` 或无匹配 active subscription 的事件**仍被接收并可查询，但永不计价**——这是 determinate 的"接收但未计价"负对照。Sidekiq `config/sidekiq/sidekiq.yml` 无 `events` 队列 → `PostProcessJob` 落 `default` 队列（concurrency 10）；事件→current_usage 可见的时延由该队列决定，必须实测。
4. **v1.53.0 的 current_usage 是 customer 作用域**：`GET /api/v1/customers/{external_customer_id}/current_usage?external_subscription_id=<task-sub>`（route：`get :current_usage, to: "customers/usage#current"`；`Invoices::CustomerUsageService.with_external_ids` 要求该订阅 **active**）。响应根 `customer_usage`：`from_datetime/to_datetime/issuing_date/currency/amount_cents/total_amount_cents/taxes_amount_cents` + `charges_usage[]`，每项含 `units`（decimal **string**）、`events_count`、`amount_cents`、`charge{charge_model}`、`billable_metric`。新 ticket 提示里的 `/subscriptions/{id}/current_usage` 是更新版 API 形态，本 release 不适用。
5. **精确金额**：charge `properties.amount` 必须是 decimal **string**（`Validators::DecimalAmountService` 直接拒绝非 string/float，BigDecimal 计算）。`standard`（per-unit）= `units × BigDecimal(amount)`；`package` = `ceil((units − free_units) / package_size) × amount`。integer units + integer-cents amount ⇒ `amount_cents` 是精确整数，无浮点。
6. 事件可查询：`GET /api/v1/events/{transaction_id}` 与 `GET /api/v1/events?external_subscription_id=&code=`（分页）；批量投递 `POST /api/v1/events/batch`（≤100/请求）。
7. 自托管 v1.53.0 源码中未发现 events API 的 rate limiter（仅有 payments 相关）；吞吐上限来自 PostgreSQL + Sidekiq，须实测而非假设。

## Global Constraints

- 证据切片：不写产品代码、不加 migration、不建 Commercial Platform seam（那是 #77）、不触碰 OpenMeter。`deploy/lago/` 内**所有已跟踪文件只读**；本实验产物只进 `deploy/lago-lab/pricing-group/` 与 `docs/migrations/lago/t04-pricing-group/`。
- 本 ticket 专属实例：通过 T01 README 的 worktree 覆盖机制写**未跟踪**的 `deploy/lago/.env`（`COMPOSE_PROJECT_NAME=weknora-lago-76`、`LAGO_API_PORT=48895`、`LAGO_FRONT_PORT=48896`，同步改 `*_URL`，追加 `LAGO_CREATE_ORG=true` + `LAGO_ORG_*` 种子值），再 `./deploy/lago/lago.sh up`。事件存储 PostgreSQL；不引入 ClickHouse/Kafka。
- `LAGO_API_KEY` 只从调用方环境读取，绝不写入文件/日志/证据；单测里也只允许 `secret-for-test-only` 这类测试值。证据提交前对全树 grep 真实 secret，零命中。
- 隔离前缀：一切 external id（customer、plan/metric code、subscription external_id、transaction_id）以 `weknora-t04-<uuid>` 开头，每 run 一个新 uuid；remote 对象只存在于本 ticket 的隔离实例中。
- 金额只用 integer minor units：Python 侧 int/`decimal.Decimal`，绝不 float；对 Lago 的金额断言是 `amount_cents`/`total_amount_cents` 的**精确整数相等**；`units` 按字符串→Decimal 比较。
- acceptance 语义：任何 2xx + event 资源 = "已接收"；"可核对"只由 current_usage 与 fixture 全字段（每 charge 的 amount_cents、units、events_count 及总额）exact 匹配证明。poll 协议：每 2 s 读一次，上限 300 s（对齐 spec 的 5 分钟 Task 暂停阈值）；超时记 `unresolved`，是 finding 不是失败静默。
- 延迟与吞吐必须来自记录的时间戳：每任务 latency = "该任务最后一个事件 2xx 时刻 → 第一次 exact 匹配读数时刻"（event-to-reconcilable-batch）；ingest 延迟单独记录；p95 与 spec 的 ≤60 s 目标显式比对并给出 verdict（超标是 blocker 证据，不是 pass）。
- 单测 Docker-free：`python3 -m unittest discover -s deploy/lago-lab/pricing-group -p 'test_*.py' -v`（目录名含连字符，不能用 dotted module 路径；测试直接 import 同目录模块）。真实相位全部脚本化、每个命令写明 expected outcome；Docker/API key 缺失 → `blocked-env`，绝不冒充通过。真实相位可错峰执行。
- 收尾 `./deploy/lago/lago.sh down`（保留 named volumes，不用 `down -v`）；remote 对象不做强制清理（隔离实例内的一次性数据），证据需说明这一点。

## Review Focus

- **把 acceptance 当 rating**：2xx 后立即读 current_usage 的快照必须入证据；外加"无订阅负对照"事件（被接收、可查询、永不计价）。任何"已收到≈已计价"的断言都是缺陷。
- **重复事件双计**：同 payload 重发必须 422 且 settle 后 `events_count`/`amount_cents` 不变；比对必须发生在 reconcile 之后，绝不能拿 PostProcessJob 未跑完的缓存读数当"未双计"证据。
- **同身份不同内容被静默吞掉/覆盖**：冲突重发必须是非 2xx 显式错误，且 `GET /api/v1/events/{transaction_id}` 读回**原始**内容；若出现 2xx 或内容被覆盖，判 FAIL 并升格为迁移 blocker 记录。
- **fixture 金额对不上**：package 的 ceil、standard 的乘法在 Python 侧用 int/Decimal 复算并单测；comparator 要求每 charge 与总额逐分相等，1 分漂移即 FAIL。
- **延迟/吞吐没测或测错对象**：所有 p50/p95 由时间戳推导（不是事后猜），事件可见性等待计入 event-to-reconcilable 延迟；峰值吞吐 = 发送窗口内 accepted events/sec；Pricing Group 基数 = 实际并发 task subscription 数，都写入证据并与 spec 目标比对。

---

### Task 1: 确定性 Pricing Group fixture 与精确金额期望模型

**Files:**
- Create: `deploy/lago-lab/pricing-group/fixture.py`
- Test: `deploy/lago-lab/pricing-group/test_fixture.py`

**Interfaces:**
- Consumes: 上方"Pinned-release 契约事实"第 4/5 条的计价语义。
- Produces: `build_run(run_id, task_count) -> RunSpec`（Customer、2 个 billable metric（SUM `units` 的 `*-model-units`；COUNT 的 `*-tool-calls`）、plan（月付、base 0、CNY、charge：standard amount `"7"` 分/单位；package amount `"1500"`、`package_size 100`、`free_units 0`）、tasks[]（每 task 订阅 external_id、事件表：transaction_id、code、timestamp、integer properties））；`expected_usage(task) -> ExpectedUsage`（每 charge 的 `units`、`events_count`、`amount_cents` 与 `total_amount_cents`，纯 int/Decimal）。

- [ ] **Step 1: 写失败的行为测试**

用 `unittest` 覆盖：参考任务（10 个事件共 1000 units + 5 个事件共 250 tool calls）期望 `model-units` charge = 7000 分、`tool-calls` charge = ceil(250/100)=3 bundle × 1500 = 4500 分、总额 11500 分、`events_count` 15/按 charge 10 与 5；package ceil 边界（恰好 100、101、0 units）；所有 external id 带 `weknora-t04-` 前缀且 run 内唯一；`json.dumps(RunSpec)` 序列化结果中不出现任何 float 字面量（`json.loads` 后递归断言类型）；timestamp 用固定单调基准使 billing period 确定且落在当前 period。

```python
def test_reference_task_expected_amount_is_exact_integer_cents():
    run = build_run("weknora-t04-deadbeef", task_count=2)
    exp = expected_usage(run.tasks[0])
    assert exp.charge("model-units").amount_cents == 7000
    assert exp.charge("tool-calls").amount_cents == 4500
    assert exp.total_amount_cents == 11500
```

- [ ] **Step 2: 跑测试确认 RED**

Run: `python3 -m unittest discover -s deploy/lago-lab/pricing-group -p 'test_*.py' -v`

Expected: 因 fixture 模块不存在而失败。

- [ ] **Step 3: 实现最小 fixture 模块**

复算逻辑的 docstring 注明出处（`app/services/charge_models/standard_service.rb`、`package_service.rb`、`Validators::DecimalAmountService`，getlago/lago-api @ 591ae90）。金额常量写成 decimal string（发给 Lago）+ int（本地期望），两处由同一常量派生，防止漂移。附 `negative_control_event(run_id)`：code 用合法 metric、`external_subscription_id` 指向不存在的订阅。附 `conflicting_variant(event)`：同 transaction_id、不同 integer properties。

- [ ] **Step 4: 跑聚焦测试确认 GREEN**

Run: `python3 -m unittest discover -s deploy/lago-lab/pricing-group -p 'test_*.py' -v` — Expected: 全绿。

- [ ] **Step 5: 提交 fixture 切片**

Commit: `feat(lago-t04): pricing-group fixture with exact integer-cents expectations`（只含本任务文件）。

### Task 2: Lago 实验客户端——投递、读回、分类、核对

**Files:**
- Create: `deploy/lago-lab/pricing-group/lago_client.py`
- Test: `deploy/lago-lab/pricing-group/test_lago_client.py`

**Interfaces:**
- Consumes: Task 1 的 RunSpec/ExpectedUsage；T01 `lago.sh` 提供的 API base（`http://127.0.0.1:48895`）与 `LAGO_API_KEY`。
- Produces: `send_event(base, key, event)`、`send_batch(...)`（≤100/请求）、`get_event(transaction_id)`、`list_events(external_subscription_id, code)`、`get_current_usage(customer_external_id, external_subscription_id)`（调用 v1.53.0 customer 作用域端点）、`classify_send(status, body)` → `accepted | duplicate_transaction_id | validation_error | unexpected`、`reconcile(observed_customer_usage, expected) -> ReconcileReport`（每 charge 与总额 exact 比较；`units` 字符串按 Decimal 比较；任何字段不符即 fail 并列出差异）、`poll_until_reconciled(..., interval=2, timeout=300)`、`report(...)` 序列化自动 redact Authorization/API key；无 `LAGO_API_KEY` → `blocked-env` 且零 API 调用。

- [ ] **Step 1: 写失败的行为测试（in-process `http.server` 假 Lago）**

沿用 `deploy/lago/test_contract_probe.py` 的假服务器模式。断言：请求只发往配置 origin 且带 Bearer；`send_event` POST `/api/v1/events`（body 含 `transaction_id`）；重复场景（422 + `error_details.transaction_id=["value_already_exist"]`）被分类为 `duplicate_transaction_id` 而非 accepted；`get_event` 走 `/api/v1/events/{transaction_id}`；`get_current_usage` 请求路径与 query 是 `/api/v1/customers/{id}/current_usage?external_subscription_id=...`；`reconcile` 对 1 分差异、units 差异、events_count 差异、缺 charge 都判 fail；序列化报告中不出现 API key。

```python
def test_one_cent_mismatch_fails_reconciliation():
    observed = customer_usage(amount_cents=11501, charges=[...])  # 期望 11500
    report = reconcile(observed, expected_usage(reference_task))
    assert report.ok is False and "amount_cents" in report.diffs
```

- [ ] **Step 2: 跑测试确认 RED** — 同 Task 1 命令，因客户端不存在而失败。

- [ ] **Step 3: 实现最小客户端与核对器**

编码前对照"契约事实"复核端点/字段（transaction_id、422 形态、customer 作用域 current_usage、charges_usage 的 units 是 string）；若真实现与事实冲突，先记 ledger 再改事实表。所有响应体原样（redact 后）进证据；分类器只依据 status + `code`/`error_details`，不做内容猜测。

- [ ] **Step 4: 跑聚焦测试确认 GREEN** — 同命令；成功、重复、冲突、核对失败、redaction、blocked-env 全绿。

- [ ] **Step 5: 提交客户端切片**

Commit: `feat(lago-t04): lago lab client with duplicate classification and exact reconciliation`。

### Task 3: 相位脚本——真实栈上的编排与测量

**Files:**
- Create: `deploy/lago-lab/pricing-group/measure.py`
- Create: `deploy/lago-lab/pricing-group/run_phase.py`
- Test: `deploy/lago-lab/pricing-group/test_measure.py`（Docker-free）

**Interfaces:**
- Consumes: Task 1 fixture、Task 2 客户端、`./deploy/lago/lago.sh`（init/up/status/down 只读复用）、T01 README 的 worktree `.env` 覆盖与 `LAGO_CREATE_ORG` 种子路径。
- Produces: `run_phase.py setup|acceptance|idempotency|load|reconcile|teardown`，每相位输出一个 sanitized JSON 到 `docs/migrations/lago/t04-pricing-group/`；`measure.py` 提供 `percentiles(samples) -> {p50, p95, max}` 与吞吐窗口计算（纯函数）。

- [ ] **Step 1: 写失败的测量与相位判定测试**

`test_measure.py`：p50/p95 在偶数/奇数样本、单样本上的正确值；吞吐 = accepted events / 发送窗口时长；unresolved 任务不得计入 latency 分母但必须单独计数。

- [ ] **Step 2: 跑测试确认 RED** — 同 Task 1 命令。

- [ ] **Step 3: 实现相位脚本（每相位写明 expected outcome）**

`setup`：若 `deploy/lago/.env` 不存在则 `lago.sh init`，随后改写 project/ports/URL（weknora-lago-76 / 48895 / 48896）并追加种子值（不改任何已跟踪文件），`up` + `status --json` 达 `ready`；按 fixture 创建 metrics、plan、customer、K+1 个订阅（K 个 load task + 1 个 reference task）。Expected: 全部 2xx；status ready。
`acceptance`：reference task 发 15 个事件 → 每个 2xx 记录时间戳；**立即**（a）`get_event` 逐个读回（可查询）、（b）`get_current_usage` 快照 A（预期尚未含新事件或仅部分——如实记录）；随后 poll 至 exact 匹配记 t1。另发 negative-control 事件：预期 2xx + 可查询 + 任何 current_usage 永不包含它。Expected: 快照 A、settle 时间线、负对照三者构成"acceptance ≠ rating"证据。
`idempotency`：settle 后取基线读数；byte-identical 重发 reference task 的 1 个事件 → 预期 422 `value_already_exist`；`conflicting_variant` 重发 → 预期同样显式 422，且 `get_event` 读回**原始** properties；再 settle 并 reconcile → 预期金额/events_count 与基线完全一致（未双计、未覆盖）。任何 2xx-on-conflict 或内容被覆盖 → 相位 FAIL。
`load`：K=64 个 task 订阅（可配置，≥32 才有效），每 task 20 事件经 batch 端点（8 并发）发送；记录 accepted events/sec（峰值）与每 task 最后事件 2xx 时刻。Expected: 全部 2xx（非 2xx 如实记录）。
`reconcile`：对全部 K task 并行 poll（2 s 间隔、300 s 上限），逐 task exact 匹配 fixture；汇总 p50/p95/max latency、unresolved 数、ingest 延迟分位数、K（Pricing Group 基数）、p95 vs 60 s verdict。Expected: 全部 task 匹配；若有 unresolved，如实输出并标注 5 分钟阈值 finding。
`teardown`：`lago.sh down`。Expected: 容器停止、volumes 保留。
所有相位：`LAGO_API_KEY` 缺失或 Docker 不可用 → 退出码 2 + `blocked-env` JSON，零 API 调用。

- [ ] **Step 4: 跑聚焦测试确认 GREEN** — `python3 -m unittest discover -s deploy/lago-lab/pricing-group -p 'test_*.py' -v`。

- [ ] **Step 5: 提交相位脚本切片**

Commit: `feat(lago-t04): phase runner and measurement for pricing-group lab`。

### Task 4: 真实运行、证据与验证收口

**Files:**
- Create: `docs/migrations/lago/t04-pricing-group/README.md`
- Create（由真实相位生成）: `t04-run.txt`、`t04-acceptance-vs-rating.json`、`t04-idempotency-conflict.json`、`t04-reconciliation.json`、`t04-latency-throughput.json`
- Test: 全部三个 test 文件复跑。

**Interfaces:**
- Consumes: Task 3 的完整相位流（setup → acceptance → idempotency → load → reconcile → teardown）。
- Produces: 可审阅的 sanitized 证据集 + 验收标准↔工件映射表。

- [ ] **Step 1: 补证据契约 README**

写明：目的与 Task→Lago 映射（引用本 plan）；每个 JSON 工件"证明什么、如何产生"；blocked-env 语义；无 secret 契约（提交前 grep）；AGPL 提示（证据非法律批准）；"acceptance ≠ rating"结论的判读方式；对 #87/#88 的两个 handed-off findings（422 不区分重试与冲突、唯一性按 (org, subscription, transaction_id) 三元组作用域）。

- [ ] **Step 2: 静态检查**

Run: `python3 -m unittest discover -s deploy/lago-lab/pricing-group -p 'test_*.py' -v`

Expected: 全绿（无 Docker 依赖）。

- [ ] **Step 3: 错峰执行真实相位（Expected outcomes 见 Task 3）**

Run: 逐相位执行 `LAGO_API_KEY=<operator key> python3 deploy/lago-lab/pricing-group/run_phase.py setup|acceptance|idempotency|load|reconcile`（key 只从环境注入）。

Expected: acceptance 相位产出"2xx 即接收 + 快照 A 未计价 + 负对照永不计价 + settle 时间线"；idempotency 相位 422×2 且金额零漂移；reconcile 相位 K 个 task 全部 exact 匹配；latency/throughput JSON 含 p50/p95、峰值 events/sec、K、p95 vs 60 s verdict。任一相位观测与契约事实不符 → 记 finding，不得改判 pass。

- [ ] **Step 4: 停栈并做证据卫生检查**

Run: `python3 deploy/lago-lab/pricing-group/run_phase.py teardown`

Run: 对即将提交的文件 grep `.env` 值与 `LAGO_API_KEY` 值 — Expected: 零命中；volumes 保留。

- [ ] **Step 5: 复跑单测、写验收映射表并提交**

验收标准 → 步骤/工件：①acceptance≠rating → Task 3 `acceptance` + `t04-acceptance-vs-rating.json`；②重复不双计/冲突显式 → Task 3 `idempotency` + `t04-idempotency-conflict.json`；③fixture 精确核对 → Task 1/3 + `t04-reconciliation.json`；④p50/p95、峰值吞吐、基数 → Task 3 + `t04-latency-throughput.json`。

Commit: `docs(lago-t04): real pricing-group lab evidence`。

## Plan Self-Review

- **Spec coverage：** 四条验收标准全部映射到 Task 3 的对应相位与 Task 4 的工件表。Settlement Batch 的"接收≠计价、聚合可关联、确定金额"三条 spec 语义分别由负对照+快照 A、Task→subscription 映射、exact reconciliation 覆盖。超出 T04 的产品行为（seam、预占、Outbox）一律未纳入。
- **执行细节扫描：** 每个任务都有文件、命令、expected result；真实相位全部脚本化且声明预期结果与 blocked-env 语义；计价复算公式标注 v1.53.0 源码出处，避免"照新文档实现旧版本"的坑（transaction_id 命名、customer 作用域 current_usage、422 形态、amount 必须 string）。
- **接口一致性：** Task 1 的 RunSpec/ExpectedUsage 被 Task 2 的 reconcile 与 Task 3 的所有相位消费；Task 2 客户端被 Task 3 消费；Task 4 消费全部相位输出。无跨 ticket 文件写入；`deploy/lago/` 只读复用。
- **Review focus 对应：** 五个风险各有一个专门的检测：快照 A + 负对照（acceptance 误判）、settle 后基线比对（双计）、读回原始内容（静默冲突）、int/Decimal 复算 + 逐分 comparator（金额漂移）、时间戳驱动的 percentiles 与 verdict（延迟未测/测错）。
- **TDD：** 三个可单测模块（fixture、客户端/核对器、测量）都是 RED→GREEN→commit；真实栈验证独立于单测证据，观测即证据，不回填单测。
