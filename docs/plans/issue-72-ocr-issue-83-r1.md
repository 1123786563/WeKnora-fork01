Review complete: 40 finding(s) across 22 selected item(s).

─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:102-104 ───
[security · high] sql() 及各处内联查询全部以模板字符串拼接 SQL：仅 TEN 做了 ^\d+$ 白名单校验，而 u.tenant（act4b payable
查询）、orderID（act3/act4 的 id='...' 与 like '%...'）、fulfilledOrder（act3 counts）、cus/cus20（act3/act5 psql
的 provider_customer_id='...'）均未校验直接插值。登录失败时 u.tenant 为 undefined、lagoCustomer
失败返回空串，会拼出语法破碎或恒不匹配的查询，产生误导性 PASS/FAIL 证据，也违反本次验收条件「数据库查询禁止用拼接组装 SQL」。sqlite3 CLI
无法真正绑定参数，至少应把所有插值统一过形状白名单（租户 ^\d+$、订单 ^ord_[0-9a-f]+$、Lago uuid）后再拼入。

+ const RE_TENANT = /^\d+$/;
+ const RE_ORDER = /^ord_[0-9a-f]+$/;
+ const RE_UUID = /^[0-9a-f-]{36}$/;
+ function sqlShape(name, value, re) {
+   if (!re.test(String(value))) {
+     console.error(`bad ${name} for SQL interpolation: ${value}`);
+     process.exit(2);
+   }
+   return String(value);
+ }
  function sql(query) {
    return execFileSync('sqlite3', [DB_PATH, query], { encoding: 'utf8', cwd: fileURLToPath(new URL('../../..', import.meta.url)) }).trim();
  }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:186-188 ───
[bug · medium] 形状校验失败后仅记一条 FAIL 便继续执行：未通过校验的 fulfilledOrder（可能含引号、换行的多行返回值）随后被原样拼进 outboxFulfill 与
attempts 两条 SQL，破坏查询语法并让幂等断言失真。应与文件顶部 TEN 校验的 fail-fast 行为一致：校验失败立即落盘本幕日志并终止。

    if (!/^ord_[0-9a-f]+$/.test(fulfilledOrder)) {
      note(act, 'fulfilled-order-shape', false, `unexpected fulfilled order id: ${fulfilledOrder}`);
+     writeFileSync(`${EV}duplicate-notify-idempotent.txt`, log.join('\n') + '\n');
+     throw new Error(`unexpected fulfilled order id: ${fulfilledOrder}`);
    }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:79-83 ───
[bug · medium] 函数名叫 newestPendingOrder 却完全不过滤 state（stub 订单有 NOTPAY/SUCCESS/CLOSED 三态），且依赖
/stub/orders JSON 对象的键插入序「取最后一个键」来猜最新单。act2/act4a/act4b 的 stubMark
目标单全由它决定：桩跨轮未重启（重跑残留）、purchaseUntilLanded 重试产生多张渠道单或并发下单时，会把错误的单标成 SUCCESS，整幕证据失真。至少应按
state='NOTPAY' 过滤，并像 browser_flow_83.mjs 那样校验 total 金额。

- async function newestPendingOrder() {
+ async function newestPendingOrder(expectedTotal) {
    const orders = await stubOrders();
-   const keys = Object.keys(orders);
-   return keys[keys.length - 1] ?? '';
+   const pending = Object.entries(orders).filter(([, o]) =>
+     o.state === 'NOTPAY' && (expectedTotal === undefined || o.total === expectedTotal));
+   return pending.at(-1)?.[0] ?? '';
  }
+ // 调用处：await newestPendingOrder(9900)


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:24-25 ───
[maintainability · medium] 业务硬编码散落多处：容器名 'weknora-lago-82flow-db-1' 在本文件出现 4
次（L24/L122/L201/L278）、Lago 端口 48889 两处（deliverWebhook 与 act5 lagoAPI）、ORG UUID
'305eddac-...'（driveToActive，与 browser_flow_83.mjs 各一份）、金额 9900（act3）与
10900000（act5）。环境迁移或多轮重跑需多点同步修改，极易顾此失彼。建议沿用本文件 BACKEND/STUB/DB_PATH 的 env+默认值模式，集中为模块级常量。

- const LAGO_KEY = execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
-   'select value from api_keys limit 1'], { encoding: 'utf8' }).trim();
+ const LAGO_DB = process.env.FLOW83_LAGO_DB ?? 'weknora-lago-82flow-db-1';
+ const LAGO_BASE = process.env.FLOW83_LAGO_BASE ?? 'http://127.0.0.1:48889';
+ const LAGO_ORG = process.env.FLOW83_LAGO_ORG ?? '305eddac-1bbd-47a3-af15-219f1d39a27d';
+ const lagoSql = (q) => execFileSync('docker', ['exec', LAGO_DB, 'psql', '-U', 'lago', '-tAc', q], { encoding: 'utf8' }).trim();
+ const LAGO_KEY = lagoSql('select value from api_keys limit 1');


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:118-118 ───
[maintainability · medium] deliverWebhook 用空 catch 吞掉全部异常（含 execFileSync 的 stderr/退出码），返回的
'deliver-pending' 也从未被 driveToActive 消费（`if (cus) deliverWebhook(cus, ORG);` 直接丢弃返回值）：webhook
投递系统性失败时日志毫无痕迹，只会表现为 18 轮轮询超时后的 FAIL，根因完全被掩盖。建议返回值携带错误详情，并由调用方 push 进 log。

