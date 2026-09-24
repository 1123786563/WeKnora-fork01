Review complete: 35 finding(s) across 86 selected item(s).

─── apps/web/src/commercial/CheckoutPage.tsx:101-103 ───
[bug · medium] purchaseErrorMessage 的闭合 reason 文案在真实失败路径上不可达，用户将看到英文后端错误。证据链：(1) 后端 POST /purchases
失败（409/503/404）返回非 2xx 的 {error, reason}，无 {success,data} 信封；(2) api-client 的 request
层（client.ts「status < 200 || >= 300 → errorFromResult」）对非 2xx 直接抛 ApiError，响应体根本不会进入
parsePurchaseView，且 errorFromResult 只提取 error/message/code，不提取顶层 reason 字段；(3) 成功路径（201/202）后端
service 的 purchaseView 恒带 Order，因此 !purchase.order 恒为 false——本分支连同 purchaseErrorMessage 的
unconfigured/unreachable/invalid_response/unsupported/canceled 映射对 POST 调用是死分支。结果：支付渠道未配置（503
unconfigured，未配置/lab 环境必现）、平台不可达等场景下，用户看到的是英文原文 "purchase temporarily unavailable"，与 R1-V01
注释声称的「按闭合 state/reason 渲染失败文案」直接矛盾。建议：在 catch 中按 ApiError.status 映射中文文案（至少覆盖 503/409），并让 reason
跨越错误链路（如后端将 reason 放入 error 对象的 details，或 errors.ts 提取顶层 reason 到 ApiError.details）后复用
purchaseErrorMessage。

-         if (!purchase.order) {
-           throw new Error(purchaseErrorMessage(purchase));
+       } catch (error) {
+         if (!active || !scopeController.isCurrent(currentScope.scope)) return;
+         // 非 2xx 失败不会到达 parsePurchaseView（reason 在 request 层丢失），
+         // 先按 ApiError.status 给出闭合中文文案；reason 透传打通后可细分。
+         setState({ status: 'error', message: purchaseSubmitErrorMessage(error) });
+       }
+ // 组件外：
+ function purchaseSubmitErrorMessage(error: unknown): string {
+   if (error instanceof ApiError) {
+     if (error.status === 503) return '支付平台暂时不可用，请稍后重试';
+     if (error.status === 409) return '报价或购买状态已变化，请返回重新获取报价';
+   }
+   return error instanceof Error ? error.message : '无法打开结算页';
-         }
+ }


─── apps/web/src/commercial/CheckoutPage.tsx:179-183 ───
[bug · low] 渠道调用失败但订单已持久化的场景（后端 202：order.checkout_error 非空、checkout_url
为空）页面会渲染「等待付款」+「待付款（权益未开通）」却没有「前往支付」链接，也没有任何错误提示；OrderView 契约未包含 checkout_error，轮询刷新（GET
/orders/:id）也只会拿到同样的空 checkout_url，用户无从知晓支付发起失败，只能永久停在待付款。建议在契约中透传 checkout_error（parseOrderView
校验为可选 string），并在 payment==='pending' 且无可安全渲染的 checkout_url 时展示渠道失败提示与恢复指引。

                {state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url) ? (
                  <p>
                    <a href={state.order.checkout_url} target="_blank" rel="noreferrer">前往支付</a>
                  </p>
+               ) : state.order.payment === 'pending' ? (
+                 <p className="wk-muted">支付发起失败（渠道未返回支付链接），请返回重新发起购买</p>
                ) : null}


─── apps/web/src/commercial/CheckoutPage.tsx:92-93 ───
[documentation · low] 重试按钮文案与本次变更矛盾：本变更已删除 idempotencyKeyRef 前端幂等键，重试语义改为后端按 quote+身份幂等返回同一订单，但
error 分支的按钮文案仍写着「重试（复用原订单与幂等键，不重复下单）」——「幂等键」机制已不存在，对用户和维护者均有误导。建议同步更新该文案（如「重试（同一报价不重复下单）」）。

          // #81：提交走 payment-gated purchase。重试语义：同一 quote 重试就是同一次
          // purchase 调用，后端按身份幂等返回同一订单（不产生第二张订单/第二张账单）。
+         // 注意：error 分支的重试按钮文案仍提「幂等键」，需同步改为不依赖已删除机制的表述。


─── apps/web/src/commercial/CheckoutPage.tsx:179-183 ───
[bug · high] 「前往支付」链接是一次性的：仅在初始 POST /purchases 响应中存在，约 3 秒后被轮询冲掉。证据链：(1) checkout_url
未落库——OrderRow/PaymentAttemptRow 无此列；(2) GET /orders/:id 走 RecoverOrderStatus，其所有返回分支（order.go
L379-387、L418-420）都不设置 CheckoutURL，orderWire 序列化为 "checkout_url": ""；(3) 本页轮询 effect 每
3s（POLL_INTERVAL_MS）执行 refreshOrder，用 GET 结果整体替换 state.order（{ ...prev, order }），链接随即卸载，而订单仍处于可支付的
pending 态。手动点「刷新订单状态」、携带 orderId 重进页面、以及 purchase 重试的 replay 路径（orderViewFromRow 不投影
checkout_url）同样拿不到链接——用户实际只有约 3 秒窗口完成支付跳转。与已确认的 checkout_error 缺失问题不同，这是成功路径（201 + 有效
checkout_url）下支付入口丢失。建议前端在 refreshOrder 合并时保留最后一次已知的安全 checkout_url，并在服务端持久化该字段使 GET /orders/:id 与
replay 路径回传（根因修复）。

-               {state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url) ? (
-                 <p>
-                   <a href={state.order.checkout_url} target="_blank" rel="noreferrer">前往支付</a>
-                 </p>
-               ) : null}
+ // refreshOrder 中合并而非整体替换，保留最后一次已知的安全 checkout_url：
+ void client.commercial.getOrder(id, currentScope.signal).then((order) => {
+   if (scopeController.isCurrent(currentScope.scope)) {
+     setState((prev) => (prev.status === 'ready'
+       ? { ...prev, order: { ...order, checkout_url: order.checkout_url ?? prev.order.checkout_url } }
+       : prev));
+   }
+ })


