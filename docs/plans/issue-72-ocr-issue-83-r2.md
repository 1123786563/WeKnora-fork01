Review complete: 21 finding(s) across 23 selected item(s).

─── .gitignore:98-102 ───
[documentation · low] 该注释与同一更新中的脚本改动相互矛盾：注释以现在时声称 "seed scripts write the RAW login answer here on
every rerun"，但本批次已把全部 seed 脚本（三个 r4-flow/seed.sh 及 seed_83.sh）改为落盘前即脱敏，重跑不会再产生含活体 JWT 的 RAW 文件。注释后半句
"keeps a future rerun's live capture OUT of git status" 也不准确——对已跟踪的 12 个 seed-login-*.json
副本，.gitignore 不生效，重跑覆盖仍会出现在 git status。该规则目前的实际效果是：未来新增目录中已脱敏的 seed-login 证据文件会被排除在 git add 之外（需 git
add -f 才能纳入，见 ocr83fix-replay/README.md
中"挡住未来新文件"的既定意图）。建议改写注释以反映当前真实行为，避免后续维护者误以为重跑仍会向磁盘泄漏活体令牌、或误以为该规则对已跟踪文件有效。

- # (OCR C-83 / C-01) Flow-evidence login captures: seed scripts write the RAW
- # login answer here on every rerun — with live access/refresh JWTs. The
- # tracked copies are redacted (token fields = "REDACTED"); this rule keeps a
- # future rerun's live capture OUT of git status until it is redacted.
+ # (OCR C-83 / C-01) Flow-evidence login captures. Seed scripts now write
+ # these files REDACTED at capture time (token fields = "REDACTED"); this
+ # rule is defense-in-depth in case a future script regresses to RAW output.
+ # Note: it only affects NEW files — already-tracked copies stay tracked, and
+ # new (redacted) evidence captures require `git add -f` to be committed.
  docs/plans/**/seed-login-*.json


─── internal/modules/commercial/payment/wechat.go:445-448 ───
[documentation · low] 本次改动后，Query 返回的 ProviderID 语义（渠道交易号优先）与 payment/provider.go 中
AttemptResult.ProviderID 的契约注释不再一致（L53-56 仍写着 "ProviderID is the provider-visible identifier of the
ORIGINAL request (for WeChat the out_trade_no)"）。现在 Create 返回原始 out_trade_no、Query 返回
transaction_id，同一字段两种语义却无文档区分。建议同步更新 provider.go 中 AttemptResult 的文档，明确 Query 结果键渠道交易号（与 Verify 产出的
fact.Transaction 同源，供 ConfirmPayment sameTxn 比对），避免后续新增 Provider 实现者按旧文档复刻 out_trade_no 语义、重新引入 #83
缺陷。



─── internal/modules/commercial/payment/alipay.go:479-482 ───
[maintainability · low] 身份键切换存在一个过渡窗口（仅当已有按旧行为落库的存量数据时成立）：部署前经 RecoverOrderStatus/CloseChannelOrder
恢复确认的 attempt，其 provider_transaction_id 已按旧逻辑落库为 out_trade_no；部署后渠道重投的真实 notify（支付宝最长重试约 24h）携带
trade_no，sameTxn 匹配失败，会为同一笔支付补记一条 spurious over-payment 审计事件（不会重复履约/重复发放，仅审计噪声，且
provider_transaction_id 会被顺带修正为 trade_no）。若该版本发布前存在生产存量，建议在发布说明中记录该一次性审计噪声，或做一次数据校正；若尚无生产数据则可忽略。



─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:71-72 ───
[maintainability · low] 脱敏过滤器在文件内重复内联 2 次（三个 r4-flow 脚本共 6 份拷贝），而同批次的 seed_83.sh 已抽取 redact_login()
帮助函数。若登录响应将来新增或重命名凭据字段（例如新的 session token），需要同步修改 6 处内联拷贝，漏改任意一处就会让活体凭据落盘、违反本注释声称的 "no usable
credential literal ever reaches a file" 红线。建议与 seed_83.sh 保持一致，抽取 redact_login() 帮助函数。

- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"
+ redact_login() { jq 'with_entries(if (.key == "token" or .key == "refresh_token") then .value = "REDACTED" else . end)'; }
+ redact_login <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ redact_login <<<"$LOGIN_B" > "$EV/seed-login-b.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:71-71 ───
[bug · low] jq 的普通赋值在键不存在时会新增该键。登录失败（如 401 返回 {"success":false,...} 的 JSON 错误体）时，落盘的捕获文件会带上 API
从未返回过的 "token":"REDACTED"、"refresh_token":"REDACTED" 字段，歪曲证据文件所记录的真实响应形状。建议只对已存在的键脱敏（with_entries
只遍历现有键），保持失败响应的原始结构。

- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ jq 'with_entries(if (.key == "token" or .key == "refresh_token") then .value = "REDACTED" else . end)' <<<"$LOGIN_A" > "$EV/seed-login-a.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:70-71 ───
[maintainability · low] 脱敏过滤器在文件内重复内联 2 次（三个 r4-flow 脚本共 6 份拷贝），而同批次的 seed_83.sh 已抽取 redact_login()
帮助函数。若登录响应将来新增或重命名凭据字段，需要同步修改 6 处内联拷贝，漏改任意一处就会让活体凭据落盘、违反 "no usable credential literal ever reaches
a file" 红线。建议抽取 redact_login() 帮助函数保持一致。

- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"
+ redact_login() { jq 'with_entries(if (.key == "token" or .key == "refresh_token") then .value = "REDACTED" else . end)'; }
+ redact_login <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ redact_login <<<"$LOGIN_B" > "$EV/seed-login-b.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:70-70 ───
[bug · low] jq 的普通赋值在键不存在时会新增该键。登录失败（如 401 返回 JSON 错误体）时，落盘的捕获文件会带上 API 从未返回过的
"token":"REDACTED"、"refresh_token":"REDACTED" 字段，歪曲证据文件所记录的真实响应形状。建议只对已存在的键脱敏（with_entries 只遍历现有键）。

- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ jq 'with_entries(if (.key == "token" or .key == "refresh_token") then .value = "REDACTED" else . end)' <<<"$LOGIN_A" > "$EV/seed-login-a.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:67-68 ───
[maintainability · low] 脱敏过滤器在文件内重复内联 2 次（三个 r4-flow 脚本共 6 份拷贝），而同批次的 seed_83.sh 已抽取 redact_login()
帮助函数。若登录响应将来新增或重命名凭据字段，需要同步修改 6 处内联拷贝，漏改任意一处就会让活体凭据落盘、违反 "no usable credential literal ever reaches
a file" 红线。建议抽取 redact_login() 帮助函数保持一致。

- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"
+ redact_login() { jq 'with_entries(if (.key == "token" or .key == "refresh_token") then .value = "REDACTED" else . end)'; }
+ redact_login <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ redact_login <<<"$LOGIN_B" > "$EV/seed-login-b.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:67-67 ───
[bug · low] jq 的普通赋值在键不存在时会新增该键。登录失败（如 401 返回 JSON 错误体）时，落盘的捕获文件会带上 API 从未返回过的
"token":"REDACTED"、"refresh_token":"REDACTED" 字段，歪曲证据文件所记录的真实响应形状。建议只对已存在的键脱敏（with_entries 只遍历现有键）。

- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ jq 'with_entries(if (.key == "token" or .key == "refresh_token") then .value = "REDACTED" else . end)' <<<"$LOGIN_A" > "$EV/seed-login-a.json"


─── internal/modules/commercial/payment/wechat.go:445-451 ───
[bug · medium] 存量数据过渡窗口（与已确认的 alipay 同类问题，但发生在微信侧）：本改动之前 wechat Query 从未解析 transaction_id、恒回
out.OutTradeNo，因此部署前经 RecoverOrderStatus 恢复确认的 attempt 已把 provider_transaction_id 落库为
out_trade_no。本版本上线后，微信 v3 回调重投（通知最长约 24h 内重试）携带 transaction_id，ConfirmPayment 的 sameTxn
匹配失败，会为同一笔支付补记一条 spurious over-payment 审计事件（不会重复履约/重复发放，provider_transaction_id 顺带被改写为
transaction_id，属一次性审计噪声）。若部署前存在存量恢复数据，建议一并评估：接受一次性审计噪声，或在 sameTxn 判定中对旧键值兼容（如同时接受等于
merchant_order_id 的历史记录并静默改写），避免与 alipay 侧风险分别处理。



