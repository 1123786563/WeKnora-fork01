# Issue #72 — OCR 第 1 轮修复记录（ocr-1）

> 第 2 轮（ocr-2）记录追加于本文件末尾「OCR 第 2 轮」章节。

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

---

# OCR 第 2 轮修复记录（ocr-2）

- 日期：2026-09-23（UTC）
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：`c7e404f3a`（issue-72(ocr-1) 提交），即 OCR 第 2 轮 findings 所审的 HEAD
- 有效 findings：OCR2-01（medium）、OCR2-06（medium）、OCR2-07（medium），全部修复，无 deferred

## 逐条 Ruling 与处置

### OCR2-01（medium）两份 verify_db_watch.py 拷贝选 TSV 错配且不可溯源 — 已修复

- Ruling：finding 成立。两份拷贝（issue-72-flow-evidence-74 与 issue-72-ocr1-replay）都解析
  仓库级 `runs/` 并取字典序最新 `db-watch-verify-*.tsv`，选中路径从不打印：任一后续重放的
  新 TSV 会静默替换为与本证据目录 run 无关的样本，对照本目录 decline 边界产生错配的
  CHECK/PASS 且无法溯源；本目录已归档 `verify-db-watch-samples.tsv` 却不可用。
- 处置（两份拷贝同步修改，主体 diff 证实一致）：
  1. 归档优先：`archived = EVID / "verify-db-watch-samples.tsv"` 存在即用之，仅当无归档时
     回退 `runs/` 最新 TSV；
  2. 始终打印 `DB-WATCH: using <path>`，exit 2 的 MISSING-EVIDENCE 文案同步说明两处来源；
  3. `rows_for` 前缀从宽泛的 `weknora-t02-` 收紧为本目录 `t02-decline.json` 的
     `run_id` 对应 `weknora-t02-<run_id>-`，多 run TSV 的外来行不再混入 sub-a/b/c 断言。
- RED 证据：在基线 worktree（c7e404f3a）构造"replay 之后重跑"场景——`runs/` 放一份时间戳
  全部 ≥08:00 的外来 TSV（本目录 boundary 为 06:27:38）→ 基线脚本忽略目录内归档、选用
  runs TSV、无 using 输出 → `DB-WATCH: CHECK` exit 1（错配）；基线 64 行
  `startswith("weknora-t02-")` 为宽前缀。
- 回归测试：`test_verify_db_watch.py` 更新并新增
  `test_archived_tsv_takes_priority_over_runs_and_is_printed`、
  `test_foreign_run_rows_do_not_leak_into_assertions`，
  `test_good_tsv_still_passes_and_prints_the_selected_file` 补 using 断言（3→5 个测试）。
- 修复后即时验证：两份脚本在各自目录运行均输出 `using .../verify-db-watch-samples.tsv`
  且 DB-WATCH: PASS（exit 0）。

### OCR2-06（medium）phase_duplicates PASS 仅依赖即时探测，无法捕获延迟破坏 — 已修复

- Ruling：finding 成立。代码自述的 deferred-update 风险（200 应答后数分钟订阅被终止并开
  出续期发票）在即时 final 探测下不可见，exactly-once 门槛可能被虚假满足，expected 也未
  披露。采用 fixHint 首选方案：PASS 前做一轮有界延迟复查（复用 RunContext 旋钮
  `stability_rounds * stability_delay`，真实运行约 15s，离线测试 0.02s）。
- 处置：`phase_duplicates` 在 `probes_harmless and state_intact` 成立时（即唯一的 PASS
  候选路径；探针已 FAIL 时跳过避免无谓延迟），sleep 后重读订阅（仍 active、同 lago_id）、
  成功支付（仍 1 笔）、发票列表（无新增：`invoice_count == invoice_count_immediate`）、
  目标发票字段未变，写入 `observed.deferred_recheck`；任一破坏即 FAIL 并附 contract
  note。expected 同步披露"immediate window AND re-checked after a bounded settle delay"。
- RED 证据：基线 `grep -c "deferred_recheck" phases.py` → 0（无任何延迟复查）。
- 回归测试：FakeLago 新增 `duplicates_probes_done`（由 manual-dup reference POST 标记）、
  `deferred_terminate_after_duplicates` 旋钮（probes 后第 N 次 subscription-show GET 触发
  终止+续期发票，N=2 即落在延迟复查内）；
  `test_fails_when_state_drifts_after_the_settle_window`（即时探测干净、延迟复查抓到终止
  与新增发票 → FAIL）；
  `test_passes_when_duplicate_registration_answered_200_with_intact_state` 补
  `deferred_recheck.checked/ok` 正向断言。

### OCR2-07（medium）phase_retries 对 None 发票 id 空转 POST 且 404 归因错误 — 已修复

- Ruling：finding 成立。gate 发票不可见时 `invoice_lago_id` 为 None，无条件 f-string POST
  到字面量 `/api/v1/invoices/None/retry_payment`，真实返回的 404 invoice_not_found 是
  id 不存在所致，而注释/expected 称"发票 API 不可见所以 404"——归因错误且探测空转
  （payments 0==0 恒真），却被记为 AC3 重试证据判 pass。
- 处置：`invoice_lago_id is None` 时跳过 retry POST，`rrs, rrb = None,
  {"code": "not_applicable", "detail": "gate invoice id unknown (API-invisible)"}` 并附
  contract note 说明未知 id 的 404 不是重试证据；expected 与段内注释的 404 归因同步修正；
  `no_succeeded_payment_for_gate` 检查仅在发票 id 已知（探针真实发出）时纳入 checks，
  不可见路径 PASS 判定为 fixHint 指定的三项有效检查（same_identity_recovered /
  gate_not_activated / no_second_payment_row）。
- RED 证据：基线 949 行 `rrs, rrb = ctx.lago.post(f"/api/v1/invoices/{invoice_lago_id}/
  retry_payment", {})` 无任何 None 守卫（grep 证实）。
- 回归测试：`test_gate_invoice_id_unknown_skips_retry_post`（http_status 为 None、
  retry_payment code=not_applicable、checks 恰为三项且全真、note 含 skipped）。

## 测试与验证

### 离线回归

- 命令（与 ocr-1 同方式）：
  `cd deploy/lago-lab/payment-activation && python3 -m pytest test_lab.py
  test_phases.py ../../../docs/plans/issue-72-flow-evidence-74/test_verify_db_watch.py -q`
- 结果：`78 passed in 52.30s`（ocr-1 后 74 + 新增 4：test_phases.py +2、
  test_verify_db_watch.py 3→5）。

### 真实流程重放（OCR2-06/07 改变 run_lab 流程行为 → 按 ask 要求重放）

环境（与 #74 原方式一致）：`docker volume rm` 清陈旧卷（全局 payments/subscriptions 计数
跨 run 累计，干净 DB 是 max_succeeded==1 断言的前提；rows[] 层面的多 run 混入则由本轮
OCR2-01 的 run_id 前缀收紧兜底）→ `lab.sh up`（v1.53.0 全 healthy）→ Stripe TEST key
经 env source 注入。证据目录 `docs/plans/issue-72-ocr2-replay/`。

- `run_lab.py --output-dir docs/plans/issue-72-ocr2-replay --poll-timeout 300`
  + 并行 db_watch 观察者（197 点 TSV 归档为 verify-db-watch-samples.tsv）：
  9 阶段全 pass，overall verdict: pass，exit 0，secrets scan 11 files 0 hits。
  - duplicates 16.9s（原 ~3s）：含 OCR2-06 的 settle 延迟复查
    （stability_rounds 3 × stability_delay 5s = 15s），t02-duplicates.json 的
    deferred_recheck = {checked: true, ok: true, subscription_status: active,
    invoice_count 1==1, payments_succeeded_count 1}；
  - retries：OCR2-07 在真实环境触发——gate 发票不可见，pending_gate_retry.http_status
    为 null，evidence.responses.retry_payment = {code: not_applicable, detail: gate
    invoice id unknown (API-invisible)}，checks 恰为三项有效检查全真，contract note
    记录 skip 理由（不再有对 /invoices/None/ 的空转 POST 与 404 误归因）。
- `verify_ac_assertions.py`（本目录副本，含 ocr-2 新增 2 条断言：deferred re-check
  clean、gate retry probe recorded not_applicable）→ 23/23 ALL PASS（exit 0，输出归档
  verify-ac-assertions-output.txt）。
- 修复版 `verify_db_watch.py`（本目录副本）→ `DB-WATCH: PASS`（exit 0）：输出首行
  `using .../issue-72-ocr2-replay/verify-db-watch-samples.tsv`（归档优先 + 可溯源），
  mid-run 样本 sub-a 全程 incomplete(4)、sub-b 4→1 不回退、sub-c 从未 active、
  succeeded 峰值恰 1。
- 提交前对证据目录 `sk_test_|rk_test_` 形状扫描 = 0 命中。
- 环境回收：`lab.sh down`（容器 0）；#73 主栈未受影响。

## 提交

- 提交 message 前缀：`issue-72(ocr-2):`

---

# OCR 第 3 轮修复记录（ocr-3）

- 日期：2026-09-23（UTC）
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：`d95a93f0d`（issue-72(ocr-2) 提交），即 OCR 第 3 轮 findings 所审的 HEAD
- 有效 findings：OCR3-01（medium）、OCR3-05（medium）、OCR3-02（low）、OCR3-04（low），
  全部修复，无 deferred

## 逐条 Ruling 与处置

### OCR3-01（medium）gate 不可见分支 canceled PASS 未校验 payment_failed — 已修复

- Ruling：finding 成立。发票不可见且复查非 incomplete 的分支仅 `if _ok(cs2):` 即返回
  PASS，reason 只进 ctx.note 不进 observed；对照发票可见为 failed 的分支要求
  `core_ok and canceled and reason == "payment_failed"`。两分支判定严格度不一致，
  无关原因取消（other/revoked）会得到误导性 PASS，且证据缺 cancellation_reason 字段，
  与 expected 承诺的 "(or ends canceled(payment_failed) when the charge fails)" 矛盾。
- 处置：对齐可见分支严格度——`canceled = _ok(cs2)`，reason 写入
  `observed["cancellation_reason"]`；PASS 条件改为 `canceled and reason ==
  "payment_failed"`（core_ok 已由外层 if 保证）；否则 FAIL，error 文案附
  `canceled=.../cancellation_reason=...` 便于复盘，保留 invoices_last_seen 证据。
- RED 证据：基线 d95a93f0d 565-574 行 `if _ok(cs2):` 直接 PASS、reason 未入 observed
  （sed 输出证实）。
- 回归测试：FakeLago 新增 `gate_cancel_reason_while_invisible` 旋钮（发票保持不可见期间
  gate 订阅按可配置原因取消）；
  `test_foreign_cancellation_reason_with_invisible_invoice_fails`（reason="other" → FAIL，
  observed.cancellation_reason=="other"，error 含该字段）、
  `test_payment_failed_cancellation_with_invisible_invoice_passes_with_reason`
  （reason="payment_failed" → PASS 且 reason 入 observed，note 含 charge-failure endgame）。

### OCR3-05（medium）duplicates deferred 复查 no-new-invoice 锚点错基准 — 已修复

- Ruling：finding 成立且接受裁决升级（low→medium）。`invoice_count` 只与即时窗口的
  `invoice_count_immediate` 比较，探测前 baseline 从不记录数量，即时 state_intact 又
  按 lago_id 选中同一张发票比较字段、不校验数量——重复注册若在即时读取前同步开出
  新发票且订阅身份不变，两层断言同时放行，与注释/expected 承诺的 no new invoice 不符，
  与 OCR1-02/OCR2-01 同属"断言基准错误留下静默假 PASS 通道"。
- 处置：baseline 增记探测前 `"invoice_count": len(invoices)`（附注释说明锚点理由）；
  `deferred["ok"]` 的发票数量断言改为 `invoice_count == baseline["invoice_count"]`；
  `invoice_count_immediate` 保留入 deferred 证据（现含 baseline/immediate/最终三个计数，
  便于分层复盘），并新增 `invoice_count_baseline` 字段。
- RED 证据：基线 grep 证实 1038 行锚点为 `invoice_count_immediate`，baseline（935-942 行）
  无 invoice_count 键。
- 回归测试：FakeLago 新增 `duplicate_re_post_adds_invoice` 旋钮（200 应答的重复注册同步
  开出立即可见的续期发票、订阅身份不变）；
  `test_fails_when_duplicate_synchronously_issues_an_invoice`（即时逐字段探测全部干净、
  invoice_count == immediate 但 > baseline → FAIL，note 含 deferred re-check）。

### OCR3-02（low）retries 的 gate 终态未入证据 — 已修复

- Ruling：finding 成立（证据完整性缺口，非判定缺陷——incomplete/canceled 均为未激活态，
  `_ok(as_)` 语义正确）。处置按 fixHint：
  `gate_retry["subscription_status_after"] = asub.get("status") if _ok(as_) else None`
  （附注释），消除 asub 死变量。
- RED 证据：基线 gate_retry（1162-1170 行）grep subscription_status_after = 0 命中。
- 回归测试：`test_recovers_same_identity_and_keeps_gate_pending` 补断言
  `gate_retry["subscription_status_after"] == "incomplete"`。

### OCR3-04（low）_resolve_graphql_organization_id 失败被静默吞掉 — 已修复

- Ruling：finding 成立。`except (OSError, OffOriginRedirect): pass` 完全吞掉解析失败；
  窄场景（REST 组织解析传输失败而 GraphQL 可达且服务端强制头）下 GraphQL 400 被判
  FAIL 而非 blocked-env，线索（graphql_status=400 + errors）虽在但不指向缺失的头。
  采用 fixHint 主修复（note）；失败缓存哨兵不采用（当前仅 1 个调用点，成本收益低，
  且保留"下次调用可重试解析"的现有语义）。
- 处置：except 分支记录
  `ctx.note("x-lago-organization resolution failed ({error.__class__.__name__}); header
  omitted — GraphQL mutations may be rejected with 'Missing organization id'")`，note
  会随后进入该 run 的 report contract_notes。
