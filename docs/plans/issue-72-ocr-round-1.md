Review complete: 38 finding(s) across 102 selected item(s).

─── apps/web/src/commercial/CheckoutPage.tsx:179-179 ───
[bug · low] 「前往支付」链接仅校验 checkout_url 存在性与 scheme 白名单，未校验订单支付状态。getOrder 轮询路径对已支付订单确实不再返回
checkout_url（service/commercial/order.go GetOrder 对非 pending 订单的投影省略该字段），但 POST purchase
的回放路径（purchase.go orderViewFromRow）对已支付订单仍无条件透传 checkout_url（R1-V03 注释明确「a POST replay after payment
still answers the paid order verbatim」）。用户支付完成后经回放路径进入此页面时，会在 payment='paid' 的订单上看到「前往支付」入口，与紧邻待付款
Status 的 R1-V12「已支付/终态不再误导重复支付」意图不一致。建议与 Status 一致地补上 payment === 'pending' 条件（渠道侧虽有防重，但 UI
不应展示误导性支付入口）。

-               {state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url) ? (
+               {state.order.payment === 'pending' && state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url) ? (


─── apps/web/src/commercial/CheckoutPage.tsx:92-93 ───
[maintainability · low] 本次变更移除了 idempotencyKeyRef，重试语义改为「同一 quote 重试即同一次 purchase 调用（后端按 quote_id
唯一索引 + 身份幂等回放既有订单）」，但下方错误分支（第 165
行）的重试按钮文案仍为「重试（复用原订单与幂等键，不重复下单）」——「幂等键」这一机制已不存在，文案与实现不符，会误导用户与后续维护者。建议同步更新文案，例如「重试（复用同一报价，不重复下单）」。



─── apps/web/src/commercial/CheckoutPage.tsx:94-95 ───
[maintainability · low] provider: 'wechat' 为内联业务硬编码。契约类型为 'wechat' |
'alipay'，后端两个渠道均已接线（payment/wechat.go、payment/alipay.go 均产出 CheckoutURL），但前端硬编码导致支付宝渠道在 UI
不可达，后续接入时还需回改此处。若属第一切片（#81 仅微信）的产品范围决策，建议按本文件既有 DEFAULT_PLAN_KEY 等默认值模式提取为模块级常量（如 const
DEFAULT_PAYMENT_PROVIDER = 'wechat'）并注明切片限制，消除散落的魔法字符串。

          const purchase = await client.commercial.purchase(
-           { quote_id: quoteRef.current.id, provider: 'wechat' },
+           { quote_id: quoteRef.current.id, provider: DEFAULT_PAYMENT_PROVIDER }, // 第一切片仅微信


─── deploy/lago-lab/payment-activation/clients.py:390-404 ───
[maintainability · low] 重试循环中的 `last_error` 是死存储：它只在 except 分支被赋值，从未被读取——最终失败路径是裸
`raise`（重新抛出当前捕获的异常），并不依赖 `last_error`。徒增阅读负担并暗示存在实际上不存在的错误聚合逻辑。直接删除即可（裸 `raise` 不需要 `as error` 绑定）。

-         last_error = None
          for attempt in range(transport_retries + 1):
              try:
                  with self._opener.open(request, timeout=self._timeout) as response:
                      status = response.status
                      raw = response.read()
                  break
              except HTTPError as error:
                  raw = error.read()
                  return error.code, _parse_json_body(raw)
-             except (URLError, OSError) as error:
-                 last_error = error
+             except (URLError, OSError):
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/phases.py:640-650 ───
[test · medium] 发票可见且为 failed 的分支（PASS 判定）缺少 AC1 的 exactly-once 支付数校验，与另外两条平行分支严格度不一致：发票不可见分支（第 604
行）要求 `len(non_succeeded) <= 1` 才允许 canceled(payment_failed) 判 PASS，open/pending 分支也将
`len(non_succeeded) <= 1` 纳入 pre_charge_ok（第 667 行），唯独本分支只检查 `core_ok and canceled and reason ==
"payment_failed"` 即返回 PASS。且第 571-574 行注释明确声称 payments 校验 "asserted on this path too — not only when
the gating invoice is visible"，与本分支实际实现相矛盾。由于本次变更把 customer A 也换成
pm_card_authenticationRequired，本分支正是 gate 阶段最可能命中的终局路径——若失败路径出现重复支付行（回归），实验将给出误导性的 PASS，且 observed
中也不记录 payments_non_succeeded_count，证据缺失。建议本分支同样调用 _payments_for 并将 `len(non_succeeded) <= 1` 纳入 PASS
条件（该方法处于外层 try 内，OSError 会同样映射为 blocked，行为与下方 654-657 行一致）。

+             ps, payments = _payments_for(ctx, customer["external_id"])
+             non_succeeded = [p for p in payments if p.get("status") != "succeeded"]
+             observed["payments_non_succeeded_count"] = len(non_succeeded)
              ctx.state["gate"] = {
                  "subscription_external_id": ext,
                  "invoice_lago_id": observed["invoice_lago_id"],
                  "customer_external_id": customer["external_id"],
              }
-             if core_ok and canceled and reason == "payment_failed":
+             if core_ok and canceled and reason == "payment_failed" \
+                     and len(non_succeeded) <= 1:
                  ctx.note(
                      "3DS off-session charge failed: invoice visible as failed, "
                      "subscription canceled(payment_failed), entitlements stayed 404"
                  )
                  return _report(ctx, "gate", expected, observed, PASS)


─── deploy/lago-lab/payment-activation/phases.py:701-710 ───
[bug · low] phase_gate 的 pre-charge 回退 PASS 分支(发票可见为 open/pending、稳定性窗口内订阅转为 canceled)只检查 `canceled`
即判 PASS,不校验 cancellation_reason——而本次改写后的 expected 文本明确承诺 "or ends canceled(payment_failed) when the
charge fails",且两条平行分支均已收紧为 `reason == "payment_failed"`(发票不可见分支第 604 行还叠加了 non_succeeded<=1;failed
分支第 645 行同样要求 reason)。若订阅因其他原因(或无 reason)被取消(例如重复注册延迟终止、人工取消),本分支仍会 PASS,产生与 expected
承诺不符的证据。建议与平行分支对齐:计算 reason 后要求 `canceled and reason == "payment_failed"`,否则 FAIL 并附上实际 reason。

-             if canceled:
-                 reason = (sb or {}).get("subscription", {}).get("cancellation_reason")
+             canceled = _ok(ss)
+             reason = ((sb or {}).get("subscription") or {}).get("cancellation_reason") \
+                 if canceled else None
+             if canceled and reason == "payment_failed":
                  ctx.note(
                      "3DS window did not hold: subscription became canceled "
                      f"(cancellation_reason={reason}); AC1 evidence is the pre-charge "
                      "observation window between creation and the first charge attempt"
                  )
                  status = PASS
              else:
-                 observed["error"] = "pre-charge window observed but state drifted unexpectedly"
+                 observed["error"] = (
+                     "pre-charge window observed but the cancellation contract did "
+                     f"not hold (canceled={canceled}, cancellation_reason={reason!r})"
+                 )


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:104-108 ───
[bug · medium] 该降级守卫恒为 False,属不可达死代码:开头 `if archived.exists(): path = str(archived)` 已保证
`archived.exists()` 为真时必有 `path == str(archived)`,而 `archived.exists()`
为假时第一个合取项即为假。因此注释(R1-V10)承诺的"runs/ 回退 TSV 无法归属本 run 时降级为 MISSING-EVIDENCE(exit 2)"永不生效——回退路径会继续用外来/多
run 的全库支付聚合计算 max_succeeded,并可能给出误导性的 CHECK(exit
1),让操作者去追查不存在的业务状态回归,违背脚本自身文档声明的退出码语义(2=证据缺失需重放,1=断言失败需排查)。建议将条件改为 `not archived.exists()`(注意同目录离线测试
test_verify_db_watch.py 中 runs/-回退用例目前预期 exit 1,需一并更新),或删除该死分支及其注释。另:ocr1/ocr3-replay
下的同族脚本复制了同一守卫,存在相同问题。

- if archived.exists() and path != str(archived):
+ if not archived.exists():
      print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
            "whole-DB aggregate and cannot be keyed to this run, so the "
            "exactly-once max_succeeded assertion is not decidable")
      sys.exit(2)


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:111-112 ───
[bug · medium] 该降级分支不可达（死代码），且与声明的意图相反：上方路径选择块中 `if archived.exists(): path = str(archived)`，因此
`archived.exists()` 为真时必然 `path == str(archived)`，整个条件 `archived.exists() and path != str(archived)`
恒为 False——WARNING 打印与 `sys.exit(2)` 永远不会执行。而紧邻的 R1-V10 注释和 docstring 明确声明：runs/ 回退时 payments 聚合无法按
run 隔离、max_succeeded 不可判定，应响亮降级为 MISSING-EVIDENCE（退出码 2），正确条件应为 `not archived.exists()`。影响：当本目录归档
TSV 缺失而 runs/ 残留其它 replay 的 db-watch-verify-*.tsv 时，脚本会静默采用外来 TSV 继续断言，把证据错配误报为普通 CHECK（退出码
1，形似业务状态回归），误导复核与自动化判定。注意：flow-evidence-74 / ocr2-replay / ocr3-replay 三个副本中该条件完全相同，需一并修复。

- if archived.exists() and path != str(archived):
+ if not archived.exists():
      print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "


─── internal/handler/commercial.go:486-491 ───
[bug · medium] Purchase 的 default 分支会将基础设施错误以 400 + 原始 err.Error() 透出：可达该分支的错误包括
EnsureBillingAccount / GetPublication / GetVersion / CreateOrder（OpenOrder、LatestVersion）的 gorm
与存储层错误，以及快照 JSON 解析错误（quoteForTenant 的 "invalid_quote_row: %v"）。这既把服务端故障误标为客户端错误（调用方按 4xx
语义重试同一请求），又向公网边界暴露内部错误细节（SQL/驱动错误文本）。虽然与既有 CreateOrder 的 default 写法一致，但 Purchase
协作面更宽、落入该分支的基础设施错误类别更多，建议新链路对未知错误返回 500 + 固定文案（原始错误记录在服务端日志）。

- 	case errors.Is(err, repocommercial.ErrQuoteAlreadyUsed):
- 		// (R1-V21) Same race outcome as POST /orders: 409, not a generic 400.
- 		c.JSON(http.StatusConflict, gin.H{"error": "quote already used"})
  	default:
- 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
+ 		// 未知/基础设施错误不透出内部细节，也不误导客户端按 400 重试语义处理。
+ 		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})
  	}


