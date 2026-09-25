Review complete: 50 finding(s) across 104 selected item(s).

─── deploy/lago-lab/payment-activation/clients.py:390-404 ───
[maintainability · low] last_error 是死代码：401 行 last_error = error 赋值后从未被读取，最后一次尝试时 403 行的裸 raise 在
except 块中直接重抛当前活动异常，并不依赖该变量。它的存在会误导维护者以为重抛或日志使用了它，建议删除。

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


─── deploy/lago-lab/payment-activation/phases.py:1414-1419 ───
[test · low] settled（canceled）分支的 checks 缺少 no_succeeded_payment 断言，与 not-settled 分支不对称：上方 1362 行已采集
observed["payments_succeeded_count"]，但此处只记录不校验。若出现订阅 canceled(payment_failed) 却存在 succeeded payment
的矛盾状态（正是负控要证明"激活仅发生在提供商支付成功"的反向证据），负控仍会 PASS。建议与 not-settled 分支对齐，加入相同检查。

          checks = {
              "canceled": observed["subscription_status"] == "canceled",
              "payment_failed_reason": observed["cancellation_reason"] == "payment_failed",
              "invoice_unpaid_terminal": invoice_unpaid_terminal,
              "entitlements_unusable": es == 404,
+             "no_succeeded_payment": observed["payments_succeeded_count"] == 0,
          }


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:12-12 ───
[documentation · low] 行号失准:两处 "# ocr-2:" 标注实际位于本文件第 76 行和第 86 行(已用搜索核实),docstring 声称 74/84,各偏差 2
行。该句本是为纠正 R1-V23(前版错误声称 "unmodified")而写的自指性说明,如今行号再次失准,同样会误导后续审查轮次对副本差异的定位。建议改为
76/86,或干脆去掉硬编码行号(以标注所在断言描述定位,免疫行号漂移)。

- two ocr-2-specific assertion annotations: "# ocr-2:" at lines 74 and 84
+ two ocr-2-specific assertion annotations: "# ocr-2:" at lines 76 and 86


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:15-15 ───
[maintainability · low] 死代码:R3-17 移除 runs/ 回退逻辑后,`import glob` 在本文件再无任何使用点(全文件无 `glob.`
调用),应连同下方未引用的 `RUNS` 常量一并删除。



─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:22-29 ───
[maintainability · medium] 两个问题:1) `RUNS` 常量定义后从未被引用,与上方的 `import glob` 同为 R3-17 移除 runs/ 回退后的死代码;2)
本注释块末句 "Fall back to the newest runs/ TSV only when no archive exists, and always print the file
actually used" 与紧随其后的 R3-17 注释及实际代码(无归档时直接 `sys.exit(2)`)直接矛盾。后续维护者若只读上方注释可能恢复该回退路径——而 R3-17 已判定
runs/ TSV 是无法按 run 键控的全库聚合,会让 exactly-once(max_succeeded==1)断言在跨 run 的外来数据上静默求值,破坏证据结论。建议删除 `RUNS`
行,并把注释末句改为与 exit 2 行为一致。

- RUNS = Path(__file__).resolve().parents[3] / "deploy/lago-lab/payment-activation/runs"
  # Prefer THIS evidence directory's archived copy (ocr-2): replayed observer
  # TSVs land in the repo-level runs/ dir and db_watch.sh writes a fresh
  # timestamped file per invocation, so "newest TSV under runs/" can silently
  # pair a foreign run's samples with this directory's decline boundary (and