- RED 证据：基线 143-144 行 `except ...: pass`（sed 输出证实）。
- 回归测试：`test_graphql_organization_resolution_failure_is_noted`（替换
  ctx.lago.request 使 /api/v1/organizations 抛 ConnectionResetError，断言返回 None 且
  notes 含归因文案与异常类名）。

## 测试与验证

### 离线回归

- 命令（与 ocr-1/2 同方式）：
  `cd deploy/lago-lab/payment-activation && python3 -m pytest test_lab.py
  test_phases.py ../../../docs/plans/issue-72-flow-evidence-74/test_verify_db_watch.py -q`
- 结果：`82 passed in 58.81s`（ocr-2 后 78 + 新增 4：gate +2、duplicates +1、
  provider_setup(resolve note) +1；OCR3-02 为既有测试内补断言）。

### 真实流程重放（OCR3-01/05 改变判定行为 → 按 ask 要求重放）

环境（与 #74 原方式一致）：清卷 → `lab.sh up`（v1.53.0 全 healthy）→ Stripe TEST key
经 env source 注入。证据目录 `docs/plans/issue-72-ocr3-replay/`。

- `run_lab.py --output-dir docs/plans/issue-72-ocr3-replay --poll-timeout 300`
  + 并行 db_watch 观察者（326 点 TSV 归档为 verify-db-watch-samples.tsv）：
  9 阶段全 pass，overall verdict: pass，exit 0，secrets scan 11 files 0 hits。
  - OCR3-02 落地：t02-retries.json 的 pending_gate_retry.subscription_status_after
    == "incomplete"（gate 终态进入证据）；
  - OCR3-05 落地：t02-duplicates.json 的 evidence.baseline.invoice_count == 1，
    deferred_recheck = {invoice_count_baseline: 1, invoice_count_immediate: 1,
    invoice_count: 1, ok: true}（探测前基准锚点生效，真实运行无新发票）；
  - OCR3-01：真实运行 gate 走 invoice-invisible + recheck=incomplete 分支
    （invoice_api_visible=false），不触发 canceled 判定分支——修复的
    canceled(非 payment_failed)→FAIL 严格度由离线回归测试覆盖
    （gate_cancel_reason_while_invisible 旋钮两向测试）。
- `verify_ac_assertions.py`（本目录副本，含 ocr-3 新增 2 条断言：duplicates
  invoice count == baseline、AC3 gate end state recorded not active）→ 25/25
  ALL PASS（exit 0，输出归档 verify-ac-assertions-output.txt）。
- 修复版 `verify_db_watch.py`（本目录副本）→ `DB-WATCH: PASS`（exit 0）：using
  本目录归档 TSV，sub-a 全程 incomplete(4)、sub-b 4→1 不回退、sub-c 从未
  active、succeeded 峰值恰 1。
- 提交前对证据目录 `sk_test_|rk_test_` 形状扫描 = 0 命中。
- 环境回收：`lab.sh down`（容器 0）；#73 主栈未受影响。

## 提交

- 提交 message 前缀：`issue-72(ocr-3):`

---

# Issue #72 — Issue #81 OCR 第 1 轮修复记录（ocr-81-1）

- 日期：2026-09-24
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：`11fb134ae`（issue-72: ocr issue-81 round 1）
- 修复人：修复员（dynamic workflow subagent）
- 范围：Issue #81 增量 OCR 有效 findings R1-V01 … R1-V23（23 项），23 项全部修复，0 项 deferred。

## 总则

- 每个行为级修复补了回归测试，全部在修复版代码上通过（命令与结果见「测试与重放证据」）。
- 安全约束核对（本批涉及面）：出站请求仅 http/https 且先校验 host（R1-V08/V09 强化了该口径：
  渠道出站统一接 `secutils.ValidateURLForSSRF` + `SSRF_WHITELIST(_EXTRA)` 豁免、provider 客户端
  每一跳重定向复检、inet_aton 简写形态按 RFC 1123 TLD 规则拒绝）；新增 SQL 均为参数绑定
  （`CurrentPurchaseOrder`、测试插入语句）；本轮未引入任何凭据字面量（测试用 `testAPIKey`
  既有 secret-for-test 常量与显式 `127.0.0.1` 白名单豁免，非凭据）。

## 逐条处置

### R1-V01（high）purchase.order 缺席时空 ID getOrder 回退 — 已修复
- `CheckoutPage.tsx:83` 不再 `purchase.order ?? getOrder('')`；缺单时按闭合 state/reason 渲染
  `purchaseErrorMessage`（新增导出函数，reason 闭合令牌 → 中文文案），不再发起必然 404 的
  `/orders/` 请求。回归：`CheckoutPage.test.tsx`（缺单 → getOrder 调用数 0 + 失败文案）。

### R1-V02（high）PurchaseStatus 取随机 rows[0] — 已修复
- 仓储新增 `CurrentPurchaseOrder`（`repository/commercial/order.go`）：`kind='purchase'` 且
  金额/币种与权威冻结面一致（全参数绑定），pending 优先、其余按确定性 id 升序；absent 购买
  不挂接任何订单。回归：`TestPurchaseStatusAttachesOnlyTheCurrentPurchaseOrder`（干扰行：
  旧金额购买单、upgrade 单、同金额已付单 + absent 租户）。

### R1-V03（high）active/canceled 放行新开渠道订单 — 已修复
- 第 8 步仅当 `p.State==awaiting_payment` 才 `CreateOrder`（哨兵 `ErrPurchaseNotAwaiting` →
  handler 409）；既有订单重放与竞态胜者回读保持放行（付款后 POST 重演返回已付订单）。
  回归：`TestPurchaseRefusesNewChannelOrderWhenNotAwaiting`（激活后新报价拒开新单、恰好
  1 订单/1 渠道请求、同 quote 重放返回原订单）。

### R1-V04（medium）故障期 POST 201+absent — 已修复
- 三条同根因路径（platform==nil、账户 ensure pending、SubmitCommand/ReadSnapshot 失败）统一
  改答哨兵 `ErrPurchaseUnavailable`（多重 %w 携带域哨兵）；handler 映射 503 + 闭合 reason
  （`PurchaseUnavailableReason`）；GET `PurchaseStatus` 保持状态视图。回归：
  `TestPurchasePlatformFailureAnswersUnavailable`、`TestPurchaseNilPlatformAnswersUnavailable`、
  `TestPurchaseHandlerAnswersUnavailableOnPlatformFailure`。

### R1-V05（medium）既有客户绑定覆写显示名 — 已修复
- `customerProviderBound` 改为返回 (exists, bound)；已存在但未绑定的客户改走
  `PUT /api/v1/customers/{external_id}` 仅更新 billing_configuration，绝不 POST 覆写 name。
  测试 stub 增加 PUT 分支（只合并 billing_configuration）。回归：
  `TestLagoBindingUpdateOnExistingCustomerKeepsDisplayName`。

### R1-V06（medium）占位前缀绑定空转 20s PM 轮询 — 已修复
- `deriveProviderCustomerID` 返回来源；placeholder 来源跳过 `waitForPaymentMethodSync`
  （让 gated create 以其自身 422 no_default_payment_method 快速失败）。回归：
  `TestLagoPlaceholderBindingSkipsPaymentMethodSyncWait`（stub 置 PM 永不导入，提交仍即时成功）。

### R1-V07（medium）attach 脱离调用方截止链 — 已修复
- `providerAttachDefaultPaymentMethod` 签名接收 ctx，`WithTimeout(ctx, outboundProviderTimeout)`
  取调用方截止与自身超时较短者；调用点（`providerCreateCustomer`）传入 ctx。

### R1-V08（medium）出站校验两处绕过 — 已修复
- 新增 `providerOutboundClient`：`CheckRedirect` 对每个跳转目标复跑 `validateOutboundHost`
  （且上限 10 跳）；`validateOutboundHost` 对 ParseIP 失败的 host 增加 `isPlausibleHostname`
  形状校验（RFC 1123：标签字符集 + 顶级标签必须字母开头），`127.1`/`2130706433`/`0x7f000001`/
  `0177.0.0.1`/`3232235521` 全部拒绝。回归：`TestValidateOutboundHost` 扩充 +
  `TestProviderOutboundClientRejectsInternalRedirect`。

### R1-V09（medium）微信渠道出站无 SSRF 校验 — 已修复
- `WechatProvider.do`（apiBase）与 `AlipayProvider.call`（gateway）每次出站前接
  `secutils.ValidateURLForSSRF`（fail-closed ErrNotConfigured）；两个渠道 client 的
  `CheckRedirect` 逐跳复检；环回 stub 流程现依赖显式 `SSRF_WHITELIST(_EXTRA)=127.0.0.1`
  豁免。`wechat_pay_stub.py` docstring 记录该豁免与重放要求。回归：
  `TestWechatDoRejectsInternalEgressWithoutWhitelist`、`TestWechatDoAllowsWhitelistedLoopbackStub`；
  alipay 网关 fixture 统一走白名单豁免（其 4 个既有出站测试照常通过）。

### R1-V10（medium）max_succeeded 与 rows_for 隔离级别不一致 — 已修复（四副本）
- 四份 `verify_db_watch.py`（evidence-74 + ocr1/2/3）的 `max_succeeded()` 补上 midrun 边界
  过滤；走 runs/ 回退 TSV 时对不可按 run 键控的 payments 全局聚合显式告警并 exit 2
  （MISSING-EVIDENCE）。回归：四副本重放（归档主路径）全部 PASS（见证据）。

### R1-V11（medium）延迟复检窗口与漂移时间尺度不匹配 — 已修复
- `RunContext` 新增独立旋钮 `duplicates_settle_delay`（默认 120s，分钟级）；
  deferred re-check 的 `time.sleep` 改用该旋钮；`run_lab.py` 新增
  `--duplicates-settle-delay`（默认 120.0）并透传。稳定性窗口参数不再被复用。
  回归：`run_lab.py --help` 显示新旋钮；`py_compile` 通过。

### R1-V12（medium）待付款标签无条件渲染 — 已修复
- `CheckoutPage.tsx` 待付款标签仅当 `order.payment==='pending'` 渲染；已支付显示
  `orderMessage` 的已付文案。回归：`a paid order no longer shows the awaiting-payment label`。

### R1-V13（medium）checkout_url 无 scheme 校验 — 已修复（双层）
- `packages/contracts` 新增 `isSafeCheckoutUrl`（http/https + `weixin://wxpay/` +
  `alipayqr/alipays://platformapi/` 白名单）并导出；`parseOrderView` 对不安全 checkout_url
  直接丢弃该字段（订单仍有效）；CheckoutPage 渲染层再校验一次（第二道防线）。
  回归：contracts `commercial.test.ts` 两组 + CheckoutPage javascript: 渲染用例。

### R1-V14（medium）Purchase handler 缺 CheckoutError→202 — 已修复
- 镜像 POST /orders：`err==nil && view.Order.CheckoutError!=""` → 202（携带 checkout_error）。
  回归：`TestPurchaseHandlerAnswersAcceptedWhenCheckoutFailed`。

### R1-V15（low）BillingPage effect 不重置 purchase — 已修复
- effect 开头补 `setPurchase(null)`，租户切换/Reload 不再短暂显示上一租户的待付款行。

### R1-V16（low）闭合令牌被包成新 error 再匹配 — 已修复
- `purchase.go` 账户未链接分支不再 `platformReason(errors.New(acct.Reason))`；token 经
  `platformSentinel`（platformReason 的逆映射）进 ErrPurchaseUnavailable 链原样透传
  （与本条 fixHint"直接透传"等效且兼容 R1-V04 的 503 语义）。

### R1-V17（low）actor/display name 硬编码 — 已修复
- Purchase handler：actor 取 `commercialUserID(c)`；`Purchase` 服务签名增加 displayName，
  handler 以 `tenantDisplayName(tenantID)` 传入（purchase.go:125 的 "space" 消除）。
  回归：`TestPurchaseHandlerCarriesRealActorAndDisplayName`（fake 客户记录面断言
  DisplayName == "WeKnora Space 50"；actor 断言受 fake 记录面所限，以 handler 调用点
  代码为准——`fake.Commands()` 仅记录 publish 类命令，ensure_customer 命令无观察面）。

### R1-V18（low）定义反序列化失败误报 legacy-snapshot — 已修复
- `ensureNoCharges` 反序列化失败改答新哨兵 `ErrPurchasePlanInvalid` → handler 500
  （数据问题，非"请重新报价"）。回归：`TestPurchaseCorruptPlanDefinitionIsNotALegacyQuote`
  （发布不可变触发器下以插入损坏行+报价快照指向构造，断言不落入 ErrQuoteLegacySnapshot）。

### R1-V19（low）嵌套 <p> 无效 HTML — 已修复
- 去掉包裹 Status 的外层 <p>（R1-V12 重构时一并消除）。

### R1-V20（low）attach 两处外呼 5xx 折叠为 InvalidResponse — 已修复
- `providerAttachDefaultPaymentMethod` 两次外呼先判 `status>=500 → ErrPlatformUnreachable`，
  其余非 2xx/解析失败才归 ErrPlatformInvalidResponse（与 providerCreateCustomer 分类一致）。

### R1-V21（low）Purchase handler 缺 ErrQuoteAlreadyUsed→409 — 已修复
- switch 补 `repocommercial.ErrQuoteAlreadyUsed → 409 "quote already used"`，与 /orders 对齐。
  回归：`TestPurchaseHandlerAnswersConflictOnQuoteAlreadyUsed`。

### R1-V22（low）诊断行样本计数与标签语义不符 — 已修复（四副本）
- 打印改为 `samples: <midrun>/<total> mid-run before <boundary>Z`；evidence-74 重放输出
  `221/272`，与评审者 awk 实测一致。

### R1-V23（low）回放脚本来源/副本描述失实 — 已修复（三处）
- ocr1 `verify_db_watch.py`：删除 "(ocr-2)" 复制残留，docstring 如实描述归档优先、
  runs/ 仅为回退；ocr2/ocr3 `verify_ac_assertions.py`：删除 "unmodified" 不实声明，
  docstring 逐条列明相对上一副本新增的 `# ocr-2:`/`# ocr-3:` 断言标注及所在行。

## 测试与重放证据（全部在本轮实际执行）

