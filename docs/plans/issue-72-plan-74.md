# [Lago 02] 外部付款激活 payment-gated Subscription — 运行时证据补齐实施计划（Issue #74）

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在本 worktree 用用户已提供的 Stripe TEST 密钥重跑 `deploy/lago-lab/payment-activation/run_lab.py`，把 #74 的 AC1–AC3（及 AC4 的运行时 403）从 `blocked-env` 转为真实 pass/fail 证据，晋升证据到 `docs/migrations/lago/t02-payment-activation/` 并更新 DECISION.md 裁决，解锁 #81/#82 主链。

**Architecture:** 本 Issue 是实验型 Ticket，全部实验设施（9 阶段 runner、隔离 Compose 栈、60 个离线测试）已在集成分支交付并实测绿；本计划**不写任何新的生产代码**（Go/TS/Python 均零改动），只执行：陈旧卷重置 → 隔离栈拉起 → 带 Stripe TEST key 的真实运行（含 DB 侧投影观察者）→ 证据晋升 → 文档更新 → 密钥纪律终检。商业语义上，Lago Community v1.53.0（#73 digest 锁定）是被测权威，Stripe TEST 模式是外部付款接入的执行轨道，实验结论通过 DECISION.md 交给 #81/#82。

**Tech Stack:** Docker Compose v2（隔离项目 `weknora-lago-74`，回环端口 48891/48892）、Python 3 stdlib（runner 与阶段脚本）、Stripe TEST-mode REST（`sk_test_` 密钥，仅经环境变量注入）、Lago REST + GraphQL API、psql（容器内只读投影核验）、pytest（60 个离线回归）。

**Spec:** `docs/specs/2026-09-20-lago-billing-migration-design.md`（已批准，2026-09-20）；ADR：`docs/adr/0012-lago-as-commercial-billing-authority.md`；领域术语：`CONTEXT.md`「空间与商业归属」章（Billing Account / Channel Payment Order / Payment Fact / Fulfillment）。Issue：https://github.com/1123786563/WeKnora-fork01/issues/74（4 条验收标准，见下）。

---

## 调查结论（计划前提，2026-09-23 于本 worktree 实测）

以下事实全部在本 worktree（分支 `codex/issue-72-lago-74`，HEAD = `dafba8851`，工作区干净）核实：

1. **代码已交付**：`deploy/lago-lab/payment-activation/` 下 `phases.py`（9 个阶段函数，`phases.py:225-966`）、`run_lab.py`（runner，`run_lab.py:355`）、`lab.sh`（init/up/down/status/config，`lab.sh:152-162`）、`clients.py`、`fixtures.py`、`test_lab.py`、`test_phases.py` 全部在基线中。
2. **60 个离线测试实测绿**：`cd deploy/lago-lab/payment-activation && python3 -m pytest test_lab.py test_phases.py -q` → `60 passed in 37.20s`（本会话实跑）。
3. **当前证据状态 = RED（blocked-env）**：`evidence/` 下 7 个阶段报告（provider/gating/manual/activation/duplicates/retries/decline）均为 `"status": "blocked-env"`，原因 `"missing Stripe test key (set STRIPE_SECRET_KEY, sk_test_/rk_test_ only)"`；`t02-environment.json` 记录 `stripe.test_mode_key_present: false`、`run.overall: "blocked-env"`、`secrets_scan.clean: true`。
4. **Stripe TEST 密钥已到位**：`~/.zcode/issue72-stripe.env` 存在，仅含变量 `STRIPE_SECRET_KEY`，前缀 `sk_test_`（已验证前缀，未读取/未打印密钥值）。`run_lab.py` 只接受 `STRIPE_TEST_SECRET_KEY` 或 `STRIPE_SECRET_KEY` 且必须 `sk_test_`/`rk_test_` 前缀，live key 一律拒绝（`run_lab.py:101-113`，`clients.py:55`）。
5. **解锁命令（Issue #74 唯一评论原文，经 `gh issue view 74 --comments` 取回）**：
   ```bash
   cd .worktrees/lago-74 && ./deploy/lago-lab/payment-activation/lab.sh up
   STRIPE_SECRET_KEY=<sk_test_...> ./deploy/lago-lab/payment-activation/run_lab.py --output-dir deploy/lago-lab/payment-activation/evidence
   ./deploy/lago-lab/payment-activation/lab.sh down
   ```
   注意：旧 worktree `.worktrees/lago-74` 已删除（`ls .worktrees/` 无此目录），命令在本 worktree 执行。
6. **关键环境风险（必须处理）**：宿主上残留 `weknora-lago-74_lago_postgres_data`、`_lago_redis_data`、`_lago_storage_data` 三个旧卷（`docker volume ls` 实测），而与之配对的旧 `lab.env`（一次性种子凭据）已随旧 worktree 消失。若不先清卷，新 `lab.sh init` 生成的新种子无法注入已初始化的 postgres 卷 → GraphQL 登录失败 → 全部阶段退化为 blocked-env。**Task 1 必须先清卷**（README.md:50-52 已给出 `down -v` 的文档化擦除路径）。
7. **端口可用**：`lsof -nP -iTCP:48891 -iTCP:48892 -sTCP:LISTEN` 为空（两端口空闲）；`weknora-lago-74` 项目当前无运行容器；#73 主栈（`weknora-lago`，48889/48890）与其余开发容器健康运行中，不得触碰。
8. **既有旁支文件（不处理、只上报）**：`clients.worktree-variant.py` 是被跟踪的历史旁支（提交 `5e0c958e6` 引入，与 `clients.py` 有 4 处差异：多 `import stat`、缺「拒绝覆盖既有 lab.env」守卫、多 `"Stripe-Account": ""` 头、DELETE 带 params 的 selector 处理）。`run_lab.py` 导入的是 `clients`（即 `clients.py`），本票不改不删该旁支文件，仅在 Ledger 上报主 Agent。
9. **晋升惯例（cmp 实测）**：`docs/migrations/lago/t02-payment-activation/` 当前保存 7 个 AC/环境 JSON + `t02-run.txt`（与 deploy 侧 byte-identical 的 JSON + 操作者叙述版 timeline）；`t02-setup.json`、`t02-provider.json`、`t02-health.json` 只留在 deploy 侧 evidence（README.md:44-47 明示）。
10. **AC4 已交付**：`docs/migrations/lago/t02-payment-activation/DECISION.md`（§2 manual Premium 门控源码级验证、§4 blocker、§5 三选项推荐 (a)）；本计划补其 §2 中「运行时 403 待补」一项。

## Issue #74 验收标准（原文）

| # | 验收标准（Issue 原文） |
|---|---|
| AC1 | 付款前 Subscription 保持 incomplete，Entitlement 不可使用 |
| AC2 | 可信付款登记后 Subscription 变为 active，且重复登记不重复激活 |
| AC3 | 响应丢失、超时与重试使用同一商业身份并可恢复 |
| AC4 | 若 manual Payment 不能激活，证据明确给出受支持替代路径或 blocker |

阶段 → AC 映射（lab README.md:67-77 + DECISION.md §6）：`gate`→AC1；`activate`+`duplicates`→AC2；`retries`→AC3；`manual`+`decline_control`+DECISION.md→AC4。

## Global Constraints（批准需求原文引用；每个 Task 隐含继承）

