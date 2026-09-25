Review complete: 29 finding(s) across 103 selected item(s).

─── apps/web/src/commercial/CheckoutPage.tsx:124-130 ───
[bug · medium] 闭合文案映射只覆盖了 503 信封的顶层 reason 令牌，但同一 POST /purchases 的 409/501/500 错误族（{"error":"quote
expired"}、"invoice_quote_mismatch"、"purchase_plan_conflict"、"purchase_not_awaiting_payment"、"quote
already used" 等）既不带 reason 也不带可映射 code，closedReason 为 undefined 时 error.message
会把这些英文机器令牌原文展示给用户——与注释声明的 spec L210「页面词汇稳定，不含平台词汇」相悖。其中报价过期是真实可达路径（用户停留在结账页超过报价有效期后点重试即触发 409 "quote
expired"）。建议把 409 闭合令牌并入同一份映射（按 message 令牌匹配），未命中已知令牌时给统一中文兜底文案。

          const closedReason = error instanceof ApiError
            ? purchaseReasonMessage((error.details as { reason?: unknown } | undefined)?.reason)
+             ?? purchaseConflictMessage(error.message)
            : undefined;
          setState({
            status: 'error',
-           message: closedReason ?? (error instanceof Error ? error.message : 'Unable to open checkout'),
+           message: closedReason ?? (error instanceof ApiError ? '购买未能创建，请稍后重试' : 'Unable to open checkout'),
          });
+ 
+ // 新增：409 闭合令牌 → 中文文案（与 purchaseReasonMessage 同层维护）
+ export function purchaseConflictMessage(message: string): string | undefined {
+   switch (message) {
+     case 'quote expired': return '报价已过期，请重新获取报价';
+     case 'invoice_quote_mismatch': return '报价与账单不一致，请重新获取报价';
+     case 'purchase_plan_conflict': return '已有其他套餐的购买进行中，请刷新后重试';
+     case 'purchase_not_awaiting_payment': return '该购买已完成或已取消，无需再次支付';
+     case 'quote already used': return '该报价已被使用，请重新获取报价';
+     default: return undefined;
+   }
+ }


─── apps/web/src/commercial/CheckoutPage.tsx:47-49 ───
[bug · low] 防御分支语义反了：state==='active' 表示购买已成功（渠道支付已完成、权益已生效），但落入兜底文案「购买未能创建，请稍后重试」会诱导用户对已生效的购买再次发起
purchase。当前后端 POST 成功视图必带 order、非 awaiting 状态走 409 错误信封，此分支仅在契约漂移时可达；一旦触发文案方向错误。建议为 active
单独给出「已完成」语义文案。

-   return purchase.state === 'canceled'
-     ? '该购买已取消，请重新发起购买'
-     : '购买未能创建，请稍后重试';
+   if (purchase.state === 'canceled') return '该购买已取消，请重新发起购买';
+   if (purchase.state === 'active') return '购买已完成，如未生效请稍后刷新查看';
+   return '购买未能创建，请稍后重试';


─── apps/web/src/commercial/CheckoutPage.tsx:197-197 ───
[bug · low] R1-V12 只把「待付款」Status 限定在 payment==='pending'，但「前往支付」链接没有同条件：后端 orderWire 恒下发持久化的
checkout_url（internal/handler/commercial.go L301，无按付款态条件），轮询到 paid/closed
终态后页面仍渲染支付入口，与该注释声明的「已支付/终态不再误导重复支付」结论不一致。建议链接与 Status 同样限定在待付款期间。

-               {state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url) ? (
+               {state.order.payment === 'pending' && state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url) ? (


─── apps/web/src/commercial/CheckoutPage.tsx:101-106 ───
[maintainability · low] 本次变更删除了客户端幂等键机制（idempotencyKeyRef 与请求体中的 idempotency_key
均已移除，幂等改由服务端按身份保证），但错误态的重试按钮文案（本文件 L183）仍是「重试（复用原订单与幂等键，不重复下单）」——它描述的是已被本次改动移除的机制；且首次 purchase 失败（如
503 unreachable）时根本没有「原订单」可复用，文案与新语义不符，容易误导用户/排查者。建议同步更新按钮文案，例如「重试（同一报价不重复下单）」。

-         // #81：提交走 payment-gated purchase。重试语义：同一 quote 重试就是同一次
-         // purchase 调用，后端按身份幂等返回同一订单（不产生第二张订单/第二张账单）。
-         const purchase = await client.commercial.purchase(
-           { quote_id: quoteRef.current.id, provider: 'wechat' },
-           currentScope.signal,
-         );
+ <Button type="button" onClick={() => setRetryToken((value) => value + 1)}>重试（同一报价不重复下单）</Button>


─── deploy/lago-lab/payment-activation/evidence/t02-gating.json:11-12 ───
[maintainability · low] 同类观测键跨文件漂移（建议在生成器侧统一，或声明该区分是有意的）：t02-gating.json 用
recheck_subscription_status 表达"轮询窗口结束后的最终订阅状态"，而 t02-decline.json 对同一语义用
last_observed_incomplete；同样发生轮询耗尽时，decline/activation 输出结构化 poll_exhausted 键，gating 仅在
contract_notes 文字说明（"poll exhausted: gating invoice creation for customer A (attempts=172)"）。该差异源自
phases.py 两个阶段各自独立实现（567 行 vs 895/1342 行），未见文档声明此区分。当前 verify_ac_assertions.py 对两文件分别读取暂不受影响，但按统一
schema 解析/汇总这批证据的下游脚本需要按文件特判，且漂移的键名模式会掩盖后续真正的拼写错误引入。建议后续在 phases.py 写入侧统一为同一组键（如
final_subscription_status + poll_exhausted），或在 README 中记录此区分是有意的契约设计。



─── deploy/lago-lab/payment-activation/evidence/t02-manual.json:14-16 ───
[maintainability · low] observed 块中 before/after 证据键不对称：invoice_status 同时有 before/after，但
invoice_payment_status 只有 after、subscription_status 也只有 after。而该文件的 "state_unchanged": true
断言（expected: leaves invoice + subscription state unchanged）恰恰依赖付款状态与订阅状态的 before 比对——生成器 phases.py 的
manual 阶段实际已捕获并比较了 before["invoice_payment_status"] 和 sub_before 的状态，只是未写入 observed。作为审计证据文件，缺少
before 快照使"状态未变"无法从证据本身独立复核，需回溯上一阶段文件。建议补齐
invoice_payment_status_before、subscription_status_before（以及判定中用到的 total_paid_amount_cents 前后值）。

      "invoice_payment_status_after": "succeeded",
+     "invoice_payment_status_before": "succeeded",
      "invoice_status_after": "finalized",
      "invoice_status_before": "finalized",
+     ...
+     "subscription_status_after": "active",
+     "subscription_status_before": "active",


─── deploy/lago-lab/payment-activation/evidence/t02-retries.json:107-110 ───
[maintainability · low] "成功付款数"指标在本文件命名为 succeeded_count_before/after，而
t02-activation/decline/duplicates.json 对同一语义统一使用 payments_succeeded_count（生成器内部均由
_succeeded(payments) 计算）。这是上一轮已确认的"同类观测键跨文件漂移"问题的又一处实例（键对不同，非重复上报）：在生成器侧统一键名时建议一并纳入本处，例如统一为
payments_succeeded_count 前后缀形式，避免下游 verify 脚本适配两套命名。

        "payments_count_after": 0,
        "payments_count_before": 0,
-       "succeeded_count_after": 0,
-       "succeeded_count_before": 0
+       "payments_succeeded_count_after": 0,
+       "payments_succeeded_count_before": 0


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:22-23 ───
[documentation · low] 注释中的 "(ocr-2)" 是从 docs/plans/issue-72-ocr2-replay/verify_db_watch.py
复制时残留的目录标签：在该副本中它指其自身目录（ocr round-2 replay，run dfcc76a8…），而本目录 flow-evidence-74 对应的是 run 3dc51207…（见
t02-setup.json / verify-db-watch-samples.tsv），两者是不同的运行。该标签会误导读者以为本目录的存档 TSV 来自 ocr-2
replay，影响证据溯源说明的准确性。建议删除该标签或改为指明本目录自身的运行标识。

- # Prefer THIS evidence directory's archived copy (ocr-2): replayed observer
+ # Prefer THIS evidence directory's archived copy (run 3dc51207): replayed observer
  # TSVs land in the repo-level runs/ dir and db_watch.sh writes a fresh


─── docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py:10-12 ───
[documentation · low] docstring 中 "Endpoints implemented (only what the adapter calls)"
的括号注释与适配器实际实现不符:WechatProvider(wechat.go)除这两个端点外还会调用 POST
/v3/pay/transactions/out-trade-no/*/close(Close,wechat.go:446)、POST
/v3/refund/domestic/refunds(Refund,471)与 GET /v3/refund/domestic/refunds/*(QueryRefund,484)。#81
证据流停留在待支付窗口、不会触发这些调用,但作为取证文档,该措辞会让复用者误以为 stub 覆盖了适配器的全部出站请求;若后续(如退款/取消类验证)复用此 stub,这些操作只会得到 stub 的
404,表现为适配器侧报错而难以定位。建议在 docstring 中明确限定范围为 #81 流程所触发的端点,并说明 Close/Refund/QueryRefund 未实现、调用将返回 404。

- Endpoints implemented (only what the adapter calls):
+ Endpoints implemented (only those the #81 awaiting-payment flow triggers):
    POST /v3/pay/transactions/native          -> {"code_url": "weixin://wxpay/bizpayurl?pr=..."}
    GET  /v3/pay/transactions/out-trade-no/*  -> {"trade_state": "NOTPAY", ...}
+ NOTE: WechatProvider also calls Close (POST .../close), Refund and
+ QueryRefund (/v3/refund/domestic/refunds/*); this stub answers 404 for
+ those, so reuse beyond the awaiting-payment window needs them added.


─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:57-58 ───
[documentation · low] stub 硬依赖的三个本地密钥文件（alipay_verify_local.pem / alipay_verify_local_pub.pem /
alipay_merchant_local.pem）未提交到仓库（正确——避免凭据入库），但整个 evidence 目录（含 README「运行方式（复现）」一节）都没有记录它们的生成命令。密钥缺失时
openssl 以 check=True 直接抛错，其他评审者无法复现该取证流程，且三份文件有格式要求（PKIX 公钥、PKCS#8 商户私钥，与 Go 侧
parseRSAPublicKey/merchantKey 的解析对应），凭猜测重新生成容易踩错格式。建议在 docstring 或 README 中补上等价的 openssl
genrsa/pkcs8/pubout 生成命令序列。

+ # 生成命令（建议补入 docstring 或 README，示例）：
+ #   openssl genrsa -out alipay_verify_local.pem 2048
+ #   openssl rsa -in alipay_verify_local.pem -pubout -outform PEM \
+ #     -out alipay_verify_local_pub.pem            # PKIX -> WEKNORA_ALIPAY_PUBLIC_KEY_PATH
+ #   openssl pkcs8 -topk8 -nocrypt -in alipay_verify_local.pem \
+ #     -out alipay_merchant_local.pem              # PKCS#8 -> WEKNORA_ALIPAY_MERCHANT_KEY_PATH
  ALIPAY_KEY = f"{KEY_DIR}/alipay_verify_local.pem"
  SHARED_PUB = f"{KEY_DIR}/alipay_verify_local_pub.pem"


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:38-39 ───
[maintainability · low] 「权益已生效」完全包含「已生效」，因此 `!body.includes('已生效')`
为真时第一个操作数必为真——`!body.includes('权益已生效')` 是恒冗余条件，整个断言等价于
`!body.includes('已生效')`。若原意是断言两个不同文案，这里实际丢失了一条断言；否则应删去冗余操作数以免误导后续维护者。

-   note('no-false-effective', !body.includes('权益已生效') && !body.includes('已生效'),
+   note('no-false-effective', !body.includes('已生效'),
      'no 已生效 claim anywhere while authority subscription is incomplete (D2 frozen)');


─── docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py:12-12 ───
[documentation · low] docstring 中 "Endpooints" 为明显拼写错误（应为 "Endpoints"）。该文件自称 verbatim 复用 issue-81
的同名助手，拼写错误在两处副本中同时存在，影响文档可读性，建议一并修正。

- Endpooints implemented (only what the adapter calls):
+ Endpoints implemented (only what the adapter calls):


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:102-106 ───
[maintainability · low] runs/ 回退路径上，R1-V10 的 `if not archived.exists(): sys.exit(2)` 守卫被放在三次
rows_for() 解析之后：回退场景下这些解析结果必然被丢弃（无论如何 exit 2），且在此之前读入并解析外来 runs/ TSV 的过程可能抛未捕获异常（如旧格式行
`rows[...|incomplete]` 使 int(st) 抛 ValueError），脚本将以退出码 1 崩溃——这恰好违背守卫注释自己声明的意图（"degrade loudly to
MISSING-EVIDENCE instead of risking a false CHECK"），把本应 MISSING-EVIDENCE(2) 的场景劣化为
CHECK(1)。建议把该守卫整体上移到 rows_for() 调用之前（紧随 lines/decline 加载），既消除无效解析，也保证回退路径在任何外来 TSV 内容下都以干净的退出码 2 终止。

+ if not archived.exists():
+     print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
+           "whole-DB aggregate and cannot be keyed to this run, so the "
+           "exactly-once max_succeeded assertion is not decidable")
+     sys.exit(2)
+ 
  sub_b = rows_for("-sub-b")
  sub_c = rows_for("-sub-c")
  sub_a = rows_for("-sub-a")
  
  # (R1-V10) The rows_for() assertions are run-keyed by run_prefix, but the


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:12-12 ───
[documentation · low] docstring 中声明的注解行号不准确："# ocr-2:" 注解实际位于本文件第 76 行和第 86 行（非 74 和 84，恰好偏移 2
行）。本文件的存在目的（R1-V23）正是修正上一版错误的 "unmodified" 声明，行号声明再次出错会削弱证据工件的可信度。另外，相对 ocr-1 副本实际新增的是两条功能性断言（"AC2
duplicates deferred re-check clean" 与 "AC3 gate retry probe recorded not_applicable"，ocr-1
副本中均不存在），仅描述为 "assertion annotations"（注解）会低估两份脚本在验证行为上的差异，建议一并修正措辞。

- two ocr-2-specific assertion annotations: "# ocr-2:" at lines 74 and 84
+ two ocr-2-specific assertion additions: "# ocr-2:" at lines 76 and 86, each
+ followed by a new functional assertion (deferred re-check; not_applicable
+ gate-retry probe) absent from the ocr-1 copy


─── internal/modules/commercial/commercialplatform/config.go:42-43 ───
[maintainability · low] [maintainability · low] 环境变量名以 "WEKNORA_COMMERCIAL_STRIPE_API" + "_KEY"
形式拆分拼接,使按完整名称 grep 无法定位到定义:例如 deploy/lago/evidence/t09-run.txt 与运维文档引用的
WEKNORA_COMMERCIAL_STRIPE_API_KEY
在源码中检索不到读取点,审计"该凭据环境变量在哪里被消费"必须预知拆分约定。环境变量名称本身不是凭据(安全约束禁止的是凭据值的字面量,值只从
env/密钥服务读取),规避扫描器对名称模式的匹配属于脆弱先例——更稳妥做法是保留完整字面量并在注释说明,或调整扫描器豁免规则。ProviderCustomerPrefix
未拆分也说明该做法并不一致。

- 	EnvStripeKey              = "WEKNORA_COMMERCIAL_STRIPE_API" + "_KEY"
- 	EnvStripeAPIBase          = "WEKNORA_COMMERCIAL_STRIPE_API" + "_BASE"
+ 	// 名称非凭据(凭据是其值,仅从 env/密钥服务读取),保留完整字面量
+ 	// 以保证可 grep 审计;扫描器误报应在其豁免规则中处理。
+ 	EnvStripeKey              = "WEKNORA_COMMERCIAL_STRIPE_API_KEY"
+ 	EnvStripeAPIBase          = "WEKNORA_COMMERCIAL_STRIPE_API_BASE"


─── internal/modules/commercial/payment/alipay.go:564-566 ───
[performance · low] [performance · low] 该逐请求预检与本次同时引入的 NewSSRFSafeHTTPClient 防护完全重复且更昂贵:p.client 的
SSRFValidatingRoundTripper 已对每个出站请求(其 URL 即含 gateway)经 60s origin 级缓存路径
validateURLForSSRFForOutbound 校验,拨号层 SSRFSafeDialContext 另有 IP 钉扎;而此处 ValidateURLForSSRF →
isSSRFSafeURL(internal/utils/security.go:393)每次同步执行无缓存 net.LookupIP,给每笔下单/查单/退款及其轮询路径串行加一次 DNS
解析。另外瞬时 DNS 故障会被归类为 ErrNotConfigured(哨兵语义是"密钥引用/材料缺失",见 wechat.go:58),且 %v 丢失底层原因。前轮
OCR(docs/plans/issue-72-ocr-round-1.md)R1-15/18 已建议移到构造期或删除,本批客户端替换后建议一并落地:网关配置构造后不可变,可在
NewAlipayProvider 校验一次;若保留每请求预检,应区分 DNS 瞬时故障与配置违规并以 %w 保留原因。

- 	if err := secutils.ValidateURLForSSRF(p.cfg.gateway()); err != nil {
- 		return fmt.Errorf("%w: gateway url failed SSRF validation: %v", ErrNotConfigured, err)
- 	}
+ // NewAlipayProvider 内构造期校验一次(配置不可变):
+ //	if err := secutils.ValidateURLForSSRF(cfg.gateway()); err != nil {
+ //		return nil, fmt.Errorf("%w: gateway url failed SSRF validation: %w", ErrNotConfigured, err)
+ //	}
+ // 每请求校验由 SSRFValidatingRoundTripper(60s 缓存)+ 拨号期 IP 钉扎兜底,
+ // call() 内的逐请求未缓存预检可删除。


─── internal/modules/commercial/payment/alipay.go:205-208 ───
[bug · medium] 切换到 NewSSRFSafeHTTPClient 时静默丢弃了环境代理出网能力:原先 `&http.Client{Timeout: cfg.timeout()}` 的
Transport 为 nil,走 http.DefaultTransport(带 `Proxy: http.ProxyFromEnvironment`);而
NewSSRFSafeHTTPClient 内部的 NewSSRFSafeTransport(internal/utils/security.go:712)只设置
DialContext/DisableKeepAlives/DisableCompression,未设置 `Proxy`
字段。在仅允许代理出网(HTTP_PROXY/HTTPS_PROXY)的部署中,支付宝网关调用将改为直连并大概率超时失败。这与仓库其他模块不一致(airesource/models/chat/tran
sport.go:52、knowledge/retriever/opensearch/transport.go:61、airesource/web_search/proxy.go:47 的自定义
transport 均显式保留 ProxyFromEnvironment);且 SSRFSafeDialContext 中的 `IsSystemProxy(addr)`
旁路(security.go:822)只有在 transport 配置了代理、拨号地址为代理主机时才可达——SSRF 层本身预期代理出网,当前该路径对支付渠道是死代码。建议改用
NewSSRFSafeHTTPClientWithTransport 并在 transport 上补 `Proxy: http.ProxyFromEnvironment`(或在共享构造器中统一补上)。

- 		client: secutils.NewSSRFSafeHTTPClient(secutils.SSRFSafeHTTPClientConfig{
+ 	transport := secutils.NewSSRFSafeTransport(secutils.SSRFSafeHTTPClientConfig{
+ 		Timeout: cfg.timeout(), MaxRedirects: 10,
+ 	})
+ 	transport.Proxy = http.ProxyFromEnvironment
+ 	return &AlipayProvider{
+ 		// ...
+ 		client: secutils.NewSSRFSafeHTTPClientWithTransport(secutils.SSRFSafeHTTPClientConfig{
  			Timeout:      cfg.timeout(),
  			MaxRedirects: 10,
- 		}),
+ 		}, transport),
+ 		// ...
+ 	}, nil


─── internal/modules/commercial/payment/wechat.go:536-538 ───
[performance · low] [performance · low] 与 alipay.go call() 同一问题:此处逐请求 ValidateURLForSSRF(apiBase)
预检与 p.client 的 SSRFValidatingRoundTripper(每请求经 60s 缓存路径校验完整 URL)+ 拨号期 IP 钉扎完全重复,且走的是无缓存变体,每次同步做一次
net.LookupIP(security.go:393),APIv3 下单/查单/退款轮询每次都多一次串行 DNS 解析;瞬时 DNS 故障被误标为
ErrNotConfigured("密钥引用缺失"哨兵),%v 也丢弃底层原因。前轮 OCR R1-15/18 建议移到 newWechatProvider 构造期执行一次或直接删除(apiBase
构造后不可变)。

- 	if err := secutils.ValidateURLForSSRF(p.cfg.apiBase()); err != nil {
- 		return fmt.Errorf("%w: api base url failed SSRF validation: %v", ErrNotConfigured, err)
- 	}
+ // newWechatProvider 内构造期校验一次(配置不可变):
+ //	if err := secutils.ValidateURLForSSRF(cfg.apiBase()); err != nil {
+ //		return nil, fmt.Errorf("%w: api base url failed SSRF validation: %w", ErrNotConfigured, err)
+ //	}
+ // do() 内的逐请求未缓存预检可删除,由 SSRFValidatingRoundTripper 兜底。


─── internal/modules/commercial/payment/wechat.go:191-194 ───
[bug · medium] 与 alipay.go 同一问题:原先 `&http.Client{Timeout: cfg.timeout()}` 走 http.DefaultTransport(带
`Proxy: http.ProxyFromEnvironment`),换成 NewSSRFSafeHTTPClient 后其
transport(NewSSRFSafeTransport,internal/utils/security.go:712)未设置 `Proxy`
字段,环境代理(HTTP_PROXY/HTTPS_PROXY)被静默丢弃。在仅允许代理出网的部署中,微信 APIv3 下单/查单/退款调用将改为直连并大概率失败;且
SSRFSafeDialContext 中的 IsSystemProxy 旁路(security.go:822)在此 client 下永远不可达,与仓库其他保留
ProxyFromEnvironment 的自定义 transport(airesource/models/chat/transport.go:52 等)不一致。建议改用
NewSSRFSafeHTTPClientWithTransport 并在 transport 上补 `Proxy: http.ProxyFromEnvironment`。

- 		client: secutils.NewSSRFSafeHTTPClient(secutils.SSRFSafeHTTPClientConfig{
+ 	transport := secutils.NewSSRFSafeTransport(secutils.SSRFSafeHTTPClientConfig{
+ 		Timeout: cfg.timeout(), MaxRedirects: 10,
+ 	})
+ 	transport.Proxy = http.ProxyFromEnvironment
+ 	return &WechatProvider{
+ 		// ...
+ 		client: secutils.NewSSRFSafeHTTPClientWithTransport(secutils.SSRFSafeHTTPClientConfig{
  			Timeout:      cfg.timeout(),
  			MaxRedirects: 10,
- 		}),
+ 		}, transport),
+ 		// ...
+ 	}


─── packages/api-client/src/commercial.ts:66-66 ───
[maintainability · low] purchase() 的请求体类型内联在方法签名里，而既有 POST /orders 的 CreateOrderInput 由 contracts
导出作为单一事实来源；两者共享 quote_id/provider 词汇，后续渠道或字段演进（如扩展 provider）时缺少统一契约点，也导致 contracts 与 api-client
两处维护同一 wire 词汇。建议在 contracts 抽出并导出 PurchaseInput（顺带在 index.ts 一并导出）。

-     async purchase(input: { quote_id: string; provider: 'wechat' | 'alipay' }, signal?: AbortSignal): Promise<PurchaseView> {
+ // packages/contracts/src/commercial.ts
+ export interface PurchaseInput { quote_id:string; provider:'wechat'|'alipay' }
+ 
+ // packages/api-client/src/commercial.ts
+     async purchase(input: PurchaseInput, signal?: AbortSignal): Promise<PurchaseView> {


─── packages/contracts/src/commercial.ts:200-203 ───
[bug · medium] reason 是建议性（advisory）字段：对未知令牌整体抛错会让整份 PurchaseView 解析失败——GET 路径 BillingPage
静默丢弃购买状态（可接受降级），但 POST 路径用户会直接看到 'invalid purchase (reason)' 原文。后端先于前端发版新增闭合 reason
令牌（滚动升级/版本漂移）时即触发，恰好违背「页面词汇稳定」的设计初衷。建议未知 reason 按缺席忽略（state 仍保持严格校验，state 无法渲染时抛错是合理的）。

-   if(v.reason!==undefined&&v.reason!=='') {
-     if(typeof v.reason!=='string'||!PURCHASE_REASONS.has(v.reason)) throw new Error('invalid purchase (reason)');
+   // 未知 reason 令牌按缺席忽略：reason 是建议性字段，state 才是渲染必需。
+   if(typeof v.reason==='string'&&v.reason!==''&&PURCHASE_REASONS.has(v.reason)) {
      out.reason = v.reason;
    }


─── deploy/lago-lab/payment-activation/clients.py:390-404 ───
[maintainability · low] `last_error` 是死变量：`except (URLError, OSError)` 分支中赋值后从未被读取——最后一次尝试直接 `raise`
抛出当前异常，成功路径 `break` 也不使用它。徒增阅读困惑（读者会以为某处消费该变量），建议删除。

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
              except (URLError, OSError) as error:
-                 last_error = error
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/phases.py:767-770 ───
[maintainability · low] 按 lago_id 未命中时静默回退到 `_gating_invoice` 的泛化匹配，可能选中与 `target`
标注（"gate"/"activation"）不同的另一张发票：observed 中的 `target` 字段、before/after 对比对象与实际 POST 目标会指向不一致的发票，弱化 403
证据的可归因性。建议在回退实际生效时（选中的发票 lago_id != 目标 lago_id）记录一条 note 或在 observed 中标注 fallback 及实际发票 id。

          invoice = next(
              (inv for inv in invoices if inv.get("lago_id") == invoice_lago_id),
              _gating_invoice(invoices),
+         )
+         if invoice is not None and invoice.get("lago_id") != invoice_lago_id:
+             ctx.note(
+                 f"target invoice {invoice_lago_id} not visible via the API; "
+                 f"falling back to invoice {invoice.get('lago_id')} "
+                 f"(target={target})"
-         )
+             )


─── deploy/lago-lab/payment-activation/phases.py:147-150 ───
[maintainability · low] 解析失败的两条路径不对称：异常路径会 `ctx.note(...)` 保持可归因（注释自己也以此为设计目标），但
`/api/v1/organizations` 返回非 2xx 时（无异常抛出）既不缓存也不留任何记录，静默返回 None。此时报告中对解析失败无任何痕迹，后续 GraphQL 突变若被 400
"Missing organization id" 拒绝，将无从归因。建议在 `_ok(status)` 不成立时同样记录一条 note。

                  status, body = self.lago.request("GET", "/api/v1/organizations")
                  org = ((body or {}).get("organization") or {})
                  if _ok(status) and org.get("lago_id"):
                      self._graphql_organization_id = org["lago_id"]
+                 else:
+                     self.note(
+                         "x-lago-organization resolution failed "
+                         f"(GET /api/v1/organizations -> HTTP {status}); header "
+                         "omitted — GraphQL mutations may be rejected with "
+                         "'Missing organization id'"
+                     )


─── deploy/lago-lab/payment-activation/phases.py:262-266 ───
[maintainability · low] `_subscription_status_any` 对传输错误的 note 是重复的：它调用的 `_subscription_show` 在
`except OSError` 中已经 `ctx.note` 了完全相同的文本（第 248 行）然后 re-raise，这里再捕获、再 note 一条一模一样的消息、再
raise，会让单次传输失败在报告的 `contract_notes` 里出现两条重复记录。建议直接去掉这层 try/except（让 `_subscription_show` 的 note +
异常穿透即可），或改为直接调用 `ctx.lago.get` 由本函数唯一负责 note。

-         try:
+         # _subscription_show already notes transport errors before re-raising
-             ss, sb = _subscription_show(ctx, external_id, status)
+         ss, sb = _subscription_show(ctx, external_id, status)
-         except OSError as error:
-             ctx.note(f"transport error reading subscription {status}: {error.__class__.__name__}")
-             raise


─── internal/modules/commercial/service/commercial/purchase.go:236-241 ───
[bug · high] R1-V22 的"同一购买至多一张可付渠道订单"不变量只做了三段独立的读-判-写（GetOrderByQuote → CurrentPendingPurchaseOrder
→ CreateOrder），没有数据库层约束或锁：数据库唯一性仅存在于 quote_id 维度。两个并发 POST 携带同一购买的两张不同新报价（重报价竞速、双开标签页）可同时通过两道前置
SELECT（彼时对方订单均未提交），随后各自成功 CreateOrder 消费各自的报价——结果是同一 gating invoice 存在两张 pending
渠道订单，两个回调独立确认即为双重扣款。这正是注释声称防住的形状，但实现只缩小了窗口、未消除。

次要弱点：CurrentPendingPurchaseOrder 以 (tenant, kind, amount_fen, currency)
作为"当前购买"的匹配键，金额相等并不能证明该挂单属于本次购买——旧购买取消后遗留的同价位（不同 plan）stale pending 订单会被当作本次购买的支付入口回放。

建议：在数据库层强制原子性，例如对 commercial_orders 建部分唯一索引 (tenant_id) WHERE kind='purchase' AND
state='pending'，把插入冲突作为回放触发条件（与 ErrQuoteAlreadyUsed 的处理同构）；或在 OpenOrder 事务内对该租户的 pending purchase
订单做 SELECT ... FOR UPDATE 后再判定插入。匹配键宜锚定购买身份（如 plan code/关联订阅）而非仅金额。

- 	if existing, perr := s.orders.orders.CurrentPendingPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); perr == nil {
- 		ov := orderViewFromRow(existing)
- 		return s.purchaseView(p, snap, pub, &ov), nil
- 	} else if !errors.Is(perr, repocommercial.ErrOrderNotFound) {
- 		return PurchaseView{}, perr
- 	}
+ // 需要数据库层护栏（部分唯一索引或事务内 FOR UPDATE），示意：
+ // CREATE UNIQUE INDEX uni_commercial_orders_pending_purchase
+ //   ON commercial_orders (tenant_id) WHERE kind = 'purchase' AND state = 'pending';
+ // 插入冲突 → 走 ErrOrderNotFound 之外的回放分支（同 ErrQuoteAlreadyUsed 的处理）。


─── internal/modules/commercial/service/commercial/order.go:361-363 ───
[security · medium] SetCheckoutURL 持久化失败复用了 CheckoutError 标记并携带 `%v` 原始错误文本：gorm 失败信息可能包含 SQL
语句、表/列/约束名等内部细节，而 orderWire 会把 checkout_error 原样序列化到公共 API 边界（handler/commercial.go
orderWire），与本次购买链路其余分支"错误文案闭合、原始错误只进服务端日志"的姿态（POST /purchases 残差分支的 log.Printf + 闭合文案）不一致。

同时，此分支里渠道调用实际已成功、客户端拿到的 CheckoutURL 有效，但非空 CheckoutError 会让 handler 的 Purchase 命中 `CheckoutError !=
""` 分支降级为 202——该分支的语义是"渠道失败，需经 GET 恢复"，客户端（CheckoutPage 轮询 checkout_error）可能把一次成功的下单误判为渠道失败。

建议：持久化失败改用独立的降级标记（而非复用渠道失败的 CheckoutError），且对外文案闭合（如 "checkout_link_persistence_degraded"），原始错误用
log.Printf 落服务端日志。

  	if err := s.orders.SetCheckoutURL(ctx, id, res.CheckoutURL); err != nil {
- 		view.CheckoutError = fmt.Sprintf("checkout_url persistence failed for %s: %v", id, err)
+ 		log.Printf("commercial: checkout_url persistence failed for order %s: %v", id, err)
+ 		view.CheckoutError = "checkout_link_persistence_degraded"
  	}


─── internal/modules/commercial/service/commercial/purchase.go:236-241 ───
[bug · high] R1-V22 的重放把「pending」等价于「可支付」，但两类 pending 订单被混为一谈：渠道 Create 成功（checkout_url
已持久化、渠道侧可付）与渠道 Create 失败（openOrder 的 CheckoutError 分支：订单已提交、quote 已消费、checkout_url
为空、渠道很可能从未收到该单）。订单状态机只有 pending→paid→fulfilled（唯一出 pending 的路径是 ConfirmPayment 的渠道成功事实），没有任何
failed/canceled/过期迁移。因此一次瞬时的渠道创建失败（网关 5xx、超时）之后，该价位面会永久留下一张 pending 死单；此后每次用新报价 POST /purchases
都会在这一步被这张死单挡住：新 quote 不被消费、响应 201 但 order.checkout_url 为空（客户端没有任何支付入口），GET /orders/:id
恢复也永远无法让一个渠道从未见过的订单离开 pending——该租户在该价位面的购买被永久卡死，只能人工修库。建议：重放前区分「可付」与「不可付」的 pending 订单，例如仅当持久化的
checkout_url 非空（渠道确实创建了该单）时才原样重放；对无链接的 pending 死单先走渠道恢复查询（RecoverOrderStatus 同款
provider.Query），渠道确认未知该单时允许新开渠道订单（或在行上持久化渠道失败标记/引入终态），而不是让死单永久遮蔽新购买。



─── internal/handler/commercial.go:499-499 ───
[maintainability · low] 这里用了标准库 log.Printf，是整个 handler 包里唯一的 stdlib log 调用（其余 60+ 个文件统一使用项目
logger.Errorf/ErrorWithFields(ctx, ...)
携带请求上下文的结构化日志）。该行会绕过项目的日志级别/字段/采集管道，生产排障时这条关键的残差错误日志不会出现在结构化日志流里。建议改用项目 logger 并带上请求 ctx。

- 		log.Printf("commercial: purchase failed for tenant %d: %v", tenantID, err)
+ 		logger.Errorf(c.Request.Context(), "commercial: purchase failed for tenant %d: %v", tenantID, err)


LLM retry report summary: 1 of 90 requests affected -- 1 request failed

Review planning (1 request):
- docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr3-replay/verify_db_watch.py: timed out -> failed

Per-attempt detail: --format json (retry_report).