-   } catch { return 'deliver-pending'; }
+   } catch (e) {
+     return `deliver-failed: ${String(e.stderr ?? e.message).trim().slice(0, 160)}`;
+   }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:53-54 ───
[bug · medium] registerAndLogin 不检查注册/登录结果：登录失败时 token 与 active_tenant.id 均为 undefined，后续所有请求以
`Bearer undefined` 发出、u.tenant 又以 undefined 拼进 SQL，整幕级联 FAIL 且真实根因（用户名冲突、密码策略等）被淹没。应在返回前校验并带上下文抛错。

-   const { json } = await api('POST', '/api/v1/auth/login', null, { email, password: PW });
+   const { status, json } = await api('POST', '/api/v1/auth/login', null, { email, password: PW });
+   if (!json?.token || !json?.active_tenant?.id) {
+     throw new Error(`login failed for ${email}: HTTP ${status} ${JSON.stringify(json).slice(0, 200)}`);
+   }
    return { token: json.token, tenant: json.active_tenant.id, email };


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:213-214 ───
[bug · medium] purchaseUntilLanded 四次重试耗尽后返回 { res:null, order:null }，此处不中止：orderID 为空串会继续 stubMark
桩里最新一张（可能是残留的）单、执行 id='' 的 SQL 与各项断言，产出误导性 FAIL 证据（act4b 同一模式）。落地失败应立即 note + 落盘日志 + 终止该幕。

    const first = await purchaseUntilLanded(u.token, 'wechat', log);
    const orderID = first.order?.id ?? '';
+   if (!orderID) {
+     note(act, 'wechat-order-landed', false, 'purchase never landed; aborting act');
+     writeFileSync(`${EV}close-race-paid.txt`, log.join('\n') + '\n');
+     throw new Error('wechat order never landed');
+   }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:170-172 ───
[maintainability · low] act3 声明的 tokenF 全幕未使用（act5 的 tokenF 才被 /account 请求消费），属于死代码，建议删除以免误导读者以为
act3 也走鉴权请求。

    const act = 'act3-duplicate';
    const log = [];
-   const tokenF = MAIN_LOGIN.token;


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:278-278 ───
[maintainability · low] 同一段 docker exec psql 模板在本文件重复出现 4 份（LAGO_KEY、lagoCustomer、act3 的
lagoPayments、act5 的 lago），act5 已局部封装成 lago() 却未提升复用。建议提取模块级 helper 并统一走 LAGO_DB
常量，替换其余三处内联调用，消除复制漂移风险。

-   const lago = (q) => execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc', q], { encoding: 'utf8' }).trim();
+   // 模块级（配合 LAGO_DB 常量）：
+   // const lagoSql = (q) => execFileSync('docker', ['exec', LAGO_DB, 'psql', '-U', 'lago', '-tAc', q], { encoding: 'utf8' }).trim();
+   const lago = lagoSql;


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:301-301 ───
[test · low] 用子串 includes('9.9') 判定到账金额过于脆弱：19.9、39.9 等同样命中，行内 name/status 字段串若含 "9.9"
也会误判，且金额以魔法数内嵌。建议按列解析 amount 后做数值比较。

-   note(act, 'purchase-wallet-granted', walletsAny.includes('9.9'), 'the purchase wallet granted the 9.90 first period (status 1 = settled)');
+   const granted = walletsAny.split('\n').some((row) => {
+     const amount = Number.parseFloat(row.split('|').pop() ?? '');
+     return Number.isFinite(amount) && Math.abs(amount - 9.9) < 1e-9;
+   });
+   note(act, 'purchase-wallet-granted', granted, 'the purchase wallet granted the 9.90 first period (status 1 = settled)');