─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:49-50 ───
[bug · medium] AC1 不可见分支的断言未覆盖 phases.py 文档化的第二条 PASS 路径:gate phase 在 invoice API 不可见时,除 "recheck 仍为
incomplete" 外,还有 charge-failure endgame 路径(phases.py `_subscription_show(ctx, ext, "canceled")` 返回
canceled 且 `cancellation_reason == "payment_failed"` 时同样 `_report(..., PASS)`),且 t02-gating.json 的
expected 契约明确写有 "(or ends canceled(payment_failed) when the charge fails)"。endgame 路径下
`recheck_subscription_status` 为 None(incomplete 过滤查询已 404),此断言会把 status==pass 的合法证据误判为
FAIL,导致重放校验误报。建议与 phases.py 的两条 PASS 路径对齐,接受 endgame 终态。

-     check("AC1 invoice branch: still incomplete across window",
-           o.get("recheck_subscription_status") == "incomplete")
+     check("AC1 invoice branch: still incomplete across window or charge-failure endgame",
+           o.get("recheck_subscription_status") == "incomplete"
+           or o.get("cancellation_reason") == "payment_failed")


─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:51-54 ───
[test · medium] AC1 的 else 分支（invoice API 可见）同样未覆盖 phases.py 文档化的端局 PASS
路径，与已确认的"不可见分支缺口"是同族但不同位置的独立遗漏：phases.py 在 invoice 以 failed 状态可见且订阅
canceled(cancellation_reason=payment_failed) 时同样 _report(...,
PASS)（deploy/lago-lab/payment-activation/phases.py 的 invoice-visible-as-failed 分支），且该路径在写入
payments_non_succeeded_count 之前即返回。若某次 replay 的 t02-gating.json
命中该路径（invoice_api_visible=true、invoice_status="failed"、cancellation_reason="payment_failed"、status="
pass"），本脚本会因 invoice_status=="open" 不成立、payments_non_succeeded_count 落到缺省 99 而连报两条 FAIL——而
t02-gating.json 的 expected 契约明确写有 "(or ends canceled(payment_failed) when the charge fails)"。建议在
else 分支同样接受该端局。

+ else:
+     if o.get("invoice_status") == "failed":
+         # phases.py 可见 failed 端局: canceled(payment_failed) 同样是 AC1 的 PASS 路径
+         check("AC1 visible charge-failure endgame canceled(payment_failed)",
+               o.get("cancellation_reason") == "payment_failed")
- else:
+     else:
-     check("AC1 invoice open/pending/numberless",
+         check("AC1 invoice open/pending/numberless",
-           o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
+               o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
-           and not o.get("invoice_number"))
+               and not o.get("invoice_number"))
+         check("AC1 <=1 non-succeeded payment",
+               o.get("payments_non_succeeded_count", 99) <= 1)


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:111-115 ───
[bug · high] 回退守卫恒为假,是永不可达的死代码:`path` 仅在 `if archived.exists():` 分支中被赋值为 `str(archived)`,而 `elif
matches:`(runs/ 回退)只在归档不存在时执行,此时第一个条件 `archived.exists()` 已为 False。因此 `archived.exists() and path !=
str(archived)` 恒为 False,WARNING 打印与 `sys.exit(2)`(MISSING-EVIDENCE 降级)永远不会触发。这直接使上方 R1-V10
注释声明的安全网失效:一旦本目录归档 TSV 缺失而 runs/ 下残留外域(或多 run)运行的 db-watch-verify-*.tsv,脚本会用无法按 run 键隔离的整库 payments
聚合直接裁决 `max_succeeded()==1`,可能给出与本次运行无关的假 PASS/CHECK,破坏"付款恰好一次"验收证据的可信度。建议改为判定 `not
archived.exists()`(即任何 runs/ 回退都降级)或引入显式 `used_fallback` 标志。

- if archived.exists() and path != str(archived):
+ used_fallback = not archived.exists()
+ ...
+ if used_fallback:
      print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
            "whole-DB aggregate and cannot be keyed to this run, so the "
            "exactly-once max_succeeded assertion is not decidable")
      sys.exit(2)


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:11-13 ───
[documentation · low] docstring 的溯源元数据与实际不符：(1) ocr-2 副本实际为 99 行（其 diff 头 @@ -0,0 +1,99 @@），并非 "97
lines"；(2) 本文件中 "# ocr-2:" 注释实际位于第 77 与 92 行，"# ocr-3:" 注释位于第 82 与 98 行，而 docstring 声称 74/89 与
79/95，全部偏移 +3（疑似未计入本 docstring 相对 ocr-2 版本的增长）。该文件的核心宗旨是精确记录 OCR 重放差异，且 R1-V23
专门纠正过前次"unmodified"的错误声明，失真的行数/行号会误导后续审计与重放比对。建议按实际值修正。