- # the pick was never printed). Fall back to the newest runs/ TSV only when
- # no archive exists, and always print the file actually used.
+ # the pick was never printed). No runs/ fallback (R3-17): a runs/ TSV is a
+ # whole-DB aggregate that cannot be keyed to this run, so a missing archive
+ # degrades to MISSING-EVIDENCE (exit 2) below.
  archived = EVID / "verify-db-watch-samples.tsv"


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:126-126 ───
[documentation · low] R1-V22 将本行改为 "{midrun_lines}/{len(lines)}" 格式与中段计数口径后,同目录归档的执行输出
verify-db-watch-output.txt 未同步刷新,仍是修正前格式与旧口径:"samples: 197 (mid-run before
2026-09-23T08:58:42Z)"——其中 197 是含尾段(decline 终局 + 清理删除)的全部行数 len(lines);而本 TSV 共 197 行,严格早于边界的中段样本实际为
176 行(08:47:04–08:58:40),当前脚本在该证据上会打印 "samples: 176/197 mid-run before ..."。即本脚本(docstring 声称
"executed against the docs/plans/issue-72-ocr2-replay/ evidence
directory")已无法复现自己的归档输出,后续审计轮对照脚本与证据时会误判输出被篡改或脚本未按当前版本执行。建议用修正后脚本重新执行并刷新该归档输出(断言结论仍为 PASS,仅样本计数行更新)。



─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:112-112 ───
[maintainability · low] 存在性守卫与文件自述不变量不一致：第95-98行注释明确声称"缺键应输出命名 FAIL 而非 KeyError 崩溃"（ocr-1 R1-37），但本行
`r["observed"]["pending_gate_retry"]` 及下一条 check 中的 `r["evidence"]["responses"]["retry_payment"]`
仍是直接下标——同一条 check 内对 `gr` 用 `.get()`、对 `r` 用下标，风格割裂。同类下标访问还见第44-49行 `env["run"]...`、第56-57行
`o["subscription_status"]`/`o["entitlements_status"]`、第81-82行 `oa["checks"]` 等、第85行
`d["observed"]["final"]`、第108行 `r["observed"]["checks"]`。已核对当前提交的
t02-retries/t02-duplicates/t02-decline 证据键齐全，运行不会触发；但作为重放校验脚本，若未来证据键名漂移将以裸 KeyError 终止而非列出命名 FAIL
清单，削弱失败诊断性，也与文件内已有的守卫风格（第99-103行 `_baseline` 的 `.get` 链）不一致。建议统一按该模式改写。

- gr = r["observed"]["pending_gate_retry"]
+ gr = (r.get("observed") or {}).get("pending_gate_retry") or {}
+ check("AC3 gate retry probe recorded not_applicable",
+       gr.get("http_status") is None
+       and ((r.get("evidence") or {}).get("responses", {}).get("retry_payment") or {}).get("code") == "not_applicable")


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:15-15 ───
[other · low] `import glob` 在 R3-17 改动移除 runs/ 回退逻辑后不再有任何调用（全文无 `glob.` 引用），属于回退分支删除后的导入残留，建议删除。

- import glob
+ # 删除此行（与下方 RUNS 一并清理）


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:22-22 ───
[other · low] 模块级变量 `RUNS` 赋值后从未被读取，是已删除的 runs/ 回退分支的残留；且其上方第27-28行注释仍写着 "Fall back to the newest
runs/ TSV only when no archive exists"，与实际行为（归档缺失即打印 WARNING 并 exit 2，无任何回退）矛盾，会误导读者以为仍存在 runs/
回退路径。建议将 `RUNS` 赋值行连同该段过时回退描述一并删除。

- RUNS = Path(__file__).resolve().parents[3] / "deploy/lago-lab/payment-activation/runs"
+ # 删除 RUNS 赋值行，并把上方 "Fall back to the newest runs/ TSV only when
+ # no archive exists" 的过时描述改为：归档缺失时直接以 MISSING-EVIDENCE 退出


─── internal/handler/commercial.go:531-534 ───
[bug · low] PurchaseStatus 的残差分支以 400 + 原始 err.Error() 回答,与同一 diff 中 POST /purchases
残差分支刻意建立的姿态相悖:该分支承载的是服务端故障(存储读失败等),400 会让客户端把服务端故障当自己的请求错误处理;err.Error() 可能携带 gorm/驱动细节越过公共边界,而 POST
分支同位置明确写了"never raw error text across the public boundary"。建议对齐:500 + 闭合文案,原始错误仅进服务端日志(文件已引入 log)。

- 	case errors.Is(err, commercialsvc.ErrQuoteTenantMismatch):
- 		c.JSON(http.StatusNotFound, gin.H{"error": "purchase not found for this tenant"})
  	default:
- 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
+ 		log.Printf("commercial: purchase status failed for tenant %d: %v", tenantID, err)
+ 		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase status failed"})


─── internal/handler/commercial.go:497-500 ───
[bug · medium] POST /purchases 的 switch 缺少两个可达 sentinel 的分支，均落入 default → 500，与 default
分支自己声称的「残差是服务端故障」矛盾：
1) `repocommercial.ErrQuoteNotFound`：Purchase 第 1 步直接透传 `QuoteSnapshotForTenant` 的错误，而
`CatalogStore.GetQuote`（catalog.go:172-179）对不存在的 quote 返回 ErrQuoteNotFound——拼错/已清理的 quote_id
是最寻常的客户端输入，同一 switch 中 tenant-mismatch 已答 404（"quote not found for this tenant"），GET /orders/:id 同样答
404，这里却答 500 并记一条 purchase failed 的服务端错误日志（误导告警）。
2) `repocommercial.ErrPurchasePendingExists`：PurchaseService 的 sweep-重试路径明确设计了「二次冲突 → surface the
conflict error, never loop」，该 sentinel 会从这里逃逸到 handler；同一 diff 中 POST /orders handler 已为它加了 409 +
附带现有 pending order 的分支，而 POST /purchases 没有——并发冲突被答成 500。
建议在 default 前补上这两个 case。

  	case errors.Is(err, repocommercial.ErrQuoteAlreadyUsed):
  		// (R1-V21) Same race outcome as POST /orders: 409, not a generic 400.
  		c.JSON(http.StatusConflict, gin.H{"error": "quote already used"})
+ 	case errors.Is(err, repocommercial.ErrQuoteNotFound):
+ 		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
+ 	case errors.Is(err, repocommercial.ErrPurchasePendingExists):
+ 		// (R2-26) sweep 后仍冲突的真实并发竞态：409，与 POST /orders 同语义。
+ 		c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists"})
  	default:


─── internal/modules/commercial/commercialplatform/lago_purchase.go:250-260 ───
[bug · medium] 永久性配置形态被归类为可重试的 unreachable:在两种结构性形态下(deriveProviderCustomerID 走占位前缀、或生产
StripePmToken 为空),门控 create 必然 422
no_default_payment_method——支付方式导入在构造上永远不可能落地。ensureProviderBinding 的跳过轮询分支(R1-V06/R1-V24)自己已认定这两种形态
"the import can never land",config.go 的 F11 注释也承诺 binding 无默认支付方式时 create "fails closed",但此分支却返回瞬态
sentinel ErrPlatformUnreachable。服务层(purchase.go)将该 sentinel 透传为 503 + reason
"unreachable",客户端/运维会按瞬态网络故障反复重试一个结构性不可能成功的购买,错误定性永久失真。建议在配置不可能产生默认支付方式时直接返回终态
ErrPlatformUnconfigured(与 F2 payment_provider_not_found 的处理对齐)。

  	case status == http.StatusUnprocessableEntity &&
  		strings.Contains(string(respBody), "no_default_payment_method"):