─── docs/plans/issue-72-flow-evidence-83/seed-login-a.json:0-0 ───
[security · high] 将完整可用的 JWT access token（exp 2026-09-28）与 refresh_token（exp
2026-10-04，长效可刷新）以字面量提交进仓库，直接违反本分支安全验收约束「凭据只从环境变量或密钥服务读取，源码、示例和测试不得写入可用的凭据字面量」；该文件是 seed_83.sh
运行期产物（每轮重跑重新生成），不应入库。建议：git rm --cached 并将 seed-login-*.json 加入
.gitignore；如需保留证据仅保留脱敏版（token/refresh_token 置为 REDACTED），并使已泄露的 refresh token 失效。



─── docs/plans/issue-72-flow-evidence-83/seed-login-b.json:1-1 ───
[security · high] 将完整可用的 JWT access token 与 refresh_token 以字面量提交进仓库（tenant 40 owner 身份，refresh 有效期约
7 天），违反安全验收约束「源码、示例和测试不得写入可用的凭据字面量」；且该账号持有 platform 范围 plan_publish 授权（seed_83.sh 直写
commercial_grants），令牌泄露即可冒充发布者。建议：git rm --cached + .gitignore，证据保留脱敏版，并失效对应 refresh token。



─── docs/plans/issue-72-flow-evidence-83/seed-login-c.json:0-0 ───
[security · high] 提交了字面量 JWT access/refresh token（tenant
17，早期轮次遗留，非本轮主角），违反「源码、示例和测试不得写入可用的凭据字面量」约束；过期遗留令牌同样不应入库（泄露后可用于比对/重放测试）。建议从仓库移除（git rm --cached +
.gitignore seed-login-*.json），历史证据以脱敏形式保留。



─── docs/plans/issue-72-flow-evidence-83/seed-login-d.json:1-1 ───
[security · high] 提交了字面量 JWT access/refresh token（tenant
18，早期轮次遗留），违反「源码、示例和测试不得写入可用的凭据字面量」约束。建议从仓库移除（git rm --cached + .gitignore），如需留证仅保留脱敏字段。



─── docs/plans/issue-72-flow-evidence-83/seed-login-e.json:0-0 ───
[security · high] 提交了字面量 JWT access/refresh token（tenant
19，早期轮次遗留），违反「源码、示例和测试不得写入可用的凭据字面量」约束。建议从仓库移除（git rm --cached + .gitignore），如需留证仅保留脱敏字段。



─── docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh:20-20 ───
[bug · medium] apiv3.key 为原始随机 32 字节，而两个消费方都会对文件内容做空白剥离：Go 侧 NewWechatProvider 先 bytes.TrimSpace
再强校验 32 字节（wechat.go L167-169），Python stub 的 _load_apiv3 同样 strip() 后校验。当随机密钥的首或尾字节恰为 ASCII
空白（0x09-0x0D、0x20，概率约 4.6%）时，两侧都会把密钥截成 31 字节并在启动时报错（"must be 32 bytes, got 31"），整个复验轮随机失败；且脚本末尾的 `wc
-c` 只校验文件长度，无法发现该问题（文件本身仍是 32 字节）。建议在生成时构造性地保证首尾字节非空白。

- head -c 32 /dev/urandom > "$KEYS/apiv3.key"
+ # 消费方会 strip() 首尾空白字节并被截短成 31 字节 -> 启动失败；首尾各取 1 个非空白字节夹住 30 个随机字节
+ nonws_byte() { head -c 64 /dev/urandom | tr -d '\t\n\v\f\r ' | head -c 1; }
+ { nonws_byte; head -c 30 /dev/urandom; nonws_byte; } > "$KEYS/apiv3.key"


─── docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh:9-9 ───
[bug · low] `${TMPDIR}` 既无缺省值也无路径分隔符：在 `set -u` 且 TMPDIR 未设置的常见 Linux 环境（cron/CI/容器）会直接报 "unbound
variable" 退出；在 TMPDIR=/tmp 的环境下默认目录变成 `/tmpissue83-v2-keys`（落在根目录，通常无写权限导致 mkdir 失败）。建议改为
`${TMPDIR:-/tmp}/...`。

- KEYS="${1:-${TMPDIR}issue83-v2-keys}"
+ KEYS="${1:-${TMPDIR:-/tmp}/issue83-v2-keys}"


─── docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh:8-9 ───
[security · low] 脚本生成商户/平台私钥 PEM 与 apiv3.key，但未设置 umask：文件按调用进程的默认 umask（常见 022）以 0644 落盘，与本轮在
v2_up_backend.sh 中强调的"密钥只经运行时 0600 env 文件注入"纪律不一致，多用户主机上可被其他本地账户读取。建议生成前 `umask 077`。

  set -euo pipefail
- KEYS="${1:-${TMPDIR}issue83-v2-keys}"
+ umask 077   # 私钥与 apiv3.key 以 0600 落盘，兑现运行时凭据纪律
+ KEYS="${1:-${TMPDIR:-/tmp}/issue83-v2-keys}"


─── docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh:23-23 ───
[bug · low] `${TMPDIR}issue83-wechat-stub.log` 直接拼接、无 `/` 分隔符：仅在 macOS（launchd 的 TMPDIR 以 `/` 结尾）下才与
api_recovery_83.mjs 读取的 `${tmpdir()}/issue83-wechat-stub.log` 解析到同一路径；在 TMPDIR=/tmp 的 Linux 环境下会写成
`/tmpissue83-wechat-stub.log`（根目录、多半写失败），而 mjs 读 `/tmp/issue83-wechat-stub.log`，取证侧 catch
后拿到空字符串，回调证据静默丢失；TMPDIR 未设时 `set -u` 直接退出。下一行 alipay 日志同理。建议统一 `${TMPDIR:-/tmp}/issue83-*.log`。

-   nohup python3 "$EV/wechat_native_stub.py" >> "${TMPDIR}issue83-wechat-stub.log" 2>&1 &
+   nohup python3 "$EV/wechat_native_stub.py" >> "${TMPDIR:-/tmp}/issue83-wechat-stub.log" 2>&1 &
+   # alipay 行同步改为 "${TMPDIR:-/tmp}/issue83-alipay-stub.log"


─── docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py:285-289 ───
[bug · low] ThreadingHTTPServer 每请求一线程，而 ORDERS 是无锁的模块级共享 dict：close 端点是 check-then-act（先读
state=="SUCCESS" 再写 "CLOSED"），与并发到达的 /stub/mark 交错时，可能把已被标记 SUCCESS 的订单覆盖回 CLOSED 并返回 204 ——
这恰好会污染本工具要取证的 #83 close-race 形状（应为 400 ORDER_PAID）。虽然当前 mjs 驱动按顺序 await，但后端的 Query/Close 恢复调用与驱动侧
mark/orders 轮询天然可能并发到达。建议加一把 threading.Lock 保护 ORDERS 的读-改-写（mark/close/notify 路径）。

+ # 模块级：ORDERS_LOCK = threading.Lock()
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


─── docs/plans/issue-72-flow-evidence-83/seed-login-f.json:0-0 ───
[security · high] 内容与 seed-login-a.json 逐字节相同（seed_83.sh 运行期 cp 产物），同样携带可用 JWT access/refresh token
字面量，违反「源码、示例和测试不得写入可用的凭据字面量」约束。建议：连同 a-e 一并 git rm --cached 并 .gitignore seed-login-*.json；下游
api_recovery_83.mjs 对 seed-login-f.json 的依赖可在运行时由 seed_83.sh 现场生成，无需入库。



─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:186-189 ───
[security · medium] FLOW83_TENANT 的数字校验「只记录不中止」：note('tenant-env', false, ...) 之后流程继续，未通过校验的 TENANT
随即在下方 lagoCustomer() 中以 `weknora-tenant-${TENANT}` 拼进 docker exec psql 的 SQL（where
c.external_id='...'），恶意 env 值可注入额外语句读写/篡改 Lago 数据库，白名单校验形同虚设。建议与入口处 EMAIL/PASSWORD 校验保持一致
fail-fast，确保未通过校验的值永不进入 SQL 拼接。

    const TENANT = process.env.FLOW83_TENANT ?? '';
    if (!/^\d+$/.test(TENANT)) {
-     note('tenant-env', false, `FLOW83_TENANT must be the protagonist's numeric tenant id (got '${TENANT}')`);
+     console.error(`FLOW83_TENANT must be the protagonist's numeric tenant id (got '${TENANT}')`);
+     process.exit(2);
    }


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:70-71 ───
[security · medium] grant 与 count 两条 SQL 均以 '$UID_B' 字符串拼接执行，违反验收约束「数据库查询一律使用参数绑定，禁止用拼接、format 或
f-string 组装 SQL」；uuid_shape 白名单目前缓解了注入，但校验与拼接在代码上分离，后续复制此模式或新增插值字段时保护即失效。sqlite3 CLI
不支持绑定参数，建议至少将「uuid_shape 校验 + SQL 执行」收敛进同一函数强制耦合（校验失败即 return 1），或改走后端管理 API 授予 plan_publish 避免直写库。