1. **外部付款激活语义**：spec「Quotes, payments, and fulfillment」节原文 ——"A supported Lago external-payment integration records the Payment. Only a Lago Subscription observed as active enables Entitlement and fulfillment."（本票 AC2 判据即此句。）
2. **manual 不可用时的路径**：spec 同节原文 ——"If Lago manual Payment does not release the activation rule in the pinned Community version, the implementation must use a Lago-supported external Payment Provider adapter. It may not force the Subscription active locally."（禁止本地强置 active / 直写数据库伪造状态。）
3. **Premium 出界**：spec「Out of Scope」原文 ——"Lago Premium features, including Premium Invoice Preview or Subscription Overrides."→ `LAGO_LICENSE` 必须为空（`lab.sh:68` 非空即 die）。
4. **pinned release**：spec「Security, privacy, licensing, and lifecycle」原文 ——"Local and CI environments use a pinned Lago Compose release."→ 只用 `deploy/lago/images.lock.json` 锁定的 v1.53.0 五镜像 digest；`t02-environment.json` 的 `release.release` 必须为 `"v1.53.0"`。
5. **完成证据门槛**：spec「Completion gate」原文 ——"Mock success, an OpenAPI schema, an event-ingestion response, or a healthy Lago API process is not sufficient completion evidence."→ AC 判据只认真实运行阶段 `pass` + DB 侧观察，blocked-env/fail 不得宣称通过。
6. **凭据纪律**：spec 原文 ——"Lago API credentials live only in server-side secret management. Browser, mobile, Agent context, logs, and generated artifacts cannot access them."→ `lab.env` git 忽略且 600 权限；Stripe TEST 密钥只经 `source ~/.zcode/issue72-stripe.env` 进入环境变量；密钥值不得出现在任何仓库文件、脚本、日志、提交或截图中；提交前终检（Task 4）。
7. **金额整数**：spec 原文 ——"Monetary values use integer minor units and never pass through binary floating point."→ 断言一律用 `*_amount_cents` 整数（如 `total_paid_amount_cents == total_amount_cents`）。
8. **并行 worktree 隔离**（AGENTS.md + waves 编排 E 节）：只操作本 worktree；#73 主栈 `weknora-lago`、`weknora-lago-75/76`、OpenMeter 栈一律只读；每条 docker compose 命令必须带 `-f deploy/lago/compose.yaml --env-file <lab.env> -p weknora-lago-74`；端口固定 48891/48892（不占用 5272/5273 及其他任务的端口）；不使用共享 `git stash` 协调。
9. **数据库查询**：本计划不新增任何生产 SQL 代码；DB 核验只读 SELECT 常量语句（无外部输入拼接），与 Mimosa 约束（参数绑定、禁拼接）一致。
10. **服务端外联**：本计划不新增生产外联代码；实验链路的唯一外联是 runner/容器到 `https://api.stripe.com`（HTTPS）与本机回环 Lago API（`http://127.0.0.1:48891`，隔离实验的既定设计，lab README.md:10-11）。

## Review Focus（最可能咬人的五类输入/故障，及其归属测试）

1. **陈旧种子卷 vs 新 lab.env**（登录失败/组织错配 → 全阶段假 blocked-env）：Task 1 Step 3 强制清卷 + Task 2 Step 4 断言 `graphql_login_ok == true` 把它显式化。
2. **密钥形状错误**（live key 被拒 / 空 key 静默退化为 blocked-env / 密钥打印到终端）：Task 1 Step 2 与 Task 2 Step 3 双重前缀门禁（只输出 bool，不回显密钥）。
3. **密钥泄露进证据/提交**（sk_test 值、JWT、API key、操作者密码）：Task 2 Step 4 断言 runner 自带扫描 `secrets_scan.clean == true`；Task 4 Step 2 对密钥真值与 `sk_test_[0-9A-Za-z_-]{16,}` 形状做提交历史 + 工作树双扫描。
4. **fail/blocked 被当 pass 晋升**（把未通过运行写进 DECISION 误导 #81/#82）：Task 3 Step 1 门禁只晋升 `run.overall == "pass"` 的运行；否则走显式 fail 分支文档（Task 3 Step 6）。
5. **误伤兄弟栈**（漏带 `-p weknora-lago-74` 的 compose 命令杀掉 #73 主栈）：Task 1 Step 2 记录兄弟栈基线、所有命令模板钉死 project 名、Task 4 Step 4 复核兄弟栈容器数与健康不变。

## 文件地图（创建/修改/测试）

| 动作 | 路径 | 说明 |
|---|---|---|
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-setup.json` | 新运行覆盖（阶段报告） |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-provider.json` | 同上 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-gating.json` | AC1 证据 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-manual.json` | AC4 证据（运行时 403） |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-activation.json` | AC2 证据 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-duplicates.json` | AC2 证据 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-retries.json` | AC3 证据 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-decline.json` | 负对照证据 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-cleanup.json` | 清理证据 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-environment.json` | 运行环境 + 密钥扫描汇总 |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-run.txt` | runner 原始 timeline |
| 修改 | `deploy/lago-lab/payment-activation/evidence/t02-health.json` | 栈健康快照（Task 1 采集） |
| 修改 | `docs/migrations/lago/t02-payment-activation/t02-{gating,manual,activation,duplicates,retries,decline,cleanup,environment}.json` + `t02-run.txt` | 晋升（仅 pass 分支） |
| 修改 | `docs/migrations/lago/t02-payment-activation/README.md` | 状态列 + 解读更新 |
| 修改 | `docs/migrations/lago/t02-payment-activation/DECISION.md` | §2/§3/§5/§6/§7 运行时裁决更新 |
| 追加 | `docs/plans/issue-72-ledger-74.md` | 执行 Ledger |
| 创建（git 忽略） | `deploy/lago-lab/payment-activation/lab.env` | `lab.sh init` 生成，600 权限，绝不提交 |
| 创建（git 忽略） | `deploy/lago-lab/payment-activation/runs/db_watch.sh`、`runs/db-watch-<时间戳>.tsv` | DB 侧观察者及其输出（`runs/` 在 `.gitignore`） |
| 测试 | `deploy/lago-lab/payment-activation/test_lab.py`、`test_phases.py` | 60 个离线回归（只跑不改） |

**明确不改动**：`phases.py`/`clients.py`/`run_lab.py`/`lab.sh`/`fixtures.py`/两个测试文件（已交付且绿）；`deploy/lago/` 全目录（DECISION.md §7：对 #73 资产零改动、只读复用）；`clients.worktree-variant.py`（旁支历史，上报不动）；`docs/plans/2026-09-20-lago-billing-waves.md`（主 Agent 编排文档）；任何 Go/TS 生产代码。

## Consumes / Produces（精确签名）

**Consumes（前置 Issue 与基线已提供）：**

1. #73 pinned 栈资产（只读复用，`lab.sh:21`、`run_lab.py:46-47`）：
   - `deploy/lago/compose.yaml`、`deploy/lago/images.lock.json`、`deploy/lago/health.py`
   - `health.py` 接口（本次计划仅经 `lab.sh status` 间接消费）：`load_release_identity(lock_path=LOCK_PATH) -> {"release": str, "images": dict}`；`build_snapshot(rows, api_ok, release_identity) -> {"release", "images", "api": {"state"}, "services": dict, "overall": str}`（`overall == "ready"` 为健康判据，`health.py:95-106`）。
2. 基线实验设施（本分支已含）：
   - `lab.sh` 子命令：`init|up|down|status|config`（`lab.sh:152-162`）。
   - `run_lab.py` CLI：`--output-dir`（默认 `<lab>/evidence`）、`--lab-env`（默认 `<lab>/lab.env`）、`--poll-interval 3.0`、`--poll-timeout 300.0`、`--stability-rounds 3`、`--stability-delay 5.0`；退出码 `0=pass / 1=fail / 2=blocked-env`（`run_lab.py:50`）。
   - `phases.RunContext.__init__(lago_url, api_key, run_id=None, prefix=None, stripe_key=None, graphql_jwt=None, stripe_base_url="https://api.stripe.com", poll_interval=2.0, poll_timeout=180.0, stability_rounds=3, stability_delay=3.0, request_timeout=30.0)`（`phases.py:64`）。
   - 阶段函数签名：`phase_setup(ctx)`、`phase_provider_setup(ctx)`、`phase_gate(ctx)`、`phase_manual(ctx)`、`phase_activate(ctx)`、`phase_duplicates(ctx)`、`phase_retries(ctx)`、`phase_decline_control(ctx)`、`phase_cleanup(ctx)`，均 `-> report: dict`，report schema：`{"phase": str, "run_id": str, "expected": str, "observed": dict, "status": "pass"|"fail"|"blocked-env", "evidence": dict, "contract_notes": list[str], "recorded_at": str}`。
   - `clients.STRIPE_TEST_PREFIXES = ("sk_test_", "rk_test_")`（`clients.py:55`）；`clients.sanitize(value, extra_secrets=())`；`LagoRestClient` / `StripeTestClient(stripe_key, base_url=..., timeout=...)`。
3. 用户输入：`~/.zcode/issue72-stripe.env` 的 `STRIPE_SECRET_KEY`（`sk_test_` 前缀已验证；仅测试环境凭据）。

**Produces（本计划产出的接口）：**

1. 运行时证据包（供 #74 验收与 #81/#82 决策消费）：
   - `deploy/lago-lab/payment-activation/evidence/t02-{setup,provider,gating,manual,activation,duplicates,retries,decline,cleanup,environment}.json` + `t02-run.txt` + `t02-health.json`，schema 同上 report；`t02-environment.json` 额外含 `run.overall`、`run.phase_statuses`、`release.release`、`stripe.{key_source,test_mode_key_present,refused_non_test_key,api_container_reachability}`、`secrets_scan.{files_scanned,scrubbed,hits_after_scrub,clean}`。
   - 晋升副本：`docs/migrations/lago/t02-payment-activation/`（7 个 AC/环境 JSON byte-identical + 叙述版 `t02-run.txt`）。
2. `docs/migrations/lago/t02-payment-activation/DECISION.md` 更新后的裁决（#81/#82 的消费物）：manual 运行时 403 证实（§2）、provider 路径端到端运行时证实或证伪（§3）、§5 选项 (a)/(b) 证据基础刷新、§6 AC 映射状态刷新。**无任何代码接口变更**（无 Go/TS/Python API、无路由、无 DB migration）。
3. Ledger：`docs/plans/issue-72-ledger-74.md`（执行记录 + 上报事项）。

## 真实流程验证方案（本 Issue 的核心交付即真实环境验证）

**性质**：纯后端/运维实验链路（无用户可见页面）。Lago 前端 `http://127.0.0.1:48892` 仅作可选人工观察（见本节末尾），权威验证为 API 链路 + DB 投影核验。

