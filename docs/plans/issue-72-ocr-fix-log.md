# Issue #72 — OCR 第 1 轮修复记录（ocr-1）

- 日期：2026-09-23（UTC）
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：`0ef4577b6`（issue-72: ocr round 1），即 OCR 第 1 轮 findings 所审的 HEAD
- 修复人：修复员（dynamic workflow subagent）

## 总则

- 全部 9 个有效 findings 按根因修复，无 deferred。
- 每个行为级修复补了回归测试；全部测试在修复版代码上通过
  （`python3 -m pytest test_lab.py test_phases.py ../../../docs/plans/issue-72-flow-evidence-74/test_verify_db_watch.py -q`
  → 74 passed，基线 64 + 新增 10）。
- 对每个 finding 先在基线提交 `0ef4577b6` 上复现/证明旧行为（RED 证据，见各条），
  再在修复版上验证（GREEN）。
- 安全约束核对：本轮不涉及服务端新增外部请求、数据库 SQL 与凭据写入；
  Stripe 测试 key 仅经 `~/.zcode/issue72-stripe.env` source 进入环境变量，
  从未回显或写入仓库（`t02-environment.json` secrets_scan 0 hits）。

## 逐条 Ruling 与处置

### OCR1-03（high）decline_control settled 分支发票不可见误判 FAIL — 已修复

- Ruling：finding 成立。expected 文本（"whenever the API can see it"）自认不可见应放行，
  而 `invoice_unpaid_terminal` 检查在 `invoice_api_visible=False`（`closed` 是
  INVISIBLE_STATUS，API 不返回该发票）时对 None 状态判 False，行为正确的运行会被误判
  FAIL。以"承诺与观测一致"为准豁免，而非放宽检查。
- 处置：`phases.py` settled 分支先计算 `invoice_unpaid_terminal =
  observed["invoice_status"] in ("closed", "failed")`，发票 API 不可见时置 True 并
  `ctx.note("invoice API-invisible (closed is INVISIBLE_STATUS); unpaid-terminal
  check waived")`；checks 字典引用该局部值。
- RED 证据：基线 `python3 -c "print(None in ('closed','failed'))"` → `False`。
- 回归测试：`test_phases.py::TestDeclineControlPhase::
  test_canceled_with_api_invisible_terminal_invoice_waives_check`
  （FakeLago 新旋钮 `decline_invoice_terminal_invisible` 模拟 v1.53.0 终态 closed
  发票从 API 消失）。

### OCR1-07（medium）phase_gate 观测段 OSError 未映射 blocked-env — 已修复

- Ruling：finding 成立。模块头（phases.py:15-17）规定 unreachable Lago API 应为
  blocked-env（exit 2）；发票轮询与全部复查探测（`ctx.poll(fetch_invoices...)`、
  `_subscription_show`）不在 try 内，异常穿透到 run_lab.run_one 的兜底 `except
  Exception`，被记为 fail/unexpected_error（exit 1）。与兄弟阶段不一致，属映射缺陷。
- 处置：把 phase_gate 从发票轮询起到函数尾的整段观测逻辑（152 行）包进
  `try/except OSError → return _blocked(ctx, "gate", f"Lago API unreachable
  ({error.__class__.__name__})", expected)`；段内原有局部 return（含 FAIL 分支）
  正常返回不受影响（Python try 块内 return 不经过 except）。
- RED 证据：AST 检查基线 `phase_gate`：`ctx.poll` 调用位于第 514 行，不在任何
  OSError try 覆盖内（`inside an OSError try: False`）。
- 回归测试：`TestGatePhase::test_transport_failure_during_invoice_poll_is_blocked_env`、
  `TestGatePhase::test_transport_failure_during_recheck_probe_is_blocked_env`
  （发票轮询传输故障、不可见分支复查探测传输故障，均须 blocked-env 而非异常穿透）。

### OCR1-04（medium）StripeTestClient._form 盲重试无 Idempotency-Key — 已修复

- Ruling：finding 成立。首请求已达 Stripe 而响应丢失时，传输层重试会真实创建第二个
  客户，孤儿对象 cleanup 永不删除，与 phases.py:21-24 "Unknown-outcome POSTs are
  never blind retried" 相悖。采用 fixHint 首选方案：每次逻辑调用生成一次
  Idempotency-Key 并放入 Request headers——重试循环复用同一 Request 对象即复用同一
  key，Stripe 重放原始响应而非重复执行创建；保留传输重试能力。
- 处置：`clients.py::_form` headers 增加 `"Idempotency-Key": secrets.token_urlsafe(16)`
  （`secrets` 已 import），docstring 说明重试复用同一 key 的理由。
- RED 证据：基线 `grep -ci idempotency clients.py` → `0`。
- 回归测试：`test_lab.py::TestStripeClient::test_form_calls_carry_a_fresh_idempotency_key`
  （每次逻辑调用携带新 key）、`test_transport_retry_reuses_the_same_idempotency_key`
  （RecordingHandler 新增 `idempotency_key` 记录与 `drop_first_n` 丢首包模拟，
  断言重试两次请求同一 key）。