─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:124-124 ───
[bug · medium] checkout 重试第 4 次（attempt===3）失败后仅 note(false) 即继续主流程，紧接着的
waitForSelector('a[href^="weixin://wxpay/"]') 必然在 60s 后超时抛错，掩盖真实失败点且跳过其后全部断言。建议最终失败时立即抛出带上下文的错误（配合外层
catch 落盘 RESULT/progression），让失败原因停在 checkout 阶段。

-     if (attempt === 3) note('checkout-awaiting-payment', false, 'order never landed (see backend log)');
+     if (attempt === 3) {
+       note('checkout-awaiting-payment', false, 'order never landed (see backend log)');
+       throw new Error('checkout never reached awaiting-payment after 4 attempts');
+     }


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:246-249 ───
[bug · medium] 主流程 try 只有 finally 没有 catch：任何 waitForSelector 超时或异常都会带非零码直接逃逸，末尾 console.log('RESULT
...') 与 purchase-state-progression.txt 写盘（finally 内未包含）均不会执行，取证时丢失已积累的 PASS/FAIL 明细与状态轨迹。同时
stubOrders/stubMark/stubNotify/purchaseState 四个 fetch 均无 try/catch，stub 或后端不可用时以未处理 rejection 崩溃。建议补
catch 记录异常后仍写 progression 与 RESULT，再按 results 决定退出码。

+ } catch (err) {
+   note('uncaught-flow-error', false, String(err?.message ?? err));
  } finally {
    await browser.close();
  }
  console.log('RESULT ' + JSON.stringify(results));


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:184-185 ───
[maintainability · medium] settle 段业务常量硬编码且无 env 覆盖：ORG UUID、下方 lagoCustomer() 内的 docker 容器名
weknora-lago-82flow-db-1、execFileSync 参数中的 Lago base http://127.0.0.1:48889，以及跨轮次目录依赖
../issue-72-flow-evidence-82/.../deliver_stripe_webhook.py；WEB/STUB/BACKEND 均支持 env 覆盖而它们不支持，换一套
Lago 栈复用脚本即静默失败（且 ORG 与 api_recovery_83.mjs:128 重复定义）。建议提升为 FLOW83_ORG / FLOW83_LAGO_DB /
FLOW83_LAGO_BASE / FLOW83_DELIVER 环境变量。



─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:152-154 ───
[bug · low] 以 keys[keys.length - 1] 依赖 JSON 键插入序隐式假设选取「本轮」订单：stub 的 ORDERS 值只有
{total,state}（wechat_native_stub.py:267）无时间戳字段，且该 stub 被多 act 共用（同一 :8296 实例），若残留旧单或并发他单，会对错误订单执行
mark SUCCESS + 签名 notify，污染证据甚至误推进他单状态。建议：点击切换按钮前快照 stubOrders() 键集合、切换后取差集新键；或让 stub 为每单记录
created_at 供选取。



─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:81-83 ───
[maintainability · low] FLOW83_TOKEN 缺失时 purchaseState() 返回占位 state 'env-missing-token'，不
fail-fast：settle 轮询永远等不到 active，仍空转 24 次 × 10s（约 4 分钟）才以 settle-webhook-active 失败收场，浪费时间且失败原因不直观。该
env 实为链路必需（active 判定依赖它），建议与 EMAIL/PASSWORD 一样在入口校验并 process.exit(2)。



─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:92-92 ───
[maintainability · low] `jq ... | say "... $(cat)"` 依赖「命令替换在参数展开时吞掉管道 stdin」的隐蔽时序才拿到 jq
输出，可读性差且脆弱（say 一旦改为读取 stdin、或片段被拷到非 bash 环境即失效）；该写法沿袭自 82 轮三个 seed.sh。建议改为直接命令替换，让数据流显式可见。

- jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json" | say "lago 9900 plans: $(cat)"
+ say "lago 9900 plans: $(jq -c '[.plans[] | select(.amount_cents==9900) | {code, amount_cents, interval}]' "$EV/api-03-lago-plans.json")"


