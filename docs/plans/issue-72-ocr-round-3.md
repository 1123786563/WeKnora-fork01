Review complete: 40 finding(s) across 104 selected item(s).

─── apps/web/src/commercial/BillingPage.tsx:117-117 ───
[style · low] 同一购买状态在两个页面的用户可见措辞不一致：BillingPage 显示「待付款（权益未开放）」，CheckoutPage（及其测试断言
/待付款（权益未开通）/）显示「待付款（权益未开通）」。「未开放」与「未开通」混用会让用户认为是两种不同状态。建议统一措辞——CheckoutPage.test.tsx 已断言「未开通」，改
BillingPage 侧成本最低。

-                 {purchase?.state === 'awaiting_payment' ? ' · 待付款（权益未开放）' : ''}
+                 {purchase?.state === 'awaiting_payment' ? ' · 待付款（权益未开通）' : ''}


─── apps/web/src/commercial/CheckoutPage.tsx:99-99 ───
[bug · medium] purchaseErrorText 的最终兜底违反本次变更自己确立的 spec L210 原则（闭合中文文案、不把英文原文展示给用户）：非 ApiError
的网络层错误（如 fetch 抛出的 TypeError "Failed to fetch"）会走到 `error.message` 分支直接把浏览器英文技术信息渲染给用户；非 Error
值则展示英文常量 'Unable to open checkout'。注释中"网络层等保留原始 message"是有意设计，但结果是用户在断网等场景看到英文技术串，与 503/409
分支的中文闭合哲学不一致。建议非服务端错误也收敛为中文兜底文案（如「网络异常，请稍后重试」）。

-   return error instanceof Error ? error.message : 'Unable to open checkout';
+   // 网络层等非服务端错误同样不向用户展示英文技术信息（spec L210：页面词汇稳定）。
+   return error instanceof ApiError ? PURCHASE_ERROR_FALLBACK : '网络异常，请稍后重试';


─── apps/web/src/commercial/CheckoutPage.tsx:141-141 ───
[maintainability · low] provider: 'wechat' 为业务硬编码：契约层 purchase 签名已支持 'wechat' |
'alipay'（packages/api-client/src/commercial.ts L66），后端支付宝渠道也已接入（evidence-82），但 UI
提交路径写死微信渠道，用户永远无法触达支付宝。若 #82 切片不含渠道选择 UI，此处将成为永久限制。建议至少提取为具名常量并注释切片计划，或预留渠道选择入口。

-           { quote_id: quoteRef.current.id, provider: 'wechat' },
+           // #81 第一切片固定微信渠道；#82（alipay）接入后改为渠道选择。
+           { quote_id: quoteRef.current.id, provider: PURCHASE_PROVIDER_WECHAT },


─── apps/web/src/commercial/CheckoutPage.tsx:76-76 ───
[maintainability · low] PURCHASE_CONFLICT_MESSAGES 以后端 handler 的英文自由文本长句作为机器令牌逐字匹配（如本行 41
字符含分号空格的句子）。后端 commercial.go 同族错误已存在 4 种措辞变体（L469 "...; please re-quote"、L702 无后缀、L871/883 "...; cut
a new quote and retry"），任何一处措辞漂移（标点、空格）都会让前端映射静默失配、降级为兜底文案，且现有测试只覆盖 'quote expired' 和 'unreachable'
两个代表，无法捕获失配。建议与 503 分支对齐：后端 409/500 族也在信封中携带短闭合令牌（复用 reason 字段），前端按键精确匹配，长句仅作日志。



─── apps/web/src/commercial/CheckoutPage.tsx:44-44 ───
[style · low] purchaseErrorMessage / purchaseReasonMessage / purchaseConflictMessage /
purchaseErrorText 四个函数全部 export，但全仓库无外部消费方（CheckoutPage.test.tsx
通过渲染级断言覆盖路径，未直接导入这些函数）。若无近期外部使用计划，建议去掉 export 保持模块私有；或让测试直接导入以固定各令牌分支的行为。