### OCR1-01（medium）verify_db_watch.py 空 glob 取 [-1] 必然 IndexError — 已修复

- Ruling：finding 成立。目标 TSV 在 git-ignored `runs/` 下，全新克隆/清理后必为空，
  IndexError 属未分类崩溃，无法区分"证据缺失"与"校验失败"。
- 处置：先判空再索引，缺失时打印可操作信息（含路径与 git-ignored 提示）并以独立
  退出码 `sys.exit(2)`（MISSING-EVIDENCE）退出；模块 docstring 补充 0/1/2 退出码契约。
- RED 证据：在基线 worktree（`git worktree add --detach /tmp/issue72-red-check
  0ef4577b6`）运行基线脚本（其 runs/ 天然为空）→ `IndexError: list index out of
  range`（verify_db_watch.py:17）。
- 回归测试：新文件 `docs/plans/issue-72-flow-evidence-74/test_verify_db_watch.py`：
  `test_missing_tsv_exits_2_with_actionable_message`。

### OCR1-02（medium）ok_states_a 空真（vacuous pass）— 已修复

- Ruling：finding 成立。`set() <= {4}` 恒 True，sub_a 行缺失（tag 漂移/未采样）时
  "A 全程 incomplete"静默通过；对照 sub_c 有显式 `sub_c_seen` 守卫，唯 sub-a 缺。
- 处置：`ok_states_a = len(sub_a) > 0 and set(sub_a) <= {4}`，附注释说明存在性守卫
  与 sub_c_seen 同风格。
- RED 证据：基线 `python3 -c "print(set() <= {4})"` → `True`（空真）。
- 回归测试：`test_verify_db_watch.py::test_missing_sub_a_samples_is_check_not_vacuous_pass`
  （只有 -b/-c 行的 TSV → CHECK exit 1）；同文件 `test_good_tsv_still_passes`
  正向对照确保未破坏原判定。

### OCR1-06（low）PHASE_ORDER 与 docstring 保留旧执行顺序 — 已修复

- Ruling：选择"同步修正 + 测试锁死"而非删除：PHASE_ORDER 是可导入的执行顺序契约，
  修好并让测试对照 runner 顺序，比删除更能防止未来分叉（run_lab 的 order 原为函数
  内局部变量，提升为模块级 `PHASE_SEQUENCE` 供测试对照）。
- 处置：
  - `phases.py::PHASE_ORDER` 改为 setup→provider_setup→gate→activate→manual→
    duplicates→retries→decline_control→cleanup，附注释说明 manual 必须后置的原因
    （v1.53.0 3DS gate 发票 API 不可见，manual 探针需要 customer B 的 finalized
    发票）。
  - `run_lab.py` 模块 docstring 顺序改为 `gate -> activate -> manual`；函数内局部
    `order` 提升为模块级 `PHASE_SEQUENCE` 常量并在 `run_experiment` 引用。
- RED 证据：基线 PHASE_ORDER 第 5/6 位为 `phase_manual`/`phase_activate`（旧序）。
- 回归测试：`test_phases.py::TestRunnerContract::test_phase_order_contract_matches_runner_order`
  （PHASE_ORDER 名单 == PHASE_SEQUENCE 名单 + cleanup 收尾 + activate 先于 manual）。

### OCR1-08（low）phase_gate expected 文案与不可见 PASS 路径不符 — 已修复

- Ruling：finding 成立。expected 无条件承诺 "state stable across the
  auth-challenge window" 与发票观测，而两条新 PASS 路径未观测这些项即返回 PASS，
  证据报告出现承诺与观测脱节。按 fixHint 对齐 decline_control/retries 的
  "whenever the API can see it" 式措辞。
- 处置：expected 改写为 "…incomplete with entitlements 404 and holds that state
  across the auth-challenge window (or ends canceled(payment_failed) when the
  charge fails); the gating invoice is open/pending … whenever the API can see
  it (v1.53.0 keeps unsettled gate invoices INVISIBLE)"。
- 回归测试：`TestGatePhase::test_passes_when_invoice_stays_api_invisible_and_incomplete_holds`
  增加 `assertIn("whenever the API can see it", report["expected"])`。

### OCR1-09（low）phase_provider_setup docstring 仍写 chargeDeclined — 已修复

- Ruling：finding 成立。STRIPE_PAYMENT_METHODS 实际 `c →
  pm_card_authenticationRequired`（chargeDeclined 类 token 已不可 attach），docstring
  是映射表现状的直接反例。
- 处置：docstring C 行改为 "C: pm_card_authenticationRequired (negative control:
  off-session charge fails authentication_required)"。
- 回归测试：`TestProviderSetupPhase::test_docstring_matches_the_payment_method_map`
  （含 authenticationRequired、不含 chargeDeclined）。

### OCR1-10（low）fixtures.py 模块头仍按顶层键声明 customers 契约 — 已修复

- Ruling：finding 成立。customer_payload 实际构造 `customer.billing_configuration`
  嵌套且顶层键被静默忽略，模块头与"固化 pinned 契约文档"定位矛盾。
- 处置：模块头该条目改为 `(customer.billing_configuration: payment_provider,
  provider_customer_id, provider_payment_methods; top-level provider keys are
  silently ignored)`。
