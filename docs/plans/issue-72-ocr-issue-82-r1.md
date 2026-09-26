Review partially complete: 119 finding(s); 44 of 95 selected item(s) failed.

─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:36-36 ───
[security · high] 登录密码被硬编码为字面量 'issue82-Flow-Pw-d'，而非使用上方已校验的 PASSWORD 环境变量（FLOW82_PASSWORD_D /
FLOW82_PASSWORD）。这违反了本任务的安全验收条件（凭据只从环境变量读取，源码/示例/测试不得写入可用凭据字面量），也与文件头注释 "credentials env-REQUIRED"
自相矛盾——环境变量校验形同虚设，且 PASSWORD 变量声明后从未使用（死代码）。同目录的 browser_flow_82.mjs、r4-flow/browser_03_paid_face.mjs
等均使用 page.fill('#auth-password', PASSWORD)。应改为填充 PASSWORD，若 env 与该字面量不一致，当前实现还会导致登录意外失败。

-   await page.fill('#auth-password', 'issue82-Flow-Pw-d');
+   await page.fill('#auth-password', PASSWORD);


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:16-16 ───
[maintainability · low] import 语句位于 fileURLToPath 首次调用（第 12 行 const EV = fileURLToPath(...)）之后。虽然
ESM 静态 import 会被提升、运行时不出错，但先使用后导入的写法具有误导性；同目录其他脚本（如
browser_flow_82.mjs、r4-flow/browser_03_paid_face.mjs）均将 import { fileURLToPath } from 'node:url';
置于文件顶部与其它 import 一起，建议保持一致。

+ // 移至文件顶部，与 import { createRequire } from 'node:module'; 并列：
+ import { createRequire } from 'node:module';
  import { fileURLToPath } from 'node:url';


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_02_sync_face.mjs:45-47 ───
[test · medium] 刷新按钮的点击通过 `.catch(() => {})` 静默吞掉所有异常，且失败后脚本仍会以 detail "after revisit/refresh
x3/reload/poll" 记录 PASS。该脚本的核心证据主张是"手动刷新被真实重放且不推进订单"——若按钮因文案调整、条件渲染变化或定位失败导致三次点击一次都没落地（已确认按钮文案当前在
CheckoutPage.tsx:348 匹配，但复制到 r4-flow2/r4-flow3 的三份脚本不会同步感知前端改动），PASS 结论就失去意义。用 evaluate 规避
actionability 门槛本身没问题（注释已说明原因），但至少应记录点击是否落地，例如监听刷新请求或把 catch 结果记入 results。

-     await page.locator('button:has-text("刷新订单状态")').first()
-       .evaluate((b) => b.click()).catch(() => {});
+   for (let i = 0; i < 3; i++) {
+     const clicked = await page.locator('button:has-text("刷新订单状态")').first()
+       .evaluate((b) => b.click()).then(() => true, () => false);
+     if (!clicked) note(`refresh-click-${i + 1}`, false, '刷新订单状态 button not found — manual refresh NOT replayed');
      await page.waitForTimeout(1200);
+   }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_02_sync_face.mjs:53-59 ───
[test · medium] 断言基于整个 main 的 innerText 做全局子串匹配：`!body.includes('已付款')` / `!body.includes('已生效')`
是整页否定断言。已确认前端这两个词目前仅由 order-state.ts / purchaseStateMessage 在 paid/active 状态渲染，但 main
还包含报价明细、渠道异常提示等所有区块，未来任何新增静态文案（如帮助说明"支付成功后此处将显示已付款"）都会导致误 FAIL，且失败时无从定位来源；正向锚点 `待付款（权益未开通）`
同样与前端文案强耦合。状态文案实际渲染在 `section[aria-live="polite"]` 内（CheckoutPage.tsx:295-300），把断言范围缩小到该 section
可显著降低与无关区块的耦合。

-   const body = await page.locator('main').innerText();
-   note('sync-return-still-awaiting', body.includes('待付款（权益未开通）'),
+   const statusText = await page.locator('section[aria-live="polite"]').innerText();
+   note('sync-return-still-awaiting', statusText.includes('待付款（权益未开通）'),
      'after revisit/refresh x3/reload/poll: still 待付款（权益未开通）');
-   note('sync-return-no-paid-claim', !body.includes('已付款'),
-     'no 已付款 claim anywhere on the sync-return face');
-   note('sync-return-no-active-claim', !body.includes('已生效'),
-     'no 已生效 claim anywhere on the sync-return face');
+   note('sync-return-no-paid-claim', !statusText.includes('已付款'),
+     'no 已付款 claim in the order status section');
+   note('sync-return-no-active-claim', !statusText.includes('已生效'),
+     'no 已生效 claim in the order status section');


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_02_sync_face.mjs:11-11 ───
[maintainability · low] FLOW82_WEB 未设置时静默回退到硬编码的 `http://localhost:5194`，与下面对 FLOW82_EMAIL_A /
FLOW82_PASSWORD_A
缺失即报错退出的处理不一致：目标环境指错时脚本会一直打到本地固定端口，失败原因（连接拒绝/超时堆栈）不会指向真实问题。作为留证脚本，建议与账号环境变量一致——未显式设置即报错退出，避免在错误环境上跑出
误导性结果。

- const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5194';
+ const WEB = process.env.FLOW82_WEB ?? '';
+ if (!WEB) { console.error('missing required env: FLOW82_WEB'); process.exit(2); }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_02_sync_face.mjs:49-51 ───
[maintainability · low] 固定时序 waitForTimeout(3500) 依赖注释中"轮询周期 3s"的假设，前端一旦调整轮询间隔，"let one poll tick
land" 会静默落空，脚本 PASS 但实际未覆盖 poll 路径。另外该脚本与
r4-flow2/browser_02_sync_face.mjs、r4-flow3/browser_02_sync_face.mjs
是近似复制的三份（已比对：当前断言文案、刷新次数、等待时长一致，无实质漂移，但 r4-flow3 已丢失解释性注释），后续任何参数调整需人工同步三份，建议至少把等待时长提取为带说明的常量并注明与
CheckoutPage 轮询间隔的关联。

+   // CheckoutPage poll interval is 3s — wait one full tick plus margin.
+   const POLL_INTERVAL_MS = 3000;
    await page.reload();
    await page.waitForSelector('text=待付款（权益未开通）', { timeout: 45000 });
-   await page.waitForTimeout(3500); // let one poll tick land
+   await page.waitForTimeout(POLL_INTERVAL_MS + 500);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_02_sync_face.mjs:61-63 ───
[test · low] 登录失败或 waitForSelector 超时（如订单已被推进、页面异常）时异常直接冒泡，仅有 finally 关浏览器：无 RESULT
结构化输出、无失败截图，验收时只能从堆栈反推失败步骤，且与断言失败（有 FAIL 行 + 截图）的留证口径不一致。作为留证脚本，建议补 catch 分支记录失败 step 并落一张失败现场截图。

+ } catch (err) {
+   note('script-error', false, String(err));
+   await page?.screenshot({ path: `${EV}03-sync-return-error.png`, fullPage: true }).catch(() => {});
  } finally {
    await browser.close();
  }


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:55-59 ───
[test · high] billing 页断言存在竞态，可能空过（vacuous PASS）：BillingPage 的套餐行 <li> 在 summary 成功后即渲染，而
`已付款待激活`/`已生效` 状态后缀来自另一个异步 purchaseStatus() 请求（BillingPage.tsx:83-87），渲染晚于
<li>。`waitForSelector('main li, main table')` 只是结构选择器，并不保证购买状态文案已加载（注释所说的 "waiting for the plan row
copy" 与实际不符），随后立即断言 `!billing.includes('已生效')` 在窗口期内会无条件通过——而这正是本脚本要取证的核心验收点，假 PASS 会被固化为证据截图与
RESULT 行。建议与同目录 r4-flow/browser_03_paid_face.mjs:49-53 一致：先正向等待 `text=已付款待激活`（purchaseStatus
已渲染的信号）再做否定断言，并同时断言正向状态。

-   // (OCR r4) waitForTimeout replaced by waiting for the plan row copy.
-   await page.waitForSelector('main li, main table', { timeout: 30000 });
+   // (OCR r4) waitForTimeout replaced by waiting for the plan-row status copy
+   // (purchaseStatus render) before the negative assertion.
+   await page.waitForSelector('text=已付款待激活', { timeout: 30000 });
    const billing = await page.locator('main').innerText();
-   note('billing-no-false-effective', !billing.includes('已生效'),
-     'billing plan row does not claim 已生效');
+   note('billing-paid-awaiting-activation', billing.includes('已付款待激活') && !billing.includes('已生效'),
+     'billing plan row shows 已付款待激活 and does not claim 已生效');


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:75-77 ───
[maintainability · medium] 死代码：全库检索确认 `window.__r4PurchasesSeen` 仅在此处出现，前端代码（含本次同时修改的
CheckoutPage.tsx/BillingPage.tsx）从未对该全局赋值，因此该 waitForFunction 每次运行必然超时并被 `.catch(() => {})`
静默吞掉，造成固定约 1 秒的空等，且会误导维护者以为 provider 断言依赖该全局。实际断言已由 `page.on('request')` 捕获的 purchases
数组支撑（awaiting-payment 面渲染时请求体必然已捕获），建议直接删除这一行。

    // Wire assertion: the purchase submit body carried provider == alipay.
-   await page.waitForFunction(() => window.__r4PurchasesSeen > 0, null, { timeout: 1000 }).catch(() => {});
    const providerOk = purchases.length >= 1 && purchases.every((p) => p.provider === 'alipay');


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:90-91 ───
[test · low] order-info.json 写入时机过晚：writeFileSync 放在 billing 页断言之后，若其间任一步抛异常（如 billing
waitForSelector 超时、goto 失败），finally 仅关闭浏览器、异常向上抛出，已成功捕获的 orderId/checkoutHref/purchases
将全部丢失——而该文件正是后续段脚本（02/03/04 以 orderId 为 argv）与审计链路的输入。建议在捕获完成后立即落盘（或移入 finally），并对 orderId
空串、checkoutHref 为 null 做显式校验（getAttribute 可返回 null，目前会原样序列化）。另外日志拼接 `p.quote_id?.slice(0, 12) + '…'`
在 quote_id 缺失时会输出 "undefined…"。

+   if (!/^ord_[0-9a-f]+$/.test(orderId) || !checkoutHref) {
+     note('order-info-complete', false, `orderId=${orderId} checkoutHref=${checkoutHref}`);
+   }
    writeFileSync(`${EV}order-info.json`, JSON.stringify({ orderId, checkoutHref, purchases }, null, 2));
    console.log('ORDER ' + orderId);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:58-59 ───
[test · low] 该步骤的 ok 值硬编码为 true，证据记录中这一项的 PASS 并非来自任何实际校验（仅隐式依赖上一行 page.check
未抛错），削弱了证据文件的判定语义。建议回读勾选状态作为判定依据。

    await page.check('input[name="payment-channel"][value="alipay"]');
-   note('channel-explicit-alipay-select', true, 'explicitly selected the 支付宝 radio');
+   note('channel-explicit-alipay-select', await page.isChecked('input[name="payment-channel"][value="alipay"]'),
+     'explicitly selected the 支付宝 radio');


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:95-96 ───
[bug · low] `process.exit()` 紧跟 `console.log('ORDER …')`/`RESULT` 输出：当 stdout 为管道（CI
采集证据日志的常见场景）时，Node 不保证退出前刷新未完成的异步写，末尾的 ORDER/RESULT 行可能被截断，导致 CI 中证据不完整。建议改用 `process.exitCode`
赋值，让事件循环在输出刷新后自然退出（browser 已在 finally 中关闭，无其他挂起资源）。

  console.log('RESULT ' + JSON.stringify(results));
- process.exit(results.every((r) => r.ok) ? 0 : 1);
+ process.exitCode = results.every((r) => r.ok) ? 0 : 1;


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:52-53 ───
[test · low] note('channel-explicit-alipay-select', true, ...) 将 ok 硬编码为 true，results 取证记录中该步骤恒为
PASS，不反映「显式选择后 radio 确为选中」的真实校验结果；一旦 page.check 行为变化或被重构，该步骤仍会报 PASS，削弱 results
数组作为取证结论的可靠性（r4-flow/r4-flow3 同族脚本存在同样问题）。建议在 check 后回读选中状态作为断言值。

    await page.check('input[name="payment-channel"][value="alipay"]');
-   note('channel-explicit-alipay-select', true, 'explicitly selected the 支付宝 radio');
+   const alipayExplicitlyChecked = await page.isChecked('input[name="payment-channel"][value="alipay"]');
+   note('channel-explicit-alipay-select', alipayExplicitlyChecked, 'explicitly selected the 支付宝 radio');


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:1-3 ───
[maintainability · low] 本脚本与 ../r4-flow/browser_01_checkout.mjs、../r4-flow3/browser_01_checkout.mjs
构成三份约 90% 相同的近似副本，且断言漂移已经发生：round 1 版本的 purchase-wire-provider-alipay detail 额外输出
quote_id，并残留一段无效果的调试死代码 waitForFunction(() => window.__r4PurchasesSeen > 0, ..., { timeout: 1000
}).catch(() => {})。取证目录按轮次快照可以理解，但后续轮次若继续复制，断言演进时各轮结论会不一致且难以比对。建议后续新增轮次时将共享断言（登录、radio、待付款形态、wire
provider、billing 行）提取为公共模块，或至少在每轮头注释中以断言清单形式显式同步与上一轮的差异。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:55-55 ───
[maintainability · low] 脚本在 checkout 页等待「待付款（权益未开通）」、在 billing
页等待「待付款（权益未开放）」，两处文案仅一字之差（未开通/未开放），已核实与 apps/web/src/commercial/CheckoutPage.tsx 与 BillingPage.tsx
当前文案一致，但这两个近似文案分别硬编码在两处 waitForSelector：任一前端文案微调都会让脚本以 selector
超时的形式失败，且从超时报错很难定位到是哪一处文案变了。建议在两处等待旁注明文案来源组件（CheckoutPage 状态徽标 / BillingPage 套餐行），或推动前端改用稳定的
data-testid 以消除对一字之差文案的依赖。



─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:78-79 ───
[bug · low] detail 字符串拼接缺陷：当某个捕获的请求体缺少 quote_id（例如 push 进 purchases 的是 `{ unparseable: true
}`，或重试请求体不含 quote_id）时，`p.quote_id?.slice(0, 12)` 经可选链返回 undefined，随后 `+ '…'` 会拼接出字符串
`"undefined…"`。这恰是 wire 断言失败、最需要人工看证据排查的场景，日志却输出误导性的 undefined。建议改为 `p.quote_id ?
p.quote_id.slice(0, 12) + '…' : '(no quote_id)'` 之类对缺失值显式标注的写法。

    note('purchase-wire-provider-alipay', providerOk,
-     `POST /purchases bodies: ${JSON.stringify(purchases.map((p) => ({ provider: p.provider, quote_id: p.quote_id?.slice(0, 12) + '…' })))}`);
+     `POST /purchases bodies: ${JSON.stringify(purchases.map((p) => ({ provider: p.provider, quote_id: p.quote_id ? p.quote_id.slice(0, 12) + '…' : '(no quote_id)' })))}`);


─── deploy/lago-lab/payment-settle-trigger/phases.py:54-59 ───
[maintainability · medium] phases.py 自身的 sys.path.insert(0, _PA_DIR) 与注释意图相反，存在模块绑定反转风险。当前正确性完全依赖
run_lab.py 恰好先 sys.path.append(_PA_DIR)（使此处的 not in sys.path 短路）且 LAB_DIR 已在
sys.path[0]——这是隐式跨文件顺序耦合。一旦 phases.py 被任何其他方式导入（REPL、pytest、未来工具），此处的 insert(0) 会把
payment-activation 目录推到搜索路径最前，`import fixtures` 将绑定到 pa 的 fixtures.py（已确认存在同名文件，且其缺少
sign_stripe_event/unsettled_intents/gated_subscription_payload/build_pi_succeeded_event 等本地
API），后续调用全部 AttributeError 或静默使用错误实现。同目录 fixtures.py 第 34-43 行已示范正确做法（importlib 以 pa_fixtures
别名加载），建议 phases.py 对 clients 采用相同模式，消除对 sys.path 顺序的依赖。

+ import importlib.util
+ 
  _PA_DIR = Path(__file__).resolve().parent.parent / "payment-activation"