- evidence directory. Derived from the ocr-2 copy (97 lines): it keeps the
- two "# ocr-2:" annotations (lines 74 and 89) and adds two ocr-3-specific
- assertion annotations, "# ocr-3:" at lines 79 and 95 (R1-V23: the previous
+ evidence directory. Derived from the ocr-2 copy (99 lines): it keeps the
+ two "# ocr-2:" annotations (lines 77 and 92) and adds two ocr-3-specific
+ assertion annotations, "# ocr-3:" at lines 82 and 98 (R1-V23: the previous


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:107-108 ───
[bug · high] 此降级守卫恒为假，是不可达的死代码：归档 TSV 存在时，上方第一个分支已令 path = str(archived)，第二个合取项 path !=
str(archived) 必为假；归档不存在时，第一个合取项 archived.exists() 直接为假。因此 R1-V10 注释声明的安全策略——"runs/ 回退 TSV 在用 → 打印
WARNING 并 exit(2) MISSING-EVIDENCE，避免在整库聚合 payments 数据上冒假 CHECK 风险"——永远不会执行。一旦证据目录缺少归档 TSV 而
runs/（git-ignored，db_watch.sh 每次调用写一个新时间戳文件）残留外来运行的 TSV，脚本会在自身注释认定"不可判定"的数据上继续执行 max_succeeded()==1
断言，外来 succeeded 计数会注入该整库聚合断言，可能产出假 CHECK 或给 payments 恰好一次不变量假信心。正确条件应为 `if path !=
str(archived):`（该缺陷由 ocr-2 副本同位置复制而来）。

  # degrade loudly to MISSING-EVIDENCE instead of risking a false CHECK.
- if archived.exists() and path != str(archived):
+ if path != str(archived):


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:26-26 ───
[documentation · low] ocr-3 副本未更新这处目录标注：注释说 "Prefer THIS evidence directory's archived copy
(ocr-2)"，但 EVID 是本脚本所在目录（即 docs/plans/issue-72-ocr3-replay/），"THIS evidence directory" 现在是 ocr-3 而非
ocr-2。该括号标注在 ocr-2 原件中用于指明"本 (ocr-2) 目录的归档副本"，复制到 ocr-3 后应同步改为 (ocr-3)；按本仓库对重放副本溯源标注的精确性要求（如 R1-V23
对错误 "unmodified" 声明的纠正），失真的目录指向会误导后续审计/重放比对时对归档优先策略适用范围的理解。

- # Prefer THIS evidence directory's archived copy (ocr-2): replayed observer
+ # Prefer THIS evidence directory's archived copy (ocr-3): replayed observer


─── internal/handler/commercial.go:486-492 ───
[bug · medium] Purchase 的 default
分支会接住未被哨兵覆盖的服务端内部错误：QuoteSnapshotForTenant→quotes.GetQuote、GetPublication/GetVersion、EnsureBillingAc
count 的 DB 错误（EnsurePending/MarkLinked，注释明确"Only DB errors return an error"）、OpenOrder/LatestVersion
的 DB 错误、以及 ErrInvalidQuoteRow 包装的损坏快照 JSON，这些都会以 400 + err.Error() 应答。后果有二：(1)
服务端故障被误标为客户端错误（400），与同文件 GetOrder 的 default→500 姿态不一致；(2)
原始错误文本（驱动/SQL/主机上下文）在公共边界回传，违背本端点注释声明的闭合词汇约束（503 分支专门强调 never raw error text，但 default
分支仍泄漏）。请求侧校验在调用 service 前已完成，落到 default 的错误几乎都是服务端问题，建议对齐 GetOrder 的 500 + 固定文案姿态。

  	case errors.Is(err, repocommercial.ErrQuoteAlreadyUsed):
  		// (R1-V21) Same race outcome as POST /orders: 409, not a generic 400.
  		c.JSON(http.StatusConflict, gin.H{"error": "quote already used"})
  	default:
- 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
+ 		// Remaining failures are server-side (DB outage, corrupt snapshot,
+ 		// context cancellation): answer 500 with a stable closed message
+ 		// (GetOrder posture) instead of echoing raw driver text as a 400.
+ 		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})
  	}
  }


─── internal/modules/commercial/commercialplatform/lago_purchase.go:335-335 ───
[performance · medium] 预算组合缺陷：createPurchaseSubscription 的总预算
purchaseRequestTimeout=25s（subscriptionRequestTimeout 15s+10s），但串行路径的标称预算之和远超：dev/test 配置 PM token
时最多 3 次出站 Stripe 调用（create/attach/default，各 ≤15s）+ 本处 20s 的 pmSyncWait 轮询 + 多次 Lago 调用（各 ≤5s
客户端预算）。任何一次出站超过数秒，20s 轮询就无法在总预算内完成，会以 ctx 取消的 "payment method sync interrupted" 而非设计的 "not imported
in time" 收场；即使全部出站正常（~1-2s/跳），3 跳 + 20s 轮询也 ≈26s > 25s。POST /commercial/purchases 在 handler
上同步执行（c.Request.Context()，无额外超时），单次请求可占用接近 25s；且用户重试时绑定已写入、跳过出站创建，每次都重新烧满约 20s 轮询窗口，无退避。建议：将轮询
deadline 从属于 ctx 剩余预算（至少让错误分类准确），并考虑缩短 pmSyncWait 或将绑定+同步等待移出同步请求路径（#82 checkout 回调驱动）。

  		return a.waitForPaymentMethodSync(ctx, externalCustomerID)
+ // waitForPaymentMethodSync 内：
+ // deadline := time.Now().Add(pmSyncWait)
+ // if d, ok := ctx.Deadline(); ok {
+ // 	if rem := time.Until(d); rem > 0 && rem < pmSyncWait {
+ // 		deadline = time.Now().Add(rem)
+ // 	}
+ // }


─── internal/modules/commercial/commercialplatform/lago_purchase.go:251-251 ───
[maintainability · low] 422 恢复路径丢弃了创建响应体（createPurchaseSubscription 中 `status, _, err :=
a.do(...)`），无法区分 "#76 重复身份 422" 与 "门控创建被拒"：占位前缀绑定（无真实 provider 客户）必然走到这里；另 ensureProviderBinding
对已绑定客户（bound 早退）跳过 waitForPaymentMethodSync，PM 导入未落地时也会在此收到 422。两种情况均被归类为终态语义的
ErrPlatformInvalidResponse "purchase replay cannot be verified"，误导 dev/test 排障与上层重试判定（服务层对
InvalidResponse 会先做 plan-conflict 探测再统一包 503）。建议参照 ensureProviderBinding 处理
payment_provider_not_found 的方式检视 422 响应体，将 no_default_payment_method 类拒绝归类为可重试的
ErrPlatformUnreachable。

+ 		if found && sub.PlanCode == payload.PlanCode {
+ 			return receipt, nil
+ 		}
+ 		if strings.Contains(string(respBody), "no_default_payment_method") {
+ 			return commercial.CommandReceipt{}, fmt.Errorf("%w: default payment method not imported yet", commercial.ErrPlatformUnreachable)
+ 		}
  		return commercial.CommandReceipt{}, fmt.Errorf("%w: purchase replay cannot be verified", commercial.ErrPlatformInvalidResponse)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:587-588 ───
