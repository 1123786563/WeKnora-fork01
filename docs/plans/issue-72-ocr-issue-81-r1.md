Review complete: 35 finding(s) across 84 selected item(s).

─── docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py:4-8 ───
[security · medium] 已核实本 stub 的集成前提:WEKNORA_WECHAT_API_BASE_URL 在 providers_env.go:144
被原样读入,WechatProvider.do()(wechat.go:515)使用普通 http.Client,且 internal/modules/commercial/ 全树没有任何
ValidateURLForSSRF/NewSSRFSafeHTTPClient 调用——因此指向 127.0.0.1:8291
无需任何豁免即可被服务端调用。这与本任务声明的安全验收条件『服务端发起外部请求前校验 host 并拒绝 localhost/环回/私有/保留地址』不符:代码库其他出站路径(OSS/S3/webhook
等)均走 ValidateURLForSSRF 并以 SSRF_WHITELIST 作为显式豁免机制,而渠道出站路径完全没有。建议渠道出站统一接入 SSRF-safe
client,使本证据流程依赖显式、可审计的 SSRF_WHITELIST=127.0.0.1 豁免,并在 stub docstring/证据文档中记录该豁免,避免证据默认建立在未校验的出站路径上。

- The real WechatProvider adapter (internal/modules/commercial/payment/wechat.go)
- is configured against this stub via WEKNORA_WECHAT_API_BASE_URL so the
- channel-availability PRE-check (purchase.go review F1) passes and the full
- purchase chain (gated subscription -> match gate -> channel order) can run
  end-to-end without real channel credentials.
+ 
+ Note: the channel outbound path currently performs no host validation on
+ WEKNORA_WECHAT_API_BASE_URL. Once the SSRF acceptance criterion (scheme +
+ host check rejecting loopback/private/reserved addresses) is wired into the
+ adapter, this evidence run additionally requires an explicit, auditable
+ SSRF_WHITELIST=127.0.0.1 entry instead of relying on the validation gap.


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:60-63 ───
[bug · medium] max_succeeded() 遍历全部采样行，既不按 run_prefix 过滤也不限定 mid-run 边界，而 TSV 中 payments[...]
段是观察者的全库聚合（db_watch.sh: SELECT status, count(*) FROM payments GROUP BY status，不按 external_id
键控）。脚本自身明确支持回退到 runs/ 下字典序最新的 db-watch-verify-*.tsv，且 run_prefix 处注释承认该 TSV 可能是含旧 replay
残留的多运行文件：此时外来 run 的 succeeded 计数会抬高 max_succeeded()，使本次运行被误判为 CHECK 失败，与 rows_for()
的隔离语义（及注释声称的多运行防串扰设计）不对称。建议与 rows_for 对齐，跳过既非中段、也不含本 run 前缀的行（注：因 payments 为全局聚合，行级过滤只能剔除外来 run
独占的采样行，共享脏库下的彻底隔离需观察者按 run 键控，可作为后续改进）。

  def max_succeeded():
      best = 0
      for ln in lines:
+         if not midrun(ln) or run_prefix not in ln:
+             continue  # align with rows_for(): skip tail and foreign-run lines
          m = re.search(r"payments\[([^\]]*)\]", ln)


─── apps/web/src/commercial/CheckoutPage.tsx:83-83 ───
[bug · high] purchase.order 缺席时以空字符串回退 getOrder，必然产生非法请求。后端已确认存在多个 err==nil 但不带 order
的成功分支（platform==nil → {state:'absent', reason:'unconfigured'}；账户未链接；SubmitCommand/ReadSnapshot 失败走
pendingView → {state:'absent', reason:'unreachable'|'invalid_response'|...}，均以 201 {success:true}
返回）。此时前端会 GET /api/v1/commercial/orders/（encodeURIComponent('') 为空），得到 404 或路由错配的原始报错，闭合的 reason
词汇完全丢失，用户看不到「渠道未配置/平台不可达」等真实原因，购买失败分支没有可用 UI。建议按 purchase.state/reason 分支渲染专门的失败文案（而非空 ID 查询）。

-         const order = purchase.order ?? (await client.commercial.getOrder('', currentScope.signal));
+         if (!purchase.order) {
+           // 渠道/平台未确认（state absent + 闭合 reason token）：呈现失败文案，而非空订单号查询。
+           if (active && scopeController.isCurrent(currentScope.scope)) {
+             setState({ status: 'error', message: purchaseErrorMessage(purchase) });
+           }
+           return;
+         }
+         const order = purchase.order;


─── apps/web/src/commercial/CheckoutPage.tsx:151-160 ───
[bug · medium] 「待付款（权益未开放）」与「前往支付」链接在 ready 态无条件渲染，与订单真实状态脱节：(1) 本页带已有 orderId 进入（付款后返回）时
order.payment 可能已是 paid/fulfilled；(2) 轮询/手动「刷新订单状态」把 order 更新为已支付后，页面仍显示待付款并保留有效支付入口，误导用户重复支付。应依据
order.payment === 'pending'（或 !isTerminal）条件渲染状态标签与 checkout_url 链接。

