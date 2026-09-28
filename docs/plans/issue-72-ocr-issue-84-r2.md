Review complete: 121 finding(s) across 166 selected item(s).

─── apps/web/src/commercial/BillingPage.tsx:55-56 ───
[bug · medium] 错误分支把原始 error.message（ApiError 的英文文案，如 'Expected an account response object'、后端 500 的
'account status unavailable'）或英文兜底 'Unable to load credits breakdown' 直接写入 accountState.message
并渲染到用户可见的 Status 卡片，违反本 PR 自己在 CheckoutPage 建立并多处引用的 spec L210 约束（页面词汇稳定、不含平台词汇）。建议原始 message 仅进
console，用户面收敛为闭合中文文案。

      if (signal?.aborted) throw error;
-     return { status: 'error', message: error instanceof Error ? error.message : 'Unable to load credits breakdown' };
+     if (error instanceof Error) console.warn('billing account load failed:', error.message);
+     return { status: 'error', message: '余额信息加载失败，请稍后重试' };


─── apps/web/src/commercial/BillingPage.tsx:129-131 ───
[bug · low] loadCommercialAccount 在 signal.aborted 时 rethrow，而调用侧 void ...then(...) 未挂
.catch——租户切换/卸载时在途请求被 scope signal 中止会触发 unhandled promise rejection（控制台噪音、可能触发全局错误上报）。summary/usage
是既有的同款模式，但新代码不应再复制第三份；建议补一个吞掉 abort 的 catch（或 loader 对 abort 不 rethrow 而返回中性态）。

      void loadCommercialAccount(client.commercial, scope.signal).then((next) => {
        if (active && scopeController.isCurrent(scope.scope)) setAccountState(next);
-     });
+     }).catch(() => { /* abort on scope switch/unmount: no state write */ });


─── apps/web/src/commercial/BillingPage.tsx:61-65 ───
[bug · low] microToDisplay 用 Number(micro) 换算：micro 字符串超过 Number.MAX_SAFE_INTEGER（约
9.007×10^15，即显示值约 90 亿元额度）时会静默丢失精度，展示错误金额且无任何提示。契约层金额刻意用 digit string 保留精度，展示层却先坍缩为 double。建议用
BigInt 做除法（如 (BigInt(micro) / 10000n 逐位或 scale 后 format），或至少在超界时给出明确处理。

  export function microToDisplay(micro: string): string {
-   const n = Number(micro);
-   if (!Number.isFinite(n)) return micro;
-   return (n / 1_000_000).toFixed(2);
+   if (!/^-?\d+$/.test(micro)) return micro;
+   const v = BigInt(micro);
+   const sign = v < 0n ? '-' : '';
+   const abs = sign ? -v : v;
+   const whole = abs / 1_000_000n;
+   const frac = (abs % 1_000_000n).toString().padStart(6, '0').slice(0, 2);
+   return `${sign}${whole}.${frac}`;
  }


─── apps/web/src/commercial/BillingPage.tsx:79-80 ───
[maintainability · low] 「30 天近到期窗口」是业务口径数字，直接以 30 * 24 * 3600 * 1000
魔法表达式硬编码在前端，且未与后端任何到期提醒口径对齐说明（后端无对应常量）。按检查清单业务数字应提取命名常量；建议定义为模块级 EXPIRING_WINDOW_DAYS/MS
并注明口径来源，便于后续与后端提醒窗口统一调整。

+ const EXPIRING_WINDOW_MS = 30 * 24 * 3600 * 1000;
+ // ...
    const delta = exp - now.getTime();
-   return delta >= 0 && delta <= 30 * 24 * 3600 * 1000;
+   return delta >= 0 && delta <= EXPIRING_WINDOW_MS;


─── apps/web/src/commercial/BillingPage.tsx:178-178 ───
[maintainability · low] 「 · 付款异常（待处理）」后缀字面量在两个条件分支重复出现（fulfillment==='attention' 与
active+payment_attention 两处）。本 PR 的 D15-f 主旨正是把状态文案收敛到共享词表（PURCHASE_STATE_LABEL），建议同层提取一个共享常量（如
order-state.ts 中 export const PAYMENT_ATTENTION_SUFFIX = ' · 付款异常（待处理）'），避免词表之外的私有副本再度漂移。

-                 {purchase?.order?.fulfillment === 'attention' ? ' · 付款异常（待处理）' : ''}
+                 {purchase?.order?.fulfillment === 'attention' ? PAYMENT_ATTENTION_SUFFIX : ''}


─── apps/web/src/commercial/BillingPage.tsx:225-225 ───
[bug · low] projected_at 在契约解析中被显式降级为
''（OCR84-R1-04），但渲染层无条件输出「对账时间：{projected_at}」——投影无时间戳的账号会渲染出空尾巴「对账时间：」。同理批次行 granted_at 降级为 ''
时会渲染「充值批次 · 」的悬挂分隔符。建议对空值做条件渲染，与契约层「缺省即隐藏」的降级语义对齐。

-           <p className="wk-muted">对账时间：{accountState.credits.projected_at}</p>
+           {accountState.credits.projected_at
+             ? <p className="wk-muted">对账时间：{accountState.credits.projected_at}</p>
+             : null}


─── apps/web/src/commercial/CheckoutPage.tsx:366-369 ───
[bug · medium] 渠道切换重启按钮的承诺与后端语义不符：restartCheckout 走新报价新 purchase，但当租户已有一张带有效链接的 pending 订单时，后端
Purchase 命中 ErrPurchasePendingExists
不变量（internal/modules/commercial/service/commercial/purchase.go:347-416，(R2-26) 分支对新报价重提交明确「Answer
the winner's order verbatim — never a second channel request」），会把旧渠道（如微信）订单原样 201 返回，并不创建支付宝新单。随后
run() 中 setSubmittedChannel(channelRef.current) 把 submittedChannel 重置为支付宝，与 radio 一致，失配横幅条件 channel
!== submittedChannel 永假——结果是：用户点了「改用支付宝重新发起支付」，页面静默回到微信订单+微信支付链接，且原先的「当前订单以 微信支付
创建」警示消失，用户可能以为已切换渠道实则在旧渠道付款。注：无链接 pending 的重启（R3-09，走
SweepStaleLinklessPending）是成立的，此问题仅影响渠道失配场景。建议：purchase 返回后按订单实际 provider 回写 submittedChannel（wire
上有 provider 字段，parseOrderView 原样透传，可用与 payment_attention
相同的加法字段断言读取），让失配横幅如实重现，或在横幅文案中说明旧单仍有效需先完成/关闭。

                    <Status tone="error">当前订单以 {submittedChannel === 'alipay' ? '支付宝' : '微信支付'} 创建</Status>
                    <Button type="button" onClick={() => { restartCheckout(); }}>
                      改用{channel === 'alipay' ? '支付宝' : '微信支付'}重新发起支付（获取新报价）
                    </Button>
+                   {/* run() 中 purchase 成功后： */}
+                   {/* const provider = (purchase.order as OrderView & { provider?: 'alipay' | 'wechat' }).provider;
+                       if (provider) setSubmittedChannel(provider); */}


─── apps/web/src/commercial/CheckoutPage.tsx:345-345 ───
[style · low] fieldset 使用静态内联 style 对象（border/radius/margin/padding 均为固定值），违反「除动态样式外禁止内联
style」的规范。BillingPage 中已有 USAGE_TABLE 这类提取为常量的 tailwind 工具串模式，建议同样提取为模块级常量或样式类，保持两页一致的样式管理方式。

-               <fieldset style={{ border: '1px solid #e7e7ea', borderRadius: 8, margin: '12px 0', padding: '8px 12px' }}>
+               <fieldset className={CHANNEL_FIELDSET}>


─── deploy/lago-lab/payment-activation/phases.py:650-657 ───
[bug · medium] 问题：invoice 可见为 failed 的分支中，新调用 `_payments_for` 后未像同 diff 中 invoice-invisible
分支（ocr-84 R1-21，先 `observed["payments_api_status"] = ps` 并 `if not _ok(ps): return
_blocked(...)`）那样先行断言 HTTP 状态。LagoRestClient 仅在传输错误时抛异常，4xx/5xx 会以空列表返回，此时 `non_succeeded = []` 使
`len(non_succeeded) <= 1` 恒真——payments API 故障时该分支的恰好一次断言（len<=1）会被空洞满足，PASS 未被证明；且 observed 未记录
`payments_api_status`，证据无法事后甄别。注释声称与两个兄弟分支"same exactly-once
strictness"，但本分支缺少状态先行校验，恰是前一批修复的同类缺陷在此处被遗漏。

              # (ocr-1 R1-05) Same exactly-once strictness as the two sibling
              # branches (the invoice-invisible canceled endgame and the
              # open/pending pre_charge_ok): the payments API is not affected
              # by the invoice INVISIBLE_STATUS, so the failed endgame PASS
              # also stands on at most one non-succeeded payment.
              ps, payments = _payments_for(ctx, customer["external_id"])
+             observed["payments_api_status"] = ps
+             if not _ok(ps):
+                 return _blocked(ctx, "gate",
+                                 f"payments API answered HTTP {ps} — cannot assert the exactly-once line",
+                                 expected=expected)
              non_succeeded = [p for p in payments if p.get("status") != "succeeded"]
              observed["payments_non_succeeded_count"] = len(non_succeeded)


─── deploy/lago-lab/payment-settle-trigger/lab.sh:39-39 ───
[maintainability · low] 问题：`sed -n '2,18p'` 截取脚本头部注释作为 usage 输出，但头注释块共到第 20 行（"...the 82flow stack
owns 48889/48890 territory and the operator stack the rest."），第 18 行恰在句中截断（"...the isolation pins
are overridden HERE to this"），末尾两句描述被静默丢弃，`lab.sh --help`
输出止于半句。建议将范围扩到完整头注释（'2,20p'），或在注释块后加分界标记按标记截取，避免后续改动头注释时再次漂移。

-   sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
+   sed -n '2,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'


─── deploy/lago-lab/payment-settle-trigger/phases.py:753-756 ───
[bug · medium] 问题：`inbound_webhooks` 前后计数校验对 None 边界处理错误。`lago_db_query` 在 docker/psql 子进程失败时返回
None（按模块自身 A-07/F52 约定属环境缺口）：(1) 当 `inbound_before=None` 而 `inbound_after` 为数字时，`(inbound_before is
not None and ...)` 为假，"row did not land" 检查空洞通过——P-D 接收半程（行落地）未被证明却可继续走向 PASS，达与 webhook_status==200
的断言强度不匹配；(2) 当两者均为 None（docker 不可用）时走 FAIL("row did not land")，但这是环境事实而非契约违反，按本实验室自身截决规则应归类
blocked-env（对照：同函数中 `secret`/`org_id` 读不到时正是返回 _blocked）。两种边界均未按预期分类。

-         if inbound_after is None or (inbound_before is not None
-                                      and int(inbound_after) < int(inbound_before) + 1):
+         if inbound_before is None or inbound_after is None:
+             return _blocked(ctx, "settle_trigger",
+                             "inbound_webhooks count not readable via docker compose "
+                             f"exec db psql (before={inbound_before}, after={inbound_after}) "
+                             "— cannot assert the P-D receive half", expected)
+         if int(inbound_after) < int(inbound_before) + 1:
              observed["error"] = "inbound_webhooks row did not land"
              return _report(ctx, "settle_trigger", expected, observed, FAIL)


─── deploy/lago-lab/payment-settle-trigger/phases.py:413-413 ───
[maintainability · low] 问题：webhook 投递出口使用裸 `build_opener(ProxyHandler({}))`，而同文件 `graphql()` 已按 OCR
r2 改为 origin 绑定的 `clients._proxyless_opener(urlsplit(self.lago_url)[:2])`（OffOriginRedirect
拒绝跨源重定向）。两条出口的代理/重定向纪律不一致：webhook 请求携带着真实 webhook_secret 计算的 HMAC 签名与 PI 载荷，若 Lago 返回跨源重定向会被裸 opener
静默跟随，既进反了仓库安全约束（外部请求纪律）也与同文件声明的机制相悖。

-         opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
+         from urllib.parse import urlsplit
+         opener = clients._proxyless_opener(urlsplit(self.lago_url)[:2])


─── deploy/lago-lab/payment-settle-trigger/run_lab.py:314-315 ───
[bug · low] 问题：secrets 扫描未通过时将局部 `overall` 翻转为 "fail" 决定退出码，但 `environment["run"]["overall"]`
在扫描前已写入且之后仅追加 `environment["secrets_scan"] = scan` 重写文件，`run.overall` 从未同步刷新；timeline 中先前的 "overall
verdict: pass" 行 likewise 未更正。结果是 `t11-environment.json` 可声称 overall=pass 而进程退出码为
1，证据文件与判定自相矛盾（该文件是晋升 docs/ 的依据）。

      if not scan["clean"]:
          overall = "fail"
+         environment["run"]["overall"] = overall
+         timeline.log("overall verdict overridden to fail by secrets scan")


─── internal/handler/commercial.go:875-880 ───
[maintainability · low] 该日志分支会将 CurrentPayablePendingOrderView 的 ErrOrderNotFound
误标为读取故障。ErrOrderNotFound 在此是预期结果而非故障:部分唯一索引已证明存在 channel_failed=false 的 pending
订单,但链接持久化降级(checkout_url='')或清扫竞态的 winner 不满足 payable 谓词(checkout_url <>
''),仓储层注释(R3-26)与服务层同一读取的处理(purchase.go:368 以 !errors.Is(gerr, ErrOrderNotFound)
区分)都将其视为正常降级路径。当前写法会让每次无链接 winner 竞态都输出一条暗示"swept mid-flight, storage fault"的错误级日志,误导排障。建议对
ErrOrderNotFound 静默降级(响应仍为裸 409,控制流不变),仅对真实存储故障保留该诊断日志。

- 		} else {
+ 		} else if !errors.Is(rerr, repocommercial.ErrOrderNotFound) {
  			// (r2:823) The bare-409 degrade keeps one diagnosable line: the
  			// index proved a pending exists but the replay read failed
- 			// (swept mid-flight, storage fault).
+ 			// (swept mid-flight, storage fault). ErrOrderNotFound is the
+ 			// EXPECTED link-less winner face (R3-26: checkout_url=''), not
+ 			// a fault — it stays silent on the same bare 409.
  			log.Printf("commercial: pending-purchase conflict read failed for tenant %d: %v", tenantID, rerr)
  		}