[maintainability · low] providerAttachDefaultPaymentMethod 的 attach 与默认支付方式更新两次出站调用均未携带
Idempotency-Key（providerOutboundCall 已支持该参数）。响应丢失或后续 Lago 绑定写入失败导致整条命令重试时：Stripe 客户创建因带 key
幂等重放同一客户，但 attach 会对同一客户再次执行——Stripe 对 pm_card_* 令牌的 attach 会克隆出新的客户作用域 pm_
id（代码注释自述），重试一次即在该客户上累积一个克隆支付方式，且默认支付方式随最后一次克隆漂移。建议为两跳传入确定性幂等键（例如以 providerCustomerID+pmToken 派生并以
":attach"/":default" 区分）。

  	status, data, err := a.providerOutboundCall(ctx, "/v1/payment_methods/"+url.PathEscape(pmToken)+"/attach",
- 		"customer="+url.QueryEscape(providerCustomerID), "")
+ 		"customer="+url.QueryEscape(providerCustomerID), providerCustomerID+":"+pmToken+":attach")


─── internal/modules/commercial/commercialplatform/lago_purchase.go:219-222 ───
[bug · medium] 身份重放（及下方 422 恢复分支的 `found && sub.PlanCode == payload.PlanCode`）只比较 plan
code，不检查已持有订阅的 Status：一个 canceled/terminated 的购买同样被当作成功重放返回回执。购买身份是租户 ID
的纯函数（ExternalPurchaseSubscriptionID，D1），本变更集内没有任何重建/重开路径（Lago 也不会复用已取消订阅的 external_id
再建）：一旦购买被取消（readPurchaseSnapshot 明确预期 canceled/terminated 存在），后续每次 create_purchase_subscription
都回答"成功"回执，而 PurchaseService.Purchase 第 8 步会因 state != awaiting_payment 以 ErrPurchaseNotAwaiting
永久失败——租户购买能力不可恢复地死锁，且该终态与其他失败不可区分。建议重放成功仅限 incomplete/active，canceled/terminated 应 fail closed
并给出可区分的冲突错误（422 恢复分支同理）。

  		// Identity replay: same purchase + same plan → the same receipt,
  		// never a second subscription or a second gating invoice (F7).
+ 		switch sub.Status {
+ 		case "incomplete", "active":
- 		return receipt, nil
+ 			return receipt, nil
+ 		case "canceled", "terminated":
+ 			// A held but canceled purchase is a dead identity (no re-create
+ 			// path): fail closed with a distinguishable conflict instead of a
+ 			// success receipt for a subscription that can never activate.
+ 			return commercial.CommandReceipt{}, fmt.Errorf("%w: purchase canceled", commercial.ErrPlatformInvalidResponse)
+ 		default:
+ 			return commercial.CommandReceipt{}, fmt.Errorf("%w: unknown purchase status", commercial.ErrPlatformInvalidResponse)
+ 		}
  	}


─── internal/modules/commercial/commercialplatform/lago_purchase.go:364-366 ───
[bug · low] waitForPaymentMethodSync 对 2xx 响应体的 json.Unmarshal 失败被静默吞掉（只有 err == nil 且列表非空才返回）：持续畸形的
payment_methods 响应会让每次购买都空转完整 pmSyncWait（20s）预算，最终被误报为 ErrPlatformUnreachable（瞬态、可重试语义），违背本文件自述的
fail-closed 分类契约（malformed → ErrPlatformInvalidResponse，终态语义），也拉长了已被确认的预算组合问题的实际表现。建议解析失败立即按 invalid
response 终止。

- 			if err := json.Unmarshal(body, &parsed); err == nil && len(parsed.PaymentMethods) > 0 {
+ 			if err := json.Unmarshal(body, &parsed); err != nil {
+ 				return fmt.Errorf("%w: payment method read malformed", commercial.ErrPlatformInvalidResponse)
+ 			}
+ 			if len(parsed.PaymentMethods) > 0 {
  				return nil
  			}


─── internal/modules/commercial/payment/alipay.go:200-208 ───
[security · medium] 客户端使用默认 Transport，缺少拨号层 SSRF 防护：`ValidateURLForSSRF` → `isSSRFSafeURL` 内部通过
`net.LookupIP` 解析并校验 IP 后，实际请求由默认 Transport 在拨号时再次独立解析 DNS。两次解析之间存在 DNS rebinding TOCTOU 窗口（校验时返回公网
IP、拨号时返回 127.0.0.1/内网 IP 即可绕过 R1-V09 要求的拒绝规则；每个重定向跳同样受影响）。代码库其它出站路径（airesource chat
transport、opensearch、web_search proxy、mysql_ssrf）均以 `SSRFSafeDialContext` 在 TCP
汇聚点加固，`ssrf_outbound_cache.go` 注释也明确指出 URL 级校验需由 `SSRFSafeDialContext` 兜底。建议改用
`secutils.NewSSRFSafeHTTPClientWithTransport`：可同时获得拨号期 IP 校验、60s TTL 缓存的出站校验（避免 `call()`
每次请求都串行多做一次全量 DNS 解析）、重定向跳数上限和跨主机敏感头剥离。注意 `MaxRedirects` 需显式设置，为 0 时会拒绝所有重定向。

- 		client: &http.Client{
- 			Timeout: cfg.timeout(),
- 			CheckRedirect: func(req *http.Request, via []*http.Request) error {
- 				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
- 					return fmt.Errorf("alipay redirect target failed SSRF validation: %w", err)
- 				}
- 				return nil
- 			},
- 		},
+ 		client: secutils.NewSSRFSafeHTTPClientWithTransport(secutils.SSRFSafeHTTPClientConfig{
+ 			Timeout:      cfg.timeout(),
+ 			MaxRedirects: 10,
+ 		}, nil),