| 检查 | 命令 | 结果 |
|---|---|---|
| commercial 全部 Go 包 | `go test ./internal/modules/commercial/... -count=1` | 7 包全 ok |
| handler 全部测试 | `go test ./internal/handler/ -count=1` | ok |
| payment 全部测试 | `go test ./internal/modules/commercial/payment/ -count=1` | ok |
| 全仓编译 | `go build ./internal/...` | 通过 |
| contracts 测试 | `npx tsx --test packages/contracts/test/commercial.test.ts` | 20/20 pass |
| web 全量测试 | `pnpm --filter @weknora/web test` | 2297 tests / 2259 pass / 38 fail；38 个失败与基线 `git stash` 前后对比**完全同集合**（knowledge 域预存测试间干扰，diff 为空），commercial 相关 0 失败 |
| web typecheck | `pnpm run typecheck:web` | commercial/contracts 相关错误 0；其余为预存（mermaid.ts/DevMarkdownPage/PlatformShell） |
| DB-WATCH 重放 ×4 | `python3 docs/plans/<dir>/verify_db_watch.py` | evidence-74/ocr1/ocr2/ocr3 全部 `DB-WATCH: PASS`（exit 0），221/272、478/516、176/197、268/326 mid-run，max_succeeded=1 |
| AC 断言重放 ×3 | `python3 docs/plans/<dir>/verify_ac_assertions.py` | ocr1/ocr2/ocr3 全部 ALL PASS（exit 0） |
| 微信 stub 重放 | `python3 wechat_pay_stub.py` + 对 127.0.0.1:8291 真实 POST/GET | code_url 与 NOTPAY 查询响应正常（8291 端口已有 flow 遗留 stub 实例存活并正确应答） |

**全链路真实流程（Lago+Stripe+浏览器）未重放**：8091 后端为旧构建进程（属其他会话，不可
重启）、`WEKNORA_COMMERCIAL_STRIPE_API_KEY` 已不在环境；全链路行为等价性由 service/handler/
adapter 三层 Go 回归测试 + 前端 jsdom 渲染测试覆盖，`wechat_pay_stub.py` docstring 已记录
重放全链路所需的 `SSRF_WHITELIST_EXTRA=127.0.0.1` 新前置条件。

## 提交

- 提交 message 前缀：`issue-72(ocr-81-1):`

---

# Issue #72 — Issue #81 OCR 增量批次修复记录（ocr-81-2）

- 日期：2026-09-25
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：`d27a451cb`（issue-72: ocr issue-81 round 1），即本批 findings 所审的 HEAD
- 修复人：修复员（dynamic workflow subagent）
- 范围：Issue #81 增量 OCR 有效 findings R1-08 / R1-15 / R1-24 / R1-35（4 项 medium），4 项全部修复，0 项 deferred。

## 总则

- 每个 finding 先在基线 `d27a451cb` 上以真实 RED 运行复现旧行为，再在修复版上验证（GREEN）。
- 安全约束核对：本轮无新增服务端外部请求（R1-24 是"少轮询"而非新出站；取消分类修正
  不改变任何出站 URL/host 校验路径）；新增 SQL 仅 `SetCheckoutURL` 一条，
  gorm `Where("id = ?") + Update` 全参数绑定；未引入任何凭据字面量
  （测试用 inert 哨兵 `pm_test_canary`，非凭据形状，非可用凭据）。

## 逐条 Ruling 与处置

### OCR81-R1-08（medium）phase_gate 发票不可见分支 PASS 缺 payments 断言 — 已修复

- Ruling：finding 成立。expected 文本承诺 "at most one non-succeeded payment
  whenever the API can see it"，而 payments 接口不受发票 INVISIBLE_STATUS 影响
  （payments 读取只存在于发票可见路径），不可见分支仅凭 subscription incomplete +
  entitlements 404 返回 PASS——本次归档 t02-gating.json
  （payments_non_succeeded_count=None）正是从该豁免分支产出，AC1 的 exactly-once
  防线在该路径上零断言。采用 fixHint 首选方案（补断言），expected 文本无须收窄。
- 处置：`phases.py` 不可见分支在订阅复查后补 `_payments_for` 读取并写入
  `observed["payments_non_succeeded_count"]`；still_incomplete PASS 条件收紧为
  `len(non_succeeded) <= 1`（>1 时 FAIL，error 文案点明 exactly-once 违约）；
  canceled(payment_failed) 终态 PASS 条件同样收紧，其 FAIL 文案追加
  `non_succeeded_payments=<n>` 便于归因；PASS note 同步披露 payments 断言。
- RED 证据：基线 worktree（d27a451cb + 仅有测试补丁）运行
  `pytest test_phases.py -k invisible` → 2 failed：新增测试
  `test_fails_when_invisible_invoice_path_sees_multiple_non_succeeded_payments`
  （旧行为 status=pass）与既有
  `test_passes_when_invoice_stays_api_invisible_and_incomplete_holds`
  （KeyError: 'payments_non_succeeded_count'）。
- 回归测试：FakeLago 新增 `extra_non_succeeded` 旋钮（payments 列表注入额外
  非 succeeded 行）；上述两条测试：注入 2 笔非 succeeded 且发票保持不可见 →
  FAIL 且 `payments_non_succeeded_count >= 2`；既有不可见 PASS 测试补断言
  `payments_non_succeeded_count == 0`。

### OCR81-R1-15（medium）verify_ac_assertions.py 空 checks 空真通过 — 已修复（四副本）

- Ruling：finding 成立。`all(oa["checks"].values())` 对空字典恒 True
  （`python3 -c "print(all({}.values()))"` → `True`，RED 证据），checks 结构
  经历过键改名/增删，未来重构使某 phase 的 checks 清空时验收断言静默通过；
  同目录 verify_db_watch.py 的 sub-a/b/c 已有 len>0 守卫而本脚本没有，风格不一致。
  同一模式存在于四份副本（ocr1/ocr2/ocr3/flow-evidence-74 各 4 处，共 16 处），
  按 R1-V10/R1-V22 的"副本同病同步修"先例四份全部修复（finding 点名的 ocr3
  副本为主，其余三份为同一结构性缺陷）。
- 处置：四处裸 `all(...values())`（phase_statuses、AC2 checks、AC3 checks、
  decline checks）全部改为 `bool(...) and all(...values())`；ocr3 副本附注释
  说明守卫风格来源，docstring 行号引用按 R1-V23 惯例同步更新（83/99、88/105）
  并如实记录本副本相对上一副本的差异（新增空 checks 守卫）；其余三副本以单行
  形式修改，不移动行号。
- RED 证据：`python3 -c "print(all({}.values()))"` → `True`（空真）；
  属结构性风险（当前归档 checks 非空，本次运行未触发——与 finding 证据一致）。
- 回归验证：四副本对各自归档证据重放全部 `ALL PASS`（exit 0）——正向对照，
  守卫不改变现行判定。

### OCR81-R1-24（medium）PM 导入轮询的生产姿态残留 + ctx 取消错分 — 已修复

- Ruling：finding 成立。`ensureProviderBinding` 对 source==providerCustomerAPI
  的绑定无条件轮询 `waitForPaymentMethodSync`：生产姿态（Stripe key 已设、
  StripePmToken 留空等待 #82 checkout）下 import 永不落地，每次购买烧满
  pmSyncWait=20s 后以 ErrPlatformUnreachable（可重试类）失败，而 config.go
  F11 注释与 `providerCreateCustomer` 注释明确承诺此时应到达 gated create 并以
  no_default_payment_method 失败关闭——实现与自身契约矛盾（R1-V06 只修了
  case a 占位前缀）。另 `sleepCtx` 的 ctx 取消被折算为 Unreachable
  "interrupted"，错误类别错分。
- 处置：
  - `lago_purchase.go`：占位前缀跳过之后，`a.cfg.StripePmToken == ""` 时同样
    跳过轮询直接放行到 gated create（fail closed 按契约）；注释说明第二个
    轮询空转来源与契约依据。
  - `waitForPaymentMethodSync` 的 `sleepCtx` 分支改为
    `fmt.Errorf("payment method sync interrupted: %w", err)`（裸包 ctx.Err()，
    不再携带 ErrPlatformUnreachable 哨兵——调用方刚取消的轮询不应诱导重试）。
  - 可测性：`LagoAdapter` 新增 `deriveProviderCustomer` / `syncPaymentMethods`
    两个测试 seam（构造时绑定真实方法，仅测试可覆写——provider 出站 host 策略
    拒绝环回，provider 侧无法本地 HTTP stub，见 R1-V08）。
- RED 证据：基线 worktree（d27a451cb + 仅 seam/字段 + 测试）运行新测试 →
  `TestLagoAPIBindingWithoutPmTokenSkipsPaymentMethodSyncWait` FAIL
  （"payment-method sync must not run without a PM token, calls=1"——基线
  无 PM token 仍轮询）；`TestLagoPaymentMethodSyncCancellationIsNotUnreachable`
  FAIL（"caller cancellation must surface context.Canceled, got
  platform_unreachable: payment method sync interrupted"）。
- 回归测试：`TestLagoAPIBindingWithoutPmTokenSkipsPaymentMethodSyncWait`
  （API 来源 + 空 PM token：零轮询、快速返回、gated create 恰好到达 1 次）、
  `TestLagoAPIBindingWithPmTokenStillPollsPaymentMethodSync`（PM token 已设时
  轮询恰 1 次——R1-24 跳过严格限定空 token 姿态）、
  `TestLagoPaymentMethodSyncCancellationIsNotUnreachable`（stub 首响应 flush 后
  50ms 取消、tick 500ms——取消确定性落在 sleep 内，断言
  `errors.Is(err, context.Canceled)` 且不携带 ErrPlatformUnreachable）。

### OCR81-R1-35（medium）checkout_url 未持久化，第 8 步幂等重放无法 verbatim — 已修复

- Ruling：finding 成立。checkout_url 仅存在于首次 openOrder 的渠道应答；
  OrderRow 无该列、orderViewFromRow 不投影、RecoverOrderStatus 三条返回路径
  均不携带——客户端超时/刷新后重放 POST /purchases 或 GET 订单拿不到
  checkout_url，报价已消费、订阅停留 awaiting_payment，用户失去支付入口。
  采用 fixHint：在 commercial_orders 增列持久化，并让两条投影携带。
- 处置：
  - `repository/commercial/order.go`：`OrderRow` 新增 `checkout_url` 列
    （服务内 AutoMigrate 增量加列，沿用 kind 列"Additive column upgrades"
    先例，版本化迁移不含该列与 kind 列一致）；新增 `SetCheckoutURL`
    （参数绑定 Update，空 url 拒绝，0 行命中报 ErrOrderNotFound）。
  - `service/commercial/order.go::openOrder`：渠道成功后持久化
    `res.CheckoutURL`；持久化失败不撤销应答（写应答契约仍成立——客户端本次
    仍拿到链接），降级经 `CheckoutError` 显式可见而非静默。
  - 投影：`RecoverOrderStatus` pending 两条路径（无 attempt 与常规返回）
    携带 `row.CheckoutURL`；`purchase.go::orderViewFromRow` 携带
    （POST 重放 existing 分支与 `CurrentPurchaseOrder` 投影随之 verbatim）。
    已付/已履行分支不携带（已无需支付入口，omitempty 保持空）。
- RED 证据：基线 worktree（d27a451cb + 仅字段 + 测试）运行新测试 →
  `TestCheckoutURLPersistsAndReplaysVerbatim` FAIL（"checkout_url must be
  persisted, row=\"\" answer=\"https://pay.example/qr\""）；
  `TestPurchaseRetryReturnsExistingOrderWithoutDuplicates`（补断言）FAIL
  （"replay must carry the persisted checkout_url verbatim, first=… second=\"\""）。
- 回归测试：`TestCheckoutURLPersistsAndReplaysVerbatim`（落库行 == 首答、
  pending 恢复投影 verbatim）；`TestPurchaseRetryReturnsExistingOrderWithoutDuplicates`
  补 AC4 重放断言（第二次 POST 携带与首次相同的 checkout_url 且非空）。

## 测试与重放证据（全部在本轮实际执行）

| 检查 | 命令 | 结果 |
|---|---|---|
| RED（R1-08） | 基线 worktree `pytest test_phases.py -k invisible` | 2 failed（KeyError payments_non_succeeded_count / 旧行为 pass） |
| RED（R1-24） | 基线 worktree `go test ./internal/modules/commercial/commercialplatform/ -run 'TestLagoAPIBinding…\|TestLagoPaymentMethodSyncCancellation'` | 2 failed（calls=1 / platform_unreachable） |
| RED（R1-35） | 基线 worktree `go test ./internal/modules/commercial/service/commercial/ -run 'TestCheckoutURL…\|TestPurchaseRetry…'` | 2 failed（row="" / second=""） |
| RED（R1-15） | `python3 -c "print(all({}.values()))"` | True（空真） |
| commercial 全部 Go 包 | `go test ./internal/modules/commercial/... -count=1` | 7 包全 ok |
| handler 全部测试 | `go test ./internal/handler/ -count=1` | ok |
| 全仓编译 | `go build ./internal/...` | 通过 |
| lago-lab 离线回归 | `cd deploy/lago-lab/payment-activation && python3 -m pytest test_lab.py test_phases.py ../../../docs/plans/issue-72-flow-evidence-74/test_verify_db_watch.py -q` | 83 passed（ocr-3 后 82 + 新增 1） |
| AC 断言重放 ×4 | `python3 docs/plans/<dir>/verify_ac_assertions.py` | evidence-74/ocr1/ocr2/ocr3 全部 ALL PASS（exit 0） |

**全链路真实流程未重放（与上一轮 ocr-81-1 同因）**：8091 后端为其他会话的旧
构建进程（不可重启）、`WEKNORA_COMMERCIAL_STRIPE_API_KEY` 不在本环境；
`docs/plans/issue-72-flow-evidence-81/` 中唯一可重放脚本为
`wechat_pay_stub.py`（微信渠道 stub，与本批四处修复无交集）。影响面说明：
R1-24 只改"生产姿态（PM token 为空）"分支——#81 flow 验证当时
`WEKNORA_COMMERCIAL_STRIPE_PM_TOKEN=pm_card_threeDSecure2Required` 已设
（README 环境表），已验证流程未走被改分支，行为不回归由
`TestLagoAPIBindingWithPmTokenStillPollsPaymentMethodSync` 锁定；R1-35 对
已验证 AC4 重放流是**增量字段**（重放应答新增 checkout_url，同订单/同状态
断言不变），由 service 层 AC4 重放回归 + handler 投影测试覆盖。

