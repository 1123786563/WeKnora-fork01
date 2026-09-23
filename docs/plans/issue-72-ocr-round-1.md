Review complete: 10 finding(s) across 27 selected item(s).

─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:17-17 ───
[bug · medium] `glob.glob(...)[-1]` 未处理结果为空的情况：脚本依赖的
`deploy/lago-lab/payment-activation/runs/db-watch-verify-*.tsv` 是被
`deploy/lago-lab/payment-activation/.gitignore`（`runs/` 条目）忽略的运行时产物，不随仓库提交（当前检出中亦无任何匹配文件）。在全新克隆或
runs/ 被清理的环境运行时，对空列表取 `[-1]` 会直接抛 `IndexError: list index out of
range`，无法区分"观察者证据缺失"与"校验失败"，报错不具备可操作性。建议先判空并给出明确的缺证据提示后再索引。

- path = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))[-1]
+ matches = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))
+ if not matches:
+     print(f"DB-WATCH: no observer TSV under {RUNS} (db-watch-verify-*.tsv; runs/ is git-ignored)")
+     sys.exit(2)
+ path = matches[-1]


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:73-73 ───
[test · medium] `ok_states_a` 存在空真（vacuous pass）问题：当 `sub_a` 为空列表时（观察者 TSV 的 `rows[...]` 格式变化、tag
不匹配、或 sub-a 行从未被采样到），`set() <= {4}` 恒为 True，该断言静默通过而非失败。脚本对 sub-c 已明确加了存在性守卫（`sub_c_seen =
len(sub_c) > 0`），sub-b 也通过 `seen_active` 隐式要求至少一次正向观测，唯独 sub-a 缺少同类守卫，会导致 "DB-WATCH: PASS"
在证据缺失时仍然通过，削弱校验可信度。建议补上存在性检查。

- ok_states_a = set(sub_a) <= {4}           # A stays incomplete pre-charge window
+ ok_states_a = len(sub_a) > 0 and set(sub_a) <= {4}  # A seen; stays incomplete (4) pre-charge window


─── deploy/lago-lab/payment-activation/phases.py:1212-1214 ───
[bug · high] settled（已 canceled）分支的 `invoice_unpaid_terminal` 检查与同文件自身的发票可见性假设矛盾：phase_manual
docstring 和 phase_retries 注释都明确指出 v1.53.0 下 `closed` 与 `open` 同属 INVISIBLE_STATUS、客户发票 API
不会返回，因此当客户 C 的订阅取消后发票终态为 closed 时，`_gating_invoice` 返回 None，`observed["invoice_status"]` 为
None，该检查必然为 False —— 行为完全正确的运行会被误判为 FAIL。这与 expected 文本 "invoice in an unpaid terminal state (closed
or failed) whenever the API can see it" 直接矛盾（"whenever the API can see it" 已承认不可见情形）。settled 分支虽在
timeout_hours:0 的实验窗口内未被触发（两份 t02-decline.json 证据均走 not settled 分支），但该分支的存在正是为了覆盖取消可达的情形，一旦走到 closed
终态就会产生错误验收结论。建议在发票不可见时放行该项（`invoice_api_visible` 已在上方记录），并可附 ctx.note 说明。

-             "payment_failed_reason": observed["cancellation_reason"] == "payment_failed",
-             "invoice_unpaid_terminal": observed["invoice_status"] in ("closed", "failed"),
-             "entitlements_unusable": es == 404,
+             "invoice_unpaid_terminal": (
+                 observed["invoice_status"] in ("closed", "failed")
+                 or not observed["invoice_api_visible"]
+             ),


─── deploy/lago-lab/payment-activation/clients.py:365-366 ───
[bug · medium] 传输失败重试对非幂等 POST 不安全：该通道承载 `POST /v1/customers`（create_customer）等创建型调用，若首次请求已达 Stripe
而响应丢失（读超时/连接重置），重试会创建第二个 Stripe 客户——create_customer 返回的是第二个 id，第一个客户成为孤儿对象且 cleanup 只删除记录在案的
id（state.customers 中只存最终返回的那个），泄漏到测试账户中。这也与 phases.py 模块头声明的实验准则 "Unknown-outcome POSTs are never
blind retried" 相悖。attach/set_default/delete 本身幂等，风险集中在 create。建议在构造 Request 时生成一次
Idempotency-Key（重试复用同一 Request 对象即复用同一 key，Stripe 会重放原始响应），或仅对幂等调用启用 transport_retries。

      def _form(self, method, path, params=None, transport_retries=2):
          """One form-encoded Stripe call.
+ 
+         ...
+         """
+         body = urlencode(params or {}).encode("utf-8")
+         request = Request(
+             f"{self._base_url}{path}",
+             data=body if method != "DELETE" else None,
+             headers={
+                 "Authorization": f"Bearer {self._api_key}",
+                 "Content-Type": "application/x-www-form-urlencoded",
+                 # one key per logical call: a transport retry replays the
+                 # original response instead of re-executing a create POST
+                 "Idempotency-Key": secrets.token_urlsafe(16),
+             },
+             method=method,
+         )