─── internal/modules/commercial/payment/alipay.go:202-207 ───
[bug · medium] 自定义 `CheckRedirect` 会禁用 net/http 默认的"连续 10 次请求后停止"重定向策略（该默认仅在 `CheckRedirect` 为 nil
时生效），而此闭包未检查 `len(via)`。恶意或异常网关可以在都通过 SSRF 校验的公网主机之间循环 302，请求会一直跟随重定向直到客户端 Timeout（默认
10s）耗尽，把支付/退款/查询操作拖满超时并占用调用方资源。建议在闭包内加跳数上限（若已按上一条意见改用 `NewSSRFSafeHTTPClientWithTransport`，其
`newSSRFCheckRedirect` 已强制 `MaxRedirects`，可忽略本条）。

  			CheckRedirect: func(req *http.Request, via []*http.Request) error {
+ 				if len(via) >= 10 {
+ 					return fmt.Errorf("alipay too many redirects")
+ 				}
  				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
  					return fmt.Errorf("alipay redirect target failed SSRF validation: %w", err)
  				}
  				return nil
  			},


─── internal/modules/commercial/payment/alipay.go:560-562 ───
[maintainability · low] 此处 SSRF 校验失败包装的哨兵是 wechat.go 中定义的 ErrNotConfigured（错误串为
"wechat_not_configured"），而不是本文件的 ErrAlipayNotConfigured。Alipay 网关未通过 SSRF
校验（例如误配为内网地址且未加白名单）时，向上抛出的错误形如 "alipay precreate xxx: wechat_not_configured: gateway url failed SSRF
validation: ..."，运维据此排查会被误导到微信渠道配置。本文件其余 fail-closed 路径（构造器、merchantKey）均使用
ErrAlipayNotConfigured，此处应保持一致。另外内层 err 用 %v 丢失了原始错误身份，与同文件 CheckRedirect 中 %w 的写法不一致，建议一并改为 %w（不影响
wechat_test.go 对 ErrNotConfigured+"SSRF" 文本的断言，该断言针对的是 wechat.go 的对应路径）。

  	if err := secutils.ValidateURLForSSRF(p.cfg.gateway()); err != nil {
- 		return fmt.Errorf("%w: gateway url failed SSRF validation: %v", ErrNotConfigured, err)
+ 		return fmt.Errorf("%w: gateway url failed SSRF validation: %w", ErrAlipayNotConfigured, err)
  	}


─── internal/modules/commercial/payment/wechat.go:188-196 ───
[security · medium] 客户端使用默认 Transport，缺少拨号层 SSRF 防护：`ValidateURLForSSRF` → `isSSRFSafeURL` 内部通过
`net.LookupIP` 解析并校验 IP 后，实际请求由默认 Transport 在拨号时再次独立解析 DNS。两次解析之间存在 DNS rebinding TOCTOU 窗口（校验时返回公网
IP、拨号时返回 127.0.0.1/内网 IP 即可绕过 R1-V09 要求的拒绝规则；每个重定向跳同样受影响）。代码库其它出站路径（airesource chat
transport、opensearch、web_search proxy、mysql_ssrf）均以 `SSRFSafeDialContext` 在 TCP
汇聚点加固，`ssrf_outbound_cache.go` 注释也明确指出 URL 级校验需由 `SSRFSafeDialContext` 兜底。建议改用
`secutils.NewSSRFSafeHTTPClientWithTransport`：可同时获得拨号期 IP 校验、60s TTL 缓存的出站校验（避免 `do()` 每次请求都串行多做一次全量
DNS 解析，也不会因预检 DNS 瞬时失败而误报 ErrNotConfigured）、重定向跳数上限和跨主机敏感头剥离。注意 `MaxRedirects` 需显式设置，为 0 时会拒绝所有重定向。

- 		client: &http.Client{
- 			Timeout: cfg.timeout(),
- 			CheckRedirect: func(req *http.Request, via []*http.Request) error {
- 				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
- 					return fmt.Errorf("wechat redirect target failed SSRF validation: %w", err)
- 				}
- 				return nil
- 			},
- 		},
+ 		client: secutils.NewSSRFSafeHTTPClientWithTransport(secutils.SSRFSafeHTTPClientConfig{
+ 			Timeout:      cfg.timeout(),
+ 			MaxRedirects: 10,
+ 		}, nil),


─── internal/modules/commercial/payment/wechat.go:190-195 ───
[bug · medium] 自定义 `CheckRedirect` 会禁用 net/http 默认的"连续 10 次请求后停止"重定向策略（该默认仅在 `CheckRedirect` 为 nil
时生效），而此闭包未检查 `len(via)`。恶意或异常端点可以在都通过 SSRF 校验的公网主机之间循环 302，请求会一直跟随重定向直到客户端 Timeout
耗尽，把支付/退款/查询操作拖满超时并占用调用方资源。建议在闭包内加跳数上限（若已按上一条意见改用 `NewSSRFSafeHTTPClientWithTransport`，其
`newSSRFCheckRedirect` 已强制 `MaxRedirects`，可忽略本条）。

  			CheckRedirect: func(req *http.Request, via []*http.Request) error {
+ 				if len(via) >= 10 {
+ 					return fmt.Errorf("wechat too many redirects")
+ 				}
  				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
  					return fmt.Errorf("wechat redirect target failed SSRF validation: %w", err)
  				}
  				return nil
  			},


─── internal/modules/commercial/repository/commercial/order.go:233-244 ───
[bug · medium] 以 (tenant, kind=purchase, amount_fen, currency) 匹配"当前购买"过于宽泛:OrderRow 无
created_at/plan 关联列,租户历史上同价位的历史购买、报价过期前已开出的废弃 pending 单都会落入匹配集;pending 优先叠加随机 hex 的 id ASC 只能任意挑选(多张
pending 时取哪张不确定),废弃 pending 单永远不会有终态(无超时回收任务),PurchaseStatus 的 out.Order 可能长期指向错误/过期订单——与
purchase.go 的跨报价重复开单问题叠加后,支付入口可能指向旧订单。建议至少收紧到 pending 单并与当前报价/发布计划建立显式关联(如在订单行落 plan_code
或创建时间列),同时评估 Find 全量加载加 LIMIT 的必要性。

  	var rows []OrderRow
  	if err := s.db.WithContext(ctx).