─── internal/handler/commercial.go:511-516 ───
[maintainability · low] 该分支为不可达死代码：ErrQuoteTenantMismatch 只在 quoteForTenant（quote 读取路径）中产生，而
PurchaseStatus 服务层不读 quote——它只可能返回 nil 错误或 tenantID==0 的 ErrInvalidQuoteRow（经 commercialTenantScope
后亦不可达）。default 分支同理几乎不可达，且与 Purchase 一样存在 err.Error() 直接透出的问题。建议删除该哨兵分支，default 返回固定文案，避免误导后续维护者以为存在
quote 越权读取路径。

- 	case errors.Is(err, commercialsvc.ErrQuoteTenantMismatch):
- 		c.JSON(http.StatusNotFound, gin.H{"error": "purchase not found for this tenant"})
  	default:
- 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
+ 		c.JSON(http.StatusBadRequest, gin.H{"error": "purchase status unavailable"})
  	}
  }


─── internal/modules/commercial/commercialplatform/fake.go:652-656 ───
[test · low] 新的 create_purchase_subscription 分支未遵循 fake 中既有的 failSubmits
故障注入约定：ensure_customer、ensure_subscription、grant_included_credits 在 f.failSubmits != nil
时都会先应用权威状态（persisted-but-response-lost 建模）再返回注入错误，而本分支直接忽略该旋钮。结果是 #81
购买路径的「已持久化但响应丢失→按身份重放收敛」恢复场景（#73/#80 的既有测试模式）无法用现有旋钮在 fake 上演练，outbox/恢复测试对该命令族存在盲区。建议参照
ensure_subscription 的写法：把 providerBindings/purchaseSubs 的写入收进 apply 闭包，在 f.failSubmits != nil 时先
apply() 再返回 f.failSubmits。

  		f.mu.Lock()
  		defer f.mu.Unlock()