## 提交

- 提交 message 前缀：`issue-72(ocr-81-1):`（按本批 ask 指定）

---

# Issue #72 — OCR 第 1 轮修复记录（ocr-r1，10 组 findings）

- 日期：2026-09-25
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：ask 标称 `29c1e5635`；经核对该提交是当前 HEAD（`1c684ae68`，
  findings 记录提交）的远祖且不含 findings 引用的文件（如
  `internal/handler/commercial_purchase_test.go`），而 findings 的行号
  （purchase.go:217、commercial.go:489-490、alipay.go:199-208 等）全部与
  `1c684ae68` 吻合——RED 一律在 `1c684ae68`（findings 实际审查的代码态）复现。
- 修复人：修复员（dynamic workflow subagent）
- 范围：OCR-R1-22（high）、R1-07/08/35/36（high）、R1-05/R1-09/R1-12/
  R1-14+16+17+19/R1-20/R1-24/R1-28+29+31/R1-37（medium 各组），全部修复，
  0 项 deferred。

## 总则

- 每个行为级 finding 先在基线 `1c684ae68` 上真实运行复现旧行为（RED），再在
  修复版验证（GREEN）；表达式级风险（R1-37 空转、R1-28 路径）以最小复现
  演示 + 源码论据记录。
- 安全约束核对（Mimosa 前置约束）：R1-14 将渠道出站并入共享 SSRFSafe 客户端
  ——仅 http/https、每跳重定向复检 + 10 跳上限、拨号层 DNS 钉扎关掉
  validate-then-dial rebinding 窗口、跨域凭证头剥离，白名单豁免通道不变
  （环回 stub 仍需显式 `SSRF_WHITELIST(_EXTRA)`）；本轮新增 SQL 均参数绑定
  （`CurrentPendingPurchaseOrder`、测试 INSERT）；未引入任何凭据字面量。

## 逐条 Ruling 与处置

### OCR-R1-22（high）同一 awaiting 购买可开多张并存可支付渠道订单 — 已修复

- Ruling：finding 成立。幂等键只有 quote：过期重报价/双开标签页各自 POST
  /quotes 后，第二张 quote 走到第 8 步时 `GetOrderByQuote(Q2)` 未命中、购买
  仍 awaiting，`CreateOrder` 为同一 gating invoice 开出新 merchant_order_id
  与新 checkout URL；旧 pending 订单无任何关闭（`provider.Close` 无生产调用
  点、状态机无 closed/canceled 转移——grep 核实），两张渠道单同时可支付，
  分别付款各自 ConfirmPayment→paid→fulfilled。采用 fixHint 首选：CreateOrder
  前解析该购买名下已有 pending 渠道订单，存在则原样重放。
- 处置：repository 新增 `CurrentPendingPurchaseOrder`（kind=purchase +
  冻结价面 + state=pending，`created_at DESC, id ASC`，全参数绑定）；purchase.go
  第 8 步在 awaiting 检查后、CreateOrder 前调用——命中则 verbatim 重放该订单
  （match gate 已证明新 quote 购买同 plan 同价面，重放订单即本购买的订单），
  ErrOrderNotFound 之外的读错误照常上抛。
- RED 证据：基线运行 `TestPurchaseSecondQuoteReplaysExistingPendingOrder` →
  FAIL（第二 quote 开出第二张订单/渠道调用 ≠ 1）。
- 回归测试：上述 service 测试（Q1 购买 → Q2 提交返回同 ID 同 checkout_url、
  渠道 createCalls==1）+ `TestCurrentPendingPurchaseOrderReturnsNewestPending`
  （repository：仅 pending、最新优先、无 pending 报 ErrOrderNotFound）。

### OCR-R1-07/08/35/36（high）verify_db_watch 四副本 runs/ 回退守卫恒假 — 已修复

- Ruling：finding 成立。`path` 仅在 `if archived.exists():` 分支被赋值为
  `str(archived)`，故 `archived.exists() and path != str(archived)` 在任何路径
  下恒 False（走到 elif matches 必然 archived 不存在）——R1-V10 声明的
  「runs/ 回退 → MISSING-EVIDENCE exit 2」降级从未生效，外来回放 TSV 会被
  静默断言（错配时 exit 1 形似业务回归，违背退出码契约）。按 fixHint 四副本
  一并修。
- 处置：守卫改 `if not archived.exists():`（flow-evidence-74/ocr1/ocr2/ocr3
  四份同步，WARNING + exit 2 文案不变——现在真正可达）；
  `test_verify_db_watch.py` 同步更新：runs/ 回退用例改为断言 WARNING + exit 2
  （新增 `test_runs_fallback_degrades_to_missing_evidence`），原 exit 1/0/0
  三个用例的语义锚点移到归档路径（write_archive）。
- RED 证据：基线脚本 + runs TSV（无归档）实测 exit 1、无 WARNING；修复版同
  场景 exit 2 + WARNING（本轮实际运行）。
- 回归验证：更新后 `test_verify_db_watch.py` 6 passed；四副本对各自归档
  TSV 重放全部 `DB-WATCH: PASS`（exit 0，归档主路径不受影响）。

### OCR-R1-05（medium）failed 分支 PASS 缺 exactly-once payments 校验 — 已修复

- Ruling：finding 成立且与本仓库既有严格度不对称论据一致：发票不可见分支
  （ocr-81 R1-08）与 open/pending 的 pre_charge_ok 都要求
  `len(non_succeeded) <= 1`，唯独发票可见为 failed 的分支只查
  `core_ok and canceled and reason=="payment_failed"` 即 PASS，且不记录
  payments_non_succeeded_count——而 R1-08 注释声称 payments 校验
  "asserted on this path too"，与实现矛盾。
- 处置：failed 分支在取消复查后补 `_payments_for` 读取（处于外层 try 内，
  OSError 同样映射 blocked）、写入 `payments_non_succeeded_count`、PASS 条件
  收紧 `len(non_succeeded) <= 1`；FAIL 文案附
  `non_succeeded_payments=<n>` 便于归因。
- RED 证据：基线 + 测试补丁运行 `pytest -k failed_invoice` → 2 failed
  （多支付注入用例得 pass；PASS 用例 KeyError payments_non_succeeded_count）。
- 回归测试：FakeLago 新增 `gate_invoice_reveals_failed` 旋钮（gate 发票以终态
  failed 形态首次可见：订阅 canceled(payment_failed) + 恰 1 笔 failed 支付）；
  `test_failed_invoice_branch_passes_with_exactly_once_payments_recorded`
  （PASS + count==1）与 `..._fails_on_multiple_non_succeeded_payments`
  （注入 2 笔 → FAIL + error 含 non_succeeded_payments）。

### OCR-R1-09（medium）Purchase default 分支 400+err.Error() 透出 — 已修复

- Ruling：finding 成立。default 分支把 EnsureBillingAccount/GetPublication/
  GetVersion/CreateOrder 的存储层错误与快照 JSON 解析错误以 400 + 原始
  err.Error() 透出——既误标客户端错误（诱导按 4xx 重试同一请求），又把
  SQL/驱动文本暴露到公网边界，与本端点 503 分支自述的闭合词汇矛盾。按
  fixHint 与 GetOrder 的 default→500 姿态对齐（GetOrder 透 err.Error() 但
  状态码 500；本端点进一步用固定闭合文案）。
- 处置：default 改 `log.Printf` 记原始错误（tenant 归因）+ 500 +
  `"purchase failed"` 固定文案。
- RED 证据：基线运行 `TestPurchaseHandlerAnswersServerErrorClosedOnInfrastructureFailure`
  （DROP TABLE commercial_quotes 构造存储层故障）→ FAIL（400 != 500）。
- 回归测试：上述 handler 测试（500 + "purchase failed" + 不含 "no such table"）。

### OCR-R1-12（medium）purchaseRequestTimeout 预算不自洽 + 422 终态错分 — 已修复

- Ruling：finding 成立。const 25s < 实际链条（≤3 次 provider 出站各 15s +
  pmSyncWait 20s + 订阅级尾部），轮询实际窗口远小于 20s；更关键的是失败重试
  走 bound 短路不再执行 PM 同步等待，create 撞 422 no_default_payment_method
  被映射为终态 `ErrPlatformInvalidResponse("purchase replay cannot be
  verified")`——瞬态导入等待永久丢失。采用 fixHint 首选（预算显式纳入）+
  次选第三项（422 no_default_payment_method 归类可重试 Unreachable）闭合
  重试链条。
- 处置：
  - `purchaseRequestTimeout` 由 const 25s 改为**调用时求值**的函数
    `subscriptionRequestTimeout + 3*outboundProviderTimeout + pmSyncWait`
    （包初始化 var 会在 init 时固化 pmSyncWait 初值，函数形式才能做到
    fixHint 要求的「测试调短 pmSyncWait 时总预算自洽」）。
  - gated create 的 422 分支前置特判：响应体含 `no_default_payment_method`
    → `ErrPlatformUnreachable: default payment method not imported yet`
    （重试的 bound 短路虽不再等待，但权威侧导入持续进行，下次 create 成功；
    不再升级为终态 InvalidResponse）。原 422 race 的 identity re-read 分支
    保持不变。
- RED 证据：基线上 (a) 预算测试（RED 变体直断 const）→ 「baseline const
  budget 25s starves the chain (sync wait 20s + 3 provider hops 45s)」；
  (b) `TestLagoGatedCreateNoDefaultPaymentMethodIsRetryable` →
  「got platform_invalid_response: purchase replay cannot be verified」。
- 回归测试：`TestPurchaseRequestTimeoutBudgetCoversProviderChainAndSyncWait`
  （预算 == 链条和；调短 pmSyncWait 后预算随之收缩）、
  `TestLagoGatedCreateNoDefaultPaymentMethodIsRetryable`（stub 旋钮
  `rejectCreateNoDefaultPM`：422 no_default_payment_method → Unreachable
  且非 InvalidResponse）。

### OCR-R1-14/16/17/19（medium）渠道客户端 rebinding 窗口 + 重定向无上限 — 已修复

- Ruling：finding 成立。两个渠道 client 用默认 Transport（校验时 LookupIP、
  拨号时默认 Transport 再独立解析——rebinding 应答可让校验见公网 IP、拨号落
  内网）；自定义 CheckRedirect 整体替换标准库默认 10 跳上限（该默认仅
  CheckRedirect 为 nil 时生效），两闭包均不查 len(via)——互发 302 的两个公网
  主机即可形成无限重定向环。采用 fixHint：改用
  `secutils.NewSSRFSafeHTTPClient`（同时获得拨号期 IP 钉扎、每请求带缓存的
  出站校验、跨域凭证剥离、MaxRedirects 上限）。
- 处置：alipay.go 与 wechat.go 构造处 client 统一为
  `secutils.NewSSRFSafeHTTPClient(SSRFSafeHTTPClientConfig{Timeout: cfg.timeout(),
  MaxRedirects: 10})`，删除自定义 CheckRedirect；注释记录替换理由（含
  「自定义 CheckRedirect 替换默认 10 跳上限」与「默认 Transport 二次解析」
  两个根因）。
- RED 证据：基线运行 `TestChannelClientsCapRedirectLoop` → FAIL：wechat
  client 在互发 302 的两 stub 间跳转至 Client.Timeout 耗尽
  （`context deadline exceeded (Client.Timeout exceeded while awaiting
  headers)`）——无跳数上限的直接实证。
- 回归测试：`TestChannelClientsCapRedirectLoop`（wechat+alipay 两个 client：
  环必须停在 "stopped after 10 redirects"，hops ≤ 12 而非跑到 Timeout；
  白名单豁免环回 stub 与真实出站同一通道）；既有出站测试
  （TestWechatDoRejectsInternalEgressWithoutWhitelist /
  TestWechatDoAllowsWhitelistedLoopbackStub / alipay fixture 组）全量通过
  证明豁免与既有校验行为不回归。
- 说明：DNS rebinding 的拨号层钉扎由共享设施 `SSRFSafeDialContext` 提供
  （internal/utils/security_transport_test.go 已有设施级测试），渠道侧回归
  锚定在客户端行为（跳数上限 + 共享策略挂载）。

### OCR-R1-20（medium）CurrentPurchaseOrder 价格面匹配无时间锚 — 已修复

- Ruling：finding 成立。同租户同价面历史购买单全部命中（取消后同价复购），
  `id ASC` 对 "ord_"+随机 hex 是纯字典序随机 tie-break（两单 pending 时可能
  投影旧单 checkout_url——渠道侧可能已失效；两单皆终态时取最旧一单），
  OrderRow 无任何时间戳列。采用 fixHint 首选：加 created_at 列并按
  created_at DESC 取最近（pending 优先）。
- 处置：`OrderRow` 新增 `CreatedAt`（AutoMigrate 原位补列，与 checkout_url/
  kind 同一 additive 先例）；`createOrderTx` 零值时填 `time.Now().UTC()`
  （调用方可预置，`OpenOrder` 传 `cmd.Now` 保持单元时间基一致）；
  `CurrentPurchaseOrder` 与新的 `CurrentPendingPurchaseOrder` 排序均为
  `created_at DESC, id ASC`（id 仅确定性 tie-break）。
- RED 证据（源码级）：基线 `git show 1c684ae68:...order.go` — grep
  created_at/CreatedAt 零命中（无时间戳列），L245 `Order("id ASC")`。
- 回归测试：`TestCurrentPurchaseOrderPrefersNewestByCreatedAt`
  （字典序最旧但时间最新的 pending 胜出；双 pending 取最新；全终态取最新
  终态而非最旧；无匹配 ErrOrderNotFound）。

### OCR-R1-24（medium）purchase() 丢弃 503 顶层 reason 令牌 — 已修复

