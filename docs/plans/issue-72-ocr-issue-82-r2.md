Review complete: 101 finding(s) across 99 selected item(s).

─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:35-35 ───
[security · high] 登录口令 'issue82-Flow-Pw-b'
以明文字面量写入脚本，直接违反本分支安全验收红线「凭据只从环境变量读取，源码、示例和测试不得写入可用的凭据字面量」（OCR round-1/3/4 已在 r4-flow
系列脚本中指出同类问题，此处漏修）。同时上方已强制校验的 PASSWORD
变量（FLOW82_PASSWORD_B/FLOW82_PASSWORD）从未被使用，构成声明未读的死代码：脚本要求必须设置该环境变量却在登录时忽略它，换环境运行时口令不一致会导致登录失败且难以排查。
应改为填充 PASSWORD。

-   await page.fill('#auth-password', 'issue82-Flow-Pw-b');
+   await page.fill('#auth-password', PASSWORD);


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:13-16 ───
[maintainability · low] import { fileURLToPath } 声明位于使用点（const EV = fileURLToPath(...)）之后，仅靠 ESM
静态提升才不报错，风格误导且在引入 import/first 类 lint 或重构为 CJS 时会踩坑。建议将 import 移至文件顶部（与 browser_flow_82.mjs 的顺序一致）。

+ import { fileURLToPath } from 'node:url';
+ const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5192';
  const EV = fileURLToPath(new URL('.', import.meta.url)) + 'reverify1/';
  const ORDER = process.argv[2];
  if (!ORDER) { console.error('usage: node browser_paid_face_82.mjs <orderId>'); process.exit(2); }
- import { fileURLToPath } from 'node:url';


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:12-15 ───
[maintainability · low] 与 browser_paid_face_82.mjs 同一问题：import { fileURLToPath } 位于使用点之后，仅靠 ESM
静态提升生效。建议移至文件顶部，与其余脚本保持一致。

+ import { fileURLToPath } from 'node:url';
+ const WEB = process.env.FLOW82_WEB ?? 'http://localhost:5192';
  const EV = fileURLToPath(new URL('.', import.meta.url));
  const ORDER = process.argv[2];
  if (!ORDER) { console.error('usage: node browser_sync_face_82.mjs <orderId>'); process.exit(2); }
- import { fileURLToPath } from 'node:url';


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:24-24 ───
[maintainability · low] 业务金额默认值 '¥99.00' 属于业务数字硬编码，与种子数据/报价脚本（seed.sh、api-03-quote-*.json 中的
99.00）形成两处独立维护点：报价金额调整而未设置 FLOW82_EXPECT_CNY 时，断言会按旧默认值校验导致失败且原因隐蔽。脚本对凭据已采用
env-REQUIRED（无源码回退）策略，建议金额口径保持一致，改为必填环境变量并在缺失时报错退出。

- const EXPECT_CNY = process.env.FLOW82_EXPECT_CNY ?? '¥99.00';
+ const EXPECT_CNY = process.env.FLOW82_EXPECT_CNY ?? '';
+ if (!EXPECT_CNY) {
+   console.error('missing required env: FLOW82_EXPECT_CNY (no source-code fallback)');
+   process.exit(2);
+ }


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:38-41 ───
[maintainability · low] 三个顶层脚本（browser_flow_82 / browser_paid_face_82 /
browser_sync_face_82）各自复制了完全相同的登录流程、note/results 记录器与浏览器生命周期脚手架，且与 r4-flow*/reverify 目录下的副本合计 15
处（检索 form[aria-label="Login form"] 可见）。登录选择器或流程一旦变更需多处同步修改，极易漂移。建议在本证据目录抽取共享 helper（如
loginAndRecord.mjs），顶层脚本复用；历史 r4-flow*/reverify 快照作为冻结证据可保持原样。



─── docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py:26-26 ───
[documentation · low] 此行修改后与下一行旧文案（"payment_provider_unconfigured (the SSRF gate refusing the
loopback base)."）直接拼接，句子不通且新旧行为描述互相矛盾：新文案说会返回 202 + channel-failed (checkout_error) 订单，旧残留又说 503
payment_provider_unconfigured。这会误导复现 #81 流程的操作者判断无 SSRF 豁免时的实际行为。建议删除残留的旧表述，例如收尾为 "...without it the
purchase answers 202 with a channel-failed (checkout_error) order (the R3-27 fixed chain): the SSRF
gate refuses the loopback base."。

- before starting the WeKnora backend; without it the purchase answers 202 with a channel-failed (checkout_error) order (the R3-27 fixed chain)
+ before starting the WeKnora backend; without it the purchase answers 202 with a
+ channel-failed (checkout_error) order (the R3-27 fixed chain): the SSRF gate
+ refuses the loopback base.


─── docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py:28-28 ───
[maintainability · low] 端口 8291 硬编码且无环境变量覆盖，而同一提交的 alipay_gateway_stub.py 明确注明 "parallel
verification rounds drift the loopback ports" 并提供了 FLOW82_ALIPAY_PORT 覆盖（且本注释自称是 #81 stub 的 verbatim
reuse，#81 stub 同样占用 8291）。并行运行两轮验证（或与 #81 的 stub 并存）时微信 stub 将端口冲突。建议对齐提供 FLOW82_WECHAT_PORT 覆盖，默认保持
8291。

- HOST, PORT = "127.0.0.1", 8291
+ HOST, PORT = "127.0.0.1", int(os.environ.get("FLOW82_WECHAT_PORT", "8291"))


─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:98-103 ───
[maintainability · low] except Exception 过宽：密钥文件缺失/不可读（FileNotFoundError）、openssl
调用失败（CalledProcessError）等配置错误也会被归为"签名无效"并返回 False，调用方只会打出 "REJECTED signature ...
sign_ok=False"。排障时容易把 KEY_DIR 配错误判为签名规则不匹配。建议仅在验签路径上捕获具体异常并让配置类错误抛出，或至少在捕获时打印异常类别。

      try:
-         sig = base64.b64decode(sign)
+         sig = base64.b64decode(sign, validate=True)
+     except (ValueError, TypeError):
+         return False
-         return rsa_verify_sha256(SHARED_PUB,
+     return rsa_verify_sha256(SHARED_PUB,
-                                  request_sign_content(params).encode(), sig)
+                              request_sign_content(params).encode(), sig)
-     except Exception:
-         return False


─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:87-89 ───
[maintainability · low] finally 块内的局部 import os 与模块顶部的 import os 冗余（模块顶部已 import
os），属于无意义的遮蔽导入。直接使用模块级导入即可。

      finally:
-         import os
          os.unlink(sig_path)


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:76-76 ───
[maintainability · medium] 死代码：全仓检索确认 `window.__r4PurchasesSeen` 仅在本脚本出现（前端代码未注入该全局变量），此
waitForFunction 每次运行必然在 1s 后超时并被 `.catch(() => {})` 静默吞掉——固定空耗 1 秒，且让读者误以为 wire
断言存在页面侧插桩前置。前轮审查（issue-72-ocr-issue-82-r1.md）已标记此问题，后续重跑版本 r4-flow2/browser_01_checkout.mjs
也已删除该行，此处应同步删除。

-   await page.waitForFunction(() => window.__r4PurchasesSeen > 0, null, { timeout: 1000 }).catch(() => {});
+   // wire 断言直接基于 page.on('request') 捕获的 purchases 数组，无需页面侧插桩。
+   const providerOk = purchases.length >= 1 && purchases.every((p) => p.provider === 'alipay');


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:90-94 ───
[test · medium] 本腿只有 try/finally 没有 catch：一旦登录被拒或任一 waitForSelector
超时（取证脚本最需要捕获的失败形态），异常直接抛到顶层，`console.log('RESULT ...')` 与 `process.exit` 不会执行，下游读者看到的是"零证据行"而非可区分的
script-error FAIL——与本组 browser_02/04 注释中明确的 A-03 约定相悖（后续 r4-flow2 同款脚本已补齐 catch）。建议补上与 02/04 一致的
catch 块，并将 `page` 提升为 try 外的 `let page;` 以便 catch 中截图。

-   writeFileSync(`${EV}order-info.json`, JSON.stringify({ orderId, checkoutHref, purchases }, null, 2));
-   console.log('ORDER ' + orderId);
+ } catch (err) {
+   // (A-03) 基础设施失败 → 显式 FAIL note，保证 RESULT 汇总打印、退出码非零。
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}02-billing-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
  } finally {
    await browser.close();
  }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_03_paid_face.mjs:55-58 ───
[test · medium] 本腿与 browser_01 同样只有 try/finally 没有 catch：登录被拒或选择器超时会直接抛异常，`RESULT` 汇总不打印、退出前零证据行，与
browser_02/04 的 A-03 约定不一致（后续 r4-flow3 同款脚本已补齐）。建议补上 catch 块，并将 `page` 提升为 try 外的 `let page;` 以便
catch 中截图。

-   await page.screenshot({ path: `${EV}05-billing-paid-awaiting-activation.png`, fullPage: true });
+ } catch (err) {
+   // (A-03) 基础设施失败 → 显式 FAIL note，保证 RESULT 汇总打印、退出码非零。
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}05-billing-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
  } finally {
    await browser.close();
  }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:44-48 ───
[maintainability · medium] 重复样板（同样适用于本组 browser_02/03/04，且 r4-flow2/r4-flow3 又复制了 8 份）：四个腿重复约 40
行公共代码——createRequire 引 Playwright、WEB/EV/EMAIL/PASSWORD 环境读取与校验、note/results 助手、登录填表序列、RESULT
汇总与退出码收尾；路由（/platform/billing/checkout、/api/v1/commercial/purchases）与 45000/60000/3500/1200ms
超时字面量也散落各处。重复已实际造成漂移（仅 02/04 获得 A-03 catch、01 遗留 __r4PurchasesSeen 死等待）。建议抽取共享模块（如 r4-flow/lib.mjs
导出 login/note/runLeg 与路由/超时常量），后续每轮复核只改一处。

-   await page.goto(`${WEB}/login`);
-   await page.fill('#auth-email', EMAIL);
-   await page.fill('#auth-password', PASSWORD);
-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
-   await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });
+ // lib.mjs: export const LOGIN = { emailSel: '#auth-email', pwdSel: '#auth-password',
+ //   submitSel: 'form[aria-label="Login form"] button[type="submit"]', timeoutMs: 45000 };
+ // export async function login(page, { web, email, password }) { ... }
+ // 各腿改为: await login(page, ctx);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:73-73 ───
[test · low] 空值防护缺失：`getAttribute('href')` 在链接元素缺 href 属性时返回 null，此处未校验即按正常路径写入
order-info.json（该文件是留给后续腿接力的产物），null 会静默传导且不产生任何 FAIL note；另外第 78 行 `p.quote_id?.slice(0, 12) + '…'`
在 quote_id 缺失时会打印 "undefined…"，污染证据明细（应改为 `p.quote_id ? p.quote_id.slice(0, 12) + '…' :
'(none)'`）。建议补一条显式 href 断言。

    const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