─── internal/modules/commercial/payment/alipay.go:201-202 ───
[bug · low] 重挂环境代理后,走代理时 SSRFSafeDialContext 收到的实际拨号地址是代理地址,依赖 security.go:822 的 IsSystemProxy(addr)
旁路放行。但该旁路用 url.Parse(代理URL).Host 与拨号地址做全等比较:代理 URL 未写显式端口时(如
HTTP_PROXY=http://proxy.internal),parse.Host 不含端口,而 Go Transport 拨代理会经 canonicalAddr 补默认端口(如
:80),两边失配 → 旁路不生效 → 拨号继续走受限后缀/私有 IP 拦截,代理被 SSRF 策略静默拒绝。这正是 R2-17 注释声称要修复的『代理-only
出网不可达』在窄配置形态下复发,且报错会指向自己的代理(『resolves to restricted IP』),难以排查。建议在 IsSystemProxy 中两侧剥离端口后比较 host(或对
http/https 默认端口做规范化),或至少在注释中注明代理 URL 必须带显式端口。



─── internal/modules/commercial/payment/alipay.go:213-216 ───
[maintainability · low] 这段 transport 组装+代理重挂+SSRF 客户端构建与 wechat.go (R2-19) 逐行重复(Proxy
策略、Timeout、MaxRedirects 三处参数各一份)。两渠道的出网安全策略后续单侧修改(如仅调一处 MaxRedirects 或 Proxy 处理)会造成安全配置漂移。建议在
payment 包内提取共享构造助手,一处定义、两渠道复用;后续如需修正代理旁路端口匹配问题也有唯一落点。

- 		client: secutils.NewSSRFSafeHTTPClientWithTransport(secutils.SSRFSafeHTTPClientConfig{
- 			Timeout:      cfg.timeout(),
+ // newChannelHTTPClient 构建渠道共享的 SSRF 安全出网客户端:重挂环境代理、
+ // 拨号期 DNS 固定、逐跳重定向校验并以 MaxRedirects 封顶。
+ func newChannelHTTPClient(timeout time.Duration) *http.Client {
+ 	transport := secutils.NewSSRFSafeTransport(secutils.SSRFSafeHTTPClientConfig{})
+ 	transport.Proxy = http.ProxyFromEnvironment
+ 	return secutils.NewSSRFSafeHTTPClientWithTransport(secutils.SSRFSafeHTTPClientConfig{
+ 		Timeout:      timeout,
- 			MaxRedirects: 10,
+ 		MaxRedirects: 10,
- 		}, channelTransport),
+ 	}, transport)
+ }


─── internal/modules/commercial/payment/provider.go:56-57 ───
[documentation · medium] 本次改动在扩展 AmountFen/AmountCurrency 文档的同时，上方仍保留旧语义："ProviderID is the
provider-visible identifier of the ORIGINAL request (for WeChat the out_trade_no)"。而同一变更集中 wechat.go
/ alipay.go 的 Query 已改为优先返回渠道交易号（transaction_id / trade_no，见 wechat.go 新注释 "keying out_trade_no here
would misread the SAME payment ... as two different channel transactions"）。接口文档与实现直接矛盾：后续按此注释实现新
Provider、或按 out_trade_no 消费 Query 结果的调用方，会重新引入 #83 的重复事实/虚假超付审计缺陷。建议同步改写该句，说明字段语义按方法区分：Create
返回原始请求标识（out_trade_no / MerchantOrderID），Query 在渠道应答携带交易号时返回渠道交易号。

- // or code URL when the channel returns one. AmountFen (#84/G2) is the
- // COLLECTED amount the channel's Query reports (WeChat amount.total /
+ // AttemptResult reports the provider-side state of one attempt. From
+ // Create, ProviderID is the provider-visible identifier of the ORIGINAL
+ // request (out_trade_no / MerchantOrderID); from Query it is the channel
+ // TRANSACTION id when the answer carries one (WeChat transaction_id /
+ // Alipay trade_no), falling back to the original identifier — the value
+ // feeds ConfirmPayment's duplicate-fact unity check. CheckoutURL carries
+ // the customer-facing payment link or code URL when the channel returns
+ // one. AmountFen (#84/G2) is the ...


─── internal/modules/commercial/payment/providers_env.go:61-64 ───
[maintainability · low] 确定性修复只落在了 alipayConfigFromEnv：同文件 wechatConfigFromEnv（第 139 行）的
partial-config 错误仍遍历 map（for name, dst := range required），错误信息里点名的缺失变量仍随 map 迭代顺序抖动——正是本处注释声称要消除的
"map iteration made the message flaky across runs" 缺陷。建议将 wechat 侧 required
一并改为固定顺序切片，两渠道错误信息行为保持一致，避免后人参照 alipay 修复时遗漏另一条腿。

  	set := []struct {
  		name string
  		dst  *string
  	}{
+ 		{"WEKNORA_ALIPAY_APP_ID", &cfg.AppID},
+ 		...
+ 	}
+ 	// wechatConfigFromEnv 的 required 应采用同样的固定顺序切片，
+ 	// 消除其 partial-config 错误信息的 map 迭代非确定性。