- 		// (R1-12) The authority's payment-method import had not landed when
- 		// the gated create ran (the bounded sync wait above is the primary
- 		// absorber). The replay's bound short-circuit does NOT re-wait, so
- 		// escalating this transient import lag to a terminal verdict would
- 		// permanently lose the sync window — classify it as the retryable
- 		// unreachable instead: the import keeps progressing server-side and
- 		// the next attempt's create succeeds.
+ 		// Structural postures can never import a default payment method:
+ 		// a placeholder binding has no provider customer, and an empty
+ 		// StripePmToken means none was ever attached (config.go F11). Answer
+ 		// the definitive unconfigured sentinel so the wire reason stops
+ 		// promising a retry that cannot succeed.
+ 		if a.cfg.StripeAPIKey == "" || a.cfg.StripePmToken == "" {
+ 			return commercial.CommandReceipt{}, fmt.Errorf(
+ 				"%w: no default payment method source configured", commercial.ErrPlatformUnconfigured)
+ 		}
+ 		// (R1-12) Genuine import lag stays retryable — the import keeps
+ 		// progressing server-side and the next attempt's create succeeds.
  		return commercial.CommandReceipt{}, fmt.Errorf(
  			"%w: default payment method not imported yet", commercial.ErrPlatformUnreachable)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:552-557 ───
[bug · low] 429 被折叠为终态 invalid_response:Stripe 的限流响应(429)是典型瞬态失败,与 503 同属可重试类别,但此处(以及
providerAttachDefaultPaymentMethod 的 attach/default 两次分类)将其归入 ErrPlatformInvalidResponse。R1-V20
注释明确声明瞬态/终态契约——"folding it into the terminal invalid-response sentinel would stop callers from ever
retrying"——429 恰好落入该陷阱:线上 reason token 为 invalid_response(终态语义),客户端会放弃一次本可稍后成功的购买。建议与 5xx 一并归为
unreachable。

  	if status < 200 || status >= 300 {
- 		if status >= 500 {
+ 		if status >= 500 || status == http.StatusTooManyRequests {
+ 			// 429 rate limiting is transient, same class as 5xx (R1-V20).
  			return "", fmt.Errorf("%w: provider customer create unavailable", commercial.ErrPlatformUnreachable)
  		}
  		return "", fmt.Errorf("%w: provider customer create rejected", commercial.ErrPlatformInvalidResponse)
  	}


─── internal/modules/commercial/commercialplatform/lago_purchase.go:166-168 ───
[security · low] isPlausibleHostname 放行单标签主机名:"api"、"internal" 等单标签名能通过全部校验(唯一标签同时充当 TLD 且以字母开头)。在
Kubernetes/容器环境,系统解析器会对单标签名应用 search domain 并可能解析到内网 Service/ClusterIP,从而绕过 S1
出站策略的意图——validateOutboundHost 的文档声明 "a hostile configuration can never point the egress at internal
infrastructure",但单标签 + search domain 是该声明的具体反例。建议至少要求不少于两个标签(主机名含一个点)。

+ 	if len(labels) < 2 {
+ 		// Single-label names resolve via resolver search domains in
+ 		// container/K8s environments and can reach internal services.
+ 		return false
+ 	}
  	tld := labels[len(labels)-1]
  	return tld[0] >= 'a' && tld[0] <= 'z'
  }


─── internal/modules/commercial/commercialplatform/lago_purchase.go:337-337 ───
[security · medium] S1 出站校验覆盖缺口：validateOutboundHost 的文档声称约束 "every server-side outbound request
this adapter issues"，且验收条件要求服务端外部请求一律先校验 host（拒绝 localhost/环回/私有/保留地址），但它只在
providerOutboundCall（Stripe 侧）与重定向检查中被调用。本 diff 新增的 Lago 侧出站调用（/api/v1/customers
GET/POST、/payment_methods 轮询）全部经 a.do 直拼 a.cfg.BaseURL，无任何 host 校验。StripeAPIBase 与 BaseURL
是同一信任级别（均为服务端 env），前者校验、后者不校验不一致：BaseURL 配置失误指向 10.x/127.0.0.1 等内部地址时不会被拒绝，与 validateOutboundHost
自身声明的意图矛盾。建议在 configured()（或 a.do 构造请求前）对 BaseURL 做同策略校验；若因测试需用 127.0.0.1 stub 而无法直接强制，应提供显式的
dev/test 旁路标记，而非完全不校验。

- 	status, respBody, err := a.do(ctx, http.MethodPost, "/api/v1/customers", body)
+ // 在 configured() 或 a.do 发起请求前：
+ if err := validateOutboundHost(a.cfg.BaseURL); err != nil {
+ 	return fmt.Errorf("%w: platform base host policy violation", commercial.ErrPlatformUnconfigured)
+ }
+ // （dev/test 需用环回 stub 时，通过显式的本地回环旁路开关豁免，而非默认不校验）


─── internal/modules/commercial/service/commercial/order.go:119-123 ───
[bug · medium] 启动回填与滚动部署/多实例的在途订单存在同类竞态:该 UPDATE 在每次启动时把所有 link-less、channel_failed=false 的 pending
购买单标为 channel_failed。注释假设它们"全部是旧管线存量行",但在多实例部署中,另一实例可能正处于"已插行、渠道 Create
在途、链接未落库"的窗口——本实例重启执行回填会把那行活单标死,释放唯一索引槽,下一笔 checkout 即开出第二张渠道单,而原单链接仍可支付(双重扣款,同
SweepStaleLinklessPending 的问题)。存量行与本版本新建行的可靠区分点是 created_at:该列随本次变更一起引入,旧管线行恒为 NULL,而新代码
createOrderTx 总会写入时间戳。建议把回填限定在存量形状(created_at IS NULL),把上线后产生的 link-less 残留留给带时限的 sweep 路径处理。

  	if err := db.Exec(`UPDATE commercial_orders SET channel_failed = true
  		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = false
+ 		AND created_at IS NULL
  		AND (checkout_url IS NULL OR checkout_url = '')`).Error; err != nil {
  		return nil, fmt.Errorf("commercial pending-purchase backfill: %w", err)
  	}