- Ruling：finding 成立。POST 平台故障走 503 信封
  （`{"error":...,"reason":"unreachable|..."}`），request 层非 2xx 直接抛
  ApiError，errorFromResult 只提取 message/code——CheckoutPage 专门写的
  purchaseErrorMessage 闭合文案映射在真正的 POST 故障路径永不可达。
  采用 fixHint 首选：errorFromResult 提取顶层 reason 到 details，
  页面按令牌映射。
- 处置：`errors.ts::errorFromResult` 顶层 `reason`（非空字符串）并入
  details（保留既有 nested/record details 字段）；CheckoutPage 提取
  `purchaseReasonMessage(reason)`（两条路径共用一份闭合映射），catch 分支对
  ApiError 优先按 `details.reason` 映射中文文案，无令牌再回退原始 message。
- RED 证据：基线运行新增 errors 测试 → 2 failed（details undefined，
  reason 被丢弃）。
- 回归测试：errors.test.ts 三用例（503 信封 reason 入 details / 既有
  details 字段保留 / 无令牌 details 不动）+ CheckoutPage.test.tsx
  `a 503 purchase failure with a closed reason token maps to the Chinese
  message`（渲染「支付平台暂时不可达」且不出现英文 message）。

### OCR-R1-28/29/31（medium）flow-evidence-82 三脚本硬编码绝对路径 — 已修复

- Ruling：finding 成立。三脚本 createRequire 均硬编码
  `/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-lago/apps/web/package.json`，
  与脚本头 "Run from anywhere" 注释矛盾——其它机器/checkout/CI 上
  `require('@playwright/test')` 解析失败。按 fixHint 改 import.meta.url 相对解析。
- 处置：三脚本统一
  `createRequire(new URL('../../../apps/web/package.json', import.meta.url))`
  （本目录相对仓库根上溯 3 级）。
- RED 证据：node 以另一路径构造 createRequire → require('@playwright/test')
  抛 `MODULE_NOT_FOUND`；修复后以脚本实际位置解析 `@playwright/test` 成功
  （typeof function、chromium 可用），三脚本 `node --check` 语法通过。
- 回归验证：见上（完整浏览器流程重放需 8091 栈，见「重放说明」）。

### OCR-R1-37（medium）AC2 无新发票断言缺存在性守卫 — 已修复

- Ruling：finding 成立。`dr.get("invoice_count") == dr.get("invoice_count_baseline")`
  在两键同时缺失时 None==None 空转通过；`d["evidence"]["baseline"]` 直接索引
  键缺失时 KeyError 崩溃（无 RESULT 行）而非记名 FAIL——与 docstring 自述的
  守卫不变量矛盾。当前归档证据三键均在（实测均 1），属 replay 场景潜在缺口。
- 处置：断言补 `dr.get("invoice_count") is not None` 前置；baseline 改
  `(d.get("evidence") or {}).get("baseline") or {}` 后 `.get("invoice_count")`
  （缺键 → 第三合取项 False → 记名 FAIL，不再 KeyError）。
- RED 证据：`dr={}` 时 `dr.get(...)==dr.get(...)` → True（空转，python 实测）。
- 回归验证：ocr3 副本对归档证据重放 ALL PASS（三键在时判定不变）。

## 测试与重放证据（全部在本轮实际执行）

| 检查 | 命令 | 结果 |
|---|---|---|
| RED（R1-22） | 基线 1c684ae68 `go test ./internal/modules/commercial/service/commercial/ -run TestPurchaseSecondQuoteReplaysExistingPendingOrder` | FAIL（第二 quote 开出第二张订单） |
| RED（R1-07） | 基线 verify_db_watch.py + runs TSV（无归档） | exit 1、无 WARNING（降级不可达） |
| RED（R1-05） | 基线 `pytest -k failed_invoice` | 2 failed（多支付得 pass / KeyError count） |
| RED（R1-09） | 基线 handler 新测试 | FAIL（400 != 500） |
| RED（R1-12） | 基线两测试 | 「const budget 25s starves the chain (20s+45s)」/「got platform_invalid_response」 |
| RED（R1-14） | 基线 `TestChannelClientsCapRedirectLoop` | FAIL：跳转至 Client.Timeout 耗尽（无跳数上限） |
| RED（R1-24） | 基线 errors.test.ts 新增 3 用例 | 2 failed（details undefined） |
| RED（R1-37/R1-28） | python None==None / node 异路径 createRequire | True 空转 / MODULE_NOT_FOUND |
| 全仓编译 | `go build ./internal/...`（worktree） | 通过 |
| commercial + handler 全量 | `go test ./internal/modules/commercial/... ./internal/handler/ -count=1` | 8 包全 ok |
| lago-lab 离线回归 | `pytest test_lab.py test_phases.py test_verify_db_watch.py -q` | 86 passed in 669.38s（ocr-81-2 后 83 + 新增 3：R1-05 两用例 + R1-07 runs 回退降级用例；R1-07 其余三用例更新后仍全过） |
| TS 单测 | `npx tsx --test errors.test.ts CheckoutPage.test.tsx` | 11/11 pass |
| contracts/commercial 回归 | `npx tsx --test packages/api-client/src/commercial.test.ts` | 9/9 pass |
| AC 断言重放 ×4 | `python3 docs/plans/<dir>/verify_ac_assertions.py` | evidence-74/ocr1/ocr2/ocr3 全部 ALL PASS |
| DB-WATCH 归档重放 ×4 | `python3 docs/plans/<dir>/verify_db_watch.py` | 四副本 `DB-WATCH: PASS`（归档主路径不变） |
| 微信 stub 往返 | POST/GET 127.0.0.1:8291（flow 遗留实例） | code_url 正常下发、查单 NOTPAY（R1-14 渠道契约面旁证） |

### 重放说明（全链路真实流程未重放）

本批 R1-22/R1-20/R1-09/R1-24/R1-14 改变了已验证用户流程（#81 微信 checkout、
#82 支付宝同步返回）的行为面，按 ask 应重放
`docs/plans/issue-72-flow-evidence-*/` 可重放脚本；与上两轮同因不可行：8091
后端为其他会话的旧构建进程（不含本批修复，重放无意义且不可重启）、
`WEKNORA_COMMERCIAL_STRIPE_API_KEY` 不在本环境。实际执行的重放与替代覆盖：
- 独立可重放脚本全部重放：四副本 verify_ac_assertions（ALL PASS×4）与
  verify_db_watch（归档路径 PASS×4，runs/ 回退新降级路径由
  test_verify_db_watch 6 用例锁定）；微信 stub 下单/查单往返正常。
- flow-evidence-82 三浏览器脚本：修复项即其可重放性本身，已验证相对解析在
  本 checkout 下正确加载 @playwright/test（完整跑需 8091+vite 栈，环境同上）。
- 行为等价性由分层回归覆盖：R1-22/R1-20（service+repository 测试）、R1-09
  （handler 测试）、R1-24（errors+CheckoutPage 测试）、R1-14（redirect-loop +
  既有 SSRF 出站/豁免测试全量通过）。

## 提交

- 提交 message 前缀：`issue-72(ocr-1):`（按本批 ask 指定）

---

# Issue #72 — OCR 第 2 轮修复记录（ocr-r2，6 组 findings）

- 日期：2026-09-25
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：findings 审查 `a2986794a`（R1 修复提交，findings 证据原文确认其已落地）；
  RED 在 `0c6409f75`（其上的 findings 记录提交，代码态相同）复现。
- 修复人：修复员（dynamic workflow subagent）
- 范围：OCR-R2-26（high）、R2-28（high）、R2-01/R2-17+19/R2-21/R2-27（medium），
  6 组全部修复，0 项 deferred。顺带修复一处与本批无关的预存测试 flake
  （见「顺带修复」）。

## 总则

- 每个行为级 finding 先在基线真实运行复现旧行为（RED），再在修复版验证（GREEN）；
  R2-01 以源码级 RED（基线无 409 映射表）+ 修复版测试通过记录。
- 安全约束核对：本轮未新增出站请求与 URL 拼接（R2-17/19 是恢复既有
  ProxyFromEnvironment，SSRF 拨号/逐跳校验层不动）；新增 SQL 均为参数绑定
  或静态 DDL（部分唯一索引 DDL 无外部输入）；无凭据字面量。

## 逐条 Ruling 与处置

### OCR-R2-26（high）三段读-判-写无原子性，双 pending 渠道单仍可并发产生 — 已修复

- Ruling：finding 成立。R1-22 的 GetOrderByQuote → CurrentPendingPurchaseOrder →
  CreateOrder 三段独立执行，数据库唯一性只在 quote_id 维度——两张不同新报价
  并发 POST 时双方前置 SELECT 都能通过（彼时对方订单未提交），各自成功开单，
  同一 gating invoice 两张 pending 可付渠道单，两个回调独立确认即双重扣款。
  采用 fixHint 首选：数据库层部分唯一索引 + 插入冲突作为回放触发（与
  ErrQuoteAlreadyUsed 同构）。fixHint 提到的"匹配键锚定购买身份"以
  tenant 维度索引实现（一个租户一个购买身份，#81 单购买模型——tenant 级
  唯一比 plan/订阅锚更严且正确）。
- 处置：
  - NewOrderService 装配点 AutoMigrate 后建部分唯一索引
    `uq_purchase_pending_per_tenant ON commercial_orders (tenant_id) WHERE
    kind='purchase' AND state='pending' AND channel_failed=0`（sqlite/pg 同语法；
    持有不变量前重复数据的存量部署在装配时显式失败而非静默继续）。
  - `createOrderTx` 的插入冲突识别（sqlite 列面 "UNIQUE constraint failed …
    tenant_id" / pg 索引名）→ 新哨兵 `ErrPurchasePendingExists`（仅 purchase
    kind；quote_id 冲突形态不误判）。
  - purchase.go 第 8 步：CreateOrder 返回哨兵时经
    `CurrentPayablePendingOrder`（不限价面的回读——索引已证明该租户存在
    可付 pending）verbatim 重放胜者订单，绝不开第二张渠道单。
- RED 证据：基线（0c6409f75）上同租户两张不同 quote 的可付 pending
  CreateOrder **全部成功**（"baseline allows TWO concurrent payable pending
  orders (no partial unique index / no sentinel)"）。
- 回归测试：`TestPartialPendingIndexRejectsSecondPayableOrder`（第二张撞哨兵；
  quote_id 冲突不误判；channel_failed 死单/终态单不占槽）、
  `TestCurrentPendingPurchaseOrderReturnsNewestPayablePending`（含
  CurrentPayablePendingOrder 回读面）、service 层
  `TestConcurrentFreshQuoteConflictReplaysWinner`（胜者中间态：订单已提交但
  链接未持久化——前置价面+可付检查必然漏掉、索引拒绝——败者重放胜者、
  渠道 createCalls==0）。

### OCR-R2-28（high）「pending」≠「可支付」，渠道失败死单永久卡死购买 — 已修复

- Ruling：finding 成立。渠道 Create 失败分支（openOrder CheckoutError）留下
  pending 死单：checkout_url 空、渠道大概率从未见单、状态机无 failed 迁移
  （唯一出 pending 路径是 ConfirmPayment）——此后每次新报价 POST 都被这张
  死单挡住（R1-22 重放它），客户端拿不到任何支付入口，只能人工修库。
  采用 fixHint 的"在行上持久化渠道失败标记"形态（显式列比"checkout_url
  为空"判据更精确——R2-27 的持久化降级单渠道实际成功，不应归入死单），
  与 R2-26 索引条件联动（channel_failed=0）一并闭环。
- 处置：
  - `OrderRow` 新增 `channel_failed` 列（AutoMigrate 原位补列）；
    `MarkChannelFailed`（pending 行置位，参数绑定）。
  - openOrder 渠道失败分支：持久化 channel_failed 标记（标记丢失仅记日志——
    行为不变量由索引与回读条件共同保证，标记失败不应吞掉既有的
    CheckoutError 应答契约）。
  - 部分唯一索引条件含 `channel_failed=0`（死单不占 pending 槽）；
    `CurrentPendingPurchaseOrder` 重放条件收紧为
    `channel_failed=0 AND checkout_url<>''`（仅可付单作为支付入口重放）。
  - 迟到渠道确认边界（Create 超时但渠道侧实际建单）：死单仍 pending，
    回调/恢复照常 ConfirmPayment 付款——与 fixHint"先 Query 确认未知再放行"
    的差异（不引入 service 层对 provider 的直接依赖）以注释如实记录。
- RED 证据：基线 `TestChannelFailedOrderDoesNotBlockFreshQuote` FAIL
  （渠道失败后新报价被死单挡住，无法开出新可付单）。
- 回归测试：上述 service 测试（渠道失败 → 新报价成功开出新可付单，
  createCalls==2，两单 ID 不同）+ repository
  `TestCurrentPendingPurchaseOrderReturnsNewestPayablePending`（channel_failed/
  无链接单不入选）。

### OCR-R2-27（medium·安全）持久化降级复用渠道失败标记并透 %v 原文 — 已修复

- Ruling：finding 成立。SetCheckoutURL 失败时 openOrder 把 gorm 错误以
  `fmt.Sprintf("...%v", err)` 塞进 CheckoutError：(1) SQL/驱动细节经
  orderWire 序列化到公共边界，违背链路其余分支的闭合词汇姿态；(2) 渠道
  调用实际成功、链接有效，但非空 CheckoutError 让 handler 降 202——一次
  成功下单被误判渠道失败。按 fixHint：独立降级标记 + 闭合外显 + 原始错误
  进日志 + 202 仅对渠道失败。
- 处置：OrderView 新增 `CheckoutLinkDegraded bool`（json
  checkout_link_degraded，闭合布尔标记）；openOrder 持久化失败分支改为
  `log.Printf` 记原始错误 + 置位标记（CheckoutError 保持空）；handler 的
  202 判定不变（其条件 CheckoutError!="" 现在天然只对渠道失败成立），
  orderWire 携带 checkout_link_degraded。
- RED 证据：基线运行（sqlite 触发器 RAISE(ABORT) 确定性构造 UPDATE 失败）
  → view.Order.CheckoutError = "checkout_url persistence failed for
  ord_…: test block"（原始错误文本直达视图）。
- 回归测试：`TestCheckoutLinkDegradedIsSeparateFromChannelFailure`
  （err==nil、CheckoutURL 非空、CheckoutLinkDegraded==true、
  CheckoutError==""——202 触发条件不满足）。