─── deploy/lago-lab/payment-activation/clients.py:393-397 ───
[maintainability · low] `last_error` 是死存储：只在 except 分支被赋值，从未被读取（最后一次尝试直接 `raise` 原异常，保留了完整回溯）。连同循环前的
`last_error = None` 一并删除，或改为在重试前记录日志/note 以提供诊断价值。

-             except (URLError, OSError) as error:
-                 last_error = error
+             except (URLError, OSError):
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/run_lab.py:264-267 ───
[maintainability · low] 阶段顺序出现两个相互矛盾的定义：本次将 manual 移到 activate 之后，但 phases.py 末尾的
`PHASE_ORDER`（1313-1323 行）仍是旧顺序（gate -> manual -> activate），且 run_lab.py 模块 docstring 第 10 行仍写
"setup -> provider_setup -> gate -> manual -> activate -> ..."。`PHASE_ORDER`
目前虽无调用方，但它作为可导入的执行顺序契约与实际运行顺序分叉后，后续维护者若改用它（或按 docstring 理解）会重新引入本次修复的缺陷——manual 在 activate 前运行时 gate
发票不可见且 activation 尚未产生，导致 manual 被 blocked。建议同步更新 phases.PHASE_ORDER 与模块 docstring，或删除冗余的
PHASE_ORDER。

          # manual runs after activate: on v1.53.0 the 3DS gate invoice stays
          # API-invisible (open/closed are INVISIBLE_STATUS), so the manual-403
          # probe needs customer B's finalized invoice as its target.
+         # Keep phases.PHASE_ORDER and this module's docstring in sync with
+         # this order.
          ("manual", phases.phase_manual),


─── deploy/lago-lab/payment-activation/phases.py:539-540 ───
[bug · medium] 新增的三个 `_subscription_show` 调用（本行的 incomplete 探测、下方 canceled 探测、以及 invoice=="failed"
分支里的 canceled 探测）都不在任何 try/except OSError 内。`_subscription_show` 在传输错误时 note 后 re-raise，异常会穿透
phase_gate 到 run_lab.run_one 的兜底 `except Exception`，最终被记为 `fail`（unexpected_error，exit
1）——而模块头部的判定规则明确 "unreachable Lago API" 应为 `blocked-env`（exit 2）。phase_manual / phase_retries /
phase_decline_control 中同类调用都包了 `except OSError → _blocked`，建议这里保持一致（例如把发票可见性判定段包进同一个 try，OSError 映射为
blocked）。



─── deploy/lago-lab/payment-activation/phases.py:550-556 ───
[documentation · low] 新增的 PASS 路径（发票始终 API 不可见、未执行稳定窗口观察）与 phase_gate 开头的 `expected` 文案不符——expected
仍承诺 "gating invoice open/pending with no number ... state stable across the auth-challenge
window"。报告消费者会看到 expected 与 observed 不匹配却 status=pass 的证据。建议同步改写 expected，说明发票仅作上下文、AC1 以
subscription incomplete + entitlements 404 为准（与 decline_control/retries 的 expected 改写方式一致）。



─── deploy/lago-lab/payment-activation/phases.py:57-58 ───
[documentation · low] C 的支付方式改为 pm_card_authenticationRequired 后，phase_provider_setup 的 docstring（第
347-348 行）仍写着 "C: pm_card_visa_chargeDeclined (negative control)"，与映射表现状矛盾，会误导按 docstring
理解实验设置的读者。建议一并更新该 docstring。



─── deploy/lago-lab/payment-activation/fixtures.py:78-82 ───
[documentation · low] payload 已迁入 customer.billing_configuration 且 docstring 明确"顶层 payment_provider
键会被静默忽略"，但 fixtures.py 模块头（第 10-12 行）仍按顶层键声明契约："POST /api/v1/customers ... (payment_provider,
provider_customer_id, provider_payment_methods permitted)"。该文件的价值正是固化 pinned 契约文档，建议同步更新模块头为
billing_configuration 嵌套形态，避免读者按旧契约构造 payload。