─── internal/modules/commercial/service/commercial/purchase.go:276-279 ───
[bug · high] sweep+重试与并发胜者的在途窗口竞态,可对同一 gating invoice 开出两张可付渠道单 → 双重扣款。openOrder
的顺序是:插行(channel_failed=false、无链接)→ 渠道 Create(网络调用,秒级窗口)→ SetCheckoutURL。处于该在途窗口的胜者订单与
persist-degraded 残留形状完全相同(pending + channel_failed=false + checkout_url='')。此时败者走
ErrPurchasePendingExists 分支:CurrentPayablePendingOrder 找不到可付单 → SweepStaleLinklessPending 把在途的活行误标
channel_failed=true → 释放唯一索引槽 → 重试 CreateOrder 成功,开出第二张渠道单。而胜者的客户端已经从其 201 应答里拿到可用链接,SetCheckoutURL
又只按 id 更新(无 channel_failed 守卫),链接仍会落库;两张单各自回调 ConfirmPayment 独立确认(键是订单级
merchant_order_id,无购买级幂等拦截),客户可对同一订阅付两次款。注释假设"link-less 残留只可能是
persist-degraded"不成立——在途窗口是常态形状,且比持久化降级常见得多。同理,即使是真正的 persist-degraded
残留,其链接仍在客户手中且渠道侧仍可支付,"不可重放"不等于"不可支付"。建议:sweep 仅限确定已死的行(如 created_at 早于 渠道超时+checkoutPersistTimeout
的阈值),sweep 未命中时直接以冲突错误回答让客户端稍后重试/轮询重放,而不是开第二张渠道单;或对租户级 checkout 串行化(advisory lock),让败者等待胜者渠道调用结束。



─── internal/modules/commercial/service/commercial/purchase.go:404-408 ───
[bug · low] 重放路径丢失了渠道失败的 202 姿态（R1-V14）：渠道 Create 失败时原始应答是 202 + `checkout_error`（订单
pending、无链接），客户端按提示通过 GET /orders/:id 恢复；但客户端重试同一 quote 时会走 `GetOrderByQuote` 重放（ErrQuoteAlreadyUsed
竞态重放同理），`orderViewFromRow` 无法重建 CheckoutError（该字段不落库）→ handler 命中 `err == nil` 分支答 201
"Created"，返回一个既无 checkout_url 又无 checkout_error 的 pending 订单， checkout 页拿不到支付入口也拿不到错误信号。行上已有
`ChannelFailed` 列，足以重建该姿态。

  func orderViewFromRow(r repocommercial.OrderRow) OrderView {
- 	return OrderView{ID: r.ID, QuoteID: r.QuoteID, State: r.State,
+ 	ov := OrderView{ID: r.ID, QuoteID: r.QuoteID, State: r.State,
  		AmountFen: r.AmountFen, Currency: r.Currency, CheckoutURL: r.CheckoutURL,
  		Version: r.Version}
+ 	// 渠道失败标记的 pending 单在重放时保持 202 姿态（R1-V14）：
+ 	// CheckoutError 不落库，用行上的 ChannelFailed 列重建闭合 token。
+ 	if r.ChannelFailed && r.State == domain.OrderStatePending && r.CheckoutURL == "" {
+ 		ov.CheckoutError = "channel checkout failed for " + r.ID
+ 	}
+ 	return ov
  }


─── packages/contracts/src/commercial.ts:199-199 ───
[bug · low] parsePurchaseView 对可选字段的空值处理不一致：amount_fen 显式容忍 null（第196行 `v.amount_fen!==null`），而
currency 的守卫只排除 undefined 和空串——`currency:null` 会通过守卫进入 nonEmptyString 直接抛错，使整个视图解析失败为 'invalid
purchase (currency)'。这正是本函数 R2-21 注释所规避的「advisory 元数据导致整视图失败」模式。当前 Go 端 purchaseWire 仅在 Currency
非空时输出（string+omitempty），null 不会上链，属版本漂移（后端或中间层把空串置为 null）下的潜在脆弱点，建议补齐 null 守卫保持两个字段行为对称。

-   if(v.currency!==undefined&&v.currency!=='') out.currency = nonEmptyString(v.currency,'currency','purchase');
+   if(v.currency!==undefined&&v.currency!==null&&v.currency!=='') out.currency = nonEmptyString(v.currency,'currency','purchase');


─── apps/web/src/commercial/CheckoutPage.tsx:153-154 ───
[documentation · low] 本变更换掉了 createOrder+幂等键流程（idempotencyKeyRef 已删除、purchase 仅传
quote_id），但错误态的重试按钮文案仍是「重试（复用原订单与幂等键，不重复下单）」，两处已与实际行为脱节：①「幂等键」机制在新流程中已不存在；②报价级冲突（isQuoteLevelConflic
t）会清空 quoteRef，此时重试实际会重新报价并创建一张新订单，与「复用原订单…不重复下单」相矛盾，可能让用户误解会产生重复扣款或找不到新订单。建议同步更新该按钮文案。

-         // #81：提交走 payment-gated purchase。重试语义：同一 quote 重试就是同一次
-         // purchase 调用，后端按身份幂等返回同一订单（不产生第二张订单/第二张账单）。
+ // 按钮文案建议（错误态重试按钮）：
+ <Button type="button" onClick={() => setRetryToken((value) => value + 1)}>重试（同一报价服务端幂等；报价失效时自动重新报价）</Button>


─── apps/web/src/commercial/CheckoutPage.tsx:269-270 ───
[maintainability · low] ready 分支中安全判定 `state.order.checkout_url &&
isSafeCheckoutUrl(state.order.checkout_url)`
被完整计算了两次（渲染「前往支付」链接处与此处反向判断）。这是支付链接的安全门，双写容易在后续修改中漂移（一处改判定、另一处遗漏）。建议在 ready 分支提取局部常量，保持单一来源。

-               {state.order.payment === 'pending'
-                 && !(state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url)) ? (
+ const checkoutHref = state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url)
+   ? state.order.checkout_url
+   : null;
+ // …
+ {checkoutHref ? (
+   <p><a href={checkoutHref} target="_blank" rel="noreferrer">前往支付</a></p>
+ ) : null}
+ {state.order.payment === 'pending' && !checkoutHref ? (
+   <>
+     <Status tone="error">支付渠道异常，此订单暂无可用支付链接</Status>
+     <Button type="button" onClick={restartCheckout}>重新发起支付（获取新报价）</Button>
+   </>
+ ) : null}


─── deploy/lago-lab/payment-activation/evidence/t02-retries.json:103-105 ───
[documentation · medium] 权威证据快照与当前生成代码键集漂移（final-report 4-03，已核验成立但未修复）：1) pending_gate_retry 缺少
subscription_status_after 键——phases.py:1255 在 retries 阶段无条件写入该键（ocr-3
修复），docs/plans/issue-72-ocr3-replay/t02-retries.json:109 已包含；2) http_status: 404 +
retry_payment.code "invoice_not_found" 是 ocr-2 修复前旧路径的输出：同一 run 的 gate 阶段
invoice_api_visible=false（invoice_lago_id=None），旧代码实际对 /api/v1/invoices/None/retry_payment 发起 POST
才得到 404，当前 phases.py:1203-1220 明确跳过该探针并记录 code "not_applicable"（注释称把这种 404 当重试证据是 "dressing the 404
up as retry evidence"）。本快照无法通过最新 verify_ac_assertions.py（ocr3 副本 :118 要求 subscription_status_after
in (incomplete, canceled)）。建议用 HEAD phases.py 重跑刷新权威目录（docs/migrations 晋升副本与该目录
byte-identical，需同步），或直接晋升 ocr3-replay 快照，避免权威证据记录生成器已判定为无效的探针结果。