─── internal/modules/commercial/service/commercial/order.go:598-601 ───
[bug · medium] landClosed 的两次落地更新（MarkAttemptClosed + MarkChannelFailed）没有包在同一个事务里：若第一条 UPDATE
落地后进程崩溃或第二条遇到瞬时 DB 故障，会留下"attempt 已 closed 但订单仍 payable（channel_failed=false 且 checkout_url
非空）"的半状态。该状态没有任何 API 恢复路径：Purchase 的渠道切换分支需要 pending attempt 才能进入（此时已无），#82 冻结重放分支会一直把已关闭渠道的死链
CheckoutURL 返给租户；SweepStaleLinklessPending 只清扫空 URL 的行；CloseChannelOrder
无其他生产调用方。结果是租户购买被永久卡死，只能人工改库。建议在仓储层提供单一事务方法，将 attempt 关闭与 channel_failed 标记原子落地。

- 	landClosed := func() (OrderView, error) {
- 		if err := s.orders.MarkAttemptClosed(ctx, orderID); err != nil {
- 			return OrderView{}, err
+ // OrderStore 新增（repository/commercial/order.go），landClosed 改调它：
+ func (s *OrderStore) MarkChannelClosed(ctx context.Context, orderID string) error {
+ 	if orderID == "" {
+ 		return ErrInvalidOrderRow
+ 	}
+ 	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
+ 		if err := tx.Model(&PaymentAttemptRow{}).
+ 			Where("order_id = ? AND state = ?", orderID, PaymentAttemptStatePending).
+ 			Update("state", PaymentAttemptStateClosed).Error; err != nil {
+ 			return err
+ 		}
+ 		return tx.Model(&OrderRow{}).
+ 			Where("id = ? AND state = ?", orderID, domain.OrderStatePending).
+ 			Update("channel_failed", true).Error
+ 	})
- 		}
+ }


─── internal/modules/commercial/service/commercial/order.go:602-607 ───
[bug · low] 此处 MarkChannelFailed 在与并发确认（回调/RecoverOrderStatus 的 ConfirmPayment）赛跑输掉时，WHERE
state=pending 命中 0 行，返回 ErrOrderNotFound——支付事实其实已成功落地，调用方（Purchase）却收到"订单不存在"的误导性哨兵错误（外层可能映射为
404）。MarkAttemptClosed 的注释声称"与确认赛跑是 no-op"，但紧随其后的 MarkChannelFailed 并非 no-op。重试可自愈（下次进入走非 pending
早退分支），但错误契约失真。建议 RowsAffected==0 时按当前状态重读作答，或让仓储返回可区分的哨兵由这里转译。

  		if err := s.orders.MarkChannelFailed(ctx, orderID); err != nil {
+ 			if errors.Is(err, repocommercial.ErrOrderNotFound) {
+ 				// 与并发确认赛跑：资金事实已落地，按当前状态作答而非误报 not found
+ 				after, rerr := s.orders.GetOrder(ctx, orderID)
+ 				if rerr != nil {
+ 					return OrderView{}, rerr
+ 				}
+ 				return OrderView{ID: after.ID, QuoteID: after.QuoteID, State: after.State,
+ 					AmountFen: after.AmountFen, Currency: after.Currency, Provider: att.Provider,
+ 					Version: after.Version}, nil
+ 			}
  			return OrderView{}, err
  		}
- 		return OrderView{ID: row.ID, QuoteID: row.QuoteID, State: domain.OrderStatePending,
- 			AmountFen: row.AmountFen, Currency: row.Currency, Provider: att.Provider,
- 			Version: row.Version}, nil


─── internal/modules/commercial/service/commercial/order.go:623-627 ───
[bug · medium] txn := res.ProviderID 这条推导对 provider 的隐含契约是"Query 的 ProviderID 必须与该渠道回调事实的
Transaction 同源（渠道交易号）"。wechat 在本次 diff 中已对齐（transaction_id 优先），但 alipay.Query 仍返回
out.OutTradeNo（alipay.go:472-476），而 alipay 回调事实携带的是 tradeNo（alipay.go:342）。于是 alipay 腿上"先经本路径（或
RecoverOrderStatus）恢复、后回调重放同一笔支付"时，ConfirmPayment 的 sameTxn 判定失败：provider_transaction_id 被改写、误发一条
over-payment 审计事件——这正是本次要修的 #83 duplicate-fact-unity 缺陷在 alipay 渠道上的残留。建议同步把 alipay.Query 改为优先返回
out.TradeNo（缺失再回退 out_trade_no）。

  	case payment.StateSucceeded:
  		txn := res.ProviderID
  		if txn == "" {
  			txn = att.MerchantOrderID
  		}
+ 		// 前置条件：provider.Query 的 ProviderID 必须与该渠道回调事实的
+ 		// Transaction 同源（渠道交易号）。wechat 已满足；alipay.Query 目前
+ 		// 仍返回 out_trade_no，需同步改为 id := out.TradeNo 优先，否则
+ 		// alipay 腿的 sameTxn 幂等判定仍会被同一笔支付击穿（#83 残留）。