- if str(_PA_DIR) not in sys.path:
-     sys.path.insert(0, str(_PA_DIR))
+ # 以别名加载 pa 的 clients（同本目录 fixtures.py 的做法），不触碰 sys.path，
+ # 保证 bare `import fixtures` 恒解析到本目录模块。
+ _spec = importlib.util.spec_from_file_location("pa_clients", _PA_DIR / "clients.py")
+ clients = importlib.util.module_from_spec(_spec)
+ _spec.loader.exec_module(clients)
  
- import clients  # noqa: E402  (payment-activation clients, read-only)
- import fixtures  # noqa: E402  (THIS directory's payloads shadow pa's fixtures)
+ import fixtures  # noqa: E402  (本目录模块，靠脚本目录在 sys.path[0] 解析)


─── deploy/lago-lab/payment-settle-trigger/phases.py:240-243 ───
[bug · medium] lago_db_query 的 subprocess.run(timeout=30) 未捕获异常，与同文件 _rails_runner/_compose_exec_env
的防护（except (OSError, subprocess.SubprocessError)）不一致。subprocess.TimeoutExpired 继承 SubprocessError 而非
OSError，psql 卡死超时会逃出 phase_settle_trigger 的 except OSError；虽然 run_lab.run_one 的兜底 except Exception
会把它记为 unexpected_error 的 fail（进程不崩、cleanup 仍执行），但该阶段既无法按模块契约归为 blocked-env（db 卡死属环境缺口），也丢失最后观测态与
evidence。organization_webhook_scope 与 inbound_webhooks
前后计数均依赖此路径。顺带说明：StripeTestClient.create_customer/attach_payment_method 失败抛出的 LabError(RuntimeError)
同样逃逸 except OSError，分类面不完整。

+         try:
-         result = subprocess.run(command, capture_output=True, text=True,
+             result = subprocess.run(command, capture_output=True, text=True,
-                                 timeout=30)
+                                     timeout=30)
+         except (OSError, subprocess.SubprocessError):
+             return None
          output = (result.stdout or "").strip().splitlines()
          return output[0].strip() if output else None


─── deploy/lago-lab/payment-settle-trigger/phases.py:334-336 ───
[security · low] _compose_exec_env 将新铸造的 webhook secret 以 `-e WSECRET=<secret>` 命令行参数传给 docker
compose exec：docker CLI 进程存活期间，宿主所有用户可经 ps / /proc/<pid>/cmdline 读到该值，与 provider_webhook_secret 注释中
"Secrets ... stay in memory only -- never reported raw" 的卫生意图相悖。该 secret 为测试模式、仅本地 lab stack
有效，影响有限，但既然 harness 已用 stdin 之外的通道，建议改经 stdin 传入（rails runner 代码读 $stdin.read），使密钥全程不落入 argv。

-         for key, value in env_vars.items():
-             command += ["-e", f"{key}={value}"]
+         # secret 经 stdin 传入，避免出现在 /proc/<pid>/cmdline
+         # ruby 侧: secret = $stdin.read; ...update!(webhook_secret: secret)
+         secret_stdin = "\n".join(str(v) for v in env_vars.values())
          command += ["api", "bin/rails", "runner", code]
+         result = subprocess.run(command, capture_output=True, text=True,
+                                 input=secret_stdin, timeout=180)


─── deploy/lago-lab/payment-settle-trigger/phases.py:704-707 ───
[bug · low] P-D 的 inbound_webhooks 增量断言存在边界空洞：当 before 读取失败返回 None 而 after 成功时，`inbound_before is
not None and ...` 短路使"新增一行"校验被无条件放过，探针在证据不完整时仍判 pass（evidence t11-settle-trigger-pbce.json 证实正常形态为
{"before": "1", "after": "2"}，非 None 路径校验有效）。反向地 after 为 None（DB 读取失败）会被判为 FAIL("row did not land")
而非 blocked-env——读取失败与行未落地是不同语义。另外 int() 对非数字 stdout（如 compose 告警混入）会抛未防护的 ValueError。建议：任一侧读不到即记
note 并归 blocked-env；int 转换包 try。

-         if inbound_after is None or (inbound_before is not None
-                                      and int(inbound_after) < int(inbound_before) + 1):
+         if inbound_before is None or inbound_after is None:
+             ctx.note("inbound_webhooks count unreadable via psql (before=%r after=%r)"
+                      % (inbound_before, inbound_after))
+             return _blocked(ctx, "settle_trigger",
+                             "inbound_webhooks count not readable from the lab DB",
+                             expected)
+         try:
+             landed = int(inbound_after) >= int(inbound_before) + 1
+         except ValueError:
+             landed = False
+         if not landed:
              observed["error"] = "inbound_webhooks row did not land"
              return _report(ctx, "settle_trigger", expected, observed, FAIL)


─── deploy/lago-lab/payment-settle-trigger/phases.py:805-805 ───
[bug · low] cleanup 漏删本运行创建的 Lago payment provider：phase_gated_3ds 经 GraphQL
addStripePaymentProvider 创建了 provider（{prefix}-stripe），phase_cleanup 未尝试删除它（evidence
t11-cleanup.json 的 objects 仅 subscription/customer/stripe_customer/plan/stripe_webhook_endpoint
五项），导致 expected 所述 "every lab object created this run is deleted" 与实际不符却报 pass——每次运行在 lab stack 累积一个
StripeProvider（含 webhook_secret）。建议在 cleanup 中尝试删除 provider（或至少将留存对象如实记入 observed/notes，修正 expected
措辞），避免探针报告对自身清理完整性做出不实断言。

+     provider_code = state.get("provider_code")
+     if provider_code:
+         try:
+             gstatus, gbody = ctx.graphql(fixtures.DESTROY_PROVIDER_QUERY,
+                                          {"code": provider_code})
+             record("payment_provider", provider_code,
+                    "deleted" if _ok(gstatus) else "failed", gstatus)
+         except OSError:
+             record("payment_provider", provider_code, "failed", None)
+ 
      failures = [item for item in objects if item["outcome"] != "deleted"]


─── deploy/lago-lab/payment-settle-trigger/phases.py:128-130 ───
[maintainability · low] RunContext.feature_code 属性声明后全文件（含
run_lab.py、fixtures.py）无任何引用，属死代码——plan_code 有实际用途而 feature_code 从未参与建单。建议移除，或在需要 feature
的探针阶段真正接线使用。

-     @property
-     def feature_code(self):
-         return f"{self.prefix}-feature"
+ # 删除该 property；若后续探针需要 feature，再随用例一并引入。


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_02_sync_face.mjs:39-43 ───
[test · medium] `.catch(() => {})` 会把「刷新 x3」这一核心前置动作的失败静默吞掉：若前端按钮文案/结构变化（当前文案在 CheckoutPage.tsx:348
为「刷新订单状态」），locator 解析将走 45s 默认超时后被 catch 吞掉，实际一次刷新都未执行，而后续「仍是待付款/无已付款声明」断言会空转通过并以 exit 0
收尾——这正是该脚本要防的假阳性证据。另外 round 1（r4-flow/browser_02_sync_face.mjs L41-43）有注释解释为何用 DOM 级 click（3s
轮询重渲染导致 Playwright actionability gate 无法稳定命中），本副本把注释删了，建议恢复并在点击失败时记入 results。

+   // 同步回放 x3：页面每 3s 轮询重渲染，Playwright actionability gate 难以稳定命中，
+   // 故用 DOM 级 click；但点击失败必须记入结果，不能静默吞掉（否则产生假阳性证据）。
    for (let i = 0; i < 3; i++) {
-     await page.locator('button:has-text("刷新订单状态")').first()
-       .evaluate((b) => b.click()).catch(() => {});
+     const btn = page.locator('button:has-text("刷新订单状态")').first();
+     if ((await btn.count()) === 0) {
+       note('refresh-click-x3', false, '「刷新订单状态」按钮未找到，刷新前置动作未执行');
+       break;
+     }
+     await btn.evaluate((b) => b.click())
+       .catch((e) => note('refresh-click-x3', false, `click failed: ${e}`));
      await page.waitForTimeout(1200);
    }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_02_sync_face.mjs:56-59 ───
[test · low] try 块只有 finally 没有 catch：登录失败（waitForURL 超时）或任一 waitForSelector
超时抛错时，浏览器虽会关闭，但异常会继续向上传播，`RESULT {...}` 汇总行不会输出，失败原因只剩裸堆栈、机器不可解析，证据日志不完整（round 1 脚本同样如此）。建议补 catch
记录失败步骤，保证 RESULT 总会输出。

+ } catch (err) {
+   note('script-error', false, String(err));
  } finally {
    await browser.close();
  }
  console.log('RESULT ' + JSON.stringify(results));


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_02_sync_face.mjs:51-52 ───
[maintainability · low] 同一「同步回访面」验证逻辑在 r4-flow、r4-flow2、r4-flow3 三份 browser_02_sync_face.mjs
中近乎逐行复制，且本轮已出现复制漂移——round 1 中解释 DOM 级 click
原因的注释在本副本中被删除。后续若页面文案/断言口径调整，任何一处漏改都会使对应轮次的证据脚本静默失效（叠加上面的静默 catch
更难察觉）。若证据目录允许共享代码，建议抽出公共的登录/断言脚手架；若各轮必须保持冻结快照，至少应保证副本逐字一致。

-   note('sync-return-no-paid-claim', !body.includes('已付款'),
-     'no 已付款 claim anywhere on the sync-return face');
+ // 可抽取共享断言（示意）：
+ // import { assertSyncReturnFace } from '../_lib/sync_face_asserts.mjs';
+ // await assertSyncReturnFace(page, note);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:45-47 ───
[test · low] 账单页断言未与目标订单绑定：waitForSelector('text=已生效') 与 billing.includes('已生效') 都是整页文本匹配。已核实前端
BillingPage 运行时『已生效』仅来自套餐行 purchase.state === 'active' 后缀（BillingPage.tsx L119，且页面只有单一套餐行），但该
purchase 是空间级当前购买投影（purchaseStatus），并非按 orderId 查询——若空间在本订单之前已存在 active
购买，断言仍会通过，取证结论无法区分『本订单生效』与『空间原本已生效』。r4-flow 为全新 seed 空间时风险有限，但作为 AC1 尾段证据，建议将断言收敛到套餐行并核对该行文本。

    await page.goto(`${WEB}/platform/billing`);