**环境与端口**：docker compose Lago 实验栈 = `./deploy/lago-lab/payment-activation/lab.sh`（Compose 项目 `weknora-lago-74`，回环 48891 API / 48892 front，五镜像 v1.53.0 digest 锁定，与 #73 主栈 48889/48890 及 75/76 栈完全隔离）；WeKnora 后端/前端**不需要启动**（本票不触产品代码，商业 readiness 端点等不在本票范围）；避开 :5272/:5273；Stripe TEST key 经 `~/.zcode/issue72-stripe.env` 注入。

**种子数据**：`lab.sh init` 一次性生成（`LAGO_CREATE_ORG=true` + 随机操作者账号/组织 API key，首次 `up` 时由 migrate 服务消费，lab.env.example:15-22）；实验对象全部由 runner 以 `weknora-t02-<run-uuid>` 前缀自建自清（Feature/Plan/Entitlement/3 个 Stripe 测试客户 A(3DS 挑战卡)/B(成功卡)/C(拒付卡)/gated Subscription/gating Invoice），无个人数据。

**请求序列（API 调用链，由 run_lab.py 编排，此处为人可复核的链路）**：
1. `GET http://127.0.0.1:48891/health` → 200；`POST /graphql`（loginUser，操作者 JWT，绝不落盘）。
2. `setup`：`POST /api/v1/features`、`POST /api/v1/plans`（monthly/pay_in_advance/5000 cents）、`POST /api/v1/plans/{code}/entitlements`（hash 形态，DECISION.md §7 契约注记）。
3. `provider_setup`：GraphQL `AddStripePaymentProvider` + Stripe TEST API 建 3 客户并挂默认支付方式（`https://api.stripe.com/v1/customers|payment_methods|...`）。
4. `gate`（AC1）：`POST /api/v1/subscriptions`（`activation_rules: [{type: payment, timeout_hours: 0}]`）→ 断言 `subscription.status == "incomplete"`；`GET /api/v1/subscriptions/{ext}/entitlements` → 404；`GET /api/v1/customers/{id}/invoices` → gating invoice `open` + `payment_status: pending` + 无编号；`GET .../payments` → ≤1 非 succeeded；稳定性窗口 3×5s。
5. `manual`（AC4）：`POST /api/v1/payments {invoice_id, amount_cents, reference, paid_at}` → 预期 403（Community Premium 门控），状态不变。
6. `activate`（AC2）：客户 B gated 订阅 → 轮询 `GET /api/v1/subscriptions/{ext}?status[]=active` → active；entitlements 200 含计划 feature；invoice `finalized`+编号+`succeeded`；恰好 1 笔 succeeded provider payment（`provider_payment_id` 非空）；`total_paid_amount_cents == total_amount_cents`。
7. `duplicates`（AC2）：同 external_id 重新 POST 订阅 / `retry_payment` / manual 再 POST → 全部拒绝，终态 byte-identical。
8. `retries`（AC3）：响应丢失恢复 = 同 external_id GET（带显式 `status[]`）恰好 1 个对象且 lago_id 相同；对 pending 门的 retry 不产生第二笔 payment 行。
9. `decline_control`：客户 C 拒付 → invoice `closed`、订阅 `canceled` + `cancellation_reason: payment_failed`、entitlements 404。
10. `cleanup`：删除本 run 创建的全部对象并复读 404 验证。

**数据库/投影状态核验（Task 2 Step 3 的 DB 观察者）**：运行期间每 2s 在 lab 的 postgres 容器内执行只读采样 `SELECT status, count(*) FROM subscriptions GROUP BY status` / `SELECT status, count(*) FROM payments GROUP BY status` / `SELECT status, payment_status, count(*) FROM invoices GROUP BY 1,2` / `SELECT external_id, status FROM subscriptions WHERE external_id LIKE 'weknora-t02-%'`，落 TSV（git 忽略目录）。通过判据：任一采样点 succeeded payments 总数 ≤1 且运行中段恰为 1（DB 级 exactly-once）；`-sub-b` 行出现过 `active` 且之后不回退；`-sub-c` 出现 `canceled`。注：`-sub-a` 的 incomplete 窗口可能短于采样间隔，DB 采样捕不到不算失败——AC1 权威证据是 API 级 gate 阶段报告。

**通过判据（汇总）**：`run_lab.py` 退出码 0；`t02-environment.json` 的 `run.overall == "pass"`、九阶段 `phase_statuses` 全 `pass`、`release.release == "v1.53.0"`、`stripe.test_mode_key_present == true`、`secrets_scan.clean == true`；本节上述各 AC 断言全真；DB 观察者判据全真。任一不满足 → 走 fail/blocked 分支如实记录，不得晋升为 pass。

**可选人工观察（不作为判据）**：运行期间用 Playwright MCP 打开 `http://127.0.0.1:48892`，以 lab.env 的操作者邮箱/密码登录（密码框为掩码输入，截图不得含密钥明文），在 Customers/Subscriptions 列页观察 `incomplete → active`。截图保存到 git 忽略目录 `deploy/lago-lab/payment-activation/runs/`（如 `runs/front-subscriptions-<时间戳>.png`），不提交；若发现截图可能含敏感值则不保留。

---

### Task 1: 预检、陈旧卷重置与隔离栈拉起

**Files:**
- Create（git 忽略）: `deploy/lago-lab/payment-activation/lab.env`（由 `lab.sh init` 生成）
- Modify: `deploy/lago-lab/payment-activation/evidence/t02-health.json`（新健康快照）
- Test: `deploy/lago-lab/payment-activation/test_lab.py`、`test_phases.py`（回归，只跑不改）