─── internal/modules/commercial/repository/commercial/order.go:534-538 ───
[bug · medium] 误导性哨兵：此分支在订单存在但已并发离开 pending（典型：渠道关闭期间支付回调已 ConfirmPayment 成交，订单变 paid）时返回
ErrOrderNotFound —— 订单并非不存在，而是刚刚 PAID。该哨兵经 CloseChannelOrder.landClosed 原样上抛，再被 purchase.go
渠道切换分支（`if cerr != nil { return PurchaseView{}, cerr }`）直接作为 Purchase 的错误答案：用户刚付款成功的请求会收到
order_not_found 错误（handler 将该哨兵归入 404/错误分支），而非已支付视图；虽然重试可经 GetOrderByQuote
自愈，但错误契约会误导调用方（可能诱发重新报价/重复购买）。建议：返回专用的状态冲突哨兵（包内已有 ErrInvalidOrderState 可复用），并/或在 landClosed 中对
errors.Is(err, ErrOrderNotFound) 时重读订单、对非 pending 状态作答已定结局（与 SUCCEEDED 分支的 after 重读对称）。注意
CloseChannelOrder 无 attempt 分支的 MarkChannelFailed（RowsAffected==0 同样返回
ErrOrderNotFound）在并发确认下也有同一形状。

  		if res.RowsAffected == 0 {
- 			return ErrOrderNotFound
+ 			// The order exists but concurrently left pending (e.g. the
+ 			// callback confirmed the payment mid-close): not-found misleads —
+ 			// surface a state conflict so CloseChannelOrder can re-read and
+ 			// answer the decided state instead of a 404 for a paid order.
+ 			return ErrInvalidOrderState
  		}
  		return nil
  	})


─── internal/modules/commercial/repository/commercial/order.go:497-499 ───
[maintainability · low] 死代码：MarkAttemptClosed 在生产与测试代码中均无调用点 —— CloseChannelOrder 的落账已改用事务化的
CloseAttemptAndRetireChannel（OCR C-07 成对落地），该单步方法被整体取代后遗留。若后续 #84/#92
生命周期票需要单步关闭入口，届时应随真实调用方一起添加；建议当前删除以缩小维护面（也避免后来者误用非事务版造成 C-07 已修复的半落地状态复发）。



─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:133-140 ───
[bug · medium] sqlShape() 失败路径与注释声明相悖：line 135 写入 files['sql-shape-failure.txt'] 后随即
process.exit(2)，而 files 的落盘循环（writeFileSync）位于脚本末尾 catch 之后，永远不会执行。结果是：(1) sql-shape-failure.txt
本身永不落盘；(2) 此前各 act 已累积的 recovery-no-notify.txt、close-race-*.txt、duplicate-notify-idempotent.txt
等日志全部丢失。这与 (C-02) 注释 "a failed shape check writes the act log and terminates" 以及 (C-04) "must leave
diagnosable evidence, never a half-written run" 的设计意图直接冲突——形状校验失败时反而产生最难诊断的一次静默半写。建议在退出前先 flush 已累积的
files，或改为抛出异常交给顶层 catch（catch 之后的落盘循环会自然执行，RESULT 汇总也会打印）。

-   function sqlShape(name, value, re) {
+ function sqlShape(name, value, re) {
-     if (!re.test(value)) {
+   if (!re.test(value)) {
-       files['sql-shape-failure.txt'] = `refusing to interpolate ${name} into SQL: ${JSON.stringify(value)}\n`;
+     files['sql-shape-failure.txt'] = `refusing to interpolate ${name} into SQL: ${JSON.stringify(value)}\n`;
-       console.error(`bad ${name} for SQL interpolation: ${JSON.stringify(value)}`);
+     console.error(`bad ${name} for SQL interpolation: ${JSON.stringify(value)}`);
+     flushFiles(); // 先落盘全部已累积证据再退出
-       process.exit(2);
+     process.exit(2);
-     }
+   }
-     return value;
+   return value;
-   }
+ }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:222-225 ───
[bug · low] act3 块内 `const tokenF = TOKEN_MAIN;` 声明后从未被引用（本幕的断言全部走 SQL/TEN 与 stubNotify，act5
的同名变量才有实际使用），是遗留的死变量，容易让读者误以为本幕发起了 API 调用。建议删除。

      const act = 'act3-duplicate';
      const log = [];