-   await page.waitForSelector('text=已生效', { timeout: 45000 });
-   const billing = await page.locator('main').innerText();
+   const planRow = page.locator('li', { hasText: '套餐' }).first();
+   await planRow.waitFor({ timeout: TIMEOUT_MS });
+   const rowText = await planRow.innerText();
+   note('billing-active-face', rowText.includes('已生效'), `billing plan row: ${rowText}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:51-55 ───
[test · low] 异常路径丢失结果输出：try 块内的 goto/fill/waitForSelector 均无
catch，任一等待超时（如『权益已生效』未出现）时异常直接以未处理拒绝抛出，console.log('RESULT ...') 不再执行，note() 已记录的 PASS/FAIL
明细与退出码判断全部丢失，失败时只能看到 Playwright 堆栈，与脚本自身『逐步 note + RESULT 汇总』的取证设计不符。建议补 catch 记录失败步骤，保证失败路径也输出
RESULT（script-error 置 ok=false，退出码语义不变）。

+ } catch (error) {
+   note('script-error', false, error instanceof Error ? error.message : String(error));
  } finally {
    await browser.close();
  }
  console.log('RESULT ' + JSON.stringify(results));
  process.exit(results.every((r) => r.ok) ? 0 : 1);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:1-1 ───
[maintainability · low] 三份近乎相同的脚本副本：r4-flow、r4-flow2、r4-flow3 三目录各自维护 browser_01~04、seed
等文件，本文件与另两目录的 browser_04_active_face.mjs 断言逐字重复（r4-flow2 L42、r4-flow3 L34 同为
no-awaiting-leftover）。前端文案（权益已生效/待付款/已付款，权益处理中）或选择器一旦变更需三处同步，任一遗漏即导致该组取证静默失效。取证目录保持快照自包含有其合理性，但建议至少在
文件头注明三组脚本的同步维护约定，或将登录+断言骨架提取到 issue-72-flow-evidence-82 目录层共享。



─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:28-29 ───
[maintainability · low] 超时值硬编码 5 次且冗余：45000 在本文件出现 5 次（setDefaultTimeout + 4 处显式 timeout），其中第 23 行
setDefaultTimeout(45000) 已将该值设为页面所有等待操作的默认超时，后续三处显式 { timeout: 45000 } 与默认值完全相同、纯冗余。建议提取 const
TIMEOUT_MS 常量并删除等值的显式 timeout 传参（或统一引用常量），降低调整时的多处修改；同时 '#auth-email'、'form[aria-label="Login
form"]' 等选择器与 LoginPage 实现强耦合（当前已核实一致），前端变更会使脚本以超时形式静默失败，集中常量也便于排查。

+ const TIMEOUT_MS = 45000;
+ // ...
    const page = await (await browser.newContext()).newPage();
-   page.setDefaultTimeout(45000);
+   page.setDefaultTimeout(TIMEOUT_MS);
+   // 后续 waitForURL/waitForSelector 无需再显式传 timeout（与默认值相同）


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:66-66 ───
[test · low] checkoutHref 取出后没有任何断言：pay-link-present 只验证了「前往支付」文本存在，而 getAttribute('href') 可能返回
null（如 <a> 未渲染 href、或匹配到错误态占位元素）。此时 order-info.json 会记录 "checkoutHref": null，但本轮取证所有步骤仍显示
PASS，「渠道支付链接已渲染」这一证据并不完整。建议补一条对 href 非空且格式合法的断言。

    const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
+   note('pay-link-href-valid', typeof checkoutHref === 'string' && /^https?:\/\//.test(checkoutHref), String(checkoutHref));


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:80-80 ───
[test · low] order-info.json 是无条件写入的：即使 order-id-visible（orderId 可能为 ''）或
purchase-wire-provider-alipay 等关键断言失败，失败状态的数据仍会被固化为 artifact，而后续 legs（browser_02/03/04）以该 orderId
作为输入运行。操作者若依据文件内容而非退出码继续执行后续 legs，会在无效/错误订单上采集 sync/paid 证据。建议仅在关键断言通过时写入，或在 JSON 中附带 leg1 是否通过的标志。

+   const criticalOk = /^ord_[0-9a-f]+$/.test(orderId) && providerOk;
+   if (criticalOk) {
-   writeFileSync(`${EV}order-info.json`, JSON.stringify({ orderId, checkoutHref, purchases }, null, 2));
+     writeFileSync(`${EV}order-info.json`, JSON.stringify({ orderId, checkoutHref, purchases }, null, 2));
+   }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:82-84 ───
[test · medium] 主流程只有 try/finally、没有 catch：任一 waitForSelector 超时或 goto 失败会直接抛出，最终 'RESULT'
汇总行不会输出，也不会留下失败时刻的截图（脚本以 unhandled rejection 形式退出）。对取证脚本而言，失败现场恰恰是最需要保留的证据——每步的 PASS/FAIL
行虽会实时打印，但缺少汇总 JSON 与失败截图会让事后归档不完整。建议增加 catch，先输出 RESULT 汇总再以非零码退出。

+ } catch (err) {
+   console.log('RESULT ' + JSON.stringify(results));
+   console.log('ERROR ' + (err instanceof Error ? err.message : String(err)));
+   process.exitCode = 1;
  } finally {
    await browser.close();
  }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:43-44 ───
[security · high] 脚本把登录响应原文（含可用的 Bearer token 与 refresh_token，均为 HS256 JWT，前缀 eyJhbGciOiJI…）写入 git
跟踪的 evidence 文件 seed-login-a/b.json，且这两个文件已随本次变更提交（docs/plans/issue-72-flow-evidence-82/ 下无
.gitignore 规则）。脚本头部声明"凭据 env 注入、源码无字面量"（FLOW82_R4_PW），但运行期签发的可用令牌被固化进版本库，直接违反本分支安全验收条件
3（源码/示例/测试不得写入可用凭据字面量）；若 82flow 实验栈复用固定 JWT 签名密钥，提交的令牌可对任何同密钥环境重放。建议落盘前剥离令牌字段、完整响应改写入非跟踪路径供后续
browser_*.mjs 读取。

- LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"); echo "$LOGIN_A" > "$EV/seed-login-a.json"
- LOGIN_B=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local"); echo "$LOGIN_B" > "$EV/seed-login-b.json"
+ # evidence 只保留脱敏副本，完整令牌写入非跟踪路径供 browser_*.mjs 使用
+ jq 'del(.token, .refresh_token)' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ printf '%s' "$LOGIN_A" > "$EV/.runtime/seed-login-a.json"   # .runtime/ 加入 .gitignore


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:55-55 ───
[security · medium] 本行 insert 与下一行 select 均把 $UID_B 直接拼进 SQL，而 UID_B 来自后端 HTTP 登录响应经 jq
解析，属外部输入：响应被篡改（如 id 含单引号）即可注入任意 SQL，直接违反本分支验收条件 2（数据库操作禁止拼接组装 SQL）。且该写法已被
r4-flow2/seed.sh、r4-flow3/seed.sh 原样复制扩散。sqlite3 CLI 无原生参数绑定，建议插值前先做格式白名单校验，使注入在语法上不可能。

+ case "$UID_B" in
+   ''|*[!0-9a-fA-F-]*) say "FAIL: unexpected UID_B from login response"; exit 1 ;;
+ esac
  sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:16-16 ───
[bug · medium] $PW 有 :? 守卫，但 $DB_PATH 完全未校验：未设置时要等 set -u 在后面的 sqlite3 行抛出晦涩的 unbound
variable（此时占位租户与主角已注册，留下半成品 seed 状态）；若指向其他既有数据库，plan_publish 授权种子行会被写进错误的库，导致关键前置静默失效或污染其他库。r4-flow
README 已约定 DB_PATH 由 env 提供，建议与 PW 一样在开头做存在性守卫。

  PW="${FLOW82_R4_PW:?missing required env FLOW82_R4_PW}"
+ DB_PATH="${DB_PATH:?missing required env DB_PATH (per r4-flow README: data/issue82-r4verify.db)}"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:75-76 ───
[test · medium] Lago 落库断言只检查"存在 amount_cents==9900 的计划且 N>=1"，对本轮发布不具特异性：同一 82flow 栈上前几轮（含
reverify、r4-flow 各轮）早已 seed 过同样的 9900 pro 计划，即使本轮发布未真正同步到 Lago，断言也会因历史计划通过，形成假阳性、削弱证据链。前面的 publish
receipt 断言只证明后端发出了命令，无法替代此处的 Lago 侧验证。建议发布前取基线计数并断言计数增加，或按本轮 external plan code / created_at 时间窗过滤。

- N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans.json")
- [ "$N" -ge 1 ] || { say "FAIL: no 9900 plan visible in Lago"; exit 1; }
+ # 在 publish 前先取基线 BASE_9900，此处改为断言计数较基线增加
+ [ "$N" -gt "$BASE_9900" ] || { say "FAIL: this round's plan not visible in Lago (base=$BASE_9900 now=$N)"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:30-31 ───
[bug · low] 默认占位数 6 只覆盖注释所记的 round 1（1..6）；注释自述 round 2 占用 1..8（r4-flow3 记到 1..10），而同系列的
r4-flow2/r4-flow3 复制了同一个默认值 6。按默认值重跑时占位不足，主角会落在已被占用的 Lago 租户身份上，造成跨轮次数据串扰——正确性完全依赖调用方记得每轮显式调大
FLOW82_PAD_COUNT。建议改为必填 env 或默认取已记录的最大占位数。

- say "== register ${FLOW82_PAD_COUNT:-6} placeholders (absorb Lago tenant ids) =="
- for i in $(seq 1 "${FLOW82_PAD_COUNT:-6}"); do
+ : "${FLOW82_PAD_COUNT:?set to the max Lago tenant id already occupied on the 82flow stack (see header comment)}"
+ for i in $(seq 1 "$FLOW82_PAD_COUNT"); do


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:52-52 ───
[test · low] 失败诊断不足且守卫名不副实：reg() 只打印 HTTP 码不判失败；login/draft/publish 的 curl 均未加 -f 或断言状态码，非 JSON
错误体会被原样 tee 进 evidence；本行守卫只校验 TENANT_A/TENANT_B，TOKEN_B 为 null（登录失败）时要到 publish 环节才间接失败，报错会指向
publish 而非登录；TOKEN_A 提取后在本脚本中从未使用。建议守卫覆盖 token 字段（或移除未用的 TOKEN_A），关键 curl 统一加状态断言。

- [ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }
+ [ "$TOKEN_B" != "null" ] && [ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] \
+   || { say "FAIL: login response incomplete"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:70-70 ───
[test · low] docker exec 内 "select value from api_keys limit 1" 无 ORDER BY 任取一把 Lago API key：栈上存在多把
key（或后续新增低权限 key）时可能取到权限不足的一把，使 plans 查询间歇性 401，断言变成非确定性脆弱点。建议加确定性排序，或改由 lab 环境变量注入专用 key。

- LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]')
+ LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:24-24 ───
[bug · low] reg() 与 login() 均以字符串插值把 $PW、邮箱拼进 JSON 请求体：当 FLOW82_R4_PW 含 " 或 \ 时会构造出非法 JSON，注册/登录以
400 失败且难排查。建议用 jq -n 构造请求体（login() 同理）。

-     -d "{\"username\":\"$1\",\"email\":\"$1\",\"password\":\"$PW\"}"
+     -d "$(jq -nc --arg u "$1" --arg p "$PW" '{username:$u,email:$u,password:$p}')"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_03_paid_face.mjs:54-57 ───
[maintainability · medium] 主流程只有 try/finally 而无 catch：当 waitForSelector
超时（例如修复未生效、『已付款，权益处理中』未出现——这恰是本轮复验要捕捉的核心失败场景）或登录/定位失败时，异常以未捕获方式抛出，note() 不会记录 FAIL，最终的 RESULT JSON
也不会输出，下游无法区分『断言失败』与『登录/选择器等脚本基础设施故障』，削弱了取证脚本的诊断价值。建议补充 catch 兜底，把异常记入 results 并确保 RESULT
汇总总能打印（r4-flow / r4-flow3 的同名脚本存在同样问题，可一并修正）。

+ } catch (err) {
+   results.push({ step: 'script-error', ok: false, detail: String(err?.message ?? err) });
  } finally {
    await browser.close();
  }
  console.log('RESULT ' + JSON.stringify(results));


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs:39-40 ───
[test · medium] 竞态规避方式脆弱：`waitForSelector('text=权益已生效')` 通过后仅靠固定 `waitForTimeout(4500)`
等旧状态文案消失（注释自述 one poll tick can still hold the old Status line），随后立即断言
`!body.includes('待付款')`。对比同系列脚本：r4-flow（第 1 轮）正是无此等待导致失败，本文件与 r4-flow3 用固定 4.5s/2s 睡眠补救——轮询周期超过 4.5s
的慢环境仍会间歇性 FAIL，快环境则白白多等。建议改为显式等待旧文案消失（条件与断言保持同一判定），既消除竞态又缩短等待：

-   await page.waitForTimeout(4500); // round-1 observation: one poll tick can still hold the old Status line
+   // round-1 竞态：等待旧状态文案真正消失（与下方断言同一判定），替代固定 4.5s 睡眠
+   await page.waitForFunction(
+     () => {
+       const text = document.querySelector('main')?.innerText ?? ''
+       return !text.includes('待付款') && !text.includes('已付款，权益处理中')
+     },
+     { timeout: 45000 },
+   )
    const body = await page.locator('main').innerText();


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs:52-55 ───
[test · low] 异步步骤无 catch：登录凭据错误时 `waitForURL` 需等满 45s 才以超时堆栈失败；任何 await 抛错时因无 catch，末尾的 `RESULT`
JSON 汇总行不会输出（证据采集时缺少整体结论），错误仅为原始 Playwright 堆栈，无法直接看出卡在哪一步。建议补 catch 并复用现有 note()
机制记录失败步骤，退出码逻辑无需改动即可保持非零：

+ } catch (err) {
+   note('browser-leg-error', false, err?.message ?? String(err));
  } finally {
    await browser.close();
  }
  console.log('RESULT ' + JSON.stringify(results));


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs:48-50 ───
[test · low] 断言强度不足：前一行 `waitForSelector('text=已生效')` 成功后，`billing.includes('已生效')`
几乎必然为真（同义反复），且未限定在套餐行——当前 BillingPage 仅在 active 状态渲染 ` ·
已生效`，暂无误匹配，但页面文案演进后可能假阳性。建议至少把断言收敛到套餐行（billing-summary-list 首行），与断言语义"billing plan row"保持一致：

    const billing = await page.locator('main').innerText();
-   note('billing-active-face', billing.includes('已生效'),
+   const planRowText = await page.locator('[data-testid="billing-summary-list"] li').first().innerText();
+   note('billing-active-face', planRowText.includes('已生效'),
      `billing plan row: ${billing.split('\n').find((l) => l.includes('已生效')) ?? '(absent)'}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs:29-33 ───
[maintainability · low] 登录流程块在 82 系列 15 个 browser_*.mjs 脚本（r4-flow/r4-flow2/r4-flow3 各 leg
及首轮脚本）中近乎逐字重复，本文件又是一份拷贝。若选择器或登录交互发生变化需同步修改多处，易造成各轮证据脚本漂移。既有轮次可冻结不动，建议后续新增脚本提取共享 helper（如同目录
`_login.mjs` 导出 `login(page, { web, email, password })`）：

+   // 建议后续脚本复用共享 helper：
+   // import { login } from '../_login.mjs';
+   // await login(page, { web: WEB, email: EMAIL, password: PASSWORD });
    await page.goto(`${WEB}/login`);
    await page.fill('#auth-email', EMAIL);
    await page.fill('#auth-password', PASSWORD);
    await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
    await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_02_sync_face.mjs:44-46 ───
[test · low] 3500ms 的「one poll window」等待与前端 CheckoutPage 的 POLL_INTERVAL_MS =
3000（CheckoutPage.tsx:11）是隐式魔法数耦合，且 round 1 中说明该耦合的注释（// let one poll tick
land）在本副本被删除。风险：一旦前端轮询间隔上调（如改为 5000ms），3500ms 窗口内将一个 poll tick 都不会发生，但断言详情仍写「after revisit/refresh
x3/reload/poll」并以 PASS 收尾——「轮询窗口不得推进订单」这一证据点会静默空转，与脚本要防范的假阳性证据同类。建议至少恢复耦合注释，并在等待期间对 GET
/commercial/orders/:id 的请求计数做正向断言（pollTicks > 0），确保证据确实覆盖了至少一次轮询。

+   let pollTicks = 0;
+   page.on('request', (r) => { if (r.url().includes(`/commercial/orders/${ORDER}`)) pollTicks += 1; });
    await page.reload();
    await page.waitForSelector('text=待付款（权益未开通）', { timeout: 45000 });
+   // 前端轮询间隔 POLL_INTERVAL_MS = 3000（CheckoutPage.tsx）；3500ms 仅够一个 tick
    await page.waitForTimeout(3500);
+   note('sync-return-poll-fired', pollTicks > 0, `observed ${pollTicks} poll request(s) during the window`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:38-40 ───
[test · low] 恒真断言，FAIL 分支不可达：前一行 waitForSelector('text=权益已生效') 一旦成功，即代表该文案已渲染（CheckoutPage 将状态文案渲染在
<main> 内，见 CheckoutPage.tsx L276/L31），随后的 body.includes('权益已生效') 恒为
true；若文案始终未出现，只会以超时异常收场（叠加本组已确认的『无 catch』问题，RESULT 中不会留下任何 FAIL 记录）。也就是说 checkout-active-face 这一步的
ok 判定永远无法产生 FAIL，逐步 PASS/FAIL 取证对该步骤失去判别力。建议把等待收敛的 waitForSelector 结果本身作为 ok 记录（try/catch 捕获超时记
FAIL），并删除恒真的 includes 复核。

+   let activeOk = false;
+   try {
-   await page.waitForSelector('text=权益已生效', { timeout: 45000 });
+     await page.waitForSelector('text=权益已生效', { timeout: 45000 });
-   const body = await page.locator('main').innerText();
-   note('checkout-active-face', body.includes('权益已生效'), 'checkout shows 权益已生效');
+     activeOk = true;
+   } catch { /* 超时收敛失败 → 记 FAIL 而非抛异常丢失 RESULT */ }
+   note('checkout-active-face', activeOk, 'checkout shows 权益已生效');


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:46-48 ───
[test · low] 与上方 checkout-active-face 相同的恒真模式：waitForSelector('text=已生效') 成功即保证 '已生效' 存在（BillingPage
内容均渲染在 <main> 内），随后 billing.includes('已生效') 恒为 true，该步骤的 FAIL 分支同样不可达；文案缺失时只表现为超时异常。建议同法处理：将
waitForSelector 的成败直接作为该步骤的 ok 记录（try/catch 捕获超时），保留行级 detail 用于输出命中的行文本。

+   let billingOk = false;
+   try {
-   await page.waitForSelector('text=已生效', { timeout: 45000 });
+     await page.waitForSelector('text=已生效', { timeout: 45000 });
+     billingOk = true;
+   } catch { /* 超时收敛失败 → 记 FAIL */ }
    const billing = await page.locator('main').innerText();
-   note('billing-active-face', billing.includes('已生效'),
+   note('billing-active-face', billingOk,


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_03_paid_face.mjs:41-43 ───
[maintainability · low] 整个浏览器验证流程只有 try/finally 而没有 catch：若任一 waitForSelector
超时（例如支付回调尚未投影、页面停留在待付款面），脚本会以未捕获异常退出，只打印 Playwright 堆栈，不会输出 RESULT JSON 汇总，且此时已记录的 PASS/FAIL
步骤也会丢失，不利于证据留档。建议增加 catch 记录失败步骤并保证 RESULT 行始终输出（退出码仍为 1），与兄弟脚本保持一致地增强。

+ } catch (err) {
+   note('script-error', false, String(err?.message ?? err));
  } finally { await browser.close(); }
  console.log('RESULT ' + JSON.stringify(results));
  process.exit(results.every((r) => r.ok) ? 0 : 1);


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_03_paid_face.mjs:21-21 ───
[maintainability · low] 45000ms 超时值在本脚本中硬编码重复了 5 次（setDefaultTimeout 及各
waitForSelector/waitForURL），调整超时时需要逐处修改且易漏。建议提取为模块级常量（如 const TIMEOUT_MS = 45_000）统一引用。

-   page.setDefaultTimeout(45000);
+ const TIMEOUT_MS = 45_000;
+ // ...
+   page.setDefaultTimeout(TIMEOUT_MS);


─── deploy/lago-lab/payment-settle-trigger/phases.py:454-461 ───
[bug · medium] Stripe 客户对象在创建后未立即登记到 ctx.state，早失败路径会泄漏该对象且 cleanup 无法感知：步骤 3 已在 Stripe 侧创建了
customer（并 attach 了 3DS pm），但 ctx.state["customer"] 直到步骤 4 Lago customer 创建成功后才写入。若 Lago create 被拒（如
external_id 冲突/校验失败返回非 2xx，此处按设计走 FAIL 报告返回）或步骤 3-4 之间抛出 clients.LabError（见另一条评论），cleanup 拿不到
stripe_customer_id，永不删除 cus_...，直接违反 phase_cleanup 的 expected 契约（"every lab object created this run
is deleted"），测试账号残留会随失败运行累积。建议在 create_customer 返回后立即把 id 记入 state，并让 phase_cleanup 独立于
state["customer"] 读取它（state.get("stripe_customer_id") or (customer or
{}).get("stripe_customer_id")）。

+         ctx.state["stripe_customer_id"] = stripe_customer_id  # cleanup anchor: set at creation
          cs, _cb = ctx.lago.post("/api/v1/customers", fixtures.customer_payload(
              ext, "WeKnora T11 customer A (3DS card)", stripe_customer_id,
              payment_provider_code=provider.get("code")))
          if not _ok(cs):
              return _report(ctx, "gated_3ds", expected,
                             {"error": f"Lago customer create rejected (HTTP {cs})"}, FAIL)
          ctx.state["customer"] = {
              "external_id": ext, "stripe_customer_id": stripe_customer_id}


─── deploy/lago-lab/payment-settle-trigger/phases.py:488-490 ───
[bug · medium] 三个 phase 的兜底 except OSError 覆盖不了所引客户端的实际失败类型：clients.LabError 继承
RuntimeError（clients.py 第 67 行），而 StripeTestClient.create_customer / attach_payment_method /
set_default_payment_method 在非 200 时 raise LabError（第 417/426/435 行），LagoRestClient 对跨源重定向 raise
OffOriginRedirect（LabError 子类，第 79 行）。这些异常会直接逃出 phase 函数（settle_probe / settle_trigger 的同款 except
OSError 亦然），由 run_lab.run_one 的兜底 except Exception 记为
unexpected_error——绕过了本文件自己声明的裁决契约：观测/证据字段丢失、ctx.drain_notes() 未被调用导致 contract_notes 清空，且"环境类失败应判
blocked-env"的分类意图失效。建议把三处兜底改为 except (OSError, clients.LabError)（OffOriginRedirect 已被 LabError
覆盖），或对 Stripe 辅助调用单独捕获并产出带 evidence 的结构化 FAIL 报告。

-     except OSError as error:
+     except (OSError, clients.LabError) as error:
          return _blocked(ctx, "gated_3ds",
                          f"transport error ({error.__class__.__name__})", expected)


─── deploy/lago-lab/payment-settle-trigger/phases.py:814-817 ───
[documentation · low] 注释声称的执行顺序保护并不存在："(guarded by the runner import)" —— run_lab.py 只是 import
phases 并在 PHASE_SEQUENCE 中手工复刻同一序列（第 68-72 行），没有任何代码或测试比较两个元组；对比 payment-activation 实验室，那里有
test_phases.py::test_phase_order_contract_matches_runner_order 真正守住这一契约，而本目录没有任何测试文件。目前 PHASE_ORDER
本身也无人引用（run_lab 只引用各 phase 函数），一旦有人向 PHASE_ORDER 增补 phase 而未同步 PHASE_SEQUENCE，runner
会静默跳过新阶段，且注释给出的安全承诺是虚假的。建议照搬 payment-activation 的守卫测试，或改写注释去掉不存在的 guard 声明。

  # Execution-order contract: gated_3ds (setup) -> settle_probe (P-A) ->
  # settle_trigger (P-B/C/D/E) -> cleanup. The runner's PHASE_SEQUENCE mirrors
- # this tuple (guarded by the runner import).
+ # this tuple (guarded by a runner-order test mirroring payment-activation's
+ # test_phase_order_contract_matches_runner_order).
  PHASE_ORDER = (


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:39-42 ───
[bug · low] 脚本中全部 curl 调用(reg()、login()、draft/publish、Lago plans 查询)均未设置 --connect-timeout /
--max-time。若 :8093 后端或 :48889 Lago 无响应、或出现半开连接,脚本会在无人值守的 evidence 采集过程中无限期挂起,seed-run.txt
停在中途、evidence 半成品,与各环节的 FAIL-fast 断言设计不符。建议对全部 curl 统一追加如 --connect-timeout 5 --max-time
30,使挂起转化为可诊断的快速失败。

  login() { # login <email> -> token
-   curl -s -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' \
+   curl -s --connect-timeout 5 --max-time 30 -X POST "$BACKEND/api/v1/auth/login" -H 'Content-Type: application/json' \
      -d "{\"email\":\"$1\",\"password\":\"$PW\"}"
  }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_02_sync_face.mjs:30-30 ───
[test · medium] 三次手动刷新点击的失败被 `.catch(() => {})` 静默吞掉，且成功与否不进入
`results`：若「刷新订单状态」按钮因文案/结构变更未渲染，这一步会每次空等满 45s 默认超时后被跳过，最终 `sync-return-still-awaiting` 仍可能仅凭 reload
通过。该脚本的核心证据之一就是"手动刷新不推进订单"，但 RESULT 里没有任何一步证明刷新路径真的被执行。建议统计成功点击次数并作为 note 落盘。

-     await page.locator('button:has-text("刷新订单状态")').first().evaluate((b) => b.click()).catch(() => {});
+   let refreshClicked = 0;
+   for (let i = 0; i < 3; i++) {
+     try {
+       await page.locator('button:has-text("刷新订单状态")').first().evaluate((b) => b.click());
+       refreshClicked += 1;
+     } catch {}
+     await page.waitForTimeout(1200);
+   }
+   note('sync-return-refresh-clicked', refreshClicked === 3, `manual refresh clicked ${refreshClicked}/3`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_02_sync_face.mjs:1-2 ───
[maintainability · low] 从 r4-flow/r4-flow2 复制时丢掉了两处关键注释：为什么用 DOM 级 `.evaluate((b) => b.click())`
而非普通 click（页面每 3s 轮询重渲染，Playwright actionability 检查无法 settle），以及 `waitForTimeout(3500)`
是为了等一个轮询周期落地。同目录 browser_01 也保留了 "Same assertions as rounds 1-2; see ../r4-flow2/..."
的指引而本文件没有，下一轮复验的读者会无从判断这些非直觉写法的来由。建议在头部补一行指向 r4-flow2 并说明断言一致，循环处补回注释。

- // Issue #82 R-4 re-verification ROUND 3 — browser leg 2 (AC2 sync-return face).
+ // Issue #82 R-4 re-verification ROUND 3 (post-fix #2) — browser leg 2 (AC2 sync-return face).
+ // Same assertions as rounds 1-2; see ../r4-flow2/browser_02_sync_face.mjs.
  import { createRequire } from 'node:module';


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_02_sync_face.mjs:41-41 ───
[test · low] 只有 `finally` 没有 `catch`：任何一步抛错（订单被错误推进导致 `waitForSelector` 超时等）都会以 unhandled rejection
直接退出，既不打印 `RESULT` 汇总，也不落失败现场截图——而本目录之所以存在 round 2/3，恰恰是因为失败场景真实发生过，失败证据正是最需要的产物。建议补 catch 记录失败 note
并截图后再抛出（需把 `page` 提到 try 外声明以便 catch 中引用）。

+   } catch (err) {
+     note('sync-return-error', false, String(err));
+     await page?.screenshot({ path: `${EV}03-sync-return-failure.png`, fullPage: true }).catch(() => {});
+     throw err;
- } finally { await browser.close(); }
+   } finally { await browser.close(); }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:66-68 ───
[test · medium] 脚本只有 try/finally 而无 catch 分支：任一 waitForSelector 超时或前置步骤抛错时，进程在输出 RESULT
汇总行之前即以未捕获异常退出（退出码同样非零），与断言失败无法区分，且已完成步骤的取证结果不会汇总落盘。建议捕获异常后记入 note 并照常输出 RESULT 汇总，便于区分环境故障与断言失败。

+ } catch (err) {
+   note('script-error', false, String(err?.message ?? err));
  } finally { await browser.close(); }
  console.log('RESULT ' + JSON.stringify(results));
  process.exit(results.every((r) => r.ok) ? 0 : 1);


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:40-41 ───
[test · low] note('channel-explicit-alipay-select', true, ...) 的 ok 恒为 true，不构成有效断言却以 PASS
记入取证结果，会稀释证据可信度（page.check 失败时是抛错而非记 FAIL）。建议改为可验证的布尔表达式，例如 check 之后再次断言 isChecked 为 true。

    await page.check('input[name="payment-channel"][value="alipay"]');
-   note('channel-explicit-alipay-select', true, 'explicitly selected the 支付宝 radio');
+   note('channel-explicit-alipay-select', await page.isChecked('input[name="payment-channel"][value="alipay"]'), 'explicitly selected the 支付宝 radio');


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:52-52 ───
[test · low] checkoutHref 通过 getAttribute('href') 获取后未做非空校验就写入 order-info.json：pay-link-present
仅断言了页面文本包含「前往支付」，若链接元素缺失 href 属性则取证文件中记录为 null 而无任何 FAIL 提示。建议增加一条 note 断言 checkoutHref 为非空且以
http(s) 开头，保证取证完整性。

    const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
+   note('pay-link-href-valid', typeof checkoutHref === 'string' && /^https?:\/\//.test(checkoutHref), `checkout href: ${checkoutHref}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:54-54 ───
[security · medium] UID_B 未做空值校验且直接拼入 SQL：脚本只断言 TENANT_A/TENANT_B 非 null，未校验 UID_B/TOKEN_B；若登录响应缺
.user.id，jq -r 会输出字面量 "null"，该值被拼进 insert 语句写入 commercial_grants.user_id，导致授权行无效、后续 publish
401/403，且 "granted: 1 row(s)" 的输出会误导排障。同时按项目安全约束（作为验收条件）"数据库查询一律使用参数绑定，禁止拼接组装
SQL"，从登录响应解析的值不应未经校验直接内插进 SQL。sqlite3 CLI 无绑定参数，至少应在拼接前校验 UID_B 为非空 UUID 格式（并顺带校验 TOKEN_B 非
null）。r4-flow / r4-flow3 存在相同问题。

+ [ "$UID_B" != "null" ] && [ "$TOKEN_B" != "null" ] || { say "FAIL: login user id/token missing"; exit 1; }
+ [[ "$UID_B" =~ ^[0-9a-fA-F-]{36}$ ]] || { say "FAIL: UID_B is not a uuid: $UID_B"; exit 1; }
  sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:74-75 ───
[test · medium] Lago 断言存在跨轮次假阳性风险：注释声称按 amount+currency+interval 校验，实际 select(.amount_cents==9900)
只过滤金额（未校验 code/currency/interval）；而 round 1（r4-flow）已在同一 82flow 共享 Lago 栈上发布过 9900 计划并留下残留，本轮若
publish 受理但 Lago 同步（lago 投影）失败，N>=1 仍会因残留数据通过，断言无法证明"本轮发布已到达 Lago"，产出无效证据。建议在发布前采集 Lago 中 9900
计划数量作为基线，发布后要求数量严格增加（或按本轮唯一 plan code 精确匹配）。r4-flow3 同样存在该弱断言。

+ # 发布前采集基线（在 draft/publish 之前拉取一次）
+ BASE_N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans-baseline.json")
+ ...
  N=$(jq '[.plans[] | select(.amount_cents==9900)] | length' "$EV/api-03-lago-plans.json")
- [ "$N" -ge 1 ] || { say "FAIL: no 9900 plan visible in Lago"; exit 1; }
+ [ "$N" -gt "$BASE_N" ] || { say "FAIL: no NEW 9900 plan visible in Lago (base=$BASE_N now=$N)"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:30-30 ───
[bug · medium] FLOW82_PAD_COUNT 默认值与注释意图矛盾：脚本头注释及占位注释均明确 "round 1 used 1..6, round 2 uses
1..8"（本轮需吸收共享 Lago 栈上已占用的 tenant id 1..8），但代码默认值仍为 :-6，且全仓库无任何位置导出该变量。重跑时若忘记导出
FLOW82_PAD_COUNT=8，只会注册 6 个占位，主角将落在 tenant id 7/8，与 round 1 主角在共享 Lago 栈上的 weknora-tenant-7/8
冲突，破坏脚本赖以成立的轮次隔离假设（本次实际运行已正确导出——证据文件中主角 tenant B=10——但脚本作为可复现证据默认值不应指向错误轮次）。建议把默认值改为 8，或与
FLOW82_R4_PW 一致用 :? 强制显式提供。r4-flow3（注释为 round 3: 1..10）默认值同样是遗留的 6。

- for i in $(seq 1 "${FLOW82_PAD_COUNT:-6}"); do
+ PAD_COUNT="${FLOW82_PAD_COUNT:-8}"   # round 2 需吸收 1..8，不可回退到 round 1 的 6
+ say "== register $PAD_COUNT placeholders (absorb Lago tenant ids) =="
+ for i in $(seq 1 "$PAD_COUNT"); do


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:15-15 ───
[maintainability · low] DB_PATH 缺少与 FLOW82_R4_PW 一致的前置校验：脚本对密码用 :? 强制检查，但后续两次 sqlite3 依赖的 DB_PATH
既无默认值也无存在性检查；set -u 下未导出时会以晦涩的 unbound variable 中断，且其为相对路径（README 约定 data/issue82-r4verify.db，依赖 cd
后的 worktree 根与后端 cwd 一致），错误时不易定位。建议在头部与 PW 一并强校验。

  PW="${FLOW82_R4_PW:?missing required env FLOW82_R4_PW}"
+ DB_PATH="${DB_PATH:?missing required env DB_PATH (the sqlite db the :8093 backend runs on, e.g. data/issue82-r4verify.db)}"


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_03_paid_face.mjs:42-43 ───
[bug · low] console.log('RESULT ...') 之后立即调用 process.exit() 存在输出截断风险：Node.js 在 stdout 为管道（如 `node
browser_03_paid_face.mjs "$ORDER" > evidence/xxx.log` 或经 `| tee`、被 lab 脚本捕获）时写入是异步的，process.exit()
会丢弃尚未 flush 的写请求，导致这行核心证据汇总（RESULT JSON 及最终 PASS/FAIL 判定）可能丢失，且退出码已发出、无法重试。Node
官方文档对此有明确警告。此处该脚本本意就是输出留档，建议改用 process.exitCode 赋值，让事件循环自然排空后再退出，保证 stdout 完整落盘。

  console.log('RESULT ' + JSON.stringify(results));
- process.exit(results.every((r) => r.ok) ? 0 : 1);
+ process.exitCode = results.every((r) => r.ok) ? 0 : 1;


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_02_sync_face.mjs:37-37 ───
[test · medium] 三条 note 都未断言页面呈现的就是传入的目标订单 ORDER。页面的恢复依赖 router 从 `?order=` 查询参数取
orderId（apps/web/src/router.tsx:690）：一旦该接线回归（查询参数被丢弃/trim 为空），CheckoutPage 会以空 orderId 走
quote+purchase 分支新建一张订单并同样渲染「待付款（权益未开通）」——脚本将对一张与目标 ORDER 无关的新订单全量 PASS，产出假阳性证据（AC2
要防的正是这类回归，且脚本不监听请求，无法察觉多出的 POST /purchases）。建议像本目录 browser_01 提取订单号那样，补一条订单一致性断言再下结论。

+   const orderLine = body.split('\n').find((l) => l.includes(ORDER)) ?? '';
+   note('sync-return-order-match', orderLine.includes(ORDER), `resumed face is target order ${ORDER}`);
    note('sync-return-still-awaiting', body.includes('待付款（权益未开通）'), 'after revisit/refresh x3/reload/poll: still 待付款（权益未开通）');


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:34-36 ───
[test · medium] 注册调用的 HTTP 状态只被打印、从未被断言。`reg` 失败（HTTP 4xx/5xx，或后端未启动时 curl 输出
000）时脚本照常继续：只要有一个占位注册失败，主角就会静默落到已被前几轮在共享 Lago 栈上占用的 tenant id 上，直接破坏本脚本赖以成立的"占位吸收 Lago tenant
id"隔离机制，而后续的 TENANT_A/TENANT_B 非 null 检查发现不了（登录仍可能成功）。重跑同一前缀时 register 返回 4xx（邮箱已存在）但 login
依旧成功，脚本会带着旧用户/旧 tenant 继续产出证据。另外 `say "... $(reg ...)"` 的退出状态取自 say，set -e 不会拦截命令替换里的 curl
失败。建议对每次注册断言 2xx，非 2xx 立即 fail closed（重跑容忍 409 也应显式写明）。

+ reg_expect() { # reg_expect <email> — 注册并断言 2xx
+   local code
+   code=$(reg "$1")
+   case "$code" in
+     2*) say "register $1: HTTP $code" ;;
+     *) say "FAIL: register $1 -> HTTP $code"; exit 1 ;;
+   esac
+ }
  say "== register protagonists =="
- say "a (browser): HTTP $(reg "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local")"
- say "b (publisher): HTTP $(reg "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local")"
+ reg_expect "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"
+ reg_expect "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:62-62 ───
[test · medium] draft 调用缺少与 publish 对应的 jq -e 断言。draft 失败（401/422/非 JSON 错误页）时，`curl | tee | jq |
say` 管道的退出状态取自末端的 say，set -e 不会拦截，脚本继续执行，最终只在 publish 的 receipt 检查处以 "publish receipt mismatch"
报错，误导排障。更关键的是发布 URL 硬编码 `/pro/1`，与 draft 响应的 .data.version 脱钩：若在未重置数据库的情况下重跑，本轮 draft 实际失败或生成 v2，而
publish 仍作用于遗留的 v1 草稿且 receipt 校验可通过，产出与本次 draft 无关的"有效"证据（本次运行碰巧是新库，draft 返回 v1 才掩盖了该缺口）。建议对 draft
增加 jq -e 断言，并用 draft 返回的 version 驱动 publish URL。

-     "limits":{},"charges":[]}' | tee "$EV/api-01-draft.json" | jq -c '{success, state:.data.state, key:.data.plan_key, v:.data.version}' | say "draft: $(cat)"
+ jq -e '.success == true and .data.plan_key == "pro" and .data.version == 1' "$EV/api-01-draft.json" >/dev/null \
+   || { say "FAIL: draft pro v1 rejected"; exit 1; }
+ PV=$(jq -r '.data.version' "$EV/api-01-draft.json")
+ curl -s -X POST "$BACKEND/api/v1/admin/plans/drafts/pro/$PV/publish" \


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:33-39 ───
[maintainability · low] 删除 runs/ 回退逻辑后遗留了死代码:`import glob`(L18)与 `RUNS`(L25)已无任何代码引用(原先唯一的引用是已删除的
`glob.glob(str(RUNS / ...))`)。此外其上方注释(L26-31 "Fall back to the newest runs/ TSV only when no archive
exists...")与 docstring(L14 "the runs/ observer TSV is only a fallback when no archive
exists")仍描述已被移除的回退行为,与实际逻辑矛盾。建议一并清理,避免误导后续维护者以为回退仍然存在。

- # (R3-17) The runs/ fallback is NOT decidable evidence: a runs/ TSV is a
- # whole-DB payments aggregate that cannot be keyed to THIS run, so the
- # exactly-once max_succeeded assertion would be evaluated on foreign data.
- # Degrade to MISSING-EVIDENCE AT SELECTION TIME - no runs/ probe, no
- # "using <file>" line for a file the assertions never read (the old shape
- # printed both "using <runs tsv>" and the WARNING, and the runs/ read+parse
- # below it was dead computation).
+ # 同时删除 L18 的 `import glob`、L25 的 `RUNS = ...` 定义,
+ # 并更新 L13-16 docstring 与 L26-31 注释,移除对 runs/ fallback 的描述:
+ # e.g. 注释改为:
+ # Data source is THIS directory's archived verify-db-watch-samples.tsv only;
+ # a runs/ TSV is a whole-DB aggregate that cannot be keyed to this run and
+ # is therefore never used (its absence degrades to MISSING-EVIDENCE, exit 2).


─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:52-53 ───
[bug · high] 不可见分支接受的终态值 "canceled" 永远不会出现在证据里，修复未生效。phases.py
不可见分支（payment-activation/phases.py:565-570）的 recheck 是 `_subscription_show(ctx, ext, "incomplete")`
的结果：Lago v1.53.0 按 `?status=` 过滤，charge-failure endgame 中该查询返回 404 错误体（无 "subscription" 键），因此
`recheck_subscription_status` 记录为 None；订阅 canceled 的事实只落在 `observed["cancellation_reason"] ==
"payment_failed"`（phases.py:603）。`None in ("incomplete", "canceled")` 为 False，final-audit 第 4 条(a)
所述的合法 PASS 证据仍会被此断言误判 FAIL。应接受 None（并要求 cancellation_reason 为 payment_failed），"canceled" 在此分支不可达。

      check("AC1 invoice branch: gate held across window",
-           o.get("recheck_subscription_status") in ("incomplete", "canceled"))
+           o.get("recheck_subscription_status") == "incomplete"
+           or (o.get("recheck_subscription_status") is None
+               and o.get("cancellation_reason") == "payment_failed"))


─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:57-60 ───
[bug · high] else 分支接受的终态字面量 "closed-failed" 同样不可达。phases.py 可见 failed
endgame（payment-activation/phases.py:626-639）记录的 `invoice_status` 是 "failed"（test_phases.py:957 断言
`observed["invoice_status"] == "failed"`），全仓库没有任何代码会产出 "closed-failed"。因此 final-audit 第 4 条所述的发票可见
failed endgame PASS 证据（invoice_status=="failed" 且 subscription canceled(payment_failed)）仍会让本条断言
FAIL。应改为接受 invoice_status=="failed"（可加 cancellation_reason=="payment_failed" 对齐 phases.py 的 PASS
条件）。

      check("AC1 invoice open/pending/numberless",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
-           or o.get("invoice_status") == "closed-failed")
+           or (o.get("invoice_status") == "failed"
+               and o.get("cancellation_reason") == "payment_failed"))


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:23-27 ───
[maintainability · low] 导入区两处问题：① `import sys` 全文未使用（死导入）；② 第 46 行 `urllib.error.HTTPError` 依赖
`urllib.request` 内部转引 `urllib.error` 的副作用才能解析，属 CPython 实现细节而非语言保证，建议显式导入 `urllib.error`，与仓库内 t11
`phases.py` 的用法保持一致。

  import subprocess