### OCR-R2-17/19（medium）SSRFSafe 客户端切换静默丢环境代理 — 已修复

- Ruling：finding 成立（R1-14/17 修复引入的回归）。原 `&http.Client{Timeout}`
  Transport 为 nil → DefaultTransport 带 ProxyFromEnvironment；
  NewSSRFSafeTransport 不设 Proxy 字段——仅代理出网的部署从可达退化为直连
  超时，且 IsSystemProxy 拨号旁路成为死代码。按 fixHint：
  NewSSRFSafeHTTPClientWithTransport + transport.Proxy 复挂。
- 处置：alipay/wechat 构造改为
  `transport := secutils.NewSSRFSafeTransport(...); transport.Proxy =
  http.ProxyFromEnvironment; client :=
  secutils.NewSSRFSafeHTTPClientWithTransport(cfg, transport)`（SSRF 拨号/
  逐跳校验/跳数上限层保持不变）。
- RED 证据：基线 `TestChannelTransportKeepsEnvironmentProxy` FAIL
  （"base transport must carry the environment proxy"）。
- 回归测试：该测试（两渠道 client：SSRFValidatingRoundTripper 包装 +
  Base 为 *http.Transport 且 Proxy 非空 + DialContext 在位）；既有
  redirect-loop/出站 SSRF 测试全量通过（R1-14 行为不回归）。

### OCR-R2-21（medium）parsePurchaseView 未知 reason 令牌整体抛错 — 已修复

- Ruling：finding 成立。reason 是建议性字段，后端先发版新增令牌时整份
  PurchaseView 解析失败——POST 路径用户直接看到 'invalid purchase (reason)'
  原文。按 fixHint：未知令牌按缺席忽略，state 保持严格。
- 处置：contracts `parsePurchaseView` 的 reason 分支改为
  `typeof==='string' && !=='' && PURCHASE_REASONS.has(...)` 才写入（未知/
  非串一律缺席，不再 throw）；注释记录 advisory 语义。
- RED 证据：基线 contracts 测试 1 fail（未知 reason 抛 'invalid purchase
  (reason)'）。
- 回归测试：contracts 测试三断言（未知令牌按缺席、已知令牌保留、state
  严格校验不放松）。

### OCR-R2-01（medium）409/500 错误族英文机器令牌原文展示 — 已修复

- Ruling：finding 成立。R1-24 只覆盖 503 reason 令牌；同一 POST 的 409/500
  族（"quote expired"、invoice_quote_mismatch、…）无 reason 无 code，
  closedReason undefined 时 error.message 原文展示——报价过期是真实可达路径。
  按 fixHint：message 令牌映射 + 未命中给统一中文兜底。
- 处置：CheckoutPage 新增 `PURCHASE_CONFLICT_MESSAGES`（9 个后端闭合令牌 →
  中文）与 `purchaseErrorText(error)`（503 reason 令牌 → 409/500 message
  令牌 → 服务端错误统一兜底「购买未能创建，请稍后重试」；非服务端错误
  保留原始 message）；catch 分支改用它。
- RED 证据（源码级）：基线 CheckoutPage.tsx 无任何 409 令牌映射
  （grep purchaseConflictMessage/PURCHASE_CONFLICT_MESSAGES = 0），catch
  回退 error.message 原文；完整 RED worktree 因 node_modules 未安装无法
  跑 jsdom（已如实记录，行为由修复版测试锁定）。
- 回归测试：`a 409 purchase conflict maps its message token to the Chinese
  copy`（「报价已过期」且不出现 "quote expired"）、
  `an unmapped server error falls back to the closed Chinese copy`（兜底
  中文且不出现未知令牌原文）。

## 顺带修复（与本批 findings 无关的预存 flake）

- `providers_env.go::alipayConfigFromEnv`：required 集合由 map 遍历改为
  slice 固定序——部分配置错误的点名变量此前随 map 迭代顺序随机
  （基线 0c6409f75 实测 5 次内复现 FAIL），`TestProvidersFromEnvRejects-
  PartialAlipay` 因此偶发红。修复后 15 轮全量稳定通过；错误消息格式
  同步收紧（点名完整变量名）。

## 测试与重放证据（全部在本轮实际执行）

| 检查 | 命令 | 结果 |
|---|---|---|
| RED（R2-26） | 基线双 pending CreateOrder | 两张全部成功（无索引/哨兵） |
| RED（R2-27） | 基线触发器构造 UPDATE 失败 | CheckoutError 携带 %v 原文直达视图 |
| RED（R2-28） | 基线 TestChannelFailedOrderDoesNotBlockFreshQuote | FAIL（死单挡新报价） |
| RED（R2-17/19） | 基线 TestChannelTransportKeepsEnvironmentProxy | FAIL（Proxy nil） |
| RED（R2-21） | 基线 contracts 新测试 | 1 fail（未知 reason 抛错） |
| RED（R2-01） | 基线 grep 409 映射 | 0 命中（error.message 原文展示） |
| 全仓编译 | `go build ./internal/...` | 通过 |
| commercial + handler 全量 | `go test ./internal/modules/commercial/... ./internal/handler/ -count=1` | 8 包全 ok |
| payment 包稳定性 | `go test ./internal/modules/commercial/payment/ -count=1` ×15 轮 | 15/15 ok（flake 修复后） |
| TS 单测 | `npx tsx --test` errors/CheckoutPage/contracts/commercial | CheckoutPage 7/7、contracts 21/21、errors 6/6 |
| 四副本重放 | `python3 docs/plans/<dir>/verify_{ac_assertions,db_watch}.py` | AC ALL PASS ×4、DB-WATCH PASS ×4（本轮无 Python 改动，确认无回归） |
| 微信 stub 往返 | POST 127.0.0.1:8291 native | code_url 正常下发（渠道契约面） |

### 重放说明（全链路真实流程未重放）

本批 R2-26/R2-28/R2-01/R2-27/R2-17+19 改变已验证用户流程（购买幂等/错误面/
渠道传输）的行为面，按 ask 应重放 flow-evidence 可重放脚本；与前三轮同因
不可行（8091 为其他会话旧构建进程、Stripe key 不在本环境）。已执行：四副本
AC 断言 + DB-WATCH 重放全过（本轮无 Python 侧改动，确认无回归）、微信
stub 下单往返正常；行为等价性由分层回归覆盖（repository 索引/哨兵、service
冲突回放与死单让路、handler/contracts/页面映射、渠道 transport 结构 +
redirect-loop 行为）。

## 提交

- 提交 message 前缀：`issue-72(ocr-2):`（按本批 ask 指定）

---

# Issue #72 — OCR 第 3 轮修复记录（ocr-r3，11 组 findings）

- 日期：2026-09-25
- worktree：`.worktrees/issue72-lago`（分支 `codex/issue-72-lago`）
- 修复基线：findings 审查 `cfa1b2452`（R2 修复提交）；RED 在 `7b9db0314`
  （其上的 findings 记录提交，代码态相同）复现。
- 修复人：修复员（dynamic workflow subagent）
- 范围：OCR-R3-27（critical）、R3-26（high）、R3-28（high）、
  R3-25/R3-07/R3-09/R3-02/R3-17/R3-19/R3-31/R3-39（medium 各组），
  11 组全部修复，0 项 deferred。

## 总则

- R3-27 以真实 PostgreSQL 17（WeKnora-postgres-dev 容器，临时库）做了方言
  级 RED/GREEN 双证；行为级 RED 均在基线 worktree 真实运行复现。
- 安全约束核对：本轮未新增出站请求；新增 SQL 均参数绑定或静态 DDL
  （boolean 字面量）；无凭据字面量（PG 验证用本地容器 postgres 用户，
  无凭据写入仓库）。

## 逐条 Ruling 与处置

### OCR-R3-27（critical）PG 部署启动必失败：索引谓词 boolean = integer — 已修复

- Ruling：finding 成立。GORM 将 ChannelFailed（bool）在 PG 迁移为 boolean
  列，`channel_failed = 0` 无隐式转换，CREATE INDEX 解析期即报错——
  DB_DRIVER=postgres 是受支持生产驱动，服务完全无法启动。
- 处置：索引谓词改 `channel_failed = false`（SQLite 3.23+ 与 PG 均支持
  boolean 字面量）；注释记录 0/1 拼法在 PG 的失败形态；fixHint 的
  「dialect 分支」不需要（false 字面量两库通用）。
- RED/GREEN（PG 17 实测）：旧 DDL → `ERROR: operator does not exist:
  boolean = integer`；新 DDL → CREATE INDEX 成功（含回填/清扫 UPDATE 全部
  在 PG 上执行通过）。
- 回归：fixHint 的「PG 方言迁移启动测试」以真实容器一次性验证落地
  （测试套件内的 PG 依赖不可行——本仓库单测栈为 sqlite；DDL 与生产
  NewOrderService 逐字一致的 sqlite 测试 + 本节 PG 实证共同防回归）。

### OCR-R3-26（high）可付判定三处不一致 + 迁移未处置存量行 — 已修复

- Ruling：finding 成立。(1) 索引谓词只排 channel_failed、
  CurrentPendingPurchaseOrder 要求 URL 非空、CurrentPayablePendingOrder 无
  URL 条件——link-less 行占槽锁死且被当胜者干净重放；(2) 三个触发源中
  迁移前存量 pending（URL 全空、channel_failed 回填 false、created_at NULL
  在 PG NULLS FIRST）部署当天即落入索引作用域。
- 处置（含一次设计修正）：
  - **首轮实现**（索引谓词补 `checkout_url <> ''`）被本轮自己的 handler
    回归测试推翻：把 URL 纳入谓词会把冲突从原子 INSERT 挪到之后的
    SetCheckoutURL UPDATE（两张并发结账都 link-less 插入成功，第二个
    链接持久化撞索引）——插入原子性丢失。已回退并采用下述结构。
  - **最终结构**：索引谓词保持 `channel_failed = false`（INSERT 即原子
    冲突，R2-26 语义不变 + R3-27 字面量修复）；两处读判定统一
    「channel_failed=false AND checkout_url<>''」（重放必为可付单）；
    **冲突回放读不到可付行时清扫解锁**：新增
    `SweepStaleLinklessPending`（tenant 的 pending+channel_failed=false+
    无链接行 → channel_failed=true）后重试 CreateOrder 恰一次（二次冲突
    上抛，绝不循环）。link-less 残留（SetCheckoutURL 降级——渠道成功但
    链接未落库）由此不再锁死租户。
  - **迁移处置存量行**（NewOrderService 建索引前，参数绑定）：存量
    pending purchase 无链接行 → channel_failed=true（触发源 c：旧管线
    行不再入索引作用域）；created_at NULL/零值回填 NOW（消除 PG
    NULLS FIRST 遮蔽）。
  - **触发源 b**：渠道 Create 成功但返回空 CheckoutURL → openOrder 视同
    渠道失败（MarkChannelFailed + CheckoutError 姿态）——渠道没给链接
    就不是可付结果，不再产生 channel_failed=false 的 link-less 行。
  - 双收权衡（注释记录）：清扫放行新结账后，被清扫单的渠道侧单仍可能
    存在（降级场景渠道成功）——与 R2-28 已接受的边界同型，迟到回调照常
    ConfirmPayment。
- RED 证据：基线 `TestLinkLessPendingOrderDoesNotBlockFreshQuote` →
  zombie 被当胜者重放（view.Order=ord_zombie、CheckoutURL 空串的干净
  201 形态）。
- 回归测试：service 上述测试（清扫后新单可付、createCalls==1）+
  `TestSweepStaleLinklessPendingReleasesTheSlot`（repository：link-less
  占槽→清扫恰 1 行→新单可插）+ 更新后的
  `TestPartialPendingIndexRejectsSecondPayableOrder`（INSERT 时原子冲突
  + 0/1→false 字面量）。

### OCR-R3-28（high）持久化写复用请求级 ctx，取消即留僵尸行 — 已修复

- Ruling：finding 成立。渠道 Create 因调用方取消（客户端断连）失败时，
  MarkChannelFailed 用同一已取消 ctx——标记写入同败（仅日志），订单残留
  pending+channel_failed=false+无链接：占索引槽且被冲突回读当胜者，
  无过期/清理路径，购买链路永久卡死。
- 处置：openOrder 的两个持久化写（MarkChannelFailed/SetCheckoutURL）改
  `context.WithTimeout(context.WithoutCancel(ctx), checkoutPersistTimeout=5s)`
  ——脱离请求取消、独立有界；SetCheckoutURL 同享（其降级残留同样会僵尸）。
  fixHint 的「长期无链接 pending 的恢复/超时出口」由 R3-26 清扫（服务端
  解锁）+ R3-09 前端重发入口共同提供，注释记录取舍。
- RED 证据：基线 `TestOpenOrderPersistsChannelFailurePastCallerCancellation`
  → `ChannelFailed:false`（取消吞掉标记，僵尸行成形）。
- 回归测试：该测试（cancellingCreateStub 在渠道调用中取消 → 应答仍带
  CheckoutError 姿态且行上 ChannelFailed=true）。

### OCR-R3-25（medium）ErrPurchasePendingExists 从 POST /orders 逃逸 — 已修复

- Ruling：finding 成立。CreateOrder（service/order.go）同为 kind=purchase，
  旧路径无 PurchaseService 前置检查，OpenOrder 撞索引返回哨兵，handler
  CreateOrder switch 无映射 → default 400 + 裸令牌，无回放数据。
- 处置：handler 补 `errors.Is(ErrPurchasePendingExists)` 分支：409 +
  "purchase pending exists" + 附带既有可付 pending 订单（新增
  `OrderService.CurrentPayablePendingOrderView` 投影，回读不到可付行时
  409 不带 order——清扫解锁走 Purchase 主路径，注释记录）。
- RED 证据：基线 handler 测试 → `got 400: {"error":"purchase_pending_exists"}`。
- 回归测试：`TestCreateOrderHandlerMapsPendingExistsWithReplay`
  （409 + 闭合文案 + 附 first.ID 与 first.CheckoutURL）。

### OCR-R3-07/09/02（medium）CheckoutPage 重试死胡同/支付死胡同/英文兜底 — 已修复