- 		// ensureProviderBinding semantics: the create command guarantees the
- 		// customer carries a provider binding (D3) — recorded by the fake at
- 		// first create, never duplicated.
+ 		apply := func() {
+ 			// ... 现有 providerBindings / purchaseSubs 写入逻辑移入此处 ...
+ 		}
+ 		if f.failSubmits != nil {
+ 			// Persisted-but-response-lost：权威状态已应用，调用方观察到注入失败。
+ 			apply()
+ 			return commercial.CommandReceipt{}, f.failSubmits
+ 		}
+ 		apply()


─── internal/modules/commercial/commercialplatform/lago_purchase.go:39-42 ───
[bug · medium] 超时预算不自洽，且失败后重试永久丢失 pmSync 等待窗口。purchaseRequestTimeout = 15s+10s =
25s，但生产首次绑定路径（StripeAPIKey 与 StripePmToken 均配置）实际包含：customerProviderBound（共享 client ≤5s）+ Stripe
create/attach/default（最多 3 次，各自 ≤15s，t09 实测约 3.5s/次）+ pmSyncWait 20s + identity read + create
POST。pmSync 轮询的实际可用窗口远小于 pmSyncWait=20s；外层 deadline 一旦在轮询中耗尽，a.do 会把 ctx 超时包装为
ErrPlatformUnreachable。更关键的是：该次失败后重试走 ensureProviderBinding 的 bound 短路（客户已绑定）直接返回，不再执行
syncPaymentMethods，create 直接撞 422 no_default_payment_method，被 createPurchaseSubscription 的 422 分支映射为
terminal ErrPlatformInvalidResponse("purchase replay cannot be verified")——本应由 20s
轮询吸收的瞬态导入等待被永久丢失并升级为终态错误（fail-closed 可恢复，但产生错误的错误分类与一段时间的购买失败）。建议把 provider 出站预算与 pmSyncWait
显式纳入总预算（pmSyncWait 目前是 var 以便测试调短，purchaseRequestTimeout 需同步改为 var），或让 bound 短路路径在导入尚未确认时仍执行一次有界等待。

  // purchaseRequestTimeout bounds one create_purchase_subscription command:
- // the binding read/write, the possible provider-customer round trip, the
- // identity read and the gated create.
- const purchaseRequestTimeout = subscriptionRequestTimeout + 10*time.Second
+ // binding read/write (≤ subscriptionRequestTimeout) + up to three provider
+ // round trips (outboundProviderTimeout each) + the pm-sync poll (pmSyncWait)
+ // + the identity read and the gated create. A var (not const) so tests that
+ // shrink pmSyncWait keep the total budget self-consistent.
+ var purchaseRequestTimeout = subscriptionRequestTimeout + 3*outboundProviderTimeout + pmSyncWait


─── internal/modules/commercial/commercialplatform/lago_purchase.go:365-369 ───
[bug · low] 与下方 sleepCtx 分支的 R1-24 注释声明的不变量不一致：轮询中的 a.do 传输错误（包含 caller ctx 取消/deadline 到期——例如外层
purchaseRequestTimeout 在某次 GET 进行中而非 sleep tick 中到期）统一携带 ErrPlatformUnreachable sentinel，而注释明确要求
caller 取消不得携带该 sentinel（否则重试协调器会重新进入刚被预算取消的轮询）。建议在返回前检查 ctx.Err()，取消时按 sleepCtx 分支相同方式返回裸 ctx 错误链。

  		status, body, err := a.do(ctx, http.MethodGet,
  			"/api/v1/customers/"+url.PathEscape(externalCustomerID)+"/payment_methods", nil)
  		if err != nil {
+ 			if ctxErr := ctx.Err(); ctxErr != nil {
+ 				return fmt.Errorf("payment method sync interrupted: %w", ctxErr)
+ 			}
  			return err
  		}


─── internal/modules/commercial/payment/alipay.go:200-208 ───
[security · medium] (R1-V09) 该客户端只做了"请求前校验"，缺少拨号期(dial-time) IP 钉扎，存在经典 DNS rebinding TOCTOU
窗口：ValidateURLForSSRF→isSSRFSafeURL 会做一次 net.LookupIP 校验解析结果，但默认 Transport
在真正建连时会再次独立解析域名；攻击者控制的域名可在校验时返回公网 IP、拨号时返回环回/私网/元数据地址，直接绕过本改动要满足的 SSRF
验收条件。项目已有专门设施：secutils.NewSSRFSafeHTTPClient（= SSRFSafeDialContext 解析一次并只拨已校验 IP +
SSRFValidatingRoundTripper 每请求校验 + newSSRFCheckRedirect 限跳数/跨域剥离凭证头），chat
transport、web_search、opensearch、qqbot/wecom、mysql_ssrf 均已采用，ssrf_outbound_cache.go 的注释也明确 rebinding
由拨号层兜底。另外自定义 CheckRedirect 会取代标准库默认的 10 跳上限，当前实现没有 len(via) 限制，恶意网关可用无限重定向链耗尽请求直到
Timeout。SSRF_WHITELIST 豁免的 loopback 实验桩在共享设施下同样放行，不受影响。

- 		client: &http.Client{
- 			Timeout: cfg.timeout(),
- 			CheckRedirect: func(req *http.Request, via []*http.Request) error {
- 				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
- 					return fmt.Errorf("alipay redirect target failed SSRF validation: %w", err)
- 				}
- 				return nil
- 			},
- 		},
+ 		client: secutils.NewSSRFSafeHTTPClient(secutils.SSRFSafeHTTPClientConfig{
+ 			Timeout:      cfg.timeout(),
+ 			MaxRedirects: 10,
+ 		}),