- import sys
  import time
+ import urllib.error
  import urllib.parse
  import urllib.request


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:46-50 ───
[bug · low] 重试循环两处缺陷：① 第 3 次（最后一次）尝试失败后仍会先 `time.sleep(4.5)` 再退出循环并 `raise SystemExit`，造成无意义的退出延迟；②
`HTTPError` 分支对错误响应体直接 `json.loads`，若网关/中间层返回非 JSON 或空 body（如 502 HTML），会抛出未处理的
`JSONDecodeError`，掩盖原始 HTTP 状态码，且后续 `intent list HTTP {status}` 的诊断信息无法输出。

          except urllib.error.HTTPError as e:
-             return e.code, json.loads(e.read().decode())
+             raw = e.read().decode("utf-8", "replace")
+             try:
+                 return e.code, json.loads(raw)
+             except ValueError:
+                 return e.code, {"error": raw[:200]}
          except OSError as e:
              last = e
+             if attempt < 2:
-             time.sleep(1.5 * (attempt + 1))
+                 time.sleep(1.5 * (attempt + 1))


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:89-91 ───
[maintainability · low] org 兜底查询失败时仅报 "organization id unavailable"，`out.returncode` 与 `out.stderr`
被静默丢弃：栈未启动（compose exec 失败）、lab.env 未 `lab.sh init`、docker 不可用等真实根因全部被掩盖，取证排查时无法定位。对比同目录
prepare_t9_env.sh 在失败路径均有明确提示。建议失败时把 rc/stderr 带入退出信息。

          args.org = (out.stdout or "").strip()