- 		Where("tenant_id = ? AND kind = ? AND amount_fen = ? AND currency = ?",
- 			tenantID, domain.OrderKindPurchase, amountFen, currency).
- 		Order("id ASC").Find(&rows).Error; err != nil {
+ 		Where("tenant_id = ? AND kind = ? AND amount_fen = ? AND currency = ? AND state = ?",
+ 			tenantID, domain.OrderKindPurchase, amountFen, currency, domain.OrderStatePending).
+ 		Order("id ASC").Limit(1).Find(&rows).Error; err != nil {
  		return OrderRow{}, err
- 	}
- 	for _, row := range rows {
- 		if row.State == domain.OrderStatePending {
- 		return row, nil
- 		}
+ 	}
+ 	if len(rows) > 0 {
+ 		return rows[0], nil
  	}
+ 	// TODO(#81): 为订单行落 plan_code/created_at 关联列后再回退历史单,
+ 	// 避免同价位历史购买被投影为当前购买。


─── internal/modules/commercial/service/commercial/order.go:256-258 ───
[maintainability · low] 两个分层卫生问题:1) QuoteSnapshotForTenant 是导出方法却返回未导出类型
quoteSnapshot(包外无法声明该返回值,golint/revive 会报 exported ... returns unexported type);当前仅同包使用可编译,但这是导出 API
契约瑕疵——建议改用包内小写命名(如 quoteSnapshotForTenant),与"quoteForTenant 保留历史私有名"的注释意图一致;2) PurchaseService
多处直接穿透 s.orders.orders 访问 OrderStore(GetOrderByQuote/CurrentPurchaseOrder),绕过 OrderService
封装,与文件头"every read goes through the repository stores inside the collaborators"宣称的协作边界相悖——建议在
OrderService 上加两个薄包装(OrderByQuote/CurrentPurchaseOrderFor)统一收口。

- func (s *OrderService) QuoteSnapshotForTenant(ctx context.Context, tenantID uint64, quoteID string) (repocommercial.QuoteRow, quoteSnapshot, error) {
+ func (s *OrderService) quoteSnapshotForTenant(ctx context.Context, tenantID uint64, quoteID string) (repocommercial.QuoteRow, quoteSnapshot, error) {
  	return s.quoteForTenant(ctx, tenantID, quoteID)
  }
+ 
+ // OrderByQuote / CurrentPurchaseOrder 包装 OrderStore 的两个购买查询,
+ // 让 PurchaseService 不再穿透 s.orders 字段直接访问仓库层。


─── internal/modules/commercial/service/commercial/purchase.go:217-225 ───
[bug · high] 步骤 8 的幂等维度是 quoteID,而门控对象是购买身份(ExternalPurchaseSubscriptionID),两者不一致导致跨报价重复开单:同一租户先以
quote1 购买(order1 pending、渠道 checkout URL 有效),随后重新报价 quote2(同 plan 同价,CreateQuote 无任何限制)再 POST——步骤 6
命中 command key 幂等重放、步骤 7 匹配门通过、GetOrderByQuote(quote2) 未命中,p.State 仍为 awaiting,于是 CreateOrder
再开一张独立渠道订单 order2。两张 pending 订单各自持有有效支付链接且互不知晓:ConfirmPayment 仅按订单/尝试去重(版本守卫是单订单
pending→paid),用户分别付款即双倍扣款+双倍履约;且第一笔付款激活订阅后,order2 依然可付,恰好绕过了 ErrPurchaseNotAwaiting 想防的"active plan
被二次收费"(该守卫只拦新开单,不拦已存在的旧单)。建议在 CreateOrder 前对"该租户当前购买已存在的 pending 订单"做重放(如复用 CurrentPurchaseOrder 并限定
pending),把开单条件收紧为"无未决购买订单"。

  	if existing, err := s.orders.orders.GetOrderByQuote(ctx, tenantID, quoteID); err == nil {
  		ov := orderViewFromRow(existing)
  		return s.purchaseView(p, snap, pub, &ov), nil
  	} else if !errors.Is(err, repocommercial.ErrOrderNotFound) {
  		return PurchaseView{}, err
  	}
  	if p.State != domain.PurchaseStateAwaitingPayment {
  		return PurchaseView{}, fmt.Errorf("%w: %s", ErrPurchaseNotAwaiting, p.State)
+ 	}
+ 	// 同一购买身份下已存在未决订单时重放,不再开第二张渠道单(防双倍扣款)。
+ 	if held, err := s.orders.orders.CurrentPurchaseOrder(ctx, tenantID, snap.PriceFen, domain.CurrencyCNY); err == nil &&
+ 		held.State == domain.OrderStatePending {
+ 		ov := orderViewFromRow(held)
+ 		return s.purchaseView(p, snap, pub, &ov), nil
  	}