- {/* AC3：购买成功后产品状态为「待付款」，付费权益未开通。 */}
+               {/* AC3：购买成功后产品状态为「待付款」，付费权益未开通。 */}
-               <p>
-                 <Status>待付款（权益未开通）</Status>
-               </p>
-               {/* 渠道支付入口（审查 F2）：渠道请求创建后展示跳转链接，用户由此完成支付。 */}
-               {state.order.checkout_url ? (
+               {state.order.payment === 'pending' && !isTerminal(state.order) ? (
                  <p>
-                   <a href={state.order.checkout_url} target="_blank" rel="noreferrer">前往支付</a>
+                   <Status>待付款（权益未开放）</Status>
                  </p>
                ) : null}


─── apps/web/src/commercial/CheckoutPage.tsx:158-158 ───
[security · medium] checkout_url 直接渲染为 <a href>，前端无任何 scheme
校验。该值源自外部支付渠道响应（payment.AttemptResult.CheckoutURL，如微信 code_url），经 orderWire → parseOrderView（整对象透传，无
URL 校验）一路到达渲染层；若渠道响应或订单数据被污染为 javascript:/data: 等危险 scheme，将成为 XSS/钓鱼跳转入口。后端 S1 的
validateOutboundHost 只约束服务端出站请求，不覆盖这条入站展示链路。建议在渲染前（或 parseOrderView 内）做 scheme 白名单校验——注意微信 code_url
为 weixin:// 深链，白名单需含 http/https 及已知渠道深链 scheme。

-                   <a href={state.order.checkout_url} target="_blank" rel="noreferrer">前往支付</a>
+                   <a href={safeCheckoutHref(state.order.checkout_url)} target="_blank" rel="noreferrer">前往支付</a>
+ 
+ // safeCheckoutHref：URL 解析失败或 protocol 不在白名单（http:/https:/weixin:/alipay:）时返回 null，链接不渲染。


─── apps/web/src/commercial/CheckoutPage.tsx:183-183 ───
[maintainability · low] new Date(state.quote.expires_at).toLocaleString()：parseQuoteView 仅校验
expires_at 为非空字符串，未校验可解析为日期；异常值会渲染出「Invalid Date」，且未固定 locale 在不同环境下格式漂移。建议解析失败时降级显示原始字符串或固定格式。

-                 <p className="wk-muted">报价有效期至：{new Date(state.quote.expires_at).toLocaleString()}</p>
+                 <p className="wk-muted">报价有效期至：{formatExpiry(state.quote.expires_at)}</p>
+ 
+ // formatExpiry：const d = new Date(iso); Number.isNaN(d.getTime()) ? iso : d.toLocaleString('zh-CN');


─── packages/api-client/src/commercial.ts:66-66 ───
[maintainability · low] purchase 的入参使用内联对象类型，与同文件 createOrder
使用契约层具名类型（CreateOrderInput）的风格不一致；'wechat' | 'alipay' 联合类型现已在 CreateOrderInput、此处内联类型、CheckoutPage
调用处重复三份，后续新增渠道需多点修改。建议在 contracts 中定义并导出 CreatePurchaseInput（如 { quote_id: string; provider:
'wechat' | 'alipay' }）保持契约对称。

-     async purchase(input: { quote_id: string; provider: 'wechat' | 'alipay' }, signal?: AbortSignal): Promise<PurchaseView> {
+     async purchase(input: CreatePurchaseInput, signal?: AbortSignal): Promise<PurchaseView> {


─── apps/web/src/commercial/BillingPage.tsx:79-80 ───
[bug · low] effect 开头重置了 state/usageState（第 71-72 行）但没有重置 purchase：切换租户或点 Reload 后，purchase 仍持有上一租户的
PurchaseView，若上一个租户处于 awaiting_payment 而新租户的 summary 先返回、purchaseStatus 后返回，新租户的套餐行会短暂错误显示「·
待付款（权益未开放）」。建议与其他两个状态同步重置。

+     setState({ status: 'error', message: 'Loading…' });
+     setUsageState({ status: 'error', message: 'Loading…' });
+     setPurchase(null);
      // #81：并行读取购买状态；失败静默降级（待付款行只是缺席，不阻塞账单页）。
      void client.commercial.purchaseStatus(scope.signal).then((view) => {


─── apps/web/src/commercial/CheckoutPage.tsx:76-77 ───
[documentation · low] 本次改动删除了客户端幂等键（idempotencyKeyRef），重试改为后端按身份幂等，但错误态的重试按钮文案（第 143
行）仍写「重试（复用原订单与幂等键，不重复下单）」，向用户描述了一个已不存在的前端机制，易造成误导。建议同步更新为「重试（同一报价按身份幂等，不重复下单）」之类的表述。



─── deploy/lago-lab/payment-activation/clients.py:400-404 ───
[maintainability · low] `last_error` 是死存储:传输重试耗尽时通过裸 `raise` 重抛当前异常,循环结束后(仅在成功 `break`
时到达)该变量从未被读取。可直接删除 `last_error = None` 与 `last_error = error` 两处赋值,避免读者误以为循环外还有使用点。

-             except (URLError, OSError) as error:
-                 last_error = error
+             except (URLError, OSError):
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:215-218 ───
[bug · medium] 对已存在客户（onboarding 时 ensureCustomer 已以真实 DisplayName 创建，lago.go:143-144 注释确认 Lago 对
external_id 是 create/upsert 语义）走到这里时 GET 返回 200 但未绑定，随后这个 POST 会把权威侧客户的 name 覆写为裸 external id（如
"weknora-tenant-42"），静默降级账单权威里的客户显示名；而若权威实际语义是 create-only（重复 external_id 返回 422
value_already_exist），则落入 default 分支的 "provider binding rejected"
永久失败，所有既有租户都无法发起购买。customerProviderBound 的 GET 已能区分 404（不存在）与 200（已存在）：不存在时 POST 创建（携带 name），已存在时应改用
PUT /api/v1/customers/{external_id} 只更新 billing_configuration，两种语义下均安全。

- 	body := map[string]any{
- 		"customer": map[string]any{
- 			"external_id": externalCustomerID,
- 			"name":        externalCustomerID,
+ 	// customerProviderBound 已经区分了 404（客户不存在）与 200（已存在未绑定）：
+ 	// 已存在客户应使用 PUT /api/v1/customers/{external_id} 仅更新
+ 	// billing_configuration（避免 upsert 覆写既有 name）；不存在时才 POST
+ 	// 创建并携带 name。


─── internal/modules/commercial/commercialplatform/lago_purchase.go:476-478 ───
[bug · medium] providerAttachDefaultPaymentMethod 用 context.Background() 派生全新超时，脱离了
createPurchaseSubscription 设置的 purchaseRequestTimeout（15+10=25s）截止与取消链。唯一调用点 providerCreateCustomer
在请求作用域内且明明持有 ctx：调用方取消或超时后，attach 的两次外呼（各 15s）仍会继续执行，整体耗时可超出命令预算最多约 30 秒。应接收并继承调用方 ctx，由
WithTimeout(ctx, ...) 收敛到较短者。

- func (a *LagoAdapter) providerAttachDefaultPaymentMethod(providerCustomerID, pmToken string) error {
- 	ctx, cancel := context.WithTimeout(context.Background(), outboundProviderTimeout)
+ func (a *LagoAdapter) providerAttachDefaultPaymentMethod(ctx context.Context, providerCustomerID, pmToken string) error {
+ 	ctx, cancel := context.WithTimeout(ctx, outboundProviderTimeout)
  	defer cancel()
+ // 调用点：a.providerAttachDefaultPaymentMethod(ctx, parsed.ID, a.cfg.StripePmToken)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:462-462 ───
[security · medium] 此处构造的 http.Client 未设置 CheckRedirect，默认会跟随最多 10 次重定向；validateOutboundHost
仅在请求构建前校验初始 base URL，30x 的 Location 目标不再复检。注释声称 "a hostile configuration can never point the egress
at internal infrastructure" 与 S1 "请求前校验 host" 的验收口径可被重定向绕过：被配置的（或被劫持的）端点 302 跳至内网/元数据地址时请求仍会发出（Go
stdlib 跨域重定向会剥掉 Authorization，凭据不泄露，但 egress 可达性构成 SSRF 面）。应设置 CheckRedirect 对每个跳转目标重新执行
validateOutboundHost（非 2xx/3xx 语义），失败即拒绝。

- 	resp, err := (&http.Client{Timeout: outboundProviderTimeout}).Do(req)
+ 	client := &http.Client{
+ 		Timeout: outboundProviderTimeout,
+ 		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
+ 			return validateOutboundHost(req.URL.String())
+ 		},
+ 	}
+ 	resp, err := client.Do(req)


─── internal/modules/commercial/commercialplatform/config.go:42-43 ───
[maintainability · low] 通过拆分字符串拼接（"WEKNORA_COMMERCIAL_STRIPE_API" +
"_KEY"）规避密钥扫描器对环境变量名的模式匹配，注释明言目的是 defuse scanners——这会反过来削弱安全扫描覆盖（扫描器按完整 env
名匹配时将漏检真实使用点），降低可检索性（grep 完整变量名查不到定义），且与未拆分的 EnvProviderCustomerPrefix
风格不一致。环境变量名本身不是凭据，混淆它对防御无增益；建议直接使用完整字面量，让扫描器正常覆盖。

- 	EnvStripeKey              = "WEKNORA_COMMERCIAL_STRIPE_API" + "_KEY"
- 	EnvStripeAPIBase          = "WEKNORA_COMMERCIAL_STRIPE_API" + "_BASE"
+ 	EnvStripeKey     = "WEKNORA_COMMERCIAL_STRIPE_API_KEY"
+ 	EnvStripeAPIBase = "WEKNORA_COMMERCIAL_STRIPE_API_BASE"


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:107-107 ───
[other · low] 诊断行的样本计数与括注语义不符：`len(lines)` 统计的是 TSV 全部行（含 decline 边界后的尾段样本），而标签 "(mid-run before
…Z)" 让读者以为这是商业窗口内的采样数。本目录归档恰好全部落在边界前（272/272）未暴露差异，但同族重放目录（issue-72-ocr2-replay）的归档为 516 行总量 vs 319
个 mid-run 样本，该打印会系统性虚报采样点数，削弱 DB-WATCH 输出与归档证据的对账性。建议改为统计 mid-run 行数（或同时给出两个数字）。

- print(f"samples: {len(lines)} (mid-run before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")
+ midrun_count = sum(1 for ln in lines if midrun(ln))
+ print(f"samples: {midrun_count}/{len(lines)} mid-run (boundary {boundary}Z)  max_succeeded_payments: {max_succeeded()}")


─── internal/modules/commercial/service/commercial/purchase.go:232-235 ───
[bug · high] PurchaseStatus 把 ListOrdersByTenant 的 rows[0] 直接当作当前购买的 order 投影,但该查询既不按 quote/Kind
关联当前购买,排序也不可靠:订单 ID 是 "ord_"+随机 8 字节 hex(newLeaseToken),`Order("id DESC")` 的字典序是随机的(仓储注释"newest
first"实际不成立)。租户一旦存在多张订单(遗留 POST /orders 购买订单、ChangePlan 的 upgrade 订单、历史购买),这个支付面向的 GET 会把一张任意的旧订单(其
state/amount_fen/order id)作为当前待付款购买的订单返回,误导客户端的支付状态判断与订单恢复指向。建议仅关联与当前购买对应的订单(例如购买快照/门控 invoice 携带的
quote 关联,或至少过滤 Kind==purchase 且 state==pending 并按确定性字段排序),而不是取任意 rows[0]。

- 	if rows, err := s.orders.orders.ListOrdersByTenant(ctx, tenantID); err == nil && len(rows) > 0 {
- 		ov := orderViewFromRow(rows[0])
+ 	// 仅关联与当前购买对应的订单,而不是任意一张历史订单
+ 	if ov, ok := s.purchaseOrderFor(ctx, tenantID, p); ok {
  		out.Order = &ov
  	}


─── internal/modules/commercial/service/commercial/purchase.go:271-273 ───
[bug · medium] 平台故障(SubmitCommand/ReadSnapshot 失败)时 Purchase 返回 nil error + 此视图,handler 的 err==nil
分支以 201 Created + success:true 应答——一次什么都没创建的 POST 得到"已创建"语义,仅以 state=absent+reason 区分;而 absent 在
purchase_command.go 的域定义是"authority definitively holds no purchase",故障期间借用该令牌等于给出一个貌似确定性的否定,与 #78
用独立 pending 状态表达故障的口径不一致。参考客户端已因此误入破损回退(CheckoutPage 对 purchase.order 为空时调用
getOrder(''))。建议平台故障走显式失败信号(如返回由 handler 映射 503+闭合 reason 的错误,对齐 ErrPaymentProviderUnconfigured
的处理),或为故障引入独立的 pending/unknown 状态令牌,不占用 absent 的确定性语义。



─── internal/modules/commercial/service/commercial/purchase.go:129-131 ───
[bug · low] platformReason(errors.New(acct.Reason)) 把已是闭合令牌的字符串包成新 error 再做 errors.Is 哨兵匹配,任何新构造的
error 都不匹配哨兵,恒落入默认分支返回 "unsupported"。即计费账户 pending
的真实原因(unconfigured/unreachable/invalid_response)在该路径上永远被改写为 "unsupported",闭合 reason 诊断失真。acct.Reason
本身就是闭合令牌(billing_account.go 只赋 platformReason 的输出或 "unconfigured"),直接透传即可。

  	if acct.State != BillingAccountLinked {
- 		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: platformReason(errors.New(acct.Reason))}, nil
+ 		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: acct.Reason}, nil
  	}


─── internal/handler/commercial.go:438-438 ───
[bug · low] actor 硬编码为 "billing-admin",偏离本文件既有约定:所有审计/ensure 调用点(AccountStatus 的
EnsureBillingAccount、CreateDraft、refunds 等)一律传 commercialUserID(c)(认证用户)。合成管理员使购买与计费账户 ensure
的审计轨迹无法归因到真实调用者。同时 PurchaseService 内 EnsureBillingAccount 的 display name 也硬编码为 "space",而其他生产调用点传
tenantDisplayName(tenantID)——首次经购买路径 ensure 的账户会在权威侧落一个错误名称。建议 actor 取自认证上下文,display name 由 handler
传入。

- 	view, err := h.purchases.Purchase(c.Request.Context(), tenantID, req.QuoteID, req.Provider, "billing-admin")
+ 	view, err := h.purchases.Purchase(c.Request.Context(), tenantID, req.QuoteID, req.Provider, commercialUserID(c))


─── internal/modules/commercial/service/commercial/purchase.go:246-249 ───
[bug · low] DefinitionJSON 反序列化失败被映射为 ErrQuoteLegacySnapshot,handler 应答 409 "quote predates the
purchase freeze; please re-quote"。但此处的 JSON 是计划版本定义(发布时由 domain.PlanVersion
序列化),损坏/格式不符与"报价早于冻结"无关:用户被引导反复重新报价,而同一损坏定义每次都会再次失败,真实的数据问题被误导性哨兵掩盖。建议引入独立哨兵(如
purchase_plan_invalid,映射 5xx/明确的不可购买语义)而非复用 legacy-snapshot 语义。



─── apps/web/src/commercial/CheckoutPage.tsx:152-154 ───
[bug · low] 嵌套段落是无效 HTML：@weknora/ui 的 Status 组件本身渲染的就是 <p role="status">（packages/ui/src/index.tsx
中 Status 的实现），外面再包一层 <p> 会产生 <p> 嵌套 <p>。React 开发态会触发 validateDOMNesting 警告；一旦该标记经 SSR/hydration
或字符串序列化输出，浏览器解析器会提前闭合外层 <p>，造成实际 DOM 与 React 树不一致（hydration mismatch）。建议去掉外层 <p>，直接让 Status 承载该行文本。

-               <p>
-                 <Status>待付款（权益未开通）</Status>
+               <Status>待付款（权益未开通）</Status>
-               </p>


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:64-67 ───
[maintainability · medium] max_succeeded() 与 rows_for() 的隔离级别不一致：rows_for() 已按脚本自身注释的威胁模型（runs/
回退取到外来运行的 TSV、共享 lab DB 残留早前回放数据）加上 run_prefix + midrun 双重过滤，但 max_succeeded()
既不受中段边界限制也无法做任何运行隔离——观察者对 payments 的采样是全库聚合（db_watch.sh: SELECT status, count(*) FROM payments GROUP
BY status，不按 external_id 键控，已由 TSV 数据印证：decline 后 failed 计数跨客户累计为 2）。因此走 runs/ 回退路径（本目录无归档 TSV
时取字典序最新的 db-watch-verify-*.tsv，可能属于另一次运行）或 DB 中残留早前回放的 succeeded 付款时，外来计数会直接注入 `max_succeeded() ==
1` 这一 exactly-once 不变量：残留付款会使本目录的干净重放误报 CHECK，回退路径还会把外来 TSV 与本目录的 decline 边界错配。建议至少补上与 rows_for 相同的
only_midrun 过滤保持一致；对无法按 run 前缀隔离的全局计数，在回退到外来 runs/ TSV 时对该不变量显式告警或降级（如 exit 2
MISSING-EVIDENCE），避免用不可隔离的数据源支撑结论。注：docs/plans/issue-72-ocr-round-4.md 第 185 行起已记录同一问题，本副本仍未修复。

- def max_succeeded():
+ def max_succeeded(only_midrun=True):
      best = 0
      for ln in lines:
+         if only_midrun and not midrun(ln):
+             continue
          m = re.search(r"payments\[([^\]]*)\]", ln)


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:26-26 ───
[documentation · low] 证据来源描述与实际取数路径不符：本目录实际存在 verify-db-watch-samples.tsv（归档优先分支可达，运行时会打印 using
.../verify-db-watch-samples.tsv），但模块 docstring 声称 "executed against ... and the runs/ observer TSV
of the replay"；同时内联注释 "(ocr-2)" 在本 ocr-1 副本中指代另一个 replay 目录，读者会误以为归档优先策略属于/指向 ocr-2
目录的文件。这是证据审计工具，来源描述失实会误导后续复核者对证据链的判断，建议删除交叉目录指代、并把 docstring 改为如实描述"优先本目录归档 TSV，runs/
仅为回退"。（docs/plans/issue-72-ocr-round-4.md 第 131 行起已记录同一问题，本副本仍未修复。）

- # Prefer THIS evidence directory's archived copy (ocr-2): replayed observer
+ # Prefer THIS evidence directory's archived copy: replayed observer


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:64-69 ───
[bug · medium] max_succeeded() 与 rows_for() 的隔离级别不一致：rows_for() 通过 run_prefix 过滤并在注释中明确防御"多 run TSV
/ 旧回放残留"（本脚本第 28-35 行自己声明的威胁场景），且只统计 midrun 边界前的样本；但 max_succeeded() 既不受 midrun
边界限制，也没有任何运行隔离——而观察者对 payments 的采样是全库状态聚合（db_watch.sh: SELECT status, count(*) FROM payments GROUP
BY status，不按 external_id 键控）。一旦归档缺失走 runs/ 回退路径（本脚本明确支持的分支）取到混有其他 replay 样本的多 run TSV，外来 run 的
succeeded 计数会泄漏进 `max_succeeded() == 1` 这条 exactly-once 不变量，导致误报 CHECK。当前本目录归档的
verify-db-watch-samples.tsv 仅含单一 run（已核实全部行属 f6bebb06），当前执行不受影响，仍建议补上与 rows_for 一致的 midrun
过滤；由于全局计数无法按 run 前缀键控，走外来 runs/ TSV 回退时宜对该不变量显式告警或降级（如 exit 2 MISSING-EVIDENCE），避免用不可隔离的数据源支撑结论。

- def max_succeeded():
+ def max_succeeded(only_midrun=True):
      best = 0
      for ln in lines:
+         if only_midrun and not midrun(ln):
+             continue
          m = re.search(r"payments\[([^\]]*)\]", ln)
          if not m:
              continue


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:10-11 ───
[documentation · low] docstring 声称 "unmodified assertion script"，但与 ocr-1
副本（docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py，87 行）对比，本副本（97 行）新增了两处带 "# ocr-2:"
标注的断言块：AC2 的 deferred re-check 检查（dr.get("checked") is True and dr.get("ok") is True）与 AC3 的 gate
重试探针 not_applicable 检查。来源声明与文件实际内容自相矛盾，会误导后续审计者对各轮回放断言集可比性的判断（新增断言读取的 deferred_recheck /
pending_gate_retry 字段已确认存在于本目录 t02-duplicates.json 与 t02-retries.json，运行时不致
FAIL，问题仅在声明失实）。建议改为如实描述相对前轮的增量。

- ocr-2 replay copy: unmodified assertion script, executed against the
- docs/plans/issue-72-ocr2-replay/ evidence directory.
+ ocr-2 replay copy: extends the ocr-1 assertion script with the AC2
+ duplicates deferred re-check and the AC3 gate-retry not_applicable probe;
+ executed against the docs/plans/issue-72-ocr2-replay/ evidence directory.


─── deploy/lago-lab/payment-activation/phases.py:1039-1041 ───
[test · medium] 延迟复检(deferred re-check)的等待窗口与它要防护的漂移时间尺度不匹配:代码注释和 expected 文本都明确说明该守卫针对的是 "200
应答的重复注册在数分钟(minutes)后终止订阅并开具新发票" 的 ocr-2 场景,但这里的等待是 `stability_rounds * stability_delay`,按
run_lab.py 的 CLI 默认值(3 × 5.0s)只有 15 秒(RunContext 构造默认也仅 3 × 3.0s = 9 秒)。15
秒后的单次复查几乎不可能观察到数分钟后的延迟漂移,导致该守卫在默认配置下基本失效,phase 仍会 PASS——这与 expected 中 "deferred re-check must stay
clean or the phase fails" 的承诺不符。建议为该复检引入独立的、分钟级的旋钮(例如 --duplicates-settle-delay,默认
>=120s)而不是复用稳定性窗口参数,或在文档中明确该窗口的上限语义。

          if probes_harmless and state_intact:
-             time.sleep(ctx.stability_rounds * ctx.stability_delay)
+             # 独立的分钟级 settle 窗口:ocr-2 观察到的漂移发生在响应数分钟后,
+             # 15s 级的 stability 窗口看不到它。
+             time.sleep(ctx.duplicates_settle_delay)  # 新增 RunContext 旋钮,默认 >=120s
              _, sub_body3 = _subscription_show(ctx, sub_ext, "active")


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:64-73 ───
[bug · low] max_succeeded() 是全脚本唯一不做 run 隔离的断言：它遍历全部样本行解析 payments[...]，既不按 run_prefix 过滤也不受 midrun
边界约束。而 TSV 中 payments[...] 是数据库全局聚合（如 failed|2;requires_action|2;succeeded|1），本身不携带 run 标识，只有
rows[...] 才带 weknora-t02-<run_id>- 前缀。脚本注释自己声明的威胁模型——回退路径下 runs/ 中的 "a multi-run TSV (an older
replay's leftovers)"——正是 rows_for() 为 sub-a/b/c 加 run_prefix 过滤的原因，但外来 run 残留的 succeeded|2+ 样本行同样会被
max_succeeded() 计入，导致 max_succeeded()==1 误判为 CHECK 失败（exit 1），与脚本声称的 run 隔离目标不一致。影响方向是误报（false
fail）而非漏报，且归档 TSV 存在的主路径不受影响，故 severity 定为 low。建议在统计前跳过 rows[...] 中不含本 run 前缀的行（并可同时应用 midrun 边界）：

  def max_succeeded():
      best = 0
      for ln in lines:
+         # only THIS run's samples count: rows[] carries the run prefix, so
+         # foreign-run rows in a mixed runs/ TSV cannot skew the verdict
+         if not midrun(ln) or run_prefix not in ln:
+             continue
          m = re.search(r"payments\[([^\]]*)\]", ln)
          if not m:
              continue
          for part in m.group(1).split(";"):
              if part.startswith("succeeded|"):
                  best = max(best, int(part.split("|")[1]))
      return best


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:10-11 ───
[documentation · low] docstring 声称 "ocr-3 replay copy: unmodified assertion script"，但本文件相对 ocr-2
副本（97 行）新增了两处明确标注 "# ocr-3:" 的断言（"AC2 duplicates invoice count == baseline" 与 "AC3 gate end state
recorded"，共 105 行），同一文件内 docstring 的"未修改"声明与代码注释的"ocr-3 修改"标注自相矛盾，会误导后续审阅者对脚本来源与变更范围的判断（例如让人以为本目录与
ocr-2 的断言强度完全一致）。建议改为如实描述相对上游版本新增的断言：

- ocr-3 replay copy: unmodified assertion script, executed against the
+ ocr-3 replay copy: the ocr-2 script plus two added assertions (AC2
+ deferred no-new-invoice count anchored on the pre-probe baseline; AC3
+ gate end state recorded), executed against the
  docs/plans/issue-72-ocr3-replay/ evidence directory.


─── internal/modules/commercial/commercialplatform/lago_purchase.go:240-240 ───
[bug · medium] 占位前缀绑定路径会确定性地卡死在这段轮询上：deriveProviderCustomerID 在没有 StripeAPIKey 时返回
ProviderCustomerPrefix 拼出的 provider_customer_id，该 id 在 provider 侧不存在任何真实客户；而 t02 lab 证据（phases.py 的
provider_setup：先 stripe.create_customer + attach + set_default，再轮询 /payment_methods）表明 Lago
的支付方式导入只对真实 provider 客户发生。于是 ensureProviderBinding 绑定 upsert 成功后无条件进入
waitForPaymentMethodSync，payment_methods 列表永远为空，prefix-only 的 dev/test 栈（config.go 注释明确宣称此路径用于
exercise the binding path）每次 create_purchase_subscription 都会空转满 pmSyncWait（且会被 25s 的
purchaseRequestTimeout 截断）后失败，错误还落在"瞬时"哨兵 ErrPlatformUnreachable
上，把永久性配置条件误分类为可重试的暂时故障。建议：deriveProviderCustomerID 返回绑定来源（真实 key / 占位前缀），占位来源时跳过 PM 同步等待（让 gated
create 用其自身明确的 422 no_default_payment_method 快速失败），或直接 fail closed 返回 ErrPlatformUnconfigured。

+ 		if placeholderBinding {
+ 			// 占位绑定背后没有真实 provider 客户，PM 永远不会被导入；
+ 			// 跳过轮询，让 gated create 以其自身的 422 no_default_payment_method
+ 			// 快速失败（或在此直接 fail closed 返回 ErrPlatformUnconfigured）。
+ 			return nil
+ 		}
  		return a.waitForPaymentMethodSync(ctx, externalCustomerID)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:85-91 ───
[security · medium] validateOutboundHost 的保留地址拒绝可被 inet_aton 简写形式绕过：net.ParseIP 只接受全写点分 IPv4 与标准
IPv6 字面量，"127.1"、"2130706433"、"0x7f000001"（以及带 %zone 的 IPv6）都会解析失败而落入"命名主机放行"分支；但在默认 cgo 构建下，拨号时
getaddrinfo 会把这些简写解析为环回/私有地址（127.1→127.0.0.1、2130706433→127.0.0.1），请求仍会打到内网。这与注释声称的 "a hostile
configuration can never point the egress at internal infrastructure" 以及 S1 "请求前校验 host 并拒绝
localhost、环回、私有和保留地址" 的验收口径不符（注：纯 Go resolver 构建下这些名字会走 DNS 而失败，故可利用性取决于构建方式，但仍属加固缺口）。建议：ParseIP
失败后额外拒绝由纯数字/十六进制/点/冒号构成且不像合法 FQDN 的主机名，或在放行前解析域名并对结果 IP 复检同样的保留段规则。

  	ip := net.ParseIP(host)
  	if ip == nil {
- 		// A named host is allowed (SSRF hardening stops IP literals and
- 		// reserved names; a DNS name that later resolves internal is out of
- 		// scope for this fixed-configuration egress).
+ 		// inet_aton 简写（"127.1"、"2130706433"、"0x7f000001"、带 zone 的 IPv6）
+ 		// 不是 ParseIP 认可的字面量，但系统解析器可能将其解析为环回/私有地址，
+ 		// 必须一并拒绝。
+ 		if isAmbiguousAddressShorthand(host) {
+ 			return fmt.Errorf("outbound host %q not allowed", host)
+ 		}
  		return nil
  	}


─── internal/modules/commercial/commercialplatform/lago_purchase.go:487-489 ───
[bug · low] attach 与 default-payment-method 更新两个分支把所有非 2xx（含 5xx）都折叠为
ErrPlatformInvalidResponse，违反适配器自身的 fail-closed 契约（lago.go:62-68：传输失败/超时/5xx 包装
ErrPlatformUnreachable，其余 4xx 才是 InvalidResponse）。与同文件 providerCreateCustomer 对 5xx→Unreachable
的分类也不一致：Stripe 瞬时故障（如 503）会被误判为终态失败，依赖哨兵区分"可重试/终态"的调用方（outbox
重试策略）将停止重试。此函数的第二处外呼（/v1/customers/{id} 的 default 更新）同样存在该问题。

- 	if status < 200 || status >= 300 || json.Unmarshal(data, &attached) != nil || attached.ID == "" {
+ 	if status >= 500 {
+ 		return fmt.Errorf("%w: provider payment method attach unavailable", commercial.ErrPlatformUnreachable)
+ 	}
+ 	if status < 200 || json.Unmarshal(data, &attached) != nil || attached.ID == "" {
  		return fmt.Errorf("%w: provider payment method attach rejected", commercial.ErrPlatformInvalidResponse)
  	}


─── internal/modules/commercial/service/commercial/purchase.go:172-179 ───
[bug · high] 匹配门只拒绝 absent 状态,active/canceled 均会放行并走到第 8 步为新报价 CreateOrder
新开渠道支付订单,但此时并不存在待付的门控发票:lago_purchase.go 的 createPurchaseSubscription 对同 plan code 的已存在订阅(无论
incomplete/active/canceled)直接返回成功回执(F7 replay 不看状态),而 readPurchaseSnapshot 会把 Lago 的
active→active、canceled/terminated→canceled 原样带回。可达路径:(a) canceled——门控支付被拒/超时后订阅被取消(t02-decline
已验证),用户重新报价再提交,门控通过、新渠道订单开出、用户真实付款,但已取消订阅的门控发票永远无法结算/激活,等于无发票扣款;(b) active——#82/#83
落地后对已生效计划重新报价再提交,同样第二次扣款且无门控发票。建议:新开订单仅允许 p.State ==
PurchaseStateAwaitingPayment(上面的既有订单重放路径可对任意状态保持放行,以支持付款成功后的 POST 重放),非 awaiting 状态返回专用哨兵并由 handler
映射 409。

- 	p := psnap.Purchase
- 	if p == nil || p.State == domain.PurchaseStateAbsent ||
- 		p.PlanCode != pub.PlanCode ||
- 		p.Currency != domain.CurrencyCNY ||
- 		p.Currency != snap.Currency ||
- 		p.AmountFen != snap.PriceFen {
- 		return PurchaseView{}, ErrInvoiceQuoteMismatch
+ 	// 第 8 步新开订单前(既有订单重放之后)补充:
+ 	// 仅待付款的门控订阅才允许新开渠道订单,active/canceled 均无待付门控发票。
+ 	if p.State != domain.PurchaseStateAwaitingPayment {
+ 		return PurchaseView{}, ErrPurchaseNotAwaitingPayment // 新哨兵,handler 映射 409
  	}
+ 	ov, err := s.orders.CreateOrder(ctx, tenantID, quoteID, providerName)


─── internal/handler/commercial.go:439-442 ───
[bug · medium] Purchase handler 缺少 POST /orders(CreateOrder handler)已有的 CheckoutError 分支:底层调用同一个
s.orders.CreateOrder,当订单已原子持久化但渠道调用失败/超时,服务返回 nil error + Order.CheckoutError 非空的视图;orders
端点按既定产品契约答 202 Accepted(注释明确"客户端经 GET /commercial/orders/:id 恢复,而非重试已消费的报价"),而这里落入 err == nil 分支答
201 Created + success:true,把渠道失败的结账当成干净创建,兄弟端点对同一失败模式的语义分叉。建议镜像 202 分支。

  	switch {
+ 	case err == nil && view.Order != nil && view.Order.CheckoutError != "":
+ 		// 与 POST /orders 的产品契约对齐:订单已持久化但渠道调用失败时答 202,
+ 		// 客户端经 GET /commercial/orders/:id 恢复,而非重试已消费的报价。
+ 		c.JSON(http.StatusAccepted, gin.H{"success": true, "data": purchaseWire(view)})
  	case err == nil:
  		c.JSON(http.StatusCreated, gin.H{"success": true, "data": purchaseWire(view)})
- 	case errors.Is(err, commercialsvc.ErrInvoiceQuoteMismatch):


─── internal/handler/commercial.go:442-445 ───
[bug · low] Purchase handler 缺少 ErrQuoteAlreadyUsed 的 case:服务在输掉报价消费竞态且回读失败时会返回
repocommercial.ErrQuoteAlreadyUsed(purchase.go 第 196 行),这里落入 default 答 400 Bad Request;而 POST
/orders 对同一哨兵映射 409 "quote already used"。两兄弟端点对同一竞态结果的契约不一致,建议补齐 409 映射。

- 	case errors.Is(err, commercialsvc.ErrInvoiceQuoteMismatch):
- 		c.JSON(http.StatusConflict, gin.H{"error": "invoice_quote_mismatch"})
  	case errors.Is(err, repocommercial.ErrQuoteExpired):
  		c.JSON(http.StatusConflict, gin.H{"error": "quote expired"})
+ 	case errors.Is(err, repocommercial.ErrQuoteAlreadyUsed):
+ 		c.JSON(http.StatusConflict, gin.H{"error": "quote already used"})


─── internal/modules/commercial/service/commercial/purchase.go:82-87 ───
[bug · low] 与已确认的 pendingView 问题(平台故障时 201+absent)同类,但这是另一条独立代码路径:platform == nil(阻断环境)时 Purchase
在未做任何校验、未创建任何资源的情况下返回 nil error,handler 以 201 Created + success:true 应答,并把域定义上表示"权威侧确定无购买"的 absent
令牌用于静态未配置场景。若仅修复 pendingView 会漏掉此分支;同一环境下渠道级未配置(第 2b 步)走 ErrPaymentProviderUnconfigured→503,平台级未配置却答
201,口径也不一致。建议 POST 场景返回哨兵错误(映射 503),GET 场景(PurchaseStatus 的同型分支)保持状态视图即可。

- 	if tenantID == 0 || quoteID == "" {
- 		return PurchaseView{}, repocommercial.ErrInvalidQuoteRow
- 	}
  	if s.platform == nil {
- 		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: "unconfigured"}, nil
+ 		// POST 未创建任何资源,不应以 201 成功视图应答;与渠道级未配置(503)对齐。
+ 		return PurchaseView{}, fmt.Errorf("%w: platform", ErrPaymentProviderUnconfigured)
  	}


LLM retry report summary: 9 of 310 requests affected -- 9 requests recovered after retry

Review planning (1 request):
- deploy/lago-lab/payment-activation/clients.py,deploy/lago-lab/payment-activation/fixtures.py,deploy/lago-lab/payment-activation/phases.py,deploy/lago-lab/payment-activation/run_lab.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Core review (8 requests):
- apps/web/src/commercial/BillingPage.tsx,apps/web/src/commercial/CheckoutPage.tsx,packages/api-client/src/commercial.ts,packages/contracts/src/commercial.ts,packages/contracts/src/index.ts: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- docs/plans/issue-72-flow-evidence-74/t02-setup.json,docs/plans/issue-72-flow-evidence-74/verify_ac_assertions.py,docs/plans/issue-72-flow-evidence-74/verify_db_watch.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- docs/plans/issue-72-ocr1-replay/t02-activation.json,docs/plans/issue-72-ocr1-replay/t02-cleanup.json,docs/plans/issue-72-ocr1-replay/t02-decline.json,docs/plans/issue-72-ocr1-replay/t02-duplicates.json,docs/plans/issue-72-ocr1-replay/t02-environment.json,docs/plans/issue-72-ocr1-replay/t02-gating.json,docs/plans/issue-72-ocr1-replay/t02-manual.json,docs/plans/issue-72-ocr1-replay/t02-provider.json,docs/plans/issue-72-ocr1-replay/t02-retries.json,docs/plans/issue-72-ocr1-replay/t02-setup.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/container/container.go,internal/handler/commercial.go,internal/modules/commercial/purchase_command.go,internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go,internal/router/routes_commercial.go: rate limited (HTTP 429) -> succeeded
- internal/container/container.go,internal/handler/commercial.go,internal/modules/commercial/purchase_command.go,internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go,internal/router/routes_commercial.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- ... and 3 more

Per-attempt detail: --format json (retry_report).