-     if not args.org:
+         if not args.org:
-         raise SystemExit("organization id unavailable")
+             raise SystemExit(
+                 "organization id unavailable "
+                 f"(rc={out.returncode}, stderr={out.stderr.strip()[:300]})")


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:44-44 ───
[maintainability · low] 移除 runs/ 回退逻辑后清理不彻底，遗留三处与死代码相关的残留：1) 第 15 行 `import glob` 的唯一调用点（`matches =
sorted(glob.glob(...))`）已随本次改动删除，导入已无任何引用；2) 第 22 行模块级变量 `RUNS` 仅在被删除的回退分支中使用，现在只定义不读取；3) 第 23-28
行注释仍描述已删除的 "Fall back to the newest runs/ TSV only when no archive exists" 回退行为，与紧随其后的 R3-17
说明（回退不可判定、选择阶段直接降级 MISSING-EVIDENCE）相互矛盾，容易让后续维护者误以为回退路径仍然存在。建议随本次改动一并删除这两个未用声明并同步修正该段注释。



─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:108-111 ───
[maintainability · low] 最终投递 POST 无任何 HTTPError 处理：Lago 返回非 2xx（签名不符 401、org id 取错 404、provider code
不匹配等）时脚本以裸 traceback 崩溃，且 Lago 响应体中的真实拒绝原因被丢弃，取证时无法区分失败根因。建议捕获 HTTPError 并将状态码与响应体带入退出信息（与 stripe()
辅助函数的错误路径对齐；配合已确认发现中的 `import urllib.error` 显式导入）。

      opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
+     try:
-     with opener.open(req, timeout=30) as r:
+         with opener.open(req, timeout=30) as r:
-         print(f"delivered event {event['id']} for intent {intent['id']} -> HTTP {r.status}")
+             print(f"delivered event {event['id']} for intent {intent['id']} -> HTTP {r.status}")
-         print(f"invoice: {intent['metadata'].get('lago_invoice_id')}")
+             print(f"invoice: {intent['metadata'].get('lago_invoice_id')}")
+     except urllib.error.HTTPError as e:
+         raise SystemExit(f"webhook delivery HTTP {e.code}: {e.read().decode(errors='replace')[:300]!r}") from e


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:71-72 ───
[maintainability · low] 列表请求非 200 时（如 STRIPE_SECRET_KEY 失效返回 401、限流 429），stripe()
已成功解析的错误响应体被丢弃，退出信息只有 `intent list HTTP 401`，Stripe 的 error.message（如 "Invalid API Key
provided"）无法帮助定位。建议把 body 中的错误消息带入 SystemExit。

      if status != 200:
-         raise SystemExit(f"intent list HTTP {status}")
+         raise SystemExit(f"intent list HTTP {status}: "
+                          + str((body.get("error") or {}).get("message", body)))


─── internal/modules/commercial/commercialplatform/config.go:114-114 ───
[maintainability · low] OutboundAllowLoopback 采用严格 `== "true"`
解析，与本仓库布尔环境变量的主流约定不一致（internal/config/config.go、internal/modules/execution/sandbox/docker_enabled.go
、internal/logger/llm_logger.go 均用 strconv.ParseBool，接受 "1"/"TRUE"/"True"）。开发者按仓库惯例设置
WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK=1 或 =TRUE 时，该开关会被静默忽略，本地 stub 验证将以难以定位的 "outbound host
not allowed" 失败。虽然失败方向是 fail-closed（安全），但建议改用 strconv.ParseBool（解析失败仍视为 false），保持一致性并消除这个易踩的坑。

- 		OutboundAllowLoopback:  getenv(EnvOutboundAllowLoopback) == "true",
+ 	// ParseBool accepts "1"/"true"/"TRUE"/"True"; anything unparseable
+ 	// stays false so the bypass fails closed.
+ 	allowLoopback, _ := strconv.ParseBool(getenv(EnvOutboundAllowLoopback))
+ 	return Config{
+ 		// ...
+ 		OutboundAllowLoopback: allowLoopback,
+ 	}


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:71-71 ───
[bug · high] "closed-failed" 是不可达的死分支，且它声称覆盖的 endgame 并未被覆盖。证据生产端
deploy/lago-lab/payment-activation/phases.py:626 将 Lago 的 invoice status 原样写入
observed.invoice_status，取值为单值枚举（"closed"/"failed"，见 phases.py:1400-1407 的 invoice_unpaid_terminal =
invoice_status in ("closed", "failed")）；发票可见的 charge-failure endgame PASS 路径（phases.py:631-658）记录的是
invoice_status == "failed"。全仓库搜索确认 "closed-failed" 复合字面量只出现在四份校验脚本和 plan 文档中，生产端从不输出。后果：该 or
分支永不为真，一份 failed-endgame 的合法 PASS 证据重放时 "AC1 invoice open/pending/numberless" 仍会假 FAIL——"终审第 4
条"的放宽在本处失效（其余三份副本存在同样问题，建议一并修正）。建议改为与生产端 phases.py:1407 对齐的 in ("failed", "closed")。

-           or o.get("invoice_status") == "closed-failed")
+           or o.get("invoice_status") in ("failed", "closed"))


─── internal/modules/commercial/payment/providers_env.go:61-69 ───
[maintainability · low] 本次改动以"map 迭代导致部分配置报错变量名不确定"为由将 alipay 必填项改为有序 slice，但同文件中结构相同的
wechatConfigFromEnv 的 required 仍是 map（L104 定义，L139 `for name, dst := range
required`），其部分配置错误同样会在不同运行间随机点名不同的缺失变量。建议对 wechat 侧做同样重构（或说明为何仅 alipay 需要确定性报错），保持两渠道部署错误行为一致。

- 	set := []struct {
- 		name string
- 		dst  *string
- 	}{
- 		{"WEKNORA_ALIPAY_APP_ID", &cfg.AppID},
- 		{"WEKNORA_ALIPAY_SELLER_ID", &cfg.SellerID},
- 		{"WEKNORA_ALIPAY_PUBLIC_KEY_PATH", &cfg.AlipayPublicKeyPath},
- 		{"WEKNORA_ALIPAY_MERCHANT_KEY_PATH", &cfg.MerchantPrivKeyPath},
- 	}
+ 	// Same deterministic-order pattern applies to required in
+ 	// wechatConfigFromEnv below, which still iterates a map.


─── internal/modules/commercial/commercialplatform/fake.go:753-756 ───
[test · low] 新增的 settle 分支未遵循 fake 既有的 FailSubmitsWith
故障注入约定:ensure_customer、ensure_subscription、grant_included_credits 都在 f.failSubmits != nil 时建模
persisted-but-response-lost(状态先应用再返回注入错误;OCR round-1 对 create_purchase_subscription 的同类遗漏已记录过该约定,见
docs/plans/issue-72-ocr-round-1.md L173)。真实适配器明确将 "late replays and outbox re-deliveries after a
lost response must not re-charge" 列为 settle 的关键属性(lago_settlement.go D2' 注释),但 fake 的 settle 完全忽略
failSubmits,导致"settle 已激活但响应丢失 → outbox 重驱动命中幂等回执"的恢复路径无法在 fake 驱动的测试中构造。建议按既有约定在应用回执与激活后返回注入错误。

  		f.receipts[cmd.Key] = receipt
  		sub.Status = "active"
  		f.purchaseSubs[payload.ExternalPurchaseSubscriptionID] = sub
+ 		if f.failSubmits != nil {
+ 			// Persisted-but-response-lost: the activation applies, the caller
+ 			// observes the injected failure (the outbox re-drive then replays
+ 			// into the receipt above — never a second settle).
+ 			return commercial.CommandReceipt{}, f.failSubmits
+ 		}
  		return receipt, nil


─── internal/modules/commercial/commercialplatform/fake.go:284-288 ───
[test · low] SetPurchaseInvoiceFees 将 InvoicePaymentStatus 无条件固定为 "succeeded",且这是该字段唯一的公开写入旋钮。真实适配器从
finalized invoice 读取 payment_status(lago_purchase.go readPurchaseInvoiceFees),可为 pending/failed;消费端
D6' 复核在 purchase_fulfillment.go L205 以 payment_status != "succeeded" 判定 Refused,该拒绝分支在 fake
驱动的服务层测试中(service 包无法访问私有字段 purchaseSubs)不可构造、无法回归。建议为 payment_status 提供独立注入旋钮,默认值保持 "succeeded"
以兼容现有用例。

- 		if s.InvoicePaymentStatus == "" {
- 			// A fee injection models the finalized stage; the D6' review
- 			// reads the invoice payment_status alongside the lines.
- 			s.InvoicePaymentStatus = "succeeded"
+ // SetPurchaseInvoicePaymentStatus overrides the finalized invoice's
+ // payment_status (SetPurchaseInvoiceFees defaults it to "succeeded") so the
+ // D6' refused path can be exercised.
+ func (f *FakeAdapter) SetPurchaseInvoicePaymentStatus(extPurchaseSubscriptionID, status string) {
+ 	f.mu.Lock()
+ 	defer f.mu.Unlock()
+ 	if s, ok := f.purchaseSubs[extPurchaseSubscriptionID]; ok {
+ 		s.InvoicePaymentStatus = status
+ 		f.purchaseSubs[extPurchaseSubscriptionID] = s
+ 	}
- 		}
+ }


─── internal/modules/commercial/commercialplatform/fake.go:740-744 ───
[test · low] settle 终态检查在此处接受 "terminated",但 fake 的 ReadSnapshot 状态投影只映射
incomplete/active/canceled,"terminated" 落入 default 分支并返回 "unknown purchase status" 错误;而真实适配器将
canceled/terminated 一并映射为 PurchaseStateCanceled(lago_purchase.go L540-541)。同包测试若将订单置为 terminated 验证
settle 拒绝,随后的快照读取会得到 ErrPlatformInvalidResponse 而非 PurchaseStateCanceled,与真实契约不对齐。建议在 ReadSnapshot 的
switch 中同步补充 terminated 映射。

- 		if sub.Status == "canceled" || sub.Status == "terminated" {
- 			// A terminal purchase can never be settled (the authority's own
- 			// truth refuses; the Lago rails would fail the same way).
- 			return commercial.CommandReceipt{}, fmt.Errorf("%w: settle target purchase is terminal", commercial.ErrPlatformInvalidResponse)
- 		}
+ // ReadSnapshot 侧同步对齐真实适配器(可置于 ReadSnapshot 的 switch 中):
+ 		case "canceled", "terminated":
+ 			p.State = commercial.PurchaseStateCanceled