-     const tokenF = TOKEN_MAIN;
      const orders = await stubOrders();


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:353-353 ───
[test · low] act5 的金额断言用裸子串匹配：`walletsAny.includes('9.9')` 会被 '19.9'、'9.90x' 或含 "9.9"
的钱包名命中；`credits.includes('10900000')` 同样会被 '110900000' 等相邻数值命中。这是产出可信证据的复验脚本，弱断言的假阳性 PASS 会直接削弱 "恰好
9.90 到账 / 恰好 10900000 微余额" 的证明力。建议对 walletsAny 按行/列解析后做数值精确比较（如 Number(amount) === 9.9），对 credits 解析
JSON 后断言具体键值，或用行级锚定正则。

-     note(act, 'purchase-wallet-granted', walletsAny.includes('9.9'), 'the purchase wallet granted the 9.90 first period (status 1 = settled)');
+     const walletGranted = walletsAny.split('\n').some((l) => { const c = l.split('|'); return c.length >= 3 && Number(c[2]) === 9.9; });
+     note(act, 'purchase-wallet-granted', walletGranted, 'the purchase wallet granted the 9.90 first period (status 1 = settled)');


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:24-25 ───
[bug · low] 此行（以及其后的 readFileSync(seed-login-f.json)、browser_flow_83.mjs 中 try 块之外的
chromium.launch()）都在脚本自身声明的顶层守卫之外：docker 容器未启动或 LAGO_KEY 读取失败时，execFileSync 直接抛出未捕获异常，进程裸栈崩溃——不打印
RESULT JSON 汇总、不落任何证据文件，违背 (C-04)/(C-05) "任何失败都留下可诊断证据" 的目标。建议把 LAGO_KEY 读取与 seed-login-f.json 解析挪进
try（或前置处包一层 catch 后 note('script','preflight',false,...) 并走统一落盘路径）。

- const LAGO_KEY = execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
+ function readLagoKey() {
+   try {
+     return execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
-   'select value from api_keys limit 1'], { encoding: 'utf8' }).trim();
+       'select value from api_keys limit 1'], { encoding: 'utf8' }).trim();
+   } catch (err) {
+     note('script', 'preflight-lago-key', false, String(err));
+     for (const [name, content] of Object.entries(files)) writeFileSync(`${EV}${name}`, content + '\n');
+     console.log('RESULT ' + JSON.stringify(results));
+     process.exit(1);
+   }
+ }
+ const LAGO_KEY = readLagoKey();


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:124-126 ───
[bug · low] chromium.launch() 在 try 块之外：Playwright 浏览器缺失/版本不匹配时进程直接抛未捕获异常退出，progression 文件与 RESULT
汇总都不会产生，违背本文件 (C-05) 注释 "任何失败都留下可诊断证据" 的设计目标。建议将 launch 纳入 try（并在 finally 中判空后再
browser.close()），或前置失败时也输出 RESULT。