+   note('pay-link-href-valid', typeof checkoutHref === 'string' && /^https?:\/\//.test(checkoutHref), String(checkoutHref));


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:40-40 ───
[maintainability · low] 随 runs/ 回退逻辑（matches = sorted(glob.glob(...)) 及 elif matches 分支）被移除，文件顶部的
`import glob` 与 `RUNS = Path(__file__).resolve().parents[3] /
"deploy/lago-lab/payment-activation/runs"`
在本文件已无任何代码引用（大小写敏感搜索确认仅剩定义与注释文字），成为死导入/死变量，建议随本次改动一并删除，避免误导后续维护者以为仍有 runs/ 探测路径。



─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:26-27 ───
[documentation · low] 紧邻本注释块上方的旧注释末句（"…the pick was never printed). Fall back to the newest runs/
TSV only when no archive exists, and always print the file actually used."）仍描述本次已删除的回退行为，与下方"无归档即
WARNING + exit(2)、不再回退"的新逻辑直接矛盾，易让维护者误以为回退路径仍然存在；建议同步删改该句。



─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:47-47 ───
[maintainability · low] 随 runs/ 回退逻辑（matches = sorted(glob.glob(...)) 及 elif matches 分支）被移除，文件顶部的
`import glob` 与 `RUNS = Path(__file__).resolve().parents[3] /
"deploy/lago-lab/payment-activation/runs"`
在本文件已无任何代码引用（大小写敏感搜索确认仅剩定义与注释文字），成为死导入/死变量，建议随本次改动一并删除，避免误导后续维护者以为仍有 runs/ 探测路径。



─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:33-34 ───
[documentation · low] 紧邻本注释块上方的旧注释末句（"…the pick was never printed). Fall back to the newest runs/
TSV only when no archive exists, and always print the file actually used."）仍描述本次已删除的回退行为，与下方"无归档即
WARNING + exit(2)、不再回退"的新逻辑直接矛盾，易让维护者误以为回退路径仍然存在；建议同步删改该句。



─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:44-44 ───
[maintainability · low] 随 runs/ 回退逻辑（matches = sorted(glob.glob(...)) 及 elif matches 分支）被移除，文件顶部的
`import glob` 与 `RUNS = Path(__file__).resolve().parents[3] /
"deploy/lago-lab/payment-activation/runs"`
在本文件已无任何代码引用（大小写敏感搜索确认仅剩定义与注释文字），成为死导入/死变量，建议随本次改动一并删除，避免误导后续维护者以为仍有 runs/ 探测路径。



─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:30-31 ───
[documentation · low] 紧邻本注释块上方的旧注释末句（"…the pick was never printed). Fall back to the newest runs/
TSV only when no archive exists, and always print the file actually used."）仍描述本次已删除的回退行为，与下方"无归档即
WARNING + exit(2)、不再回退"的新逻辑直接矛盾，易让维护者误以为回退路径仍然存在；建议同步删改该句。



─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:44-44 ───
[maintainability · low] 随 runs/ 回退逻辑（matches = sorted(glob.glob(...)) 及 elif matches 分支）被移除，文件顶部的
`import glob` 与 `RUNS = Path(__file__).resolve().parents[3] /
"deploy/lago-lab/payment-activation/runs"`
在本文件已无任何代码引用（大小写敏感搜索确认仅剩定义与注释文字），成为死导入/死变量，建议随本次改动一并删除，避免误导后续维护者以为仍有 runs/ 探测路径。



─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:30-31 ───
[documentation · low] 紧邻本注释块上方的旧注释末句（"…the pick was never printed). Fall back to the newest runs/
TSV only when no archive exists, and always print the file actually used."）仍描述本次已删除的回退行为，与下方"无归档即
WARNING + exit(2)、不再回退"的新逻辑直接矛盾，易让维护者误以为回退路径仍然存在；建议同步删改该句。



─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:88-89 ───
[bug · medium] DB_PATH 在整个脚本中从未定义、也没有像 FLOW82_R4_PW 那样用 :? 前置校验（README:69 虽记录了 `DB_PATH`
env，但脚本不自检）。在 set -u 下首次执行会在注册完 PAD+2 个租户之后才以 "unbound variable" 中止——副作用已发生且报错点远离根因；若 DB_PATH
指向不存在的路径，sqlite3 还会静默新建空库后报 no such table。建议在脚本入口（PW 校验旁）补 :? 守卫与文件存在性检查，保证 fail-fast 且在产生注册副作用之前失败。

- say "== grant plan_publish to B at platform scope (seed row) =="
- sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
+ # 与 FLOW82_R4_PW 一致的前置校验（放在脚本入口，注册副作用发生之前）
+ DB_PATH="${DB_PATH:?missing required env DB_PATH (:8093 backend sqlite db, e.g. data/issue82-r4verify.db)}"
+ [ -f "$DB_PATH" ] || { echo "DB_PATH file not found: $DB_PATH" >&2; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:66-69 ───
[security · medium] 脚本把含 token/refresh_token 的完整登录响应原样写入会随仓库提交的证据文件
seed-login-a/b.json；当前入库版本是手工脱敏的结果（"…<redacted>"），而每次重跑都会用真实 JWT 覆盖，根 .gitignore 不覆盖 docs/plans
路径，极易将可用凭据提交入库——与“凭据不得落入源码/证据”的安全验收条件相悖。issue-72-ocr-issue-82-r1.md:467 已给出 `jq 'del(.token,
.refresh_token)'` 修复但本脚本未采纳。建议落盘前脱敏，token 改从内存变量读取（TOKEN_A/TOKEN_B 的 jq 提取同步改为从 LOGIN_A/LOGIN_B
变量读取）。

- LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"); echo "$LOGIN_A" > "$EV/seed-login-a.json"
- LOGIN_B=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local"); echo "$LOGIN_B" > "$EV/seed-login-b.json"
- TOKEN_A=$(jq -r .token "$EV/seed-login-a.json")
- TOKEN_B=$(jq -r .token "$EV/seed-login-b.json")
+ LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local")
+ LOGIN_B=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local")
+ TOKEN_A=$(jq -r .token <<<"$LOGIN_A")
+ TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
+ # 证据文件脱敏后再落盘，重跑不会把真实凭据写回仓库
+ jq 'del(.token, .refresh_token)' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ jq 'del(.token, .refresh_token)' <<<"$LOGIN_B" > "$EV/seed-login-b.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:22-24 ───
[bug · low] reg()/login() 用手工转义把 $PW 拼进 JSON 字符串：FLOW82_R4_PW 一旦含双引号或反斜杠，-d 载荷会直接变成非法 JSON（注册/登录
400，且难以定位）；同时密码经 curl argv 可被本机 ps 观测，与“凭据仅经环境变量传递”的精神不完全一致。建议改用 jq -nc --arg 构造载荷（reg 与 login
两处同改），既修复转义边界也避免 argv 暴露。

    curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
      -H 'Content-Type: application/json' \
-     -d "{\"username\":\"$1\",\"email\":\"$1\",\"password\":\"$PW\"}"
+     --data "$(jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}')"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:16-17 ───
[bug · low] rails runner 的 stderr 被 2>/dev/null 吞掉：栈未启动（docker compose exec 非零退出，pipefail
下脚本静默中止、无任何输出）或 provider 行缺失（find_by 返回 nil，try(:webhook_secret).to_s 得空串、退出码 0）时，STORED 为空都会被当作“无存储
secret”，继续走铸造路径白耗 Stripe 16 个测试端点配额，最后才在 update! 步骤以晦涩的 NoMethodError 失败，根因不可见。建议保留 stderr 并显式断言
sentinel 到达，把“不可达”与“确无 secret”区分开。

- STORED=$(docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
-   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s" 2>/dev/null | sed -n 's/.*__R4SECRET__//p')
+ STORED_RAW=$(docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
+   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s") \
+   || { echo "lago api unreachable / rails runner failed (stderr above)" >&2; exit 1; }
+ case "$STORED_RAW" in *__R4SECRET__*) STORED="${STORED_RAW#__R4SECRET__}" ;; *) echo "runner output missing sentinel" >&2; exit 1 ;; esac


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:34-35 ───
[maintainability · low] prune 过滤条件是 description 含 "weknora" 即删：t11
lab（deploy/lago-lab/payment-settle-trigger/phases.py:322-326）在同一 Stripe 测试账户上创建的端点 description 同样以
"weknora t11 local settle probe …" 开头并被其 state 记录 endpoint_id 供自身 cleanup 删除，本脚本先跑一步会让 t11 的 cleanup
记为 failed（噪音性 FAIL 证据）。注释声称只清 "stale" 端点，但过滤器无法区分本 harness 的替身与其他环境仍需的端点。建议把匹配收窄到本脚本自己铸造时写入的描述串。

  for row in rows:
-     if "weknora" in str(row.get("description", "")):
+     # 只回收本 harness 自产的 r4 替身端点，避免误删 t11 等其他 lab 仍在跟踪的 weknora 端点
+     if "weknora r4 flow" in str(row.get("description", "")):


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:10-11 ───
[maintainability · low] COMPOSE_FILE 取自 $(pwd)，而本脚本（不像 seed.sh）没有 cd 到 worktree 根：从脚本所在目录或任意子目录执行时
COMPOSE_FILE 指向不存在的 compose.yaml，docker compose 以晦涩的 path not found 失败。建议与 seed.sh 一致，用脚本自身位置推导
worktree 根后再取路径。

+ cd "$(dirname "$0")/../../../.."   # 与 seed.sh 一致：从脚本自身定位 worktree 根，不依赖调用方 cwd
  PROJECT="weknora-lago-82flow"
  COMPOSE_FILE="$(pwd)/deploy/lago/compose.yaml"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:97-97 ───
[bug · low] LAGO_KEY 用 `select value from api_keys limit 1` 无 ORDER BY 任取首行：栈上存在多把 key 时取值不确定，拿到无权读
/plans 的旧 key 时只会在下游 BASE_N/N 的 jq 断言处以讪晦方式失败。docs/plans/issue-72-ocr-issue-82-r1.md:524-528
已记录同一问题及修复，本轮新增脚本仍复刻了旧写法；建议按 created_at 取最新一把，并在为空时显式报错。

- LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]')
+ LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')
+ [ -n "$LAGO_KEY" ] || { say "FAIL: no lago api key readable from weknora-lago-82flow-db-1"; exit 1; }


─── deploy/lago-lab/payment-settle-trigger/phases.py:837-837 ───
[bug · medium] phase_cleanup 违反自身 "every lab object created this run is deleted" 契约：phase_gated_3ds
通过 GraphQL addStripePaymentProvider 创建的 Lago 支付 provider（ctx.state["provider_code"]）从未被删除。已确认
deploy/lago-lab/ 全目录不存在任何 deletePaymentProvider/payment_providers/ 调用，cleanup 中也无 provider_code
消费点；每次 run 的前缀唯一（weknora-t11-{run_id[:8]}-stripe），多次运行将持续累积 provider 对象，且其 settings 中留存了通过 rails
runner 写入的 webhook_secret 和创建时提交的操作员 Stripe secretKey。evidence/t11-cleanup.json 也只记录 5 类对象、仍判 pass。若
v1.53 API 支持删除（GraphQL destroy mutation）则补上删除；若不支持，至少应在 objects 中显式记录该残留并同步调整 expected 措辞，不能静默泄漏。

+     provider_code = state.get("provider_code")
+     if provider_code:
+         try:
+             gstatus, _gbody = ctx.graphql(DESTROY_PAYMENT_PROVIDER_QUERY,
+                                           {"code": provider_code})
+             record("payment_provider", provider_code,
+                    "deleted" if _ok(gstatus) else "failed", gstatus)
+         except (OSError, clients.LabError):
+             record("payment_provider", provider_code, "failed", None)
+ 
      failures = [item for item in objects if item["outcome"] != "deleted"]


─── deploy/lago-lab/payment-settle-trigger/phases.py:314-316 ───
[security · medium] provider_code 以 f-string 内插进经 bin/rails runner 执行的 Ruby 代码（本函数 3
处、_compose_exec_env 调用侧同模式）。当前 provider_code 由 "{prefix}-stripe"（uuid
前缀）自生成，尚不可利用，但任何含单引号/双引号的取值（或未来把该值改为来自响应、配置的来源）都会破坏甚至注入该 Ruby 片段——这与仓库安全约束"禁止以拼接/f-string 组装代码与
SQL"的形态直接相悖。建议先用白名单校验再内插（或与 secret 一样经 ENV/stdin 传入后以 ENV['X'] 读取），三处一并收紧。

+         import re
+         if not re.fullmatch(r"[A-Za-z0-9_-]+", provider_code or ""):
+             return None
          value = self._runner_sentinel(
              "print PaymentProviders::StripeProvider.where(deleted_at: nil)"
              f".find_by(code: '{provider_code}').try(:webhook_secret).to_s")


─── deploy/lago-lab/payment-settle-trigger/phases.py:352-354 ───
[security · medium] WSECRET（真实 Stripe webhook 签名密钥 whsec_…）以 "-e WSECRET=<secret>" 形式进入 docker CLI 的
argv，在 exec 存续期间对本机 ps/进程列表可见，违背"凭据不经进程参数暴露"的惯例（也是 run_lab.py 密钥扫描所防御的同一类泄露面）。建议改为经 stdin
传入（subprocess.run(input=…)），Ruby 侧用 STDIN.read 取值替代 ENV['WSECRET']；顺带可去掉 env_vars 参数的 argv 组装循环。

-         command += ["-p", self.compose_project, "exec", "-T"]
-         for key, value in env_vars.items():
-             command += ["-e", f"{key}={value}"]
+         command += ["-p", self.compose_project, "exec", "-T", "api",
+                     "bin/rails", "runner", code]
+         # secret 经 stdin 传入（进程列表不可见）；Ruby 片段改读 STDIN
+         result = subprocess.run(command, input="\n".join(env_vars.values()),
+                                 capture_output=True, text=True, timeout=180)


─── deploy/lago-lab/payment-settle-trigger/phases.py:848-850 ───
[test · medium] 注释声称 PHASE_ORDER 与 runner 的 PHASE_SEQUENCE 对齐是 "(guarded by the runner import)"，但
run_lab.py 中不存在任何将两者对齐的断言，test_phases.py 也没有对应契约测试（姊妹实验室 payment-activation 在其 test_phases.py:1370 有
test_phase_order_contract_matches_runner_order，本目录的测试文件无此用例）。两份元组独立维护：未来仅在单侧增删阶段时，探测会静默漏跑并产出覆盖不完整的
"pass" 证据。建议在 run_lab.py 由 PHASE_ORDER 派生 PHASE_SEQUENCE（单一事实来源），并/或补上与姊妹实验室相同的顺序契约测试。

- # this tuple (guarded by the runner import).
+ # this tuple (guarded by test_phase_order_contract_matches_runner_order
+ # in test_phases.py).
  PHASE_ORDER = (
      phase_gated_3ds,


─── deploy/lago-lab/payment-settle-trigger/lab.sh:7-7 ───
[documentation · low] 头部注释自 t10 骨架（旧路径 deploy/lago-lab/payment-trigger/，见 issue-72-plan-82.md:115 的
git show 恢复命令）复制后未同步：Usage 行路径少写 "settle-"，且 usage() 会 sed 打印本文件 2-18 行，错误路径直接进入 --help 输出；同时第 15-16
行 "all state lives under the weknora-lago-82 Compose project (volumes weknora-lago-82_lago_*)" 与实际
readonly COMPOSE_PROJECT="weknora-lago-t11" 矛盾，误导运维排查卷与项目归属。

- # Usage: ./deploy/lago-lab/payment-trigger/lab.sh init|up|down|status|config
+ # Usage: ./deploy/lago-lab/payment-settle-trigger/lab.sh init|up|down|status|config


─── deploy/lago-lab/payment-settle-trigger/run_lab.py:139-141 ───
[bug · low] Timeline.text() 每次调用都向 self.lines 追加一行 "run finished"（副作用），而 run_experiment
在密钥扫描前后各调用一次，导致最终 t11-run.txt 出现两条 "run finished"（已提交证据 t11-run.txt 第 14、16 行实证），且第一条早于 secrets scan
日志行，时间线失真。建议让 text() 无副作用、结尾标记由调用方显式 log 一次。

      def text(self):
-         self.lines.append(f"[{utc_now()}] run finished")
+         # 无副作用：调用方在结尾显式 log("run finished")，避免多次
+         # 写文件时重复追加终结行
          return "\n".join(self.lines) + "\n"


─── apps/web/src/commercial/CheckoutPage.tsx:197-198 ───
[bug · medium] await 之后的两个副作用缺少 active/isCurrent 守卫：(1) `orderIdRef.current = order.id` 是全 run()
中唯一无守卫的 ref 写入——作用域切换（tenantId 变化触发 effect 重跑）时，旧 run 已到达但未及处理的 purchase 响应会把旧租户的订单 id 写回 ref，新 run
随即按「本页唯一订单」对跨租户订单 id 发起 getOrder，必然失败且文案误导；(2) 这里的 error setState 同样无守卫，旧 run 迟到的错误态可能覆盖新 run 的
loading 态。相邻的 ready setState 有完整守卫，建议对齐。

+         if (!purchase.order) {
+           if (active && scopeController.isCurrent(currentScope.scope)) {
-           setState({ status: 'error', message: purchaseErrorMessage(purchase) });
+             setState({ status: 'error', message: purchaseErrorMessage(purchase) });
+           }
            return;
+         }
+         const order = purchase.order;
+         if (active && scopeController.isCurrent(currentScope.scope)) {
+           orderIdRef.current = order.id;
+           setState({ status: 'ready', order, quote: quoteRef.current, purchase });
+         }


─── apps/web/src/commercial/CheckoutPage.tsx:224-224 ───
[bug · medium] 把 channel 加入 effect 依赖后，ready 态下每次切换支付渠道单选都会重跑整个加载流程：effect 开头的 `setState({ status:
'loading' })` 让页面先塌缩成「Loading checkout…」再回来，且每次多发起一次 getOrder + purchaseStatus 请求。而 channel
只在提交时刻（run() 内 purchase 调用）才被消费。建议用 ref 在提交时读取最新选择，将 channel 移出依赖数组。

-   }, [client, scopeController, retryToken, channel, scope.scope.tenantId]);
+   // channelRef 承载最新选择，单选切换不重跑加载 effect
+   const channelRef = useRef<PaymentChannel>(channel);
+   useEffect(() => { channelRef.current = channel; }, [channel]);
+   // run() 提交处：provider: channelRef.current
+   }, [client, scopeController, retryToken, scope.scope.tenantId]);