─── deploy/lago-lab/payment-activation/evidence/t02-duplicates.json:6-13 ───
[documentation · medium] 权威证据快照缺少当前生成代码必然写入的两个键（final-report 4-02，已核验成立但未修复）：1) evidence.baseline 缺
invoice_count——phases.py 在 duplicates 阶段为 baseline 记录探测前发票数（OCR3-05 锚点，见
issue-72-ocr-fix-log.md:360）；2) observed 缺 deferred_recheck（phases.py:1073 起写入，含
checked/ok/invoice_count_baseline/invoice_count_immediate）。docs/plans/issue-72-ocr3-replay/t02-dupli
cates.json:7,113 两键均有，而本快照（及 byte-identical 的 docs/migrations 晋升副本）均无。最新
verify_ac_assertions.py（ocr2/ocr3 副本 :78-103）断言 deferred_recheck.checked/ok 为真且
baseline.invoice_count == deferred_recheck.invoice_count，对本文件会因键缺失直接判 fail，AC2
的延迟复查（重注册后迟发终止/续期发票）证据在权威目录缺位。建议以 HEAD phases.py 重跑刷新或晋升 ocr3-replay 快照。



─── deploy/lago-lab/payment-activation/evidence/t02-gating.json:11-12 ───
[documentation · low] observed 缺少 payments_non_succeeded_count 键：当前 phases.py 在发票 API
不可见分支（:575-577，ocr-81 R1-08 修复）无条件写入该键，而本快照 invoice_api_visible=false 走的正是该分支；检查发现所有留存副本（deploy
权威目录与全部 ocr replay 目录）的 gate 证据均无此键，说明 gate 阶段证据全部生成于该修复落地之前，AC1 不可见分支的"至多一笔非成功支付"恰缺这行运行时证据。当前
verify_ac_assertions.py 仅在发票可见分支读取该键（o.get(..., 99) <= 1），故不会导致断言失败，影响限于证据完备性。建议与
t02-duplicates/t02-retries 一并用 HEAD 代码重跑刷新，使权威快照键集与生成器一致。



─── deploy/lago-lab/payment-activation/evidence/t02-retries.json:97-102 ───
[documentation · medium] 权威证据快照与当前生成代码的键集漂移（独立于已确认的 4-03 两条：缺 subscription_status_after、旧
404/not_found 路径）：1) observed.checks 缺 deferred_no_drift——phases.py:1303 无条件写入该 check；2) observed 缺
deferred_recheck——phases.py:1305 无条件写入（含
checked/ok/subscription_status/subscription_same_lago_id/payments_succeeded_count/invoice_count）。这是
OCR-R3-39 修复（issue-72-ocr-fix-log.md:1373-1388，retries 加入 duplicates 式 120s 延迟复查，RED 测试
test_retries_deferred_recheck_catches_late_termination 钉死键存在）落地后的新增缺口；本快照（recorded_at
2026-09-23T05:17）与全部 ocr replay 副本均生成于该修复之前，均无此二键。作为 AC3（重复注册不产生第二商业对象）的延迟复查运行时证据，快照需重跑 run_lab.py
后才会携带，建议更新权威快照或在文档中标注该缺口。

      "checks": {
+       "deferred_no_drift": true,
        "gate_not_activated": true,
        "no_second_payment_row": true,
        "no_succeeded_payment_for_gate": true,
        "same_identity_recovered": true
+     },
+     "deferred_recheck": {
+       "checked": true,
+       "invoice_count": 1,
+       "ok": true,
+       "payments_succeeded_count": 1,
+       "subscription_same_lago_id": true,
+       "subscription_status": "active"
      },