─── internal/modules/commercial/commercialplatform/lago_settlement.go:124-127 ───
[bug · high] 选中意图后直至 off-session confirm，始终未将 intent 的金额/发票归属与命令载荷做任何比对：stripeIntent 只解析
id/status/created/metadata，payload.AmountFen（冻结报价，渠道实收金额）从未参与校验。latestIntent 仅凭 created
最新这一启发式（注释自己也承认是启发式证据）就决定对哪个 PaymentIntent 发起真实扣款；同一 provider customer 名下若出现另一个更新的、携带
lago_invoice_id 的未结算意图（如非 0 金额的续费/加购/按比例拆分发票生成的 intent），会把客户的 settle 扣款打到错误发票上——这正是文件注释反复强调要避免的
"never a blind charge"。清单响应中本就包含 amount 字段，建议在 latestIntent 选定后（或筛选时）要求 intent.amount ==
payload.AmountFen，不匹配即 fail closed（ErrPlatformInvalidResponse），为资金边界增加一道廉价且可判定的归属校验。

  	intent, err := latestIntent(intents)
  	if err != nil {
  		return commercial.CommandReceipt{}, err
+ 	}
+ 	if intent.AmountFen != payload.AmountFen {
+ 		return commercial.CommandReceipt{}, fmt.Errorf(
+ 			"%w: gating intent %s amount %d does not match the frozen purchase price %d",
+ 			commercial.ErrPlatformInvalidResponse, intent.ID, intent.AmountFen, payload.AmountFen)
  	}


─── internal/modules/commercial/commercialplatform/lago_settlement.go:212-213 ───
[bug · medium] limit=20 且不处理 has_more/starting_after 分页：该列表同时承担两个正确性职责——未结算候选定位与已 succeeded
发票的幂等证据窗口。当 customer 名下意图超过 20 条（如多次失败尝试留下的兄弟意图、历史发票意图累积）时，窗口被静默截断：(a) gating intent 已 succeeded
但被挤出前 20 时，settledInvoices 为空、intents 也可能为空 → ErrPlatformInvalidResponse；(b) gating intent 未结算但不在前
20 时，若恰有其他已结算意图则直接返回 receipt 幂等空转，fulfiller 永远观察不到 active 直至预算耗尽。两种路径上游都终结为 attention
搁置（purchase_fulfillment.go:165-171 只重试 Unreachable/Unconfigured）。正确性关键读不应可被静默截断，建议循环消费
has_more/starting_after，或至少提高 limit 并在 has_more=true 时显式处理。

  	status, body, err := a.providerOutboundRequest(ctx, http.MethodGet,
- 		"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=20", "", "")
+ 		"/v1/payment_intents?customer="+url.QueryEscape(providerCustomerID)+"&limit=100", "", "")
+ 	// 消费 has_more/starting_after，直至读全；截断视为 ErrPlatformUnreachable 级可重试或显式失败


─── internal/modules/commercial/commercialplatform/lago_settlement.go:192-193 ───
[bug · medium] 预算与自身注释矛盾且覆盖不了实际调用链：注释称 "five outbound calls"，但配额只有
4×outboundProviderTimeout=60s。实际链路为 5 次 Stripe 往返（list、attach、default、intent update、confirm，每次上限
15s，最坏 75s）外加 3 次 Lago 读（快照、customerProviderBound、boundProviderCustomerID）。响应持续偏慢时 deadline 会在
confirm 段耗尽，transport 错误被包装为 ErrPlatformUnreachable 触发整段重驱，并留下 default payment method 已被改写、pm 已
attach
的半完成副作用。这与包内已有的预算纪律不一致——purchaseRequestTimeout（lago_purchase.go:47-49）按实际链路逐项求和（R1-12）。建议按同纪律核算，如
outboundProviderTimeout*5 加 Lago 读预算；同时建议给 attach/default 两个 POST 也派生确定性 Idempotency-Key（如
cmd.Key+":attach"/":default"），与同函数 update/confirm 的键策略保持一致。

- // settleRequestTimeout bounds the whole settle drive (five outbound calls).
- const settleRequestTimeout = outboundProviderTimeout * 4
+ // settleRequestTimeout bounds the whole settle drive: five provider round
+ // trips (list, attach, default, update, confirm) plus the Lago reads
+ // (snapshot, binding×2), mirroring purchaseRequestTimeout's sum discipline.
+ const settleRequestTimeout = 5*outboundProviderTimeout + subscriptionRequestTimeout


─── internal/modules/commercial/commercialplatform/lago_settlement.go:357-358 ───
[bug · medium] 忽略 io.ReadAll 的错误会把传输层故障误分类为权威侧数据异常：响应头已送达 2xx 但 body 中途被截断（连接复位）时，ReadAll 返回错误被丢弃，截断
JSON 随后在调用方 unmarshal 失败 → "malformed" →
ErrPlatformInvalidResponse。上游（purchase_fulfillment.go:165-171）对该哨兵是终结性的——直接
markActivationState(Attention) 搁置订单，而本应按瞬态网络故障走 Unreachable 重试。另外 64KiB 的 LimitReader 截断同样只表现为
malformed，无法与真实坏响应区分。建议 Readall 出错时返回 %w: ErrPlatformUnreachable（transport 类），与函数内 Do
失败的归类对齐；超限截断也应显式报错而非静默交给 JSON 解析。

- 	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
+ 	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
+ 	if err != nil {
+ 		return 0, nil, fmt.Errorf("%w: outbound provider body read failed", commercial.ErrPlatformUnreachable)
+ 	}
  	return resp.StatusCode, data, nil


─── internal/modules/commercial/commercialplatform/lago_settlement.go:99-102 ───
[maintainability · low] 这里与紧邻的 customerProviderBound 对同一端点 /api/v1/customers/{externalCustomerID}
发起两次完全相同的 GET：customerProviderBound（lago_purchase.go:494-505）本已解析出 provider_customer_id
却只返回布尔并丢弃，随后本函数再读一次。既在收费关键路径上多付出一次往返（该路径预算本就紧张），两次读取之间绑定变化还会引入 TOCTOU 不一致。建议合并为一次读取同时返回 (bound,
providerCustomerID)，或让 customerProviderBound 直接返回 id。

- 	providerCustomerID, err := a.boundProviderCustomerID(ctx, payload.ExternalCustomerID)
+ 	binding, err := a.customerProviderBinding(ctx, payload.ExternalCustomerID) // 返回 exists/bound/providerCustomerID
  	if err != nil {
  		return commercial.CommandReceipt{}, err
+ 	}
+ 	if !binding.bound {
+ 		return commercial.CommandReceipt{}, fmt.Errorf(
+ 			"%w: settle target customer carries no provider binding", commercial.ErrPlatformInvalidResponse)
  	}
+ 	providerCustomerID := binding.providerCustomerID


─── internal/modules/commercial/repository/commercial/order.go:156-159 ───
[bug · medium] 冲突翻译层漏掉了 quote 面：并发用同一 quote 双结账时（双方都通过 GetOrderByQuote 预检与 createOrderTx 的 quote_id
First 预检，随后一方提交），败者在 tx.Create 处撞 quote_id 唯一索引，拿到的是裸驱动错误——既不是 pre-check 路径返回的
ErrQuoteAlreadyUsed，也不匹配 isPendingPurchaseConflict，最终原样上抛。而 purchase.go L251
专门为竞态败者准备的重放分支（errors.Is(err, ErrQuoteAlreadyUsed)）在这一最需要它的 insert 竞态下永远无法触发，客户端收到未映射的内部错误（通常
500）而非幂等重放胜者订单。既然本次已在此处建立冲突分派，建议顺带把 quote 唯一冲突也翻译为 ErrQuoteAlreadyUsed（同包 catalog.go 已定义，handler
L503/806/899 已有映射）。

  		if row.Kind == domain.OrderKindPurchase && isPendingPurchaseConflict(err) {
- 			return ErrPurchasePendingExists
+ 			return fmt.Errorf("%w: %w", ErrPurchasePendingExists, err)
+ 		}
+ 		if isQuoteUniqueConflict(err) {
+ 			// The insert lost the concurrent quote race (both pre-checks
+ 			// passed before the winner committed) — same replay shape as
+ 			// the pre-check hit above.
+ 			return ErrQuoteAlreadyUsed
  		}
  		return err


─── internal/modules/commercial/repository/commercial/order.go:354-360 ───
[bug · medium] 无价面重放在"跨 purchase 遗留单"场景下不成立：注释断言冲突胜者"consumed a quote of the SAME purchase, hence
the same frozen face"，但索引是租户级的——前一 purchase 终止（取消）后遗留的可付 pending 单（项目终报明确承认"带 checkout_url 的废弃
pending 单无超时回收"，订单无 closed/过期迁移，属已知残留）同样占用该槽位。此时新 purchase（异价/异 plan，L228 仅要求
awaiting）结账：CurrentPendingPurchaseOrder（带价面）未命中 → CreateOrder 撞租户索引 → 本函数返回旧价旧 plan 的订单，purchase.go
L268-271 不做任何金额比对即原样重放其 checkout_url。客户会被导向一张旧价渠道单，支付后 ConfirmPayment 按旧订单冻结报价确认/发放，当前 purchase 仍滞留
awaiting。建议调用方在把该行作为重放答案返回前校验 AmountFen/Currency 与当前 purchase 冻结面一致，不一致时不作重放（走清扫或冲突错误），或在本查询保留价面输入。



─── internal/modules/commercial/repository/commercial/order.go:171-175 ───
[maintainability · low] 两个健壮性缺口：(1) 返回 ErrPurchasePendingExists 时原始驱动错误被完全丢弃，线上排障拿不到 SQLSTATE
23505/约束名/冲突细节——建议以 fmt.Errorf("%w: %w", ErrPurchasePendingExists, err) 双包裹（Go 1.20+ 多 %w，调用方
errors.Is 不受影响）；(2) 第二分支用 "UNIQUE constraint failed"+"tenant_id" 做文本匹配：SQLite 的该格式错误只含列名，未来若
commercial_orders 新增任何含 tenant_id 列的其它唯一索引（如复合唯一），其违例会被误判为 pending
冲突而触发错误的重放分支。建议优先按驱动错误码/约束名精确判定（pg 23505+索引名、sqlite 2067），文本 Contains 仅作兜底。

  	msg := err.Error()
  	if strings.Contains(msg, "uq_purchase_pending_per_tenant") {
  		return true
  	}
+ 	// TODO: narrow to the driver error code/constraint name (pg SQLSTATE
+ 	// 23505 + index name; sqlite 2067) so a future unique index touching
+ 	// tenant_id cannot be misclassified as a pending-purchase conflict.
  	return strings.Contains(msg, "UNIQUE constraint failed") && strings.Contains(msg, "tenant_id")


─── internal/modules/commercial/service/commercial/benefits.go:572-576 ───
[bug · high] 对本地未知 code 一律物化为 true 的路径存在越权窗口。证据链：真实 Lago 适配器的
readCustomerFeatures（lago.go:1089-1140）对 base("-sub") 与 purchase("-purchase") 两个订阅的 entitlements
取并集，entitlement 存在即置 true，且不过滤订阅状态（仅 404 跳过）；而 purchase gated subscription 在下单时即以 incomplete 状态创建（映射
AwaitingPayment，见 benefits_test.go:322-334 的 create→settle 两步），canceled/terminated 映射 Canceled 后
authority 侧 entitlement 也有懒清理窗口。因此当付费 plan 定义中存在 base 定义没有的 feature code（如 team_seats）时：处于
awaiting_payment（下单未付款）或 canceled 残留窗口的租户，b.Features 会携带该 code，`_, known := out[code]; !known` 判定为未知
→ 置 true，未付款/已取消购买者获得付费特性，弱化了取消/退款后的权益回收。注意 fake.go:398-404 的 b.Features 只含 base features
且携带真实布尔值，现有单测（TestBenefitsFeaturesPurchaseFaceORsPurchasePlanDefinition）无法暴露此路径。建议：物化仅限 base 订阅 leg
的 entitlements，或当 purchase 快照非 ACTIVE 时先从 entitled 中剔除其 plan 定义已知的 codes。

+ 	if perr == nil && psnap.Purchase != nil && psnap.Purchase.PlanCode != "" &&
+ 		psnap.Purchase.State != domain.PurchaseStateActive {
+ 		// 非 ACTIVE purchase 的 entitlement 残留不得物化：awaiting_payment /
+ 		// canceled 懒清理窗口内其 plan 独有 code 会越权发放付费特性。
+ 		if pub, pubErr := s.plans.FindPublicationByCode(ctx, psnap.Purchase.PlanCode); pubErr == nil {
+ 			if def, defErr := s.definitionOf(ctx, pub.PlanKey, pub.Version); defErr == nil {
+ 				for code := range def.Features {
+ 					delete(entitled, code)
+ 				}
+ 			}
+ 		}
+ 	}
  	for code := range entitled {
  		if _, known := out[code]; !known {
- 			out[code] = true // authority materialization of a code no definition knows
+ 			out[code] = true
  		}
  	}


─── internal/modules/commercial/service/commercial/benefits.go:556-559 ───
[maintainability · medium] effectiveFeatures 连续吞掉 ReadSnapshot、FindPublicationByCode、definitionOf
三处错误且无任何日志/指标。best-effort 降级意图虽已注释文档化，但后果是：ACTIVE 付费租户在 purchase 快照瞬时读取失败（unreachable/超时）或
publication 查询瞬时 DB 故障时，特性面静默降级为 base（advanced_models 判 false），付费功能间歇性被拒且完全不可观测、无法排查。同 PR 的
purchase_fulfillment.go 对同类降级已使用 logger.Warnf（markActivationState），建议此处至少对失败 leg 记一条 Warn（含
tenantID），保持模块内可观测性惯例一致。

- 	if psnap, err := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
+ 	psnap, perr := s.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
  		Kind: domain.SnapshotKindPurchase, TenantID: tenantID,