─── internal/modules/commercial/service/commercial/purchase.go:273-275 ───
[bug · medium] 只识别 closeView.State == OrderStatePaid 过于狭窄：succeeded 分支里 ConfirmPayment 落库 paid
后、GetOrder 重读前，fulfill outbox worker 可将订单推进为 fulfilled（MarkFulfilled，order.go:714-719）。此时
closeView.State == fulfilled 不会被拦截，代码落穿到 CreateOrder(quoteID,
providerName)，为一份已支付生效的购买再开一张新渠道订单，存在重复收款风险（CreateOrder 前的 p.State 是关单前读的陈旧快照，拦不住）。建议改为排除式判定：只要不是
pending（订单已决出终局），一律按 closeView 作答、不再开新单。

- 			if closeView.State == domain.OrderStatePaid {
+ 			if closeView.State != domain.OrderStatePending {
+ 				// 渠道关单仲裁出已决状态（paid，极端竞态下 fulfilled）：
+ 				// 按已决订单作答，绝不再开第二张渠道订单
  				return s.purchaseView(p, snap, pub, &closeView), nil
  			}


─── internal/modules/commercial/service/commercial/purchase.go:268-268 ───
[bug · medium] aerr == nil && att.Provider != providerName 的写法会把 FirstPendingAttempt 返回的非 NotFound
错误（连接故障、超时等瞬时 DB 错误，order.go:483 原样透传）静默归入 else 分支：失败的查询被转换成成功响应，按 #82 冻结语义把旧渠道的 CheckoutURL
当作答案返给调用方。这掩盖了基础设施故障，且用户明确要求切换到新渠道却被留在旧渠道入口。应把 ErrPaymentAttemptNotFound（冻结重放）与其他 DB 错误（上抛）区分开。

- 		if att, aerr := s.orders.orders.FirstPendingAttempt(ctx, existing.ID); aerr == nil && att.Provider != providerName {
+ 		att, aerr := s.orders.orders.FirstPendingAttempt(ctx, existing.ID)
+ 		switch {
+ 		case aerr == nil && att.Provider != providerName:
+ 			// 渠道切换：先安全退掉旧渠道条目
+ 		case errors.Is(aerr, repocommercial.ErrPaymentAttemptNotFound):
+ 			ov := orderViewFromRow(existing)
+ 			return s.purchaseView(p, snap, pub, &ov), nil // #82 冻结语义
+ 		case aerr != nil:
+ 			return PurchaseView{}, aerr // 瞬时 DB 故障不得伪装成成功重放
+ 		default:
+ 			ov := orderViewFromRow(existing)
+ 			return s.purchaseView(p, snap, pub, &ov), nil // 同渠道重放
+ 		}


─── internal/modules/commercial/service/commercial/order.go:616-621 ───
[maintainability · low] 双腿失败分支 return OrderView{}, qerr 丢掉了原始 close 错误 cerr：调用方只能看到 query 失败原因，无法用
errors.Is 区分/追溯 close 阶段的故障（例如 close 是 ORDER_PAID 还是网络错误，直接影响排障与重试策略判断）。建议用 errors.Join
同时保留两条错误链（两者均可被 errors.Is 命中）。

  	res, qerr := provider.Query(ctx, att.MerchantOrderID)
  	if qerr != nil {
- 		// Both legs failed: the outcome stays unknown — surface it, mark
- 		// nothing, keep the payable entry for a retry.
- 		return OrderView{}, qerr
+ 		// Both legs failed: the outcome stays unknown — surface BOTH causes,
+ 		// mark nothing, keep the payable entry for a retry.
+ 		return OrderView{}, errors.Join(cerr, qerr)
  	}


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:93-94 ───
[test · medium] 「assert publication landed in Lago」这一段断言空转：文件头注释自述 pro:1 已在早前轮次发布到同一 82flow Lago 栈，而
`length >= 1` 对任意存量 9900 plan 恒为真——即使本轮 publish 从未到达 Lago，该检查照样 PASS。WeKnora 侧的 publish receipt
只能证明本方出账成功，Lago 落库这一步实际没有任何针对本轮的有效断言。建议：发布前先对 GET /api/v1/plans 做快照，发布后按 code（如 `pro:1`）精确匹配并校验
amount_cents/interval 与本轮 draft 一致（或对比快照差异），确保断言确实覆盖本轮发布。

- jq -e '[.plans[] | select(.amount_cents==9900)] | length >= 1' "$EV/api-03-lago-plans.json" >/dev/null \
-   || { say "FAIL: no 9900 plan visible in Lago"; exit 1; }
+ jq -e '[.plans[] | select(.code=="pro:1" and .amount_cents==9900)] | length == 1' "$EV/api-03-lago-plans.json" >/dev/null \
+   || { say "FAIL: pro:1 not visible in Lago for this round"; exit 1; }


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:57-57 ───
[maintainability · low] TOKEN_A 提取后全程未被使用（后续只用到
TOKEN_B/UID_B/TENANT_A/TENANT_B），属于死变量。要么删除；要么把它派上用场——browser_flow_83.mjs 需要主角的
FLOW83_TOKEN/FLOW83_TENANT，可在 seed-run.txt 里以机器可读行输出（如
`FLOW83_TOKEN=$TOKEN_A`、`FLOW83_TENANT=$TENANT_A`），避免操作者再手工从 seed-login-a.json 抠值。

  TOKEN_A=$(jq -r .token "$EV/seed-login-a.json")
+ say "FLOW83_TOKEN=$TOKEN_A"   # 供 browser_flow_83.mjs 使用的机器可读输出


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:23-25 ───
[bug · low] reg_expect 的 case 只能覆盖 HTTP 状态码，传输层失败根本到不了那里：后端未启动时 curl 连接失败（exit 7），`code=$(...)`
赋值语句随即非零，`set -e` 让脚本无任何 FAIL 输出直接终止；同理下方 LOGIN_A/LOGIN_B 的 curl、LAGO_KEY 的 `docker exec ... |
tr`（pipefail 下 docker 失败同样非零）都会静默中止，取证时无从判断卡在哪一步。建议这几处命令替换都补 `|| { say "FAIL: ..."; exit 1; }` 显式报错。

    code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
      -H 'Content-Type: application/json' \
-     -d "{\"username\":\"$2\",\"email\":\"$2\",\"password\":\"$PW\"}")
+     -d "{\"username\":\"$2\",\"email\":\"$2\",\"password\":\"$PW\"}") \
+     || { say "FAIL: $label register unreachable at $BACKEND (curl exit $?)"; exit 1; }


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:14-14 ───
[maintainability · low] 本脚本的 BACKEND、Lago base `http://127.0.0.1:48889`、容器名
`weknora-lago-82flow-db-1` 全部写死且无 env 覆盖（仅 DB_PATH 支持 FLOW83_DB），而同目录 browser_flow_83.mjs 对
WEB/STUB/BACKEND 均提供 FLOW83_* 覆盖——同一套证据脚本的配置口径不一致，换一套 Lago 栈或端口复用脚本就得改源码。建议统一为
`${FLOW83_BACKEND:-http://127.0.0.1:8095}`、`${FLOW83_LAGO_BASE:-...}` 等形式。