─── apps/web/src/commercial/CheckoutPage.tsx:164-167 ───
[bug · medium] 该分支在 `await client.commercial.purchase(...)` 之后直接调用 `setState`，缺少 `run()`
内其他所有异步落态路径（成功路径 L171、catch 路径 L175）都持有的 `active && scopeController.isCurrent(currentScope.scope)`
守卫。若 POST /purchases 在途期间用户切换租户或点击「重试」（retryToken 变化触发 effect 重跑，旧 run 的 active 已置 false、新 run
已把状态重置为 loading），迟到的旧 purchase 响应若恰好 order 缺席，会把上一租户/上一次请求的错误文案覆盖到新 run 的状态上——这正是 BillingPage 中
R1-V15 注释专门防范的同类「陈旧数据覆盖」竞态。建议补上与其他分支一致的守卫。

          if (!purchase.order) {
+           if (active && scopeController.isCurrent(currentScope.scope)) {
-           setState({ status: 'error', message: purchaseErrorMessage(purchase) });
+             setState({ status: 'error', message: purchaseErrorMessage(purchase) });
+           }
            return;
          }


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:11-18 ───
[maintainability · low] 死代码:`import glob` 与 `RUNS` 在按 (R3-17) 移除 runs/ 回退逻辑(无归档 TSV 即直接
`sys.exit(2)`)后,全文件已无任何代码引用,仅剩注释提及。同时第 20-23 行注释 "Fall back to the newest runs/ TSV only when no
archive exists" 描述的回退行为与当前代码(直接退出 2)相矛盾,会误导后续维护者以为仍存在 runs/
读取路径。建议一并删除这两行死代码并修正注释(同目录测试通过复制脚本执行,不引用这两个名字,删除安全)。

- import glob
  import json
  import re
  import sys
  from pathlib import Path
  
  EVID = Path(__file__).resolve().parent
- RUNS = Path(__file__).resolve().parents[3] / "deploy/lago-lab/payment-activation/runs"


─── docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py:25-27 ───
[documentation · medium] docstring 中的失败签名描述与实际行为不符。实际传播链：SSRF 校验失败发生在
WechatProvider.do（wechat.go:544-546），被包装为 payment.ErrNotConfigured（"wechat_not_configured"）；该错误经
openOrder（order.go:432-448）作为渠道失败处理，返回 pending 订单视图 + CheckoutError 且 error 为
nil；购买处理器（commercial.go:452-461）对该形态回答 202 Accepted，而非 503。而 503 payment_provider_unconfigured 仅在
provider 名不在注册表时产生（purchase.go:130、order.go:375/518），ProviderConfigured 是纯 map
查找（order.go:318-325），不涉及 SSRF 网关。此说明会误导后续复现者的排障方向——他们会在 503/错误码层面排查，而实际症状是 202 + 订单携带
checkout_error。

  Replaying the #81 evidence flow against this stub must export that variable
- before starting the WeKnora backend; without it the purchase answers 503
- payment_provider_unconfigured (the SSRF gate refusing the loopback base).
+ before starting the WeKnora backend; without it WechatProvider.do refuses
+ the loopback base at SSRF validation, the channel Create fails, and the
+ purchase answers 202 with a pending order whose checkout_error reports the
+ channel failure (never a clean checkout link) — the provider-registry
+ PRE-check is a pure map lookup and does not see the SSRF gate.


─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:56-56 ───
[bug · low] KEY_DIR = __file__.rsplit("/", 1)[0] 在两种常见场景下会得到错误目录：1) cd 进入脚本所在目录后以 python3
alipay_gateway_stub.py 运行时，__file__ 为 "alipay_gateway_stub.py"（不含 "/"），rsplit 结果 [0]
即文件名本身，拼出的密钥路径无效，首次签名/验签时 openssl 即报找不到密钥文件；2) Windows 下路径分隔符为 "\\"，同样推导失败。建议改用
os.path.dirname(os.path.abspath(__file__))（并在文件顶部 import os，替代 rsa_verify_sha256 中 finally 内的局部导入）。

- KEY_DIR = __file__.rsplit("/", 1)[0]
+ import os
+ 
+ KEY_DIR = os.path.dirname(os.path.abspath(__file__))


─── docs/plans/issue-72-flow-evidence-82/alipay_sandbox_notify.py:30-30 ───
[bug · low] 与 alipay_gateway_stub.py 相同的问题：__file__.rsplit("/", 1)[0] 在同目录下以 python3
alipay_sandbox_notify.py 运行（__file__ 不含 "/"）或 Windows（"\\" 分隔符）时得到错误目录，拼出的 ALIPAY_KEY 路径无效，openssl
签名时直接报错。建议改用 os.path.dirname(os.path.abspath(__file__))。

- KEY_DIR = __file__.rsplit("/", 1)[0]
+ import os
+ 
+ KEY_DIR = os.path.dirname(os.path.abspath(__file__))


─── docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py:12-12 ───
[documentation · low] 模块 docstring 中 "Endpooints" 为拼写错误，应为 "Endpoints"。该行是说明 stub 实现端点的关键文档行，影响可读性。

- Endpooints implemented (only what the adapter calls):
+ Endpoints implemented (only what the adapter calls):


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:54-56 ───
[test · medium] billing 断言前的固定睡眠有竞态：BillingPage 的 purchaseStatus 是挂载时的一次性请求且失败时静默降级为
null（BillingPage.tsx:83-87），套餐行还要等 summary status==='success' 才渲染（BillingPage.tsx:110-118）。lab
环境后端/代理稍慢时 1500ms 不足以让 fetch 落定，正向断言 billing-awaiting-payment 会误报 FAIL，削弱证据可信度。建议改为等待目标文案出现后再断言。

    await page.goto(`${WEB}/platform/billing`);
    await page.waitForSelector('main', { timeout: 30000 });
-   await page.waitForTimeout(1500); // purchase status fetch settles
+   await page.waitForSelector('text=待付款（权益未开放）', { timeout: 15000 });


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:12-12 ───
[maintainability · low] WEB='http://localhost:5192' 在三份脚本中各自硬编码，报价断言里的 ¥99.00 也是业务数字硬编码（对应
formatCny(9900)）；且 env 覆盖模式（FLOW82_EMAIL/PASSWORD）只有本脚本有，后两个脚本未沿用。更换 dev
端口或套餐价格档位需要改三处源码，且三脚本配置方式不一致。建议统一为环境变量（如 FLOW82_WEB）并在证据 README 中说明。



