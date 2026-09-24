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