─── internal/modules/commercial/service/commercial/purchase.go:194-200 ───
[bug · medium] 步骤 6 成功后整条链路无原子性兜底:快照读失败、CreateOrder 的 DB 错误、或报价在步骤 2 预检与 consumeQuoteTx
之间过期,都会在权威侧留下无本地订单的 awaiting 订阅。同价重报价可自愈,但若期间计划被重发布改价,后续所有重试都命中 ErrInvoiceQuoteMismatch
且代码不触发任何取消——lago_purchase.go 明确以 timeout_hours:0 把取消时序交给协调者("cancellation timing stays with the
coordinator"),而 seam 只定义了 create 命令、purchase.go 也没有任何取消/回收动作,该孤儿 awaiting
订阅将永久阻塞该租户的购买(只能靠权威侧人工清理)。建议:要么在 mismatch 且权威侧为 awaiting 时补一个 cancel_purchase seam
命令清理孤儿,要么至少在注释/运维文档中明确该状态的恢复路径,避免"购买不可用"成为无解终态。



─── internal/modules/commercial/service/commercial/purchase.go:354-357 ───
[bug · medium] orderViewFromRow 只映射 ID/QuoteID/State/AmountFen/Currency/Version,丢弃了 OrderView 的
Provider/CheckoutURL:首次 POST /purchases 的响应带 checkout_url(前端凭它跳转支付,purchase_test 也仅对首建断言非空),而重放路径(同
quote 二次 POST、ErrQuoteAlreadyUsed 竞态兜底、GET PurchaseStatus)返回的同一订单缺 checkout_url 与 provider——且
OrderRow/PaymentAttemptRow 均不持久化 checkout
信息,视图永远无法重建。同一订单在不同路径上的投影不一致,客户端在重放场景拿到的是"无法继续支付"的降级视图。建议至少从 FirstPendingAttempt 回填
Provider,并明确重放/状态视图的支付恢复契约(走 GET /orders/:id 恢复),或考虑持久化渠道恢复所需字段。

  func orderViewFromRow(r repocommercial.OrderRow) OrderView {
  	return OrderView{ID: r.ID, QuoteID: r.QuoteID, State: r.State,
  		AmountFen: r.AmountFen, Currency: r.Currency, Version: r.Version}
+ 	// TODO(#81): 重放/状态路径的支付恢复契约待定——至少应从
+ 	// FirstPendingAttempt 回填 Provider,并在文档中说明 checkout_url
+ 	// 仅存在于首次创建响应,重放后经 GET /orders/:id 恢复。
  }


─── internal/modules/commercial/service/commercial/purchase.go:180-184 ───
[bug · medium] 消歧条件要求 State==awaiting 才判 PurchasePlanConflict,但适配层对"已持有不同 plan code 的订阅"统一返回
ErrPlatformInvalidResponse——当租户已有 ACTIVE 购买(如已购 plan B)再提交 plan A 的报价时,SubmitCommand 失败、消歧读到
State=active 不满足条件,最终落入 ErrPurchaseUnavailable:一个确定性的计划冲突被误答为
503"暂时不可用"(reason=invalid_response),客户端按瞬时故障无限重试,而重试与重新报价都永远无法成功。建议把冲突判定放宽到非 absent 状态(缺席态仍走
unavailable,因为那意味着 create 因其他原因失败)。

  			}); serr == nil && held.Purchase != nil &&
- 				held.Purchase.State == domain.PurchaseStateAwaitingPayment &&
+ 				held.Purchase.State != domain.PurchaseStateAbsent &&
  				held.Purchase.PlanCode != pub.PlanCode {
  				return PurchaseView{}, ErrPurchasePlanConflict
  			}


─── internal/modules/commercial/service/commercial/purchase.go:264-267 ───
[maintainability · low] FindPublicationByCode 的错误被整体静默吞掉:除 ErrPublicationNotFound 之外的瞬时 DB
错误也会让视图静默缺失 plan_key/plan_version,而 GET 响应仍是
200+state=active——客户端无法区分"权威侧无发布映射"与"读取故障",可能把缺失字段误读为数据不存在。若确为 best-effort 设计,建议至少区分
not-found(合法缺失)与其他错误(日志记录或经 pendingView 姿态透出),并在注释中写明该取舍。

  	if pub, err := s.plans.FindPublicationByCode(ctx, p.PlanCode); err == nil {
  		out.PlanKey = pub.PlanKey
  		out.PlanVersion = pub.Version
+ 	} else if !errors.Is(err, repocommercial.ErrPublicationNotFound) {
+ 		// 瞬时读取故障:保持 state 视图,但记录/透出,不静默丢字段。
+ 		return s.pendingView(err), nil
  	}


─── internal/modules/commercial/service/commercial/purchase.go:275-280 ───
[bug · low] PurchaseStatus 对 CurrentPurchaseOrder 的错误整体静默吞掉:与上一行 FindPublicationByCode 的吞错(已确认发现
7)同类但后果更重——Order 是该视图的支付面(前端轮询依赖 order.state/支付入口),瞬时 DB 故障会让一个 state=awaiting_payment/active 的 200
响应静默缺失 order,客户端无法区分"确实无订单"与"读取故障"。建议至少区分 ErrOrderNotFound(合法缺失)与其他错误(记录日志或经 Reason 姿态透出),并在注释中写明该
best-effort 取舍。



─── packages/api-client/src/commercial.ts:66-68 ───
[maintainability · low] purchase 的输入类型以内联对象字面量定义，重复了 CreateOrderInput 中已有的 'wechat' | 'alipay'
provider 联合类型，也违背了 contracts 包的既有约定（QuoteInput/CreateOrderInput/RefundInput 均为导出的命名类型并经 index.ts
转出）。后续新增渠道 provider 时需要同步修改多处。建议在 contracts 中定义并导出 PurchaseInput（如 export interface PurchaseInput {
quote_id: string; provider: 'wechat' | 'alipay' }），此处与 index.ts 复用。

-     async purchase(input: { quote_id: string; provider: 'wechat' | 'alipay' }, signal?: AbortSignal): Promise<PurchaseView> {
+ // packages/contracts/src/commercial.ts:
+ // export interface PurchaseInput { quote_id: string; provider: 'wechat' | 'alipay'; }
+ async purchase(input: PurchaseInput, signal?: AbortSignal): Promise<PurchaseView> {
-       return parsePurchaseView(unwrap(await request({ method: 'POST', path: '/api/v1/commercial/purchases', body: input, signal })));
+   return parsePurchaseView(unwrap(await request({ method: 'POST', path: '/api/v1/commercial/purchases', body: input, signal })));
-     },
+ },


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:104-108 ───
[bug · high] 恒假条件（不可达死代码）：`path` 仅在 `archived.exists()` 为真时才被赋值为 `str(archived)`，因此
`archived.exists() and path != str(archived)` 永远为 False，WARNING + `sys.exit(2)`
分支永远不会执行。后果：注释承诺的「runs/ 回退 TSV 在用时降级为 MISSING-EVIDENCE，避免对不可按 run 键控的整库 payments 聚合做 exactly-once
断言」从未生效——当归档 TSV 缺失（如归档被移走/单独检出）而 runs/ 下留有其他 replay 的 db-watch-verify-*.tsv 时，脚本会用异 run 的 payments
聚合执行 `max_succeeded() == 1` 断言，可能产生误判 CHECK 或假 PASS，使 #74 的 DB 侧验收结论失真。修复时注意：test_verify_db_watch.py
中
`test_missing_sub_a_samples_is_check_not_vacuous_pass`、`test_good_tsv_still_passes_and_prints_the_se
lected_file`、`test_foreign_run_rows_do_not_leak_into_assertions` 三个用例都以「无归档 + 仅 runs/ TSV」布局却断言 exit
1/0，当前锁定的正是这个缺陷行为，需同步调和；ocr1/ocr2/ocr3-replay 目录下的同名脚本复制了同一恒假条件，也应一并修复。