─── apps/web/src/commercial/CheckoutPage.tsx:150-150 ───
[bug · medium] submittedChannel 是客户端侧的猜测值：初始默认 'alipay'，而 OrderView（contracts L1-8）不携带渠道字段——通过
orderId 深链打开一个微信渠道创建的待付款订单时，submittedChannel 与订单真实渠道不符。此时切到微信单选会显示错误的「当前订单以 支付宝
创建」横幅，且「改用微信支付重新发起支付」会以相同渠道新建一张订单（无意义的重复下单）。建议：提交前 submittedChannel 置为 null（未知时不显示渠道失配横幅），或由后端在
OrderView 增补 provider 字段作为权威来源。

-   const [submittedChannel, setSubmittedChannel] = useState<PaymentChannel>('alipay');
+   // 仅在本次会话真正提交后才记录渠道；深链打开既有订单时不臆测渠道
+   const [submittedChannel, setSubmittedChannel] = useState<PaymentChannel | null>(null);
+   // 渲染处：state.order.payment === 'pending' && submittedChannel !== null && channel !== submittedChannel


─── apps/web/src/commercial/CheckoutPage.tsx:216-216 ───
[bug · medium] catch 中统一套用 purchaseErrorText 覆盖了 run() 的全部失败面：既有订单的 getOrder 查询失败（任何 ≥400 的
ApiError）也会渲染成「购买未能创建，请稍后重试」——但该路径根本没有创建动作，文案与事实相反；且下方重试按钮的「同一报价服务端幂等」对纯加载失败也是误导。建议为 getOrder
路径单独映射闭合文案（如「订单加载失败，请稍后重试」），仍符合 spec L210 的闭合词汇要求。

-         setState({ status: 'error', message: purchaseErrorText(error) });
+         // 既有订单的加载失败与购买创建失败分开映射
+         setState({ status: 'error', message: loadedExistingOrder ? '订单加载失败，请稍后重试' : purchaseErrorText(error) });


─── apps/web/src/commercial/CheckoutPage.tsx:246-246 ───
[bug · low] 本次重写了该分支的守卫（live()），但非 ready 时的错误文案仍是原始英文 `error.message` / 'Unable to load order'
直接展示给用户——与本变更在 purchaseErrorText 处贯彻的 spec L210（页面词汇稳定、不含平台词汇）目标不一致。建议同样收敛为闭合中文文案。

        if (!live()) return;
+       setState((prev) => (prev.status === 'ready'
+         ? prev
+         : { status: 'error', message: '订单加载失败，请稍后重试' }));


─── apps/web/src/commercial/CheckoutPage.tsx:33-36 ───
[style · low] purchaseStateMessage 的 default 分支是三层嵌套三元表达式（payment → payment →
fulfillment），违反检查清单「禁止嵌套三元」，且该状态机映射可读性差。建议改为 if 链或查表结构。

-     default:
-       return order.payment === 'pending' ? '待付款（权益未开通）'
-         : order.payment === 'paid' ? '已付款，权益处理中'
-         : order.fulfillment === 'fulfilled' ? '权益已生效' : orderMessage(order);
+     default: {
+       if (order.payment === 'pending') return '待付款（权益未开通）';
+       if (order.payment === 'paid') return '已付款，权益处理中';
+       if (order.fulfillment === 'fulfilled') return '权益已生效';
+       return orderMessage(order);
+     }


─── apps/web/src/commercial/CheckoutPage.tsx:341-342 ───
[maintainability · low] 上方 IIFE 刚计算过 checkoutHref，此处死胡同区块又重新计算一遍 `state.order.checkout_url &&
isSafeCheckoutUrl(...)`，与紧邻注释「checkoutHref 是唯一事实源，两处判断共用」自相矛盾。建议在 ready 分支顶部（`const order = ...`
旁）一次性计算 checkoutHref，两个区块共用同一变量。

-               {state.order.payment === 'pending'
-                 && !(state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url)) ? (
+   const order = state.status === 'ready' ? state.order : null;
+   const checkoutHref = order?.checkout_url && isSafeCheckoutUrl(order.checkout_url) ? order.checkout_url : null;
+   // 支付入口：{checkoutHref ? (<a ...>) : null}
+   // 死胡同区块：{order?.payment === 'pending' && !checkoutHref ? (...) : null}


─── apps/web/src/commercial/CheckoutPage.tsx:303-303 ───
[style · low] fieldset 使用静态内联 style（边框/圆角/边距均为固定值，非动态样式），违反检查清单「除动态样式外避免内联 style」。建议移入 CSS 类（如
.wk-fieldset）保持与其余组件样式来源一致。

-               <fieldset style={{ border: '1px solid #e7e7ea', borderRadius: 8, margin: '12px 0', padding: '8px 12px' }}>
+               <fieldset className="wk-fieldset">


─── apps/web/src/commercial/BillingPage.tsx:118-119 ───
[maintainability · low] 此处对 purchase.state 的三段条件渲染与 CheckoutPage.purchaseStateMessage
构成了同一状态机的两套措辞：'已付款待激活' vs '已付款，权益处理中'、'已生效' vs '权益已生效'。同一产品状态在不同页面呈现不同词汇，与 spec
L210「页面词汇稳定」的目标相悖，后续增改状态时也需双处同步。建议导出/收敛为一份共享的状态→文案映射供两页复用。

-                 {purchase?.state === 'paid_awaiting_activation' ? ' · 已付款待激活' : ''}
-                 {purchase?.state === 'active' ? ' · 已生效' : ''}
+ // 共享映射（示例）：
+ // export const PURCHASE_STATE_LABEL: Record<PurchaseView['state'], string> = {
+ //   awaiting_payment: '待付款（权益未开放）',
+ //   paid_awaiting_activation: '已付款待激活',
+ //   active: '已生效',
+ //   ...
+ // };
+ // {purchase ? ` · ${PURCHASE_STATE_LABEL[purchase.state] ?? ''}` : ''}


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:13-13 ───
[maintainability · medium] 证据输出目录与文件前缀硬编码为 reverify1/ 与 rv1-*，使脚本成为一次性驱动：reverify2/README.md 记载
rv2-03/rv2-04 截图同样由本脚本产出，但当前代码不改源码就无法把截图写入 reverify2/ 或以 rv2- 前缀命名（rv2-03-*.png 实际存在于 reverify2/
目录），只能靠临时改脚本或手工改名/复制，证据溯源链断裂。建议把输出目录与文件前缀参数化（env 如 FLOW82_EVIDENCE_DIR/FLOW82_EVIDENCE_TAG，或
argv），默认值保持现状。

- const EV = fileURLToPath(new URL('.', import.meta.url)) + 'reverify1/';
+ // 输出目录/前缀参数化：FLOW82_EVIDENCE_DIR=reverify2 FLOW82_EVIDENCE_TAG=rv2 node browser_paid_face_82.mjs <orderId>
+ const TAG = process.env.FLOW82_EVIDENCE_TAG ?? 'rv1';
+ const EV = fileURLToPath(new URL((process.env.FLOW82_EVIDENCE_DIR ?? 'reverify1') + '/', import.meta.url));
+ // 截图文件名相应改为 `${EV}${TAG}-03-checkout-paid-awaiting-activation.png`


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:49-52 ───
[test · low] 证据记录与脚本断言数不一致：本脚本实际输出 5 条 RESULT 断言（checkout-paid-presentation / no-false-effective /
no-awaiting-payment-leftover / billing-paid-awaiting-activation / billing-no-false-effective），而
reverify1 与 reverify2 的 README 均记录「browser_paid_face_82.mjs 4/4
PASS」——说明脚本在证据固化后追加了断言（no-awaiting-payment-leftover 对应 CheckoutPage R1-V12），现在重跑会得到 5/5 而非记录的
4/4，削弱「脚本冻结即所验证内容」的证据可信度。请核对并更新 README 中的计数（或在脚本注释标明断言追加的轮次），保持记录与驱动一致。



─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:60-60 ───
[maintainability · low] 与 browser_paid_face_82.mjs 同类问题：截图文件名硬编码 01-/02- 且固定写入脚本所在目录，而
reverify2/README.md 记载 rv2-01/rv2-02 截图由本脚本产出（reverify2/rv2-01-checkout-awaiting-payment.png
实际存在）——当前代码不改源码无法产出该路径/命名，复用只能靠临时改脚本或手工改名。建议与 paid 脚本一并参数化输出目录与前缀（FLOW82_EVIDENCE_DIR /
FLOW82_EVIDENCE_TAG），默认行为保持不变。

-   await page.screenshot({ path: `${EV}01-checkout-awaiting-payment.png`, fullPage: true });
+ const TAG = process.env.FLOW82_EVIDENCE_TAG ?? '';
+ await page.screenshot({ path: `${EV}${TAG ? TAG + '-' : ''}01-checkout-awaiting-payment.png`, fullPage: true });


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:41-42 ───
[test · low] 登录失败缺乏可诊断的错误处理（三份顶层脚本同一模式）：当 FLOW82_EMAIL/PASSWORD 不正确或租户未播种时，waitForURL 只会抛出 30s 的原始
TimeoutError——结构化 RESULT 行与任何截图都不会产生，报错也无法区分「凭据/租户问题」与「路由跳转问题」，与脚本精心构造的 note 记录器目的相悖。建议把 try/finally
升级为 try/catch/finally，把异常落成一条失败 note 并打印 RESULT 后再以非零码退出。

-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
-   await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 30000 });
+ } catch (e) {
+   note('flow-error', false, String(e?.message ?? e));
+ } finally {
+   await browser.close();
+ }


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:74-75 ───
[bug · low] console.log 之后立即 process.exit 可能截断 RESULT 输出（三份顶层脚本同一模式）：Node 官方文档明确，当 stdout
为管道（CI/日志捕获）时写入是异步的，process.exit 会丢弃未完成的写入——而 RESULT 恰是最后一行、也是机器可读的证据汇总行。此处之后已无其他逻辑，建议改用
process.exitCode 赋值，让事件循环自然排空后再退出。

  console.log('RESULT ' + JSON.stringify(results));