- BACKEND=http://127.0.0.1:8095
+ BACKEND="${FLOW83_BACKEND:-http://127.0.0.1:8095}"


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:163-164 ───
[test · low] stubMark 的返回值被完全丢弃：stub 的 /stub/mark 对未知 out_trade_no 返回 404 {"error":"unknown
out_trade_no"}（liveOrder 选错或为 '' 时即触发，见 wechat_native_stub.py:238-241），此时订单仍停在 NOTPAY，随后的 notify 会推送
trade_state=NOTPAY 的 TRANSACTION.SUCCESS——wechat.go 的 Verify 是按解密后的 trade_state 映射事实状态的，而 stubNotify
只断言 WeKnora 回了 200 ACK（pushed），所以真实失败（mark 404）不会在任何 note 中出现，只能等 60s 后
`waitForSelector('text=已付款，权益处理中')` 超时才暴露，取证时失败原因被掩盖。建议与 stubNotify 同口径断言并记入 note。

-   await stubMark(liveOrder, txnID);
+   const marked = await stubMark(liveOrder, txnID);
+   note('stub-mark-success', marked?.ok === true, `mark ${liveOrder} -> SUCCESS (txn ${txnID})`);
    const pushed = await stubNotify(liveOrder);


LLM retry report summary: 2 of 99 requests affected -- 2 requests failed

Core review (2 requests):
- docs/plans/issue-72-flow-evidence-83/account-after-wechat.json,docs/plans/issue-72-flow-evidence-83/api-01-draft.json,docs/plans/issue-72-flow-evidence-83/api-02-publish.json,docs/plans/issue-72-flow-evidence-83/api-03-lago-plans.json,docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs: timed out -> failed
- internal/modules/commercial/payment/wechat.go,internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go: timed out -> failed

Per-attempt detail: --format json (retry_report).