- if archived.exists() and path != str(archived):
+ if path != str(archived):
      print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
            "whole-DB aggregate and cannot be keyed to this run, so the "
            "exactly-once max_succeeded assertion is not decidable")
      sys.exit(2)


─── deploy/lago-lab/payment-activation/clients.py:400-404 ───
[maintainability · low] `last_error` 被赋值但在整个方法中从未被读取——最终失败通过裸 `raise`
重抛当前异常，该变量属于死代码，会误导读者以为它参与了最终错误上报。要么直接删除这两处赋值，要么真正利用它（例如在最终重抛时包装为带上下文的 LabError：`raise
LabError("stripe transport failed after retries") from last_error`）。

-             except (URLError, OSError) as error:
-                 last_error = error
+             except (URLError, OSError):
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/phases.py:599-602 ───
[maintainability · low] 此 FAIL 文案存在归因偏差：进入该分支时订阅既不是 incomplete（`still_incomplete=False`）也未被确认
canceled(payment_failed)，最严重的情形是订阅被违规激活为 active——这正是 AC1 门控失效的核心违约——但文案统一表述为"取消契约未成立
(canceled=False)"，且证据只记录了 incomplete/canceled 两次探测的结果（recheck_subscription_status 为
None），报告读者无法区分"激活违约"与"其他状态漂移"。建议在此分支补一次 active 状态探测，并按实际终态区分错误文案。

+                 as_, asub = _subscription_status_any(ctx, ext, ("active",))
+                 if _ok(as_):
+                     observed["error"] = (
+                         "invoice invisible and the gated subscription ACTIVATED "
+                         f"(status={asub.get('status')!r}) — AC1 gating violated"
+                     )
+                 else:
-                 observed["error"] = (
+                     observed["error"] = (
-                     "invoice invisible and the cancellation contract did not "
+                         "invoice invisible and the cancellation contract did not "
-                     f"hold (canceled={canceled}, cancellation_reason={reason!r})"
+                         f"hold (canceled={canceled}, cancellation_reason={reason!r})"
-                 )
+                     )


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:108-112 ───
[bug · high] 该守卫是不可达的死代码，承诺的安全降级机制完全失效：选路逻辑中 `archived.exists()` 为 True 时 `path` 必然等于
`str(archived)`（第二合取项为 False）；为 False 时（走 `elif matches` 回退分支）第一合取项即为 False。因此 `archived.exists()
and path != str(archived)` 在任何路径下恒为 False，`sys.exit(2)` 分支永不执行。后果：当本目录归档 TSV 缺失而仓库级 runs/
下残留其它回放（ocr-1/ocr-3/evidence-74）的 `db-watch-verify-*.tsv` 时（runs/ 为 git-ignored
运行时产物、每次回放生成新时间戳文件，此错配场景在 OCR round 2 中真实发生过），脚本会静默采用外来 run 的 TSV 继续执行 `max_succeeded() == 1`
整库聚合断言——payments 聚合无法用 run_prefix 隔离，外来计数直接注入 exactly-once 断言，可能输出与本次运行无关的误导性 CHECK/PASS，而 docstring
与 R1-V10 注释声明的 exit-code 契约（2 = MISSING-EVIDENCE）永远不会兑现。修复应将条件改为 `path != str(archived)`（等价于 `not
archived.exists()`），并注意同步核对回归测试中"无归档 + runs/ 有 TSV"场景的期望（目前测试断言该场景继续执行断言而非 exit 2，与守卫注释的契约不一致）

- if archived.exists() and path != str(archived):
+ if path != str(archived):
      print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
            "whole-DB aggregate and cannot be keyed to this run, so the "
            "exactly-once max_succeeded assertion is not decidable")
      sys.exit(2)


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:12-13 ───
[documentation · low] docstring 行号自述与实际不符：`# ocr-2:` 注释实际位于第 76 行（deferred re-check 处）和第 86
行（not_applicable 探针处），自述为 74/84。另外相对 ocr-1 副本（86 行）的 13 行差额并非"仅新增两条注释"，还包含配套的 `deferred_recheck` /
`pending_gate_retry` 两组断言代码（+11 行）及 docstring 本身的扩展（+2 行），"adds two ocr-2-specific assertion
annotations" 的描述低估了副本间差异。该自述是 R1-V23 更正链的组成部分（其自身就是对上一处错误陈述的更正），行号与差异描述失准会误导后续回放核对与审计定位。建议按实际行号
76/86 更正，并如实描述新增内容

- two ocr-2-specific assertion annotations: "# ocr-2:" at lines 74 and 84
+ two ocr-2-specific assertion annotations: "# ocr-2:" at lines 76 and 86,
+ each paired with a new deferred-recheck / not_applicable assertion
  (R1-V23: the previous note here wrongly claimed "unmodified").


LLM retry report summary: 2 of 73 requests affected -- 1 request failed, 1 request recovered after retry

Core review (2 requests):
- deploy/lago-lab/payment-activation/clients.py,deploy/lago-lab/payment-activation/fixtures.py,deploy/lago-lab/payment-activation/phases.py,deploy/lago-lab/payment-activation/run_lab.py: timed out -> failed
- docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr2-replay/verify_db_watch.py: network error -> succeeded

Per-attempt detail: --format json (retry_report).