- process.exit(results.every((r) => r.ok) ? 0 : 1);
+ process.exitCode = results.every((r) => r.ok) ? 0 : 1;


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_03_paid_face.mjs:38-41 ───
[test · low] 缺少订单身份锚点（A-12 对齐）：本腿与 browser_02 一样以 `?order=${ORDER}` 打开
checkout，但从不校验页面呈现的是目标订单。browser_02 的 A-12 注释已说明该风险：`?order=` 接线回归时页面会自建新订单；且 CheckoutPage 的
purchaseStateMessage 以租户级 purchase 投影优先渲染（CheckoutPage.tsx:28-38），若租户 A 在环境中残留上一轮的 paid/active
购买，`已付款，权益处理中` 断言可被陈旧购买满足而对错误订单空转通过——正是 A-12 定义并已在 leg 2 修复的假阳性证据形态。建议与 leg 2 一致补一行身份断言。

    await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
    await page.waitForSelector('text=订单结算', { timeout: 45000 });
    await page.waitForSelector('text=已付款，权益处理中', { timeout: 45000 });
    const body = await page.locator('main').innerText();
+   note('order-identity-target', body.includes(ORDER),
+     `page presents the target order ${ORDER}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_04_active_face.mjs:37-40 ───
[test · low] 缺少订单身份锚点（A-12 对齐）：与 browser_03 同理——本腿以 `?order=${ORDER}` 打开 checkout 并断言
`权益已生效`，但从不校验页面呈现的是目标订单。若 `?order=` 接线回归或租户 A 残留上一轮 active 购买（purchaseStateMessage 按租户级投影优先渲染，见
CheckoutPage.tsx:28-38），本腿的 active 断言可被陈旧购买满足，产出假阳性证据——正是 browser_02 A-12 注释定义并已在 leg 2
防御的失败形态。建议补一行与 leg 2 一致的身份断言。

    await page.goto(`${WEB}/platform/billing/checkout?order=${ORDER}`);
    await page.waitForSelector('text=订单结算', { timeout: 45000 });
    await page.waitForSelector('text=权益已生效', { timeout: 45000 });
    const body = await page.locator('main').innerText();
+   note('order-identity-target', body.includes(ORDER),
+     `page presents the target order ${ORDER}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:13-14 ───
[bug · medium] EV 的计算发生在 cd 之后但复用了相对形式的 $0:`cd "$(dirname "$0")/../../../.."` 先把 CWD 切到 worktree
root,随后 `EV="$(cd "$(dirname "$0")" && pwd)"` 里的 `dirname "$0"` 仍是调用时的相对字符串。当从脚本所在目录运行 `./seed.sh`(或
`bash seed.sh`,此时 dirname 为 `.`)时,第一个 cd 已把 CWD 定位到 worktree root,`cd .` 不再移动,pwd 返回的是 worktree root
而非脚本目录——EV 指向错误位置。后果:seed-run.txt、seed-login-a/b.json、api-01/02/03-*.json 全部写入 worktree
root(污染仓库根目录),脚本其余部分因路径自洽仍会"成功"跑完,证据落错位置是静默的,与注释声明的 "evidence lands beside this script (per-round
dir)" 直接矛盾。只有从 root 以足够长的相对/绝对路径调用时才正确。建议先在 cd 之前把脚本目录解析为绝对路径,再统一使用:
```bash
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
EV="$SCRIPT_DIR"               # evidence lands beside this script (per-round dir)
```

- cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
- EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
+ SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
+ cd "$SCRIPT_DIR/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
+ EV="$SCRIPT_DIR"   # evidence lands beside this script (per-round dir)


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_02_sync_face.mjs:58-60 ───
[test · low] 固定睡眠 3500ms 与前端 CheckoutPage 的
POLL_INTERVAL_MS=3000（apps/web/src/commercial/CheckoutPage.tsx:11）隐式耦合：reload 后首个轮询 tick 约在挂载后
3000ms 才触发，若该次订单 GET 的往返耗时超过 ~500ms，读取 innerText 时轮询结果尚未
re-render，"仍是待付款/无已付款声明"这组反向断言可能在轮询真正生效前空转通过，削弱本腿（AC2
同步回访不推进订单）的证据效力；且前端轮询间隔一旦调整，脚本会间歇性失败。建议改为确定性等待：reload 后再执行一次「刷新订单状态」点击并用 page.waitForResponse 等待订单
GET 完成后再断言，消除对轮询节奏的时序假设。

    await page.reload();
    await page.waitForSelector('text=待付款（权益未开通）', { timeout: 45000 });
-   await page.waitForTimeout(3500);
+   // deterministic instead of a fixed sleep coupled to POLL_INTERVAL_MS: click the
+   // manual refresh and wait for the order GET to settle before asserting.
+   await Promise.all([
+     page.waitForResponse((r) => r.url().includes('/api/v1/commercial/') && r.request().method() === 'GET', { timeout: 45000 }),
+     page.locator('button:has-text("刷新订单状态")').first().evaluate((b) => b.click()),
+   ]);


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs:40-40 ───
[test · low] 固定 4500ms 稳定窗同样隐式依赖前端 3s 轮询节奏（CheckoutPage POLL_INTERVAL_MS=3000）加余量：若旧 Status 行的清除晚于一个
tick（投影慢、接口往返长），no-awaiting-leftover 反向断言会间歇性失败；前端轮询间隔调整后该魔法数也随之失效。建议改为确定性等待——等旧文案真正从 DOM 消失后再读取
body。

-   await page.waitForTimeout(4500); // round-1 observation: one poll tick can still hold the old Status line
+   // deterministic instead of a fixed sleep: wait until the stale status line is
+   // actually gone from the DOM (one poll tick can still hold it).
+   await page.waitForSelector('text=已付款，权益处理中', { state: 'detached', timeout: 45000 });


─── apps/web/src/commercial/CheckoutPage.tsx:103-108 ───
[bug · medium] R3-07 集合不完整，导致确定性死循环：后端已验证 `invoice_quote_mismatch` 的判定依据是「权威持有面 vs quote
冻结快照」（purchase.go L195-213），两者对该 quote 均不可变——重放同一 quote 必然再次 409；`this plan version is not
purchasable yet`（ensureNoCharges 读 quote 冻结的 plan 定义）同理。这两枚令牌未纳入集合时，`quoteRef`
不被清空，用户点击「重试（…报价失效时自动重新报价）」会无限重放同一死 quote，且映射文案「订单金额与报价不一致，请重新发起购买」明确要求重新发起购买，与按钮实际行为（同 quote
重放）矛盾。建议纳入集合（`purchase_plan_conflict` 的两条成因与 quote 无关，可维持现状但建议注释说明）。

  const QUOTE_LEVEL_CONFLICT_TOKENS = new Set<string>([
    'quote expired',
    'quote already used',
    'quote predates the purchase freeze; please re-quote',
    'subscription changed since the quote was cut; please re-quote',
+   // 冻结快照/冻结 plan 定义对同一 quote 不可变：重放必然同一 409，重试必须走新报价
+   'invoice_quote_mismatch',
+   'this plan version is not purchasable yet; please re-quote later',
  ]);