─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:13-13 ───
[bug · low] 用 new URL('.', import.meta.url).pathname 拼截图目录在 Windows 上会得到带前导斜杠的 /C:/... 路径，Playwright
写截图可能落错位置或直接失败。应改用 node:url 的 fileURLToPath（三份脚本同样问题）。

- const EV = new URL('.', import.meta.url).pathname;
+ import path from 'node:path';
+ import { fileURLToPath } from 'node:url';
+ const EV = path.dirname(fileURLToPath(import.meta.url)) + path.sep;


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:61-63 ───
[maintainability · low] try 只有 finally 没有 catch：任一步骤抛异常（如 waitForSelector 超时）时浏览器虽被关闭，但末尾的
console.log('RESULT ...') 不会再执行，证据输出只剩原始堆栈，无法快速定位失败步骤与已通过清单。建议在 catch 中输出 results
汇总再以非零码退出（三份脚本同样问题）。

+ } catch (error) {
+   note('unexpected-error', false, String(error));
+   console.log('RESULT ' + JSON.stringify(results));
+   process.exit(1);
  } finally {
    await browser.close();
  }


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:17-21 ───
[maintainability · low] note/results 辅助、chromium.launch + try/finally 骨架、登录序列（fill
#auth-email/#auth-password → 提交 → waitForURL）在三份脚本中逐行重复约 30 行。登录表单选择器或登录流程一旦变化需要三处同步修改，建议抽取目录内共享
_lib.mjs（export note/login/WEB 等），顺带统一三脚本的 env 配置方式。



─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:27-28 ───
[security · medium] 租户 D 的邮箱与密码以可用字面量写死且完全没有环境变量覆盖（browser_flow_82.mjs 至少有 FLOW82_EMAIL/PASSWORD
兜底）。这直接违反本分支明示的安全验收约束"源码、示例和测试不得写入可用的凭据字面量"；且 docs/plans/issue-72-ocr-round-1.md（L530-532）与
issue-72-ocr-round-3.md（L120-124）已两次建议改为 FLOW82_PASSWORD_D，脚本至今未落实。建议至少与脚本一对齐 env 化。

-   await page.fill('#auth-email', 'issue82-flow-d@verify.local');
-   await page.fill('#auth-password', 'issue82-Flow-Pw-d');
+ const EMAIL_D = process.env.FLOW82_EMAIL_D;
+ const PASSWORD_D = process.env.FLOW82_PASSWORD_D;
+ if (!EMAIL_D || !PASSWORD_D) {
+   console.error('FLOW82_EMAIL_D / FLOW82_PASSWORD_D are required');
+   process.exit(2);
+ }
+ // ...
+ await page.fill('#auth-email', EMAIL_D);
+ await page.fill('#auth-password', PASSWORD_D);


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:32-35 ───
[test · medium] waitForTimeout(4000) 不可靠且削弱证据可信度：「订单结算」是 header
无条件渲染的文案（CheckoutPage.tsx:237），命中时订单数据可能仍在加载；轮询是固定 setInterval(3000)（CheckoutPage.tsx:11,226）。若带
order 参数的首个 GET / 首个轮询 tick 未完成，页面仍处于 Loading，反向断言 no-false-effective / no-awaiting-payment-leftover
会在空页面上"空泛通过"。建议先用 waitForSelector 等到已付款文案真正渲染，再做反向断言。

    await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
    await page.waitForSelector('text=订单结算', { timeout: 30000 });
-   await page.waitForTimeout(4000); // one poll tick
+   // 先等 paid 文案真正渲染，再取 body 做反向断言，避免 Loading 态"空泛通过"
+   await page.waitForSelector('text=已付款，权益处理中', { timeout: 15000 });
    const body = await page.locator('main').innerText();


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:38-38 ───
[maintainability · low] 前半个条件是冗余的：任何包含「权益已生效」的文本必然包含「已生效」，已被后半个 !body.includes('已生效')
完全覆盖，可删去前者以避免读者误以为两者语义不同。