- 说明：该项为纯文档修正，行为无变化；以修复后的真实流程重放（customers 创建 2xx）
  作为行为不回归的旁证，未单列文案断言测试。

## 测试与验证

### 离线回归

- 命令（与 #74 验证同方式，加上新增测试文件）：
  `cd deploy/lago-lab/payment-activation && python3 -m pytest test_lab.py
  test_phases.py ../../../docs/plans/issue-72-flow-evidence-74/test_verify_db_watch.py -q`
- 结果：`74 passed in 50.58s`（基线 64 + 新增 10：
  test_phases.py +5、test_lab.py +2、test_verify_db_watch.py +3）。

### 真实流程重放（修复改变已验证流程行为 → 按 ask 要求重放）

背景：OCR1-03（decline 判定）、OCR1-04（Stripe 请求头）、OCR1-07（gate 判定映射）
改变了已验证用户流程（run_lab.py 实验链路）的行为，按 ask 重放
`docs/plans/issue-72-flow-evidence-74/` 的可重放脚本，环境按 #74 原方式启动。

环境（与 VERIFY-SUMMARY 同方式）：

- `lab.sh up`：v1.53.0 五镜像 digest 锁定，全部 healthy；
  DB 投影验证轮前按原方式 `docker volume rm weknora-lago-74_lago_{postgres,redis,storage}_data`
  清陈旧卷（旧 run 的软删除残留行会污染观察者 TSV 的 sub-a/b/c 断言）。
- Stripe TEST key 经 `source ~/.zcode/issue72-stripe.env` 注入环境变量，从未回显。

重放记录（证据目录 `docs/plans/issue-72-ocr1-replay/`，git 跟踪；runs/ 为 git-ignored）：

1. 第 1 轮：`run_lab.py --output-dir docs/plans/issue-72-ocr1-replay` →
   9 阶段全 pass，overall verdict: pass，exit 0，secrets scan 11 files 0 hits；
   `verify_ac_assertions.py` → 21/21 ALL PASS（exit 0）。
   decline 走 not-settled 分支（poll_exhausted=true, invoice_api_visible=false），
   gate 走 invoice-invisible 分支（recheck=incomplete），与 #74 原验证同一分支。
2. 第 2 轮（清卷后为 DB 投影验证准备）：provider_setup 因本机到 api.stripe.com 的
   传输故障 blocked（`Stripe API unreachable (URLError)`，51s 内 transport_retries
   用尽）——该场景正是 OCR1-04 所述间歇性传输故障，被既有映射正确归类
   blocked-env（exit 2），非代码回归；本轮未创建订阅，DB 仍干净。
3. 第 3 轮（干净 DB + 并行 db_watch 观察者，采得 516 点 TSV）：
   - `run_lab.py` → 9 阶段全 pass，overall verdict: pass，exit 0，
     secrets scan 13 files 0 hits（真实 Stripe 调用全程携带 Idempotency-Key，
     provider_setup 65.9s 通过，OCR1-04 修复在真实环境工作）；
   - `verify_ac_assertions.py` → 21/21 ALL PASS（exit 0，输出归档
     verify-ac-assertions-output.txt）；
   - 修复版 `verify_db_watch.py` → `DB-WATCH: PASS`（exit 0）：319 个 mid-run 样本，
     sub-a 全程 incomplete(4)、sub-b 4→1(active) 不回退、sub-c 从未 active、
     succeeded payments 峰值恰 1（输出归档 verify-db-watch-output.txt）；
   - 观察者完整 TSV 归档为 verify-db-watch-samples.tsv（516 行含尾段）；对归档
     TSV 复核 mid-run 断言结论一致（a=[4] b=[1,4] c=[4] → PASS）。
   - 提交前对证据目录跑 `sk_test_|rk_test_` 形状扫描 = 0 命中。

环境回收：`lab.sh down`，weknora-lago-74 容器数 0（卷按惯例保留）；#73 主栈
weknora-lago 与 WeKnora-* 开发容器未受影响（全程独立 project 名与端口）。

分支说明：decline_control 本两轮真实运行均走 not-settled 分支（v1.53.0
timeout_hours:0 不会到期取消），OCR1-03 修复的 settled+不可见分支由离线回归测试
`test_canceled_with_api_invisible_terminal_invoice_waives_check` 覆盖
（FakeLago `decline_invoice_terminal_invisible` 旋钮模拟真实 closed 发票不可见行为）；
OCR1-07 的 blocked-env 映射由离线传输故障注入测试覆盖（真实重放第 2 轮的
URLError blocked 是 provider 段既有映射，非 gate 段）。

## 提交

- 提交 message 前缀：`issue-72(ocr-1):`
- 修复文件：deploy/lago-lab/payment-activation/{phases,clients,fixtures,run_lab,
  test_phases,test_lab}.py、docs/plans/issue-72-flow-evidence-74/{verify_db_watch.py,
  test_verify_db_watch.py}；
  记录：docs/plans/issue-72-ocr-fix-log.md；重放证据：docs/plans/issue-72-ocr1-replay/。