─── internal/modules/commercial/payment/wechat.go:186-187 ───
[bug · low] 与 alipay.go (R2-17) 相同的问题:重挂环境代理依赖 SSRFSafeDialContext 内 IsSystemProxy 的 host:port
全等旁路,端口缺省的代理 URL(如 HTTPS_PROXY=http://proxy)会因 canonicalAddr 补默认端口导致失配,旁路失效后代理地址落入受限后缀/私有 IP
拦截,渠道出网被静默阻断——R2-19 要修复的场景在窄配置形态下仍会复发。建议在 IsSystemProxy 中两侧剥离端口后比较 host,或注明代理 URL 必须带显式端口。



─── internal/modules/commercial/purchase_settlement_command.go:78-80 ───
[maintainability · low]
命令幂等键前缀派生风格与接缝全部既有先例不一致：EnsureSubscriptionCommandKey、GrantCreditsCommandKey、CreatePurchaseSubscripti
onCommandKey 以及同批新增的 RebalanceCreditsOrderCommandKey 均以 string(CommandKind…) 派生前缀，唯有此处硬编码字面量
"settle:"。后果：键不再自证命令种类（outbox 排障/审计时可读性下降），且键命名空间与 CommandKind 词表脱钩——未来任何以 "settle" 开头派生键的命令种类会在
outbox 去重命名空间中与本键族不可辨识地共享前缀，埋下跨种类键冲突的隐患。建议按同一先例由 CommandKind 派生（合并前修改无存量 in-flight 键的兼容负担）。

  func SettlePurchasePaymentCommandKey(externalPurchaseSubscriptionID, channelTransaction string) string {
- 	return "settle:" + externalPurchaseSubscriptionID + ":" + channelTransaction
+ 	return string(CommandKindSettlePurchasePayment) + ":" + externalPurchaseSubscriptionID + ":" + channelTransaction
  }


─── internal/modules/commercial/repository/commercial/order.go:744-748 ───
[bug · low] 该分支的异常快照 expected 面取自 attempt，但触发该分支的前提恰是前一检查已证明 attempt 与 fact
完全一致（OrderID/TenantID/AmountFen/Currency 全等），真正与 fact 不一致的是订单行 row。因此落库的异常行会呈现 kind=amount_mismatch
且 ExpectedAmountFen == ActualAmountFen、ExpectedCurrency == ActualCurrency 的自相矛盾记录，真正的期望面
row.AmountFen/row.Currency（作用域内可得）被丢弃，运营处置面看不到实际差异。registerAttemptTx 并不交叉校验 attempt
金额与订单冻结面一致，该分支可达（OCR84-r1 同结论、建议修复未落实）。建议此分支用订单行覆写 expected 面并重算 kind。

  		if err := domain.ValidatePayment(row.Domain(), fact); err != nil {
  			snap := buildMismatchAnomaly(attempt, fact)
+ 			// 该分支 attempt==fact，真正不一致的是订单行的冻结价面
+ 			snap.ExpectedAmountFen = row.AmountFen
+ 			snap.ExpectedCurrency = row.Currency
+ 			snap.Kind = ClassifyPaymentAnomaly(row.AmountFen, int64(fact.Amount), row.Currency, fact.Currency)
  			anomaly = &snap
  			return err
  		}


─── internal/modules/commercial/repository/commercial/payment_anomaly.go:184-194 ───
[maintainability · low] RowsAffected==0 且行存在时一律返回 ErrPaymentAnomalyVersionConflict（handler 侧映射 409
"anomaly changed since read"），把两种不同情况混叠为一个哨兵：版本确实过期，以及行已处于 resolved 状态（操作员对一次已成功的 resolve
做网络重放）。后者在语义上是幂等成功，却报 409 误导操作面排查（OCR84-r1 已给出同结论与修复建议，当前代码未落实）。建议回读后先判 state：已 resolved
的行直接返回该行，仅版本不匹配才报冲突。

  	if res.RowsAffected == 0 {
  		var row PaymentAnomalyRow
  		err := s.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
  		if errors.Is(err, gorm.ErrRecordNotFound) {
  			return PaymentAnomalyRow{}, ErrPaymentAnomalyNotFound
  		}
  		if err != nil {
  			return PaymentAnomalyRow{}, err
+ 		}
+ 		if row.State == PaymentAnomalyStateResolved {
+ 			return row, nil // idempotent replay of an already-resolved anomaly
  		}
  		return PaymentAnomalyRow{}, ErrPaymentAnomalyVersionConflict
  	}


─── internal/modules/commercial/service/commercial/benefits.go:674-676 ───
[performance · medium] 读路径放大（已核实）：GET /commercial/account 的每次调用都会运行完整 lazy 链（handler AccountStatus
注释明确 the whole lazy chain runs behind this GET），本函数现在每次执行固定新增：① effectiveFeatures 里的 purchase
快照读（active 时还追加 finalized-invoice 读）；② 此处无条件 SubmitCommand(RebalanceCreditsOrder)——Lago 侧每次至少 1 个
wallet-list GET（已核实适配器不按 Key 去重，稳态收敛时也是每次全量 list）；③ SyncLots 本地事务对每个未到期 lot 无条件执行
UPDATE（即使值未变，稳态也产生写放大）。BillingPage 轮询该端点时，每个租户每轮从 1 次 authority GET 变为 2-4 次串行 GET + 本地写。建议：对
rebalance 增加触发门槛（如仅当快照存在多钱包族且到期序可能变化时、或按租户限频），SyncLots 的 UPDATE 加 `WHERE remaining_micro <> CASE...`
谓词使稳态刷新只读不写。



─── internal/modules/commercial/service/commercial/order.go:515-517 ───
[maintainability · low] HasUnresolvedPaymentAnomaly 的读错误被完全静默吞掉：注释放宽为「degrades to no
attention」但未留任何日志，与本次变更集内统一的 A-23 姿势（benefits.go 的 AccountHolds 失败、SyncLots 失败均留一条 Warn）不一致。异常读持续失败时
attention 标志会静默消失，运营面无法从日志定位。建议失败时补一条 logger.Warnf（purchase.go PurchaseStatus 中同型的
HasUnresolvedPaymentAnomaly 调用同样处理）。



─── internal/modules/commercial/service/commercial/purchase.go:295-297 ───
[bug · low] 渠道切换分支缺少 paid_awaiting_activation 合成：CloseChannelOrder
的查询确认了旧渠道付款（closeView.State=paid）时，此处直接 purchaseView 透传 p.State（此路径上必为 awaiting_payment），POST 应答变成
state=awaiting_payment + order.state=paid 的矛盾面。这与同函数 D11 分支（显式 `if p.State == AwaitingPayment {
out.State = PaidAwaitingActivation }`）及 PurchaseStatus 的合成语义（#82 D3：本地 paid + authority 未激活必须投影为
paid_awaiting_activation）不一致，客户端要到下一次 GET 才能看到正确状态。建议与 D11 分支对齐补上同样的合成。



─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:389-389 ───
[maintainability · low] observeActivation 直接使用 time.Now()/time.After，而类型持有注入时钟 p.now（Fulfill 内的
period 计算均已用 p.now）。混用两个时钟使观察窗口/预算相关的测试无法确定性控制时间，也削弱了构造函数建立的注入约定。建议统一改用 p.now() 计算 deadline 与循环判断。



─── internal/modules/commercial/subscription_command.go:164-167 ───
[bug · medium] WalletRank 未对零值 ExpiresAt 设防，与自身 fail-closed 哲学及姊妹助手不一致：唯一生产调用方
rebalanceCreditsOrder（lago.go:227-231）直接以 parseRFC3339UTC(w.ExpirationAt) 填充，而 parseRFC3339UTC
的契约是「unparseable input answers the zero time (callers treat zero as unknown)」——但 WalletRank 并未把零值当作
unknown，排序中零值 time.Time 早于一切真实时刻，会被排为第 1 名并 PUT 为最高消费优先级，使到期未知的钱包先于所有会过期的批次被消费，弱化 #86/spec L132
的到期优先消费不变量（过期批次的积分可能在未被消费前失效）。同函数对重复 WalletRef、空 ref、域溢出均选择响亮报错，MonthlyWalletPriorityFor 也显式以
exp.After(time.Time{}) 排除零值，唯独此处对权威侧 expiration_at 为 null/畸形的数据异常静默按最早到期处理。建议在同一防御循环中对零值
ExpiresAt（以及 GrantedAt）fail closed。

  		if seen[b.WalletRef] {
  			return nil, fmt.Errorf("wallet rank input carries duplicate wallet ref %q", b.WalletRef)
+ 		}
+ 		if b.ExpiresAt.IsZero() {
+ 			return nil, fmt.Errorf("wallet rank input %q carries an unknown (zero) expiry", b.WalletRef)
  		}
  		seen[b.WalletRef] = true


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:16-19 ───
[maintainability · low] envRequired 已从 _browser_lib.mjs 导入但从未调用（整个取证树中仅 browser_flow_82.mjs
调用它），属于死导入；同时这里手写的 FLOW82_EMAIL_D / FLOW82_PASSWORD_D 回退检查与 browser_sync_face_82.mjs 中 _B
后缀版本近乎逐行重复，凭据校验规则变更时需要多点同步修改。建议在 _browser_lib.mjs 中增加一个支持别名的取值助手（缺失时同样打印 usage 并 exit
2），两个脚本共用，既消除死导入也去掉重复逻辑。

- if (!process.env.FLOW82_EMAIL_D && !process.env.FLOW82_EMAIL) {
-   console.error('missing required env: FLOW82_EMAIL(_D)');
-   process.exit(2);
- }
+ // _browser_lib.mjs 中新增：
+ // export function envAlias(...names) {
+ //   const hit = names.find((n) => process.env[n]);
+ //   if (!hit) {
+ //     console.error(`missing required env: ${names.join(' / ')} (no source-code fallback)`);
+ //     process.exit(2);
+ //   }
+ //   return process.env[hit];
+ // }
+ // 本文件改为：
+ const EMAIL = envAlias('FLOW82_EMAIL_D', 'FLOW82_EMAIL');
+ const PASSWORD = envAlias('FLOW82_PASSWORD_D', 'FLOW82_PASSWORD');


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:15-18 ───
[maintainability · low] envRequired 已导入但从未调用（整个取证树中仅 browser_flow_82.mjs 调用它），属于死导入；且这里手写的
FLOW82_EMAIL_B / FLOW82_PASSWORD_B 回退检查与 browser_paid_face_82.mjs 中 _D 后缀版本近乎逐行重复。建议在
_browser_lib.mjs 中提供支持别名的取值助手并在此复用，同时从 import 列表移除未使用的 envRequired。

- if (!process.env.FLOW82_EMAIL_B && !process.env.FLOW82_EMAIL) {
-   console.error('missing required env: FLOW82_EMAIL(_B)');
-   process.exit(2);
- }
+ // _browser_lib.mjs 中新增：
+ // export function envAlias(...names) {
+ //   const hit = names.find((n) => process.env[n]);
+ //   if (!hit) {
+ //     console.error(`missing required env: ${names.join(' / ')} (no source-code fallback)`);
+ //     process.exit(2);
+ //   }
+ //   return process.env[hit];
+ // }
+ // 本文件改为：
+ const EMAIL = envAlias('FLOW82_EMAIL_B', 'FLOW82_EMAIL');
+ const PASSWORD = envAlias('FLOW82_PASSWORD_B', 'FLOW82_PASSWORD');


─── docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py:285-289 ───
[bug · low] ORDERS 是模块级全局 dict，由 ThreadingHTTPServer 按连接开线程并发读写且无任何锁：close 分支存在 check-then-act（先读
state==SUCCESS 再写 CLOSED），与 /stub/mark 的写入（如编排脚本恰在窗口内把订单标为 SUCCESS）交错时，会把已标付的订单覆写为 CLOSED、按 204
应答，恰好污染 #83 要编排的 ORDER_PAID 竞态证据；/stub/notify 处理期间阻塞在 urlopen（最长 15s），同时 WeKnora 恢复轮询的 Query
可从另一连接到达，并发是真实可达而非纯理论。建议加模块级 threading.Lock 保护所有 ORDERS 的复合读改写。

+ import threading
+ ORDERS_LOCK = threading.Lock()
+ 
+ # close 分支（/stub/mark 同理）改为：
+             with ORDERS_LOCK:
-             if order["state"] == "SUCCESS":
+                 if order["state"] == "SUCCESS":
-                 log("CLOSE %s -> 400 ORDER_PAID (race shape)" % order_id)
+                     log("CLOSE %s -> 400 ORDER_PAID (race shape)" % order_id)
-                 self._reply(400, {"code": "ORDER_PAID", "message": "订单已支付，禁止关单"})
+                     self._reply(400, {"code": "ORDER_PAID", "message": "订单已支付，禁止关单"})
-                 return
+                     return
-             order["state"] = "CLOSED"
+                 order["state"] = "CLOSED"


─── docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py:207-207 ───
[maintainability · low] do_GET 中 partition("?") 得到的 query 变量从未被读取（签名校验用的是原始 self.path，订单解析用的是切分后的
path），属死变量；留着易让读者误以为查询串参与了某个分支的处理。

-         path, _, query = self.path.partition("?")
+         path, _, _ = self.path.partition("?")


─── docs/plans/issue-72-flow-evidence-84/wechat_native_stub_anomaly.py:300-304 ───
[bug · low] 与 #83 原版同源的缺陷：ORDERS 为模块级全局 dict，ThreadingHTTPServer 按连接开线程并发读写且无锁，close 分支的
check-then-act（读 state==SUCCESS 后写 CLOSED）与 /stub/mark 的写入交错时会把已标付订单覆写为 CLOSED 并按 204
应答，可能污染异常金额腿的编排断言；/stub/notify 阻塞在 urlopen 期间恢复轮询 Query 可并发到达。建议加 threading.Lock 保护 ORDERS 的复合读改写。

+ import threading
+ ORDERS_LOCK = threading.Lock()
+ 
+ # close 分支（/stub/mark 同理）改为：
+             with ORDERS_LOCK:
-             if order["state"] == "SUCCESS":
+                 if order["state"] == "SUCCESS":
-                 log("CLOSE %s -> 400 ORDER_PAID (race shape)" % order_id)
+                     log("CLOSE %s -> 400 ORDER_PAID (race shape)" % order_id)
-                 self._reply(400, {"code": "ORDER_PAID", "message": "订单已支付，禁止关单"})
+                     self._reply(400, {"code": "ORDER_PAID", "message": "订单已支付，禁止关单"})
-                 return
+                     return
-             order["state"] = "CLOSED"
+                 order["state"] = "CLOSED"


─── docs/plans/issue-72-flow-evidence-84/wechat_native_stub_anomaly.py:212-212 ───
[maintainability · low] do_GET 中 partition("?") 得到的 query 变量从未被读取（签名校验用原始 self.path，订单解析用切分后的
path），属死变量，易误导读者以为查询串参与处理。

-         path, _, query = self.path.partition("?")
+         path, _, _ = self.path.partition("?")


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:40-40 ───
[maintainability · low] 移除 runs/ 回退后，本文件顶部的 `import glob` 与 `RUNS = Path(...)` 常量不再被任何代码引用（全文件已无
`glob.` 调用，`RUNS` 仅剩定义行），成为死导入/死变量；同时其上方注释块（"Fall back to the newest runs/ TSV only when no archive
exists, and always print the file actually used."）仍在描述已被删除的回退行为，与下方 (R3-17) 的 MISSING-EVIDENCE
逻辑相矛盾。建议随本次清理一并删除，避免读者误以为 runs/ 回退仍然存在。

- path = str(archived)
+ import json
+ import re
+ import sys
+ from pathlib import Path
+ 
+ EVID = Path(__file__).resolve().parent
+ 
+ archived = EVID / "verify-db-watch-samples.tsv"


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:47-47 ───
[maintainability · low] 移除 runs/ 回退后，本文件顶部的 `import glob` 与 `RUNS = Path(...)` 常量不再被任何代码引用（全文件已无
`glob.` 调用，`RUNS` 仅剩定义行），成为死导入/死变量；同时其上方注释块（"Fall back to the newest runs/ TSV only when no archive
exists, and always print the file actually used."）仍在描述已被删除的回退行为，与下方 (R3-17) 的 MISSING-EVIDENCE
逻辑相矛盾。建议随本次清理一并删除（ocr1/ocr2/ocr3 三份回放副本同样存在）。

- path = str(archived)
+ import json
+ import re
+ import sys
+ from pathlib import Path
+ 
+ EVID = Path(__file__).resolve().parent
+ 
+ archived = EVID / "verify-db-watch-samples.tsv"


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:44-44 ───
[maintainability · low] 移除 runs/ 回退后，本文件顶部的 `import glob` 与 `RUNS = Path(...)` 常量不再被任何代码引用（全文件已无
`glob.` 调用，`RUNS` 仅剩定义行），成为死导入/死变量；同时其上方注释块（"Fall back to the newest runs/ TSV only when no archive
exists, and always print the file actually used."）仍在描述已被删除的回退行为，与下方 (R3-17) 的 MISSING-EVIDENCE
逻辑相矛盾。建议随本次清理一并删除（74/ocr1/ocr3 副本同样存在）。

- path = str(archived)
+ import json
+ import re
+ import sys
+ from pathlib import Path
+ 
+ EVID = Path(__file__).resolve().parent
+ 
+ archived = EVID / "verify-db-watch-samples.tsv"


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:44-44 ───
[maintainability · low] 移除 runs/ 回退后，本文件顶部的 `import glob` 与 `RUNS = Path(...)` 常量不再被任何代码引用（全文件已无
`glob.` 调用，`RUNS` 仅剩定义行），成为死导入/死变量；同时其上方注释块（"Fall back to the newest runs/ TSV only when no archive
exists, and always print the file actually used."）仍在描述已被删除的回退行为，与下方 (R3-17) 的 MISSING-EVIDENCE
逻辑相矛盾。建议随本次清理一并删除（74/ocr1/ocr2 副本同样存在）。

- path = str(archived)
+ import json
+ import re
+ import sys
+ from pathlib import Path
+ 
+ EVID = Path(__file__).resolve().parent
+ 
+ archived = EVID / "verify-db-watch-samples.tsv"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:29-32 ───
[maintainability · medium] 重复样板：本文件与 r4-flow 其余三腿（browser_02/03/04）各自完整复制了同一套引导——createRequire 加载
Playwright、FLOW82_WEB/EMAIL/PASSWORD 校验、note/results 断言记录器、登录流程（goto /login → fill → click →
waitForURL）、try/catch/finally + RESULT 汇总 + process.exit 退出码契约，每份约 30~40 行。而同批新增的同目录父级共享库
docs/plans/issue-72-flow-evidence-82/_browser_lib.mjs 已导出
requirePlaywright/login/note/runLeg/envRequired（其头注释明确「one copy ... instead of drifting across 15
copies」），顶层三个驱动与 r5-verify 各腿均已复用，仅 r4-flow 系列未接入。后续登录选择器或退出码契约变更需同步修改多处，极易造成各腿取证行为漂移。建议复用
_browser_lib.mjs（runLeg 目前不支持 catch 中截图，可为其扩展 screenshot 钩子后统一接入）。

- const note = (step, ok, detail) => {
-   results.push({ step, ok, detail });
-   console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
- };
+ import { runLeg, login, note, envRequired } from '../_browser_lib.mjs';
+ // runLeg(body) 内置浏览器 launch/close、A-03 catch 包装、RESULT 行与退出码契约；
+ // login(page, web, email, password) 内置冻结选择器集；note(results, step, ok, detail)。
+ // 错误时截图能力可在 _browser_lib.mjs 的 runLeg 上加 screenshot 钩子统一获得。


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_02_sync_face.mjs:23-26 ───
[maintainability · medium] 同 browser_01 注释（重复样板）：本腿完整复制了 _browser_lib.mjs（同批新增、顶层驱动与 r5-verify
已复用）已封装的 note 记录器/登录流程/RESULT 契约；此外第 60~64 行手写的 order-identity 断言也是共享库 assertOrderIdentity
的弱化重实现。建议统一接入共享库，避免选择器与契约漂移。

- const note = (step, ok, detail) => {
-   results.push({ step, ok, detail });
-   console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
- };
+ import { runLeg, login, note, assertOrderIdentity, envRequired } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_03_paid_face.mjs:23-26 ───
[maintainability · medium] 同 browser_01 注释（重复样板）：本腿完整复制了 _browser_lib.mjs（同批新增、顶层驱动与 r5-verify
已复用）已封装的 note 记录器/登录流程/A-03 catch 包装/RESULT 契约。建议统一接入共享库，避免选择器与契约漂移。

- const note = (step, ok, detail) => {
-   results.push({ step, ok, detail });
-   console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
- };
+ import { runLeg, login, note, envRequired } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:21-24 ───
[maintainability · medium] 同 browser_01 注释（重复样板）：本腿完整复制了 _browser_lib.mjs（同批新增、顶层驱动与 r5-verify
已复用）已封装的 note 记录器/登录流程/A-03 catch 包装/RESULT 契约。建议统一接入共享库，避免选择器与契约漂移。

- const note = (step, ok, detail) => {
-   results.push({ step, ok, detail });
-   console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
- };
+ import { runLeg, login, note, envRequired } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:65-66 ───
[security · medium] 敏感信息经 argv 暴露：新铸造的 Stripe webhook secret 以 `docker compose exec -e
WSECRET="$SECRET"` 的命令行参数形式传入，字面量会出现在 docker CLI 进程 argv 中、可被本机 ps 观察。这与同一变更组内 seed.sh
自身声明的纪律直接矛盾（reg() 注释明确「a curl argv password is observable in local ps」并改走 STDIN），且本脚本是
settle-evidence/prepare_t9_env.sh 的同形变体，该处同样以 argv 传递——正在形成跨脚本的系统性模式。建议改经 stdin 传入容器。

- docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T -e WSECRET="$SECRET" api bin/rails runner \
-   "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: ENV['WSECRET'])" >/dev/null
+ printf '%s' "$SECRET" | docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
+   "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: \\$stdin.read)" >/dev/null


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:16-17 ───
[bug · low] 异常处理缺口（静默死亡）：本行在 `set -euo pipefail` 下读取存量 secret——api 容器未就绪或 rails 报错时 docker
侧以非零退出，pipefail 使命令替换整体失败，errexit 直接中止脚本，而 stderr 已被 `2>/dev/null`
丢弃：操作者只能看到一个无任何输出的非零退出，无法区分「读取失败」与「确无存量 secret」。同形的 settle-evidence/prepare_t9_env.sh 已在
OCR84-R1-23② 中修复为保留 stderr 并回显尾部诊断，本变体倒退回了未修复形态。建议移除 stderr 丢弃并加显式守卫。

  STORED=$(docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
-   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s" 2>/dev/null | sed -n 's/.*__R4SECRET__//p')
+   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s" | sed -n 's/.*__R4SECRET__//p') \
+   || { echo "FAIL: reading the stored webhook secret (api container ready? see stderr above)" >&2; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:11-11 ───
[maintainability · low] 边界条件缺失：COMPOSE_FILE 依赖调用时工作目录恰为仓库根（`$(pwd)/deploy/lago/compose.yaml`），脚本内既无
cd 守卫也无文件存在性校验；从其他目录 source/执行时 docker compose 将因文件不存在而报错（或更糟：命中恰好存在的同名文件）。对比同类
deploy/lago-lab/*/lab.sh 均以脚本自身位置解析仓库根（`REPO_DIR` + 相对路径），且 seed.sh 已建立「EV 先解析再
cd」的调用目录无关性纪律（OCR84-R1-18）。建议改为脚本相对定位并加存在性断言。

- COMPOSE_FILE="$(pwd)/deploy/lago/compose.yaml"
+ COMPOSE_FILE="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)/deploy/lago/compose.yaml"
+ [ -f "$COMPOSE_FILE" ] || { echo "FAIL: compose file not found at $COMPOSE_FILE" >&2; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:77-77 ───
[bug · low] 取证产物未断言：checkoutHref 通过 getAttribute('href') 获取后未做任何 null/空值校验，也未计入 results，直接写入
order-info.json——若「前往支付」链接的 href 瞬时缺失，产物文件将携带 null 且脚本整体仍 PASS（退出码
0），下游无法察觉。这与本脚本自身强调的「断言失败与基础设施失败可区分」取证纪律不符（orderId 就有对应的 order-id-visible 断言）。建议在写盘前断言非空。

    const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
+   note('checkout-href-present', typeof checkoutHref === 'string' && checkoutHref.length > 0, String(checkoutHref));


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:160-160 ───
[security · low] 敏感信息一致性缺口：LAGO_KEY（Lago 管理 API key）从容器 DB 读出后经 `-H "Authorization: Bearer
$LAGO_KEY"` 走 curl argv，可被本机 ps 观察，与同脚本对 PW 采用 STDIN 传递的处理标准不一致。该问题已在分支自身 OCR
台账中记录（issue-72-ocr-issue-82-r1.md / issue-72-ocr-issue-84-r1.md 均记为 security·low 并给出 header
文件修法），但本轮 seed.sh 及 r4-flow2/3、r5 系列均未采纳。虽为实验栈内部 key（127.0.0.1:48889）风险有限，仍建议统一改为 header 文件方式。

- curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
+ LAGO_HDR=$(mktemp); printf 'Authorization: Bearer %s\n' "$LAGO_KEY" > "$LAGO_HDR"; trap 'rm -f "$LAGO_HDR"' EXIT
+ curl -s "http://127.0.0.1:48889/api/v1/plans" -H @"$LAGO_HDR" > "$EV/api-03-lago-plans-baseline.json"


─── docs/plans/issue-72-flow-evidence-82/_browser_lib.mjs:64-68 ───
[bug · low] runLeg 将 requirePlaywright() 与 chromium.launch() 放在 try 块之外，违背了本文件头部声明的 A-03 契约（"任何抛错的
leg 仍须产出 RESULT 行"）：当 @playwright/test 未安装（docs 树没有自己的 node_modules，createRequire 会直接抛错）或 chromium
浏览器二进制缺失时，leg 在进入 try 之前就以裸堆栈终止，RESULT JSON 行不会输出，下游按 "RESULT " 行解析结果的取证消费者将拿不到任何记录。建议把
require/launch 一并纳入 try，并让 finally 对 browser 为 undefined 的情况容错。

  export async function runLeg(body) {
-   const { chromium } = requirePlaywright();
    const results = [];
-   const browser = await chromium.launch();
+   let browser;
    try {
+     const { chromium } = requirePlaywright();
+     browser = await chromium.launch();
+     await body(results, browser);
+   } catch (err) {
+     // (A-03) A thrown browser leg must not die with a bare stack trace and
+     // a zero exit code: record the failure face, keep the RESULT contract.
+     note(results, 'leg-error', false, String(err?.message ?? err));
+   } finally {
+     await browser?.close().catch(() => { /* best-effort close */ });
+   }