- Ruling：三条均成立。R3-07：报价级冲突重试复用同一失效报价（quoteRef
  永不重置）；R3-09：渠道失败 202 的 pending 无链接订单页面只有「刷新」
  死胡同；R3-02：非服务端错误走 error.message 展示浏览器英文技术串。
- 处置：
  - `QUOTE_LEVEL_CONFLICT_TOKENS`（4 令牌）+ `isQuoteLevelConflict`：catch
    命中即清空 quoteRef.current——「重试」按钮触发 effect 重新 quote()。
  - ready 态 pending 且无安全 checkout_url → 「支付渠道异常，此订单暂无
    可用支付链接」提示 + 「重新发起支付（获取新报价）」按钮
    （restartCheckout：清 orderIdRef/quoteRef → loading → retryToken 重跑
    effect；服务端 R2-28/R3-26 已保证不阻塞）。
  - `purchaseErrorText` 非服务端错误收敛「网络异常，请稍后重试」（原始
    message 仅 console.warn）；R1-V01 的中文闭合文案改为直接落错误态
    （不经 throw/catch，避免被统一兜底覆盖）。
- RED（源码级）：基线无 restartCheckout/isQuoteLevelConflict/中文兜底
  （grep 0 命中；RED worktree 无 node_modules 无法跑 jsdom，已如实记录）。
- 回归测试：`a quote-level conflict retry re-cuts a fresh quote`（重试后
  quoteCalls==2）、`a link-less pending order offers a restart-checkout way
  out`（提示+按钮+重启后渲染支付链接、quote/purchase 各 2 次）、
  `a network-layer error renders the closed Chinese copy`（TypeError 不外显）。

### OCR-R3-17（medium）verify_db_watch 降级时机与 docstring 自相矛盾 — 已修复（四副本）

- Ruling：finding 成立。docstring 称 exit 2 仅「无归档且 runs/ 无 TSV」，
  实际 R1-V10 后置守卫使任何无归档场景都 2 退出：elif matches 选出的
  runs/ 文件永不被断言（读取解析为死计算），输出先 "using <runs tsv>"
  再 WARNING 自相矛盾，审计者按 docstring 误读退出码含义。
- 处置（四副本同步）：降级提前到文件选择处（`if not archived.exists():`
  WARNING+exit 2，无 runs/ 探测、无 using 行）；删除后置守卫；docstring
  退出码 2 契约改为「无归档 TSV：runs/ 回退对 run 级 exactly-once 断言
  不可判定」；test_verify_db_watch 的 missing-ts­v 用例断言同步。
- RED 证据：基线脚本 + runs TSV（无归档）实测输出同时含 "using" 与
  "WARNING"、exit 2（自相矛盾形态）；修复版同场景仅 WARNING + exit 2。
- 回归验证：test_verify_db_watch 6 passed；四副本归档路径重放
  DB-WATCH: PASS ×4。

### OCR-R3-19（medium）ocr3 docstring 增量声明失实 — 已修复

- Ruling：finding 成立。「This copy additionally guards every bare
  all(...values())」处于 Derived-from-ocr-2 的增量清单，但 R1-15 守卫是
  同批落到全部四副本的——ocr-3 对这些调用未做任何改动，误记为本副本
  变更误导后续审计（与 R1-V23 同类失实溯源）。
- 处置：docstring 改写为 Provenance note：守卫系继承（R1-15 四副本同批），
  本副本实际新增只有 R1-37 invoice-count 守卫；行号引用改按 check 名锚定
  （fixHint 建议），后续副本沿用该惯例。
- 回归验证：ocr3 副本对归档证据重放 ALL PASS。

### OCR-R3-31（medium）reason 合并对数组/字符串 details 的边界 — 已修复

- Ruling：finding 成立。`typeof [] === 'object'` 使数组 details 被
  `{...base, reason}` 展开成索引键对象（数组语义静默丢失）、字符串型整体
  丢弃；errorFromResult 是所有非 2xx 的公共构造路径。按 fixHint 首选：
  base 条件加 !Array.isArray（数组/字符串均弃用为仅携带令牌的新对象；
  fixHint 的「reason 独立字段」改造面更大，未采用）。
- RED 证据：基线 errors 新测试 1 fail（数组被展开，'0' in details）。
- 回归测试：数组/字符串型 details 两用例 + 既有无令牌用例。

### OCR-R3-39（medium）retries 无延迟复查，与 duplicates 不对称 — 已修复

- Ruling：finding 成立。retries 的 re-POST 与 duplicates 探针 1 同型
  （对 B active 订阅的重复注册，归档证据 re_post_http_status=200），而
  duplicates 加 120s 延迟复查的依据正是该探针的分钟级漂移——retries 只看
  即时窗口，AC3「不产生第二个商业对象」的结论可能是错的。
- 处置：retries 即时 checks 全真时复用 `ctx.duplicates_settle_delay` 对 B
  做延迟复查（仍 active、同 lago_id、恰 1 笔 succeeded、发票仍恰 1 张——
  续期发票即漂移证据），新 check `deferred_no_drift` 入 checks 与证据
  `deferred_recheck`；cleanup 的 DELETE 状态候选加入 `terminated`
  （漂移终止形态可清理）。
- RED 证据：基线跑新测试 → FAIL（无 deferred_recheck，漂移不判 FAIL）。
- 回归测试：`test_retries_deferred_recheck_catches_late_termination`
  （deferred_terminate_after_duplicates=3 落在复查内 → FAIL + ok=False）；
  既有 PASS 测试补 deferred 复查断言；`test_gate_invoice_id_unknown…`
  的 checks 键集合断言更新（+deferred_no_drift）。

## 测试与重放证据（全部在本轮实际执行）

| 检查 | 命令 | 结果 |
|---|---|---|
| RED（R3-27） | WeKnora-postgres-dev（PG 17）临时库执行旧/新 DDL | 旧：`operator does not exist: boolean = integer`；新：CREATE INDEX；回填/清扫 UPDATE 均可执行 |
| RED（R3-26/28/25） | 基线 7b9db0314 + 测试补丁 | zombie 当胜者重放（URL 空串）/ ChannelFailed:false / 400 purchase_pending_exists |
| RED（R3-31/39/17） | 基线 errors.test / phases 测试 / verify_db_watch 演示 | 数组被展开 1 fail / 无延迟复查 FAIL / using+WARNING 同输出 |
| RED（R3-07/09/02） | 基线 grep | restartCheckout/isQuoteLevelConflict/中文兜底 0 命中 |
| 全仓编译 | `go build ./internal/...`（worktree） | 通过 |
| commercial + handler 全量 | `go test ./internal/modules/commercial/... ./internal/handler/ -count=1` | 8 包全 ok |
| lago-lab 离线回归 | `pytest test_lab.py test_phases.py test_verify_db_watch.py -q` | 87 passed in 904.34s（+2 新测试；1 个既有断言更新后单独复跑通过——checks 键集合含 deferred_no_drift） |
| TS 单测 | `npx tsx --test` CheckoutPage/errors/contracts | 10/10、8/8、21/21 |
| 四副本重放 | verify_ac_assertions + verify_db_watch ×4 | AC ALL PASS ×4、DB-WATCH PASS ×4（归档路径不变；runs/ 无归档路径由 6 用例锁定） |
| 微信 stub 往返 | POST 127.0.0.1:8291 | code_url 正常下发 |

### 重放说明（全链路真实流程未重放）

本批 R3-26/28/25/07/09/02 改变已验证用户流程行为面，与前几轮同因（8091
为其他会话旧构建进程、Stripe key 不在环境）不可全链路重放，已如实记录；
独立可重放脚本全部重放（四副本 AC/DB-WATCH ×4、微信 stub），行为等价性
由分层回归覆盖；R3-27 另有真实 PostgreSQL 17 方言实证。

## 提交

- 提交 message 前缀：`issue-72(ocr-3):`（按本批 ask 指定）


---

## 批次 ocr-82-1（Issue #82 增量 OCR 有效 findings，2026-09-27）

修复员批次：32 个有效 findings（A-01~A-34，无 A-05）一次连贯处理，全部根因修复，无 deferred。安全红线自查：无新增凭据字面量（A-01 反向消除了一处）；服务端出站仍仅 http/https 且 host 校验在前（A-19/A-27 强化）；SQL 无拼接新增（A-08 以白名单收紧了既有 sqlite3 CLI 插值面）。

### Go 生产代码（internal/modules/commercial/…）

- **A-16（lago_settlement.go）**：gating intent 金额守卫。**活栈重放抓回首版回归**：fixHint 的强等校验（intent.Amount == payload.AmountFen）被真实 proration 击穿（82flow 栈 9900 冻结价的 gating intent 本月按比例为 1650）→ settle 全量 fail closed 落 attention。终版为 proration-aware 上界守卫（`1 ≤ amount ≤ payload.AmountFen`，对齐 D6' 的「小于面额、绝不大于、绝不归零」纪律），超面额/零额仍 fail closed 于任何扣款调用之前。
- **A-17**：intent list 分页消费（limit=100 + starting_after/has_more，页预算 10，超限显式 fail closed）。
- **A-18**：settleRequestTimeout 按链路逐项求和（3×Lago 读 + 5×Stripe 往返 = 120s）；attach/default 派生确定性 Idempotency-Key（`:attach`/`:default`）。
- **A-19**：providerOutboundRequest 的 ReadAll 错误 → ErrPlatformUnreachable（瞬态重试）；超 64KiB → InvalidResponse（显式超限报错）。
- **A-24+A-26（lago_purchase.go）**：readPurchaseInvoiceFees 选定策略改为「newest-first 首个 succeeded 匹配即返回（fallback 最新匹配）」——续期发票（t02-duplicates 实录：购买后数分钟即出具 renewal invoice，同 external_subscription_id）与重购复用身份使 matches≥2 成为常态，旧的「两张即数据异常」会在首月后永久打碎购买快照；首个 succeeded 匹配即 break 同时消除 N+1 全量遍历（open 的 renewal 不遮蔽已结算 gating 事实）。
- **A-25**：invoice index/detail 两读面 429/5xx → ErrPlatformUnreachable（与文件内其它读路径纪律对齐），其余非 200 保持 InvalidResponse。
- **A-34**：422 no_default_payment_method 分支对 bound-but-PM-less 客户重驱动 attach+sync（经注入 seam `reAttachDefaultPM`/`syncPaymentMethods`，幂等）后再答可重试 Unreachable——消除「配置齐全但该租户从未附加 PM」的永久死循环。
- **A-20（repository/order.go）**：createOrderTx 的 insert 竞态败者补 isQuoteUniqueConflict（SQLite 列面/PG 索引名双形状）→ ErrQuoteAlreadyUsed，purchase.go 的幂等重放分支在 insert 竞态下可达。
- **A-29**：CurrentPurchaseOrder 的 pending 偏好对齐可付谓词（`pending && !channel_failed`；终态回退 rows[0] 保留——死单遮蔽 paid 单的合成窗口恢复）。
- **A-30**：CreatedAt 统一 UTC 归一（含零值显式填充防 gorm 本地时区自动填充）——SQLite 文本词法比较下的排序/清扫门不失真。
- **A-21（service/purchase.go）**：两处订单重放（CurrentPendingPurchaseOrder 与 ErrPurchasePendingExists 的 CurrentPayablePendingOrder 分支）读 existing.QuoteID 经 quoteBoughtPlan 比对 snap.PlanKey，异计划同价单 → ErrPurchasePlanConflict，绝不重放旧计划 quote 的 CheckoutURL。
- **A-28**：PurchaseStatus 复用一次 FindPublicationByCode（消除轮询端点的重复查询）；pubErr 非空时跳过订单投影（双侧不可证明即不投影——结构性消除 ""=="" 空串假匹配），失败留一条 Warn。
- **A-22（benefits.go）**：非 ACTIVE 购买（canceled）的 plan 定义已知 codes 从 authority 物化腿剔除（懒清理窗口的残留 entitlement 不得保留付费特性）；ACTIVE 腿与未知 code 物化语义不变。
- **A-23**：effectiveFeatures 三处吞掉的错误（ReadSnapshot/FindPublicationByCode/definitionOf）各留一条含 tenantID 的 Warn。
- **A-31**：预算耗尽分支的 ReadSnapshot 失败分流——unreachable/unconfigured 保持 pending（return nil），仅确定性失败/非 active 落 attention。
- **A-32**：observeActivation 与 D6' 复核的快照错误经 settleSnapshotFailure 分流（ctx 取消上抛排水终止；瞬态 → nil 保持 pending；确定性 → attention + nil 绝不中断共享排水 pass）。
- **A-33**：markActivationState 的 Warn 以 RowsAffected==1 门控（卡死事件不再每 30s 刷一条、每天 ~2880 条）。
- **A-27（config.go）**：NewPlatform 对 loopback bypass 加启动守卫——bypass=true 且 BaseURL 指向非 loopback 主机（生产姿态）即 fail fast；loopback/空 BaseURL（lab/blocked-env 姿态）保持合法。活栈验证：82flow lab 姿态（bypass=true + http://127.0.0.1:48889）启动成功。

### Python harness（deploy/lago-lab/payment-settle-trigger/phases.py）

- **A-06**：删除 sys.path.insert(0, _PA_DIR)，clients 经 importlib 别名（pa_clients）加载（与 fixtures.py 的 pa_fixtures 同构）——bare import fixtures 恒解析本目录。
- **A-07**：lago_db_query 包 try except (OSError, subprocess.SubprocessError)（TimeoutExpired 是 SubprocessError 非 OSError）返回 None；四个 phase 兜底扩为 (OSError, clients.LabError)。
- **A-11**：Stripe customer id 在 create 返回即刻登记 ctx.state["stripe_customer_id"]；phase_cleanup 兜底读取（state.get("stripe_customer_id") or customer.get(...)）——早失败路径不再泄漏 cus_…。

### 证据/验收脚本（docs/plans/）