─── internal/modules/commercial/payment/alipay.go:564-566 ───
[performance · low] 每次网关调用（下单/查单/退款/对账轮询）都执行无缓存的 ValidateURLForSSRF，其内部 isSSRFSafeURL 每次做
net.LookupIP，等于在每个支付操作前串行增加一次 DNS 解析（重定向每跳同理）；且瞬时 DNS 故障会被归类为 ErrNotConfigured，把基础设施抖动误报为渠道未配置。utils
包为高频出站路径专门提供了 60s origin 级缓存的 validateURLForSSRFForOutbound（经 NewSSRFSafeHTTPClient 的
SSRFValidatingRoundTripper 暴露）。若按上一条建议改用共享安全客户端，此处的逐请求预检可移到构造期（或直接删除，由校验型 Transport 每请求兜底）。

- 	if err := secutils.ValidateURLForSSRF(p.cfg.gateway()); err != nil {
- 		return fmt.Errorf("%w: gateway url failed SSRF validation: %v", ErrNotConfigured, err)
- 	}
+ 	// 改用 NewSSRFSafeHTTPClient 后，SSRFValidatingRoundTripper 会对每个请求
+ 	// 走缓存路径(validateURLForSSRFForOutbound)校验；此处预检可移至
+ 	// NewAlipayProvider 构造期执行一次，避免每请求一次未缓存 DNS 解析。


─── internal/modules/commercial/payment/alipay.go:202-207 ───
[bug · low] 自定义 CheckRedirect 会整体替换 net/http 默认的"最多跟随 10
次连续重定向"策略，而本实现只校验跳转目标、从不限制跳数，等于把重定向上限放开为无限制。被劫持或恶意的网关可用两个公网主机互相 302 构成重定向环，每次支付/查单/退款调用都会持续跳转直到
client Timeout（默认 10s）耗尽，且每一跳还在 ValidateURLForSSRF 里同步执行一次无缓存的 net.LookupIP（见前述已确认问题），既占用请求
goroutine 又持续打 DNS。项目自有的共享策略 internal/utils.newSSRFCheckRedirect（NewSSRFSafeHTTPClient
使用，MaxRedirects=10）就是先 `if len(via) >= maxRedirects` 再校验目标；建议在此补上跳数上限，或直接采用
secutils.NewSSRFSafeHTTPClient（同时获得跳数限制、拨号期 IP 钉扎与 60s 缓存校验）。此问题独立于已确认的 DNS rebinding TOCTOU 发现。

  			CheckRedirect: func(req *http.Request, via []*http.Request) error {
+ 				if len(via) >= 10 {
+ 					return fmt.Errorf("alipay stopped after 10 redirects")
+ 				}
  				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
  					return fmt.Errorf("alipay redirect target failed SSRF validation: %w", err)
  				}
  				return nil
  			},


─── internal/modules/commercial/payment/wechat.go:188-196 ───
[security · medium] (R1-V09) 与 alipay.go 同一问题：仅请求前校验、无拨号期 IP 钉扎，存在 DNS rebinding TOCTOU
窗口——isSSRFSafeURL 校验时解析一次 DNS，默认 Transport 建连时再独立解析一次，重绑定应答可让实际连接落在环回/私网/元数据地址，绕过 SSRF 校验。项目已有
secutils.NewSSRFSafeHTTPClient（SSRFSafeDialContext 解析一次只拨已校验 IP + SSRFValidatingRoundTripper +
限跳数/剥凭证头的重定向策略），chat/web_search/opensearch/qqbot/wecom/mysql 均已采用。另外自定义 CheckRedirect 取代了标准库默认 10
跳上限而未补 len(via) 检查，恶意端点可用无限重定向链循环到 Timeout；WeChat APIv3 每跳还会重复走未缓存的 DNS 校验。SSRF_WHITELIST 豁免的
loopback 桩在共享设施下仍放行，不影响 flow-evidence 实验。

- 		client: &http.Client{
- 			Timeout: cfg.timeout(),
- 			CheckRedirect: func(req *http.Request, via []*http.Request) error {
- 				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
- 					return fmt.Errorf("wechat redirect target failed SSRF validation: %w", err)
- 				}
- 				return nil
- 			},
- 		},
+ 		client: secutils.NewSSRFSafeHTTPClient(secutils.SSRFSafeHTTPClientConfig{
+ 			Timeout:      cfg.timeout(),
+ 			MaxRedirects: 10,
+ 		}),


─── internal/modules/commercial/payment/wechat.go:538-540 ───
[performance · low] 每次 APIv3 请求都执行无缓存的 ValidateURLForSSRF（isSSRFSafeURL 内每次
net.LookupIP），给每个支付/查单/退款操作串行增加一次 DNS 解析，且瞬时 DNS 故障被误归类为 ErrNotConfigured。utils 包为高频出站路径提供了 60s
origin 级缓存的 validateURLForSSRFForOutbound（经 NewSSRFSafeHTTPClient 的 SSRFValidatingRoundTripper
暴露）。若改用共享安全客户端，此预检可移到 newWechatProvider 构造期执行一次，或删除由校验型 Transport 每请求兜底。

- 	if err := secutils.ValidateURLForSSRF(p.cfg.apiBase()); err != nil {
- 		return fmt.Errorf("%w: api base url failed SSRF validation: %v", ErrNotConfigured, err)
- 	}
+ 	// 改用 NewSSRFSafeHTTPClient 后，SSRFValidatingRoundTripper 会对每个请求
+ 	// 走缓存路径(validateURLForSSRFForOutbound)校验；此处预检可移至
+ 	// newWechatProvider 构造期执行一次，避免每请求一次未缓存 DNS 解析。


