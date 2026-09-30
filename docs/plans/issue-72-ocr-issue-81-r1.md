Review complete: 37 finding(s) across 84 selected item(s).

─── apps/web/src/commercial/BillingPage.tsx:79-84 ───
[bug · medium] purchase 状态在 effect 重新执行时缺少重置：切换单位/空间（或 Reload）时，state 和 usageState 都会先回到 Loading 态，但
setPurchase 没有同步清空。若上一个空间的 purchaseStatus 为 awaiting_payment，而新空间的 summary 先于 purchaseStatus
返回，新空间的套餐行会短暂拼上旧空间的「 · 待付款（权益未开放）」，形成跨空间的错误展示（竞态窗口内）。建议在 effect 开头与另外两个重置一起 setPurchase(null)。

      // #81：并行读取购买状态；失败静默降级（待付款行只是缺席，不阻塞账单页）。
+     // 作用域切换/重载时先清空旧值，避免旧空间的「待付款」短暂落到新空间的套餐行上。
+     setPurchase(null);
      void client.commercial.purchaseStatus(scope.signal).then((view) => {
        if (active && scopeController.isCurrent(scope.scope)) setPurchase(view);
      }).catch(() => {
        if (active && scopeController.isCurrent(scope.scope)) setPurchase(null);
      });


─── apps/web/src/commercial/CheckoutPage.tsx:82-83 ───
[bug · high] 空字符串回退 getOrder('') 必然失败且丢弃 reason：后端 Purchase() 在平台失败时以 err==nil 返回 201 +
{state:'absent', reason:'unreachable'|'invalid_response'|'unconfigured'} 且 Order 为 nil（pendingView
路径），getOrder('') 会请求 /api/v1/commercial/orders/（encodeURIComponent('') 为空）导致 404，用户只看到通用错误，闭合词表
reason 完全无法到达界面，违背「平台失败以闭合词表呈现」的验收要求。应按 purchase.state/reason 分支处理：无 order 时直接以映射后的文案进入 error 态（如
unconfigured→支付渠道未配置、unreachable→支付渠道暂不可达、invalid_response→支付渠道响应异常、unsupported→支付渠道不支持），而非回退
getOrder。

          // From here on this page only re-queries the same order id; it never creates another order.
-         const order = purchase.order ?? (await client.commercial.getOrder('', currentScope.signal));
+         if (!purchase.order || purchase.state !== 'awaiting_payment') {
+           // 平台失败/未配置以闭合词表呈现（state + reason），不回退 getOrder('')。
+           if (active && scopeController.isCurrent(currentScope.scope)) {
+             setState({ status: 'error', message: purchaseReasonMessage(purchase) });
+           }
+           return;
+         }
+         const order = purchase.order;


─── apps/web/src/commercial/CheckoutPage.tsx:151-154 ───
[bug · medium] 「待付款（权益未开放）」Status 在 ready 态无条件渲染：点击「刷新订单状态」或轮询到达终态后（orderMessage
会显示「已付款，权益处理中」「权益已生效」「订单已关闭」），同一 section 里仍并列显示「待付款」，UI 状态与真实支付状态自相矛盾；携带已支付 orderId 挂载本页时同样出错。应按
order.payment === 'pending' 条件渲染。

- {/* AC3：购买成功后产品状态为「待付款」，付费权益未开通。 */}
+               {/* AC3：仅未支付时展示待付款；支付完成后随 orderMessage 切换。 */}
+               {state.order.payment === 'pending' ? (
-               <p>
+                 <p>
-                 <Status>待付款（权益未开通）</Status>
+                   <Status>待付款（权益未开放）</Status>
-               </p>
+                 </p>
+               ) : null}


─── apps/web/src/commercial/CheckoutPage.tsx:156-160 ───
[security · medium] checkout_url 未经任何校验直接渲染为 <a href>：该 URL 来自渠道响应（wechat CodeURL / alipay
QRCode），服务端 validateOutboundHost 只约束服务端出站请求，浏览器侧链接全程无校验；parseOrderView 是 v as unknown as OrderView
整体透传，是本 diff 中唯一零校验的新增契约字段（同 diff 的 parseQuoteView/parsePurchaseView 都做了严格校验）。React 不会拦截 javascript:
伪协议 href，被污染的渠道响应可注入脚本或钓鱼链接。建议至少在渲染侧限定 http(s)，更彻底的做法是在 parseOrderView 中校验 checkout_url 的 scheme 为
http/https 再透传。