**Interfaces:**
- Consumes: `lab.sh init|up|status`；`deploy/lago/compose.yaml` + `images.lock.json` + `health.py`（只读）；`~/.zcode/issue72-stripe.env`（仅验证存在与前缀，不 source）。
- Produces: 运行中的 `weknora-lago-74` 隔离栈（API `http://127.0.0.1:48891` 健康，前端 48892）；`evidence/t02-health.json`（`overall == "ready"`）。

- [ ] **Step 1: 基线回归——60 个离线测试必须先绿**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74/deploy/lago-lab/payment-activation
python3 -m pytest test_lab.py test_phases.py -q
```
Expected: `60 passed`（无 fail/error；本计划编写时实测 `60 passed in 37.20s`）。若不绿，停止并按 superpowers:systematic-debugging 排查——本计划不允许改 lab 代码，回归不绿即上报主 Agent。

- [ ] **Step 2: 预检——分支/端口/兄弟栈/密钥文件**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
git rev-parse --short HEAD && git status --short   # 期望: dafba8851（基线）+ 干净（或仅 docs/plans 变更）
lsof -nP -iTCP:48891 -iTCP:48892 -sTCP:LISTEN      # 期望: 无输出（端口空闲）
docker ps --format '{{.Names}}\t{{.Status}}' | tee /tmp/t02-sibling-baseline.txt
#   期望包含 weknora-lago-api-1 等 #73 主栈容器 healthy——记下基线，Task 4 复核
python3 - <<'PY'
import os, re, sys
path = os.path.expanduser("~/.zcode/issue72-stripe.env")
ok = os.path.isfile(path)
prefix = False
if ok:
    m = re.search(r"^STRIPE_SECRET_KEY=(\S{8})", open(path).read(), re.M)
    prefix = bool(m) and m.group(1).startswith("sk_test_")
print("stripe_env_exists:", ok, "prefix_ok:", prefix)   # 只打印布尔，绝不打印密钥
sys.exit(0 if ok and prefix else 1)
PY
```
Expected: HEAD 为 `dafba8851`（或其直系后代）、端口空闲、兄弟栈基线已存 `/tmp/t02-sibling-baseline.txt`、输出 `stripe_env_exists: True prefix_ok: True`。

- [ ] **Step 3: 清除陈旧种子卷（关键——见调查结论第 6 条）**

```bash
docker compose -p weknora-lago-74 ps -q --all | wc -l   # 期望 0（无残留容器）
docker volume rm weknora-lago-74_lago_postgres_data weknora-lago-74_lago_redis_data weknora-lago-74_lago_storage_data
docker volume ls --format '{{.Name}}' | grep '^weknora-lago-74_'   # 期望: 无输出
```
Expected: 三个旧卷删除成功、`grep` 无输出。若 `volume rm` 报「volume in use」，说明有容器占用：`docker ps -a --filter label=com.docker.compose.project=weknora-lago-74` 找到并 `docker rm` 后重试；**禁止**对其他项目名的卷执行 rm。

- [ ] **Step 4: 生成 lab.env 并冷启动隔离栈**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
./deploy/lago-lab/payment-activation/lab.sh init
ls -l deploy/lago-lab/payment-activation/lab.env        # 期望: 权限 600；绝不 cat 其内容
grep -c '^LAGO_CREATE_ORG=true$' deploy/lago-lab/payment-activation/lab.env   # 期望: 1
./deploy/lago-lab/payment-activation/lab.sh up           # 冷启动约 2-4 分钟（镜像已随 #73 缓存；首次 migrate+seed）
```
Expected: `up` 退出码 0（`docker compose up -d --wait` 全 healthy）。失败时看 `./deploy/lago-lab/payment-activation/lab.sh status` 与 `docker compose -p weknora-lago-74 logs api | tail -50`；seed 失败最常见原因是卷未清干净（回到 Step 3）。

- [ ] **Step 5: 健康快照并采集 t02-health.json**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
./deploy/lago-lab/payment-activation/lab.sh status > deploy/lago-lab/payment-activation/evidence/t02-health.json
python3 -c "import json; s=json.load(open('deploy/lago-lab/payment-activation/evidence/t02-health.json')); print(s['overall'], s['release'])"
```
Expected: 输出 `ready v1.53.0`（`overall` 判据来自 `health.py:95-106`）。

- [ ] **Step 6: 提交健康快照**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
git add deploy/lago-lab/payment-activation/evidence/t02-health.json
git commit -m "issue-72(#74): refresh t02 lab stack health snapshot (ready, v1.53.0)"
```

### Task 2: 带 Stripe TEST key 的真实实验运行（RED → GREEN + DB 投影观察）

**Files:**
- Modify: `deploy/lago-lab/payment-activation/evidence/` 下全部 11 个文件（9 阶段 JSON + `t02-environment.json` + `t02-run.txt`）
- Create（git 忽略）: `deploy/lago-lab/payment-activation/runs/db_watch.sh`、`runs/db-watch-<时间戳>.tsv`
- Test: 即本 Task 的断言脚本（对证据文件断言，代替新代码的单元测试）

**Interfaces:**
- Consumes: Task 1 的运行栈；`run_lab.py`（`--output-dir`、退出码 0/1/2）；`STRIPE_SECRET_KEY` 环境变量；容器内 psql（`POSTGRES_USER`/`POSTGRES_DB` 默认 `lago`，compose.yaml:82-83）。
- Produces: `evidence/` 下真实 pass（或如实 fail/blocked-env）的运行时证据包；`runs/db-watch-*.tsv` 观察数据。

- [ ] **Step 1: RED——对当前提交的证据运行断言，确认其失败**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
for f in t02-gating t02-manual t02-activation t02-duplicates t02-retries t02-decline; do
  printf '%s: %s\n' "$f" "$(python3 -c "import json;print(json.load(open('deploy/lago-lab/payment-activation/evidence/$f.json'))['status'])")"
done
```
Expected: 六行全部 `blocked-env`（当前 RED 状态；真实运行后同一断言应全变 `pass`——GREEN）。

- [ ] **Step 2: 写 DB 观察者脚本（git 忽略目录，代码仅存于本计划与 runs/）**

```bash
mkdir -p /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74/deploy/lago-lab/payment-activation/runs
cat > /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74/deploy/lago-lab/payment-activation/runs/db_watch.sh <<'EOF'
#!/usr/bin/env bash
# T02 DB-side observer: read-only samples of Lago projection tables while
# run_lab.py executes. Output TSV to $1; stop after $2 seconds (default 1200).
set -euo pipefail
REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
COMPOSE_FILE="$REPO_DIR/deploy/lago/compose.yaml"
ENV_FILE="$REPO_DIR/deploy/lago-lab/payment-activation/lab.env"
OUT="${1:-$REPO_DIR/deploy/lago-lab/payment-activation/runs/db-watch.tsv}"
MAX_SECONDS="${2:-1200}"
mkdir -p "$(dirname "$OUT")"
: > "$OUT"
q() {
  docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p weknora-lago-74 \
    exec -T db sh -c 'psql -U "${POSTGRES_USER:-lago}" -d "${POSTGRES_DB:-lago}" -Atc "'"$1"'"' 2>/dev/null \
    | paste -sd ';' -
}
start=$(date +%s)
while :; do
  ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
  subs=$(q "SELECT status, count(*) FROM subscriptions GROUP BY status ORDER BY status")
  pays=$(q "SELECT status, count(*) FROM payments GROUP BY status ORDER BY status")
  invs=$(q "SELECT status, payment_status, count(*) FROM invoices GROUP BY 1,2 ORDER BY 1,2")
  rows=$(q "SELECT external_id, status FROM subscriptions WHERE external_id LIKE 'weknora-t02-%' ORDER BY external_id")
  printf '%s\tsubs[%s]\tpayments[%s]\tinvoices[%s]\trows[%s]\n' "$ts" "$subs" "$pays" "$invs" "$rows" >> "$OUT"
  now=$(date +%s)
  if [ $((now - start)) -ge "$MAX_SECONDS" ]; then break; fi
  sleep 2
done
EOF
chmod +x /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74/deploy/lago-lab/payment-activation/runs/db_watch.sh
```
Expected: 文件创建成功（`runs/` 在 lab `.gitignore` 中，不会入库）。