─── internal/modules/commercial/payment/wechat.go:190-195 ───
[bug · low] 与 alipay.go 相同的问题：自定义 CheckRedirect 会整体替换 net/http 默认的 10 次重定向上限，本实现只校验目标、不限制跳数。恶意/被劫持的
API base 用两个公网主机互发 302 即可形成无限重定向环，每次调用持续跳转直到 client Timeout（默认 10s）耗尽，且每跳在 ValidateURLForSSRF
里同步做一次无缓存 DNS 解析（见已确认问题）。internal/utils.newSSRFCheckRedirect 的标准做法是先 `if len(via) >= maxRedirects`
再校验目标；建议补上跳数上限，或直接改用 secutils.NewSSRFSafeHTTPClient（跳数限制 + 拨号期 IP 钉扎 + 60s 缓存一并解决）。此发现独立于已确认的 DNS
rebinding TOCTOU。

  			CheckRedirect: func(req *http.Request, via []*http.Request) error {
+ 				if len(via) >= 10 {
+ 					return fmt.Errorf("wechat stopped after 10 redirects")
+ 				}
  				if err := secutils.ValidateURLForSSRF(req.URL.String()); err != nil {
  					return fmt.Errorf("wechat redirect target failed SSRF validation: %w", err)
  				}
  				return nil
  			},


─── internal/modules/commercial/repository/commercial/order.go:248-256 ───
[bug · medium] (tenant, kind=purchase, amount_fen, currency)
的价格面匹配无法唯一识别"当前购买"：同租户同价面的历史购买单会全部命中（如取消后按同价复购）。两单同时 pending（前次购买的遗留 pending 单未清理 + 复购开新单）时，id ASC 的
tie-break 对 "ord_"+随机 hex 是纯字典序随机，可能把历史遗留单的 checkout_url（属于旧
merchant_order_id，渠道侧可能已失效）当作当前购买的支付入口投影——这正是 R1-35 要修复的"失去支付入口"问题在复购场景的复发；两单皆终态时 rows[0]
取的是最旧一单，投影出陈旧 quote_id。OrderRow 目前没有任何时间戳列可供排序。建议为 commercial_orders 增加 created_at（AutoMigrate
可原位补列）并按 created_at DESC 取最近订单（pending 优先），或将当前购买与 order_id 的关联显式持久化，替代价格面模糊匹配。

- 	for _, row := range rows {
- 		if row.State == domain.OrderStatePending {
- 			return row, nil
- 		}
- 	}
- 	if len(rows) > 0 {
- 		return rows[0], nil
- 	}
- 	return OrderRow{}, ErrOrderNotFound
+ 	// OrderRow 增加 CreatedAt time.Time `gorm:"column:created_at"` 后：
+ 	// Order("created_at DESC, id DESC")，先取最近单，再在最近若干单内优先 pending。


─── internal/modules/commercial/service/commercial/order.go:361-363 ───
[bug · low] SetCheckoutURL 失败时写入 view.CheckoutError 的降级标记只存在于本次应答：checkout_error 并非落库列，重放走
orderViewFromRow、GET 恢复走 row 读取，均无法还原该标记。后果：(1) 同一购买首次应答 202、重放变 201，对同一状态的状态码语义漂移；(2) 注释宣称"the
degraded replay is visible instead of silent"实际未达成——重放与 GET /orders/:id 看到的是既无链接也无错误标记的"干净" pending
单，降级恰恰是静默的；(3) %v 把底层 DB 错误文本经 orderWire 的 checkout_error 字段直接暴露到对外
wire。建议：持久化失败仅记录服务端日志（含原始错误），响应保持与成功路径一致的 201 语义；或把降级标记一并落库使重放/恢复能稳定还原 202 姿态。

  	if err := s.orders.SetCheckoutURL(ctx, id, res.CheckoutURL); err != nil {
- 		view.CheckoutError = fmt.Sprintf("checkout_url persistence failed for %s: %v", id, err)
+ 		// 持久化失败只记日志：CheckoutError 未落库，翻转它只会让首次 202 / 重放 201 语义漂移。
+ 		logger.Errorf(ctx, "checkout_url persistence failed for %s: %v", id, err)
  	}


─── internal/modules/commercial/service/commercial/purchase.go:217-225 ───
[bug · high] 同一 awaiting 购买可开出多张并存的可支付渠道订单。幂等键只有 quote：客户端可随意新切 quote（30 分钟过期后重新报价，或双开标签页各 POST
/quotes 一张），第二张 quote 走到此处时 GetOrderByQuote(Q2) 未命中、购买仍 awaiting，于是 CreateOrder 为同一 gating invoice
开出新的 merchant_order_id 与新 checkout URL；而旧 quote 的 pending 订单 O1 不做任何关闭（provider.Close
无任何生产调用点，订单状态机只有 pending→paid→fulfilled，无 closed/canceled 转移），两张渠道单同时可支付。客户在旧标签页支付 O1、新页面支付 O2
即对同一订阅双重收款，且两单回调会各自 ConfirmPayment→paid→fulfilled。这正是本步注释要防的"must not be charged twice"在 awaiting
期间的等价形态，也再次触发 R1-35 要修的失去/错配支付入口问题。建议：开新单前先解析该购买名下已有的 pending 渠道订单（按价格面或建立 purchase↔order
关联），存在则原样重放该订单（或先 Close 渠道单并落终态），保证任一时刻一个购买至多一张可支付渠道订单。

  	if existing, err := s.orders.orders.GetOrderByQuote(ctx, tenantID, quoteID); err == nil {
  		ov := orderViewFromRow(existing)
  		return s.purchaseView(p, snap, pub, &ov), nil
  	} else if !errors.Is(err, repocommercial.ErrOrderNotFound) {
  		return PurchaseView{}, err
  	}
  	if p.State != domain.PurchaseStateAwaitingPayment {
  		return PurchaseView{}, fmt.Errorf("%w: %s", ErrPurchaseNotAwaiting, p.State)
+ 	}
+ 	// 同一 awaiting 购买已有 pending 渠道订单时不再开第二张：
+ 	// 重放旧单，保证任一时刻至多一张可支付渠道订单。
+ 	if pending, perr := s.orders.orders.CurrentPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); perr == nil &&
+ 		pending.State == domain.OrderStatePending && pending.QuoteID != quoteID {
+ 		ov := orderViewFromRow(pending)
+ 		return s.purchaseView(p, snap, pub, &ov), nil
  	}