-               {state.order.checkout_url ? (
+               {state.order.checkout_url && /^https?:\/\//i.test(state.order.checkout_url) ? (
                  <p>
                    <a href={state.order.checkout_url} target="_blank" rel="noreferrer">前往支付</a>
                  </p>
                ) : null}


─── apps/web/src/commercial/CheckoutPage.tsx:153-153 ───
[style · low] 同一个 awaiting_payment 状态在两个页面的文案一字之差：CheckoutPage 用「待付款（权益未开通）」，BillingPage
用「待付款（权益未开放）」。用户在支付后跳回账单页会先后看到两种表述，产品词表应保持稳定（spec L210
精神）。计划文档对两处各自断言了这两个字符串，若确属刻意区分建议在注释中说明理由，否则建议统一为同一词表条目。

-                 <Status>待付款（权益未开通）</Status>
+                 <Status>待付款（权益未开放）</Status>


─── deploy/lago-lab/payment-activation/clients.py:390-391 ───
[maintainability · low] `last_error` 是死存储：它在循环内被赋值但从未被读取——最后一次尝试失败时直接 `raise`
重新抛出当前异常对象，前面的值不会用到。建议删除 `last_error = None` 与 `last_error = error` 两行（或在最终抛出时改用 `raise ... from
last_error` 使其真正参与异常链）。

-         last_error = None
          for attempt in range(transport_retries + 1):
+             try:
+                 with self._opener.open(request, timeout=self._timeout) as response:
+                     status = response.status
+                     raw = response.read()
+                 break
+             except HTTPError as error:
+                 raw = error.read()
+                 return error.code, _parse_json_body(raw)
+             except (URLError, OSError):
+                 if attempt == transport_retries:
+                     raise
+                 time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/phases.py:1038-1040 ───
[test · medium] deferred re-check 的 settle 窗口与其守护目标失配：代码注释与 expected 文本明确指出该防线针对的是"200
应答的重复注册在数分钟后（minutes later）终止订阅并开出续期发票"的迟到漂移（ocr-2），但这里复用了稳定性窗口 `stability_rounds *
stability_delay`，而 run_lab.py 的默认值为 3 × 5.0s = 15 秒——比文档描述的漂移时延低约两个数量级。默认配置下 PASS 结论仍可能重演 ocr-2
的假阳性。建议为该 re-check 引入独立的、分钟级的 settle 窗口参数（新的 RunContext/CLI 参数，默认约 120–300 秒），而不是复用稳定性旋钮；至少应使用
`max(...)` 保证一个下限。

          deferred = {"checked": False, "ok": None}
          if probes_harmless and state_intact:
-             time.sleep(ctx.stability_rounds * ctx.stability_delay)
+             # ocr-2 的漂移是 minutes 级；稳定性旋钮默认 3*5s=15s 远低于它，
+             # 需要独立的分钟级 settle 窗口（如 ctx.duplicate_settle_seconds，
+             # 默认 120-300s，可由 CLI 覆盖）。
+             time.sleep(ctx.duplicate_settle_seconds)


─── deploy/lago-lab/payment-activation/phases.py:568-574 ───
[test · medium] phase_gate 发票不可见的 PASS 分支漏掉了 expected 中仍然声明的断言：expected 文本承诺 "at most one
non-succeeded payment whenever the API can see it"，但这条分支（也是证据 t02-gating.json
实际走到的主路径，invoice_api_visible=false → pass）从未调用 _payments_for，payments_non_succeeded_count
完全缺失，"至多一条非成功支付"（AC1 的 exactly-once 防线）在此路径上无任何断言。该豁免理由不成立——payments 接口
(/api/v1/payments?external_customer_id=...) 不受发票 INVISIBLE_STATUS 影响，同文件的 phase_decline_control
在相同的发票不可见状态下就读取并断言了 payments（no_succeeded_payment）。建议在此分支补上 payments 读取并纳入 PASS 条件，或将 expected
文本收窄为与实际断言一致，避免 PASS 结论弱于报告自身声明的合同。

-             if core_ok and still_incomplete:
+             ps, payments = _payments_for(ctx, customer["external_id"])
+             non_succeeded = [p for p in payments if p.get("status") != "succeeded"]
+             observed["payments_non_succeeded_count"] = len(non_succeeded)
+             if core_ok and still_incomplete and len(non_succeeded) <= 1:
                  ctx.note(
                      "gating invoice stayed API-invisible (v1.53.0 INVISIBLE_STATUS "
                      "open); AC1 asserted from subscription incomplete + "
                      "entitlements 404 held across the window"
                  )
                  return _report(ctx, "gate", expected, observed, PASS)


─── docs/plans/issue-72-flow-evidence-74/verify_ac_assertions.py:30-32 ───
[test · low] 非阻塞健壮性建议：脚本对证据 JSON
大量使用直接下标访问（env["run"]["overall"]、oa["checks"]、d["observed"]["final"]、r["observed"]["checks"]
等），一旦某次重放的证据文件缺键或 schema 漂移，脚本会以 KeyError traceback 崩溃，跳过其后所有 AC 断言，失败原因也不表现为具名 FAIL 条目——与 check()
收集 fail 列表的设计以及已有的失败闭合写法 `o.get("payments_non_succeeded_count", 99)`
不一致。已逐一核对当前本目录七个证据文件，这些键路径全部存在，本次运行不受影响；建议对跨文件取值统一改用 .get() 并将缺键纳入具名 check（如
`oa.get("payments_succeeded_count") == 1`），确保每个 AC 的失败都能被点名输出。

- env = load("t02-environment")
- check("env.overall==pass", env["run"]["overall"] == "pass")
- check("env.release==v1.53.0", env["release"]["release"] == "v1.53.0")
+ check("AC2 exactly one succeeded payment", oa.get("payments_succeeded_count") == 1)
+ check("AC2 provider_payment_id recorded", bool(oa.get("provider_payment_id")))
+ # 其余直接下标访问同理改用 .get()，缺键时走具名 FAIL 而非 KeyError


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:60-65 ───
[bug · medium] max_succeeded() 与 rows_for() 的隔离级别不一致：rows_for() 在本脚本中专门用 run_prefix 过滤并在注释中防御"多 run
TSV / 旧回放残留泄漏外来状态"，且受 only_midrun 边界约束；但 max_succeeded() 遍历全部采样行，既不应用 midrun 边界，也无任何运行隔离。而观察者对
payments 的采样是全库聚合（实测归档 TSV 为 payments[failed|1;pending|1]，无 run 键，无法按前缀过滤）。因此当走 runs/ 回退路径（本目录归档 TSV
缺失时取字典序最新的 db-watch-verify-*.tsv，可能属于另一次 replay）或共享 lab DB 残留早前回放的 succeeded 付款时，外来计数会泄漏进
`max_succeeded() == 1` 这一 exactly-once 不变量，对本目录 run 产生误报 CHECK 失败，与注释宣称的多运行隔离意图不一致。当前执行不受影响（本目录归档
TSV 仅含单一 run），但该缺陷与 issue-72-ocr-round-4.md 中对各 replay 副本指出的完全是同一处，修复未同步到本文件。建议至少补上与 rows_for 一致的
only_midrun 过滤；对无法按 run 前缀隔离的全局计数，在回退到外来 runs/ TSV 时对该不变量显式告警或按 MISSING-EVIDENCE（exit
2）处理，避免用不可隔离的数据源支撑结论。

- def max_succeeded():
+ def max_succeeded(only_midrun=True):
      best = 0
      for ln in lines:
+         if only_midrun and not midrun(ln):
+             continue
          m = re.search(r"payments\[([^\]]*)\]", ln)
          if not m:
              continue


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:64-73 ───
[bug · low] max_succeeded() 未做 run 隔离，与 rows_for() 不一致：rows_for() 用 run_prefix 过滤外来 run 的订阅行，但 TSV 的
payments[...] 字段是无 run 归属的全局聚合计数（对比 rows[weknora-t02-<run_id>-sub-x|N]），外来 run 的 succeeded|N
会直接计入峰值。runs/ 回退路径（matches[-1]）选中的『最新 TSV』正是多 run 高危场景——本文件自己的注释也指出这一点——此时 ok 判定中的 max_succeeded()
== 1 会被外来成功扣款污染而产生伪 CHECK（失败方向保守，但破坏可复现性，也与 49-52 行声称的多 run 防护矛盾）。当前 ocr1 归档 TSV 为单 run（仅 dfcc76a8
前缀）故未触发，属回退路径上的潜在缺陷。由于 payments 字段本身无 run 键无法直接过滤，建议在选中 TSV 后检测 rows 中是否存在非本 run 的 weknora-t02-
前缀，存在时显式判 CHECK 退出，而非让全局峰值静默参与判定。

- def max_succeeded():
-     best = 0
+ def foreign_runs():
+     runs_ = set()
      for ln in lines:
-         m = re.search(r"payments\[([^\]]*)\]", ln)
+         m = re.search(r"rows\[([^\]]*)\]", ln)
          if not m:
              continue
          for part in m.group(1).split(";"):
-             if part.startswith("succeeded|"):
-                 best = max(best, int(part.split("|")[1]))
-     return best
+             ext, _, _ = part.rpartition("|")
+             if ext.startswith("weknora-t02-") and not ext.startswith(run_prefix):
+                 runs_.add(ext.rsplit("-sub-", 1)[0])
+     return runs_
+ 
+ 
+ frn = foreign_runs()
+ if frn:
+     print(f"DB-WATCH: CHECK foreign runs in TSV: {sorted(frn)} "
+           "(payments peak not attributable to this run)")
+     sys.exit(1)


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:111-111 ───
[maintainability · low] samples 计数口径与标签不符：len(lines) 是 TSV 全部行数（含 decline 边界之后的尾段样本），却紧跟 "(mid-run
before …)" 打印，读者会把总数误读成 mid-run 样本数（项目报告引用时即需另行计算 mid-run 数，如 516 行/319
mid-run）。建议分开打印两个口径，避免证据引用时混淆。

- print(f"samples: {len(lines)} (mid-run before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")
+ midrun_n = sum(1 for ln in lines if midrun(ln))
+ print(f"samples: {len(lines)} total / {midrun_n} mid-run (before {boundary}Z)  "
+       f"max_succeeded_payments: {max_succeeded()}")


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:64-67 ───
[bug · low] max_succeeded() 与 rows_for() 的隔离级别不一致：rows_for() 通过 run_prefix 过滤并在注释中明确防御多 run TSV
串扰、且只统计 midrun 边界前的样本；而 max_succeeded() 遍历全部采样行，既不按本 run 前缀过滤，也不应用 midrun 边界。TSV 的 payments[...]
是全库聚合（无 run 键），一旦走 runs/ 回退路径（脚本明确支持的分支）取到混有外来 run 样本的 TSV，或共享 lab DB 未清卷（fix-log 自述"全局
payments/subscriptions 计数跨 run 累计"，干净 DB 是该断言的前提），外来 succeeded 计数会污染 `max_succeeded() == 1` 这一
exactly-once 不变量，产生误报 CHECK。当前提交的归档 TSV 仅含单一 run（f6bebb06）且 DB 已清卷，已归档的 DB-WATCH PASS
不受影响，问题仅存在于回退/污染场景——但两条断言的隔离语义应对称。注：同型问题在 OCR 第 4 轮已对 ocr1/ocr3 副本记录（4-07/4-12，未修复），本 ocr2 副本为相同代码。

  def max_succeeded():
      best = 0
      for ln in lines:
+         if not midrun(ln) or run_prefix not in ln:
+             continue
          m = re.search(r"payments\[([^\]]*)\]", ln)


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:111-111 ───
[maintainability · low] 摘要行打印的 `len(lines)` 是 TSV 全部行数，但放在 "samples ... (mid-run before …Z)"
语境下，读者会将其理解为 mid-run 样本数，而所有断言实际只消费 `midrun(ln)` 过滤后的子集。以本目录归档数据实测：TSV 共 197 个采样行，其中仅 176 行在边界
2026-09-23T08:58:42 之前（末条 mid-run 采样 08:58:40Z，尾段自 08:58:44Z 起）；归档输出 verify-db-watch-output.txt 记录
"samples: 197"，与断言基数不一致，审计者对账时易被误导（OCR 第 4 轮 4-09 对 ocr1 副本已记录同型问题，未修复）。建议同时打印总数与 mid-run
计数，使摘要与断言基数一致。

- print(f"samples: {len(lines)} (mid-run before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")
+ midrun_count = sum(1 for ln in lines if midrun(ln))
+ print(f"samples: {len(lines)} total / {midrun_count} mid-run (before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:66-66 ───
[test · medium] all(oa["checks"].values()) 在 checks 为空字典时返回 True(同模式还有 L39 的 phase_statuses、L88 的
retries checks、L102 的 decline checks),存在空洞通过风险。checks 结构确实在演化——本批 phases.py 刚把 retries 的
gate_still_pending 改名为 gate_not_activated 并增删键;一旦未来重构使某 phase 的 checks 清空或键减少,该断言会静默通过。同目录
verify_db_watch.py 已为 sub-a/sub-c 建立存在性守卫(len(sub_a) > 0),本脚本 L56 的默认值 99
也是同类守卫,此处应保持一致:bool(oa["checks"]) and all(...)。

- check("AC2 checks all true", all(oa["checks"].values()))
+ check("AC2 checks all true", bool(oa["checks"]) and all(oa["checks"].values()))


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:81-83 ───
[test · low] dr.get("invoice_count") == dr.get("invoice_count_baseline") 在两键同时缺失时 None == None
成立;当前仅靠第三个交叉比较条件兜底,若 dr 两键与 evidence.baseline.invoice_count 三处同时缺失,整条断言会空洞通过。作为验收断言应显式拒绝 None。

  check("AC2 duplicates invoice count == baseline",
-       dr.get("invoice_count") == dr.get("invoice_count_baseline")
+       dr.get("invoice_count") is not None
+       and dr.get("invoice_count") == dr.get("invoice_count_baseline")
        and d["evidence"]["baseline"].get("invoice_count") == dr.get("invoice_count"))


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:64-69 ───
[bug · medium] max_succeeded() 统计 succeeded 支付数时既未像 rows_for() 一样按 run_prefix 过滤,也不受 midrun()
边界约束,而回退分支(runs/ 下最新 TSV)正是脚本注释自己承认的多 run 残留场景。TSV 行内 payments[...] 不带 run 标识(只有 rows[...] 携带
weknora-t02-<run_id>- 前缀),此时 max_succeeded() == 1 这条"无双重扣款"关键断言会被外来 run 的样本污染:外来 run 出现 2 笔
succeeded 会误报 CHECK,本 run 采样缺失时外来计数又会顶替满足断言,削弱脚本声明的 run 隔离保证。建议仅计入 rows[...] 中含本 run_prefix
的采样行(顺序回放下该行的 payments 快照即本 run 视角),并套用 midrun 边界。

  def max_succeeded():
      best = 0
      for ln in lines:
+         if not midrun(ln):
+             continue
+         rows = re.search(r"rows\[([^\]]*)\]", ln)
+         if not rows or not any(
+             p.rpartition("|")[0].startswith(run_prefix)
+             for p in rows.group(1).split(";") if p
+         ):
+             continue  # sample carries no row of THIS run
          m = re.search(r"payments\[([^\]]*)\]", ln)
          if not m:
              continue


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:51-52 ───
[other · low] 脚本 docstring 声明退出码 0 PASS / 1 CHECK / 2 MISSING-EVIDENCE,但
decline["recorded_at"]、decline["run_id"] 等直接下标在证据结构漂移时会以未捕获 KeyError 崩溃并以退出码 1 结束,与
CHECK(断言失败)不可区分;verify_ac_assertions.py 的
d["evidence"]["baseline"]、r["evidence"]["responses"]["retry_payment"]、fin[...] 同理。建议对入口键用 .get
并在缺失时走 exit 2(当前 t02-decline.json 中两键均存在,属防御性改进)。

  decline = json.loads((EVID / "t02-decline.json").read_text())
- boundary = decline["recorded_at"][:19]
+ recorded_at, run_id = decline.get("recorded_at"), decline.get("run_id")
+ if not recorded_at or not run_id:
+     print("DB-WATCH: t02-decline.json missing recorded_at/run_id (evidence drift)")
+     sys.exit(2)
+ boundary = recorded_at[:19]


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:101-102 ───
[style · low] elif 分支仅在 s != 1 时可达(相等情况已被 if s == 1 捕获),条件中的 s != 1 恒真,属冗余判断;可简化为 elif
seen_active:,不影响功能。

-     elif seen_active and s != 1:
+     elif seen_active:
          sub_b_regress = True


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:111-111 ───
[documentation · low] 输出行把 TSV 总行数 len(lines) 打印为 "samples"，并紧挨 mid-run 边界限定语，但断言实际只使用边界前（且按
run_prefix 过滤）的行子集。以本目录归档数据实测：共 327 行、边界(2026-09-23T10:30:49)前仅 268 行，打印 "samples: 327 (mid-run
before …Z)" 会让归档的 verify-db-watch-output.txt 高估支撑 DB-WATCH PASS 结论的采样基数（尾段 cleanup 期
canceled/terminated 样本被计入）。建议统计 mid-run 行数或明确标注口径，例如：midrun_count = sum(1 for ln in lines if
midrun(ln)) 后打印。

- print(f"samples: {len(lines)} (mid-run before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")
+ midrun_lines = [ln for ln in lines if midrun(ln)]
+ print(f"samples: {len(lines)} total / {len(midrun_lines)} mid-run (before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")


─── internal/modules/commercial/commercialplatform/config.go:42-42 ───
[security · low] 以字符串拼接拆分环境变量名并注明目的是"defuse scanners"属于安全审计反模式:环境变量名不是凭据,拆分会削弱 secret
扫描与人工审计的可检索性,且已造成形状漂移——plan-81 文档(L373-375)与 t09-run.txt 均书写完整字面量,Go 源里 grep 完整名将 miss。若 GitHub push
protection 确实误报该 env 名,建议在常量旁注明具体依据并保持文档/脚本/代码三处同一形态;否则直接写字面量。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:476-478 ───
[bug · medium] providerAttachDefaultPaymentMethod 用 context.Background() 新建超时上下文,完全脱离了
createPurchaseSubscription 已设定的 purchaseRequestTimeout(25s)请求上下文:调用方取消/截止不传播,attach + default
更新可脱离总预算额外挂起最长约 15s,链路追踪信息也一并丢失。调用链上父 ctx 就在 providerCreateCustomer 的参数里,直接透传即可。

- func (a *LagoAdapter) providerAttachDefaultPaymentMethod(providerCustomerID, pmToken string) error {
- 	ctx, cancel := context.WithTimeout(context.Background(), outboundProviderTimeout)
+ func (a *LagoAdapter) providerAttachDefaultPaymentMethod(ctx context.Context, providerCustomerID, pmToken string) error {
+ 	ctx, cancel := context.WithTimeout(ctx, outboundProviderTimeout)
  	defer cancel()


─── internal/modules/commercial/commercialplatform/lago_purchase.go:462-462 ───
[security · medium] validateOutboundHost 只校验初始 URL,而这里每请求新建的 http.Client 未设置 CheckRedirect,默认会跟随最多
10 次 3xx 重定向——配置的 StripeAPIBase 或上游一个 302 即可把出站请求导向环回/私有地址,绕过 S1 的 host 校验形成 SSRF 残留(Go 会跨域剥离
Authorization,但内网探测仍可行)。仓库已有成熟先例(internal/utils/security.go 的
newSSRFCheckRedirect、appconnector/http_policy.go 逐跳重校验),建议复用同一模式;顺带把 client 提为包级单例以保留连接复用。

- 	resp, err := (&http.Client{Timeout: outboundProviderTimeout}).Do(req)
+ // 包级初始化一次:
+ // var outboundProviderClient = &http.Client{
+ // 	Timeout: outboundProviderTimeout,
+ // 	CheckRedirect: func(req *http.Request, _ []*http.Request) error {
+ // 		return validateOutboundHost(req.URL.String())
+ // 	},
+ // }
+ 	resp, err := outboundProviderClient.Do(req)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:236-240 ───
[bug · medium] 绑定写入 2xx 后无条件轮询 PM 导入最长 20s,但两种已文档化的姿态下导入永远不会发生:(a) 占位前缀模式已被 v1.53.0
实证走不通(docs/plans/issue-72-flow-evidence-81/README.md 第 22 条:default pm 只能从真 provider 拉取);(b)
生产姿态(key 已设、StripePmToken 留空等 #82 checkout)——config.go 注释明确承诺此时应到达 gated create 并以
no_default_payment_method 失败关闭,实现却在轮询处提前以 ErrPlatformUnreachable(可重试类)失败,每次购买固定烧满 20s。建议:未附加 default
PM(无 token/占位来源)时跳过轮询、直接让 gated create 按契约失败关闭;另 sleepCtx 的 ctx 取消被归类为
Unreachable("interrupted")也属错误类别错分。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:354-356 ───
[bug · low] 索引应答命中订阅但缺失 plan_amount_cents/plan_amount_currency(权威版本升级或字段名漂移)时不报任何错误:AmountFen 静默为
0、Currency 为空串。购买路径会退化为令人困惑的 ErrInvoiceQuoteMismatch(真实原因是权威契约漂移),PurchaseStatus 则把 AmountFen=0
原样透出给前端。held 订阅缺失冻结价格面应 fail closed(invalid_response),与既有"非数字 → invalid_response"的口径对齐。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:189-192 ───
[maintainability · low] 422 分支把多种成因(external_id 重复、subscription_incomplete、PM 同步竞态的
no_default_payment_method、plan 校验失败)合并为一个标签:身份重读未命中时统一报 "purchase replay cannot be verified"。例如 PM
竞态 422 重读必然未命中,却被标成"回放无法验证",误导排障与上层对持久性/可重试错误的分类。同函数内对 payment_provider_not_found 已有按 error_details
细分的先例,建议同样识别 no_default_payment_method 并给出中性/准确的错误描述。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:223-225 ───
[maintainability · low] payment_provider_code 硬编码 "weknora-stripe":部署方必须以完全相同的 code 注册 provider(t09
实验即用此约定),而同组的 D3 旋钮(key/base/prefix/pm token)全部走 env。注册 code 不一致时绑定 422 落入
payment_provider_not_found 分支,报 "no provider registered"——与真实成因(provider 已注册、code 不同)不符。建议将该 code
也纳入 env(默认 weknora-stripe),或在 config.go 的 env 文档旁显式声明这一部署契约。



─── packages/api-client/src/commercial.ts:66-66 ───
[maintainability · low] purchase 的请求体类型以内联字面量定义在
api-client，打破了既有约定：其余线上请求输入（QuoteInput、CreateOrderInput、RefundInput）都由 contracts 包集中声明并导出，contracts
是 wire 契约的单一事实来源。内联导致 'wechat' | 'alipay' 联合类型与 contracts 中 CreateOrderInput.provider 重复，后续扩展
provider 或字段时需改两处。建议在 contracts 增加 export interface PurchaseInput { quote_id: string; provider:
'wechat' | 'alipay' } 并复用。

-     async purchase(input: { quote_id: string; provider: 'wechat' | 'alipay' }, signal?: AbortSignal): Promise<PurchaseView> {
+     async purchase(input: PurchaseInput, signal?: AbortSignal): Promise<PurchaseView> {   // PurchaseInput 从 '@weknora/contracts' 导入并集中定义


─── packages/contracts/src/commercial.ts:99-102 ───
[maintainability · low] currency 的校验结果被丢弃：nonEmptyString 仅用于抛错，随后用 v.currency as string
断言赋值，校验与赋值脱节，与同函数内其他字段的解析模式不一致，后续维护易引入绕过校验的路径。nonEmptyString 本身返回修剪前原值，直接使用其返回值赋值即可。

   if(v.currency!==undefined) {
-   nonEmptyString(v.currency,'currency','quote');
-   out.currency = v.currency as string;
+   out.currency = nonEmptyString(v.currency,'currency','quote');
   }


─── packages/contracts/src/commercial.ts:180-182 ───
[maintainability · low] amount_fen 对 null/'' 静默跳过而非报错，与同函数内其他字段的严格校验（非法即 throw）不一致：后端 purchaseWire 以
JSON 省略表示缺失（AmountFen==0 时不输出），null/'' 属于畸形载荷，静默吞掉会让下游拿到缺失金额的 PurchaseView 且难以定位。建议与 sibling
字段保持一致：仅以 undefined 判定缺失，其余交给 digitString 抛错。

-   if(v.amount_fen!==undefined&&v.amount_fen!==null&&v.amount_fen!=='') {
+   if(v.amount_fen!==undefined) {
      out.amount_fen = digitString(v.amount_fen,'amount_fen','purchase');
    }


─── internal/modules/commercial/service/commercial/purchase.go:172-179 ───
[bug · high] 匹配门(第7步)只拒绝 nil 与 PurchaseStateAbsent,未要求状态必须是 awaiting_payment:当权威快照为
active(此前已购买并支付成功后用新报价再次提交)或 canceled(支付超时/失败后重试)时,plan code/币种/金额全部相等,门禁通过并继续 CreateOrder
发起渠道支付请求。结合适配器行为(lago_purchase.go 的 createPurchaseSubscription 对同 plan code
的已有订阅无论状态一律回放成功、绝不重建),这构成两条真实可达的坏路径:1) active 重购 → 为已激活订阅再次收款(重复扣款,无新增权益);2) canceled 重试 → 收款后订阅仍是
canceled,#82/#83 的激活永远无法完成。另外第6步的冲突探针要求 held.Purchase.State == AwaitingPayment,active 且不同 plan code
时本应答 ErrPurchasePlanConflict 的确定性冲突会被误答为瞬时 invalid_response(pendingView)。建议:门禁要求 p.State ==
PurchaseStateAwaitingPayment;active 时直接返回激活视图(或专用闭合冲突),canned[应为 canceled]时返回明确的重购冲突并由 seam
负责重建;同时放宽第6步探针的状态条件。

  	p := psnap.Purchase
- 	if p == nil || p.State == domain.PurchaseStateAbsent ||
+ 	if p == nil || p.State != domain.PurchaseStateAwaitingPayment ||
  		p.PlanCode != pub.PlanCode ||
  		p.Currency != domain.CurrencyCNY ||
  		p.Currency != snap.Currency ||
  		p.AmountFen != snap.PriceFen {
+ 		if p != nil && p.State == domain.PurchaseStateActive &&
+ 			p.PlanCode == pub.PlanCode && p.AmountFen == snap.PriceFen {
+ 			// 同计划已激活:直接回答激活视图,绝不再创建渠道支付请求
+ 			return s.purchaseView(p, snap, pub, nil), nil
+ 		}
  		return PurchaseView{}, ErrInvoiceQuoteMismatch
  	}


─── internal/modules/commercial/service/commercial/purchase.go:232-235 ───
[bug · medium] PurchaseStatus 用 ListOrdersByTenant 结果的 rows[0] 充当"该购买的订单":该查询不过滤 kind(purchase 与
upgrade 混排),且 ORDER BY id DESC 作用于随机 token ord_*(newLeaseToken),并非真实时间序——rows[0]
实际是租户任意一张订单。租户存在换购/升级或历史失败订单时,会把无关订单的渠道单号、金额投影为当前购买状态,前端展示错误的待付款信息。建议至少过滤 kind ==
domain.OrderKindPurchase 并与快照金额/币种比对;更稳妥的做法是持久化订单与购买订阅(或报价)的显式关联后按关联读取,找不到就不附带 Order。

- 	if rows, err := s.orders.orders.ListOrdersByTenant(ctx, tenantID); err == nil && len(rows) > 0 {
- 		ov := orderViewFromRow(rows[0])
+ 	if rows, err := s.orders.orders.ListOrdersByTenant(ctx, tenantID); err == nil {
+ 		for _, r := range rows {
+ 			if r.Kind == domain.OrderKindPurchase && r.AmountFen == p.AmountFen && r.Currency == p.Currency {
+ 				ov := orderViewFromRow(r)
- 		out.Order = &ov
+ 				out.Order = &ov
+ 				break
+ 			}
+ 		}
  	}


─── internal/handler/commercial.go:438-438 ───
[bug · medium] Purchase 把 actor 硬编码为 "billing-admin":该值会作为 Command.Actor 传入
create_purchase_subscription 以及 EnsureBillingAccount 的 ensure_customer
命令(权威侧审计轨迹),所有购买与懒建账户操作都归因到这个合成操作者,真实操作者信息丢失。同文件既有约定(AccountStatus)以 commercialUserID(c) 作为 actor 传入
EnsureBillingAccount,建议保持一致(commercialTenantScope 的第二个返回值是 role,不是身份)。

- 	view, err := h.purchases.Purchase(c.Request.Context(), tenantID, req.QuoteID, req.Provider, "billing-admin")
+ 	actor := commercialUserID(c)
+ 	if actor == "" {
+ 		actor = "billing-admin"
+ 	}
+ 	view, err := h.purchases.Purchase(c.Request.Context(), tenantID, req.QuoteID, req.Provider, actor)


─── internal/modules/commercial/service/commercial/purchase.go:129-131 ───
[bug · medium] platformReason(errors.New(acct.Reason)) 把已经是闭合 token 的 reason 字符串重新包装成普通 error 再走
platformReason 的 errors.Is 分支——任何非空字符串都命中默认分支,恒返回 "unsupported"。例如 Lago 未配置时账户 pending 的 reason 本是
"unconfigured",购买应答却变成 "unsupported",客户端按 reason
分流会得到错误的姿态(配置缺失被报成能力不支持)。BillingAccountStatus.Reason 本就出自同一闭合映射,直接透传即可。

  	if acct.State != BillingAccountLinked {
- 		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: platformReason(errors.New(acct.Reason))}, nil
+ 		return PurchaseView{State: domain.PurchaseStateAbsent, Reason: acct.Reason}, nil
  	}


─── internal/modules/commercial/service/commercial/purchase.go:184-189 ───
[bug · medium] 第 8 步幂等重放无法兑现 "returns verbatim"：orderViewFromRow 只映射 OrderRow
的持久化字段（ID/QuoteID/State/AmountFen/Currency/Version），而 checkout_url 只在首次 openOrder 的渠道应答中出现（alipay.go
返回 QRCode、wechat.go 返回 CodeURL），OrderRow 与 PaymentAttemptRow 均未持久化 CheckoutURL。客户端网络超时/刷新后重放 POST
/purchases 时，本分支返回同一订单但没有 checkout_url，且 GET /commercial/orders/:id（RecoverOrderStatus 的最终投影同样不带
CheckoutURL）也无法再补给——报价已被消费、门控订阅停留 awaiting_payment，用户从此拿不到支付入口，只能弃单。CheckoutPage 的「前往支付」链接正依赖
order.checkout_url。建议在 openOrder 时把渠道返回的 checkout_url 持久化（commercial_orders 或
commercial_payment_attempts 增列），并让 orderViewFromRow / RecoverOrderStatus 的投影携带它，重放应答才真正做到 verbatim。



─── internal/handler/commercial.go:438-441 ───
[maintainability · low] 渠道调用失败但订单已持久 pending 的姿态（openOrder 返回 err==nil 且 CheckoutError 置位）在 POST
/commercial/orders 中以 202 Accepted 显式区分（"the client recovers through GET /commercial/orders/:id"），而
purchase 端点把同一底层事件折叠进 201 Created，两个结算面的 HTTP 语义不一致：201 断言渠道支付请求已创建，实际并未。当前 web 解析器只区分 2xx
故无直接故障，但契约读者会误读。建议与 orders 端点对齐补 202 分支。

  	view, err := h.purchases.Purchase(c.Request.Context(), tenantID, req.QuoteID, req.Provider, "billing-admin")
  	switch {
+ 	case err == nil && view.Order != nil && view.Order.CheckoutError != "":
+ 		c.JSON(http.StatusAccepted, gin.H{"success": true, "data": purchaseWire(view)})
  	case err == nil:
  		c.JSON(http.StatusCreated, gin.H{"success": true, "data": purchaseWire(view)})


─── internal/modules/commercial/service/commercial/purchase.go:125-125 ───
[maintainability · low] 懒建账户的 displayName 硬编码为字面量 "space"：同文件 AccountStatus/EnsureBenefits 的既有约定是传
h.tenantDisplayName(tenantID)（handler commercial.go:533/536）。用户首次经由结算页触发懒建（此前从未访问过账户页）时，权威侧 Lago
customer 的显示名将是 "space" 而非空间名，直接出现在 Lago 管理界面。DisplayName 虽为 advisory 元数据，仍建议将真实 display name 随
actor 一并由 handler 传入（为 Purchase 增加 displayName 参数），与 actor 归因问题一并在接缝处修正。



LLM retry report summary: 4 of 71 requests affected -- 4 requests recovered after retry

Review planning (1 request):
- internal/container/container.go,internal/handler/commercial.go,internal/modules/commercial/purchase_command.go,internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go,internal/router/routes_commercial.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Core review (3 requests):
- docs/plans/issue-72-flow-evidence-74/t02-activation.json,docs/plans/issue-72-flow-evidence-74/t02-cleanup.json,docs/plans/issue-72-flow-evidence-74/t02-decline.json,docs/plans/issue-72-flow-evidence-74/t02-duplicates.json,docs/plans/issue-72-flow-evidence-74/t02-environment.json,docs/plans/issue-72-flow-evidence-74/t02-gating.json,docs/plans/issue-72-flow-evidence-74/t02-health.json,docs/plans/issue-72-flow-evidence-74/t02-manual.json,docs/plans/issue-72-flow-evidence-74/t02-provider.json,docs/plans/issue-72-flow-evidence-74/t02-retries.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/container/container.go,internal/handler/commercial.go,internal/modules/commercial/purchase_command.go,internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go,internal/router/routes_commercial.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- internal/container/container.go,internal/handler/commercial.go,internal/modules/commercial/purchase_command.go,internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go,internal/router/routes_commercial.go: rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