- [ ] **Step 3: 注入密钥、形状门禁、启动观察者并运行真实实验（同一 shell 会话执行，环境变量不跨会话）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
set -a; source ~/.zcode/issue72-stripe.env; set +a
python3 - <<'PY'
import os, sys
key = os.environ.get("STRIPE_SECRET_KEY", "")
ok = key.startswith(("sk_test_", "rk_test_"))
print("key_present:", bool(key), "test_mode_prefix:", ok)   # 只打印布尔，绝不回显密钥
sys.exit(0 if ok else 1)
PY
WATCH_TS=$(date -u +%Y%m%dT%H%M%SZ)
./deploy/lago-lab/payment-activation/runs/db_watch.sh \
  "deploy/lago-lab/payment-activation/runs/db-watch-$WATCH_TS.tsv" 1500 &
WATCH_PID=$!
./deploy/lago-lab/payment-activation/run_lab.py \
  --output-dir deploy/lago-lab/payment-activation/evidence
RUN_RC=$?
sleep 3; kill "$WATCH_PID" 2>/dev/null || true
echo "run_lab exit code: $RUN_RC"
```
Expected（GREEN 判据）: `run_lab exit code: 0`；运行时长约 3–8 分钟（activate/decline 各有最长 300s 轮询）。观察者覆盖窗口 1500s ≥ 运行时长。
- 退出码 `2`（blocked-env）：读 `evidence/t02-environment.json` 的 `stripe.*` 与 `lago.graphql_login_ok` 定位（密钥缺失/被拒、栈不健康、`api_container_reachability.reachable != true` 即容器到 api.stripe.com 不通）；修复环境后重跑本 Step（重跑前无需清卷，cleanup 已自清对象；若 cleanup 未跑需 `lab.sh down && lab.sh up` 重置）。最多重试 2 次，仍 blocked 则停止并上报。
- 退出码 `1`（fail）：走 Task 3 Step 6 的 fail 分支，**不得**重跑刷绿掩盖。

- [ ] **Step 4: GREEN——逐 AC 断言（API 级，权威判据）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74/deploy/lago-lab/payment-activation/evidence
python3 - <<'PY'
import json, sys
def load(n): return json.load(open(f"{n}.json"))
fail = []
def check(name, cond):
    print(("PASS " if cond else "FAIL ") + name)
    if not cond: fail.append(name)

env = load("t02-environment")
check("env.overall==pass", env["run"]["overall"] == "pass")
check("env.release==v1.53.0", env["release"]["release"] == "v1.53.0")
check("env.stripe.test_mode_key_present", env["stripe"]["test_mode_key_present"] is True)
check("env.graphql_login_ok", env["lago"]["graphql_login_ok"] is True)
check("env.secrets_scan.clean", env["secrets_scan"]["clean"] is True)
check("env.all_phases_pass", all(v == "pass" for v in env["run"]["phase_statuses"].values()))

# AC1: gate
g = load("t02-gating"); o = g["observed"]
check("AC1 gate.status==pass", g["status"] == "pass")
check("AC1 subscription incomplete", o["subscription_status"] == "incomplete")
check("AC1 entitlements 404", o["entitlements_status"] == 404)
check("AC1 invoice open/pending/numberless",
      o["invoice_status"] == "open" and o["invoice_payment_status"] == "pending"
      and not o["invoice_number"])
check("AC1 <=1 non-succeeded payment", o["payments_non_succeeded_count"] <= 1)

# AC4: manual (runtime 403 on Community)
m = load("t02-manual")
check("AC4 manual.status==pass", m["status"] == "pass")

# AC2: activate + duplicates
a = load("t02-activation"); oa = a["observed"]
check("AC2 activation.status==pass", a["status"] == "pass")
check("AC2 checks all true", all(oa["checks"].values()))
check("AC2 exactly one succeeded payment", oa["payments_succeeded_count"] == 1)
check("AC2 provider_payment_id recorded", bool(oa["provider_payment_id"]))
d = load("t02-duplicates")
check("AC2 duplicates.status==pass", d["status"] == "pass")
check("AC2 duplicates final state intact",
      d["observed"]["final"]["payments_succeeded_count"] == 1
      and d["observed"]["final"]["subscription_status"] == "active")

# AC3: retries
r = load("t02-retries")
check("AC3 retries.status==pass", r["status"] == "pass")
check("AC3 checks all true", all(r["observed"]["checks"].values()))

# negative control
c = load("t02-decline")
check("decline.status==pass", c["status"] == "pass")
check("decline checks all true", all(c["observed"]["checks"].values()))

print("RESULT:", "ALL PASS" if not fail else f"{len(fail)} FAILED: {fail}")
sys.exit(0 if not fail else 1)
PY
```
Expected: 末行 `RESULT: ALL PASS`（逐项 PASS 共 20 条）。注意：若 gate 的 3DS 窗口未保持（`stable_window.still_incomplete == false`），phase 仍可按 canceled 回退分支判 pass（`phases.py:479-494`），此时 `AC1 subscription incomplete` 等字段断言以阶段 `status == "pass"` 为准、字段断言允许改为「pre-charge 窗口观察成立」——在 Ledger 记录该分支即可。

- [ ] **Step 5: DB 投影核验（观察者数据）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74/deploy/lago-lab/payment-activation/runs
python3 - <<'PY'
import glob, re, sys
path = sorted(glob.glob("db-watch-*.tsv"))[-1]
lines = open(path).read().splitlines()
def max_succeeded():
    best = 0
    for ln in lines:
        m = re.search(r"payments\[([^\]]*)\]", ln)
        if not m: continue
        for part in m.group(1).split(";"):
            if part.startswith("succeeded|"):
                best = max(best, int(part.split("|")[1]))
    return best
sub_b_active = any(re.search(r"rows\(\[[^\]]*-sub-b\|active", ln) for ln in lines)
sub_b_regress = False
seen_active = False
for ln in lines:
    m = re.search(r"rows\(\[([^\]]*)\]", ln)
    if not m: continue
    b = [p for p in m.group(1).split(";") if p.endswith("-sub-b|active")]
    if b: seen_active = True
    elif seen_active and any(p.endswith("-sub-b|incomplete") or p.endswith("-sub-b|canceled")
                             for p in m.group(1).split(";")):
        sub_b_regress = True
sub_c_canceled = any(re.search(r"rows\(\[[^\]]*-sub-c\|canceled", ln) for ln in lines)
print("samples:", len(lines), "max_succeeded_payments:", max_succeeded(),
      "sub_b_active:", sub_b_active, "sub_b_regressed:", sub_b_regress, "sub_c_canceled:", sub_c_canceled)
ok = max_succeeded() <= 1 and sub_b_active and not sub_b_regress and sub_c_canceled
print("DB-WATCH:", "PASS" if ok else "CHECK")
sys.exit(0 if ok else 1)
PY
```
Expected: `DB-WATCH: PASS`（succeeded 峰值恰为 1 且从未超过 1；`-sub-b` 达到 active 后不回退；`-sub-c` 出现 canceled）。注意 `-sub-a` 的 incomplete 窗口短于 2s 采样时可能捕不到，不作判据。若 cleanup 已把行删光导致采样末段为空，只看运行中段样本。

- [ ] **Step 6: 提交运行时证据**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
git add deploy/lago-lab/payment-activation/evidence
git status --short   # 确认只有 evidence/ 下 11 个文件、无 lab.env、无 runs/
git commit -m "issue-72(#74): real-stack t02 runtime evidence with Stripe TEST key (AC1-AC4)"
```