─── docs/plans/issue-72-flow-evidence-74/verify_ac_assertions.py:68-71 ───
[maintainability · low] 断言名 "AC1 invoice open/pending/numberless" 仍只描述旧的
open/pending/无号形态,但本次改动已把条件放宽为也接受 invoice_status 为 "failed"/"closed" 的失败终局。同批改动已把另一分支的断言名从 "still
incomplete across window" 更名为 "gate held across window" 以保持名实一致,这里漏改了:evidence 输出会出现 "PASS AC1
invoice open/pending/numberless" 而实际发票状态是 failed/closed,对审计读数有误导。(ocr1/ocr2/ocr3 三个 replay
副本为同一段代码,需同步修改。)

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or failed/closed endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:71-74 ───
[maintainability · low] 断言名 "AC1 invoice open/pending/numberless" 未随本次条件放宽同步更新:条件现在也接受
invoice_status 为 "failed"/"closed" 的失败终局,输出 "PASS AC1 invoice open/pending/numberless"
会与实际证据(failed/closed)不符。同批改动已将另一分支断言名更名以保持名实一致,此处应同样更新。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or failed/closed endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:73-76 ───
[maintainability · low] 断言名 "AC1 invoice open/pending/numberless" 未随本次条件放宽同步更新:条件现在也接受
invoice_status 为 "failed"/"closed" 的失败终局,输出 "PASS AC1 invoice open/pending/numberless"
会与实际证据(failed/closed)不符。同批改动已将另一分支断言名更名以保持名实一致,此处应同样更新。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or failed/closed endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:82-85 ───
[maintainability · low] 断言名 "AC1 invoice open/pending/numberless" 未随本次条件放宽同步更新:条件现在也接受
invoice_status 为 "failed"/"closed" 的失败终局,输出 "PASS AC1 invoice open/pending/numberless"
会与实际证据(failed/closed)不符。同批改动已将另一分支断言名更名以保持名实一致,此处应同样更新。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or failed/closed endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:41-41 ───
[documentation · low] 本批为 AC1 两个分支新增了约 25 行注释/守卫,使文件顶部 docstring 的溯源描述失真:其中声称 "Relative to the ocr-1
copy (86 lines)" 且 "# ocr-2:" 注释位于 "lines 74 and 84",而当前 ocr-1 副本已为 107 行、两个 "# ocr-2:" 注释实际在第 96 与
106 行。ocr-3 副本的同位 docstring 本次已按 "anchor descriptions on check names, not line numbers" 一并修正,ocr-2
副本应同步更新,避免留下与 R1-V23 同类的错误溯源声明。



─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:9-9 ───
[documentation · low] 紧随本行之下的 docstring 段落("ocr-1 replay copy: ... Data source is ARCHIVED-FIRST:
... the runs/ observer TSV is only a fallback when no archive exists")仍在描述本次已删除的 runs/
回退行为,与本行更新后的退出码语义以及下方 (R3-17) 的选择期直接 MISSING-EVIDENCE 逻辑直接矛盾(此位置不同于已确认的行内 "Fall back..." 注释块问题)。清理
import glob/RUNS 死代码时应一并改写这段 docstring,明确 runs/ 回退已移除、归档缺失即 exit(2)。



─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:181-182 ───
[test · medium] 提交的证据与同 commit 修复后的 seed.sh 断言不自洽，R-4 的 draft→publish→Lago 投影环节证据不成立：(1)
api-01-draft.json 显示本轮 draft version=2，而 api-02-publish.json 的 receipt 是
publish_plan_version:pro:1（version=1）——按 A-14 注释自己的语义，这记录的是一次『publish 作用于更早轮次残留的 v1 而非本轮 draft 的
v2』的运行，恰是该修复要防的缺陷现场；用当前脚本的 receipt.command_key == "publish_plan_version:pro:$DRAFT_V" 断言重放必然
FAIL。(2) api-03-lago-plans.json 中 pro plan 的 created_at（2026-09-25T03:07:32Z）早于 api-02 的
published_at（2026-09-26T15:46:12Z）一整天，Lago 里的 pro 更像前轮残留而非本轮投影（A-09 注释担心的假阳性形态）。(3) 目录内缺少当前脚本必产出的
api-03-lago-plans-baseline.json。建议用修复后的 seed.sh 重新采集本目录证据，或在目录内明确标注这批 JSON 的采集轮次/脚本版本。



─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:169-169 ───
[bug · low] 异常处理缺口（静默死亡）：draft 与 publish 两处 curl 在 set -euo pipefail 下若传输失败（backend 未起、连接拒绝），curl -s
吞掉 stderr、pipefail 触发 errexit，脚本无任何 FAIL 诊断即中止，只留下空/半截证据文件——与同脚本 reg_expect()（|| true + case 显式诊断）和
login()（|| true + 显式断言）自身建立的失败可区分纪律不一致，A-13 注释也明确说明了这一风险。建议同样追加 || true，让下方已有的 jq -e 断言统一负责失败诊断（curl
失败 → 文件为空 → DRAFT_V=0 / receipt 断言给出明确 FAIL 行）。

-     "limits":{},"charges":[]}' | tee "$EV/api-01-draft.json" | jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' | say "draft: $(cat)"
+     "limits":{},"charges":[]}' | tee "$EV/api-01-draft.json" | jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' | say "draft: $(cat)" || true
+ # (publish 行同理) 传输失败时保诊断：交给下方 jq -e 断言给出 FAIL 行，而非 pipefail 静默中止


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/diag_checkout.mjs:11-11 ───
[bug · low] 凭据环境变量未做缺失校验：同目录 browser_01~browser_04 及共享库的 envRequired() 均会先校验 FLOW82_EMAIL_A /
FLOW82_R5_PW 并以清晰提示 exit(2)（库注释中的 no-source-code-fallback 姿态）。本脚本直接传 process.env 值，缺失时会在 Playwright
深处抛出难懂的 fill(undefined) 错误。建议复用 envRequired。

- await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);
+ const { FLOW82_EMAIL_A: EMAIL, FLOW82_R5_PW: PASSWORD } = envRequired('FLOW82_EMAIL_A', 'FLOW82_R5_PW');
+ await login(page, WEB, EMAIL, PASSWORD);
+ // 并在顶部 import 中加入 envRequired：import { login, requirePlaywright, envRequired } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/diag_checkout.mjs:16-16 ───
[maintainability · low] 截图路径用裸 file URL 的 .pathname 拼接：Windows 下会得到 /C:/... 形式，且空格/非 ASCII
会被百分号编码，导致写文件失败。共享库已提供 evidencePath()（内部用 fileURLToPath），同目录其余腿均使用
evidencePath('r5b-flowcheck')，建议保持一致。

- await page.screenshot({ path: new URL('.', import.meta.url).pathname + 'diag-checkout.png', fullPage: true });
+ const EV = evidencePath('r5b-flowcheck');
+ await page.screenshot({ path: `${EV}diag-checkout.png`, fullPage: true });
+ // 并在顶部 import 中加入 evidencePath：import { login, requirePlaywright, evidencePath } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:156-157 ───
[bug · medium] BASE_N 缺少上一轮 r4-flow2 已加回的 (OCR84-R1-32) 可解析性防护：baseline curl 落空文件时 jq 输出空串（随后 [ "$N"
-gt "$BASE_N" ] 报 integer expression expected 并落入误导性 elif 分支），落盘 Lago 错误体时 jq exit 5 被 set -e
无诊断中止——两种形状都无法 fail honestly。本轮是从 r4-flow 拷贝时丢失了 r4-flow2 的加固，建议同型补回。

- BASE_N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans-baseline.json")
+ BASE_N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans-baseline.json") \
+   || { say "FAIL: baseline plans read not parseable (see api-03-lago-plans-baseline.json)"; exit 1; }
+ case "$BASE_N" in ''|*[!0-9]*) say "FAIL: baseline read not a count: '$BASE_N'"; exit 1 ;; esac
  say "baseline 9900 plans already in Lago: $BASE_N"


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:182-182 ───
[bug · medium] N 同型缺失 (OCR84-R1-32) 防护（r4-flow2 已有）：非数值形状时 [ "$N" -gt "$BASE_N" ] / -ge 的报错路径会产生误导性
FAIL 文案或直接被 set -e 以晦涩方式中止，无法与 r4-flow2 一样显式报告 'not parseable / not a count'。

- N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans.json")
+ N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans.json") \
+   || { say "FAIL: post-publish plans read not parseable (see api-03-lago-plans.json)"; exit 1; }
+ case "$N" in ''|*[!0-9]*) say "FAIL: post-publish read not a count: '$N'"; exit 1 ;; esac


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:155-155 ───
[security · medium] LAGO_KEY（Lago 管理 API key）与 TOKEN_B（JWT）均以 curl argv 形式传递（本文件 3 处 LAGO_KEY + 2 处
TOKEN_B），本机其他用户/进程可经 ps 观测，与脚本自身 reg()/login() 注释 'a curl argv password is observable in local
ps，故密码走 STDIN' 的安全姿态不一致。可改用 -H @file（curl ≥7.55）从进程替换读取，避免凭据进 argv。

- curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
+ curl -s "http://127.0.0.1:48889/api/v1/plans" \
+   -H @<(printf 'Authorization: Bearer %s\n' "$LAGO_KEY") > "$EV/api-03-lago-plans-baseline.json"
+ # draft/publish 两处 -H "Authorization: Bearer $TOKEN_B" 同样改为 -H @<(printf ...) 形式


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:91-93 ───
[bug · low] 登录失败边界：后端返回非 JSON 错误体（如网关 502 错误页）时，set -e 会在下面第一个脱敏 jq 处以晦涩 parse error 提前中止，无法到达后续
'FAIL: login token/tenant missing' 的友好诊断（注释里承诺的 || true 落空路径实际不可达）。行为仍 fail-closed，仅诊断退化，建议在脱敏 jq
前先校验响应是合法 JSON。

  LOGIN_A=$(login "$PREFIX-a@verify.local") || true
  LOGIN_B=$(login "$PREFIX-b@verify.local") || true
+ for v in "$LOGIN_A" "$LOGIN_B"; do
+   jq -e . >/dev/null 2>&1 <<<"$v" \
+     || { say "FAIL: login response is not JSON (transport error / gateway error page?)"; exit 1; }
+ done
  jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:16-16 ───
[maintainability · low] BACKEND、Lago 端点 http://127.0.0.1:48889 与容器名 weknora-lago-82flow-db-1
全部硬编码且无环境变量覆盖，而同组浏览器脚本对 WEB 已采用 process.env.FLOW82_WEB ?? 默认值
的可配置机制；跨栈/跨端口复用时需改源码，与既有风格不一致。建议至少为后续轮次提供 env 覆盖入口。

- BACKEND=http://127.0.0.1:8093
+ BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"
+ LAGO_API="${FLOW82_LAGO_API:-http://127.0.0.1:48889}"
+ LAGO_DB_CONTAINER="${FLOW82_LAGO_DB_CONTAINER:-weknora-lago-82flow-db-1}"
+ # 后文 48889 与容器名字面量相应替换为 $LAGO_API / $LAGO_DB_CONTAINER


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:20-34 ───
[maintainability · low] r4-flow3 的 browser_01–04 四个脚本各自重复了约 30 行登录、note/RESULT 汇总与 catch/finally
截图样板，而父目录 _browser_lib.mjs 已提供 runLeg/login/note/evidencePath 且 r5-verify、r5b-flowcheck 的同名脚本均已
import 复用；browser_04 的注释（OCR84-R1-19）本身就记录了'复制含 catch
的源文件时被裁剪'曾引发回归，重复拷贝会持续放大该风险。本轮作为已冻结的证据产物可不回改，但后续轮次应统一改走 ../_browser_lib.mjs。