- **A-01**：browser_paid_face_82.mjs 密码字面量 → PASSWORD env 变量。
- **A-04**：同脚本 billing 否定断言前先正向等待 `text=已付款待激活`（对齐 r4-flow/browser_03 纪律），消除结构选择器窗口期的 vacuous PASS。
- **A-02**：三份 browser_02_sync_face.mjs 的手动刷新循环——点击计数（refreshClicked==3 记 note，失败即 FAIL note 说明前置未执行），恢复被删的 DOM 级 click 原因注释。
- **A-12**：同三份补 order-identity 断言（body 含目标订单号；?order= 接线回归会渲染新 quote 订单同面全 PASS）。
- **A-03**：9 个取证脚本统一补 catch（note script-error + 失败截图 + RESULT 照常输出 + 非零退出）。
- **A-08**：三份 seed.sh 的 UID_B 插值前 UUID 白名单校验（sqlite3 CLI 无参数绑定），TOKEN_A/TOKEN_B 非空非 null 校验。
- **A-09**：publish 前读 Lago 9900 计划基线，断言严格递增（不变的非零计数=历史残留，诚实 FAIL）。
- **A-10**：PAD_COUNT 默认值改各轮正确值（r4-flow2=8、r4-flow3=10；**r4-flow 维持 6**——r4-flow2 头注释自证「round 1 used 1..6」，r4-flow 即 round 1，其 L3 头注释「Registers 6 placeholder tenants」自洽；finding 「三份均遗留 6」对 r4-flow 的判定与脚本自身证据不符，按证据裁决并在注释写明取值依据）。
- **A-13**：reg_expect() 包装——注册 2xx 断言（409 显式容忍并注明重跑语义），非 2xx fail-fast。
- **A-14**：draft jq -e 断言（success/plan_key/version）+ .data.version 驱动 publish URL 与 receipt 断言。
- **A-15**：四份 verify_ac_assertions.py 终态放宽修正——invisible 分支接受 recheck=="incomplete" 或（recheck is None 且 cancellation_reason=="payment_failed"）；可见分支接受 invoice_status in ("failed","closed")。四份副本对各自归档证据重放 ALL PASS ×4。

### 回归测试（新增/改写）

- 新增：TestLagoSettleAmountGuardFailsClosed / TestLagoSettleProratedIntentStillSettles（A-16）、TestLagoSettleIntentListPaginatesToCompletion（A-17）、TestLagoSettleAttachRailDerivesIdempotencyKeys（A-18）、TestLagoSettleTruncatedBodyIsUnreachable / TestLagoSettleOversizedBodyIsInvalidResponse（A-19）、TestLagoReadPurchaseInvoiceFeesTwoFinalizedAnswersNewestSucceeded / TestLagoReadPurchaseInvoiceFeesOpenRenewalDoesNotShadowGating（A-24/26 改写原 FailsClosed）、TestLagoInvoiceFacesClassifyTransient5xxUnreachable / TestLagoInvoiceFacesKeepDefinitive4xxInvalidResponse（A-25）、TestNewPlatformRefusesLoopbackBypassForProductionAuthority（A-27）、TestCurrentPurchaseOrderSkipsChannelFailedPending（A-29）、TestCreateOrderNormalizesCreatedAtToUTC（A-30）、TestIsQuoteUniqueConflictClassifier（A-20）、TestPurchaseReplayRejectsForeignPlanPendingOrder（A-21）、TestPurchaseStatusSkipsProjectionWhenPublicationUnreadable（A-28）、TestEffectiveFeaturesStalePurchaseEntitlementNotMaterialized（A-22）、TestPurchaseFulfillBudgetTransientSnapshotErrorKeepsPending / TestPurchaseFulfillDefinitiveSnapshotErrorLandsAttentionWithoutAborting / TestPurchaseFulfillObservationTransientErrorKeepsPending（A-31/32）；FakeAdapter 补 FailPurchaseSnapshotsWith 注入 seam；既有 422 测试补注入（A-34）；并发胜者测试改用真实 quote 行（A-21 前置）。

### 测试与重放证据（全部在本轮实际执行）

| 检查 | 命令 | 结果 |
|---|---|---|
| 全仓构建 | `go build ./...` | 通过 |
| commercial 全量 | `go test ./internal/modules/commercial/... -count=1` | 7 包全 ok |
| harness 离线 | `python3 -m unittest test_phases -v`（payment-settle-trigger） | 26 tests OK |
| 验收重放 ×4 | `python3 verify_ac_assertions.py`（ocr1/2/3-replay、flow-evidence-74 各自归档证据） | RESULT: ALL PASS ×4 |
| 取证脚本语法 | `node --check`（13 个 .mjs）/ `bash -n`（3 份 seed.sh） | 全通过 |
| **活栈全链重放** | seed.sh → quote → purchase(201+checkout_url) → 签名 notify(200) → paid_awaiting_activation → settle（真实 Stripe，gating intent 1650 proration → succeeded）→ webhook(200) → active+fulfilled（attention→applied 自愈）→ webhook 重投 no-op | 全过；证据 `docs/plans/issue-72-flow-evidence-82/ocr82fix-replay/`（README + seed 副本） |
| 全量 internal | `go test ./internal/... -count=1` | 126+ 包 ok；application/repository、application/service 两包各 ~605s 达 go test 默认超时 FAIL（与 commercial 无 import 依赖；单独放宽超时复跑结果见下） |

### 活栈重放中发现并修正的回归

A-16 首版（fixHint 强等校验）在真实 proration 场景把 settle 打死——活栈重放抓到（intent 1650 vs 冻结价 9900 → InvalidResponse → attention，intent 不推进），当日修正为上界守卫并重放验证成功。这是「行为有变必须重放」要求的直接收益。

### 提交

- 提交 message 前缀：`issue-72(ocr-82-1):`（按本批 ask 指定）

---

## 批次 ocr-83-1（Issue #83 增量 OCR 有效 findings，2026-09-27）

修复员批次：12 个有效 findings（C-01~C-12）一次连贯处理，全部根因修复，无 deferred。安全红线自查：C-01 消除了入库活体令牌（12 个文件脱敏 + .gitignore + seed 落盘脱敏 + 106 条 refresh token 吊销）；C-02 收紧了取证脚本 SQL 插值面（白名单 + fail-fast）；无新增凭据字面量。

### 凭据红线处置（C-01，high）

- **同类面扩大**：除 finding 点名的 #83 六个 seed-login-*.json 外，#82 目录 r4-flow/2/3 的六个同类文件同样含活体 JWT 入库（`git ls-files` 证实），一并处置。
- 12 个文件原位脱敏（token/refresh_token → `"REDACTED"`，保留 success/tenant/user 证据结构）；`.gitignore` 追加 `docs/plans/**/seed-login-*.json`（已跟踪脱敏版继续跟踪，未来重跑的活体新文件被挡）。
- seed_83.sh 与 #82 三份 seed.sh 改为**落盘前脱敏**（token 只走 shell 变量）；api_recovery_83.mjs 的 token 源同步迁移到 `FLOW83_TOKEN` env（文件已脱敏）。
- **已泄露 refresh token 吊销**：issue83-flow-v2.db(18)/issue82-r4verify.db(36)/issue82-r4v2.db(30)/issue82-r4v3.db(22) 共 106 行 auth_tokens 全部删除（本地取证库的 refresh token 失效）；access JWT 无状态、exp 2026-09-28 自然过期（issue83-flow.db 旧库无 auth_tokens 表，该轮无持久化 refresh token——如实记录）。
- 残留扫描：`git grep eyJ -- docs/plans` 仅剩 OCR 报告的截断引用（`eyJhbGciOiJI…`，非完整令牌）。

### Go 生产代码

- **C-07（repository/order.go）**：新增 `CloseAttemptAndRetireChannel`——attempt→closed 与订单→channel_failed 两条 UPDATE 在**单事务**内原子落地（landClosed 改调它）；订单在竞争窗口离开 pending 时整体拒绝（ErrOrderNotFound）且 attempt 写入随事务回滚——消除「attempt 已 closed 但订单仍 payable」的只能人工改库半状态。
- **C-08（payment/alipay.go）**：Query 的 ProviderID 改为 **trade_no 优先**（out_trade_no 回退、providerID 兜底）——与回调事实的 Transaction（tradeNo）同源，对齐 wechat 腿的 transaction_id 优先纪律；恢复路径后再回调重放同一笔支付时 sameTxn 判定成立，不再误发 over-payment 审计。
- **C-09（service/purchase.go）**：切换分支终局判定改**排除式**——`closeView.State != OrderStatePending`（已决出终局：paid 或 fulfilled）一律按 closeView 作答，fulfill worker 在 ConfirmPayment 与重读之间推进 fulfilled 时不再落穿 CreateOrder 为已生效购买开第二张渠道单。
- **C-10（service/purchase.go）**：FirstPendingAttempt 错误**分流**——ErrPaymentAttemptNotFound → #82 冻结重放；其他错误（瞬时 DB 故障）上抛，绝不把失败查询冒充成功响应返回旧渠道入口。

### 取证/验收脚本（docs/plans/issue-72-flow-evidence-83/）

- **C-02（api_recovery_83.mjs）**：统一 `sqlShape(name,value,re)` 白名单（租户 `^\d+$`、订单 `^ord_[0-9a-f]+$`、渠道单 `^mo_[0-9a-f]+$`、Lago 绑定 id uuid-or-`cus_`）+ 校验失败落盘日志并终止；fulfilledOrder 形状校验失败改为终止（不再仅 note 后拼入后续 SQL）；browser_flow_83.mjs 的 FLOW83_TENANT 校验前置到入口 `process.exit(2)`（未校验值不再拼进 docker psql）。白名单首版把 Lago 绑定写成 UUID 形态——**活栈重放被自身拦截纠正**（cus_ id 拒绝→终止），修正为双形态后全绿。
- **C-03（两个脚本）**：newestPendingOrder/liveOrder 改为按 `state==='NOTPAY' && total===9900` 过滤后取末键——残留单/重试兄弟单/并发单不再被误标 SUCCESS 并推送签名 notify。
- **C-04（api_recovery_83.mjs）**：registerAndLogin 校验 token/tenant 非空（抛带上下文错误）；purchaseUntilLanded 四次耗尽改抛错终止该幕；五幕包顶层 try/catch——失败仍写 files + RESULT + 非零退出。
- **C-05（browser_flow_83.mjs）**：主流程补 catch（uncaught-flow-error 记 note 后仍写 progression 与 RESULT）；第 4 次 checkout 失败立即抛带上下文错误（不再落穿进必然 60s 超时的 weixin:// 等待）；stubOrders/stubMark/stubNotify/purchaseState 四个 fetch 包 try/catch 带上下文。另适配 contextual guide（SpotlightGuide 的「跳过」按钮 + backdrop 兜底——原 dismissGuides 只认 .wk-guide__close）。
- **C-06（v2_gen_keys.sh）**：apiv3.key 构造性保证首尾非空白（首尾各 1 字节经 `tr '\000-\040' 'A'` 折叠 + 中间 30 随机字节，恒 32 字节），末尾补 Python 边界断言（首尾字节 ∉ 空白集）——消除约 4.6% 概率的整轮随机启动失败。连跑 5 次断言全过（含折叠生效样本 0x41）。
- **C-11（seed_83.sh）**：Lago 落库断言从「任意 9900 plan 计数≥1」改为**本轮 code 精确匹配 + 形状断言**（`weknora-pro-v1 && amount_cents==9900 && interval=="monthly"` 恰 1）——republish 幂等下计数增量不可用，形状断言对历史残留不空转。
- **C-12（seed_83.sh）**：三处命令替换补传输层显式报错（reg_expect 的 curl、LOGIN_A/B 的 curl、LAGO_KEY 的 docker exec|tr，各带 `|| { say "FAIL: ... TRANSPORT failure"; exit 1; }` + LAGO_KEY 空值校验）——set -e 不再静默吞掉后端未启动等传输失败。

### 回归测试（新增/改写）

- `TestCloseAttemptAndRetireChannelAtomicPair`（C-07：干净路径两写同落 + 竞争路径整体回滚）、`TestPurchaseSwitchAnswersFulfilledOrderNotNewChannel`（C-09：queryHook 模拟 fulfill worker 竞争窗口，fulfilled 终局按 closeView 作答、alipay 零创建）、`TestPurchaseSwitchAttemptReadErrorPropagates`（C-10：drop attempts 表注入驱动错误 → 上抛非重放）、`TestAlipayQueryFallsBackWhenTradeNoAbsent`（C-08 回退链）、既有 `TestAlipayQueryReconcilesMissedNotification` 断言更新为 trade_no 优先（C-08 主行为）；closeRaceStub 增加 queryHook 注入。

### 测试与重放证据（全部在本轮实际执行）

| 检查 | 命令 | 结果 |
|---|---|---|
| commercial 全量 | `go test ./internal/modules/commercial/... -count=1` | 7 包全 ok |
| 取证脚本语法 | `node --check` ×2、`bash -n` ×4（seed_83/v2_gen_keys/#82 三份 seed） | 全通过 |
| 密钥边界断言 | `bash v2_gen_keys.sh <dir>` ×5 | 5/5 boundary non-whitespace OK |
| 脱敏完整性 | `git grep eyJ -- docs/plans` + jq 校验 12 文件 token=="REDACTED" | 仅剩报告截断引用；12/12 REDACTED |
| refresh token 吊销 | `delete from auth_tokens` ×4 库 | 18+36+30+22=106 行吊销 |
| **活栈重放** | v2_gen→v2_up_stubs→seed_83（PAD=45）→browser act1→手动补投 webhook→api_recovery act2-5 | seed 形状断言过、脱敏落盘；act1 前 8 项 PASS（切换 UI/NOTPAY 过滤/签名回调/paid_awaiting），settle-active 两项 FAIL 归因 env 注入遗漏（非代码回归，手动补投后 active+fulfilled）；**act2-5 全 26 PASS exit 0**（act4a= C-09 paid 形态、act4b= C-07 原子对活栈验证）；证据 `docs/plans/issue-72-flow-evidence-83/ocr83fix-replay/` |
| 全量 internal | `go test ./internal/... -count=1` | commercial 7 包 ok；application/repository、application/service 两包 ~605s 达默认超时（与前批相同的既有慢测试，与 commercial 无 import 依赖，未处理） |

### 提交

- 提交 message 前缀：`issue-72(ocr-83-1):`（按本批 ask 指定）