- const browser = await chromium.launch();
+ let browser;
  let page;
  try {
+   browser = await chromium.launch();


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:194-197 ───
[maintainability · low] 此处的 NOTPAY/9900 过滤逻辑与 api_recovery_83.mjs 中 newestPendingOrder()（line
100-104）完全同源，且 lagoCustomer()、deliverWebhook 的 webhook 投递逻辑、ORG UUID
'305eddac-1bbd-47a3-af15-219f1d39a27d'、业务金额 9900 也都在两个文件中各自硬编码一份。计划金额一旦调整（9900 ->
其他），需要同步修改两处过滤条件与断言，漏改一处会让 browser 幕与 API 幕的证据互相矛盾（一幕标记了错误订单、另一幕断言失败）。建议抽取共享 helper（如
evidence-common.mjs 导出 newestPendingOrder/lagoCustomer/deliverWebhook/常量
PLAN_AMOUNT_FEN/ORG），两脚本共同引用。

    const orders = await stubOrders();
+   // 与 api_recovery_83.mjs 共享的过滤逻辑：建议抽到共享模块（如 evidence-common.mjs
+   // 的 newestPendingOrder(orders)），常量 PLAN_AMOUNT_FEN 统一定义
    const pendingKeys = Object.entries(orders)
-     .filter(([, v]) => v?.state === 'NOTPAY' && v?.total === 9900)
+     .filter(([, v]) => v?.state === 'NOTPAY' && v?.total === PLAN_AMOUNT_FEN)
      .map(([k]) => k);


─── docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py:285-289 ───
[bug · low] ORDERS 是模块级共享 dict，而服务器是 ThreadingHTTPServer（多线程），且 /stub/notify 会同步阻塞等待 WeKnora
应答（urlopen timeout=15s）期间服务器仍在并发处理其他请求。CLOSE 分支的 `if order["state"] == "SUCCESS" ... order["state"]
= "CLOSED"` 是无锁的 check-then-act，与 /stub/mark 的 `order["state"] = data["state"]` 交错时可能把一个刚被标记 SUCCESS
的订单覆盖为 CLOSED——这恰好会污染 act4 关单竞态轮赖以判断的证据事实（把 ORDER_PAID 竞态错误地演成 clean close）。建议加模块级 threading.Lock()
收敛 ORDERS 的全部读写（至少 close/mark 写路径），close 判定与置 CLOSED 在同一临界区内完成。

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


─── internal/modules/commercial/repository/commercial/order.go:528-530 ───
[bug · medium] 成对落账后订单行成为首个 `channel_failed=true` 且 `checkout_url`
仍非空的形态,打破了存储层自身的不变量("channel_failed ⇒ 非可付入口 ⇒ 无链"——此前 MarkChannelFailed 只在渠道 Create 失败(从未落链)或
SweepStaleLinklessPending(仅无链残留)时置位)。后果:渠道侧订单已关闭,该 code_url 已不可支付,但既有读路径仍会把它当支付入口重新发给客户——(1)
RecoverOrderStatus 的无 pending attempt 分支(FistPendingAttempt 关闭后必然 NotFound)按 R1-35 "the persisted
checkout link still rides along so the customer keeps a payment entry" 原样回链;(2) 客户用旧 quote 重放 POST
purchase 时 GetOrderByQuote(无 state/channel_failed 过滤)+ orderViewFromRow
同样原样回死链,且该重放先于渠道切换分支执行,客户拿到一个永远付不出去的入口。建议在同一事务里一并清空 checkout_url,保持与既有 channel-failed 姿态一致(所有
payable 谓词用 `checkout_url <> ''`,空串即不可付;迟到确认所需的渠道身份保存在 attempt 行,不受影响)。

  		res := tx.Model(&OrderRow{}).
  			Where("id = ? AND state = ?", orderID, domain.OrderStatePending).
- 			Update("channel_failed", true)
+ 			Updates(map[string]interface{}{"channel_failed": true, "checkout_url": ""})


─── internal/modules/commercial/repository/commercial/order.go:534-536 ───
[bug · medium] MySQL changed-rows 语义下的幂等缺失(与已确认的"并发已付款返回误导哨兵"不同触发点):同一订单被并发重复关单时(双击/双标签页各自持新 quote
进入渠道切换分支;微信关单对已关单返回成功),后到者进入本事务时订单仍为 pending、channel_failed 已为 true——UPDATE
命中行但值未变,go-sql-driver/mysql 默认(未设 CLIENT_FOUND_ROWS)的 RowsAffected 按"实际改变行数"计,返回 0,被误判为
ErrOrderNotFound:订单明明存在且仍 pending,整笔 Purchase 却以 order_not_found 失败。这与 MarkAttemptClosed
文档承诺的"replay is a no-op"幂等语义相悖(SQLite 的 changes() 按命中行计数,现有测试不会暴露)。建议 RowsAffected==0 时回读订单行区分:存在且仍
pending 且 channel_failed=true ⇒ 幂等成功;否则才是真正的拒绝落地。

  		if res.RowsAffected == 0 {
+ 			var cur OrderRow
+ 			if err := tx.Where("id = ?", orderID).First(&cur).Error; err != nil {
+ 				return err
+ 			}
+ 			if cur.State == domain.OrderStatePending && cur.ChannelFailed {
+ 				return nil // 幂等重放:成对落账已由并发请求完成
+ 			}
  			return ErrOrderNotFound
  		}


LLM retry report summary: 2 of 135 requests affected -- 1 request failed, 1 request recovered after retry

Core review (2 requests):
- docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs,docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs,docs/plans/issue-72-flow-evidence-83/seed_83.sh,docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh,docs/plans/issue-72-flow-evidence-83/v2_probe.sh,docs/plans/issue-72-flow-evidence-83/v2_up_backend.sh,docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh,docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py: timed out -> failed
- docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs,docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs,docs/plans/issue-72-flow-evidence-83/seed_83.sh,docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh,docs/plans/issue-72-flow-evidence-83/v2_probe.sh,docs/plans/issue-72-flow-evidence-83/v2_up_backend.sh,docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh,docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