-   note('no-false-effective', !body.includes('权益已生效') && !body.includes('已生效'),
+   note('no-false-effective', !body.includes('已生效'),


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:12-12 ───
[bug · low] 与 browser_flow_82.mjs 同样的跨平台问题：new URL('.', import.meta.url).pathname 在 Windows 上产生
/C:/... 前导斜杠路径，reverify1/ 子目录的截图可能写入失败。应改用 fileURLToPath。

- const EV = new URL('.', import.meta.url).pathname + 'reverify1/';
+ const EV = path.join(path.dirname(fileURLToPath(import.meta.url)), 'reverify1') + path.sep;


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:51-53 ───
[maintainability · low] 同 browser_flow_82.mjs：try 只有 finally 没有 catch，中途抛异常时 RESULT
汇总不会输出，无法从证据输出定位失败步骤。建议 catch 中输出 results 再非零退出。



─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:26-27 ───
[security · medium] 租户 B
的邮箱与密码以可用字面量写死且无环境变量覆盖，违反安全验收约束"源码、示例和测试不得写入可用的凭据字面量"；docs/plans/issue-72-ocr-round-1.md（L509-511）已建
议改为 FLOW82_PASSWORD_B，脚本未落实。与另两脚本不同，本脚本连 env 兜底都没有，凭据轮换时需改源码。

-   await page.fill('#auth-email', 'issue82-flow-b@verify.local');
-   await page.fill('#auth-password', 'issue82-Flow-Pw-b');
+ const EMAIL_B = process.env.FLOW82_EMAIL_B;
+ const PASSWORD_B = process.env.FLOW82_PASSWORD_B;
+ if (!EMAIL_B || !PASSWORD_B) {
+   console.error('FLOW82_EMAIL_B / FLOW82_PASSWORD_B are required');
+   process.exit(2);
+ }
+ // ...
+ await page.fill('#auth-email', EMAIL_B);
+ await page.fill('#auth-password', PASSWORD_B);


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:31-33 ───
[test · medium] waitForTimeout(4000) 固定睡眠有竞态：「订单结算」标题在数据加载前就渲染，CheckoutPage 轮询是固定
setInterval(3000)。若带 order 参数的首个 GET 未完成，页面仍在 Loading：正向断言 sync-face-no-advance 会误报 FAIL（flaky），反向断言
sync-face-no-benefits 会空泛通过。AC2 是"同步回跳不推进订单"这一关键安全属性，建议用 waitForSelector 等到目标文案出现后再断言，使证据确定性成立。

    await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
    await page.waitForSelector('text=订单结算', { timeout: 30000 });
-   await page.waitForTimeout(4000); // let one poll tick land (3s interval)
+   await page.waitForSelector('text=待付款（权益未开通）', { timeout: 15000 });


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:11-11 ───
[bug · low] 同 browser_flow_82.mjs：new URL('.', import.meta.url).pathname 在 Windows 上产生 /C:/...
前导斜杠路径，截图写入可能失败，应改用 fileURLToPath。



─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:40-42 ───
[maintainability · low] 同 browser_flow_82.mjs：try 只有 finally 没有 catch，中途抛异常时 RESULT 汇总不会输出。建议 catch
中输出 results 再非零退出。



─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:14-15 ───
[security · high] 兜底默认值 `issue82-Flow-Pw-a` 仍是写入源码的可用凭据字面量，直接违反本分支安全验收约束
3（"源码、示例和测试不得写入可用的凭据字面量"）——与已确认的租户 B/D 字面量属同一性质，env 覆盖只解决轮换便利，仓库里的默认密码依然可用。建议去掉可用默认值，env
缺失时直接报错退出（另两脚本一并按此模式对齐）。

- const EMAIL = process.env.FLOW82_EMAIL ?? 'issue82-flow-a@verify.local';
- const PASSWORD = process.env.FLOW82_PASSWORD ?? 'issue82-Flow-Pw-a';
+ const EMAIL = process.env.FLOW82_EMAIL;
+ const PASSWORD = process.env.FLOW82_PASSWORD;
+ if (!EMAIL || !PASSWORD) {
+   console.error('FLOW82_EMAIL / FLOW82_PASSWORD are required (no in-source credential fallback)');
+   process.exit(2);
+ }


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:44-49 ───
[test · medium] billing-no-false-effective 断言无判别力：(1) 固定 waitForTimeout(1500) 有竞态——BillingPage 的
summary 处于 Loading…/error 时套餐行不渲染（BillingPage.tsx:103-110），反向断言会在数据未落定时空泛通过；(2) 更根本的是当前 BillingPage
在任何状态下都不渲染「已生效」文案（全 apps/web 检索仅 order-state.ts 命中），该断言恒真、无法证伪，不能支撑注释声称的 "authority 订阅未完成（D2
frozen）时 billing 不谎报生效"。建议先等待有判别力的正向文案（如「无固定到期（未订阅）」）出现，再以此为主断言、辅以不含生效类词汇的反向检查。

    await page.goto(`${WEB}/platform/billing`);
-   await page.waitForSelector('main', { timeout: 30000 });
-   await page.waitForTimeout(1500);
+   // 等待 summary 成功渲染套餐行（有判别力的正向文案），避免 Loading 态下空泛通过
+   await page.waitForSelector('text=无固定到期（未订阅）', { timeout: 30000 });
    const billing = await page.locator('main').innerText();
-   note('billing-no-false-effective', !billing.includes('已生效'),
-     'billing plan row does not claim 已生效');
+   note('billing-not-effective', billing.includes('无固定到期（未订阅）') && !billing.includes('已生效'),
+     'billing 到期仍为 无固定到期（未订阅），未谎报生效（D2 frozen）');


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:18-25 ───
[maintainability · low] R3-17 移除 runs/ 回退后残留死代码：`import glob` 与 `RUNS`
在本文件中除声明/赋值外无任何引用（回退路径已整体删除，无归档时直接 sys.exit(2)）。建议一并删除，避免后续维护者误以为仍存在 runs/ 探测路径。注：同族的
flow-evidence-74、ocr2-replay、ocr3-replay 副本携带同样残留。

- import glob
  import json
  import re
  import sys
  from pathlib import Path
  
  EVID = Path(__file__).resolve().parent
- RUNS = Path(__file__).resolve().parents[3] / "deploy/lago-lab/payment-activation/runs"


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:11-16 ───
[documentation · low] docstring 与实际行为矛盾：此处称 "the runs/ observer TSV is only a fallback when no
archive exists"，但按 R3-17 修复（第 33-43 行），无归档时脚本直接以 MISSING-EVIDENCE（exit 2）退出，不存在任何 runs/ 回退路径；第 26-31
行块注释末尾 "Fall back to the newest runs/ TSV only when no archive exists, and always print the file
actually used" 同样描述的是已删除的旧行为。会误导后续维护者以为存在回退语义，建议同步修正为"无归档即降级为 MISSING-EVIDENCE，绝不回退"。



─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:0-0 ───
[bug · low] 标签错误：此检查位于 AC1（gate）的 invoice-visible 分支内，但断言名称写成了 "AC2 <=1 non-succeeded
payment"。对照原始脚本 docs/plans/issue-72-flow-evidence-74/verify_ac_assertions.py 第 52 行，此处应为 "AC1
..."。一旦该断言失败，FAIL 列表和 RESULT 行会以 "AC2" 名称误导排查方向；同时这也与文件 docstring 中 "ocr-1 replay copy: unmodified
script" 的声明矛盾（复制时实际引入了改动）。

-     check("AC2 <=1 non-succeeded payment",
+     check("AC1 <=1 non-succeeded payment",
            o.get("payments_non_succeeded_count", 99) <= 1)