- const browser = await chromium.launch();
- let page;
- try {
-   page = await (await browser.newContext()).newPage();
-   page.setDefaultTimeout(45000);
-   page.on('request', (req) => {
-     if (req.method() === 'POST' && req.url().includes('/api/v1/commercial/purchases')) {
-       try { purchases.push(JSON.parse(req.postData() ?? '{}')); } catch { purchases.push({ unparseable: true }); }
-     }
+ import { login, note, runLeg, evidencePath } from '../_browser_lib.mjs';
+ // 与 r5-verify / r5b-flowcheck 同型：runLeg 统一承载 launch/close、note/RESULT
+ // 汇总与 (A-03) script-error + 截图契约，消除四份手写拷贝的裁剪回归面。
+ await runLeg(async (results, browser) => {
+   const page = await login(browser, { web: WEB, email: EMAIL, password: PASSWORD });
+   // ...leg-specific assertions...
-   });
+ });
-   await page.goto(`${WEB}/login`);
-   await page.fill('#auth-email', EMAIL);
-   await page.fill('#auth-password', PASSWORD);
-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
-   await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/diag_checkout.mjs:5-6 ───
[maintainability · low] 浏览器生命周期缺少 try/finally 兜底：login() 内部 waitForURL 有 30s 超时，凭据错误或
/platform/billing/checkout 打不开时抛错，脚本会以 unhandled rejection 退出，末尾的 browser.close() 不会执行，Chromium
进程泄漏（诊断脚本会被反复手动运行，泄漏会累积）。共享库 runLeg() 正是为此提供 launch 后 catch + finally best-effort close 的封装；若诊断脚本不需要
RESULT 契约，也建议至少用 try/finally 保证 close。

  const browser = await chromium.launch();
+ try {
- const page = await (await browser.newContext()).newPage();
+   const page = await (await browser.newContext()).newPage();
+   // ... 现有诊断逻辑 ...
+ } catch (err) {
+   console.error('[diag] failed:', err?.message ?? err);
+   process.exitCode = 1;
+ } finally {
+   await browser.close().catch(() => { /* best-effort close */ });
+ }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:203-204 ───
[bug · medium] [medium] 共享 Lago 栈上的计数增量断言存在跨轮竞态窗口，且本轮证据链不可独立复核：(1) baseline
读取与发布后读取之间，同栈（:48889）其他轮次并发发布 9900 plan 会替本轮满足 N>BASE_N（假阳性）；反之同 code weknora-pro-v1 的 republish
幂等落库时计数不增，本轮会误走 FAIL 分支（假阴性，fix-log C-11 已记录同型问题）；(2) 本轮提交的 api-03-lago-plans.json 中唯一 9900 plan 的
created_at=2026-09-25T03:07:32Z 早于 api-02-publish.json 的 published_at=2026-09-26T16:50:10Z 一天以上，且
api-03-lago-plans-baseline.json 未随轮提交（r5-verify/r5b-flowcheck 均已提交 baseline）——R1-08 要求的「重跑并提交真实产物 +
baseline」尚未落实，N>BASE_N 的结论无法复核。建议按 issue-84-r1 的先例改为 created_at > 本轮基线时间戳 的过滤（如 PUBLISHED_AT=$(jq
'[.plans[] | select(.created_at > "'$BASELINE_TS'")] | length')），或采用 C-11 的本轮 code+形状精确断言，并随轮提交
baseline 文件。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:41-45 ───
[maintainability · low] [low] 本轮 4 个 browser leg 各自复制了登录流程、note/results 记录、script-error 兜底与 RESULT
汇总约 30 行样板，而同目录 _browser_lib.mjs 正是为消除这类漂移而建（其注释明确 'one copy; a selector change now propagates to
every leg instead of drifting across 15 copies'）。目前该目录下顶层 3 个驱动、r5-verify 4 腿、r5b-flowcheck 5 个脚本均已
import 复用，唯 r4-flow2 这批新增腿仍是私有副本——登录选择器或流程变更时这 4 份副本会静默漂移，跨轮一致性仅靠注释 'Same assertions as round 1'
手工维持。建议改为 import { login, note, runLeg, envRequired, requirePlaywright } from '../_browser_lib.mjs'
复用共享实现（注意共享库 LOGIN.timeoutMs=30000 与本腿 45000 的差异，可由共享库参数化超时）。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:37-37 ───
[security · low] [low] 密码仍经 argv 暴露：脚本注释自述威胁模型包含 'a curl argv password is observable in local
ps'，curl 段也已改走 STDIN，但 jq -nc --arg p "$PW" 仍把密码放进 jq 进程 argv（ps 同样可见，login() 内同型），后续 TOKEN_B 经 curl
-H "Authorization: Bearer ..." 与 LAGO_KEY 的 argv 传递亦属同类缺口，防护只覆盖了链路一半。可用 jq 内置 env 对象改写为 jq -nc --arg
e "$1" '{username:$e,email:$e,password:$ENV.FLOW82_R4_PW}'，令凭据全程不经任何进程 argv。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs:40-40 ───
[test · low] [low] waitForTimeout(4500) 与前端 CheckoutPage 的 POLL_INTERVAL_MS=3000
存在隐式耦合：'no-awaiting-leftover' 负面断言必须跨过至少一个 poll tick 才有效（注释自己也承认 one poll tick 可残留旧 Status
行）。轮询间隔一旦调整（如改为 10s），4500ms 不再覆盖一个完整 tick，断言会在旧快照上间歇性误判失败，证据脚本重跑时产生 flaky。建议在等待时长处显式注释锚定的 3s 轮询值（或将
POLL_INTERVAL_MS 从前端导出为共享常量、由脚本按 tick 倍数计算等待时长）。



─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_04_active_face.mjs:3-4 ───
[bug · medium] 腿头注释宣称验证「wallet balance carries the purchase credit batch」、腿名标注 "AC1 final state +
credits"、截图命名为 rv5-05-active-credits.png，但腿内没有任何钱包/额度断言：仅断言了 billing 已生效、无 stale 文案、checkout
权益已生效与订单身份。经核实 BillingPage 根本不渲染钱包余额/额度信息（搜索 钱包|额度|credits|balance 无匹配），billing.innerText
里不可能出现可断言的额度数据。取证材料的审阅者会据此误以为「购买额度批次到账」已被浏览器腿证实——这是取证声明与实际断言的失配。建议二选一：(1) 在腿内补一条真实的额度断言（如经页面会话请求
/api/v1/commercial/summary 并校验额度字段，形状以契约为准）；(2) 修正注释与截图命名，去掉 credits 声明，明确本腿只覆盖 active 状态面。

- // purchase is active: the billing row shows 已生效 and the wallet balance
- // carries the purchase credit batch. The old awaiting/paid copy must be
+ // purchase is active: the billing row shows 已生效 (credits arrival is
+ // asserted out-of-band — the billing face renders no wallet data). The old
+ // awaiting/paid copy must be GONE.


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_04_active_face.mjs:42-42 ───
[maintainability · low] reload.click().catch(() => {}) 吞掉所有异常，且 click 受
page.setDefaultTimeout(45000) 影响：若 Reload 按钮改名/未渲染/禁用，每轮循环要等满 45s 超时才被吞，180s 窗口内仅能重试约 3 次；最终抛出的
'billing projection never flipped within 180s' 会把「按钮定位失败」误诊为「投影未翻转」，与真实根因南辕北辙。建议 click
用短超时，并在连续定位失败时给出指向按钮的诊断错误。

-       await reload.click().catch(() => { /* button not (yet) rendered — retry next tick */ });
+       try {
+         await reload.click({ timeout: 2_000 });
+       } catch {
+         // 首次点击失败即区分根因：按钮缺失是定位问题，不是投影未翻转
+         throw new Error('billing Reload button not clickable — cannot drive the projection refresh');
+       }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_01_checkout.mjs:28-28 ───
[maintainability · low] catch 注释「captured below」失真：并不存在后续的第二次捕获点。JSON.parse 失败（或 postData 为空）时
purchaseProvider 保持空串，checkout-submit-provider 断言失败的 detail 只会显示 provider=""——无法区分「请求体非
JSON」「postData 缺失」与「provider 字段值错误」。建议在 catch 中保留原始 body 片段供断言 detail 诊断。

-       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; } catch { /* captured below */ }
+       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; } catch { purchaseProvider = `(unparseable body: ${(req.postData() ?? '').slice(0, 80)})`; }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:96-97 ───
[maintainability · low] TENANT_A/TENANT_B 直接用 [ ... -eq ... ] 做数值比较，但缺少与 UID_B 的 uuid_shape
白名单对称的数字形状校验：若 active_tenant.id 非数字（响应形状变化），bash 会先向 stderr 打出 "[: integer expression expected"，随后
FAIL 消息把根因指向 "concurrent registration may have broken round isolation"——双重误导。本脚本对 UID_B
已建立纵深防御先例（uuid_shape），租户断言应保持同一风格。