### Task 3: 证据晋升与文档更新（仅 pass 分支；fail 分支见 Step 6）

**Files:**
- Modify: `docs/migrations/lago/t02-payment-activation/`（7 个 JSON + `t02-run.txt` + `README.md` + `DECISION.md`）
- Modify: `docs/plans/issue-72-ledger-74.md`（追加执行记录）

**Interfaces:**
- Consumes: Task 2 的 pass 证据包；既有晋升惯例（JSON byte-identical、`t02-run.txt` 为叙述 + 内嵌 verbatim timeline）。
- Produces: 面向 #81/#82 与评审的最终证据束 + 更新后的 DECISION.md 裁决。

- [ ] **Step 1: 晋升门禁——只晋升 pass 运行**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
python3 -c "import json; e=json.load(open('deploy/lago-lab/payment-activation/evidence/t02-environment.json')); print(e['run']['overall'])"
```
Expected: `pass`。非 `pass` → 跳到 Step 6（fail 分支），Step 2-5 不执行。

- [ ] **Step 2: 逐字晋升 7 个 AC/环境 JSON 并复核 byte-identical**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
for f in t02-gating t02-manual t02-activation t02-duplicates t02-retries t02-decline t02-environment; do
  cp "deploy/lago-lab/payment-activation/evidence/$f.json" "docs/migrations/lago/t02-payment-activation/$f.json"
  cmp "deploy/lago-lab/payment-activation/evidence/$f.json" "docs/migrations/lago/t02-payment-activation/$f.json" || exit 1
done
echo "promoted 7 files, all byte-identical"
```
Expected: `promoted 7 files, all byte-identical`。注意：`t02-setup/provider/health.json` 不晋升（惯例：非 AC 工作报告留 deploy 侧，docs README.md:44-47）。

- [ ] **Step 3: 生成叙述版 docs t02-run.txt（内嵌 verbatim timeline）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
RUN_ID=$(python3 -c "import json;print(json.load(open('deploy/lago-lab/payment-activation/evidence/t02-environment.json'))['run']['run_id'])")
STAMP=$(date -u +%Y-%m-%d)
cat > docs/migrations/lago/t02-payment-activation/t02-run.txt <<EOF
Lago T02 payment-activation lab — real-run operator timeline (sanitized)
=========================================================================

Recorded $STAMP (UTC) from worktree .worktrees/issue72-n74 (branch
codex/issue-72-lago-74). Secrets (API key, operator password, GraphQL JWT,
Stripe key) are never included; the runner's own timeline is embedded
verbatim below. First attempt 2026-09-20 was blocked-env (no Stripe TEST
key); this run had STRIPE_SECRET_KEY (sk_test_ test mode) sourced from the
operator's local env file and is the full AC1-AC4 contract run.

[stack] lab.sh volumes wiped (stale seed from the deleted 2026-09-20
worktree), lab.sh init (fresh lab.env, mode 600), lab.sh up healthy,
lab.sh status overall "ready" on release v1.53.0 (evidence/t02-health.json
in the lab directory).

[run $RUN_ID] DB-side observer (read-only psql sampling every 2s) ran in
parallel; its exactly-once findings are in the execution ledger.

--- verbatim runner timeline (evidence/t02-run.txt) ---
$(cat deploy/lago-lab/payment-activation/evidence/t02-run.txt)
--- end verbatim timeline ---