─── apps/web/src/commercial/CheckoutPage.tsx:28-32 ───
[bug · medium] 三态状态机缺少 `canceled` 的显式处理：后端 GET /purchase 在购买已取消且存在匹配订单时会以 state='canceled' 携带 order
返回（purchase.go L378-395，`p.State != absent` 即含 canceled）。此时此处落入 default 分支按 order 渠道面渲染：pending
订单显示「待付款（权益未开通）」且页面继续渲染「前往支付」入口，引导用户对一个已取消的购买发起支付——而该取消订阅的 gating invoice 后端永远不予结算（purchase.go
L218-220）；若订单已 paid 则显示「已付款，权益处理中」，同样与事实相反。这也与 `purchaseErrorMessage` 对 canceled
的处理（「该购买已取消，请重新发起购买」）不一致。建议增加 canceled 专属文案，并在渲染处据此隐藏支付入口。

  export function purchaseStateMessage(purchase: PurchaseView | undefined, order: OrderView): string {
    switch (purchase?.state) {
      case 'paid_awaiting_activation': return '已付款，权益处理中';
      case 'active': return '权益已生效';
      case 'awaiting_payment': return '待付款（权益未开通）';
+     case 'canceled': return '该购买已取消，请重新发起购买';


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:15-15 ───
[bug · medium] DB_PATH 仅在两次 sqlite3 调用处直接展开，缺少与 FLOW82_R4_PW 同级的 :? 校验。环境未导出时，set -u
会在注册完全部占位符与主角之后（第 88 行）才以晦涩的 "DB_PATH: unbound variable" 中断，留下半完成的种子状态；指向错误路径时（如沿用 round 1 的
data/issue82-r4verify.db，本轮 README 约定为 data/issue82-r4v2.db）grant 会写入错误数据库且脚本无从察觉，削弱证据可信度。建议在脚本顶部与
PW 一并强制校验并检查文件存在。

  PW="${FLOW82_R4_PW:?missing required env FLOW82_R4_PW}"
+ DB="${DB_PATH:?missing required env DB_PATH (this round: data/issue82-r4v2.db)}"
+ [ -f "$DB" ] || { say "FAIL: DB_PATH=$DB not found"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:58-58 ───
[bug · medium] FLOW82_EMAIL_PREFIX 默认值 settle-r4 与 round 1（r4-flow/seed.sh）的主角账号 settle-r4-a/b
正面冲突：本轮实际运行依赖外部注入 settle-r4v2（见 README 与 seed-login-a.json 中
settle-r4v2-a@verify.local）。若不设置该变量直接运行——复用 round 1 数据库时 -a/-b 会命中 409 并静默复用 round 1 账号（reg_expect
明确容忍 409）；在新库上注册虽成功，主角却落在已被前轮占用的 Lago
身份上——两种情形都静默破坏脚本注释自述的轮次隔离，产出指向跨轮数据的伪证据。鉴于占位数已改为强制记录本轮需求，前缀同理应改为 :? 强制注入。

- reg_expect "a (browser)" "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"
+ PREFIX="${FLOW82_EMAIL_PREFIX:?missing required env FLOW82_EMAIL_PREFIX (default settle-r4 collides with round 1 protagonists)}"
+ reg_expect "a (browser)" "$PREFIX-a@verify.local"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:65-65 ───
[security · medium] 登录响应被原样落盘到受 git 跟踪的仓库路径 seed-login-a/b.json（根 .gitignore 无任何覆盖规则），其中
token/refresh_token 是实验后端可用的活体 JWT。本次提交的副本依赖人工替换为 …<redacted> 才不含可用凭据，即该工作流每次运行都会在仓库树内重新生成活体
token，一次疏忽的 git add 即提交可用凭据字面量，与安全验收条件"源码、示例和测试不得写入可用的凭据字面量"相悖。建议落盘前用 jq 剥离 token 字段（先从 shell 变量取
token，再写脱敏副本，同时移除后面从文件回读 token 的两行）。

- LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"); echo "$LOGIN_A" > "$EV/seed-login-a.json"
+ LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local")
+ LOGIN_B=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local")
+ TOKEN_A=$(jq -r .token <<<"$LOGIN_A"); TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
+ jq '.token="<redacted>" | .refresh_token="<redacted>"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ jq '.token="<redacted>" | .refresh_token="<redacted>"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:88-88 ───
[security · low] grant 与 count 两条 SQL 均以字符串拼接内插 $UID_B，与"数据库查询一律使用参数绑定"的安全约束字面不符。(A-08) 注释称 "the CLI
has no parameter binding" 已过时：sqlite3 CLI（≥3.32）支持把尾部参数绑定到 ?1..?N 占位符，可在保留 UUID 白名单的同时彻底消除拼接。

- sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
+ sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, ?1, 'plan_publish', 'flow-verifier-r4', 1);" "$UID_B"
+ say "granted: $(sqlite3 "$DB_PATH" "select count(*) from commercial_grants where capability='plan_publish' and user_id=?1;" "$UID_B") row(s)"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:96-96 ───
[bug · low] LAGO_KEY 提取无任何校验：docker exec 失败（容器名 weknora-lago-82flow-db-1 变更/容器未起）或 api_keys 表为空时
LAGO_KEY 为空串——且 docker exec 的失败经管道到 tr 后 set -e 无法捕获。随后 curl 以空 Bearer 得到 401 JSON，写盘后 jq
'[.plans[]...]' 以 "Cannot iterate over null" 的间接形式崩溃，报错不指向根因。建议提取后立即断言非空。

  LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]')
+ [ -n "$LAGO_KEY" ] || { say "FAIL: Lago api key extraction failed (docker exec weknora-lago-82flow-db-1 or api_keys empty)"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:21-23 ───
[security · low] reg/login 将密码经 curl -d 作为命令行参数传递，脚本运行期间本机 ps 输出可见 FLOW82_R4_PW
明文。实验环境（127.0.0.1）影响有限，但改用 --data @- 从 stdin 读取即可避免进程列表泄露。

    curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
      -H 'Content-Type: application/json' \
-     -d "{\"username\":\"$1\",\"email\":\"$1\",\"password\":\"$PW\"}"
+     --data @- <<< "{\"username\":\"$1\",\"email\":\"$1\",\"password\":\"$PW\"}"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:74-74 ───
[test · low] 占位符到主角的 Lago tenant id 顺序映射（主角应为 PAD+1/PAD+2，本例
9/10）隐含假设运行期间无并发注册；若他人在占位批次与主角注册之间向同一后端注册，主角会落在未预期的 tenant id 上，脚本无法察觉，产出的 tenant 断言与实际数据错位。当前仅校验非
null，建议补一条对期望序号的显式断言（本例 TENANT_A=9、TENANT_B=10，与占位 1..8 的隔离叙事互证）。

  [ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }
+ [ "$TENANT_A" -eq $((PAD + 1)) ] && [ "$TENANT_B" -eq $((PAD + 2)) ] \
+   || { say "FAIL: protagonists on unexpected tenants A=$TENANT_A B=$TENANT_B (expected $((PAD+1))/$((PAD+2))) — concurrent registration may have broken round isolation"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:51-51 ───
[maintainability · low] r4-flow / r4-flow2 / r4-flow3 的 seed.sh 为近乎完全复制的三份脚本（实测仅头部注释与 PAD 默认值 6/8/10
不同），上述任何修正（DB_PATH 校验、前缀强制注入、token 脱敏、LAGO_KEY 断言等）都需同步三处，存在修正漂移风险。建议收敛为单一脚本，轮次参数（pad
数、前缀）经必填参数或环境变量注入，各轮目录仅保留证据与薄包装。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:41-45 ───
[maintainability · low] 四个腿脚本（browser_01~04，连同 r4-flow / r4-flow3 姊妹目录共 12 份）各自复制了约 30
行完全相同的样板：chromium 引入、WEB/EV/EMAIL/PASSWORD 读取与校验、note/results 工具、这段登录点击序列、以及结尾的 RESULT 输出 +
退出码模式。登录选择器（#auth-email、form[aria-label="Login form"]）或轮询前置逻辑一旦调整，需要同步修改 12
处，且极易漏改某一腿造成假失败。建议为后续验证轮次提取共享 helper（如 docs/plans/issue-72-flow-evidence-82/_browser_lib.mjs，导出
loginAndOpen / note / runWithResult 等），现有已归档的证据快照可保持不动。



─── deploy/lago-lab/payment-settle-trigger/fixtures.py:34-36 ───
[maintainability · medium] 模块顶层 `sys.path.insert(0, _PA_DIR)` 既无必要、又重新引入了 phases.py
注释（A-06/F17）明确描述并已移除的模块绑定反转问题。无必要的证据：payment-activation/fixtures.py 只 import 标准库
datetime，`spec_from_file_location + exec_module` 的别名加载完全不依赖 sys.path（在 run_lab.py 流程中 _PA_DIR 已在
path 上、该 insert 被 if 跳过，别名加载照样工作，即为实证）。危害：在 pytest/REPL 中导入本模块后，_PA_DIR 被插到 sys.path[0]，其后任何裸名
`import phases` / `import run_lab` / `import test_phases` 都会解析到 payment-activation 目录的同名模块（t10
版本），而非本实验室的 t11 版本——正是 phases.py 注释所说"mutating the import search path INVERTED the module
binding"的失效形态。建议删除这三行 insert，仅保留别名加载，与 phases.py 的纪律保持一致。

+ # pa_fixtures.py 仅依赖标准库（datetime），别名加载无需改动 sys.path；
+ # 任何 sys.path.insert(0, _PA_DIR) 都会使后续裸名 phases/run_lab/test_phases
+ # 解析到 payment-activation 的同名模块（A-06/F17 反转），故不插入。
  _PA_DIR = Path(__file__).resolve().parent.parent / "payment-activation"
- if str(_PA_DIR) not in sys.path:
-     sys.path.insert(0, str(_PA_DIR))


─── deploy/lago-lab/payment-settle-trigger/phases.py:185-191 ───
[security · low] RunContext.graphql 自建 opener（仅 ProxyHandler({})）向 /graphql 发送携带 operator JWT 的
Authorization 头，默认 HTTPRedirectHandler 处于启用状态，重定向会被跟随——这违背了 clients.py 明文承诺并经 LagoRestClient
落实的安全不变量（"the credential is only ever sent to the configured origin and off-origin redirects are
refused rather than followed"，经 _OriginBoundRedirectHandler 实现）。在受信任的本地实验栈中影响有限，但同一凭据（operator JWT
也在 run_lab.py 的密钥扫描清单里）应沿用同一防重定向保护。建议改用 clients._proxyless_opener(origin)（本文件已大量使用 clients 私有成员，如
ctx.stripe._form、self.lago._timeout），或将 graphql 调用下沉到 LagoRestClient 复用其 origin 绑定 opener。

          request = urllib.request.Request(
              f"{self.lago_url}/graphql",
              data=payload,
              headers=headers,
              method="POST",
          )
-         opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
+         # 与 LagoRestClient 相同的 origin 绑定 opener：JWT 只发往配置的源，
+         # 跨源重定向被拒绝而非跟随（OffOriginRedirect）。
+         opener = clients._proxyless_opener(self.lago._origin)


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_04_active_face.mjs:44-46 ───
[maintainability · medium] 缺少其余三腿（browser_01/02/03）及第 2 轮同名脚本统一实现的 (A-03) script-error
捕获：r4-flow2/browser_04_active_face.mjs 带有完整 catch 块（note('script-error', false, …) + 失败截图），本轮只有
try/finally。任一 waitForSelector 超时或导航异常将以未处理异常抛出，RESULT
汇总行不会打印、失败截图不会落盘，导致该腿证据无法区分「断言失败」与「脚本基础设施失败」，与 (A-03) 取证约定不一致。修复时需同步把 `const page` 提升为 try 外的 `let
page;`，catch 中才能做 best-effort 截图。

+ } catch (err) {
+   // (A-03) Infrastructure failure → explicit FAIL note; the RESULT summary
+   // still prints so an assertion failure is distinguishable from a script
+   // infrastructure failure.
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}07-billing-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
  } finally { await browser.close(); }
  console.log('RESULT ' + JSON.stringify(results));
  process.exit(results.every((r) => r.ok) ? 0 : 1);


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:53-53 ───
[bug · low] getAttribute('href') 在元素存在但 href 属性缺失时返回 null，此处未做校验即写入 order-info.json，下游若消费
checkoutHref 会读到 null。前置 waitForSelector 只保证了 <a> 元素存在，并不保证 href 属性非空，建议按本脚本风格将空值显式记为断言（FAIL
note）而非静默写盘。

    const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
+   note('pay-link-href-present', checkoutHref !== null && checkoutHref !== '', `checkout link href: ${checkoutHref ?? '(missing)'}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:30-35 ───
[maintainability · low] 本目录 4 个脚本重复同样的登录序列、env 校验、note() 收集器与 RESULT/退出码样板（连同 r4-flow/r4-flow2 各轮共
12 份逐字复制）。轮间整份复制已产生行为漂移——browser_04 丢失 (A-03) catch 块即为其代价（r4-flow2 版本有、本轮没有）。建议在本轮目录内提取共享 helper（如
login(page)、makeNote()），后续轮次复制共享模块而非复制整份脚本，使取证约定（script-error、RESULT 退出语义）只维护一处。



─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:57-57 ───
[bug · medium] --base 的默认值读取环境变量 LAGO_API_URL 并在缺失时回退硬编码 http://127.0.0.1:48895，但配套的
prepare_t9_env.sh 只导出 LAGO_INTEGRATION_BASE_URL（取自 lab.env 的 LAGO_API_URL），并不导出 LAGO_API_URL
本身；本仓库消费该导出的 Go 集成测试（lago_settlement_integration_test.go 等）用的也是 LAGO_INTEGRATION_BASE_URL。脚本
docstring 声称 "LAGO_* from the t11 lab.env by the caller"，操作者 source prepare_t9_env.sh 后直接运行时
LAGO_API_URL 并不存在，只能依赖回退值——而 lab.sh 支持通过 LAGO_API_PORT 自定义端口（非默认时 lab.env 里的 LAGO_API_URL 不是
48895），此时请求会被静默发到错误端口，webhook 投递失败或打到错误实例。建议默认值优先读 LAGO_INTEGRATION_BASE_URL，与 t9 导出契约对齐。

-     ap.add_argument("--base", default=os.environ.get("LAGO_API_URL", "http://127.0.0.1:48895"))
+     ap.add_argument("--base", default=os.environ.get("LAGO_INTEGRATION_BASE_URL")
+                     or os.environ.get("LAGO_API_URL", "http://127.0.0.1:48895"))


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:23-25 ───
[maintainability · low] import sys 从未被使用：文件内所有退出均通过内置的 raise SystemExit(...)，无任何 sys.*
引用，属于死代码，建议删除。

  import subprocess
- import sys
  import time


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:70-70 ───
[bug · low] GraphQL 查询 paymentProviders(limit: 50) 未按 provider 类型过滤，内联片段 ... on StripeProvider {
code } 只对 Stripe 类型返回字段；若同一 org 后续注册了 Gocardless 等其他类型提供商，对应元素会被序列化为不含任何字段的空对象，c["code"] 将抛 KeyError
使整个 t9 环境准备流程以不透明回溯中断。当前 t11 栈只注册 Stripe（deploy/lago-lab 下无其他 provider 注册痕迹），暂不会触发，但这是缺失的边界处理，建议过滤无
code 字段的元素。

- codes = [c["code"] for c in (existing["data"]["paymentProviders"] or {}).get("collection") or []]
+ codes = [c["code"] for c in (existing["data"]["paymentProviders"] or {}).get("collection") or [] if "code" in c]


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:85-85 ───
[bug · medium] DB_PATH 全脚本无定义、无守卫：与 FLOW82_R4_PW 的 `:?` 显式守卫约定不一致。若调用环境未导出，set -u 要到第 85 行才以
"unbound variable" 含糊中止——此时 12 次占位/主角注册与登录等副作用已经发生，留下半完成种子状态；若 DB_PATH 指向错误或不存在路径，sqlite3 会先新建空库再报
no such table，同样发生在副作用之后。建议在顶部与 PW 相邻处补 `:?` 守卫（r4-flow/README 已将 DB_PATH 文档化为 env
约定，脚本内应同样强制），并可加文件存在性检查。

- sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
+ BACKEND=http://127.0.0.1:8093
+ PW="${FLOW82_R4_PW:?missing required env FLOW82_R4_PW}"
+ DB_PATH="${DB_PATH:?missing required env DB_PATH (sqlite db of the :8093 backend, e.g. data/issue82-r4v3.db)}"
+ [ -f "$DB_PATH" ] || { echo "FAIL: DB_PATH='$DB_PATH' does not exist" >&2; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:12-13 ───
[bug · medium] 先 cd 到 worktree root、再基于相对 `$0` 计算 EV，顺序有缺陷：当从 round 目录以 `./seed.sh` 调用时，第二次 `dirname
"$0"` 解析为 `.`，而 cwd 已是仓库根，EV 被静默解析为仓库根——`seed-run.txt`、`seed-login-*.json`、`api-*.json` 全部落错目录，`: >
"$EV/seed-run.txt"` 还会在仓库根截断同名文件，全程无任何报错，直接破坏“证据落在 per-round 目录”的证据链约定（r4-flow/r4-flow2 兄弟脚本同病）。建议在
cd 之前先把脚本目录解析为绝对路径。

- cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
- EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
+ SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"   # resolve to absolute BEFORE changing cwd
+ cd "$SCRIPT_DIR/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
+ EV="$SCRIPT_DIR"   # evidence lands beside this script (per-round dir)


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:62-63 ───
[security · medium] login() 的完整响应（含真实 token/refresh_token）被原样写入证据文件，而这类文件按项目惯例随仓库提交——本次及兄弟轮提交的
`<redacted>` 副本均为事后手工替换，仓库内不存在任何脚本化脱敏步骤（全目录检索 redact 仅命中 JSON 自身）。一旦复跑后未经手工脱敏直接提交，可用 JWT 将进入 git
历史，与本变更组“凭据不得以可用字面量进入源码/示例/测试”的验收约束相抵触。建议在脚本内固化脱敏：原始响应只留在 shell 变量，落盘的是 jq 脱敏副本，下游 jq 取值改从 shell
变量读取（token 校验会因 "<redacted>" 非空非 null 而继续通过，draft/publish 仍用真实 TOKEN_B）。

- LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local"); echo "$LOGIN_A" > "$EV/seed-login-a.json"
- LOGIN_B=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local"); echo "$LOGIN_B" > "$EV/seed-login-b.json"
+ LOGIN_A=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-a@verify.local")
+ LOGIN_B=$(login "${FLOW82_EMAIL_PREFIX:-settle-r4}-b@verify.local")
+ # 原始 token 只留在 shell 变量；证据文件落盘前脚本内脱敏，杜绝复跑+直接提交泄漏
+ jq '.token="<redacted>" | .refresh_token="<redacted>"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
+ jq '.token="<redacted>" | .refresh_token="<redacted>"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"
+ TOKEN_A=$(jq -r .token <<<"$LOGIN_A")
+ TOKEN_B=$(jq -r .token <<<"$LOGIN_B")
+ UID_B=$(jq -r .user.id <<<"$LOGIN_B")
+ TENANT_A=$(jq -r .active_tenant.id <<<"$LOGIN_A")
+ TENANT_B=$(jq -r .active_tenant.id <<<"$LOGIN_B")


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:48-50 ───
[bug · low] PAD 未校验为正整数：`for i in $(seq 1 "$PAD")` 中命令替换的失败状态不会传播给 for 命令，set -e
不触发——FLOW82_PAD_COUNT 为非数字（或 "0"，`:-` 不替换非空值）时 seq 报错、循环静默零迭代、脚本继续往下注册主角，占位完全不发生，主角恰好落在前两轮已占用的 Lago
tenant id 上——这正是脚本注释 A-10/A-13 声称必须防住的跨轮数据串扰，且无任何 FAIL 输出。建议在赋值后立即做形状校验，早失败早报告。

  PAD="${FLOW82_PAD_COUNT:-10}"
+ [[ "$PAD" =~ ^[1-9][0-9]*$ ]] || { say "FAIL: FLOW82_PAD_COUNT='$PAD' is not a positive integer — refusing to run under-padded"; exit 1; }
  say "== register $PAD placeholders (absorb Lago tenant ids) =="
  for i in $(seq 1 "$PAD"); do


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:71-71 ───
[bug · low] 边界校验漏空串：jq 对缺失字段输出字面 "null" 可被拦截，但字段值为空串时 TENANT 为 ""，`!= "null"` 会放行，后续证据（tenant A/B
标注、SEED OK 行）会以空租户落盘。建议补 `-n` 非空校验（token 侧已有等价的 for 循环校验，此处保持一致）。

- [ "$TENANT_A" != "null" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }
+ [ -n "$TENANT_A" ] && [ "$TENANT_A" != "null" ] && [ -n "$TENANT_B" ] && [ "$TENANT_B" != "null" ] || { say "FAIL: login token/tenant missing"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:20-24 ───
[bug · medium] reg_expect 声称的 "backend down -> 000" 显式 FAIL 路径实际不可达：脚本启用了 `set -e`，`code=$(reg
"$2")` 是命令替换赋值，curl 连接层失败（后端未启动）时 curl 退出码 7 会经 reg 函数直接传播给该赋值语句，set -e 在此静默中止——`case "$code"` 永远收不到
"000"，注释 A-13 承诺的 FAIL 诊断不会出现；且 curl -s 连错误消息都抑制，脚本以退出码 7 完全无输出地终止（login() 同理，连接失败时同样静默中止）。建议在 reg()
末尾对 curl 加 `|| true`，保留 "000" 输出交给 reg_expect 的 `*)` 分支显式报告，使行为与注释一致。

  reg() { # reg <email>
    curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
      -H 'Content-Type: application/json' \
-     -d "{\"username\":\"$1\",\"email\":\"$1\",\"password\":\"$PW\"}"
+     -d "{\"username\":\"$1\",\"email\":\"$1\",\"password\":\"$PW\"}" || true   # 连接层失败(退出码7)不拦截，保留 000 交给 reg_expect 显式 FAIL
  }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:93-93 ───
[bug · low] LAGO_KEY 提取后无空值断言：docker exec 失败时 pipefail 可兜底（stderr 有 docker 诊断），但当 api_keys 表为空、psql
成功返回空行时 LAGO_KEY 为空串，脚本会继续带着空 Bearer 调 Lago API，收到 401/error JSON 后才在 `BASE_N=$(jq '[.plans[] ...')`
处以误导性的 "Cannot iterate over null" 失败——且此时注册/授权/发布等副作用已全部发生。与 FLOW82_R4_PW 的显式 `:?`
守卫风格保持一致，建议提取后立即断言非空。

  LAGO_KEY=$(docker exec weknora-lago-82flow-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]')
+ [ -n "$LAGO_KEY" ] || { say "FAIL: Lago api key lookup returned empty (container/api_keys state?)"; exit 1; }


─── internal/modules/commercial/commercialplatform/lago_settlement.go:139-143 ───
[bug · high] settle 的扣款对象定位未与本次购买的 gating 发票身份绑定：stripeListGatingIntents 的候选集是该 provider customer
名下所有「未结算 + lago_invoice_id 非空」的 intent，latestIntent 仅按 created 最新消歧，settledInvoices 幂等检查也只覆盖「已解析
intent 自己的发票」。若同一 customer 名下存在另一张发票的未结算 intent 且创建时间更晚（历史残留、共享绑定、未来非 0 金额的续费/加购发票——仓库自身 OCR 报告
docs/plans/issue-72-ocr-issue-82-r1.md 已自认该「扣错发票/第二笔真实扣款」风险面），本次 settle 会确认错误 intent：金额守卫只做
1..AmountFen 限幅，一张金额更小的异发票 intent 完全可以通过，钱从结算工具真实扣出而本单 gating 发票仍挂着。这是真实扣款路径，不能只靠 latest-created
启发式。建议：解析出候选后，若未结算候选携带的 lago_invoice_id 出现多于一个不同值（同发票的残留兄弟属合法场景，单一发票 id 不受影响），fail-closed 返回
ErrPlatformInvalidResponse 交由 attention 处置，绝不盲扣；或在购买创建时记录 gating 发票身份并在 settle 时强校验。

+ 	distinct := map[string]bool{}
+ 	for _, cand := range intents {
+ 		distinct[cand.LagoInvoiceID] = true
+ 	}
+ 	if len(distinct) > 1 {
+ 		return commercial.CommandReceipt{}, fmt.Errorf(
+ 			"%w: unsettled gating intents span %d distinct invoices — ambiguous settle target",
+ 			commercial.ErrPlatformInvalidResponse, len(distinct))
+ 	}
  	if intent.Amount <= 0 || intent.Amount > payload.AmountFen {
  		return commercial.CommandReceipt{}, fmt.Errorf(
  			"%w: gating intent amount %d is not within the frozen quote face (1..%d)",
  			commercial.ErrPlatformInvalidResponse, intent.Amount, payload.AmountFen)
  	}


─── internal/modules/commercial/commercialplatform/lago_settlement.go:357-359 ───
[bug · high] settle 将 invoice_settings.default_payment_method 永久改写为结算 PM 的 clone，且无任何恢复/回写逻辑（全仓仅
lago_purchase.go:833 与此处两个写入点，均无还原路径）。而 lago_purchase.go readPurchaseInvoiceFees 的 A-24
注释自证：purchase 订阅在每个自然月边界会被权威侧 recurring billing 开出续期发票（t02-duplicates 证据）。续期发票的自动扣款走 customer
默认支付方式——本命令之后即结算工具（运维侧仪器），意味着每个 active 购买者每月都会对结算工具产生一笔真实扣款，客户的真实卡被绕过，资金来源与对账双双错配。且本次扣款本身并不依赖默认方式：步骤
(iv) 的 update+confirm 已显式携带 payment_method。建议：去掉 default_payment_method 写入（对本次 confirm 非必需），或在
confirm 成功后恢复原默认值/分离 attach-only，并在 #84 的生命周期面落地前明确续期发票的收款路由。

- 	defaultStatus, _, err := a.providerOutboundRequest(ctx, http.MethodPost,
- 		"/v1/customers/"+url.PathEscape(providerCustomerID),
- 		"invoice_settings[default_payment_method]="+url.QueryEscape(attached.ID), keyBase+":default")
+ 	// Do NOT rewrite the customer default payment method: the confirm below
+ 	// carries payment_method explicitly, and a permanent default would route
+ 	// every recurring renewal invoice onto the settlement instrument.
+ 	_ = defaultUnused


─── internal/modules/commercial/commercialplatform/lago_settlement.go:445-447 ───
[bug · medium] boundProviderCustomerID 对所有非 200 状态（含 429/5xx）统一返回 ErrPlatformInvalidResponse，违反本 PR
自己在 finalizedInvoiceIDs/stripeListGatingIntents 确立的 A-25/F96 纪律（429/5xx → ErrPlatformUnreachable
可重试）。该函数位于两条链上：settle 主链（fulfiller 步骤 ② 将 ErrPlatformInvalidResponse 落为终态 attention 记录）与
createPurchaseSubscription 的 422 恢复腿。同一 GET 在 settle 链上前一步 customerProviderBound 已正确分类，此处的瞬时 5xx
却会把订单误标 attention。注意其前一跳 customerProviderBound 与本函数读的是同一个 customer 资源，分类应一致。建议按同文件惯例拆分：429/5xx →
ErrPlatformUnreachable，其余非 200 → ErrPlatformInvalidResponse。

- 	if status != http.StatusOK {
+ 	switch {
+ 	case status == http.StatusOK:
+ 	case status == http.StatusTooManyRequests || status >= 500:
+ 		return "", fmt.Errorf("%w: binding read unavailable (HTTP %d)", commercial.ErrPlatformUnreachable, status)
+ 	default:
  		return "", fmt.Errorf("%w: binding read answered HTTP %d", commercial.ErrPlatformInvalidResponse, status)
  	}


─── internal/modules/commercial/commercialplatform/lago_settlement.go:166-169 ───
[bug · low] 结算链在扣款前对 StripeSettlePmToken 做了 fail-closed，但对同样必需的 StripeAPIKey
没有同等检查：providerOutboundRequest 会用空 key 构造 Authorization，Stripe 回 401 → classifyStripeStatus 判为
ErrPlatformInvalidResponse → fulfiller 步骤 ② 落终态 attention。配置缺失是 ErrPlatformUnconfigured 语义（同文件
createPurchaseSubscription 的 422 腿对 StripeAPIKey=="" 已如此处理），不应表现为权威拒绝。建议把 StripeAPIKey 一并纳入该
unconfigured 检查。

- 	if a.cfg.StripeSettlePmToken == "" {
+ 	if a.cfg.StripeSettlePmToken == "" || a.cfg.StripeAPIKey == "" {
  		return commercial.CommandReceipt{}, fmt.Errorf(
  			"%w: no settle payment method configured", commercial.ErrPlatformUnconfigured)
  	}


─── internal/modules/commercial/commercialplatform/lago_purchase.go:624-626 ───
[performance · medium] readPurchaseInvoiceFees 的正确性完全依赖「index 按 newest-first 返回」这一未在请求中声明的服务端约定：若
pinned v1.53 的默认排序不符（或未来版本变化），会静默选中较旧的 succeeded 发票行（如重购场景下旧价格发票），D6' 复核随之选错行。同时该读是 N+1：active
购买者的每次 purchase 快照读取都要 index 分页 + 逐发票 GET，而 benefits.go 的 effectiveFeatures 在每次 billing 访问（handler
EnsureBenefits）都读一次 purchase 快照，长历史租户（Base 月度 0 金额发票 + 续期发票累积）下每次账单请求都全量扫 index。建议：(1) 每张发票的 GET
响应本身携带 created_at/issuing_date，将其纳入解析结构并在选 newest 时显式比较，不信任 index 顺序；(2) 为该 finalized
行读取加投影缓存或仅在状态变化时刷新，避免每次 billing 访问的 N+1。

- 	var fallbackFees []commercial.InvoiceLineSnapshot
- 	var fallbackStatus string
- 	for _, id := range ids {
+ 	var parsed struct {
+ 		Invoice struct {
+ 			PaymentStatus string `json:"payment_status"`
+ 			CreatedAt     string `json:"created_at"`
+ 			// …fees as before…
+ 		} `json:"invoice"`
+ 	}
+ 	// compare parsed.Invoice.CreatedAt explicitly when choosing the newest
+ 	// succeeded match instead of trusting the index row order.


─── internal/modules/commercial/commercialplatform/config.go:176-177 ───
[maintainability · low] loopback 判定逻辑存在双实现：config.go 的 outboundHostIsLoopback 与 lago_purchase.go
validateOutboundHostWithBypass 内联的判定（EqualFold("localhost") / *.localhost /
net.ParseIP().IsLoopback）是完全相同的副本。二者分别守卫启动姿态（guardLoopbackBypassPosture）与运行时出口（validateOutboundHostW
ithBypass），任何一侧单独演化（如新增 IPv6 zone、trailing dot 形态）都会造成「启动放行、运行时拒绝」或反向的漂移。另外
guardLoopbackBypassPosture 只校验 cfg.BaseURL 不校验 cfg.StripeAPIBase（provider 侧非 loopback host
仍走全量策略，实际风险低，但姿态守卫的覆盖面与注释宣称的防御纵深不对称）。建议 validateOutboundHostWithBypass 复用本函数（或抽到共享 helper），消除双实现。

- func outboundHostIsLoopback(rawURL string) bool {
- 	parsed, err := url.Parse(rawURL)
+ // in lago_purchase.go validateOutboundHostWithBypass:
+ 	host := parsed.Hostname()
+ 	if outboundHostIsLoopback(parsed.Hostname() + "") { // share the single predicate
+ 		return nil
+ 	}
+ 	return validateOutboundHost(rawURL)


─── internal/middleware/auth.go:71-71 ───
[security · medium] 该白名单条目首次将支付回调端点暴露为匿名可达面（此前被全局 Auth 401 拦死），而 `PaymentCallbacksHandler`
对请求体是无界读取：两个分支（payment_callbacks.go:82/:142）都直接 `io.ReadAll(c.Request.Body)`，文件及全局均无
`http.MaxBytesReader`/body 上限（我已全仓检索，项目惯例是各 handler 自行加限，如 craft.go、upload_limit.go）。签名校验发生在读完整个
body 之后，任何匿名客户端都可在验签失败前迫使服务端把任意大请求体完整缓冲进内存（内存耗尽 DoS），且攻击体还可以放大 RSA 验签的 CPU 开销。Alipay/WeChat 的 notify
报文只有几 KB，建议在回调组或 handler 入口加 `http.MaxBytesReader`（如 1 MiB）封顶，与项目其他接收不可信 body 的端点保持一致。

- 	"/api/v1/commercial/callbacks/*": {"POST"},
+ // routes_commercial.go 的回调组（匿名可达后请求体必须封顶，与
+ // craft.go / upload_limit.go 的既有惯例一致）：
+ //	callbacksGroup := r.Group("/commercial/callbacks", func(c *gin.Context) {
+ //		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
+ //	})


─── internal/handler/commercial.go:818-822 ───
[bug · medium] （A-21/F105 一致性缺口）挂回的 pending 订单缺少 plan
归属证明。`CurrentPayablePendingOrder`（repository/order.go:398）刻意去掉了价面过滤，PurchaseService 路径在
purchase.go:286-293 用 `quoteBoughtPlan(existing.QuoteID) != snap.PlanKey → ErrPurchasePlanConflict`
补上了所有权校验，注释明确写着"never replaying a payment entry that settles another plan's quote"；而本分支（legacy POST
/orders 路径）无条件把该订单作为支付入口返回。可达场景：租户持有 plan A 的可付 pending 订单，再用 plan B 的新 quote 提交 POST /orders →
插入被唯一索引拒绝 → 这里返回 plan A 订单的 checkout_url；`orderWire` 不含 plan 字段，客户端无法分辨，付款将结算 plan A 的 quote 而购买意图是
plan B。建议在挂回前用 `QuoteSnapshotForTenant` 比对两侧 quote 的 PlanKey（跨 plan 时回裸 409），与服务路径保持同一防线。

  		if existing, rerr := h.orders.CurrentPayablePendingOrderView(c.Request.Context(), tenantID); rerr == nil {
+ 			// (A-21/F105) 挂回的 pending 订单必须与本次请求的 quote 购买
+ 			// 同一 plan，否则它的 checkout_url 结算的是另一个 plan 的 quote。
+ 			_, reqSnap, qerr := h.orders.QuoteSnapshotForTenant(c.Request.Context(), tenantID, req.QuoteID)
+ 			_, exSnap, eerr := h.orders.QuoteSnapshotForTenant(c.Request.Context(), tenantID, existing.QuoteID)
+ 			if qerr == nil && eerr == nil && reqSnap.PlanKey == exSnap.PlanKey {
- 			c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists",
+ 				c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists",
- 				"order": orderWire(existing)})
+ 					"order": orderWire(existing)})
- 			return
+ 				return
+ 			}
  		}


─── internal/handler/commercial.go:823-823 ───
[maintainability · low] 补充读取失败（`rerr != nil`）被静默吞掉：走裸 409 本身是正确的降级，但本模块对降级路径的一贯做法是留一行日志（见
service/commercial/order.go:448-450、:471 的 `log.Printf`）。这里留一条日志可以让"索引证明了存在 pending 却读不出可付订单"（如刚好被
R3-26 清扫、或存储抖动）的形状可诊断。

+ 		log.Printf("commercial: pending-purchase conflict read failed for tenant %d: %v", tenantID, rerr)
  		c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists"})


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:25-27 ───
[maintainability · low] 文件中 `except urllib.error.HTTPError as e:` 依赖 `import urllib.request` 传递导入
urllib.error 子模块的副作用才能访问 `urllib.error`。这在 CPython 恰好可行（urllib.request 内部 from urllib.error import
...），但属于脆弱的隐式依赖：一旦导入集合调整或运行环境差异即 AttributeError: 'urllib' has no attribute 'error'。建议显式补充 `import
urllib.error`。

  import time
+ import urllib.error
  import urllib.parse
  import urllib.request


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:28-28 ───
[bug · low] 在 `set -e`（含 pipefail）下，`export VAR="$(cmd)"` 中命令替换的失败会被 export 自身成功返回的状态掩盖（export
是命令名，set -e 不看替换的退出码）：lab.env 缺 LAGO_API_URL/LAGO_ORG_API_KEY，或 db 容器未就绪导致 psql
失败时，LAGO_INTEGRATION_BASE_URL/API_KEY/ORG_ID 会被静默置空，脚本继续执行并打印 "t9 env ready (org=)"，下游 tagged Go
测试只能以 "lago integration env not configured" skip，掩盖真实故障（违背本票 skip≠pass 的证据纪律）。建议先落临时变量并显式校验非空再
export。

- export LAGO_INTEGRATION_ORG_ID="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T db psql -U lago -tAc 'select id from organizations order by created_at limit 1' | tr -d '[:space:]')"
+ org_id="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T db psql -U lago -tAc 'select id from organizations order by created_at limit 1' | tr -d '[:space:]')"
+ [ -n "$org_id" ] || { echo "failed to resolve organization id from the lab db (is the stack up?)" >&2; return 1 2>/dev/null || exit 1; }
+ export LAGO_INTEGRATION_ORG_ID="$org_id"


─── internal/handler/commercial.go:506-516 ───
[bug · medium] （R2-26/R3-26 一致性缺口）新的 default→500 分支注释声称残余错误只有服务端 gorm/storage 失败，但
PurchaseService.Purchase 的 ErrPurchasePendingExists 恢复链有三处会把携带该哨兵的错误原样上抛：purchase.go:295（冲突读非
NotFound 失败）、:304（清扫失败）、:308（清扫后重试 CreateOrder 再次返回 ErrPurchasePendingExists——服务注释明确写着 "a second
conflict is a genuine race beyond the sweep — surface the conflict error, never
loop"，即期望边缘按冲突面应答）。这些是确定性的客户端事实（并发重复购买竞态输了），却没有对应 case，落入 default 被误判为 500 "purchase
failed"——恰是本分支自己在 ErrQuoteNotFound case 里反对的形状（"never the residual 500 which invites retrying a
deterministic failure"），且与同 diff 中 CreateOrder 补上的 ErrPurchasePendingExists→409 映射不一致。建议补一个 409
分支（可仿照 CreateOrder 附带现存可付订单）。

+ 	case errors.Is(err, repocommercial.ErrPurchasePendingExists):
+ 		// (R2-26/R3-26) 并发重复购买竞态输了：确定性的客户端冲突，
+ 		// 按冲突面应答 409，绝不能落入 500（那会把确定性失败伪装成
+ 		// 可重试的服务端故障）。
+ 		c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists"})
  	default:
- 		// (R1-09) The residual error face is server-side (gorm/storage
- 		// failures from EnsureBillingAccount/GetPublication/GetVersion/
- 		// CreateOrder, snapshot JSON corruption): 500 with a CLOSED
- 		// message — never a 400 (which invites the caller to retry the
- 		// same request as if it were a client fault) and never raw error
- 		// text across the public boundary (same posture as the 503
- 		// branch's "never raw error text"). The original error stays in
- 		// the server log.
- 		log.Printf("commercial: purchase failed for tenant %d: %v", tenantID, err)
- 		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})


─── internal/modules/commercial/repository/commercial/order.go:212-212 ───
[bug · medium] PostgreSQL 索引名匹配错误：OrderRow.QuoteID 的 `uniqueIndex` 标签未显式命名，gorm 默认 NamingStrategy
生成的是下划线形式 `idx_commercial_orders_quote_id`（idx_<table>_<column>，仓库未自定义 NamingStrategy）。PG 实际报错为
`duplicate key value violates unique constraint "idx_commercial_orders_quote_id"`，带点的
`idx_commercial_orders.quote_id` 永远不会作为子串出现，因此 PG 上并发同 quote 双结账的败者不会被翻译为
ErrQuoteAlreadyUsed，而是穿透两个匹配器返回裸驱动错误（未映射的 500）——恰是 A-20/F89 本要修复的失败形态。order_test.go:596
硬编码了同样的带点名字，测试通过但无法反映真实驱动行为。建议同时匹配下划线形式（保留带点形式作防御）。

- 	return strings.Contains(msg, "idx_commercial_orders.quote_id")
+ 	return strings.Contains(msg, "idx_commercial_orders_quote_id") ||
+ 		strings.Contains(msg, "idx_commercial_orders.quote_id")


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:175-181 ───
[bug · medium] 瞬态权威侧故障被误判为终态 attention：本分支把 ErrPlatformInvalidResponse 之外的两个哨兵视为可重试、其余全部落终态
attention，但 settle/observe 链路上的权威侧读取并未把瞬态状态归入
Unreachable——readSubscriptionByIdentity（lago.go:688-694，readPurchaseSnapshot/ReadSnapshot 的必经路径）只把
>=500 归为 Unreachable，HTTP 429 落入 default 分支的
InvalidResponse；boundProviderCustomerID（lago_settlement.go:445-447）把任何非 200（含 429/5xx）都归为
InvalidResponse。同 PR 的其他读取已修正此问题（customerProviderBound lago_purchase.go:505-506、invoice 读 A-25/F96
的注释明确写着"a transient authority failure (429/5xx) is unreachable — retryable — never the definitive
invalid-response sentinel that would park a settle replay in attention"，以及
classifyStripeStatus）。结果：观察窗口内一次 Lago 429 限流即铸造终态 attention 记录——运营据其提前退款而内置 webhook 随后激活，正是
A-31/F114 注释自身防范的 refund+grant 双赢窗口。建议对齐适配器映射（429/5xx → ErrPlatformUnreachable），让本处的
Unreachable→pending 分支覆盖瞬态抖动。

- 		// Retryable platform failures keep the event pending (the next pass
- 		// re-drives; the settle command is idempotent). Definitive
- 		// invalid-response failures surface attention — never a grant.
+ 		// 适配器侧需与 customerProviderBound / A-25(F96) / classifyStripeStatus
+ 		// 对齐：readSubscriptionByIdentity 的 429、boundProviderCustomerID 的
+ 		// 429/5xx 应映射 ErrPlatformUnreachable，使本分支覆盖瞬态限流/抖动：
+ 		//
+ 		// lago.go readSubscriptionByIdentity:
+ 		//	case status == http.StatusTooManyRequests || status >= 500:
+ 		//		return ..., fmt.Errorf("%w: subscription read unavailable", commercial.ErrPlatformUnreachable)
  		if errors.Is(err, domain.ErrPlatformUnreachable) || errors.Is(err, domain.ErrPlatformUnconfigured) {
  			return nil
  		}
  		return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)


─── internal/modules/commercial/service/commercial/fulfillment.go:341-345 ───
[bug · low] 订阅购买误路由是静默的：quote 行读取失败或 SnapshotJSON 解析失败时本函数返回 false，一个已支付（paid）的订阅购买订单会落入旧的
TopUpOrderLines 结算路径——按 1 Credit/CNY 的固定账面汇率把订阅款项当作通用充值额度发放，完全绕过 settle/激活/D6'
复核/购买首期授予链，且无任何日志。虽然正常管线下 quote 行必然存在，但该误路由的后果（客户付了订阅费、权威方永不激活、只拿到等值充值额度）与本 PR 处处强调的 fail-closed +
一条 Warn 的姿态（A-23/A-31/A-33）不一致。建议至少记录读取/解析失败；更稳妥的做法是区分 gorm.ErrRecordNotFound（legacy 充值种子 → 确定返回
false）与其他错误（记录并保持事件 pending/attention，而非误入充值结算身份）。

- func (s *FulfillmentService) isSubscriptionPurchase(ctx context.Context, row repocommercial.OrderRow) bool {
  	var q repocommercial.QuoteRow
  	if err := s.db.WithContext(ctx).Where("id = ?", row.QuoteID).First(&q).Error; err != nil {
+ 		if !errors.Is(err, gorm.ErrRecordNotFound) {
+ 			// 非 NotFound 的读取失败不可证明"不是订阅购买"：误入充值路径会以
+ 			// 账面汇率发放通用额度并绕过 settle/激活链。记录并保持事件待定。
+ 			logger.Warnf(ctx, "[CommercialFulfillment] subscription-purchase quote read failed for order %s: %v", row.ID, err)
+ 		}
  		return false
  	}


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:119-123 ───
[bug · low] A-32 只分类了快照读失败，Fulfill 中其余裸错误返回路径（GetOrder、quote 读、此处 publication
读、succeededAttempt、步骤⑥记录写入、MarkFulfilled）仍会原样上抛。fulfillment.go 的 Recover 在批内首个错误处即 return 中止整趟
drain——一个确定性失败的购买事件（如数据异常导致 succeededAttempt 的 "has no succeeded attempt" 或此处的 publication
行缺失，每轮必失败）会在每次 lease 到期（约每 leaseTTL=1min）被重新租约并再次中止当趟 pass，反复延迟排在其后的所有事件（包括普通充值订单），且永远不会落 attention
记录、永远不自愈。与 A-32 声明的"the shared drain keeps moving"契约不符。建议对这些路径复用 settleSnapshotFailure
的分类姿态：确定性形状（ErrRecordNotFound 等）→ markActivationState + 返回 nil；瞬态 DB 错误 → 返回 nil 保持 pending。

  	var pub repocommercial.PublicationRow
  	if err := p.db.WithContext(ctx).Where("plan_key = ? AND version = ?", snap.PlanKey, snap.PlanVersion).
  		First(&pub).Error; err != nil {
- 		return fmt.Errorf("purchase publication %s v%d: %w", snap.PlanKey, snap.PlanVersion, err)
+ 		if errors.Is(err, gorm.ErrRecordNotFound) {
+ 			// 确定性失败：落 attention，绝不上抛中止共享 drain 批次
+ 			return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
+ 		}
+ 		return nil // 瞬态 DB 故障：保持 pending，下一轮 drain 重驱（各步骤幂等）
  	}


─── internal/modules/commercial/commercialplatform/lago_settlement.go:118-120 ───
[bug · medium] 空候选分支的幂等判定绑定的是「客户」而非「本次购买的 gating 发票」：settledInvoices 收集的是该 provider customer 名下所有带
metadata.lago_invoice_id 的 succeeded intent——按本 PR 自己的证据（lago_purchase.go A-24：重购复用同一 external
订阅身份、t02-duplicates：活跃购买每月续期发票；lago_settlement.go 自身把续期自动扣款也走
PaymentIntent），任何有历史购买/续费的客户该集合必然非空。因此当本次购买的 gating intent 不在 unsettled 候选集中（intent 被权威作废/取消、或状态为
requires_capture/processing 被 switch 过滤掉）而快照仍为 awaiting 时，本分支会在「一分钱都没扣」的情况下返回成功 receipt：fulfiller
步骤②视作 settle 完成，随后 observeActivation 永不收敛，渠道已收款被静默搁置——既违反本文件头部 "never a fabricated outcome" 的
fail-closed 契约，也把一个本应 attention 的数据异常吞成了假成功。与已确认问题 3 同根（缺少发票身份绑定）但失败方向相反：3 是错扣（resolved intent
选错），本分支是漏扣+假成功。建议与 3 一并修复：把幂等事实收敛到本次购买的 gating 发票身份（例如经权威侧读出该购买订阅当前 gating 发票的 lago_invoice_id 后再查
settled 集合，或在 payload 中冻结该身份）；无法建立身份绑定时应答 definitive 错误（走 attention），而不是凭客户维度的历史已结算事实返回 receipt。



─── internal/modules/commercial/service/commercial/purchase.go:253-253 ───
[bug · high] R1-22 防重开检查存在 paid-awaiting 窗口缺口,可能导致同一订阅被双重收款。该检查只探测 payable PENDING 订单;当本地订单 O1 已被
ConfirmPayment 置为 paid、而权威侧订阅仍 awaiting_payment(paid_awaiting_activation 窗口:正常为事件 drain 30s 周期 +
settle + webhook 的数十秒,异常可长达 10 分钟 budget)时:
1) CurrentPendingPurchaseOrder 只匹配 state=pending → NotFound;
2) 唯一索引 uq_purchase_pending_per_tenant 谓词同样只含 state='pending',O1 已 paid 退出索引范围;
3) 于是 CreateOrder 对一张新 quote 成功插入 O3 并返回新的 CheckoutURL。
客户端在支付成功但页面仍显示未激活(轮询 GET /commercial/purchase 返回 awaiting)时再次点购买/换新 quote 重提,是自然的 UX 路径;O3 支付回调
ConfirmPayment 独立成功,fulfiller 对 O3 的 settle 幂等键绑定的是 O3 自己的渠道流水号(settle:<sub-id>:<txn>),不会与 O1
去重——结果是同一 gating 订阅被渠道扣款两次(需人工退款)。这正是本注释块自述的 "must not be charged twice" 合同要防的形状,但只防住了并发 pending,没防住
paid-未激活串行重入。建议:在此处(或 ErrPurchaseNotAwaiting gate 前)同时探测同价面、quoteBoughtPlan 匹配且 state=paid(未
fulfilled)的购买订单——命中则直接返回该订单视图(与 PurchaseStatus 的 paid_awaiting_activation 合成一致),不再开新渠道订单。



─── internal/modules/commercial/service/commercial/purchase.go:306-309 ───
[bug · medium] 并发 checkout 的重放链条在 winner 尚在渠道 Create 窗口内时必然退化为裸 ErrPurchasePendingExists 上抛,而
handler(commercial.go Purchase 的 switch)没有该哨兵的分支,落入 default → 500 "purchase failed"。路径:请求 A 的
OpenOrder 事务已提交(pending、channel_failed=false、checkout_url 仍空,占据索引)→ A 正在外呼渠道 Create(秒级)期间,请求 B 走到
ErrPurchasePendingExists 分支:CurrentPayablePendingOrder 的 payable 谓词比索引谓词多了 checkout_url <> '',读 A 的
link-less 行返回 NotFound;SweepStaleLinklessPending 有 15 分钟 age gate,A 刚创建扫不到;随后 retry 的 CreateOrder
再次撞索引(winner 仍 pending)→ return rerr 裸哨兵。双击提交/双标签页/客户端超时重试正落在这个窗口。handler default 分支的 500
语义("server-side…never invites retry")与实际所需的"稍候重试/重放 winner"正好相反。另外两处顺带问题:同一窗口里 `if _, serr :=
SweepStaleLinklessPending(...); serr != nil { return PurchaseView{}, err }` 丢弃了 serr 返回原始 err;retry
分支也未处理 ErrQuoteAlreadyUsed 的重放。建议:retry 二次撞 ErrPurchasePendingExists 时先重读一次
CurrentPayablePendingOrder(winner 此时多半已持久化 URL)并重放;仍不可得时返回一个 handler 已映射的冲突哨兵(或同步在 handler 为
ErrPurchasePendingExists 增加 409 分支携带 CurrentPayablePendingOrderView),避免把可自愈的并发竞态暴露成 500。



─── internal/modules/commercial/repository/commercial/order.go:348-350 ───
[bug · medium] A-29/F109 的防护只做了一半:pending 偏好循环排除了 channel-failed 行,但循环之后既有的 fallback `if len(rows) >
0 { return rows[0], nil }` 无条件返回最新一行。当同价面中最新行恰好是 channel-failed 的 dead pending(例如 paid 订单 O1
之后又开了一个渠道 Create 失败的 O2——awaiting 窗口内这条路径可达),循环两行都跳过(O2 是 failed pending,O1 是 paid),fallback 返回
O2;PurchaseStatus 里 quoteBoughtPlan(O2.quote) 与当前 plan 匹配,于是投影的是这条永久 pending、无 checkout_url 的 dead
订单,而较旧的 paid 订单被遮蔽——paid_awaiting_activation 合成窗口丢失,客户端看到"待支付"却没有支付入口。这正是本注释宣称要防住的场景("must never …
shadow a real paid order of the same price face … MISS the paid_awaiting_activation synthesis
window")。建议 fallback 同步携带相同谓词:跳过 channel-failed(以及 link-less)的 pending 行,取第一个非 dead 行(即最新的
paid/fulfilled 行)。



─── internal/modules/commercial/service/commercial/purchase.go:327-327 ───
[documentation · low] godoc 绑定错位:原属于 PurchaseStatus 的文档注释("PurchaseStatus answers the purchase
projection for one tenant…")现在紧贴在 var purchaseAudit 声明之上,会被 godoc 绑定到 purchaseAudit 而不是
PurchaseStatus,而 PurchaseStatus 自身失去了 doc comment(其下一段注释已挂在 quoteBoughtPlan 上)。建议把 purchaseAudit
的声明(连同它自己的 attention-audit 注释)移到 PurchaseStatus 的 doc comment 之前,恢复各声明与文档的一一对应。



LLM retry report summary: 4 of 426 requests affected -- 1 request failed, 3 requests recovered after retry

Core review (4 requests):
- docs/plans/issue-72-flow-evidence-82/r4-flow2/order-info.json,docs/plans/issue-72-flow-evidence-82/r4-flow2/seed-login-a.json,docs/plans/issue-72-flow-evidence-82/r4-flow2/seed-login-b.json,docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh: timed out -> failed
- docs/plans/issue-72-flow-evidence-74/verify_ac_assertions.py,docs/plans/issue-72-flow-evidence-74/verify_db_watch.py,docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr1-replay/verify_db_watch.py,docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr2-replay/verify_db_watch.py,docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py,docs/plans/issue-72-ocr3-replay/verify_db_watch.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py,docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py,docs/plans/issue-72-flow-evidence-82/alipay_sandbox_notify.py,docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- docs/plans/issue-72-flow-evidence-81/wechat_pay_stub.py,docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py,docs/plans/issue-72-flow-evidence-82/alipay_sandbox_notify.py,docs/plans/issue-72-flow-evidence-82/wechat_pay_stub.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