- export function purchaseErrorMessage(purchase: PurchaseView): string {
+ function purchaseErrorMessage(purchase: PurchaseView): string {


─── apps/web/src/commercial/CheckoutPage.tsx:138-141 ───
[bug · medium] 重试按钮文案与已删除的机制脱节：error 分支的按钮仍写「重试（复用原订单与幂等键，不重复下单）」（L214），但本次改动删除了
idempotencyKeyRef，purchase 请求不再携带
idempotency_key，幂等保障已改为后端按身份幂等。用户可见文案引用已不存在的「幂等键」，会误导用户/验收对防重复扣款机制的认知。建议同步更新按钮文案，例如「重试（同一报价即同一次购买，不重
复下单）」。



─── apps/web/src/commercial/CheckoutPage.tsx:161-161 ───
[bug · medium] 报价级冲突令牌的重试是死胡同：PURCHASE_CONFLICT_MESSAGES 中多个令牌的文案都指示用户「请重新获取报价」，但 quoteRef.current
一旦赋值就永不重置（L131 只在为空时才调用 quote）。error 态唯一的「重试」按钮会复用同一张已失效的报价再次 POST /purchases，后端（commercial.go
L467/L477/L469）对同一 quote 确定性返回同一 409，用户反复点击只会得到同样的错误，本页没有任何重新报价的路径，只能离开页面再回来。建议在捕获到报价级冲突令牌（quote
expired / quote already used / quote predates… / subscription changed…）时清空
quoteRef.current，使「重试」真正走一次新的 quote。



─── apps/web/src/commercial/CheckoutPage.tsx:158-160 ───
[maintainability · low] purchase 专属兜底文案被套用到非 purchase 失败上：这个 catch 覆盖 run() 的全部请求（含 getOrder 与
quote）。当 orderIdRef.current 已固定（购买已成功、effect 重跑进入 getOrder 分支）而 getOrder 返回 5xx
时，ApiError.status>=400 分支会展示「购买未能创建，请稍后重试」——事实上订单已创建且可能正处于待付款，失败只是订单视图加载；quote
阶段的失败同理被误标为「购买未能创建」。建议把 purchaseErrorText 的令牌映射/兜底限定在 purchase 调用周围（局部 try/catch
或标记错误来源），getOrder/quote 失败走各自的中性文案。



─── apps/web/src/commercial/CheckoutPage.tsx:228-228 ───
[bug · medium] 渠道支付失败（后端 R1-V14 分支）时本页成为支付死胡同：渠道调用失败时后端仍以 202 返回持久化的 pending 订单且 checkout_url
为空（commercial.go L453-461，orderWire 的 checkout_url
为空字符串）。此时页面只渲染「待付款（权益未开通）」+「刷新订单状态」，没有「前往支付」链接、没有任何渠道异常提示或重新发起支付的入口——刷新永远拿不到链接（GET
返回的是同一张渠道失败订单），用户在本页无法完成支付。服务端已保证渠道失败/无链接的 pending 订单不会阻塞新结账（service/purchase.go R2-28），建议在
order.payment === 'pending' 且无安全 checkout_url 时给出「重新发起支付」入口（清空 orderIdRef/quoteRef 后重跑）或至少提示渠道异常。



─── docs/plans/issue-72-flow-evidence-74/t02-health.json:1-1 ───
[maintainability · low] 本文件为单行压缩 JSON，与同目录其余 9 个证据文件（两空格缩进 +
末尾换行）序列化格式不一致。根因在生成端走了两条序列化路径：deploy/lago-lab/payment-activation/lab.sh 第 123 行使用 json.dumps(...,
sort_keys=True)（无 indent），而其余 t02-*.json 由 run_lab.py 的 write_json 以 sort_keys=True, indent=2
写出。同一证据目录内格式不统一会降低按键/按行 diff 的可读性，也使后续快照比对需区分序列化差异。建议统一序列化路径（生成端补 indent=2，或将本快照归一化为与其余证据一致的缩进格式）。



─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:55-56 ───
[bug · low] KEY_DIR 用 __file__.rsplit("/", 1)[0] 解析脚本目录,当以裸文件名启动(如 `python3
alipay_gateway_stub.py`,__file__ 为 "alipay_gateway_stub.py"、不含 "/")时,rsplit 返回整个文件名,KEY_DIR 变成
"alipay_gateway_stub.py",拼出的密钥路径不存在。后果具有误导性:rsa_verify_sha256 中 openssl 读不到 SHARED_PUB → returncode
!= 0 → verify_request 恒为 False,所有请求一律落入 isv.invalid-signature 拒绝分支,像是签名实现错了而非路径解析错了。建议改为
os.path.dirname(os.path.abspath(__file__))。

+ import os
+ 
  HOST, PORT = "127.0.0.1", 8292
- KEY_DIR = __file__.rsplit("/", 1)[0]
+ KEY_DIR = os.path.dirname(os.path.abspath(__file__))


─── docs/plans/issue-72-flow-evidence-82/alipay_sandbox_notify.py:30-30 ───
[bug · low] 与 alipay_gateway_stub.py 相同的问题:__file__.rsplit("/", 1)[0] 在脚本以裸文件名启动(如 `python3
alipay_sandbox_notify.py ...`,__file__ 不含 "/")时返回文件名本身,ALIPAY_KEY 指向不存在的
"alipay_sandbox_notify.py/alipay_verify_local.pem",openssl 因 check=True 直接抛 CalledProcessError。建议改为
os.path.dirname(os.path.abspath(__file__))。

- KEY_DIR = __file__.rsplit("/", 1)[0]
+ import os
+ KEY_DIR = os.path.dirname(os.path.abspath(__file__))


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:28-28 ───
[security · low] 口令字面量硬编码且无环境变量覆盖:同组 browser_flow_82.mjs 使用 process.env.FLOW82_PASSWORD ?? '...'
模式,本脚本直接写入 'issue82-Flow-Pw-d'。docs/plans/issue-72-ocr-round-1.md(第 532 行)已建议改为 FLOW82_PASSWORD_D
环境变量覆盖但未在本文件应用。虽为 .verify.local 本地实验室账号、影响有限,但与"测试不得写入可用凭据字面量"的验收约束及同组脚本风格不一致,建议对齐。

-   await page.fill('#auth-password', 'issue82-Flow-Pw-d');
+ const PASSWORD = process.env.FLOW82_PASSWORD_D ?? 'issue82-Flow-Pw-d';
+ // ...
+   await page.fill('#auth-password', PASSWORD);


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:27-27 ───
[security · low] 口令字面量硬编码且无环境变量覆盖:同组 browser_flow_82.mjs 使用 process.env.FLOW82_PASSWORD ?? '...'
模式,本脚本直接写入 'issue82-Flow-Pw-b'。docs/plans/issue-72-ocr-round-1.md(第 511 行)已建议改为 FLOW82_PASSWORD_B
环境变量覆盖但未在本文件应用。虽为 .verify.local 本地实验室账号、影响有限,但与"测试不得写入可用凭据字面量"的验收约束及同组脚本风格不一致,建议对齐。

-   await page.fill('#auth-password', 'issue82-Flow-Pw-b');
+ const PASSWORD = process.env.FLOW82_PASSWORD_B ?? 'issue82-Flow-Pw-b';
+ // ...
+   await page.fill('#auth-password', PASSWORD);


─── docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py:12-12 ───
[documentation · low] docstring 拼写错误:"Endpooints" 应为 "Endpoints"。该文件自称 "Verbatim reuse of the Issue
#81 evidence helper",但 #81 原文(docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py 第 10 行)使用的是正确的
"Endpoints implemented",复制时引入了新错误,建议修正以保持与原文一致。

- Endpooints implemented (only what the adapter calls):
+ Endpoints implemented (only what the adapter calls):


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:10-13 ───
[documentation · low] docstring 的自指行号不准确：声称两处 "# ocr-2:" 注释位于第 74 行和第 84 行，但按本文件实际行号它们分别位于第 76 行（"#
ocr-2: the PASS now also stands on the deferred re-check after the settle"）和第 86 行（"# ocr-2: with
the gate invoice API-invisible the retry probe is skipped and"）。该说明本身是 R1-V23 的行号纠正产物（前版错误声称
"unmodified"），纠正后行号仍偏差 2 行；本文件系列以行号级自我描述支撑 diff 级复核审计，失准的行号会误导后续审计。建议改为不依赖行号的位置描述（行号极易随文件演进腐烂），或更正为
76/86。

  ocr-2 replay copy, executed against the docs/plans/issue-72-ocr2-replay/
  evidence directory. Relative to the ocr-1 copy (86 lines) this copy adds
- two ocr-2-specific assertion annotations: "# ocr-2:" at lines 74 and 84
+ two ocr-2-specific assertion annotations: the "# ocr-2:" comments at the
+ AC2 deferred re-check and the AC3 gate-retry assertions
  (R1-V23: the previous note here wrongly claimed "unmodified").


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:9-12 ───
[documentation · medium] docstring 对退出码 2 的触发条件描述与实际行为矛盾：此处声称 2 仅在"无归档且 runs/ 下无
db-watch-verify-*.tsv"时出现，且第 30-31 行注释声明"无归档时回退使用 runs/ 最新 TSV"；但第 108-112 行的 R1-V10 守卫 `if not
archived.exists(): sys.exit(2)` 使任何无归档场景——包括 matches 非空、已选定 path、打印 `using <path>` 并完整读入解析该
TSV——都必然以 2 退出。后果：(1) `elif matches`
分支选出的文件永远不会用于断言，其后的读取与解析（boundary、rows_for、max_succeeded）是无效计算；(2) 输出先打印 "using <runs tsv>" 再打印
WARNING 退出，自相矛盾；(3) 审计者/流水线依据 docstring 会把退出码 2 误读为"该环境从未运行过 db_watch"，而实际含义是"runs/ TSV 存在但对 run 级
exactly-once 断言不可判定"。建议将无归档守卫提前到文件选择处尽早 sys.exit(2)（消除无效计算），并同步修正 docstring：退出码 2 = 本目录无归档
verify-db-watch-samples.tsv（runs/ 回退 TSV 无法把全库 payments 聚合键到本次运行，不可作为断言输入）。

  Exit codes: 0 PASS, 1 CHECK (an assertion failed), 2 MISSING-EVIDENCE (no
- usable TSV: no archived verify-db-watch-samples.tsv next to this script and
- no db-watch-verify-*.tsv under runs/ — that directory is a git-ignored
- runtime artifact, so a fresh clone has none until db_watch.sh is replayed).
+ archived verify-db-watch-samples.tsv next to this script; a runs/
+ db-watch-verify-*.tsv fallback is not usable — it cannot key the whole-DB
+ payments aggregate to this run, so the exactly-once assertion is not
+ decidable from it and the script degrades loudly instead).
+ 
+ # 并将文件选择处的 fallback 提前降级，消除无效计算：
+ # if archived.exists():
+ #     path = str(archived)
+ # elif matches:
+ #     print("DB-WATCH: WARNING runs/-fallback TSV in use — payments are a "
+ #           "whole-DB aggregate and cannot be keyed to this run, so the "
+ #           "exactly-once max_succeeded assertion is not decidable")
+ #     sys.exit(2)
+ # else:
+ #     print(f"DB-WATCH: no observer TSV under {RUNS} (db-watch-verify-*.tsv; "
+ #           "runs/ is git-ignored) and no archived verify-db-watch-samples.tsv "
+ #           "next to this script")
+ #     sys.exit(2)


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:10-13 ───
[documentation · low] docstring 中的行号溯源声明与文件实际不符:实际第二处 "# ocr-2:" 注解在第 105 行(非 99),第二处 "# ocr-3:" 在第
111 行(非 105)——插入 R1-37 存在性守卫块(第 90-94 行)后,后续注解整体偏移了 6 行但行号未同步更新;且 ocr-2 副本实际约 99 行(非 "97 lines")。该
docstring 正是为执行 R1-V23(纠正前一副本错误的溯源声明)而写的,自身却携带失实行号,会直接误导后续 OCR
重放审计。建议改为与实际一致的行号,或在后续重放副本中改用不依赖行号的锚定描述(如引用 check 名称而非行号),避免每次插入守卫代码后行号漂移。



─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:14-16 ───
[documentation · medium] docstring 中的增量声明与事实不符:"This copy additionally guards every bare
all(...values()) with an existence check (ocr-81 R1-15)" 处于 "Derived from the ocr-2 copy"
的增量清单段落(keeps… adds… additionally…),读作本副本新增了这些守卫。但核对原件:ocr-1 副本(自述 "unmodified" 的基础脚本)第 39/66/78/83
行与 ocr-2 副本第 41/68/85/96 行,全部四处 all(...values()) 调用(env.all_phases_pass、AC2 checks、AC3
checks、decline checks)均已带 bool(...) 存在性守卫;ocr-3 对这些调用未做任何守卫改动,仅新增了 "# ocr-81:" 描述性注释(第 72-74 行)和针对
invoice-count 检查的 R1-37 守卫(第 90-98 行)。该句会把继承自基础脚本的守卫误记为 ocr-3 引入的变更,误导后续 OCR 重放审计(与 R1-V23
纠正的失实溯源同类,且修正行号后此句仍会残留)。建议改为陈述现状并明确守卫并非本副本新增。

- note here wrongly claimed "unmodified"). This copy additionally guards every
- bare all(...values()) with an existence check (ocr-81 R1-15): an empty checks
- dict is a FAIL, never a vacuous pass.
+ note here wrongly claimed "unmodified"). Every all(...values()) in this
+ copy is guarded with an existence check — present since the base script and
+ unchanged from ocr-2; this copy only adds the "# ocr-81:" annotation
+ documenting it (ocr-81 R1-15): an empty checks dict is a FAIL, never a
+ vacuous pass.


─── internal/handler/commercial.go:531-534 ───
[bug · low] default 分支以 400 返回 err.Error() 原文，与本文件 Purchase 的失败姿态（闭合文案 "purchase failed" +
500、原始错误仅进服务端日志）不一致。当前 PurchaseService 仅在 tenantID==0 时返回非 nil 错误（已被上面的 403
租户守卫拦截），分支暂不可达；但一旦未来从这里冒出存储层错误，会把内部错误细节暴露到公共边界，并以 400 误导客户端按自身错误重试。建议对齐 POST 路径的闭合 500 + 日志写法。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:620-622 ───
[bug · low] attach 与 default 更新是两次串行的跨洲往返（t09 证据单程 ~3.5s），却共享同一个 15s 预算；而 purchaseRequestTimeout
的预算契约（及测试 TestPurchaseRequestTimeoutBudgetCoversProviderChainAndSyncWait）为三次 provider 往返"each
bounded by outboundProviderTimeout"各分配 15s。共享预算下，一次缓慢的 attach（如 10s+）会把 default 更新挤到剩余时间不足一个
RTT，context deadline 超时被误报为 provider
不可达，触发整条幂等链路（绑定读+身份读+create）重跑，带来不必要的用户可见延迟。建议为两次调用各自派生独立预算，与预算契约注释一致。

  func (a *LagoAdapter) providerAttachDefaultPaymentMethod(ctx context.Context, providerCustomerID, pmToken string) error {
- 	ctx, cancel := context.WithTimeout(ctx, outboundProviderTimeout)
- 	defer cancel()
+ 	attachCtx, attachCancel := context.WithTimeout(ctx, outboundProviderTimeout)
+ 	defer attachCancel()
+ 	status, data, err := a.providerOutboundCall(attachCtx, "/v1/payment_methods/"+url.PathEscape(pmToken)+"/attach",
+ 		"customer="+url.QueryEscape(providerCustomerID), "")
+ 	// ……attach 处理……
+ 	updateCtx, updateCancel := context.WithTimeout(ctx, outboundProviderTimeout)
+ 	defer updateCancel()
+ 	status, _, err = a.providerOutboundCall(updateCtx, "/v1/customers/"+url.PathEscape(providerCustomerID),
+ 		"invoice_settings[default_payment_method]="+url.QueryEscape(attached.ID), "")


─── internal/modules/commercial/commercialplatform/lago_purchase.go:607-609 ───
[bug · low] 两个问题：(1) 丢弃 io.ReadAll 的读取错误后，"2xx 但被传输层/代理截断的响应体"与"provider 返回畸形 JSON"不可区分——前者会被归类为终态的
ErrPlatformInvalidResponse（如 provider customer create malformed），而真实原因是瞬时的网络截断，按 fail-closed
契约应为可重试的 unreachable；客户端若按 reason=invalid_response 视为终态，会对一次本可重试成功的购买直接放弃。建议像 a.do 之外的语义一样区分读取错误并归类为
unreachable。(2) providerOutboundCall 每次调用通过 providerOutboundClient() 新建 http.Client，一条购买链路最多三次对同一
host 的出站调用各自完成 TLS 握手、无连接复用；可考虑包级复用 client（保留 CheckRedirect 策略）。

  	defer resp.Body.Close()
- 	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
+ 	data, readErr := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
+ 	if readErr != nil {
+ 		return 0, nil, fmt.Errorf("%w: outbound provider response truncated", commercial.ErrPlatformUnreachable)
+ 	}
  	return resp.StatusCode, data, nil


─── internal/modules/commercial/payment/alipay.go:572-574 ───
[bug · low] 此处 SSRF 预检拒绝包装的是微信渠道的哨兵 ErrNotConfigured（wechat.go:60 定义，错误串为 "wechat_not_configured"），而
alipay.go 其余所有 fail-closed 路径（NewAlipayProvider、QueryRefund、merchantKey）均使用
ErrAlipayNotConfigured（"alipay_not_configured"）。当支付宝网关配置指向未豁免的私有/环回地址时，支付宝订单的 CheckoutError 会以
"wechat_not_configured" 呈现（openOrder 将 err 全文写入 CheckoutError 返回给客户端），渠道归因错误、误导排障；未来任何按
errors.Is(err, ErrAlipayNotConfigured) 分支的调用方也不会命中此路径。建议改用 ErrAlipayNotConfigured 保持渠道哨兵一致。

  	if err := secutils.ValidateURLForSSRF(p.cfg.gateway()); err != nil {
- 		return fmt.Errorf("%w: gateway url failed SSRF validation: %v", ErrNotConfigured, err)
+ 		return fmt.Errorf("%w: gateway url failed SSRF validation: %v", ErrAlipayNotConfigured, err)
  	}


─── internal/modules/commercial/payment/providers_env.go:58-61 ───
[maintainability · low] 本次修复将 alipayConfigFromEnv 的 partial-config 错误提示改为确定性输出，但同一文件下方的
wechatConfigFromEnv（本函数之后的微信侧解析）仍使用完全相同的 map[string]*string
随机遍历模式拼装缺失变量名——微信渠道部分配置时错误信息依然跨运行不稳定，与本次修复的动机相悖。建议同步将 required map 改为固定顺序的 slice，保持两个渠道的错误契约一致。

- 	// Fixed iteration order (slice, not map) so the partial-config error
- 	// names a DETERMINISTIC variable — map iteration made the message flaky
- 	// across runs.
+ 	// wechatConfigFromEnv 应同步应用相同模式：
  	set := []struct {
+ 		name string
+ 		dst  *string
+ 	}{
+ 		{"WEKNORA_WECHAT_APP_ID", &cfg.AppID},
+ 		{"WEKNORA_WECHAT_MCH_ID", &cfg.MchID},
+ 		{"WEKNORA_WECHAT_MCH_SERIAL", &cfg.MchSerial},
+ 		{"WEKNORA_WECHAT_MCH_KEY_PATH", &cfg.MchKeyPath},
+ 		{"WEKNORA_WECHAT_APIV3_KEY_PATH", &cfg.APIv3KeyPath},
+ 	}


─── internal/modules/commercial/repository/commercial/order.go:156-158 ───
[bug · medium] 新哨兵 ErrPurchasePendingExists 同样会从遗留路径 POST /commercial/orders 逃逸（CreateOrder 创建的也是
kind=purchase 订单，旧路径没有 PurchaseService 那样的 pre-check），而 internal/handler/commercial.go 的 CreateOrder
错误分支没有映射该哨兵，落入 default 返回 400 + 原始 token "purchase_pending_exists"，且响应不带任何可回放的订单数据（此索引生效前，第二个新报价的
checkout 是允许成功的）。应在 CreateOrder 处理器中映射 ErrPurchasePendingExists（409 并回放现有 pending 订单），与
PurchaseService 的 ErrQuoteAlreadyUsed/ErrPurchasePendingExists 处理对齐。



─── internal/modules/commercial/repository/commercial/order.go:354-357 ───
[bug · high] "可支付"（payable）判定链不一致，且迁移未处理存量行，部署即可把租户的购买流程永久锁死。CurrentPendingPurchaseOrder 明确以
`checkout_url <> ''` 排除 link-less 行（注释承诺 "a channel-failed or link-less pending order is not a
payment entry and does not block this checkout"），但部分唯一索引谓词（只排除 channel_failed）与本函数都不含该条件。于是任何
pending + channel_failed=false + checkout_url 为空的行都会：(1) 命中 uq_purchase_pending_per_tenant，阻塞该租户所有新
checkout（commercial 模块没有任何 pending 订单过期/清理任务，无法自愈）；(2) 被本函数当作"胜者订单"重放，而 orderViewFromRow 不带
CheckoutURL，客户端拿到一个无法支付、也永远无法重建的干净 201 应答。即使按已确认发现修复 ctx 复用问题，仍存在三个残留触发源：(a) SetCheckoutURL 因非 ctx
的普通 DB 错误失败（R2-27 降级路径，渠道调用已成功）；(b) 渠道 Create 成功但返回空 CheckoutURL 时 SetCheckoutURL 拒绝空串，同样留下
link-less 行；(c) 最确定的一条——本次迁移前创建的所有 pending purchase 订单（POST /orders 旧管线）：checkout_url 列新加、存量行全为
NULL/空，channel_failed 新列以默认 false 回填，部署当天这些行即落入索引作用域并触发上述锁死。另注：存量行 created_at 为 NULL，PG 上 `ORDER BY
created_at DESC` 默认 NULLS FIRST，CurrentPurchaseOrder/CurrentPayablePendingOrder
会进一步优先选中这些旧行。建议：统一三处判定（索引谓词或重放读取补 checkout_url 条件需权衡降级行的二次扣款风险，至少对 link-less
行给出可恢复的闭合错误而非干净重放），并在迁移时显式处置存量 pending purchase 行（标记 channel_failed、回填 created_at 或提供解除路径）。



─── internal/modules/commercial/service/commercial/order.go:112-116 ───
[bug · critical] PostgreSQL 部署启动必失败：GORM 在 PG 上将 OrderRow.ChannelFailed（bool）迁移为 boolean 列，而 PG
没有boolean = integer 的隐式转换，`WHERE ... channel_failed = 0` 会在 CREATE INDEX 解析期直接报 `operator does not
exist: boolean = integer`。DB_DRIVER=postgres 是受支持的生产驱动（internal/container/container.go
initDatabase），此错误使 NewOrderService 返回错误、容器 must() panic，服务完全无法启动（仅 SQLite 部署可启动）。注释 "SQLite and
PostgreSQL share this partial-index syntax" 不成立（本仓库 internal/application/repository/chunk.go 也已有针对
PG bool 比较的兼容先例）。改为 `channel_failed = false`（SQLite 3.23+ 与 PG 均支持该字面量），或按 db.Dialector.Name() 分支生成
DDL。

  	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_purchase_pending_per_tenant
  		ON commercial_orders (tenant_id)
- 		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = 0`).Error; err != nil {
+ 		WHERE kind = 'purchase' AND state = 'pending' AND channel_failed = false`).Error; err != nil {
  		return nil, fmt.Errorf("commercial pending-purchase invariant: %w", err)
  	}


─── internal/modules/commercial/service/commercial/order.go:370-372 ───
[bug · high] MarkChannelFailed（以及成功路径的 SetCheckoutURL）复用请求级 ctx：当渠道 Create 正是因客户端断连/ctx
取消而失败时，标记写入同样以 context.Canceled 失败，订单残留 pending、channel_failed=false、checkout_url=''。该僵尸行命中
uq_purchase_pending_per_tenant 部分索引（索引只排除 channel_failed），并被 CurrentPayablePendingOrder（不过滤
checkout_url）当作"胜者订单"原样重放：租户后续任何新报价都撞索引、收到无支付链接也无 CheckoutError 的"干净 201"；而 OrderStore 仅有
ConfirmPayment/MarkFulfilled 两个状态出口、无过期/清理路径，购买链路被永久卡死，只能手工修库。SetCheckoutURL 降级（R2-27）的行同样是
channel_failed=false 且无链接，阻塞方式相同。建议这两个持久化写入改用脱离请求取消的 ctx（context.WithoutCancel 或带超时的独立 ctx），并为长期无链接的
pending 订单提供恢复/超时出口。



─── internal/modules/commercial/service/commercial/purchase.go:217-222 ───
[bug · low] 同一 quote 的重试重放不区分 channel_failed 订单：首次应答是 202 + CheckoutError（渠道调用失败），客户端用同一 quote_id 重试
POST /purchases 得到的却是"干净 201"——orderViewFromRow 不合成 CheckoutError，重放的是一个无链接、无错误的 pending
订单，写应答姿态自相矛盾，客户端也无法从重放答案判断渠道失败。建议 orderViewFromRow 对 ChannelFailed 行合成
CheckoutError（或重放时排除不可支付行），使重试答案与首次 202 姿态一致。



─── packages/api-client/src/commercial.ts:66-66 ───
[maintainability · low] purchase 入参以内联字面量类型定义，provider 联合类型 'wechat' | 'alipay' 与 contracts 中
CreateOrderInput 的同名字段重复维护，且未像 CreateOrderInput 一样在 contracts 导出命名类型——后续新增支付渠道时需要在多处同步修改。建议在
contracts 定义并导出 PurchaseInput（或提取共享的 PaymentProvider 类型别名）供 api-client 引用。

-     async purchase(input: { quote_id: string; provider: 'wechat' | 'alipay' }, signal?: AbortSignal): Promise<PurchaseView> {
+     async purchase(input: PurchaseInput, signal?: AbortSignal): Promise<PurchaseView> {
+       // contracts: export interface PurchaseInput { quote_id: string; provider: 'wechat' | 'alipay' }


─── packages/api-client/src/errors.ts:74-77 ───
[bug · medium] reason 合并逻辑对 details 形态的边界处理有隐患：1) details 为数组时，`typeof [] === 'object'` 成立，数组会被 `{
...base, reason }` 展开成 `{0:'a', 1:'b', reason}` 这样的索引键对象，原有数组语义静默丢失；2) details 为字符串时被整体丢弃替换为
`{reason}`。当前唯一携带顶层 reason 的响应是购买 503 信封（无 details，且 CheckoutPage 只读 `.reason`），所以暂无实际触发方，但
errorFromResult 是所有非 2xx 响应的公共构造路径，未来任何端点同时携带数组/字符串型 details 与顶层 reason
都会踩坑。建议至少排除数组（丢弃也优于伪装成对象），更彻底的做法是把 reason 作为 ApiError 的独立字段而非混入 details。

    if (reason !== undefined) {
-     const base = typeof details === 'object' && details !== null ? details as Record<string, unknown> : {};
+     const base = typeof details === 'object' && details !== null && !Array.isArray(details)
+       ? details as Record<string, unknown>
+       : {};
      details = { ...base, reason };
    }


─── packages/contracts/src/commercial.ts:115-118 ───
[maintainability · low] currency 字段在同一契约文件内存在两套宽松度不一致的解析语义：parseQuoteView 中 undefined 跳过、null/空串经
nonEmptyString 抛错；parsePurchaseView 中 undefined/空串跳过、null 抛错（amount_fen 则是
undefined/null/空串三者均跳过）。同名wire字段在相邻解析器间规则漂移，后续维护易引入分歧。另外此处丢弃 nonEmptyString 的校验返回值再用 `as string`
断言，属于多余的双重表达——直接使用返回值即可。建议统一为一个 optionalNonEmptyString 辅助或在两处对齐跳过规则。

   if(v.currency!==undefined) {
-   nonEmptyString(v.currency,'currency','quote');
-   out.currency = v.currency as string;
+   out.currency = nonEmptyString(v.currency,'currency','quote');
   }


─── packages/contracts/src/commercial.ts:133-135 ───
[maintainability · low] line_items 校验与接口注释宣称的契约强度不符：注释称 kind 是闭集 subscription_fee token，实现仅做
nonEmptyString 未限制取值（若为未来 kind 扩展有意预留，建议在注释中写明）；name 为非字符串时静默置 '' 而非抛错，宽松解析会掩盖前后端契约漂移（例如后端误发 null
name 时前端渲染空标题而非失败暴露问题）。后端 quoteWire 固定输出字符串 name，前端收紧为 nonEmptyString 不会误伤现有响应。

     return { kind: nonEmptyString(li.kind,'kind','quote line item'),
-     name: typeof li.name==='string'?li.name:'',
+     name: nonEmptyString(li.name,'name','quote line item'),
      amount_fen: digitString(li.amount_fen,'amount_fen','quote line item') };


─── packages/contracts/src/commercial.ts:40-41 ───
[maintainability · low] checkout_url 的空串分支与"缺失即省略"语义不一致：条件中 `v.checkout_url!==''`
使空串跳过校验并原样保留在返回视图上。而后端 orderWire 对未支付订单固定输出 `checkout_url: ""`（无 omitempty），因此前端拿到的 OrderView
常态携带空串而非缺省，与接口注释 "absent while unpaid/unconfigured" 相悖。渲染层 falsy
检查使其暂无功能性危害，但建议让空串也进入删除分支，统一"非安全或空即缺省"的语义。

-  if(v.checkout_url!==undefined&&v.checkout_url!==null&&v.checkout_url!==''&&
+  if(v.checkout_url!==undefined&&v.checkout_url!==null&&
      !isSafeCheckoutUrl(String(v.checkout_url))) {


─── packages/contracts/src/commercial.ts:52-52 ───
[security · low] isSafeCheckoutUrl 的定位是防钓鱼/防危险 scheme 的第二道防线，但白名单放行了明文 http:// 支付入口链接。主流渠道的支付入口（微信
Native 的 weixin://wxpay/ 深链、支付宝/收银台的 https:// 链接）均不会返回明文 http 链接，而 http
链接在传输中可被中间人替换为钓鱼页——与该白名单的防钓鱼目标自相矛盾。建议将 http(s) 分支收紧为仅 https（SAFE 正则去掉 `http`，保留 `https?` 中的 `s`
必选），weixin/alipay 深链分支不受影响。

-  const SAFE = /^(https?:\/\/|weixin:\/\/wxpay\/|alipayqr:\/\/platformapi\/|alipays:\/\/platformapi\/)/i;
+  const SAFE = /^(https:\/\/|weixin:\/\/wxpay\/|alipayqr:\/\/platformapi\/|alipays:\/\/platformapi\/)/i;


─── deploy/lago-lab/payment-activation/clients.py:400-404 ───
[maintainability · low] 死存储：`last_error` 在循环前初始化并在 except 分支赋值，但从未被读取——最终传输失败路径直接 `raise`
当前异常，不经过该变量。按死代码处理建议删除两处赋值，减少阅读噪音。

-             except (URLError, OSError) as error:
-                 last_error = error
+             except (URLError, OSError):
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)
+ # 同时删除循环前的 `last_error = None`


─── deploy/lago-lab/payment-activation/phases.py:147-150 ───
[maintainability · low] 非传输类失败未按注释宣称的方式归因：/api/v1/organizations 返回非 2xx（401/500/404 等）时不走 except
分支，`_ok(status)` 为假即静默返回 None 且不记 note。这与 except 分支注释"Not silently swallowed (ocr-3) ... keeps that
failure attributable in the report notes instead of a bare FAIL"矛盾——此时 GraphQL 变更仍会以裸 400 'Missing
organization id' 失败，报告里找不到根因指向 organizations 解析失败。建议在 else 分支补一条与传输失败同风格的 note。

                  status, body = self.lago.request("GET", "/api/v1/organizations")
                  org = ((body or {}).get("organization") or {})
                  if _ok(status) and org.get("lago_id"):
                      self._graphql_organization_id = org["lago_id"]
+                 else:
+                     self.note(
+                         "x-lago-organization resolution returned HTTP "
+                         f"{status} (lago_id={'present' if org.get('lago_id') else 'missing'}); "
+                         "header omitted — GraphQL mutations may be rejected "
+                         "with 'Missing organization id'"
+                     )


─── deploy/lago-lab/payment-activation/phases.py:1330-1337 ───
[performance · low] 上方 `ctx.poll(canceled, lambda r: r[0], "customer C cancellation")`
的唯一收敛条件是本段注释自述在 timeout_hours: 0 下实验窗口内不可达的 canceled 状态，因此默认运行每次都会耗尽完整 poll_timeout（run_lab 默认
300s；已归档证据 t02-decline.json 即为 poll exhausted attempts=199 后才走本分支判 pass），叠加 duplicates 阶段 120s
延迟复查显著拉长单次运行。建议让轮询谓词同时接受"incomplete 状态已持续保持 stability_rounds × stability_delay"作为收敛终态（仍保留中途出现
canceled 的即时收敛），或为本阶段设置显著更短的等待上限。



─── deploy/lago-lab/payment-activation/phases.py:1256-1264 ───
[test · medium] retries 的即时窗口 checks 与 duplicates 新增的延迟复查不对称：探针 (1) 对 customer B 的 active 订阅执行了与
phase_duplicates 探针 1 完全相同的重复注册 re-POST（归档证据 t02-retries.json 中 re_post_http_status=200），而本次变更为
duplicates 增加 120s 延迟复查的依据正是『一次 200 应答的重复注册曾数分钟后终止订阅并开具新发票』。若本阶段自己的 re-POST 触发同样的延迟漂移，retries 仍会判
pass——危害要么以无关的 cleanup 失败形式出现，要么（终止后状态落在 cleanup 的候选状态内时）完全不被察觉，实验对 AC3『不产生第二个商业对象』的结论因此可能是错的。建议在
retries 末尾复用 ctx.duplicates_settle_delay 对 B 的权威状态（active + same lago_id + 无新发票）做同样的延迟复查，并考虑在
cleanup 的状态候选中加入 terminated。



─── deploy/lago-lab/payment-activation/phases.py:540-544 ───
[performance · low] 该轮询的收敛条件（门控发票变为 API 可见）在发票停留于 open（本文件自述的
INVISIBLE_STATUS）期间结构性不可满足，而这恰是本阶段两个文档化 PASS 结局之一（『跨认证挑战窗口保持 incomplete』）。归档证据 t02-gating.json
证实：'poll exhausted: gating invoice creation for customer A (attempts=172)' 之后才走 invisible 分支判
pass——即当前 Stripe 行为下每次成功运行都会先烧满整个 poll_timeout（run_lab 默认 300s）。这是已确认的 decline_control
轮询不可达问题的同族但独立的一处，且耗尽备注 "gating invoice creation" 把『API
不可见』误述为『未创建』（发票即时就创建了）。建议：为这段观察窗口引入专用（更短的）旋钮（如 duplicates_settle_delay 的做法），或让谓词在订阅离开
incomplete（canceled 可观测）时也提前收敛，并把耗尽备注改为 "gating invoice stayed API-invisible" 以免误导报告读者。