─── packages/api-client/src/commercial.ts:66-66 ───
[maintainability · low] purchase 的入参为内联类型 { quote_id; provider }，其中 provider:'wechat'|'alipay' 与
CreateOrderInput.provider 重复定义。同文件的 QuoteInput/CreateOrderInput/RefundInput 均沉淀在 contracts 包并经
index.ts 导出共享，此处打破既有分层约定：后续新增渠道或调整字段时需多处同步修改。建议在 contracts 包新增并导出 PurchaseInput（连同 provider
联合类型），api-client 引用之。

-     async purchase(input: { quote_id: string; provider: 'wechat' | 'alipay' }, signal?: AbortSignal): Promise<PurchaseView> {
+ // contracts: export interface PurchaseInput { quote_id:string; provider:'wechat'|'alipay'; }
+     async purchase(input: PurchaseInput, signal?: AbortSignal): Promise<PurchaseView> {


─── packages/api-client/src/commercial.ts:66-68 ───
[bug · medium] purchase() 丢弃 503 信封顶层的闭合 reason 令牌：后端平台故障（handler Purchase 的 ErrPurchaseUnavailable
分支，internal/handler/commercial.go:476-479）以 {"error":"purchase temporarily
unavailable","reason":"unreachable|unconfigured|..."} 回 503，但 errorFromResult 只提取
message/code，unwrap 只处理 success 信封——reason 到达不了调用方。而服务层 POST 的平台故障从不走 2xx+data（PurchaseView.reason
仅出现在 GET purchaseStatus 的 2xx），因此 CheckoutPage 为这些令牌专门写的闭合失败文案映射（purchaseErrorMessage）在真正的 POST
故障路径永不可达，用户只能看到英文原始 message。建议在此捕获 503 的 ApiError 并保留/转译 reason（需要 errorFromResult 透传顶层 reason，或将其放入
error.details），使 503 与 2xx 共用同一套闭合失败文案。

      async purchase(input: { quote_id: string; provider: 'wechat' | 'alipay' }, signal?: AbortSignal): Promise<PurchaseView> {
+       try {
-       return parsePurchaseView(unwrap(await request({ method: 'POST', path: '/api/v1/commercial/purchases', body: input, signal })));
+         return parsePurchaseView(unwrap(await request({ method: 'POST', path: '/api/v1/commercial/purchases', body: input, signal })));
+       } catch (error) {
+         // R1-V04：503 信封顶层的闭合 reason 令牌随错误透传（errorFromResult
+         // 需携带顶层 reason），页面才能把 503 映射到与 2xx 相同的闭合失败文案。
+         if (error instanceof ApiError && error.status === 503) { /* attach reason */ }
+         throw error;
+       }
      },


─── packages/contracts/src/commercial.ts:40-44 ───
[maintainability · low] checkout_url 的消毒条件把 null 与 undefined/空串一同排除在校验之外，但 undefined 是键不存在而 null 会作为
checkout_url:null 原样透传到返回对象，违反 OrderView 声明的 checkout_url?:string 类型（消费方若直接按 string 调用方法会抛错）。当前后端
orderWire 恒序列化字符串（空串表示无链接），null 不会出现，但 R1-V13 作为纵深防线应自身自洽：null 应与不安全 scheme 一样被剥离（delete 键），而非保留。

-  if(v.checkout_url!==undefined&&v.checkout_url!==null&&v.checkout_url!==''&&
+  if(v.checkout_url===null) {
+   const out={...v}; delete out.checkout_url;
+   return out as unknown as OrderView;
+  }
+  if(v.checkout_url!==undefined&&v.checkout_url!==''&&
      !isSafeCheckoutUrl(String(v.checkout_url))) {
    const out={...v}; delete out.checkout_url;
    return out as unknown as OrderView;
   }


─── packages/contracts/src/commercial.ts:133-135 ───
[maintainability · low] line_items.name 对非字符串值静默替换为空串，而同一对象内 kind（nonEmptyString）与
amount_fen（digitString）均抛错拒绝，校验姿态不一致：畸形服务端响应会被静默吞掉、问题推迟到渲染层才暴露。后端 quoteWire 恒以字符串序列化
name，该兜底分支实际不可达，属于掩盖协议漂移的死代码。建议与相邻字段一致地 fail-fast（空串可放行，非字符串抛错）。

+    if(typeof li.name!=='string') throw new Error('invalid quote (line_items)');
     return { kind: nonEmptyString(li.kind,'kind','quote line item'),
-     name: typeof li.name==='string'?li.name:'',
+     name: li.name,
      amount_fen: digitString(li.amount_fen,'amount_fen','quote line item') };


─── packages/contracts/src/commercial.ts:196-199 ───
[maintainability · low] parsePurchaseView 的空值容错姿态不一致：amount_fen 对 null/空串显式排除后静默跳过，而 currency 仅排除
undefined 与空串——currency 为 null 时会落入 nonEmptyString 抛 'invalid purchase
(currency)'。两个字段对同类畸形输入行为相反，且后端 purchaseWire 对空值是整体省略键而非置 null。建议统一为同一姿态（与 amount_fen 一致地把 null
视同缺省跳过，或一致 fail-fast）。另注：parseQuoteView 的 currency 接受任意非空串，而 OrderView.currency 限定
'CNY'，宽严也不一致，可顺带对齐。

    if(v.amount_fen!==undefined&&v.amount_fen!==null&&v.amount_fen!=='') {
      out.amount_fen = digitString(v.amount_fen,'amount_fen','purchase');
    }
-   if(v.currency!==undefined&&v.currency!=='') out.currency = nonEmptyString(v.currency,'currency','purchase');
+   if(v.currency!==undefined&&v.currency!==null&&v.currency!=='') out.currency = nonEmptyString(v.currency,'currency','purchase');


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:9-9 ───
[maintainability · medium] createRequire 硬编码了开发者本机绝对路径
/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-lago/apps/web/package.json，与脚本头部注释 "Run from
anywhere: node browser_flow_82.mjs" 直接矛盾：任何其他机器、其他 checkout 路径或 CI 上 require('@playwright/test')
都会解析失败。建议改为基于 import.meta.url 的相对解析（本文件位于 docs/plans/issue-72-flow-evidence-82/，相对仓库根上溯 3
级），随目录迁移仍然可移植。

- const require = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-lago/apps/web/package.json');
+ const require = createRequire(new URL('../../../apps/web/package.json', import.meta.url));


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:7-7 ───
[maintainability · medium] 与 browser_flow_82.mjs 相同的问题：createRequire 硬编码开发者本机绝对路径，脚本在其他机器/CI 上无法解析
@playwright/test，损害 evidence 的可复现性。建议统一改为基于 import.meta.url 的相对路径解析。

- const require = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-lago/apps/web/package.json');
+ const require = createRequire(new URL('../../../apps/web/package.json', import.meta.url));


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:26-27 ───
[security · low] 租户 B 的邮箱与密码以字面量硬编码且无环境变量覆盖，与 browser_flow_82.mjs 的 process.env.FLOW82_EMAIL ??
'...' 模式不一致，也不符合本迁移分支"凭据只从环境变量读取、源码与测试不写入可用凭据字面量"的约束精神。虽然已确认这是 README 记录的一次性本地 lab 账号（verify.local 域
+ 一次性 sqlite 库，无真实凭据价值），仍建议与 browser_flow_82.mjs 对齐使用 env 覆盖写法，避免凭据字面量进一步扩散。

-   await page.fill('#auth-email', 'issue82-flow-b@verify.local');
-   await page.fill('#auth-password', 'issue82-Flow-Pw-b');
+   const EMAIL = process.env.FLOW82_EMAIL_B ?? 'issue82-flow-b@verify.local';
+   const PASSWORD = process.env.FLOW82_PASSWORD_B ?? 'issue82-Flow-Pw-b';
+   await page.fill('#auth-email', EMAIL);
+   await page.fill('#auth-password', PASSWORD);


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:8-8 ───
[maintainability · medium] 与 browser_flow_82.mjs 相同的问题：createRequire 硬编码开发者本机绝对路径，脚本在其他机器/CI 上无法解析
@playwright/test，损害 evidence 的可复现性。建议统一改为基于 import.meta.url 的相对路径解析。

- const require = createRequire('/Users/wuyongjun/trea/WeKnora-fork01/.worktrees/issue72-lago/apps/web/package.json');
+ const require = createRequire(new URL('../../../apps/web/package.json', import.meta.url));


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:27-28 ───
[security · low] 租户 D 的邮箱与密码以字面量硬编码且无环境变量覆盖，与 browser_flow_82.mjs 的 process.env.FLOW82_EMAIL ??
'...' 模式不一致，也不符合本迁移分支"凭据只从环境变量读取、源码与测试不写入可用凭据字面量"的约束精神。虽然已确认这是 README 记录的一次性本地 lab 账号，仍建议对齐使用 env
覆盖写法。

-   await page.fill('#auth-email', 'issue82-flow-d@verify.local');
-   await page.fill('#auth-password', 'issue82-Flow-Pw-d');
+   const EMAIL = process.env.FLOW82_EMAIL_D ?? 'issue82-flow-d@verify.local';
+   const PASSWORD = process.env.FLOW82_PASSWORD_D ?? 'issue82-Flow-Pw-d';
+   await page.fill('#auth-email', EMAIL);
+   await page.fill('#auth-password', PASSWORD);


─── docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py:12-12 ───
[style · low] 模块 docstring 中 "Endpooints" 为拼写错误（应为 "Endpoints"）。全库检索确认该错误仅存在于本文件——docstring 声称本文件是
#81 helper 的 verbatim 复用，但 #81 原版拼写正确，说明是复制时新引入的笔误，会影响文档可读性，建议修正。

- Endpooints implemented (only what the adapter calls):
+ Endpoints implemented (only what the adapter calls):


─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:15-18 ───
[documentation · low] stub 运行时硬依赖同目录的三个本地密钥文件（alipay_verify_local*.pem /
alipay_merchant_local.pem），但已确认这些 .pem 未提交入库（正确，避免密钥入库），且整个目录（含 README
的"运行方式（复现）"小节）没有任何密钥再生成命令的记录——README 仅注明"本地测试密钥对（无真实凭据价值）"。第三方按 README 复跑时 openssl dgst -sign
会因密钥缺失直接 check=True 抛错，evidence 链路无法复现。建议在本 docstring 或 README 中补充 openssl genrsa / rsa
-pubout（PKIX）/ pkcs8 转换的具体生成命令（含 WEKNORA_ALIPAY_PUBLIC_KEY_PATH / WEKNORA_ALIPAY_MERCHANT_KEY_PATH
对应格式）。



─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:108-112 ───
[bug · high] 该保护条件恒为 False，是死分支：当 `archived.exists()` 为 True 时，第 30-31 行已保证 `path ==
str(archived)`，故 `path != str(archived)` 必为 False；当 `archived` 不存在时（恰是 runs/ fallback
生效的场景），`archived.exists()` 本身为 False。因此 R1-V10 注释与 docstring 声明的保护——"fallback TSV 上 payments 聚合无法按
run 键控，应响亮地降级为 MISSING-EVIDENCE (exit 2)"——永远不会触发：外来 run 的 TSV 仍会被用于计算不可判定的 `max_succeeded()`
断言，警告也永不打印，导致 fallback 场景下产生误导性的 exit 1 (CHECK) 而非声明的 exit 2。条件应为 `not archived.exists()`（等价于 `path
!= str(archived)`）。注意：issue-72-flow-evidence-74 与 issue-72-ocr3-replay 目录下的 verify_db_watch.py
副本也带有同一恒假条件，建议一并修复。

- if archived.exists() and path != str(archived):
+ if not archived.exists():
      print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
            "whole-DB aggregate and cannot be keyed to this run, so the "
            "exactly-once max_succeeded assertion is not decidable")
      sys.exit(2)


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:108-112 ───
[bug · high] 该守卫分支是永假死代码：上方第 31-38 行的选择逻辑决定了只要 `archived.exists()` 为真，`path` 就已被赋值为
`str(archived)`，两个条件不可能同时成立；`archived` 不存在时第一个合取项又为假。因此注释声明的安全行为——在 runs/ 回退 TSV 上因 payments
全库聚合无法按键隔离而降级为 MISSING-EVIDENCE（exit 2）——永远不会触发。虽然 ocr-3 目录当前提交了归档 TSV（默认路径不受影响），但本仓库的 replay
工作流正是把该脚本复制到新证据目录（74→ocr1→ocr2→ocr3 皆如此），新目录无归档 TSV 而本机 runs/ 留有旧 TSV 时，回退路径上 `max_succeeded() == 1`
的 exactly-once 断言会对来自其他 run 的不可隔离数据照常执行，可能产生虚假 CHECK（exit 1），破坏脚本文档化的 exit-code 契约。此条件从 ocr-2
副本原样继承，两份同样失效。修复：守卫应针对"处于 runs/ 回退"这一状态，即 `not archived.exists()`（能走到这里 matches 必非空，前方的 else 分支已
exit 2）。

- if archived.exists() and path != str(archived):
+ if not archived.exists():
      print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
            "whole-DB aggregate and cannot be keyed to this run, so the "
            "exactly-once max_succeeded assertion is not decidable")
      sys.exit(2)


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:90-92 ───
[bug · medium] 这条 ocr-3 新增断言缺少存在性守卫：当 `deferred_recheck` 的 `invoice_count`/`invoice_count_baseline`
与 `evidence.baseline.invoice_count` 因 schema 漂移同时缺失时，`None == None` 使断言空转通过（vacuous PASS），与本文件
docstring 明确声明的设计不变量（"guards every bare all(...values()) with an existence check — an empty checks
dict is a FAIL, never a vacuous pass"）相矛盾——关键的无新发票断言在证据键缺失时将形同虚设。已核实当前提交的 t02-duplicates.json
中三键均存在（均为 1），故为 replay/旧版 phases.py 证据下的潜在缺口而非现行错误；另外 `d["evidence"]["baseline"]` 为直接索引，键缺失时会以
KeyError 崩溃（traceback、无 RESULT 行）而非记名 FAIL。建议按本文件其余断言的守卫风格补 `is not None` 检查。

  check("AC2 duplicates invoice count == baseline",
-       dr.get("invoice_count") == dr.get("invoice_count_baseline")
+       dr.get("invoice_count") is not None
+       and dr.get("invoice_count") == dr.get("invoice_count_baseline")
        and d["evidence"]["baseline"].get("invoice_count") == dr.get("invoice_count"))


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:11-11 ───
[documentation · low] docstring 声称 "Derived from the ocr-2 copy (97 lines)"，但 ocr-2
副本（docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py）实际为 99 行内容（本次变更清单 +99/-0，含末尾空行共 100 行），97
不准确。本文件内的行号注记（"# ocr-2:" at 83/99、"# ocr-3:" at 88/105）经逐行核对均正确，且该项目的 replay 注记专门用于跨轮自审（R1-V23
即因前序行数/行号声明错误而设立），行数声明不准会误导后续 replay 的交叉复核。

- evidence directory. Derived from the ocr-2 copy (97 lines): it keeps the
+ evidence directory. Derived from the ocr-2 copy (99 lines): it keeps the


LLM retry report summary: 8 of 65 requests affected -- 5 requests failed, 3 requests recovered after retry

Review planning (1 request):
- docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr2-replay/verify_db_watch.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed

Core review (6 requests):
- docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py,docs/plans/issue-72-flow-evidence-82/alipay_sandbox_notify.py,docs/plans/issue-72-flow-evidence-82/api-01-draft.json,docs/plans/issue-72-flow-evidence-82/api-02-publish.json,docs/plans/issue-72-flow-evidence-82/api-03-quote-b.json,docs/plans/issue-72-flow-evidence-82/api-04-purchase-alipay.json,docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs,docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs,docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs,docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr2-replay/verify_db_watch.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr3-replay/verify_db_watch.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- docs/plans/issue-72-ocr2-replay/t02-activation.json,docs/plans/issue-72-ocr2-replay/t02-cleanup.json,docs/plans/issue-72-ocr2-replay/t02-decline.json,docs/plans/issue-72-ocr2-replay/t02-duplicates.json,docs/plans/issue-72-ocr2-replay/t02-environment.json,docs/plans/issue-72-ocr2-replay/t02-gating.json,docs/plans/issue-72-ocr2-replay/t02-manual.json,docs/plans/issue-72-ocr2-replay/t02-provider.json,docs/plans/issue-72-ocr2-replay/t02-retries.json,docs/plans/issue-72-ocr2-replay/t02-setup.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- docs/plans/issue-72-ocr2-replay/t02-activation.json,docs/plans/issue-72-ocr2-replay/t02-cleanup.json,docs/plans/issue-72-ocr2-replay/t02-decline.json,docs/plans/issue-72-ocr2-replay/t02-duplicates.json,docs/plans/issue-72-ocr2-replay/t02-environment.json,docs/plans/issue-72-ocr2-replay/t02-gating.json,docs/plans/issue-72-ocr2-replay/t02-manual.json,docs/plans/issue-72-ocr2-replay/t02-provider.json,docs/plans/issue-72-ocr2-replay/t02-retries.json,docs/plans/issue-72-ocr2-replay/t02-setup.json: rate limited (HTTP 429) -> succeeded
- ... and 1 more

Comment filtering (1 request):
- docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py,docs/plans/issue-72-flow-evidence-82/alipay_sandbox_notify.py,docs/plans/issue-72-flow-evidence-82/api-01-draft.json,docs/plans/issue-72-flow-evidence-82/api-02-publish.json,docs/plans/issue-72-flow-evidence-82/api-03-quote-b.json,docs/plans/issue-72-flow-evidence-82/api-04-purchase-alipay.json,docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs,docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs,docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs,docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed

Per-attempt detail: --format json (retry_report).