[post] lab.sh down (volumes preserved); sibling projects weknora-lago
(#73), weknora-lago-75/76 untouched throughout.
EOF
tail -5 docs/migrations/lago/t02-payment-activation/t02-run.txt
```
Expected: 文件以 `--- end verbatim timeline ---` 起的后记结尾；`RUN_ID` 为本次运行 UUID（与 `t02-environment.json` 一致）。

- [ ] **Step 4: 更新 docs README.md 状态列与解读（脚本驱动，值取自证据）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
python3 - <<'PY'
import json, re
from pathlib import Path
p = Path("docs/migrations/lago/t02-payment-activation/README.md")
text = p.read_text()
ev = json.load(open("deploy/lago-lab/payment-activation/evidence/t02-environment.json"))
phases = ev["run"]["phase_statuses"]
# 状态列：把每个证据文件行的 "`blocked-env` — ..." 单元替换为实际 status
mapping = {
    "t02-environment.json": "environment",
    "t02-gating.json": "gate", "t02-manual.json": "manual",
    "t02-activation.json": "activate", "t02-duplicates.json": "duplicates",
    "t02-retries.json": "retries", "t02-decline.json": "decline_control",
    "t02-cleanup.json": "cleanup",
}
for fname, phase in mapping.items():
    # 表头为 | File | What it proves | Status in this run | How it was produced |
    # group1 = "| `file` | cell2 |"，中段 [^|]* = 状态列（第 3 列）全部内容，group2 = 收尾竖线
    pat = re.compile(r"(\|\s*`" + re.escape(fname) + r"`\s*\|[^|]*\|)[^|]*(\|)")
    status = phases[phase]
    text, n = pat.subn(lambda m: m.group(1) + f" `{status}` (real run, Stripe TEST key present) " + m.group(2), text, count=1)
    assert n == 1, f"status cell not found for {fname}"
p.write_text(text)
print("status column updated from run.overall =", ev["run"]["overall"])
PY
grep -c 'blocked-env — requires\|blocked-env` overall' docs/migrations/lago/t02-payment-activation/README.md || echo "0 leftover blocked-env cells"
```
Expected: 每个被替换行显示实际 `status`（应全为 `pass`）；剩余 blocked-env 计数为 0（或仅剩描述首次运行的叙述句，需人工确认为历史叙述而非当前状态）。随后人工完成三处叙述更新：

1. 头部「Produced 2026-09-20 on the T02 worktree (`lago-74-payment-activation-lab`)」之后追加一句：
   > Re-run with a Stripe TEST-mode key on 2026-09-23 (worktree `.worktrees/issue72-n74`); the files below are that passing run.
2. 「Interpretation for Ticket #74」节中 “In **this** run those files carry `blocked-env` verdicts because no Stripe TEST-mode key was available; the contract claims themselves are NOT asserted from this run.” 一句替换为：
   > In the re-run (Stripe TEST key present via caller environment) those files carry real runtime verdicts — every phase `pass` — so the AC1–AC4 contract claims ARE asserted from this run; the first attempt's `blocked-env` reports remain in git history as the honest environment-gap record.
3. 「Evidence contract」节第三条 bullet 中 “In this run the caller environment had no Stripe TEST-mode key (`STRIPE_TEST_SECRET_KEY` / `STRIPE_SECRET_KEY`), so every provider/payment-dependent phase reports `blocked-env` with its explicit reason.” 替换为：
   > In the first attempt (2026-09-20) the caller environment had no Stripe TEST-mode key, so every provider/payment-dependent phase reported `blocked-env`; the 2026-09-23 re-run had the test key present and every phase reports a real verdict.

另外「Regenerating」节的示例命令中 `STRIPE_SECRET_KEY=<sk_test_…>` 保持为前缀占位写法（不得填入真值）。

- [ ] **Step 5: 更新 DECISION.md 运行时裁决（脚本注入实际观测值）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
python3 - <<'PY'
import json
from pathlib import Path
lab = "deploy/lago-lab/payment-activation/evidence/"
man = json.load(open(lab + "t02-manual.json"))
act = json.load(open(lab + "t02-activation.json"))
ret = json.load(open(lab + "t02-retries.json"))
p = Path("docs/migrations/lago/t02-payment-activation/DECISION.md")
t = p.read_text()

old2 = """- **Runtime evidence this run:** [`t02-manual.json`](./t02-manual.json)
  reports `blocked-env` — the manual attempt requires customer A's gating
  invoice, which requires a provider-connected customer, which requires a
  Stripe TEST-mode key that was absent from the caller environment. The
  403-on-Community claim is therefore **source-verified, unproven at
  runtime in this run**; re-run with a test key to capture the live 403."""
new2 = """- **Runtime evidence (re-run with a Stripe TEST key):**
  [`t02-manual.json`](./t02-manual.json) reports `pass` —
  `POST /api/v1/payments` against customer A's open gating invoice returned
  HTTP 403 Forbidden and left invoice + subscription state unchanged. The
  Community 403 is now **runtime-proven**, matching the source verdict."""
assert old2 in t; t = t.replace(old2, new2)

old3 = """- **Runtime evidence this run:** [`t02-activation.json`](./t02-activation.json),
  [`t02-duplicates.json`](./t02-duplicates.json),
  [`t02-retries.json`](./t02-retries.json),
  [`t02-decline.json`](./t02-decline.json) all report `blocked-env`
  (missing Stripe TEST-mode key in the caller environment —
  [`t02-environment.json`](./t02-environment.json) records
  `stripe.test_mode_key_present: false` with the env-var names checked).
  The end-to-end provider-path proof is therefore **pending one
  environment input**, not a negative result: the stack, runner, operator
  login, real object creation (`t02-setup.json`: **pass**), and cleanup
  (`t02-cleanup.json`: **pass**) all worked on the pinned runtime.
  Re-running with `STRIPE_SECRET_KEY=<sk_test_…>` executes the full AC1–AC4
  chain unchanged (see the lab README workflow)."""
new3 = f"""- **Runtime evidence (re-run with a Stripe TEST key,
  [`t02-environment.json`](./t02-environment.json)
  `run.overall: pass`):** [`t02-activation.json`](./t02-activation.json)
  reports `pass` — customer B's gated subscription became `active` with
  {act['observed']['payments_succeeded_count']} succeeded provider payment
  (`provider_payment_id` present), the invoice finalized, numbered and
  `payment_status: succeeded`, entitlements 200 with the plan feature,
  totals matched in integer cents.
  [`t02-duplicates.json`](./t02-duplicates.json) `pass` (all duplicate
  re-registration probes rejected, final state byte-identical);
  [`t02-retries.json`](./t02-retries.json) `pass`
  ({json.dumps(ret['observed']['checks'])});
  [`t02-decline.json`](./t02-decline.json) `pass` (declined charge →
  invoice closed, subscription `canceled` with
  `cancellation_reason: payment_failed`, entitlements 404).
  The end-to-end provider-path activation is therefore
  **runtime-proven on the pinned Community v1.53.0**."""
assert old3 in t; t = t.replace(old3, new3)

old5 = """**Explicitly unproven:** (i) the AC1–AC4
  runtime evidence of this lab is blocked-env pending a Stripe TEST-mode key;"""
new5 = """**Explicitly unproven:** (i) ~~the AC1–AC4
  runtime evidence of this lab is blocked-env pending a Stripe TEST-mode
  key~~ (resolved by the re-run: all phases `pass`);"""
assert old5 in t; t = t.replace(old5, new5)

t = t.replace("(this run: `blocked-env`)", "(re-run: `pass`)")
old7 = """- **Environment gap to close:** supply `STRIPE_TEST_SECRET_KEY` (or
  `STRIPE_SECRET_KEY`) with an `sk_test_`/`rk_test_` value and re-run the
  documented workflow to convert every `blocked-env` phase report into a
  real pass/fail verdict for AC1–AC4."""
new7 = """- **Environment gap closed (2026-09 re-run):** the documented re-run with a
  Stripe TEST-mode key converted every phase report into a real verdict;
  see [`t02-environment.json`](./t02-environment.json) (`run.overall:
  pass`)."""
assert old7 in t; t = t.replace(old7, new7)
p.write_text(t)
print("DECISION.md updated: 4 sections rewritten, AC table statuses flipped")
PY
grep -n 'blocked-env' docs/migrations/lago/t02-payment-activation/DECISION.md | head
```
Expected: `DECISION.md updated: 4 sections rewritten, AC table statuses flipped`；剩余 `blocked-env` 出现处仅应为 §2「首次运行 blocked-env」的历史叙述与 §5 划线项（人工目检确认无「当前仍 blocked」的误导表述）。§6 表格 4 行的 `(re-run: pass)` 由脚本统一替换。

- [ ] **Step 6: fail/blocked 分支（仅当 Task 2 未取得 pass 时执行，替代 Step 2-5）**

1. 不晋升任何 JSON（docs 侧维持首次运行的 blocked-env 束，git 历史即记录）。
2. 在 `DECISION.md` §3 末尾追加「Re-run outcome (honest failure)」小节，模板（`<…>` 为运行值槽位，用给定命令取得后填入）：
   ```markdown
   ### Re-run outcome (honest failure) — <date>
   Re-run with a Stripe TEST key ended `run.overall: <overall>` (exit <rc>).
   Failing phase(s): `<phase>` — observed: `<paste the phase report's
   observed JSON verbatim>`; contract deviation vs expected: `<one-paragraph
   diff between report.expected and observed>`. No AC claim is asserted from
   this run. #81/#82 must treat section 5 options accordingly until the
   deviation is resolved.
   ```
   取值命令：`jq '.run.overall' <evidence>/t02-environment.json`、逐个 `jq '{status, observed}' <evidence>/t02-<phase>.json | grep -B1 status`。
3. 在 Ledger「上报事项」登记失败阶段与推测根因，随后仍执行 Step 7 完成提交（含 DECISION 追加小节与 Ledger），再上报主 Agent（不要自行改 lab 代码重试刷绿；如需修复实验脚本属新工作，另立任务）。
4. `t02-run.txt`（deploy 侧）保持 runner 原样输出即可，无需晋升。

- [ ] **Step 7: 更新 Ledger 并提交**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
# 编辑 docs/plans/issue-72-ledger-74.md：填写 Task 1-3 的 commit、AC→证明表、DB 观察者结论
git add docs/migrations/lago/t02-payment-activation docs/plans/issue-72-ledger-74.md
git commit -m "issue-72(#74): promote t02 runtime evidence and update DECISION (AC1-AC4 pass)"
```

### Task 4: 密钥纪律终检、回归、栈回收与上报

**Files:**
- Modify: `docs/plans/issue-72-ledger-74.md`（终检记录 + 上报事项）
- Test: 全套离线回归 + 双重密钥扫描

**Interfaces:**
- Consumes: 前三个 Task 的全部产出；`~/.zcode/issue72-stripe.env`（仅作扫描基准值，不回显）。
- Produces: 终检通过的提交历史 + Ledger 上报块（主 Agent 据此推进 #74 关闭与 #81/#82 解锁）。

- [ ] **Step 1: 离线回归最终跑**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74/deploy/lago-lab/payment-activation
python3 -m pytest test_lab.py test_phases.py -q
```
Expected: `60 passed`（证据与文档变更不应影响测试；若红，说明意外改动了代码——`git diff --stat` 排查并还原）。

- [ ] **Step 2: 密钥双扫描（真值 + 形状）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
KEY="$(sed -n 's/^STRIPE_SECRET_KEY=//p' ~/.zcode/issue72-stripe.env | tr -d '"')"
[ -n "$KEY" ] || { echo 'empty key ref'; exit 1; }
echo "-- value scan over every commit on this branch (since dafba8851) --"
if git grep -F -- "$KEY" $(git rev-list dafba8851..HEAD) 2>/dev/null; then echo 'LEAK: key value in commit content'; exit 1; else echo 'commits: clean'; fi
echo "-- value scan over working tree (untracked included; excludes .git/lab.env/runs) --"
if grep -RF -- "$KEY" . --exclude-dir=.git --exclude=lab.env --exclude-dir=runs 2>/dev/null; then echo 'LEAK'; exit 1; else echo 'worktree: clean'; fi
echo "-- shape scan over all evidence/doc changes since baseline --"
if git diff dafba8851..HEAD -- docs/migrations deploy/lago-lab/payment-activation/evidence | grep -En 'sk_test_[0-9A-Za-z_-]{16,}|rk_test_[0-9A-Za-z_-]{16,}'; then echo 'LEAK: real key shape'; exit 1; else echo 'shape: clean'; fi
```
Expected: 三段均 `clean`。说明：仓库合法存在前缀模式文本（如 README 的 `sk_test_…`、`clients.py:55` 的 `("sk_test_", "rk_test_")`、test_lab.py:37 的拆分拼接 canary），形状扫描用 16+ 字符的真实密钥形状正则避免误报；若扫描报 LEAK，立即脱敏（替换为 `***REDACTED***`）、重写提交（`git commit --amend` 或 revert），并记录到 Ledger。

- [ ] **Step 3: 回收实验栈（保留卷，惯例与 2026-09-20 运行一致）**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
./deploy/lago-lab/payment-activation/lab.sh down
docker compose -p weknora-lago-74 ps -q --all | wc -l    # 期望: 0
```
Expected: `down` 成功、项目容器数为 0；`weknora-lago-74_lago_*` 卷保留（下次 `lab.sh up` 复用已 seed 状态；注意：届时 lab.env 未变则登录正常）。

- [ ] **Step 4: 复核兄弟栈未受影响**

```bash
docker ps --format '{{.Names}}\t{{.Status}}' > /tmp/t02-sibling-after.txt
diff <(grep -E 'weknora-lago|WeKnora' /tmp/t02-sibling-baseline.txt) \
     <(grep -E 'weknora-lago|WeKnora' /tmp/t02-sibling-after.txt) && echo 'siblings unchanged'
```
Expected: `siblings unchanged`（#73 主栈与开发容器状态与 Task 1 基线一致；`weknora-lago-74_*` 七个容器出现在 after 但不在 baseline 的 diff 属预期——用 grep 过滤 `-74` 或人工确认即可）。

- [ ] **Step 5: Ledger 终稿与收尾提交**

```bash
cd /Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-n74
# Ledger 补：终检结果（60 passed / 双扫描 clean / siblings unchanged / down 完成）、
# 上报事项（见下）
git add docs/plans/issue-72-ledger-74.md
git commit -m "issue-72(#74): ledger final — evidence complete, secrets scans clean"
```

**上报事项（写入 Ledger，由主 Agent 处理，执行者不直接改 GitHub 状态）：**
1. #74 AC1–AC4 运行时证据状态（pass 全绿或 fail 明细），供主 Agent 决定关票。
2. DECISION.md §5 选项 (a)（Premium manual Payment）vs (b)（provider 轨道）仍需 spec/ADR owner 决策确认——本票证据刷新了 (b) 的运行时基础，但 (a) 的 Premium 流仍为源码级验证。
3. `clients.worktree-variant.py`（提交 5e0c958e6 引入的旁支副本）建议主 Agent 评估删除或归档，避免误导后续读者。
4. 本次运行依赖的 `~/.zcode/issue72-stripe.env` 为本机测试凭据，后续 CI/他人复跑需自行提供 TEST key（README 已有说明）。

## 文档与提交步骤（汇总）

| 顺序 | 提交 | 内容 |
|---|---|---|
| 1 | `issue-72(#74): refresh t02 lab stack health snapshot (ready, v1.53.0)` | Task 1：`evidence/t02-health.json` |
| 2 | `issue-72(#74): real-stack t02 runtime evidence with Stripe TEST key (AC1-AC4)` | Task 2：`evidence/` 其余 11 文件 |
| 3 | `issue-72(#74): promote t02 runtime evidence and update DECISION (AC1-AC4 pass)` | Task 3：docs 晋升 + README/DECISION + Ledger |
| 4 | `issue-72(#74): ledger final — evidence complete, secrets scans clean` | Task 4：Ledger 终稿 |

（规划提交先于以上：`issue-72(#74): plan` = 本计划 + Ledger 创建。）

## 验收标准 → Task → 测试 追踪矩阵

| 验收标准 | 承载 Task / Step | 判定测试（命令与期望） | 证据文件 |
|---|---|---|---|
| AC1 付款前 Subscription incomplete、Entitlement 不可用 | Task 2 Step 3（gate 阶段真实执行）、Step 4 | 断言 `AC1 gate.status==pass` 等 5 条 PASS；`t02-gating.json` 字段 `subscription_status=="incomplete"`、`entitlements_status==404`、`invoice_status=="open"`、`invoice_payment_status=="pending"`、`payments_non_succeeded_count<=1` | `evidence/t02-gating.json` → `docs/.../t02-gating.json` |
| AC2 可信付款激活 active 且重复登记不重复激活 | Task 2 Step 3（activate+duplicates）、Step 4、Step 5 | `AC2 activation.status==pass`、`checks all true`、`payments_succeeded_count==1`、`duplicates.status==pass` + 终态不变；DB 观察者 `max_succeeded_payments==1` 且 `sub_b_regressed==false` | `t02-activation.json`、`t02-duplicates.json`、`runs/db-watch-*.tsv` |
| AC3 响应丢失/超时/重试同一商业身份可恢复 | Task 2 Step 3（retries）、Step 4 | `AC3 retries.status==pass`、`checks{same_identity_recovered,no_second_payment_row,...}` 全真 | `t02-retries.json` |
| AC4 manual 不能激活时给出替代路径或 blocker | Task 2 Step 3（manual+decline）、Step 4；Task 3 Step 5（DECISION §2/§4/§5 更新） | `AC4 manual.status==pass`（运行时 403）、`decline.status==pass`；DECISION.md §2 含运行时 403 证实、§4 blocker、§5 三选项 | `t02-manual.json`、`t02-decline.json`、`DECISION.md` |
| 环境可信（pinned + 密钥在场 + 零泄露） | Task 1 Step 5；Task 2 Step 4/5；Task 4 Step 2 | `env.release==v1.53.0`、`test_mode_key_present==true`、`secrets_scan.clean==true`、双扫描 clean | `t02-environment.json`、`t02-health.json` |
| 回归不破坏既有交付 | Task 1 Step 1；Task 4 Step 1 | `60 passed` 两次 | pytest 输出记录进 Ledger |

## Self-Review 结论（编写者自检）

1. **Spec 覆盖**：spec 中与 #74 直接相关的四条（payment activation rule 不完整态、supported integration 记录 Payment、manual 不可用须走 provider 且禁止本地强置、completion gate 拒绝 mock 证据）全部映射到 AC 断言与 Global Constraints 1/2/5；本票不承担 spec 其余实现项。无缺口。
2. **占位符扫描**：全文无 TBD/TODO/「稍后实现」；证据依赖的运行值一律以「给定命令从证据 JSON 读取注入」处理（Task 3 Step 4/5 脚本），唯一显式模板槽位在 fail 分支 Step 6 并附取值命令。
3. **类型/接口一致性**：`phase_*(ctx) -> report dict`、`RunContext` 构造参数、`lab.sh` 子命令、`run_lab.py` CLI 与退出码、`t02-environment.json` 字段名均逐一对照源码（`phases.py:64`、`run_lab.py:50,53-65,355-367`、`lab.sh:152-162`）核实；DB 观察者只依赖 `subscriptions.status`、`invoices.status/payment_status`、`payments.status` 列（阶段代码经 API 序列化自这些模型字段）。
4. **Review Focus**：五项均落到具体 Task/Step 的测试（见该节）。
5. **Honest failure**：fail/blocked 分支（Task 2 Step 3、Task 3 Step 6）明确「不晋升、不刷绿、如实记录并上报」，符合 spec completion gate。