- 	}); err == nil && psnap.Purchase != nil &&
+ 	})
+ 	if perr != nil {
+ 		// fail-open by design, but observable: 付费面降级必须可排查
+ 		logger.Warnf(ctx, "[Benefits] purchase snapshot read failed, feature face degrades to base (tenant %d): %v", tenantID, perr)
+ 	} else if psnap.Purchase != nil &&
  		psnap.Purchase.State == domain.PurchaseStateActive && psnap.Purchase.PlanCode != "" {


─── internal/modules/commercial/service/commercial/benefits.go:503-508 ───
[maintainability · low] 跨 grant family 按 Period 求和后，视图以单一 ExpiresAt 门控整个 Period 的总和（registry 主循环用
base 行的本地到期，missing 分支用快照内该 Period
的最大到期）。当前正确性隐式依赖一个不在本文件强化的外部约定：EnsureMonthlyCredits（benefits.go:360）与 PurchaseFulfiller.Fulfill
第⑤步（purchase_fulfillment.go）对同一 Period 都写入 PeriodEnd(period)，即同 Period 两 family 到期严格一致。一旦将来 purchase
首期有效期规则分叉（如购买赠送更长有效期），或 Lago 侧 wallet ExpirationAt 异常（readBenefitsSnapshot 解析失败会得到 zero time，missing
分支清零购买余额），先到期批次的余额就会搭上更晚的到期继续可花——弱化 Credits 到期规则（需求红线），反向则购买余额被 base
行先过期连带清零。建议至少在此处注释锁定该不变量，并在分叉发生时改为按 wallet family 元数据（WalletMetaPeriod vs
WalletMetaPurchasePeriod）分别聚合、各自按自身到期门控。

+ 	// NOTE（跨 family 到期不变量）: 按 Period 求和后以单一 ExpiresAt 门控总和，
+ 	// 正确性依赖 base 与 purchase 两 family 对同一 Period 都写入
+ 	// PeriodEnd(period)（EnsureMonthlyCredits / PurchaseFulfiller.Fulfill ⑤）。
+ 	// 若任一 family 的有效期规则分叉，需改为按 wallet family 元数据
+ 	// （WalletMetaPeriod vs WalletMetaPurchasePeriod）分别聚合、各自按自身
+ 	// 到期门控，否则先到期批次余额会搭更晚的到期继续可花。
  	for _, batch := range b.Batches {
  		balances[batch.Period] += batch.BalanceMicro
  		if batch.ExpiresAt.After(expires[batch.Period]) {
  			expires[batch.Period] = batch.ExpiresAt
  		}
  	}


─── internal/modules/commercial/commercialplatform/lago_purchase.go:643-648 ───
[bug · high] 「两张及以上含购买订阅费的终态发票 = 数据异常」这一假设与权威自身的续期行为冲突：购买订阅是按月 recurring
订阅（billing_time=calendar，billing period 见 flow evidence），且全模块无任何取消/终止命令——订阅永远保持 active。Lago v1.53
的月度 billing job 会在每个周期边界为该订阅出具 renewal invoice（fee.external_subscription_id
相同、item.type=subscription，实验室证据 t02-duplicates 已实证 renewal invoice 会被出具），默认 grace=0 下即刻 finalized
并进入本索引。因此首次成功付款约一个月后 matches 必然 ≥2，readPurchaseSnapshot 从此永久返回
invalid_response：benefits.effectiveFeatures（benefits.go:556-559）会静默把付费租户降回 base
特性、PurchaseStatus/purchase 重放持续失败（pendingView/503）。建议：以索引 newest-first 语义取第一条匹配（或按 activation
时间窗/payment_status 判别 gating 发票、或在履约时持久化 gating invoice lago_id 供后续直读），把 >1 视为正常续期而非异常。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:607-609 ───
[bug · medium] 此处（及 finalizedInvoiceIDs 中同型的 invoice index 分支）把 429/5xx 等瞬时错误一律归类为终态
ErrPlatformInvalidResponse，与本文件同批修改在
customerProviderBound/waitForPaymentMethodSync/ensureProviderBinding 中新增的 `status ==
http.StatusTooManyRequests || status >= 500 → ErrPlatformUnreachable` 分类相矛盾。后果：settlePurchasePayment
第 (i) 步（lago_settlement.go:69）对 active 购买的幂等重放读取会因 Lago 瞬时 502/429 返回 invalid_response，服务层 Fulfill 第
② 步（purchase_fulfillment.go:168-171）将其视为 definitive 而落 attention 标记（而非保持 pending 等待重试），同时
PurchaseStatus 向客户端给出错误的 invalid_response 闭环 reason。建议与文件内其它读路径一致：429/5xx →
ErrPlatformUnreachable（可重试），其余非 200 才 invalid_response。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:599-603 ───
[performance · medium] 对每个 active 购买租户，每次快照读取都要执行「索引分页（≤10 页）+ 对每张终态发票逐张串行 GET（≤1000 张，包含每月累积的 Base
计划 0 元发票——本函数注释自证索引会列出它们）」的 N+1 遍历，且全部挤在 readPurchaseSnapshot 的 subscriptionRequestTimeout=15s
预算内。该读取位于热点路径：benefits.effectiveFeatures 每次 benefits 刷新（benefits.go:456-459）、PurchaseStatus
每次轮询都会触发。历史按月线性增长，延迟随之线性上升，最终必然超时——届时 benefits 面对付费租户静默退回 base 特性（err==nil 判断吞掉失败），PurchaseStatus
开始报错。建议至少：首次成功识别后缓存/持久化 gating invoice 的 lago_id（此后直读单张），或利用 newest-first
顺序在首个匹配处提前结束遍历，或将该遍历移出同步读路径。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:80-80 ───
[security · medium] 环回豁免由环境变量 WEKNORA_COMMERCIAL_OUTBOUND_ALLOW_LOOPBACK
驱动（config.go:114，仅靠注释约定「production must never set it」），且 lago.go:287 与 lago_settlement.go:334
两处权威/Stripe 出站都消费它。S1 验收条件要求「请求前校验并拒绝 localhost、环回……」——生产环境一份从 dev 复制的 env（该开关=true）即可让 SSRF 防线对
loopback 打开（直连 WeKnora 宿主机上的本地服务），无任何环境断言拦截。建议为豁免增加非生产守卫（如启动时校验 provider/Release 指明生产则拒绝该组合并 fail
fast），或仅允许在显式 dev/test 构建姿态下生效。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:665-666 ───
[maintainability · low] 查询串中的 external_customer_id 使用 url.PathEscape 拼接：PathEscape 不转义 '&'、'+'、'#'
等查询定界字符，一旦 ID 格式变化即产生参数注入/参数丢失。当前 ExternalCustomerID 恒为 "weknora-tenant-<n>"（安全字符集）故不可达，但作为模式应改用
url.Values{}.Encode() 或 url.QueryEscape，避免后续复制该写法到可变值上。



─── internal/modules/commercial/commercialplatform/lago_purchase.go:562-563 ───
[maintainability · low] Lago 原始 payment_status 字符串（"pending"/"succeeded"…）被原样写入 seam 的
PurchaseSnapshot.InvoicePaymentStatus，而同一读取中 InvoiceLineSnapshot.Kind 特意声明 "the closed port word,
never the provider's raw type"——两者口径不一致，且消费方（purchase_fulfillment.go:205）已硬编码 provider 词汇
"succeeded"。虽然该字段未进入客户端契约（PurchaseView 不含它），但 purchase_command.go 属 seam 冻结面，建议在 seam 内映射为闭合令牌（如
succeeded/其他），保持 provider-neutral 原则一致。



─── internal/modules/commercial/service/commercial/order.go:296-296 ───
[maintainability · low] checkout_link_degraded 标记未形成端到端闭环：handler 的 orderWire 会把它挂到 wire 上，但共享契约
packages/contracts/src/commercial.ts 的 OrderView 未声明该字段，parseOrderView
重建对象时会将其丢弃，apps/web（CheckoutPage/BillingPage）中也无任何消费方。R2-27 注释宣称的语义（客户端可区分降级成功——链接不会在 replay
中重发，应提示用户保存）因此完全无法传达给前端，该字段目前是死信号。建议在本批同步给 contracts 的 OrderView 增加可选字段并在 parseOrderView
中透传（或在注释中明确其为服务端遥测专用、客户端不应依赖），否则前后端对该标记的语义理解会随时间漂移。



─── internal/modules/commercial/service/commercial/order.go:448-450 ───
[maintainability · low] 新增的降级/清扫失败日志使用 stdlib log.Printf，而整个 commercial
模块（fulfillment.go、purchase.go、purchase_fulfillment.go，以及本文件所属包）统一使用项目结构化 logger（logger.Warnf，可携带
ctx）。stdlib 日志会绕过统一日志管线（级别/格式/采集），且这是 commercial 模块中唯一的 "log" 导入。建议改用 logger.Warnf 保持一致。

  		if ferr := s.orders.MarkChannelFailed(persistCtx, id); ferr != nil {
- 			log.Printf("commercial: channel-failed mark lost for order %s: %v", id, ferr)
+ 			logger.Warnf(persistCtx, "[CommercialOrder] channel-failed mark lost for order %s: %v", id, ferr)
  		}


─── internal/modules/commercial/commercialplatform/fake.go:470-470 ───
[maintainability · low] fakeWalletPeriod 将购买钱包名前缀硬编码为 extCustomer+"-purchase-",本地复制了
ExternalPurchaseSubscriptionID 的身份派生格式(="weknora-tenant-<id>-purchase")。对照 lago.go
readBenefitsSnapshot 的确定性名称回退(L1052-1055),真实适配器用的是规范函数派生的前缀
commercial.ExternalPurchaseSubscriptionID(tenantID)+"-"。platform.go 对确定性身份的契约是"never a local
copy":一旦购买身份格式演进,fake 会静默不再识别购买钱包——benefits 快照中购买批次悄悄消失而 Lago
适配器仍识别,形成只有单一适配器具备的行为(正是本文件头部声明为缺陷的那类漂移),且 fake 驱动的服务层测试不会暴露。建议与真实适配器一致,由 tenantID 经规范函数派生购买前缀。

- for _, prefix := range []string{extCustomer + "-purchase-", extCustomer + "-"} {
+ // 调用侧(benefits 分支)改传 query.TenantID:
+ //   if period, ok := fakeWalletPeriod(query.TenantID, extCustomer, w.Name); ok {
+ func fakeWalletPeriod(tenantID uint64, extCustomer, name string) (string, bool) {
+ 	for _, prefix := range []string{commercial.ExternalPurchaseSubscriptionID(tenantID) + "-", extCustomer + "-"} {
+ 		suffix, ok := strings.CutPrefix(name, prefix)
+ 		if !ok || len(suffix) != 7 {
+ 			continue
+ 		}
+ 		if _, err := commercial.PeriodEnd(suffix); err != nil {
+ 			continue
+ 		}
+ 		return suffix, true
+ 	}
+ 	return "", false
+ }


─── internal/modules/commercial/service/commercial/purchase.go:345-350 ───
[bug · medium] 计划一致性校验存在两个对立的缺陷,均源于 wantPlanKey 的失败默认值 "" 可与 quoteBoughtPlan 的失败默认值 "" 相等:

1) 假匹配:当 FindPublicationByCode 出错(如本地 publications 表缺失该 plan_code——迁移/重置环境下的真实形态,或 DB 瞬时错误)时
wantPlanKey="";若订单 quote 又恰好不可读,quoteBoughtPlan 也返回 "",""=="" 成立,历史异计划同价订单会被投影,甚至被合成
paid_awaiting_activation——恰好击穿 quoteBoughtPlan 注释声称的 "an unmatchable value, so the caller never
projects the order" 不变量。
2) 单侧失败丢投影:仅计划查询失败时 wantPlanKey="" ≠ 订单 quote 的真实 PlanKey,当前合法订单(含 CheckoutURL 付款入口)从 GET
/commercial/purchase 中被静默丢弃,paid_awaiting_activation 合成也随之丢失;若 publication 持续缺失,该状态视图永久降级。

另外此处对 FindPublicationByCode 的第二次调用与第 329 行完全重复(同方法内同一查询执行两次)。

建议:复用第一次查询的结果,并在查询失败时直接跳过订单投影(双侧"不可证明即不投影")。

- 		wantPlanKey := ""
- 		if pub, err := s.plans.FindPublicationByCode(ctx, p.PlanCode); err == nil {
- 			wantPlanKey = pub.PlanKey
+ 	pub, pubErr := s.plans.FindPublicationByCode(ctx, p.PlanCode)
+ 	if pubErr == nil {
+ 		out.PlanKey = pub.PlanKey
+ 		out.PlanVersion = pub.Version
- 		}
+ 	}
+ 	// ...
+ 	if p.State != domain.PurchaseStateAbsent && pubErr == nil {
  		if row, err := s.orders.orders.CurrentPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); err == nil &&