+ tenant_shape() { [[ "$1" =~ ^[0-9]+$ ]]; }
+ tenant_shape "$TENANT_A" && tenant_shape "$TENANT_B" \
+   || { say "FAIL: active_tenant.id not numeric (A='$TENANT_A' B='$TENANT_B') — unexpected login response shape"; exit 1; }
  [ "$TENANT_A" -eq $((PAD + 1)) ] && [ "$TENANT_B" -eq $((PAD + 2)) ] \
    || { say "FAIL: protagonists on unexpected tenants A=$TENANT_A B=$TENANT_B (expected $((PAD+1))/$((PAD+2))) — concurrent registration may have broken round isolation"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:10-11 ───
[documentation · low] 头注释称「fresh db: pad 0 is enough for an isolated db」，但下方校验强制 FLOW82_PAD_COUNT >=
1（PAD 为 0 时直接 FAIL "must be >= 1"）。按注释设 0 的使用者会立即撞上与文档相矛盾的失败。注释与校验逻辑需统一：要么允许
0，要么修正注释说明下限及其原因（租户序号断言依赖占位数量）。

- # Registers FLOW82_PAD_COUNT placeholders (fresh db: pad 0 is enough for an
- # isolated db — the default keeps the r4 shape) then the two protagonists
+ # Registers FLOW82_PAD_COUNT placeholders (must be >= 1 — the tenant-serial
+ # assertion below counts them; the default 2 keeps the r4 shape) then the
+ # two protagonists


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:80-81 ───
[maintainability · low] login 失败路径绕过了本脚本自建的 say FAIL 诊断风格：reg 特意保留 "keep the 000 diagnosis
reachable"，但 login 后端不可达时 curl -s 输出为空，随后 jq 处理空输入（REDACTED 写文件或 .token 提取）会以 jq 原生 "parse error"
退出——set -e 下脚本确实非零退出，但错误信息既无 FAIL 前缀也不指向 backend 不可达这一根因。建议在消费 LOGIN_A/LOGIN_B 前先做非空校验。

  LOGIN_A=$(login "$PREFIX-a@verify.local") || true
  LOGIN_B=$(login "$PREFIX-b@verify.local") || true
+ [ -n "$LOGIN_A" ] && [ -n "$LOGIN_B" ] \
+   || { say "FAIL: empty login response (is the backend at $BACKEND reachable?)"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:153-154 ───
[maintainability · low] LAGO_KEY 的提取缺少 `|| true`：在 `set -euo pipefail` 下，docker exec
失败（容器未启动/名称不符）会使整条赋值以非零状态退出并被 set -e 直接中止——下一行精心准备的 `[ -n "$LAGO_KEY" ] || say "FAIL: no lago api
key readable..."` 在该路径上不可达，seed-run.txt 不会留下 FAIL 记录，只剩 docker 的裸 stderr。这正是 reg_expect
注释里已论证过的同型缺陷（"a connection-refused curl exits non-zero and set -e would otherwise abort BEFORE the
case below can report the diagnostic code"），该处却未按同型加固。行为仍 fail-closed，仅诊断退化，建议与 reg_expect 对齐补上 `||
true`，让显式 FAIL 分支覆盖 docker 失败形状。

- LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')
+ LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]') || true
  [ -n "$LAGO_KEY" ] || { say "FAIL: no lago api key readable from weknora-lago-82flow-db-1"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:170-174 ───
[maintainability · low] 传输层失败会绕过脚本自建的显式 FAIL 诊断：reg/login 路径已专门用 `|| true` + 显式 case 处理（注释自述『a
connection-refused curl exits non-zero and set -e would otherwise abort BEFORE the case below can
report the diagnostic code』），但 draft/publish 这两处 curl 管道、baseline 读取 `curl -s ... >
api-03-lago-plans-baseline.json`、以及 LAGO_KEY 的 `docker exec ... | tr` 都没有同等处理。在 `set -euo pipefail`
下，连接拒绝/容器未起会让脚本在到达 A-14 的 jq -e 断言或 『not parseable』 检查之前直接以 curl/docker 的裸退出码中止——seed-run.txt 里没有任何
FAIL 行，证据读者无法区分『断言失败』与『基础设施未起』（正是 A-03/A-13 注释要避免的形状）。建议对这几处补 `|| true`（或前置端口/容器探活），让失败落到既有的显式 FAIL
分支上，与 reg/login 的姿势保持一致。



─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_02_webhook.sh:8-10 ───
[bug · medium] EV 在 `cd "$(dirname "$0")/../../../.."` 之后才解析——这正是 seed.sh 本轮注释 OCR84-R1-18
明确修复的同型缺陷（seed.sh:19 与 replay_03_restart.sh:13 均已改为先解析再 cd）。当操作员在脚本目录内以 `./replay_02_webhook.sh`
相对调用时，cd 已切到 worktree 根，`dirname "$0"` 退化为 `.`，EV 静默变成仓库根，replay-02-webhook.txt
等证据文件全部落错位置。建议与同目录两个脚本对齐：先解析 EV，再基于 `$EV` 计算 cd 目标。

  set -euo pipefail
- cd "$(dirname "$0")/../../../.."
  EV="$(cd "$(dirname "$0")" && pwd)"
+ cd "$EV/../../../.."


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_04_active_face.mjs:31-32 ───
[bug · medium] detached 轮询存在 TOCTOU 竞态：`waitForSelector('text=已付款待激活', { state: 'detached' })`
在元素从未挂载时会立即成功。BillingPage 挂载后 `purchase` 初始为 null（购买状态后缀要等 purchaseStatus 首次拉取落地才渲染），而本腿只等了 `main`
出现——循环首轮大概率在 fetch 落地前执行，detached 因空 DOM 立即成立、首轮即 break：180s 轮询预算被整段跳过，一次 Reload
都没点。若此刻投影尚未翻转（恰是本循环设计要容忍的场景），后续 `waitForSelector('text=已生效', 45s)` 会因页面不再重取而在 45s 后以误导性的 leg-error
超时收场，与 OCR84-R1-33 注释声称的确定性相悖。建议进入循环前先等购买行渲染出任一状态后缀（确认首次 purchaseStatus 已落地），detached 判定才真正有意义。

    const reload = page.getByRole('button', { name: 'Reload' });
+   // 先等首次 purchaseStatus 落地：购买行渲染出任一状态后缀后 detached 判定才有
+   // 意义——pre-fetch 的空 DOM（purchase=null）会让 detached 立即成立、首轮即
+   // break，180s 轮询预算被整段跳过。
+   await page.waitForSelector(
+     'li:has-text("待付款"), li:has-text("已付款待激活"), li:has-text("已生效")',
+     { timeout: 45_000 },
+   );
    const deadline = Date.now() + 180_000;


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_02_webhook.sh:33-35 ───
[test · low] 头注释承诺「every projection must be byte-identical (no second payment, no wallet
change)」，但脚本只 dump 输出供人工比对，没有任何相等性断言——无论投影是否被重复投递污染都 exit 0。同组的 replay_03_restart.sh 本轮刚因「静默截断且无
FAIL 标记会产生假阳性 PASS」被加固（OCR84-R1-07②），本脚本存在同款弱点；`sleep 3` 也是兄弟脚本已弃用的固定等待。另外整轮
id（cus_VL2rl1L1V8yBYd、org/invoice UUID、容器名）硬编码，而 deliver_stripe_webhook.py 的 --org/--base 均有 env
fallback，与文件头「never hardcoded」的姿态不一致。建议至少对可机读的量做断言：投递前后 fulfillment 计数、payments succeeded 计数、wallet
余额任一变化即输出 FAIL 并以非零码退出。

+ # 投递前先取基线（与 replay_03 的 before/after 同款）
+ BEFORE=$(sqlite3 data/issue82-r5b-verify.db "select count(*) from commercial_fulfillment_records;")
+ # ……（投递 + sleep 3 改为短轮询后重读四对象）……
    echo "--- local fulfillment count ---"
-   sqlite3 data/issue82-r5b-verify.db "select count(*) from commercial_fulfillment_records;"
+   AFTER=$(sqlite3 data/issue82-r5b-verify.db "select count(*) from commercial_fulfillment_records;")
+   echo "fulfillment records: $BEFORE -> $AFTER"
  } | tee "$EV/replay-02-webhook.txt"
+ [ "$AFTER" = "$BEFORE" ] || { echo "FAIL: fulfillment count changed after duplicate webhook ($BEFORE -> $AFTER)"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_03_restart.sh:28-29 ───
[bug · low] `PID=$(lsof ... -t)` 无空值守卫：后端本就未监听 :8093 时，lsof 以退出码 1 结束且输出为空——该赋值位于 `{ ... } | tee`
管道内，在 `set -euo pipefail` 下会直接中止脚本，replay-03-restart.txt 截断在 before 行且无任何 FAIL 标记。这正是本文件头注释
OCR84-R1-07 刻意消灭的「静默失败」形态，建议补显式守卫。

-   PID=$(lsof -nP -iTCP:8093 -sTCP:LISTEN -t)
+   PID=$(lsof -nP -iTCP:8093 -sTCP:LISTEN -t) || true
+   if [ -z "$PID" ]; then
+     echo "FAIL: nothing listening on :8093 — backend not running" | tee -a "$EV/replay-03-restart.txt"
+     exit 1
+   fi
    echo "killing backend pid=$PID"


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:70-70 ───
[maintainability · medium] 内嵌 Python 对 lab.env 键直接下标访问（LAGO_ORG_USER_EMAIL /
LAGO_ORG_USER_PASSWORD），缺键时以裸 KeyError traceback 退出，且发生在 shell 层守卫（只检查了 LAGO_API_URL /
LAGO_ORG_API_KEY）之后、提供方注册之前——环境停留在半初始化状态且无可读诊断。这与脚本自身在 OCR84-R1-23①/R1-24 注释中声明的「显式守卫、skip≠pass
诊断纪律」相悖；run_lab.py:197-198 也以 .get(..., "") 默认空读取这两个键，说明缺键是被认可的输入面。建议补显式缺键守卫。

+ missing = [k for k in ("LAGO_API_URL", "LAGO_ORG_USER_EMAIL", "LAGO_ORG_USER_PASSWORD") if not env.get(k)]
+ if missing:
+     print("lab.env missing keys:", ",".join(missing), "(run lab.sh init)"); sys.exit(1)
  base, email, password = env["LAGO_API_URL"], env["LAGO_ORG_USER_EMAIL"], env["LAGO_ORG_USER_PASSWORD"]


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:104-104 ───
[maintainability · medium] REST 组织查询结果用 ["organization"]["lago_id"] 链式直接取值：Lago 返回 200 但响应形状漂移（或组织无
lago_id）时为 KeyError/TypeError 裸栈退出；且此处 urllib 对非 2xx 会先抛 HTTPError
同样无守卫。仓库既有先例（payment-activation/phases.py:147-150 的 _resolve_graphql_organization_id）对同一端点采用 ((body
or {}).get("organization") or {}).get("lago_id") 防御式解析并显式记录失败原因，建议对齐该写法，失败时打印错误摘要并以非零退出。

-     org_id = json.loads(r.read().decode())["organization"]["lago_id"]
+     org = (json.loads(r.read().decode()).get("organization") or {})
+     org_id = org.get("lago_id")
+ if not org_id:
+     print("organizations read: no lago_id in response"); sys.exit(1)


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:51-54 ───
[performance · low] 重试循环共 3 次（attempt=0,1,2），第三次（最后一次）OSError 失败后仍会执行 time.sleep(1.5 * (2 + 1)) 睡满
4.5 秒才退出循环并 raise SystemExit——末次失败后的退避延迟是纯浪费（脚本即将终止，不会再发起请求）。应仅在还有后续尝试时休眠。

          except OSError as e:
              last = e
+             if attempt < 2:
-             time.sleep(1.5 * (attempt + 1))
+                 time.sleep(1.5 * (attempt + 1))
      raise SystemExit(f"stripe {method} {path}: {last}")


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:90-93 ───
[maintainability · low] --org 兜底路径硬编码了 compose 项目名 weknora-lago-t11 与 env 文件路径
deploy/lago-lab/payment-settle-trigger/lab.env，而配套的 prepare_t9_env.sh 支持 <lab-dir>
参数自定义实验室目录（LAB_DIR 可变）。操作者以自定义目录初始化栈时，本兜底查询的是默认目录的栈/空结果，脚本以 "organization id unavailable"
中止，两脚本间存在参数化不一致。且这两个常量在 lab.sh/run_lab.py 中已有定义，此处为第 4/5 处重复。建议允许经环境变量（或 --lab-dir 参数）覆盖，与
prepare_t9_env.sh 的用法对齐。

          out = subprocess.run(
              ["docker", "compose", "-f", "deploy/lago/compose.yaml",
-              "--env-file", "deploy/lago-lab/payment-settle-trigger/lab.env",
-              "-p", "weknora-lago-t11", "exec", "-T", "db", "psql", "-U", "lago",
+              "--env-file", os.environ.get("T9_LAB_ENV", "deploy/lago-lab/payment-settle-trigger/lab.env"),
+              "-p", os.environ.get("T9_COMPOSE_PROJECT", "weknora-lago-t11"), "exec", "-T", "db", "psql", "-U", "lago",
+              "-tAc", "select id from organizations order by created_at limit 1"],
+             capture_output=True, text=True, timeout=30)


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:151-153 ───
[security · medium] 与安全验收条件"数据库查询一律参数绑定、禁止拼接组装 SQL"冲突：sql()（以及 act3/act5 内联的 docker psql
查询）直接执行模板字符串拼接的 SQL，助手本身不做任何校验，防护完全依赖各调用点自觉先过 sqlShape/uuidShape/入口正则。已逐一核对当前 18 条 SQL 的全部插值入口（TEN
入口正则、fulfilledOrder/orderID/mo/cus 经 Shape、u.tenant 内联
tenantShape），暂无遗漏；但新增调用点漏加白名单不会有任何报警，未验证值将直接进入查询，可能产出误导性证据或非预期查询结果。建议把白名单校验下沉到助手层：提供受控的查询构造器，只接受"常量片
段 + 已过白名单的值"交替序列（或 tagged template 内强制每个插值先过 sqlShape），使漏校验在构造期即失败，而非依赖调用点纪律。

- function sql(query) {
-   return execFileSync('sqlite3', [DB_PATH, query], { encoding: 'utf8', cwd: fileURLToPath(new URL('../../..', import.meta.url)) }).trim();
+ // 受控构造器：插值只能以已过白名单的 Validated 值传入，裸字符串拼接不再可能
+ function sqlQuery(strings, ...validated) {
+   return strings.reduce((q, s, i) => q + (i ? validated[i - 1].value : '') + s);
  }
+ const V = (name, value, re) => { if (!re.test(value)) throw new Error(`bad ${name}`); return { value }; };
+ // 用法：sql(sqlQuery`select ... where id='${V('order id', id, /^ord_[0-9a-f]+$/)}'`)


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:24-25 ───
[maintainability · low] 这段 docker exec 与其后的 seed-login-f.json readFileSync 均位于顶层 try 守护之外：docker
未运行、容器不存在或文件缺失时脚本同步裸抛，末尾的 files 落盘循环与 RESULT 汇总都不会执行，只剩一段栈回溯——与脚本 (C-04) 注释"任何中止仍写出可诊断证据"的自我承诺不一致（且与
FLOW83_PW/FLOW83_TOKEN 缺失时 console.error + exit 2 的既有风格不统一）。建议给这两步补显式 try/catch 转 console.error +
process.exit(2)，或挪进大 try 由顶层 catch 统一收口。



─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:180-181 ───
[maintainability · medium] 基础设施地址硬编码且参数化不一致：ORG UUID 此处与 browser_flow_83.mjs 各一份；Lago base
'http://127.0.0.1:48889' 在本脚本 deliverWebhook(行164)、act5 lagoAPI(行365) 及 browser 脚本轮询共 3 处内联；容器名
'weknora-lago-82flow-db-1' 两脚本共 5 处——而 BACKEND/STUB/DB_PATH/TENANT 均有 env 覆盖，唯独 Lago
栈三者没有。仓库的回放惯例是复制目录换端口重跑（82 目录的 r4/r5 变体即是），换栈时漏改任一处会打到错误栈上产出错误证据。建议集中为顶部常量并补
FLOW83_LAGO_BASE/FLOW83_ORG/FLOW83_LAGO_CONTAINER env 默认值（9900/10900000 属断言期望值，保留即可）。



─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:237-237 ───
[maintainability · medium] 与 api_recovery_83.mjs 同款硬编码：ORG UUID、Lago base
'http://127.0.0.1:48889'、容器名 'weknora-lago-82flow-db-1' 在两脚本间各自内联（本脚本的 WEB/STUB/BACKEND 均已 env 化，唯独
Lago 栈没有）。建议两脚本共享同一组顶部常量/env 默认值，避免换栈重放时双份漂移。



─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:239-241 ───
[maintainability · low] act3 内 `const tokenF = TOKEN_MAIN;` 声明后从未被引用（实际消费点是 act5 行 388-389
的同名变量），属残留死代码；它的存在还暗示本幕原计划有 API 侧断言却未落地，干扰阅读。建议删除此行。



─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:202-206 ───
[maintainability · low] stubOrders/stubMark/stubNotify 封装、NOTPAY/9900 待付单过滤、lagoCustomer
查询、deliverWebhook 调用在 browser_flow_83.mjs 与 api_recovery_83.mjs 之间整段复制，且本轮 82 目录已新增 _browser_lib.mjs
共享库而 83 未复用。两侧对过滤条件或 mark 语义的后续修复（如 C-03 这类变更）需双份同步，极易行为漂移。建议仿照 _browser_lib.mjs 抽出共享模块（stub 客户端 +
待付单过滤 + lagoCustomer/deliverWebhook）供两脚本 import。



─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:289-291 ───
[test · low] act4a 的 wechat-order-landed 断言 ok 硬编码为 true——与 act2 同名断言（行 213）携带的 `(checkout_url ??
'').startsWith('weixin://')` 真实检查形成对比。purchaseUntilLanded 只保证 201 + order.id，并不校验渠道；恒真条目在 RESULT
汇总里稀释判别力，也掩盖了"落地的是 wechat 渠道单"这层语义。建议复用 act2 的 checkout_url 前缀检查。

      const first = await purchaseUntilLanded(u.token, 'wechat', log);
      const orderID = orderShape(first.order?.id ?? '');
-     note(act, 'wechat-order-landed', true, `order=${orderID}`);
+     note(act, 'wechat-order-landed', (first.order?.checkout_url ?? '').startsWith('weixin://'), `order=${orderID}`);


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:132-134 ───
[maintainability · low] page 提升到 try 之外，但 catch 与 finally 均未引用它：流程中途失败时没有失败现场截图，外层声明本身也无必要。二选一：改为
try 内 const 声明；或更有价值地，在 catch 里补一张失败截图（page?.screenshot(...).catch(() => {})），让 (C-05)
注释承诺的"不留无声崩溃"的证据链更完整。



─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:95-98 ───
[maintainability · low] `subprocess.run` 以 `capture_output=True` 捕获了 stderr 却从未输出：docker compose
exec 失败（db 容器未就绪、compose 文件路径错误等）时 stdout 为空，脚本只报 "organization id unavailable"，真实的 docker
错误信息被静默吞掉，与配套脚本 prepare_t9_env.sh 声明的「skip≠pass 诊断纪律」不符。建议在 returncode 非零时回显 stderr 尾部再退出。

              capture_output=True, text=True, timeout=30)
+         if out.returncode != 0:
+             raise SystemExit(f"docker compose exec failed: {(out.stderr or '')[-500:]}")
          args.org = (out.stdout or "").strip()
      if not args.org:
          raise SystemExit("organization id unavailable")


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:115-117 ───
[maintainability · low] 最终向 Lago webhook 端点的 POST 未捕获 HTTPError：签名不符（401）、org/code 不匹配（404）或 Lago
未就绪（5xx）时，urllib 直接抛裸 HTTPError traceback。同文件 `stripe()` 帮助函数已示范了捕获 HTTPError 并返回 code+body
的模式，此处作为脚本的最后一环（交付成败即任务成败）反而无守卫，丢失了 Lago 返回的错误体上下文。

      opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
+     try:
-     with opener.open(req, timeout=30) as r:
+         with opener.open(req, timeout=30) as r:
-         print(f"delivered event {event['id']} for intent {intent['id']} -> HTTP {r.status}")
+             print(f"delivered event {event['id']} for intent {intent['id']} -> HTTP {r.status}")
+     except urllib.error.HTTPError as e:
+         raise SystemExit(f"webhook delivery HTTP {e.code}: {e.read().decode()[:400]}")


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:78-79 ───
[maintainability · low] `stripe()` 特意捕获 HTTPError 并解析出 JSON body（其中含 Stripe 的 error.message，如
"Invalid API Key provided"），但 main 里非 200 时只打印状态码、丢弃了 body——凭据失效或限流时操作者只能看到一个裸的 HTTP 状态码，无法定位原因。建议把
body 摘要并入退出消息。

      if status != 200:
-         raise SystemExit(f"intent list HTTP {status}")
+         raise SystemExit(f"intent list HTTP {status}: {str(body)[:400]}")


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:177-179 ───
[maintainability · low] 脚本在 OCR84-R1-23① 注释中声明「每处追加 || true，让显式守卫接管失败路径」，但 `SECRET=$(python3 ...)`
这处纯赋值漏加了：set -euo pipefail 下命令替换失败（如 mint 过程中 Stripe 网络错误抛 HTTPError）会直接 errexit，下方 `[ -z "$SECRET"
]` 的 "secret mint failed" 守卫永远不可达，且此时只留下 python traceback 而非预期的守卫消息。应与 `_t9_base`/`_t9_org` 保持一致，追加
`|| true` 让显式守卫接管失败路径。

-   )
+   ) || true
    if [ -z "$SECRET" ]; then
      echo "secret mint failed" >&2


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_01_checkout.mjs:46-48 ───
[bug · high] 断言文案已过时，会在本 PR 自身的前端构建上确定性失败：本次更新把 CheckoutPage 的待付款文案统一为共享词表（purchaseStateMessage 优先
PURCHASE_STATE_LABEL，awaiting_payment → 「待付款（权益未开放）」），旧的私有文案「待付款（权益未开通）」只剩 purchase 缺席/absent
时的渠道兜底。而自动提交路径在同一个 ready setState 里就带上了 purchase（POST /purchases 响应经 purchaseWire 必含
state=awaiting_payment），首屏即渲染「未开放」——本腿从头到尾不会出现「未开通」，checkout-awaiting-payment 断言必然 FAIL（对齐 billing
断言应改用「未开放」；r5-verify README 记录的 7/7 PASS 只能是词表统一前的构建跑出来的，取证已与合并态不一致）。

    note(results, 'checkout-awaiting-payment',
-     body.includes('等待付款') && body.includes('待付款（权益未开通）'),
-     'order status: 等待付款 + 待付款（权益未开通）');
+     body.includes('等待付款') && body.includes('待付款（权益未开放）'),
+     'order status: 等待付款 + 待付款（权益未开放）');


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_02_sync_face.mjs:40-41 ───
[bug · high] 两处断言仍在找已退役的兜底文案「待付款（权益未开通）」。本 PR 的 CheckoutPage 在深链加载 getOrder 后会跟进 purchaseStatus（新增的
void client.commercial.purchaseStatus(...)），一旦返回
awaiting_payment，三态面即翻成共享词表「待付款（权益未开放）」：首个断言（sync-face-no-advance）只能在 purchaseStatus
落地前的瞬态窗口靠竞态通过；而本处刷新断言读取 body 时初始加载的 purchaseStatus 早已落地，页面标签已是「未开放」——确定性 FAIL。应改为断言「待付款（权益未开放）」（与
billing 断言同源），两处一起改。

-   note(results, 'sync-face-refresh-still-awaiting', body.includes('等待付款') && body.includes('待付款（权益未开通）'),
+   note(results, 'sync-face-refresh-still-awaiting', body.includes('等待付款') && body.includes('待付款（权益未开放）'),
      'after an explicit refresh the order is STILL awaiting payment');


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:127-127 ───
[maintainability · medium] Lago baseline 读取缺少失败诊断，且会污染证据文件：Lago 不可达时 curl -s 抑制错误信息、set -e 静默退出（无任何
FAIL 输出）；若返回 401/HTML 错误体，则会原样写入 api-03-lago-plans-baseline.json（取证文件被错误体覆盖），随后 jq 的 `.plans[]` 在
null 上报原生 "Cannot iterate over null"——与脚本自建的 pre-flight/FAIL 风格（如 LAGO_KEY 的非空守卫）不一致。建议捕获 HTTP 码并在非
2xx 时给出指向根因的 FAIL。

- curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
+ if ! curl -sf "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" -o "$EV/api-03-lago-plans-baseline.json"; then
+   say "FAIL: lago plans baseline read failed (is :48889 reachable / api key valid?)"; exit 1
+ fi


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:142-143 ───
[security · medium] publish 收据断言把 $PLAN_KEY/$DRAFT_V 直接内插进 jq 程序字符串，与五行上方 draft 断言已采用的 --arg
绑定（--arg k "$PLAN_KEY"）自相矛盾。PLAN_KEY 含双引号时轻则 jq 语法错误被误报为 "receipt mismatch"，重则可构造恒真表达式（如
PLAN_KEY='a" or true or "b' 使 == 比较被 or true 短路）让校验恒
PASS——对一个以可信断言为产物的取证脚本是验证完整性缺口，也不符合本分支「外部输入一律参数绑定、不拼接查询」的纪律。建议改用 --arg 绑定。

- jq -e ".data.receipt.command_key == \"publish_plan_version:$PLAN_KEY:$DRAFT_V\"" "$EV/api-02-publish.json" >/dev/null \
+ jq -e --arg ck "publish_plan_version:$PLAN_KEY:$DRAFT_V" '.data.receipt.command_key == $ck' "$EV/api-02-publish.json" >/dev/null \
    || { say "FAIL: publish receipt mismatch"; exit 1; }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:385-387 ───
[test · medium] 断言 `walletsAny.includes('9.9')` 是对整段 psql 输出（w.name|wt.status|wt.amount
多行拼接）的子串匹配：金额为 19.9、99.9、9.95 等错误值，或任意行任意列（钱包名、status）中恰好出现 "9.9" 都会误判 PASS；且 detail 声称 "status 1 =
settled" 但断言并未校验 status 列，也未校验行数为 1。作为四对象证据之一，这会漏掉发放金额错误/未结算的钱包事实。建议按列解析后精确比较金额与 status。

-     const walletsAny = lago(`select w.name, wt.status, wt.amount from wallet_transactions wt join wallets w on w.id=wt.wallet_id join customers c on c.id=w.customer_id where c.external_id='weknora-tenant-${TEN}' and w.name like '%purchase%'`);
-     log.push(`purchase wallets: ${walletsAny.replace(/\n/g, ' ; ')}`);
-     note(act, 'purchase-wallet-granted', walletsAny.includes('9.9'), 'the purchase wallet granted the 9.90 first period (status 1 = settled)');
+     const rows = walletsAny.split('\n').filter(Boolean).map((r) => r.split('|'));
+     note(act, 'purchase-wallet-granted',
+       rows.length === 1 && rows[0][2] === '9.9' && rows[0][1] === '1',
+       `the purchase wallet granted the 9.90 first period settled (rows=${rows.length})`);


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:395-395 ───
[test · medium] `credits.includes('10900000')` 同样是对 JSON.stringify 结果的子串匹配：余额为 109000009、210900000
等错误值时子串仍命中，误发放/漏发放的 credits 错误不会被该证据捕获。建议直接读取 balance_micro 字段做严格相等比较。

-     note(act, 'product-credits-feature', credits.includes('10900000'), `credits face carries the 1.0 base + 9.9 purchase balance: ${credits.slice(0, 80)}`);
+     const bal = acct.json?.data?.benefits?.credits?.balance_micro ?? '';
+     note(act, 'product-credits-feature', bal === '10900000', `credits face carries the 1.0 base + 9.9 purchase balance: ${bal}`);


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:67-71 ───
[test · low] quote() 未校验返回结构即访问 `json.data.id`：当 quotes 返回错误信封（{error:...}，如 4xx/5xx）时以裸
TypeError（Cannot read properties of undefined）中断，且该异常发生在 for 循环体内，会直接绕过 purchaseUntilLanded
余下的重试与终结日志——与脚本 (C-04)「答案先校验、失败带上下文终止」的自我约定不一致。建议校验后再返回。

  async function quote(token) {
    const { json } = await api('POST', '/api/v1/commercial/quotes', token,
      { plan_key: 'pro', plan_version: 1, subscription_version: 0 });
+   if (!json?.data?.id) throw new Error(`quote: no data.id in answer: ${JSON.stringify(json).slice(0, 200)}`);
    return json.data.id;
  }


─── docs/plans/issue-72-flow-evidence-84/run_84.sh:20-23 ───
[documentation · low] 端口预检的提示文案与实际行为不一致：echo 宣称「被占整体 +10 顺延并同步脚本
env」，但循环实现是任一端口（8096/5197/8298/8299）被占即 exit
1，并无任何顺延逻辑。复验者按文案预期「换端口继续」，实际会直接失败终止，容易误判失败原因。建议二选一：修正文案为「被占即失败，需操作者手动改端口后重跑」，或真正实现顺延逻辑。

- echo "== 0. 端口预检（8096/5197/8298/8299 空闲；被占整体 +10 顺延并同步脚本 env）=="
+ echo "== 0. 端口预检（8096/5197/8298/8299 空闲；任一被占即失败退出，需手动换端口并同步各脚本 env）=="
  for p in 8096 5197 8298 8299; do
    lsof -iTCP:$p -sTCP:LISTEN >/dev/null 2>&1 && { echo "port $p occupied"; exit 1; }
  done


─── docs/plans/issue-72-flow-evidence-84/run_84.sh:47-49 ───
[bug · low] 固定 sleep 25 判定后端就绪不可靠：Step 4 以 go run ./cmd/server
冷启动，项目文档（issue-72-plan-82.md）自述「首次编译数分钟」，冷缓存/低配机下 25s 必然不足。届时后端尚未监听 :8096，seed_84.sh 首个注册即以
TRANSPORT failure 整体退出，且无重试/轮询边界处理，直接阻断复验。脚本第 4 步 echo 已给出就绪判据（login 应答 4xx 即活），建议据此实现轮询等待而非固定
sleep（此问题在 issue-72-ocr-issue-84-r1.md 中已被记录为 bug·low，本次仍未修复）。

  echo "== 6. 种子（pad + 主角 + pro 9900 发布 + Lago 断言）=="
- sleep 25
+ for i in $(seq 1 120); do
+   code=$(curl -s -o /dev/null -w '%{http_code}' -X POST http://127.0.0.1:8096/api/v1/auth/login \
+     -H 'Content-Type: application/json' -d '{}' || true)
+   case "$code" in 4*|5*) echo "backend ready after ~$((i*2))s (HTTP $code)"; break ;; esac
+   sleep 2
+ done
  bash docs/plans/issue-72-flow-evidence-84/seed_84.sh


─── docs/plans/issue-72-flow-evidence-84/up_stubs_84.sh:31-35 ───
[test · low] 冒烟检查只打印 HTTP 状态码而不做任何断言：注释写明「expect 401」但无校验，stub 进程仅 sleep 2 即测（nohup
启动慢时未就绪），两种情况下都会静默通过，冒烟失去拦截意义（对比第 2 步 --selftest 的进程内硬断言）。另外 pkill -f 'alipay_gateway_stub.py'
按脚本名模式清理，会连带杀死 82/83 轮并行运行的同类 stub（跨轮次副作用），与注释中「避开 :8291-8297 既往占用」的并行隔离意图不符。建议捕获状态码并校验（非预期即 exit
1），清理改为记录本脚本启动的 pid 或按端口精确匹配。

  sleep 2
- echo '--- smoke: wechat anomaly stub rejects unsigned native create (expect 401) ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8298/v3/pay/transactions/native -d '{}'
- echo '--- smoke: alipay stub answers precreate-unauth ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8299/gateway.do -d 'service=alipay.trade.precreate'
+ code=$(curl -s -o /dev/null -w '%{http_code}' -X POST 127.0.0.1:8298/v3/pay/transactions/native -d '{}')
+ echo "wechat anomaly stub unsigned create -> $code (expect 401)"
+ [ "$code" = 401 ] || { echo 'FAIL: wechat stub smoke mismatch'; exit 1; }
+ code=$(curl -s -o /dev/null -w '%{http_code}' -X POST 127.0.0.1:8299/gateway.do -d 'service=alipay.trade.precreate')
+ echo "alipay stub precreate-unauth -> $code"
+ case "$code" in 4*|5*) ;; *) echo 'FAIL: alipay stub smoke mismatch'; exit 1 ;; esac


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:113-117 ───
[test · medium] C-12 传输层防护不完整：脚本在 reg_expect、LOGIN_A/B、LAGO_KEY 三处已按注释加固 curl 传输失败路径，但本处 plan
draft、随后的 publish 及 api-03-lago-plans 拉取共三个 curl 仍无 `|| { say FAIL ...; exit 1; }` 防护。后端或 Lago 不可达时
curl 以非零退出（如 exit 7）直接触发 set -e，脚本零输出静默终止——seed-run.txt 无 FAIL 记录即中断，承诺的 HTTP-status FAIL
路径对传输层不可达，与本脚本自身声明的 fail-loudly 标准不一致，削弱证据链可审计性。

  curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts" \
    -H "Authorization: Bearer $TOKEN_B" -H 'Content-Type: application/json' -d '{
      "plan_key":"pro","name":"Pro","amount_fen":9900,"currency":"CNY",
      "included_credits_micro":9900000,"features":{"advanced_models":true},
-     "limits":{},"charges":[]}' > "$EV/api-01-draft.json"
+     "limits":{},"charges":[]}' > "$EV/api-01-draft.json" \
+   || { say "FAIL: plan draft TRANSPORT failure (curl exit $? — backend at $BACKEND reachable?)"; exit 1; }
+ # publish 与 api-03-lago-plans 两处 curl 同样追加 || { say ...; exit 1; }


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:43-44 ───
[test · low] FLOW83_PAD_COUNT 未做整数校验即进入 seq，传错值（如 "38 " 或 "3x"）只得到晦涩的 seq 报错；对照下方
FLOW83_LAGO_OCCUPIED_MAX 已有同型校验。README 记录 v2 轮实际以 FLOW83_PAD_COUNT=38 运行，属真实外部输入面。另
TENANT_A/TENANT_B 从登录响应取出后仅查了非 null 即进入 `[ -gt ]` 数值比较，id 形态异常时得到 bash "integer expression expected"
而非明确 FAIL——与本脚本 fail-loudly 风格不一致，建议以同型 case 校验后再生效。

  PAD="${FLOW83_PAD_COUNT:-14}"
+ case "$PAD" in ''|*[!0-9]*) say "FAIL: FLOW83_PAD_COUNT must be an integer, got '$PAD'"; exit 1 ;; esac
  for i in $(seq 1 "$PAD"); do


─── docs/plans/issue-72-flow-evidence-83/v2_up_backend.sh:20-21 ───
[test · medium] v2 库路径与同轮复用脚本的默认值错配：本脚本把后端指向 data/issue83-flow-v2.db，但 seed_83.sh 的 FLOW83_DB 与
api_recovery_83.mjs 的 DB_PATH 默认仍是 data/issue83-flow.db。忘设 FLOW83_DB 时 seed 会把 commercial_grants
授权种子写进 v1 旧库，本脚本启动的 v2 后端读不到 plan_publish 授权，publish 将以误导性的 "receipt mismatch" 失败；api 腿则对旧库跑全部 SQL
断言，根因难辨。README 仅在文档层记录了参数化，脚本侧无任何提示或护栏（docs/plans/issue-72-ocr-issue-84-r1.md:1811
已提出该问题，仍未落地）。建议启动时打印/校验配套 env，或提供统一注入 FLOW83_DB 等变量的 v2 runner。

  export SERVER_PORT=8095 SERVER_HOST=127.0.0.1
  export DB_DRIVER=sqlite DB_PATH=data/issue83-flow-v2.db
+ echo 'companion env for seed_83.sh / api_recovery_83.mjs (required this round):'
+ echo "  FLOW83_DB=$DB_PATH FLOW83_BACKEND=http://127.0.0.1:8095 FLOW83_STUB=http://127.0.0.1:8296 FLOW83_PAD_COUNT=<past occupied Lago tenants>"


─── docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh:8-10 ───
[security · low] 两个问题：1) 生成的商户/平台私钥 PEM 与 apiv3.key 未设 umask 077，按调用进程默认 umask（常见 022）以 0644 落盘，与
v2_up_backend.sh 强调的"密钥只经运行时 0600 env
文件注入"纪律不一致，多用户主机上可被其他本地账户读取（docs/plans/issue-72-ocr-issue-83-r1.md:193 已提出该建议，未落地）；2)
`${TMPDIR}issue83-v2-keys` 缺 `/` 分隔符且无默认值——Linux TMPDIR=/tmp 时解析为 /tmpissue83-v2-keys（根目录，mkdir
多半失败），TMPDIR 未设时 set -u 直接退出，仅 macOS launchd TMPDIR（带尾 `/`）侥幸正确。

  set -euo pipefail
- KEYS="${1:-${TMPDIR}issue83-v2-keys}"
+ umask 077   # 私钥与 apiv3.key 以 0600 落盘，兑现运行时凭据纪律
+ KEYS="${1:-${TMPDIR:-/tmp}/issue83-v2-keys}"
  mkdir -p "$KEYS"


─── docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh:13-15 ───
[maintainability · low] pkill -f 以裸文件名匹配任意进程命令行：同机任何命令行含该文件名的无关进程（grep、tail -f
日志、编辑器打开脚本）都会被误杀（SIGTERM）。建议匹配更完整的启动形态（含 python3 前缀），或由脚本自管 pid 文件做定点清理。

- pkill -f 'wechat_native_stub.py' 2>/dev/null || true
- pkill -f 'alipay_gateway_stub.py' 2>/dev/null || true
+ pkill -f "python3 .*wechat_native_stub\.py" 2>/dev/null || true
+ pkill -f "python3 .*alipay_gateway_stub\.py" 2>/dev/null || true
  sleep 1


─── docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh:32-33 ───
[test · low] 冒烟检查只回显 HTTP 码，"expect 401" 仅存在于提示文本、未做任何断言；且固定 sleep 2 后立即探测，stub 慢启动时要么 curl 连接拒绝触发
set -e 静默退出（无 FAIL 信息），要么拿到非预期状态码后照常继续。建议捕获状态码并断言（微信 stub 未签名请求应为 401），连接层失败以 000 兜底参与断言。

  echo '--- smoke: wechat stub rejects unsigned native create (expect 401) ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8296/v3/pay/transactions/native -d '{}'
+ SMOKE=$(curl -s -o /dev/null -w '%{http_code}' -X POST 127.0.0.1:8296/v3/pay/transactions/native -d '{}' || echo 000)
+ [ "$SMOKE" = 401 ] || { echo "FAIL: wechat stub smoke expected 401, got $SMOKE"; exit 1; }


─── docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh:26-29 ───
[bug · low] 两处日志路径 `${TMPDIR}issue83-*.log` 缺 `/` 分隔符：api_recovery_83.mjs 以
`tmpdir()/issue83-*-stub.log` 读取（该文件 155/158 行），Linux TMPDIR=/tmp 时本脚本写
/tmpissue83-*-stub.log（根目录、多半写失败），与读取路径错开导致取证侧 catch 后拿到空串、回调证据静默丢失；TMPDIR 未设时 set -u 直接退出。仅 macOS
launchd TMPDIR（带尾 `/`）两侧才解析一致（docs/plans/issue-72-ocr-issue-83-r1.md:203 已提出，未落地；wechat
行同步修复）。另注意跨目录复用 ../issue-72-flow-evidence-82/alipay_gateway_stub.py 并以 FLOW82_* 前缀传参（名称已核对与上游 59/62
行消费一致），上游文件移动或 env 改名即静默断链，nohup 启动失败仅体现在日志文件——冒烟 curl 是唯一兜底但无 FAIL 信息。

  FLOW82_ALIPAY_PORT=8297 \
  FLOW82_KEY_DIR="$KEYS" \
-   nohup python3 "$EV/../issue-72-flow-evidence-82/alipay_gateway_stub.py" >> "${TMPDIR}issue83-alipay-stub.log" 2>&1 &
+   nohup python3 "$EV/../issue-72-flow-evidence-82/alipay_gateway_stub.py" >> "${TMPDIR:-/tmp}/issue83-alipay-stub.log" 2>&1 &
  echo "alipay stub pid $!"


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:135-135 ───
[bug · medium] `jq ... | say "lago 9900 plans: $(cat)"` 的求值顺序使该行完全失效:参数 `$(cat)` 在管道建立之前、于当前 shell
中展开,读取的是脚本自身的 stdin(而不是 jq 的管道输出);同时 `say()` 是 `echo "$@" | tee`,从不读自己的 stdin,因此 jq
通过管道送来的列表被整体丢弃。后果:前台手动运行时 `$(cat)` 阻塞等待终端 EOF,脚本在此挂死;后台/detached 运行(stdin 为 null)时打印空后缀,9900
计划诊断列表永远到不了 seed-run.txt。应改为在命令替换内直接调用 jq。

- jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json" | say "lago 9900 plans: $(cat)"
+ say "lago 9900 plans: $(jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json")"


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:69-70 ───
[bug · low] TOKEN_A/TOKEN_B(以及下方 UID_B/TENANT_A/TENANT_B)的 jq 提取没有 `|| { say FAIL; exit 1; }`
兜底:若登录接口返回 200 但正文不是 JSON(反向代理错误页、空响应等),jq 以 exit 2 失败,set -e 直接静默终止,seed-run.txt 无任何 FAIL
记录——正是本脚本在各 curl/api 调用处反复加固的 silent-death 路径。建议为这几处 jq 提取补齐与 LOGIN_A 同型的 fail-loudly 兜底。

- TOKEN_A=$(jq -r .token <<<"$LOGIN_A")
- TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
+ TOKEN_A=$(jq -r .token <<<"$LOGIN_A") || { say "FAIL: login A response not JSON"; exit 1; }
+ TOKEN_B=$(jq -r .token <<<"$LOGIN_B") || { say "FAIL: login B response not JSON"; exit 1; }


─── docs/plans/issue-72-flow-evidence-83/v2_probe.sh:8-8 ───
[bug · low] `read -r L < <(docker exec ...)` 在被探测的故障场景下会提前杀死探测:进程替换中的 docker exec 失败或 api_keys
为空时输出为空流,`read` 读到 EOF 返回 1,set -e 使脚本静默退出——step3-lagolen、step4-cwd 与 `which go docker python3`
全部不再输出,而这正是该探测脚本(run_in_background 差异定位)存在的意义;且退出码 1 无法与其它错误区分。建议改用命令替换并以 `|| true` 容忍失败,让 lagolen=0
打印出来后继续后续步骤。

- read -r L < <(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d "[:space:]")
+ L=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d "[:space:]") || L=""


─── internal/modules/commercial/commercialplatform/lago_settlement.go:298-298 ───
[bug · medium] 列表页 limit=100 与 providerOutboundRequest 的 64KiB 响应体上限不匹配：Stripe PaymentIntent
列表行序列化后通常约 0.7~1.5KB（含 payment_method_options/metadata 等），一页 ~50-90 条即可超过 65536 字节。一旦该客户的 intent
累积超过阈值（失败 3DS 重试的残留兄弟、续期扣款 intent 等都会累积，F-1 证据本身表明残留兄弟是真实形态），每次列表读都会命中 "provider response body
exceeds … bytes" 的 ErrPlatformInvalidResponse（definitive），settle 轨道对该客户永久失败并入
attention——而这次列表读正是唯一的定位手段。建议把 limit 降到 ~25（25×~1.5KB 远低于 64KiB），并相应调大 stripeListPageBudget 以保持
~1000 条的总覆盖。

- 		path := "/v1/payment_intents?customer=" + url.QueryEscape(providerCustomerID) + "&limit=100"
+ 		path := "/v1/payment_intents?customer=" + url.QueryEscape(providerCustomerID) + "&limit=25"


─── internal/modules/commercial/commercialplatform/lago.go:227-231 ───
[bug · medium] parseRFC3339UTC 把缺失/不可解析的 expiration_at、created_at 静默映射为零值时间，而 WalletRank 按
(ExpiresAt ASC, GrantedAt ASC) 排序，零值会排到最前——一个时间戳异常的钱包会拿到消费优先级 1，并把其余全部钱包的优先级顺次推低，且每次 rebalance
都确定性地复现（不会自愈）。这与本变更自己声明的 fail-closed 纪律矛盾（OCR84-R1-09：域外 rank、重复 ref 都会响亮失败；r2:624：无法排序的行按畸形数据
fail-closed），lagoWallet.ExpirationAt 为 null 时 JSON 反序列化为 "" 正是可达输入。建议在构造 rank 输入时对不可解析时间戳直接返回
ErrPlatformInvalidResponse。

+ 		expiry, perr := time.Parse(time.RFC3339, w.ExpirationAt)
+ 		if perr != nil {
+ 			return commercial.CommandReceipt{}, fmt.Errorf("%w: wallet %q expiration_at unparsable", commercial.ErrPlatformInvalidResponse, w.Name)
+ 		}
+ 		granted, gerr := time.Parse(time.RFC3339, w.CreatedAt)
+ 		if gerr != nil {
+ 			return commercial.CommandReceipt{}, fmt.Errorf("%w: wallet %q created_at unparsable", commercial.ErrPlatformInvalidResponse, w.Name)
+ 		}
  		inputs = append(inputs, commercial.WalletRankInput{
  			WalletRef: w.Name,
- 			ExpiresAt: parseRFC3339UTC(w.ExpirationAt),
- 			GrantedAt: parseRFC3339UTC(w.CreatedAt),
+ 			ExpiresAt: expiry.UTC(),
+ 			GrantedAt: granted.UTC(),
  		})


─── internal/modules/commercial/commercialplatform/lago.go:252-255 ───
[bug · low] 这个新写的 PUT 状态分支漏掉了本变更集在所有其它路径（readSubscriptionByIdentity 的 D13
注释、waitForPaymentMethodSync、customerProviderBound、发票读取、classifyStripeStatus 等）统一引入的
429→ErrPlatformUnreachable 拆分：钱包优先级 PUT 被限流（429）时会落入 default 分支，被归类为 definitive 的
invalid_response，与文件内 "the transient split every other read path already applies"
的自述不一致。当前调用方（benefits 刷新）只 Warn、下轮重收敛，影响有限，但建议补齐拆分以免未来调用方继承错误分类。

  		switch {
  		case status >= 200 && status < 300:
- 		case status >= 500:
+ 		case status == http.StatusTooManyRequests || status >= 500:
  			return commercial.CommandReceipt{}, fmt.Errorf("%w: wallet priority update unavailable", commercial.ErrPlatformUnreachable)


─── internal/modules/commercial/commercialplatform/lago_settlement.go:107-115 ───
[maintainability · low] 这里对同一资源做了两次完全相同的 GET /api/v1/customers/:id：customerProviderBound 只为判定
bound，紧接着 boundProviderCustomerID 又读一遍同一客户取 provider_customer_id。两次串行往返占用 settleRequestTimeout
预算，还引入一个（理论上的）双读不一致窗口。boundProviderCustomerID 的 404/缺 provider_customer_id 分支已经覆盖了未绑定的语义（都归为
definitive invalid_response），可以只保留后一次读取。

- 	_, bound, err := a.customerProviderBound(ctx, payload.ExternalCustomerID)
+ 	providerCustomerID, err := a.boundProviderCustomerID(ctx, payload.ExternalCustomerID)
  	if err != nil {
  		return commercial.CommandReceipt{}, err
- 	}
- 	if !bound {
- 		return commercial.CommandReceipt{}, fmt.Errorf(
- 			"%w: settle target customer carries no provider binding", commercial.ErrPlatformInvalidResponse)
  	}
- 	providerCustomerID, err := a.boundProviderCustomerID(ctx, payload.ExternalCustomerID)


─── internal/modules/commercial/commercialplatform/lago_settlement.go:75-78 ───
[bug · medium] settle 幂等 fast-path 与自身注释矛盾：算法 (i) 步承诺 "already active → idempotent receipt, zero
outbound calls"，但 readPurchaseSnapshot（lago_purchase.go）在 State==active 时会执行
readPurchaseInvoiceFees——finalizedInvoiceIDs 的索引分页读 + 逐张发票 detail 读（N+1，含 Base plan 的全部 finalized 0
金额发票）。后果：(1) 每次 settle 重放（outbox 重投递、lost-response 重试）都对该客户全部 finalized 发票做 N+1 读，且跑在
readPurchaseSnapshot 内部派生的 subscriptionRequestTimeout（15s）预算里，发票历史较长的客户会超时；(2) 幂等 no-op
的失败面被无关读扩大——索引 429/超时以 unreachable 失败（可重试），而 created_at 不可解析或索引超出 pageBudget 以 definitive 的
ErrPlatformInvalidResponse 失败，会把一个已完成、无需任何动作的 settle 重放 park 进 attention。建议：第 (i) 步只需要
Purchase.State——提取 state-only 的轻量读取（跳过 finalized-invoice 读），或在 settle 的 active 幂等分支将 invoice
读失败降级为只读 state 后照常返回 receipt。



LLM retry report summary: 1 of 247 requests affected -- 1 request failed

Review planning (1 request):
- internal/modules/commercial/commercialplatform/config.go,internal/modules/commercial/commercialplatform/fake.go,internal/modules/commercial/commercialplatform/lago.go,internal/modules/commercial/commercialplatform/lago_purchase.go,internal/modules/commercial/commercialplatform/lago_settlement.go: timed out -> failed

Per-attempt detail: --format json (retry_report).