- 			quoteBoughtPlan(ctx, s.orders, tenantID, row.QuoteID) == wantPlanKey {
+ 			quoteBoughtPlan(ctx, s.orders, tenantID, row.QuoteID) == pub.PlanKey {


─── internal/modules/commercial/service/commercial/purchase.go:244-249 ───
[bug · medium] 重放分支缺少计划归属校验,与本 diff 给 PurchaseStatus(audit 9)加的防护不一致。CurrentPendingPurchaseOrder 仅按
tenant+amount+currency+pending+payable 过滤(repository order.go:331),不校验既有订单的 quote 购买的计划;注释的论证("the
match gate above already proved the quote buys the same plan at the same frozen face, so the
replayed order IS this purchase's order")是无效推理——match gate 证明的是"当前提交的 quote"购买计划 P,而不是"被重放的既有订单的
quote"购买计划 P。

可达路径:旧版 POST /commercial/orders(handler commercial.go:772 →
OrderService.CreateOrder)不经任何权威购买门禁即可为任意已发布计划的 quote 开出 kind=purchase 的 pending 订单。租户先用旧路径为计划 Y
开单(payable pending),再走 Purchase 流程购买同价位面的计划 X(权威侧 absent → 成功创建 awaiting X,match gate 通过),此处即会把 Y 的
CheckoutURL 原样返回;用户付款后回调按 Y 的 quote 结算,与权威侧 awaiting 的 X 订单错位。audit 9 的注释本身已承认"同价位面异计划订单在 republish
窗口真实存在",却只给状态路径加了校验。

注意 ErrPurchasePendingExists 分支(line 268 的
CurrentPayablePendingOrder)同样无计划校验,修复需两处一致:计划不匹配时不要重放,宁可返回冲突错误(绝不返回错误订单的付款链接)。

- 	if existing, perr := s.orders.orders.CurrentPendingPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); perr == nil {
+ 	if existing, perr := s.orders.orders.CurrentPendingPurchaseOrder(ctx, tenantID, p.AmountFen, p.Currency); perr == nil &&
+ 		quoteBoughtPlan(ctx, s.orders, tenantID, existing.QuoteID) == snap.PlanKey {
  		ov := orderViewFromRow(existing)
  		return s.purchaseView(p, snap, pub, &ov), nil
- 	} else if !errors.Is(perr, repocommercial.ErrOrderNotFound) {
+ 	} else if perr != nil && !errors.Is(perr, repocommercial.ErrOrderNotFound) {
  		return PurchaseView{}, perr
  	}


─── internal/modules/commercial/service/commercial/purchase.go:285-288 ───
[bug · low] 清扫后的单次重试直接透传 rerr,未复用上方 ErrQuoteAlreadyUsed 的优雅重放分支:重试若再次撞唯一索引(阻塞槽位是无链接且不足 15 分钟的残留——清扫按
SweepStaleAge 时间阈值是 no-op),客户端拿到裸的 ErrPurchasePendingExists;而 Purchase handler 的错误
switch(commercial.go:452-517)没有该 sentinel 的分支,落入 default 变成 500 "purchase failed"。同理,line 274 与 line
283 两处 return PurchaseView{}, err 丢弃了 gerr/serr,排障时只剩第一次的冲突错误,丢失真实原因。建议用 %w 保留原始冲突身份的同时附带次级错误(如
fmt.Errorf("%w: sweep retry: %v", err, rerr)),handler 侧可考虑补充 409 分支对齐 POST /orders 的 R3-25 处理。

  		ov2, rerr := s.orders.CreateOrder(ctx, tenantID, quoteID, providerName)
  		if rerr != nil {
- 			return PurchaseView{}, rerr
+ 			return PurchaseView{}, fmt.Errorf("%w: pending-slot retry: %v", err, rerr)
  		}


─── internal/modules/commercial/service/commercial/purchase.go:185-185 ───
[bug · low] 冲突判定放宽到 State != Absent 后,canceled/terminated 的终态持有也会命中(readPurchaseSnapshot 将 Lago
canceled/terminated 映射为 PurchaseStateCanceled 而非 Absent,且 createPurchaseSubscription 对异计划一律返回
invalid response)。后果:租户取消计划 Y 后购买不同计划 X 将确定性得到 409 purchase_plan_conflict,而该错误的契约语义是"the first
purchase wins, the late caller re-quotes"——重新报价对终态残留永不生效,客户端进入无解死路。注释只论证了 ACTIVE 与 awaiting
两种形态。若终态残留下的重新购买确实要留到 #84/#92 的生命周期清理,建议至少在注释中明确覆盖 Canceled,或暂将终态排除出 409 判定以免误导客户端重试/重报价。

  			held.Purchase.State != domain.PurchaseStateAbsent &&
+ 			held.Purchase.State != domain.PurchaseStateCanceled &&


─── internal/modules/commercial/service/commercial/purchase.go:300-306 ───
[documentation · low] purchaseAudit 的声明连同其注释被插在了 PurchaseStatus 的文档注释与函数声明之间,且两段注释连续无空行,构成同一个 doc
comment 块:按 Go 文档关联规则,整块注释(以 "PurchaseStatus answers the purchase projection..." 开头)都会挂到 var
purchaseAudit 上,PurchaseStatus 本身反而失去文档,godoc/IDE 悬浮提示也会错位。建议把 purchaseAudit(带自己的注释)整体移到
PurchaseStatus 文档注释之前,恢复两段注释各自归属。

  // purchaseAudit is the attention-audit sink (OCR final audit 10, minimal
  // face): a purchase attempt that leaves the authority holding an
  // awaiting/terminal subscription it can no longer settle writes ONE
  // closed-token attention line (log by default; tests capture). The
  // lifecycle face (auto-cancel/terminate) belongs to #84/#92 and is
  // explicitly NOT taken here.
  var purchaseAudit = func(tenantID uint64, quoteID, token string) {
+ 	logger.Warnf(context.Background(),
+ 		"[CommercialPurchase] attention: tenant=%d quote=%s token=%s", tenantID, quoteID, token)
+ }
+ 
+ // PurchaseStatus answers the purchase projection for one tenant: the
+ // authority truth (closed state + frozen price face) plus the local order
+ // when one exists.
+ func (s *PurchaseService) PurchaseStatus(ctx context.Context, tenantID uint64) (PurchaseView, error) {


─── internal/modules/commercial/repository/commercial/order.go:298-307 ───
[bug · medium] CurrentPurchaseOrder 的 pending 优先回路未排除 channel_failed 死单。本次变更引入 R2-28 之后，"pending"
不再等价于活的支付入口：channel Create 失败（或空链接）的订单停留 pending 但被明确定义为不可付、不可回放。同价面下死单会遮蔽真正的已支付订单——场景：checkout
渠道失败留下死单 A（pending, channel_failed=true，较早），客户端换新 quote 重开订单 B（较晚）并经回调支付成功；created_at DESC 排序后循环先见
B（非 pending，跳过）再命中死单 A 并返回。调用方 PurchaseStatus（purchase.go L349-360）会把这张永久 pending
的死单挂为当前购买订单投影给用户，且恰好错过 p.State==awaiting && row.State==paid 的 paid_awaiting_activation
合成窗口——购买已成交却仍显示待付款。建议 pending 偏好与 CurrentPendingPurchaseOrder 的可付语义对齐，排除 channel_failed 行（终态回退
rows[0] 仍保留）。

- 		Where("tenant_id = ? AND kind = ? AND amount_fen = ? AND currency = ?",
- 			tenantID, domain.OrderKindPurchase, amountFen, currency).
- 		Order("created_at DESC, id ASC").Find(&rows).Error; err != nil {
- 		return OrderRow{}, err
- 	}
  	for _, row := range rows {
- 		if row.State == domain.OrderStatePending {
+ 		// (R2-28) channel-failed 死单不是支付入口，不得遮蔽更新的已付订单。
+ 		if row.State == domain.OrderStatePending && !row.ChannelFailed {
  			return row, nil
  		}
  	}


─── internal/modules/commercial/repository/commercial/order.go:135-139 ───
[bug · medium] created_at 的时间基不一致 + SQLite 文本词法比较导致清扫门与排序失真。store 默认值、SweepStaleLinklessPending 的
staleBefore、迁移回填全部使用 UTC，但生产写入路径传入的 cmd.Now 是 time.Now()（本地时区，service/commercial/order.go
L405），而本处按"不覆盖调用方值"原样落库。gorm.io/driver/sqlite（mattn）以 "2006-01-02 15:04:05.999999999-07:00" 文本存储
time.Time 并按词法比较/排序：在非 UTC 时区服务器（或 DST 切换、迁移回填的 UTC 行与新写入的本地行混存）下，SweepStaleLinklessPending 的
created_at < ? 年龄门和三处 created_at DESC 排序都会失真——正偏移时区（如 +08:00）下本地渲染恒大于 UTC 边界，15 分钟门实际退化为跨日才生效，冲突路径
sweep+retry-once 对当日残留形同虚设；负偏移时区下刚创建、链接尚在落库的新单反而可能立即被清扫，直接违反"不得清扫自己正在 checkout 的单"的 (OCR r4)
保证。仓库其它模块已有明确规避此问题的先例（internal/application/repository/agent_run.go nowSQL 注释："without relying on
lexical timezone ordering"，用 julianday 比较）。建议在 createOrderTx 统一归一化时区（不改变时刻，仅统一渲染），或在 SQLite 方言下用
julianday 比较。

  	// (R1-20) creation timestamp: the deterministic recency anchor. Tests
- 	// may pre-set it; the store never overwrites a caller-provided value.
+ 	// may pre-set it; the store never overwrites a caller-provided INSTANT
+ 	// but normalizes its rendering to UTC — SQLite stores datetimes as text
+ 	// and orders them lexically, so mixed zone offsets corrupt both
+ 	// created_at DESC ordering and the sweep's age gate.
  	if row.CreatedAt.IsZero() {
  		row.CreatedAt = time.Now().UTC()
+ 	} else {
+ 		row.CreatedAt = row.CreatedAt.UTC()
  	}


─── packages/contracts/src/commercial.ts:202-202 ───
[maintainability · low] null 容忍不对称：本次改动为 amount_fen/currency（以及 reason 的 typeof 检查）增加了 null
容忍，但同一函数里 plan_key、plan_version 仍只容忍 undefined。按 R2-21
注释自己设想的场景（滚动升级/版本漂移时新版后端可能发出更新的线格式），若后端对可选字段输出 null（如 Go 指针类型序列化），`plan_key: null` 仍会触发整视图解析失败
'invalid purchase (plan_key)'——正是这次 reason 改动想避免的失败面。建议与相邻可选字段的守卫保持一致，补上 null 检查。

    if(v.currency!==undefined&&v.currency!==null&&v.currency!=='') out.currency = nonEmptyString(v.currency,'currency','purchase');
+   if(v.plan_key!==undefined&&v.plan_key!==null) out.plan_key = nonEmptyString(v.plan_key,'plan_key','purchase');
+   if(v.plan_version!==undefined&&v.plan_version!==null) {


─── internal/modules/commercial/service/commercial/purchase.go:345-348 ───
[performance · low] 同一请求内对 FindPublicationByCode(ctx, p.PlanCode) 做了两次完全相同的 DB 查询:第 329-332
行的外层读取已经拿到 pub(用于 out.PlanKey/out.PlanVersion),这里又以相同实参重查一遍只为取 pub.PlanKey。GET PurchaseStatus
是结账页轮询端点,每次轮询多付一次 publications 查询;且两处读取在 DB 瞬时抖动下可能给出不一致结果(外层成功、内层失败),与已确认的 wantPlanKey
空串问题叠加。建议把第一次读取的结果提升为共享变量,两次使用一次查询。

+ 	pub, pubErr := s.plans.FindPublicationByCode(ctx, p.PlanCode)
+ 	if pubErr == nil {
+ 		out.PlanKey = pub.PlanKey
+ 		out.PlanVersion = pub.Version
+ 	}
+ 	// ...(p.State != Absent 分支内)
- 		wantPlanKey := ""
+ 	wantPlanKey := ""
- 		if pub, err := s.plans.FindPublicationByCode(ctx, p.PlanCode); err == nil {
+ 	if pubErr == nil {
- 			wantPlanKey = pub.PlanKey
+ 		wantPlanKey = pub.PlanKey
- 		}
+ 	}


─── internal/modules/commercial/service/commercial/purchase.go:306-309 ───
[maintainability · low] purchaseAudit 硬编码 context.Background(),丢掉了调用点的请求 ctx:logger.GetLogger(c) 依赖
ctx 携带请求级 logger 字段/trace,用 Background 后这条 attention 行无法与触发它的请求关联。另外 logger 包自身的文档明确指出
audit-relevant 事件(跨租户探测、不变量违反——正是本 sink 的定位)应使用 WarnWithFields 结构化字段,便于日志聚合器直接索引 tenant/quote
标识,而非解析自由文本。两个调用点(211/229 行)都有 ctx 可传,建议签名带 ctx 并改用结构化字段输出。

- var purchaseAudit = func(tenantID uint64, quoteID, token string) {
- 	logger.Warnf(context.Background(),
- 		"[CommercialPurchase] attention: tenant=%d quote=%s token=%s", tenantID, quoteID, token)
+ var purchaseAudit = func(ctx context.Context, tenantID uint64, quoteID, token string) {
+ 	logger.WarnWithFields(ctx, logger.Fields{
+ 		"tenant_id": tenantID, "quote_id": quoteID, "token": token,
+ 	}, "[CommercialPurchase] attention")
  }


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:135-138 ───
[bug · medium] 预算耗尽分支把任意 ReadSnapshot 错误直接落为 terminal attention 记录。但 lago.go 中 429/5xx/网络抖动均映射为
ErrPlatformUnreachable（瞬时、可重试），与步骤②/⑤对同类错误的刻意处理（return nil 保持 pending、不写记录）不一致。瞬时抖动恰好落在预算耗尽的首个 pass
时，会写下与"权威永不激活"同形的终态记录并触发 Warnf 告警；运营据此退款后，后续 pass 仍持续探测，若权威随后激活，步骤⑥会用 DoUpdates 把 attention 覆盖为
APPLIED 并发放 credits——出现退款+发放的双得窗口。建议：unreachable/unconfigured 视为瞬时（return nil），仅确定性状态（canceled / 非
active）落 attention。

- 		if serr != nil || snap.Purchase == nil ||
- 			snap.Purchase.State != domain.PurchaseStateActive {
+ 		if serr != nil {
+ 			if errors.Is(serr, domain.ErrPlatformUnreachable) || errors.Is(serr, domain.ErrPlatformUnconfigured) {
+ 				return nil // 瞬时故障：保持 pending，下一 pass 再探测，不落终态记录
+ 			}
+ 			return serr
+ 		}
+ 		if snap.Purchase == nil || snap.Purchase.State != domain.PurchaseStateActive {
  			return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
  		}


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:178-181 ───
[bug · medium] 观察窗内的 ReadSnapshot 瞬时错误（429/5xx/网络抖动 → ErrPlatformUnreachable）会作为 error
一路上抛：fulfillEvent 对 purchaser.Fulfill 的 error 原样返回，而 FulfillmentService.Recover 对批次内第一个 error
直接终止整个排水 pass（fulfillment.go:176-180），排在后面的所有事件本轮全部跳过。Lago
短暂不可用或限流期间，购买类事件会让共享排水上的其他订单（含普通充值订单）反复饿死，与本文件步骤②/⑤"unreachable→return nil"的刻意瞬时处理不一致。步骤④的 psnap
读取（ReadSnapshot 后 `if err != nil { return err }`）同样如此。建议对 unreachable/unconfigured 归类为"尚未激活"（return
nil，事件保持 pending），仅确定性错误才作为 error 中断。

  	active, state, err := p.observeActivation(ctx, order.TenantID)
  	if err != nil {
+ 		if errors.Is(err, domain.ErrPlatformUnreachable) || errors.Is(err, domain.ErrPlatformUnconfigured) {
+ 			return nil // 瞬时故障：事件保持 pending，下一 pass 重放，不中断共享排水 pass
+ 		}
  		return err
  	}


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:356-358 ───
[maintainability · medium] attention/refused 落地后 Fulfill 返回 nil，事件永远保持 pending（fulfillEvent 只在订单
Fulfilled 时才转 Sent；OutboxStateDead 从未被购买路径使用）：此后每个 30s 排水 pass 都会重放——预算耗尽分支每轮发起 1 次外部 ReadSnapshot
探测并再次调用 markActivationState。OnConflict DoNothing 保证记录只写一次，但 Warnf 每轮都打：每个卡死订单每天约 2880
条相同告警，attempt_count 无上界增长，外部调用量与日志量随卡死订单数线性放大（注释宣称的 "stop driving" 实际从未停止）。建议至少用 Create 的
RowsAffected==1 门控日志（仅首次落地告警），并考虑为已 attention 的事件引入 dead-letter 或放慢探测节奏。

+ 	res := p.db.WithContext(ctx).Clauses(clause.OnConflict{
+ 		DoNothing: true,
+ 	}).Create(&rec)
+ 	if res.Error != nil {
+ 		return res.Error
+ 	}
+ 	if res.RowsAffected == 1 {
- 	logger.Warnf(ctx, "[CommercialFulfillment] purchase %s activation landed %s", orderID, state)
+ 		logger.Warnf(ctx, "[CommercialFulfillment] purchase %s activation landed %s", orderID, state)
+ 	}
  	return nil
  }


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:308-308 ───
[test · low] 结构体注入的 p.now 时钟与裸 time.Now() 混用：deadline 及循环内的 time.Now().Before(deadline)
均绕过了可注入时钟，导致单测无法通过 now 钩子确定性控制观察窗口（本包其余时间语义均走 p.now，如 TestPurchaseGrantPeriodTakesActivationMonth 覆盖
purchaser.now）。建议统一改用 p.now()。

- 	deadline := time.Now().Add(p.firstPass)
+ 	deadline := p.now().Add(p.firstPass)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:307-308 ───
[bug · medium] 该 422 分支用「当前配置非空」来区分 fail-closed 与可重试,但配置非空并不代表该租户的 provider customer 曾被附加过默认
PM。当绑定是在 StripePmToken 为空时创建的(config.go F11 注明这正是生产 posture:providerCreateCustomer 跳过
attach、ensureProviderBinding 跳过 sync,绑定不带默认 PM),之后运维再配置 StripePmToken,既有 bound
租户会进入死循环:ensureProviderBinding 的 bound 短路使 attach 永远不再执行(attach 只在 unbound 时经 deriveProviderCustomer
运行),settle rail 的 PM 附加又只对已处于 awaiting 的购买执行——而订阅从未创建成功,永远到不了 awaiting。于是每次购买都 422 → 配置齐全 → 判为可重试的
ErrPlatformUnreachable,实际是一个永不自愈的永久失败,且错误类别误导(终态 posture 被报为瞬时不可达)。placeholder 绑定(key 为空+prefix)后补
key+token 的变体同样命中。建议:在该分支中不要以进程配置为判据,而以绑定本身是否携带默认 PM 为准——例如对 bound 客户经 boundProviderCustomerID +
providerAttachDefaultPaymentMethod 重新驱动附加(再 create 一次),或在绑定时记录 attach 来源、对未附加过 PM 的绑定维持 fail-closed
unconfigured。

+ 	case status == http.StatusUnprocessableEntity &&
+ 		strings.Contains(string(respBody), "no_default_payment_method"):
+ 		if a.cfg.StripeAPIKey == "" || a.cfg.StripePmToken == "" {
+ 			return commercial.CommandReceipt{}, fmt.Errorf(
+ 				"%w: no settle-able default payment method without provider key/pm token",
+ 				commercial.ErrPlatformUnconfigured)
+ 		}
+ 		// 配置齐全 ≠ 该绑定曾附加过默认 PM:bound 短路使 attach 不会重跑,
+ 		// 对在 token 为空期落库的绑定,此处需先驱动附加再判可重试。
+ 		if err := a.ensureBoundDefaultPaymentMethod(ctx, payload.ExternalCustomerID); err != nil {
+ 			return commercial.CommandReceipt{}, err
+ 		}
  		return commercial.CommandReceipt{}, fmt.Errorf(
  			"%w: default payment method not imported yet", commercial.ErrPlatformUnreachable)


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:194-199 ───
[bug · high] 步骤④（激活后 D6' 复核）的 ReadSnapshot 错误原样上抛，与步骤②/⑤对同类错误的刻意分流不一致，且这是观察窗之外的独立调用点（修复 finding 2 的
observeActivation 不会覆盖此处）：1) 瞬时错误（429/5xx/超时 → ErrPlatformUnreachable）会让 fulfillEvent 把 error
原样返回，Recover 对批次内第一个 error 直接终止整个排水 pass——激活刚达成、恰逢一次限流抖动时，本事件发放被推迟且排在后面的所有事件（含充值订单）本轮全部饿死；2) 确定性
ErrPlatformInvalidResponse（如适配器 readPurchaseInvoiceFees 的 fail-closed："multiple finalized purchase
invoices"、invoice body malformed、非 200）会在每个 pass 重复返回同一
error——该事件永久卡住共享排水，后续事件被无限期饿死，而本文件对确定性失败的既定约定是落 attention/refused 终态并 return nil（如步骤②对 settle
invalid-response 的处理）。建议：unreachable/unconfigured → return nil 保持 pending（各步幂等，下轮重放）；其余确定性错误 →
markActivationState(attention) 后 return nil。

  	psnap, err := p.platform.ReadSnapshot(ctx, domain.SnapshotQuery{
  		Kind: domain.SnapshotKindPurchase, TenantID: order.TenantID,
  	})
  	if err != nil {
- 		return err
+ 		if errors.Is(err, domain.ErrPlatformUnreachable) || errors.Is(err, domain.ErrPlatformUnconfigured) {
+ 			return nil // transient: the next pass re-drives (every step is idempotent)
+ 		}
+ 		return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
  	}


LLM retry report summary: 111 of 820 requests affected -- 58 requests failed, 53 requests recovered after retry

Review planning (22 requests):
- deploy/lago-lab/payment-activation/phases.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- deploy/lago-lab/payment-settle-trigger/evidence/t11-environment.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- deploy/lago-lab/payment-settle-trigger/evidence/t11-settle-trigger-pbce.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- deploy/lago-lab/payment-settle-trigger/fixtures.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- deploy/lago-lab/payment-settle-trigger/run_lab.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 17 more

Core review (88 requests):
- apps/web/src/commercial/BillingPage.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- apps/web/src/commercial/CheckoutPage.tsx: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- deploy/lago-lab/payment-activation/phases.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- deploy/lago-lab/payment-settle-trigger/.gitignore: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- deploy/lago-lab/payment-settle-trigger/evidence/t11-cleanup.json: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- ... and 83 more

File grouping (1 request):
- __grouping__: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed

Per-attempt detail: --format json (retry_report).
