Review partially complete: 111 finding(s); 3 of 138 selected item(s) failed.

─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:82-86 ───
[bug · medium] rsa_verify_sha256 未区分「验签失败」与「openssl/密钥配置错误」：openssl dgst -verify
在密钥文件缺失/不可读时同样以非零码退出，此处仅看 returncode 会静默返回 False，被 verify_request 上游当作签名不匹配处理，stub 随即回答
isv.invalid-signature——这正是 verify_request 中 (OCR r2) 注释明确声明必须避免的失败模式（注释要求：密钥缺失/openssl 失败属于
CONFIGURATION 错误必须 raise，仅解码失败才归为签名无效），配置错误会把取证人员引向错误的排查方向。建议按 openssl 输出区分："Verified OK"
为真、"Verification Failure" 为假、其余抛出携带 stderr 的 RuntimeError。

          proc = subprocess.run(
              ["openssl", "dgst", "-sha256", "-verify", pub_pem,
               "-signature", sig_path],
              input=content, capture_output=True)
-         return proc.returncode == 0
+         out = proc.stdout.decode(errors="replace")
+         if "Verified OK" in out:
+             return True
+         if "Verification Failure" in out:
+             return False
+         raise RuntimeError(
+             f"openssl dgst -verify failed (rc={proc.returncode}): "
+             f"{proc.stderr.decode(errors='replace').strip()}")


─── docs/plans/issue-72-flow-evidence-82/_browser_lib.mjs:65-69 ───
[bug · low] chromium.launch() 位于 try 块之外：浏览器启动失败（未安装 chromium、可执行文件缺失等）时 runLeg 会以裸堆栈终止，不产出 RESULT
行也不记录 leg-error FAIL 项，破坏了本文件头部声明的 A-03 契约（"a browser leg that THROWS must still emit a RESULT line
and a non-zero exit code"）——RESULT 消费方在该失败面会解析失败。建议将 launch 纳入 try，并在 finally 中对可能未初始化的 browser 做防护。

    const { chromium } = requirePlaywright();
    const results = [];
-   const browser = await chromium.launch();
+   let browser;
    try {
+     browser = await chromium.launch();
      await body(results, browser);


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:16-25 ───
[maintainability · low] 手写的 FLOW82_EMAIL(_D)/FLOW82_PASSWORD(_D) 回退校验块与 browser_sync_face_82.mjs 中的
_B 版本逐行同构，且与库内 envRequired 的职责重叠（只是缺少「多候选回退」变体）——这正是 _browser_lib.mjs 以"单一副本防漂移"为初衷要消除的重复。建议在
_browser_lib.mjs 中新增 envAny(primary, fallback, label) 一类的回退变体，两条腿统一消费，避免后续新腿再复制第三份。

- if (!process.env.FLOW82_EMAIL_D && !process.env.FLOW82_EMAIL) {
-   console.error('missing required env: FLOW82_EMAIL(_D)');
-   process.exit(2);
- }
- if (!process.env.FLOW82_PASSWORD_D && !process.env.FLOW82_PASSWORD) {
-   console.error('missing required env: FLOW82_PASSWORD(_D)');
-   process.exit(2);
- }
- const EMAIL = process.env.FLOW82_EMAIL_D ?? process.env.FLOW82_EMAIL;
- const PASSWORD = process.env.FLOW82_PASSWORD_D ?? process.env.FLOW82_PASSWORD;
+ // _browser_lib.mjs 中新增：
+ // export function envAny(primary, fallback, label) {
+ //   const v = process.env[primary] ?? process.env[fallback];
+ //   if (!v) {
+ //     console.error(`missing required env: ${primary}(_${label})`);
+ //     process.exit(2);
+ //   }
+ //   return v;
+ // }
+ // 本腿消费：
+ // const EMAIL = envAny('FLOW82_EMAIL_D', 'FLOW82_EMAIL', 'D');
+ // const PASSWORD = envAny('FLOW82_PASSWORD_D', 'FLOW82_PASSWORD', 'D');


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:41-46 ───
[maintainability · medium] 本 PR 同批新增的共享库 ../_browser_lib.mjs（导出
login/note/runLeg/evidencePath/assertOrderIdentity）已被 r5-verify 与 r5b-flowcheck 的全部 browser 腿复用，但
r4-flow2 四条腿仍各自内联同一套骨架（createRequire bootstrap、env 必填校验、note/results 收集、登录序列、A-03 catch、RESULT
汇总与退出码）。且实现已与库分叉：此处异常 note 步骤名为 'script-error' 而库为 'leg-error'、用同步 process.exit 而库用
process.exitCode、登录超时 45000ms 而库 LOGIN.timeoutMs 为 30000ms —— 下游按 RESULT 行步骤名解析时两种命名并存，骨架级修正（如 A-03
语义）需在多副本同步修改。建议改 import 共享库；若为保持与 round 1（../r4-flow）逐行同构的冻结复验纪律而有意不复用，请在文件头注释明确记录该原因，避免后续轮次误判为遗漏。

-   await page.goto(`${WEB}/login`);
-   await page.fill('#auth-email', EMAIL);
-   await page.fill('#auth-password', PASSWORD);
-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
-   await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });
-   note('login', true, `logged in as ${EMAIL}`);
+ import { login, note as noteLib, runLeg } from '../_browser_lib.mjs';
+ // ... 每腿仅保留本腿特有断言，登录/A-03/RESULT 骨架交给 runLeg


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_02_sync_face.mjs:31-35 ───
[maintainability · low] 与 browser_01 同款问题：登录序列（含选择器
#auth-email/#auth-password/form[aria-label="Login form"]）与 A-03 catch/RESULT/退出码骨架均为
../_browser_lib.mjs 已导出能力的第 3 份内联副本（本腿另有的差异：异常步骤名 'script-error' vs 库 'leg-error'）。选择器一变需同步改
r4-flow2 四处 + r4-flow/r4-flow3 + 库本身，建议统一改 import 共享库，或注释说明冻结 round-1 骨架的复验纪律原因。

-   await page.goto(`${WEB}/login`);
-   await page.fill('#auth-email', EMAIL);
-   await page.fill('#auth-password', PASSWORD);
-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
-   await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });
+ import { login, note as noteLib, runLeg } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_03_paid_face.mjs:31-35 ───
[maintainability · low] 与 browser_01 同款问题：本腿是同一登录/A-03/RESULT 骨架在 ../_browser_lib.mjs
之外的又一内联副本（且异常步骤名 'script-error' 与库的 'leg-error' 分叉）。本腿特有的 waitForSelector('已付款，权益处理中')/billing
断言才是应保留的增量，建议骨架改由共享库提供。

-   await page.goto(`${WEB}/login`);
-   await page.fill('#auth-email', EMAIL);
-   await page.fill('#auth-password', PASSWORD);
-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
-   await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });
+ import { login, note as noteLib, runLeg } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs:30-34 ───
[maintainability · low] 与 browser_01 同款问题：同一登录/A-03/RESULT/退出码骨架的第 4 份内联副本，未复用同 PR 新增且已被
r5-verify/r5b-flowcheck 采用的 ../_browser_lib.mjs（异常步骤名 'script-error' vs 库 'leg-error'、45s vs 库 30s
超时均已分叉）。建议统一 import 共享库，或注释说明为保持 round-1 同构而有意冻结。

-   await page.goto(`${WEB}/login`);
-   await page.fill('#auth-email', EMAIL);
-   await page.fill('#auth-password', PASSWORD);
-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
-   await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 45000 });
+ import { login, note as noteLib, runLeg } from '../_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:130-131 ───
[security · low] 两条 sqlite3 语句均以字符串拼接方式插入来自登录响应的外部输入 UID_B。uuid_shape 白名单（36 位
[0-9a-fA-F-]）当前确实封死了注入面，且注释已声明 CLI 无参数绑定；但这与分支验收红线"数据库查询一律参数绑定、禁止拼接组装 SQL"字面冲突，且防线是"先校验、后拼接"的松耦合 ——
后续在同文件追加使用其他未校验变量（如 TENANT_*、DRAFT_V）的 sqlite3 语句时不会被迫经过该闸门，即成实际注入点。建议：(a) 把"uuid_shape 校验 + sqlite3
执行"收敛为单一函数（如 sql_uuid <uuid> <stmt-template>），使任何新插值点必须过闸；或 (b) 改用 python3 -c 调 sqlite3 模块的真参数绑定执行该
grant（环境已有 python/jq 依赖先例）。

- sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
- say "granted: $(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"
+ sql_uuid() { # sql_uuid <uuid> <sql with one %s placeholder>
+   uuid_shape "$1" || { say "FAIL: refusing SQL interpolation of non-UUID '$1'"; exit 1; }
+   sqlite3 "$DB" "${2//%s/$1}"
+ }
+ sql_uuid "$UID_B" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '%s', 'plan_publish', 'flow-verifier-r4', 1);"
+ say "granted: $(sql_uuid "$UID_B" "select count(*) from commercial_grants where capability='plan_publish' and user_id='%s';") row(s)"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:14-14 ───
[maintainability · low] BACKEND、Lago 地址 :48889、容器名 weknora-lago-82flow-db-1 三处环境拓扑全部硬编码且无 env 覆盖，与同组
browser 腿 FLOW82_WEB/FLOW82_EXPECT_CNY 可覆盖的姿态不一致（browser_01 注释还钉死了 ':5194 -> :8093'
拓扑）。端口漂移或换栈复跑时需改脚本源码，与 FLOW82_R4_PW/DB_PATH/FLOW82_EMAIL_PREFIX
的注入风格不统一。建议同样参数化：BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"、LAGO_API="${FLOW82_LAGO_API:-htt
p://127.0.0.1:48889}"、LAGO_DB_CONTAINER="${FLOW82_LAGO_DB_CONTAINER:-weknora-lago-82flow-db-1}"。

- BACKEND=http://127.0.0.1:8093
+ BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"
+ LAGO_API="${FLOW82_LAGO_API:-http://127.0.0.1:48889}"
+ LAGO_DB_CONTAINER="${FLOW82_LAGO_DB_CONTAINER:-weknora-lago-82flow-db-1}"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:143-143 ───
[security · low] LAGO_KEY 经 docker exec psql 读出后以 -H "Authorization: Bearer $LAGO_KEY" 走 curl
命令行参数，本地 ps 进程列表可观察到完整 key；与本脚本自身对登录密码"jq 构造 + --data @- 走 STDIN 避免 ps 观察"的安全姿态不一致（多轮 seed/replay
脚本同姿势，可一并统一）。虽为实验栈凭据，仍属敏感信息处理标准不统一。建议改为 stdin 传头：curl -s -H @<(printf 'Authorization: Bearer %s\n'
"$LAGO_KEY") …，或 --config - 由 stdin 注入。

- curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
+ curl -s "$LAGO_API/api/v1/plans" -H @<(printf 'Authorization: Bearer %s\n' "$LAGO_KEY") > "$EV/api-03-lago-plans-baseline.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:35-35 ───
[high] prune 过滤条件过宽，会误删兄弟实验栈的 Stripe webhook endpoint。全仓检索确认同一 Stripe 测试账户下至少有两处以含 "weknora" 的
description 铸造 endpoint：settle-evidence/prepare_t9_env.sh（description="weknora t9 integration
secret-mint stand-in (harness-delivered)"）和
deploy/lago-lab/payment-settle-trigger/phases.py（description="weknora t11 local settle probe
{run_id} …"，且 phases.py 会把 endpoint_id 记入 run state 并在 cleanup 阶段对其执行 DELETE）。本脚本的 `"weknora" in
description` 会把这些非本栈资源一并删除：t11 lab 记录的 endpoint id 被提前删掉后，其 cleanup 阶段的 DELETE 将 404 而记为
failed，直接污染该轮 cleanup 证据；同时构成跨栈资源破坏。建议把过滤收窄到本脚本自建的资源特征（如 description 精确匹配 "weknora r4 flow
secret-mint stand-in"，或匹配本栈 URL 前缀 https://lago-r4-local.invalid）。

- if "weknora" in str(row.get("description", "")):
+ for row in rows:
+     if str(row.get("description", "")).startswith("weknora r4 flow secret-mint stand-in") \
+        or str(row.get("url", "")).startswith("https://lago-r4-local.invalid"):


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:55-56 ───
[security · medium] 新铸造的 Stripe webhook secret（whsec_…）以 `-e WSECRET="$SECRET"` 命令行 argv 形式传入 docker
compose exec，在本机 ps 输出中于 exec 生命周期内可见。同 PR 的 deploy/lago-lab/payment-settle-trigger/phases.py 已按 OCR
r2 结论改为 secret 经 STDIN 传入、Ruby 侧用 STDIN.read 取值（注释明确："never the docker exec argv (-e WSECRET=… is
visible in the local process list …)"），本脚本（晚于该修复新增）却重新引入了 argv 传密写法，也与 seed.sh 中防御 curl argv
密码的姿态不一致。建议改为 stdin 传递。

- docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T -e WSECRET="$SECRET" api bin/rails runner \
-   "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: ENV['WSECRET'])" >/dev/null
+ printf '%s' "$SECRET" | docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
+   "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: STDIN.read)" >/dev/null


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:11-11 ───
[low] COMPOSE_FILE 直接取 $(pwd)，未像同目录 seed.sh 那样 `cd "$(dirname "$0")/../../../.."` 归一到 worktree
根（本脚本为可执行 +x，而模板来源 prepare_t9_env.sh 是文档约定从仓库根 source 的 source-only 脚本，二者前提不同）。从任意非根目录执行会以 compose
"file not found" 报错，错误语义与真实原因（CWD 依赖）不符。建议用脚本自身位置推导。

- COMPOSE_FILE="$(pwd)/deploy/lago/compose.yaml"
+ COMPOSE_FILE="$(cd "$(dirname "$0")/../../../.." && pwd)/deploy/lago/compose.yaml"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:16-17 ───
[low] 读取存量 secret 时同时丢弃 stderr（2>/dev/null）并让管道末端的 sed 吞掉 docker exec 的非零退出码：rails runner 因容器未就绪/DB
抖动等瞬时故障失败与"provider 确实无存储 secret"两种情形不可区分，都会落入铸造+update! 覆盖分支——可能把上一轮已落库且仍有效的 webhook secret
静默覆盖为新值，并额外消耗 Stripe 测试账户 16 个 endpoint 的配额。建议先检查读取命令自身是否成功，失败即显式退出。

- STORED=$(docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
-   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s" 2>/dev/null | sed -n 's/.*__R4SECRET__//p')
+ STORED_OUT=$(docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
+   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s") \
+   || { echo "FAIL: stored-secret read failed (container/rails runner error)" >&2; exit 1; }
+ STORED=$(sed -n 's/.*__R4SECRET__//p' <<<"$STORED_OUT")


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:33-34 ───
[maintainability · low] 删除 runs/ 回退后清理不彻底，遗留三处残留：1) 第 11 行 `import glob` 的唯一调用点（`matches =
sorted(glob.glob(...))`）已随本次改动删除，导入现无任何引用；2) 第 18 行模块级常量 `RUNS` 只在被删除的回退分支中使用，现在只定义不读取；3) `archived`
定义上方的注释仍写 "Fall back to the newest runs/ TSV only when no archive exists, and always print the file
actually used"，与紧随其后的 R3-17 行为（无归档即 exit 2、绝不探测 runs/）直接矛盾，易让后续维护者误以为回退路径仍存在。ocr-issue-82-r1.md 对
ocr2 副本的同类发现已明确要求随本次改动一并删除这两个未用声明并修正该段注释。建议删除 `import glob` 与 `RUNS`，并把注释改写为仅描述"仅读本目录归档 TSV，缺失即
MISSING-EVIDENCE"。



─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:40-41 ───
[maintainability · low] 删除 runs/ 回退后清理不彻底，遗留三处残留：1) 第 18 行 `import glob` 的唯一调用点（`matches =
sorted(glob.glob(...))`）已随本次改动删除，导入现无任何引用；2) 第 25 行模块级常量 `RUNS` 只在被删除的回退分支中使用，现在只定义不读取；3) `archived`
定义上方的注释（第 30-31 行）仍写 "Fall back to the newest runs/ TSV only when no archive exists"，与紧随其后的 R3-17
行为（无归档即 exit 2、绝不探测 runs/）直接矛盾；docstring 第 14-15 行 "the runs/ observer TSV is only a fallback when
no archive exists" 同样已过时。建议删除 `import glob` 与 `RUNS`，并同步修正这两处注释为"仅读本目录归档 TSV，缺失即 MISSING-EVIDENCE"。



─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:37-38 ───
[maintainability · low] 删除 runs/ 回退后清理不彻底，遗留三处残留：1) 第 15 行 `import glob` 的唯一调用点（`matches =
sorted(glob.glob(...))`）已随本次改动删除，导入现无任何引用；2) 第 22 行模块级常量 `RUNS` 只在被删除的回退分支中使用，现在只定义不读取；3) `archived`
定义上方的注释（第 27 行）仍写 "Fall back to the newest runs/ TSV only when no archive exists"，与紧随其后的 R3-17
行为（无归档即 exit 2、绝不探测 runs/）直接矛盾。ocr-issue-82-r1.md 对本副本的发现（1022-1026
行）已明确要求随本次改动一并删除这两个未用声明并修正该段注释。建议删除 `import glob` 与 `RUNS`，并把注释改写为仅描述"仅读本目录归档 TSV，缺失即
MISSING-EVIDENCE"。



─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:37-38 ───
[maintainability · low] 删除 runs/ 回退后清理不彻底，遗留三处残留：1) 第 15 行 `import glob` 的唯一调用点（`matches =
sorted(glob.glob(...))`）已随本次改动删除，导入现无任何引用；2) 第 22 行模块级常量 `RUNS` 只在被删除的回退分支中使用，现在只定义不读取；3) `archived`
定义上方的注释（第 27 行）仍写 "Fall back to the newest runs/ TSV only when no archive exists"，与紧随其后的 R3-17
行为（无归档即 exit 2、绝不探测 runs/）直接矛盾。建议删除 `import glob` 与 `RUNS`，并把注释改写为仅描述"仅读本目录归档 TSV，缺失即
MISSING-EVIDENCE"。



─── docs/plans/issue-72-flow-evidence-74/verify_ac_assertions.py:68-71 ───
[documentation · low] 断言标签 "AC1 invoice open/pending/numberless" 未随条件放宽更新：新增的 or 臂还接受 invoice_status
为 "failed"/"closed" 的可见终局证据，而 check() 的 PASS/FAIL 输出以该标签为准——一份 failed-endgame 的合法证据通过时会打印 "PASS AC1
invoice open/pending/numberless"，与实际断言内容不符，误导取证排查。建议把标签改为覆盖两种形态。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or terminal failed/closed",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:71-74 ───
[documentation · low] 断言标签 "AC1 invoice open/pending/numberless" 未随条件放宽更新：新增的 or 臂还接受 invoice_status
为 "failed"/"closed" 的可见终局证据，而 check() 的 PASS/FAIL 输出以该标签为准——一份 failed-endgame 的合法证据通过时会打印 "PASS AC1
invoice open/pending/numberless"，与实际断言内容不符，误导取证排查。建议把标签改为覆盖两种形态。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or terminal failed/closed",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:73-76 ───
[documentation · low] 断言标签 "AC1 invoice open/pending/numberless" 未随条件放宽更新：新增的 or 臂还接受 invoice_status
为 "failed"/"closed" 的可见终局证据，而 check() 的 PASS/FAIL 输出以该标签为准——一份 failed-endgame 的合法证据通过时会打印 "PASS AC1
invoice open/pending/numberless"，与实际断言内容不符，误导取证排查。建议把标签改为覆盖两种形态。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or terminal failed/closed",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:82-85 ───
[documentation · low] 断言标签 "AC1 invoice open/pending/numberless" 未随条件放宽更新：新增的 or 臂还接受 invoice_status
为 "failed"/"closed" 的可见终局证据，而 check() 的 PASS/FAIL 输出以该标签为准——一份 failed-endgame 的合法证据通过时会打印 "PASS AC1
invoice open/pending/numberless"，与实际断言内容不符，误导取证排查。建议把标签改为覆盖两种形态。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice open/pending/numberless or terminal failed/closed",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_04_active_face.mjs:19-21 ───
[test · medium] 缺失 (A-03) 防护，属跨轮复制漂移：同组 browser_01/02/03 均在 catch 中 note('script-error', ...) 并保证
RESULT 摘要与退出码逻辑执行，上一轮 r4-flow2/browser_04_active_face.mjs 也有完整 catch；本脚本只有 try/finally。若 "权益已生效" 45s
内未出现（waitForSelector 超时即基础设施失败），异常将以未捕获形式终止：不打印 RESULT 行、不落 script-error
截图，仅剩裸堆栈，使断言失败与脚本基础设施失败不可区分，直接违反本组脚本自述的 A-03 约定，削弱 ROUND 3 AC1 尾段证据在故障场景下的可判读性。建议对齐其余三段脚本：将 page 提升到
try 外声明，catch 中记录 FAIL note 与截图。

  const browser = await chromium.launch();
+ let page;
  try {
-   const page = await (await browser.newContext()).newPage();
+   page = await (await browser.newContext()).newPage();
+   // …正文不变…
+ } catch (err) {
+   // (A-03) Infrastructure failure → explicit FAIL note; the RESULT summary
+   // still prints so an assertion failure is distinguishable from a script
+   // infrastructure failure.
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}07-billing-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
+ } finally { await browser.close(); }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:18-18 ───
[maintainability · medium] 四个 browser_*.mjs 各自内联重复登录流程、note/RESULT/exit 样板（每脚本约 20-30
行），而同一目录上层本次更新已新增共享库 _browser_lib.mjs（导出 login/note/runLeg/envRequired，runLeg 内置 A-03 catch
包装），r5-verify 与 r5b-flowcheck 共 9 个脚本均已复用。r4-flow3 未复用属遗漏而非项目约定；同一断言/防护的修复需在
r4-flow/r4-flow2/r4-flow3 × 4 脚本共 12 处同步——browser_04 丢失 catch
正是这种漂移的实例。建议后续轮次统一改用共享库（本轮证据已冻结则从下一轮开始收敛）。

- const note = (step, ok, detail) => { results.push({ step, ok, detail }); console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`); };
+ import { login, note, runLeg, evidencePath, envRequired } from '../_browser_lib.mjs';
+ // note(results, step, ok, detail) / 登录流程 / (A-03) catch 包装 / RESULT 合约
+ // 收敛到共享库一份，避免 12 份拷贝漂移（r4-flow3 browser_04 丢 catch 即实例）。


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:127-127 ───
[security · medium] 外部输入 $UID_B 以字符串插值拼入两条 sqlite3 语句。已核实当前两个调用点均在 uuid_shape
校验之后、护栏有效，但分支安全验收条件明确"数据库查询一律使用参数绑定、禁止拼接 SQL"，且该模式已在
r4-flow/r4-flow2/r5-verify/r5b-flowcheck/seed_83 共 6 个取证脚本中复制——后续任何人在护栏之前新增插值调用点即形成 SQL
注入面，白名单护栏无法保护新增点。建议改用 python3 的 sqlite3 模块（原生 ? 绑定）执行 grant 写入与回读，从根上满足验收条件，uuid_shape 校验可保留为纵深防御。

- sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
+ python3 - "$DB" "$UID_B" <<'PY'
+ import sqlite3, sys
+ db, uid = sys.argv[1], sys.argv[2]
+ conn = sqlite3.connect(db)
+ conn.execute(
+     "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) "
+     "values (0, ?, 'plan_publish', 'flow-verifier-r4', 1)",
+     (uid,),
+ )
+ conn.commit()
+ print("granted:", conn.execute(
+     "select count(*) from commercial_grants where capability='plan_publish' and user_id=?",
+     (uid,),
+ ).fetchone()[0], "row(s)")
+ PY


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:140-140 ───
[security · low] LAGO_KEY（Lago 管理 API key）以 -H "Authorization: Bearer $LAGO_KEY" 落在 curl 命令行参数中，本机
ps 输出可见；而本脚本对登录密码已刻意采用 STDIN（--data @-）传递并注明理由（"a curl argv password is observable in local
ps"），同一文件内敏感信息暴露姿势不一致。两处 GET 调用（本行 baseline 与第 165 行）可改用 curl --config - 经 stdin 传入 header，key 既不进
argv 也不落盘。

- curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
+ curl -s "http://127.0.0.1:48889/api/v1/plans" --config - > "$EV/api-03-lago-plans-baseline.json" <<CFG
+ header = "Authorization: Bearer $LAGO_KEY"
+ CFG


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:14-14 ───
[maintainability · low] 基础设施参数全部硬编码且不可用环境变量覆盖：BACKEND=127.0.0.1:8093、Lago 端口 48889、容器名
weknora-lago-82flow-db-1、psql 用户 lago、计划金额 9900/9900000 均为字面量。同目录 browser_*.mjs 已采用 env
带默认值姿势（FLOW82_WEB ?? default），本脚本与该约定不一致；且这些字面量在 r4-flow/r4-flow2/r5-verify/r5b-flowcheck
各脚本中重复出现，更换端口/容器即需逐文件修改。建议至少为 BACKEND、Lago API 地址与容器名提供可覆盖的环境变量默认值。

- BACKEND=http://127.0.0.1:8093
+ BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"
+ LAGO_API="${FLOW82_LAGO_API:-http://127.0.0.1:48889}"
+ LAGO_DB_CONTAINER="${FLOW82_LAGO_DB_CONTAINER:-weknora-lago-82flow-db-1}"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:11-11 ───
[maintainability · low] COMPOSE_FILE 直接取 $(pwd)，未像同目录 seed.sh 那样 `cd "$(dirname "$0")/../../../.."`
归一到 worktree 根（模板来源 prepare_t9_env.sh 是文档约定从仓库根 source 的 source-only 脚本，本脚本是可执行 +x，前提不同）。从任意非根目录执行会以
compose "file not found" 形式失败，报错语义与真实原因（CWD 依赖）不符。建议用脚本自身位置推导。

- COMPOSE_FILE="$(pwd)/deploy/lago/compose.yaml"
+ COMPOSE_FILE="$(cd "$(dirname "$0")/../../../.." && pwd)/deploy/lago/compose.yaml"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:131-131 ───
[security · medium] 两条 sqlite3 语句均以 '$UID_B' 字符串插值组装 SQL，与项目安全红线"数据库查询一律参数绑定、禁止拼接组装 SQL"直接冲突；当前仅靠
uuid_shape UUID 白名单这一道单点防线兜底（虽然该正则可证明排除注入字符）。sqlite3 CLI 自 3.32 起支持 .parameter
绑定，可用它消除拼接；白名单可保留作为纵深防御。

- sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
+ sqlite3 "$DB" \
+   ".parameter init" \
+   ".parameter set @uid '$UID_B'" \
+   "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, @uid, 'plan_publish', 'flow-verifier-r4', 1);" \
+   "select count(*) from commercial_grants where capability='plan_publish' and user_id=@uid;"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_03_paid_face.mjs:55-56 ───
[test · medium] 本脚本只有 try/finally 而缺少同组 browser_02、browser_04 以及 r4-flow2/browser_03_paid_face.mjs
均已落地的 (A-03) catch 兜底：登录被拒或选择器超时等脚手架故障将以未捕获异常抛出，RESULT 汇总 JSON
不会打印（仅剩非零退出码），也不留存错误现场截图——下游读者无法区分"断言失败"与"基础设施故障"，破坏本组证据脚本自我约束的故障可观测性约定（属遗漏而非约定差异）。

    await page.screenshot({ path: `${EV}05-billing-paid-awaiting-activation.png`, fullPage: true });
+ } catch (err) {
+   // (A-03) 与 leg 02/04 相同：基础设施故障落为显式 FAIL note，RESULT 汇总仍打印、退出码非零。
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}04-paid-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
  } finally {


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:76-76 ───
[maintainability · low] 死代码：全仓检索（含 apps/web 前端代码）确认 window.__r4PurchasesSeen 无任何赋值点，该
waitForFunction 必然超时且被 .catch(() => {}) 静默吞掉——抓包实际完全依赖上方的 page.on('request') 监听。此行已在 OCR r1/r2
两轮评审中被标记为应删除的死代码却仍残留，每次运行空耗约 1 秒并误导读者以为存在页面侧埋点，建议直接删除。

-   await page.waitForFunction(() => window.__r4PurchasesSeen > 0, null, { timeout: 1000 }).catch(() => {});
+   （删除该行：purchases 的捕获与断言已由 page.on('request') 监听与 purchase-wire-provider-alipay 断言完整覆盖。）


─── apps/web/src/commercial/CheckoutPage.tsx:92-94 ───
[maintainability · high] [HIGH] 前端以英文自由文本整句作为精确匹配键，与 internal/handler/commercial.go 的 409/500
错误措辞形成跨层脆弱耦合。已核实：POST /purchases 现有 9 个令牌虽逐字吻合，但后端同一 sentinel（ErrQuoteVersionConflict）在
CreateOrder/ChangePlan 已存在多种措辞变体（L469/L819/L915），措辞漂移风险真实存在；且 L513 的可重试冲突 "purchase pending exists"
未收录，只能落到通用兜底。更严重的是 isQuoteLevelConflict 失配时不会清空 quoteRef——报价级冲突对同一 quote 是确定性
409，用户点「重试」将永远复用同一失效报价，形成死循环，直接违背 R3-07 的设计目标。建议：与 503 reason 信封同构，让后端为 409/500 族也输出稳定机器令牌（顶层 reason
或 code 字段），前端按令牌而非 message 原文匹配；至少补录 "purchase pending exists" 并将令牌集中为与后端同步维护的共享常量。

- const PURCHASE_CONFLICT_MESSAGES: Record<string, string> = {
-   'quote expired': '报价已过期，请重新获取报价',
-   'invoice_quote_mismatch': '订单金额与报价不一致，请重新发起购买',
+ // 后端 409/500 族复用 503 信封形状输出稳定令牌：
+ // c.JSON(http.StatusConflict, gin.H{"error": "quote expired", "reason": "quote_expired"})
+ // 前端优先按 details.reason 令牌命中，message 原文仅作兼容兜底：
+ const PURCHASE_CONFLICT_TOKENS: Record<string, string> = {
+   quote_expired: '报价已过期，请重新获取报价',
+   invoice_quote_mismatch: '订单金额与报价不一致，请重新发起购买',
+   purchase_pending_exists: '存在处理中的购买，请稍后刷新重试',
+   // …与后端闭集同步维护
+ };


─── apps/web/src/commercial/CheckoutPage.tsx:28-30 ───
[maintainability · medium] [MEDIUM] order-state.ts 的 PURCHASE_STATE_LABEL 注释声明自身是唯一共享词表（D15-f："no
page keeps a private copy of the words"），但此处又私有持有一套三态文案且用词不一致：paid_awaiting_activation
为「已付款，权益处理中」vs 词表「已付款待激活」、awaiting_payment 为「权益未开通」vs 词表「权益未开放」、active 为「权益已生效」vs「已生效」——同一产品状态在
BillingPage 与 CheckoutPage 向用户展示两套词汇，后续改词必然漂移，D15-f 的收敛目标落空。另 canceled 的祈使句长文案作为 BillingPage
套餐行后缀（「套餐名 · 该购买已取消，请重新发起购买」）语义生硬。建议 purchaseStateMessage 直接消费 PURCHASE_STATE_LABEL（canceled
完整句与短标签可分离）。

  export function purchaseStateMessage(purchase: PurchaseView | undefined, order: OrderView): string {
+   // 消费共享词表（D15-f）：三态与 canceled 均取 PURCHASE_STATE_LABEL
    switch (purchase?.state) {
-     case 'paid_awaiting_activation': return '已付款，权益处理中';
+     case 'awaiting_payment':
+     case 'paid_awaiting_activation':
+     case 'active':
+     case 'canceled':
+       return PURCHASE_STATE_LABEL[purchase.state];
+     default:
+       if (order.payment === 'pending') return PURCHASE_STATE_LABEL.awaiting_payment;
+       if (order.payment === 'paid') return PURCHASE_STATE_LABEL.paid_awaiting_activation;
+       if (order.fulfillment === 'fulfilled') return PURCHASE_STATE_LABEL.active;
+       return orderMessage(order);
+   }
+ }


─── apps/web/src/commercial/CheckoutPage.tsx:269-273 ───
[bug · medium] [MEDIUM] state.purchase 存在多个异步写入点（run() 深链后台拉取、POST 后初次 setState、此处 refreshOrder 的嵌套
purchaseStatus、轮询 effect），各写入仅有 scope 守卫而无时序/版本控制——较慢的旧 purchaseStatus 响应晚到会把新状态覆盖回旧状态（如
paid_awaiting_activation 回退为 awaiting_payment），需等下一拍轮询自愈，期间三态文案与支付入口隐藏逻辑可能出现瞬时错误呈现。建议对 purchase
更新施加单调状态序守卫（awaiting_payment < paid_awaiting_activation < active，终态不回退）或引入请求序号。另：getOrder.then 内嵌
purchaseStatus.then 的两层嵌套 Promise 链建议改写为 async/await 顺序结构，符合项目「优先 async/await、禁止回调地狱」的异步规范。

-       void client.commercial.purchaseStatus(currentScope.signal).then((purchase) => {
-         if (live()) {
-           setState((prev) => (prev.status === 'ready' ? { ...prev, purchase } : prev));
+   const refreshOrder = useCallback(async (): Promise<void> => {
+     const id = orderIdRef.current;
+     if (!id) return;
+     const currentScope = scopeController.current();
+     const live = (): boolean =>
+       !currentScope.signal.aborted && scopeController.isCurrent(currentScope.scope);
+     try {
+       const order = await client.commercial.getOrder(id, currentScope.signal);
+       if (!live()) return;
+       setState((prev) => (prev.status === 'ready' ? { ...prev, order } : prev));
+       try {
+         const purchase = await client.commercial.purchaseStatus(currentScope.signal);
+         if (live()) applyPurchaseMonotonic(purchase); // 按状态序单调应用，拒绝回退
+       } catch { /* silent degrade */ }
+     } catch (error: unknown) {
+       if (!live()) return;
+       // …
-         }
+     }
-       }).catch(() => { /* silent degrade */ });
+   }, [client, scopeController]);


─── apps/web/src/commercial/CheckoutPage.tsx:287-289 ───
[bug · low] [LOW] restartCheckout 清空了 orderIdRef 与 quoteRef 但未重置
submittedChannel——重启后到新一次提交前该值仍指旧渠道，若期间订单重新加载为 pending，失配横幅会基于过期信息渲染。另外渲染层存在重复入口：当订单 pending、无有效
checkout_url、且用户切换过渠道单选时，「渠道失配横幅」（含「改用X重新发起支付」）与「支付渠道异常」块（含「重新发起支付」）同时渲染，出现两条错误提示与两个语义等价的按钮。建议补
setSubmittedChannel(null)，并让两个渲染块互斥（如失配横幅仅在存在可用支付链接时展示，或合并为一个入口）。

    const restartCheckout = useCallback((): void => {
      orderIdRef.current = null;
      quoteRef.current = null;
+     setSubmittedChannel(null);
+     setState({ status: 'loading' });
+     setRetryToken((value) => value + 1);
+   }, []);


─── apps/web/src/commercial/CheckoutPage.tsx:138-138 ───
[bug · low] [LOW] 注释声称「原始 message 仅进 console」，但未映射已知令牌的 ApiError（status ≥ 400）分支在下方 console.warn
之前就直接返回了 PURCHASE_ERROR_FALLBACK——未知服务端令牌静默降级为通用文案且不留任何控制台痕迹。这恰好削弱了跨层 message
令牌耦合（PURCHASE_CONFLICT_MESSAGES）发生措辞漂移时的可诊断性：漂移发生后既无用户侧文案也无日志可循。建议在该分支 return 前补 console.warn 记录
status 与原始 message。

-     if (error.status !== undefined && error.status >= 400) return PURCHASE_ERROR_FALLBACK;
+     if (error.status !== undefined && error.status >= 400) {
+       console.warn('checkout purchase failed:', error.status, error.message);
+       return PURCHASE_ERROR_FALLBACK;
+     }


─── apps/web/src/commercial/CheckoutPage.tsx:334-334 ───
[style · low] [LOW] 支付渠道 fieldset 使用全常量的静态内联 style（border/borderRadius/margin/padding 均非动态值），违反
React 规范「除动态样式外避免内联 style」。建议提取为 CSS 类（项目已有 wk-* 类体系或所在样式文件）。

-               <fieldset style={{ border: '1px solid #e7e7ea', borderRadius: 8, margin: '12px 0', padding: '8px 12px' }}>
+ <fieldset className="wk-checkout-channel-fieldset">


─── deploy/lago-lab/payment-settle-trigger/run_lab.py:319-320 ───
[bug · medium] 密钥扫描失败时 overall 被降级为 "fail" 并以退出码 1 返回，但第二次 write_json 只补充了 secrets_scan
字段，environment["run"]["overall"] 仍在首次构造时固化为旧值（如 "pass"）。最终 t11-environment.json 记录的 verdict
与进程退出码不一致，会误导后续证据归档与人工复核（这个 lab 的核心产物就是证据文件）。建议在重写前同步更新 run.overall。

      # rewrite timeline + environment with the final scan result
+     environment["run"]["overall"] = overall
      environment["secrets_scan"] = scan


─── deploy/lago-lab/payment-settle-trigger/run_lab.py:27-31 ───
[maintainability · low] import subprocess 在本文件没有任何使用点（docker/psql 子进程都在 phases.py
中派生），属死导入，会误导读者认为执行器本身派生子进程。

  import argparse
  import json
  import os
- import subprocess
  import sys


─── .gitignore:102-103 ───
[maintainability · medium] 新增的无锚定 data/ 规则会忽略仓库任意层级名为 data
的目录，而仓库中已存在被跟踪的此类目录内容（internal/textconv/data/
的词典文件、internal/modules/agentruntime/agent/persona/data/ 的
profiles/questions）。已跟踪文件不受影响，但维护者未来向这些目录新增数据文件时会被静默排除在 git status 之外，造成遗漏提交。另外该行与上方注释（仅解释
seed-login 捕获文件）意图不符且自身无说明。若意图只是忽略根目录的本地数据库目录（上方 43-46 行已有 data/files/、data/weknora.db 等规则），建议锚定为
/data/ 并补充注释。

  docs/plans/**/seed-login-*.json
- data/
+ # 根目录本地运行数据（SQLite 等），不波及 internal/ 下的已跟踪 data 目录
+ /data/


─── deploy/lago-lab/payment-activation/phases.py:575-577 ───
[test · low] ps 赋值后从未使用（本函数既有代码惯用 _ 占位，如 1202/1228 行）；更重要的是 _payments_for 在 HTTP 非 2xx（如瞬时
5xx/鉴权失败）时返回空列表而非抛错，此时 non_succeeded 恒为空、len ≤ 1 恒真，新增的 exactly-once 断言会静默空转（假阴性通过），与 "the payments
API is NOT affected ... so AC1's exactly-once line is asserted on this path" 的设计意图相悖。建议至少把 ps 记入
observed，或在非 2xx 时不计入该断言判定。

              ps, payments = _payments_for(ctx, customer["external_id"])
+             observed["payments_http_status"] = ps
              non_succeeded = [p for p in payments if p.get("status") != "succeeded"]
              observed["payments_non_succeeded_count"] = len(non_succeeded)


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:103-103 ───
[security · high] SQL 拼接违反分支安全红线，且与本文件头注释及紧邻的安全注释自相矛盾。头注释声称 "sqlite3 ?1 parameter binding"（OCR r2
fixes applied），而 OCR r2 修复记录（docs/plans/issue-72-ocr-issue-82-r2.md:746-747）确实把这一写法改成了 `values (0,
?1, ...) " $UID_B"` 的 CLI 尾参绑定 —— 说明 sqlite3 CLI 是支持参数绑定的，第 100-102 行 "the shell's sqlite3 CLI has
NO usable parameter binding" 的注释不成立。本脚本退回到 '$UID_B' 拼接：a) 违反本次迁移分支的验收红线"数据库查询一律使用参数绑定，禁止拼接 SQL"；b)
注释声称的加固并未落实，会误导后续轮次复制该模式。select count 行同理。

- sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r5', 1);"
+ sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, ?1, 'plan_publish', 'flow-verifier-r5', 1);" "$UID_B"
+ say "granted: $(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id=?1;" "$UID_B") row(s)"


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_04_active_face.mjs:12-12 ───
[bug · medium] `ORDER = process.argv[2] ?? ''`
的宽容缺省会让空参数静默通过，并触发本脚本声称要避免的行为。前端路由（apps/web/src/router.tsx:690）对 `?order=` 空值返回空串，CheckoutPage 的
orderIdRef 对空串走 falsy 分支 → auto-submit 一笔新购买 —— 恰好违背脚本自己注释的前提 "an active purchase must never
auto-submit a new one"，还会在隔离环境里制造一笔计划外的订单污染后续证据。browser_02/browser_03 对同一参数都有 usage 退出，此处应保持一致；末尾的
`if (ORDER) assertOrderIdentity` 条件防护可随之去掉。

- const ORDER = process.argv[2] ?? '';
+ const ORDER = process.argv[2];
+ if (!ORDER) { console.error('usage: node browser_04_active_face.mjs <orderId>'); process.exit(2); }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_01_checkout.mjs:62-63 ───
[bug · medium] orderID 未判空即输出锚点。_browser_lib.mjs 的 assertOrderIdentity 在提取失败时不抛错，只记一条 FAIL note
并返回哨兵字符串 '(no order id visible)'，脚本随后仍会打印 `ANCHOR_ORDER_ID=(no order id visible)`。runLeg 虽会把退出码置
1，但下游 shell 若只 grep ANCHOR_ORDER_ID 不检查退出码，就会把这个垃圾值当订单号拼进 leg02-04 的 URL，导致 same-order
断言整体失真且难以定位。建议提取失败时立即中止本 leg。

    // The order id is the anchor every later leg must re-observe (A-12).
+   if (!orderID.startsWith('ord_')) { throw new Error('order id anchor missing — refusing to emit ANCHOR_ORDER_ID'); }
    console.log(`ANCHOR_ORDER_ID=${orderID}`);


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:8-9 ───
[documentation · low] 头注释与代码行为矛盾：注释称 "fresh db: pad 0 is enough for an isolated db"，但下方校验强制
FLOW82_PAD_COUNT >= 1（`[ "$PAD" -ge 1 ]` 会拒绝 0）。按注释语义 pad 0 应被允许（隔离库上 0
个占位即够），复用者会被误导。建议二选一：改注释与校验一致，或把校验放宽为 >= 0 并让 seq 空序列自然跳过占位注册。

- # Registers FLOW82_PAD_COUNT placeholders (fresh db: pad 0 is enough for an
- # isolated db — the default keeps the r4 shape) then the two protagonists
+ # Registers FLOW82_PAD_COUNT placeholders (>= 1 enforced below; on an
+ # isolated fresh db even 1 keeps the r4 shape) then the two protagonists


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:18-18 ───
[maintainability · low] BACKEND、Lago 端口 48889 与容器名 weknora-lago-82r5-db-1 均硬编码且无 env 覆盖，而同目录 browser
腿的 WEB 地址可通过 FLOW82_WEB 覆盖 —— 同一轮证据的两侧配置姿态不一致，换端口/容器名重放时必须改脚本源码。建议与 FLOW82_WEB 对齐，提供带默认值的 env 覆盖。

- BACKEND=http://127.0.0.1:8093
+ BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"
+ LAGO_BASE="${FLOW82_LAGO_BASE:-http://127.0.0.1:48889}"
+ LAGO_DB="${FLOW82_LAGO_DB_CONTAINER:-weknora-lago-82r5-db-1}"


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_01_checkout.mjs:28-28 ───
[maintainability · low] 空 catch 的注释 "captured below" 与事实不符：下方没有任何兜底捕获或重试。若 purchases 请求体不是合法
JSON（或事件时序错过），purchaseProvider 静默保持空串，断言失败时只剩 provider="" 一行 detail，缺乏诊断线索，且注释会误导排查方向。至少应在 catch
中输出原始 postData 片段。

-       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; } catch { /* captured below */ }
+       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; }
+       catch { console.warn('purchases request body is not JSON:', req.postData()?.slice(0, 120)); }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_01_checkout.mjs:58-59 ───
[documentation · low] 脚本对 checkout 页断言 '待付款（权益未开通）'（对应 CheckoutPage.tsx:32 的本地映射），对 billing 页断言
'待付款（权益未开放）'（对应 order-state.ts:14 的 PURCHASE_STATE_LABEL）—— 前端两处文案本身
近形不一致，本脚本如实固化了这个差异。当前断言与前端现状匹配不会失败，但一旦前端统一文案（大概率统一为其中一个），本断言和 browser_04 的
billing-no-stale-paid-copy 断言都会连带失败，且证据链固化了疑似文案笔误。建议与前端确认 '开通/开放'
的取舍后在脚本或前端统一，或至少在脚本注释标注该差异来自前端双文案现状。



─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_03_paid_face.mjs:40-41 ───
[test · low] 对整页 innerText 的否定断言隐含"该租户仅此一笔购买"的前提：seed.sh 的 400 already-exists
重跑容差会复用已有账号，此时账号下若残留早前轮次的已生效套餐行，billing 页会同时渲染多行，负向断言 !includes('已生效') 将误报
FAIL（尽管本笔购买状态正确）。browser_04 的 billing-no-stale-paid-copy 断言同理。建议把否定断言限定到目标 plan row 那一行文本（类似
billing-paid-awaiting-activation 里的逐行 find），或在前置说明该前提由 fresh db 保证。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:12-13 ───
[bug · medium] EV 的解析顺序存在路径错位 bug：先 `cd` 到 worktree root、再回头用 `$(dirname "$0")` 计算 EV，依赖 $0 是相对原 cwd
的路径。当操作者在脚本所在目录内以 `./seed.sh`（或 `bash seed.sh`）调用时，`dirname "$0"` 为 `.`，此时 `cd .` 解析到的是已切换后的
cwd（worktree root）而非 r4-flow2 证据目录——seed-run.txt、seed-login-a/b.json、api-0*.json 会全部错位写到仓库根目录，而
browser 腿的截图（用 import.meta.url 解析）仍在正确位置，证据集合被拆散且不易察觉。应在 cd 之前先固化 EV（或基于 $BASH_SOURCE 用绝对路径 cd）。

- cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
- EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
+ EV="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # evidence lands beside this script (per-round dir)
+ cd "$EV/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:92-95 ───
[bug · low] 注释声明的失败路径并未兑现：上方注释说 `|| true` 让传输失败的登录（empty body）"fall through 到下方的显式断言，而非无诊断地 set -e
中止"，但如果 LOGIN_A/LOGIN_B 为空串或非 JSON（curl 失败、后端回 HTML 错误页），本行的 `jq ... <<<"$LOGIN_A"` 会以 parse error
非零退出，set -e 在这里直接中止——到不了第 106/121 行的 "login token/tenant missing" / "token missing/null"
显式断言，操作者看到的是一条无上下文的 jq parse 报错。两行 REDACTED 写文件应容忍非 JSON 输入（或先检查 -n 且可解析），让控制流真正落到显式断言。

  LOGIN_A=$(login "$PREFIX-a@verify.local") || true
  LOGIN_B=$(login "$PREFIX-b@verify.local") || true
- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json"
- jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_B" > "$EV/seed-login-b.json"
+ jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_A" > "$EV/seed-login-a.json" 2>/dev/null || say "WARN: login A body empty/non-JSON (transport failure?) — falling through to explicit assertions"
+ jq '.token = "REDACTED" | .refresh_token = "REDACTED"' <<<"$LOGIN_B" > "$EV/seed-login-b.json" 2>/dev/null || say "WARN: login B body empty/non-JSON (transport failure?) — falling through to explicit assertions"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:34-36 ───
[security · low] 防 ps 观察的声明只对 curl 生效、被 jq argv 抵消：注释以"a curl argv password is observable in local
ps"为由改用 `--data @-`，但 `jq -nc --arg p "$PW"` 本身就把完整密码放进了 jq 进程的命令行参数，本地 ps 同样可见——密码只是从 curl 的 argv
移到了 jq 的 argv，防护形同虚设（login() 第 83 行同款）。若确需规避 ps 观察，应让密码全程走 stdin（如 `printf '%s' "$PW" | jq -Rs --arg
e "$1" '{username:$e,email:$e,password:.}'`），或修正注释以免误导后续轮次沿用该姿态。

-   jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}' \
+   printf '%s' "$PW" | jq -Rs --arg e "$1" '{username:$e,email:$e,password:.}' \
      | curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
          -H 'Content-Type: application/json' --data @-


─── docs/plans/issue-72-flow-evidence-82/browser_flow_82.mjs:9-9 ───
[maintainability · low] 未使用的导入：`fileURLToPath` 在本文件中从未被引用——证据路径计算已重构进 `_browser_lib.mjs` 的
`evidencePath()`（该函数在库内自行调用 fileURLToPath），此处是重构遗留的死导入，应删除。

- import { fileURLToPath } from 'node:url';
+ import { LOGIN, login, note, runLeg, evidencePath, envRequired } from './_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:7-7 ───
[maintainability · low] 未使用的导入：`envRequired` 被导入但全文从未调用——下方凭据校验是手写的 FLOW82_EMAIL_D/FLOW82_EMAIL 回退
if 块（即已确认的重复校验问题）。按该发现的建议将校验改为消费库内回退变体（如 envAny）后此导入才会被真正使用；若维持手写校验，则应从导入列表中移除
envRequired，避免误导读者以为校验走了库函数。

- import { login, note, runLeg, evidencePath, envRequired } from './_browser_lib.mjs';
+ import { login, note, runLeg, evidencePath } from './_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:6-6 ───
[maintainability · low] 未使用的导入：`envRequired` 被导入但全文从未调用——下方凭据校验是手写的 FLOW82_EMAIL_B/FLOW82_EMAIL 回退
if 块。应让校验消费库内回退变体（与 paid-face 腿一并统一），否则从导入列表中移除 envRequired。

- import { login, note, runLeg, evidencePath, envRequired } from './_browser_lib.mjs';
+ import { login, note, runLeg, evidencePath } from './_browser_lib.mjs';


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:90-94 ───
[test · medium] 缺少同族脚本均已落地的 (A-03) catch 兜底：本脚本只有 try/finally，登录被拒或选择器超时等脚手架故障将以未捕获异常抛出——RESULT 汇总
JSON 不打印、不留错误现场截图，下游读者无法区分"断言失败"与"基础设施故障"。同目录 browser_02/browser_04 以及
r4-flow2/browser_01_checkout.mjs、r4-flow3/browser_01_checkout.mjs 均已实现该兜底（全仓唯一缺此兜底的 browser_01
就是本文件），与已确认的 browser_03 缺失同性质，属遗漏而非约定差异。建议补齐（注意 page 目前在 try 内以 const 声明，需先提升为 let page; 再进入 try，与
browser_02/browser_04 一致）。

-   writeFileSync(`${EV}order-info.json`, JSON.stringify({ orderId, checkoutHref, purchases }, null, 2));
-   console.log('ORDER ' + orderId);
+ } catch (err) {
+   // (A-03) 脚手架故障显式落 FAIL note：RESULT 汇总仍打印、退出码非零，
+   // 便于下游区分"断言失败"与"基础设施故障"。
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}02-billing-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
  } finally {
    await browser.close();
  }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_04_active_face.mjs:12-13 ───
[bug · medium] 缺少 orderId 参数时静默回退为空串，与 leg 02/03 的缺参即 exit(2) 行为不一致，且带来实际风险：router.tsx 中 `new
URLSearchParams(...).get('order')?.trim() ?? ''` 会把 `checkout?order=` 解析为空串，CheckoutPage
将其视为"无订单"并自动走新报价 + 新购买提交——这恰好违反本 leg 自己声明的约束（"an active purchase must never auto-submit a new
one"），可能在采证时意外创建新订单污染实验室状态。同时 `if (ORDER)` 会让 A-12 订单身份锚点断言被静默跳过，深链断言失去保护。建议与 leg 02/03
对齐：缺参直接报错退出，并让 assertOrderIdentity 无条件执行。

- const ORDER = process.argv[2] ?? '';
+ const ORDER = process.argv[2];
+ if (!ORDER) { console.error('usage: node browser_04_active_face.mjs <orderId>'); process.exit(2); }
  if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW || !process.env.FLOW82_EXPECT_CNY) {


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_01_checkout.mjs:12-15 ───
[maintainability · low] 重复样板（四个 leg 逐字重复）：FLOW82_EMAIL_A/FLOW82_R5_PW/FLOW82_EXPECT_CNY 环境校验 +
process.exit(2)、`newContext().newPage()` + `setDefaultTimeout(45000)` + `login(...)` 在 browser_01~04
中各出现一次。共享库 _browser_lib.mjs 已导出 envRequired(...)（含同样的 exit(2) 语义与 "no source-code fallback" 提示）却未被这些
leg 使用，两套校验文案已经开始漂移。建议改用 envRequired 收敛校验，页面创建+登录也可下沉为库助手（如 openLegPage(browser, WEB, EMAIL, PW)）。

- if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW || !process.env.FLOW82_EXPECT_CNY) {
-   console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW / FLOW82_EXPECT_CNY');
-   process.exit(2);
- }
+ import { login, note, runLeg, evidencePath, assertOrderIdentity, envRequired } from '../_browser_lib.mjs';
+ // ...
+ const { FLOW82_EMAIL_A: EMAIL, FLOW82_R5_PW: PASSWORD, FLOW82_EXPECT_CNY: EXPECT_CNY } =
+   envRequired('FLOW82_EMAIL_A', 'FLOW82_R5_PW', 'FLOW82_EXPECT_CNY');


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_01_checkout.mjs:28-28 ───
[maintainability · low] catch 注释 `/* captured below */` 具有误导性：下方并不存在任何二次捕获或补救逻辑。若 JSON.parse
失败（如请求体非 JSON），purchaseProvider 将保持空串，checkout-submit-provider 断言会以 provider=""
失败且没有任何可诊断信息（无法区分"请求体未携带 provider"与"请求体解析失败"）。建议在 catch 中记录解析失败并在断言 detail 中带出，便于采证失败时定位。

-       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; } catch { /* captured below */ }
+       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; }
+       catch { providerParseError = 'purchases request body was not valid JSON'; }


─── apps/web/src/commercial/CheckoutPage.tsx:378-379 ───
[bug · low] [LOW] 「支付渠道异常」块对 canceled 购买缺乏防御，与三态文案矛盾：上方 L366 的支付入口已对 `state.purchase?.state ===
'canceled'` 做了防御性隐藏，但本块条件只看 order 渠道面（`payment === 'pending'` 且无有效 checkout_url）。当购买已取消而渠道订单仍
pending、无链接时（如渠道创建失败的订单随后被 sweep
取消后深链回看），页面会同时渲染「该购买已取消，请重新发起购买」与「支付渠道异常，此订单暂无可用支付链接」——两条矛盾的错误提示，且「渠道异常」的诊断本身是错误的（真实原因是取消而非渠道故障）。建议与
L366 的防御对齐，在 canceled 时不渲染渠道异常块（重新发起支付的出路已由 canceled 文案语义覆盖；此为与已确认发现 4「失配横幅 vs 渠道异常块重复入口」不同的
canceled 维度问题）。

-               {state.order.payment === 'pending'
+               {state.purchase?.state !== 'canceled'
+                 && state.order.payment === 'pending'
                  && !(state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url)) ? (


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_04_active_face.mjs:30-31 ───
[test · medium] 本腿头注释与截图命名均声明要验证 credits（"AC1 final state + credits"、"the wallet balance carries the
purchase credit batch"、rv5-05-active-credits.png），但正文没有任何针对钱包余额/积分批次的断言——只检查了 已生效 文案与旧文案消失。且
BillingPage 目前并不渲染钱包/额度信息，contracts 中也没有 credits 读回端点，积分到账这一声明的验收点在证据链上实际完全未被验证。建议二选一：(1) 在本腿或伴随 API
读回中补充积分断言（对照 seed 的 included_credits_micro = PLAN_FEN*1000 验证到账量）；(2) 若本轮不覆盖
credits，修正头注释与截图命名，避免证据链过度声明。



─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_02_sync_face.mjs:35-38 ───
[test · low] 注释声称这里通过 waitForResponse 确定性地等待"页面自身的刷新入口"的响应，但 CheckoutPage 对非终态订单每
3s（POLL_INTERVAL_MS=3000）就由 setInterval 自动轮询同一个 GET /commercial/orders/:id（refreshOrder），该 predicate
同样会命中自动轮询的响应——等到的那次响应不一定是点击触发的刷新，紧随其后的 innerText 读取可能与点击响应的 DOM
更新竞态。若支付状态恰在本腿中途被翻转（正是本腿要排除的情形），存在读到翻转前旧 DOM 而 false PASS 的窗口，削弱了 "deterministic" 的声明。建议在 predicate
中排除点击前发起的请求，例如以点击时刻为界只接受之后发起的请求。

+   const clickAt = Date.now();
    await Promise.all([
-     page.waitForResponse((r) => r.url().includes(`/commercial/orders/${ORDER}`), { timeout: 45000 }),
+     page.waitForResponse((r) => r.url().includes(`/commercial/orders/${ORDER}`)
+       && (r.request().timing()?.startTime ?? 0) >= clickAt, { timeout: 45000 }),
      page.getByRole('button', { name: /刷新订单状态/ }).click(),
    ]);


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_03_restart.sh:38-38 ───
[test · medium] 头部注释声称 "assert the activation/grant/event counts are ALL unchanged"，但脚本只 echo
before/after 值，没有任何比较逻辑——计数变化（例如重启后 outbox 重放产生了第二次 grant/fulfill 事件）时脚本仍以退出码 0
结束，不变量破坏只能靠人工读日志发现，与注释声明的 assert 语义不符。建议捕获 before 快照并在 after 阶段比较，不一致即 exit 1（表名与 kind='fulfill'
查询本身已验证有效，缺的只是失败断言）。

-   echo "after (one drain pass past): order=$(sqlite3 "$DB" "$Q_ORD") activations=$(sqlite3 "$DB" "$Q_ACT") fulfill_events=$(sqlite3 "$DB" "$Q_EVT")"
+ BEFORE="$(sqlite3 "$DB" "$Q_ORD")|$(sqlite3 "$DB" "$Q_ACT")|$(sqlite3 "$DB" "$Q_EVT")"
+ AFTER="$(sqlite3 "$DB" "$Q_ORD")|$(sqlite3 "$DB" "$Q_ACT")|$(sqlite3 "$DB" "$Q_EVT")"
+ [ "$AFTER" = "$BEFORE" ] || { echo "FAIL: state changed across restart ($BEFORE -> $AFTER)"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_03_restart.sh:34-34 ───
[test · low] 健康轮询 40 次后若 code 仍非 200（或为空，例如 start_backend_8093.sh 启动即崩溃），脚本只打印 "health=$code"
就继续执行后续断言阶段，证据会在后端实际未存活的状态下生成。建议在轮询结束后校验 code=200，否则以非零退出，避免产出误导性证据文件。

+ [ "$code" = "200" ] || { echo "FAIL: backend not healthy after restart (health=$code)" | tee -a "$EV/replay-03-restart.txt"; exit 1; }
  echo "health=$code after restart" | tee -a "$EV/replay-03-restart.txt"


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/diag_checkout.mjs:11-11 ───
[test · low] 缺少 FLOW82_EMAIL_A / FLOW82_R5_PW 的存在性校验，与 _browser_lib.mjs 的约定（"the caller validates
the env vars and passes them in"，并提供 envRequired helper）及同目录 browser_02/03/04 脚本的显式校验不一致。env 缺失时将以
Playwright fill(selector, undefined) 的晦涩 TypeError 失败，而非清晰的 "missing required env" 提示。建议复用库内的
envRequired 或在调用前显式校验。

+ import { login, requirePlaywright, envRequired } from '../_browser_lib.mjs';
+ // ...
+ envRequired('FLOW82_EMAIL_A', 'FLOW82_R5_PW');
  await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/diag_checkout.mjs:16-16 ───
[bug · low] 截图路径用 new URL('.', import.meta.url).pathname 拼接：URL.pathname 返回 percent-encoded
字符串，当仓库路径含空格或非 ASCII 字符时会生成错误文件路径（例如 /my%20repo/...），截图静默写到别处或失败。_browser_lib.mjs 内部统一使用
fileURLToPath（见其 evidencePath 实现），建议保持一致，直接复用该 helper。

- await page.screenshot({ path: new URL('.', import.meta.url).pathname + 'diag-checkout.png', fullPage: true });
+ import { fileURLToPath } from 'node:url';
+ await page.screenshot({ path: fileURLToPath(new URL('diag-checkout.png', import.meta.url)), fullPage: true });


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_02_webhook.sh:13-13 ───
[test · low] LAGO_KEY 从 docker exec 读取后没有做非空校验，与同组 seed.sh 的显式判空（[ -n "$LAGO_KEY" ] || exit
1）不一致。当容器未运行或查询失败时 LAGO_KEY 为空串，后续四处 curl 会带着空的 Bearer 头打到 Lago，产生连锁的 jq
解析错误而非清晰的失败原因，降低回放证据的可诊断性。已确认 deliver_stripe_webhook.py 的 --customer/--base/--org/--code
参数签名与本脚本调用匹配，此处仅需补判空。

  LAGO_KEY=$(docker exec weknora-lago-82r5-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')
+ [ -n "$LAGO_KEY" ] || { echo "FAIL: no lago api key readable from weknora-lago-82r5-db-1" >&2; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:173-175 ───
[bug · high] A-09 严格递增断言与本轮实际证据矛盾，且该守卫从未在本次运行中执行过：(1) 本目录 seed-run.txt 的运行记录缺少本脚本必然打印的 "== read Lago
9900-plan baseline (BEFORE this round's publish) ==" 与 "baseline 9900 plans already in Lago: N"
两行，占位注册消息也缺少 "(registered)" 后缀，且 api-03-lago-plans-baseline.json 不存在于 r4-flow3 目录 —— 即证据由未含 A-09
守卫的旧版 seed.sh 产生，脚本与证据版本不一致；(2) 实质问题上，api-03-lago-plans.json 中唯一的 9900 计划
created_at=2026-09-25T03:07:32Z，比本轮 publish receipt（2026-09-26T18:58:19Z）早约 40 小时，且与
r4-flow/r4-flow2 的快照逐字节相同 —— 本轮 publish 未在共享 warm 栈上新增 Lago 计划行，BASE_N=1、N=1，committed
脚本在本轮环境下重跑必然落入本 elif 分支 FAIL exit 1。该断言只在冷/重置栈可行（对照 r5-verify：baseline 为空、0→1 通过）。因此 README 断言表第 1
行"发布 plan→Lago 落库 ✅"实际并未被这份证据证明 —— 恰是 A-09 注释自述要阻止的"跨轮残留假阳性"。建议改为逐轮可观测的到达证明：draft 时向计划注入轮次唯一标记（如
description 携带 round tag）并在发布后断言 Lago 读回该标记（或断言 code 匹配计划的 created_at 晚于 baseline 读取时刻），并用 committed
版本脚本重跑 seed 使证据与脚本一致。

- if [ "$N" -gt "$BASE_N" ]; then
-   say "lago 9900 plan count increased ($BASE_N -> $N): THIS round's publish arrived"
- elif [ "$N" -ge 1 ]; then
+ # 到达证明不应依赖"计划行数递增"（同 code 重发布在 warm 栈上是原地更新，计数恒不变）：
+ # draft 时注入轮次唯一标记，发布后断言 Lago 中该 code 的计划携带本轮标记。
+ MARK="r4-flow3-$(date -u +%H%M%S)"   # 或直接用固定轮次标记 "r4-flow3"
+ # ... draft payload 增加 "description":"$MARK" ...
+ MATCHED=$(jq -r --arg m "$MARK" '[.plans[] | select(.code=="weknora-pro-v1" and .description==$m)] | length' "$EV/api-03-lago-plans.json")
+ [ "$MATCHED" -ge 1 ] || { say "FAIL: this round's publish marker '$MARK' not visible on weknora-pro-v1 in Lago"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:12-13 ───
[bug · low] EV 在 `cd` 到 worktree 根之后才用 `dirname "$0"` 计算：`$0` 是相对调用路径时（例如在轮次目录内自然地执行 `./seed.sh` 或
`bash seed.sh`），第一次 cd 后 cwd 已是仓库根，`dirname "$0"` 退化为 `.`，EV 解析为仓库根 ——
所有证据文件（seed-run.txt、seed-login-*.json、api-*.json、baseline）会静默散落到仓库根而非"beside this script"。应在 cd 之前（用
BASH_SOURCE）先解析出脚本目录。

- cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
- EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
+ EV="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"   # resolve evidence dir BEFORE any cd
+ cd "$EV/../../../.."                  # worktree root (docs/plans/<dir>/<round> -> root)


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_02_sync_face.mjs:35-39 ───
[test · medium] 刷新后的断言存在网络/DOM 竞态，负向证据可能是空转的：waitForResponse 只同步到 getOrder 的响应，而本页三态文案由 purchase
投影驱动，且 refreshOrder（CheckoutPage.tsx L265-273）是在 getOrder 的 .then 里才发起 purchaseStatus（GET
/api/v1/commercial/purchase）请求——即 waitForResponse resolve 时投影刷新连请求都尚未发出，紧随其后的 innerText()
读到的大概率是刷新前的 DOM。若后端真在同步回访路径错误确认了付款，这里旧投影（awaiting_payment）优先级高于 order.payment
兜底，页面仍显示「待付款（权益未开通）」，AC2 的负向断言会假通过。建议在断言前再同步等待 purchaseStatus 的响应（或基于响应体断言），保证读到的是刷新后的渲染。

    await Promise.all([
      page.waitForResponse((r) => r.url().includes(`/commercial/orders/${ORDER}`), { timeout: 45000 }),
      page.getByRole('button', { name: /刷新订单状态/ }).click(),
    ]);
+   // The three-state face rides the purchase projection, which refreshOrder
+   // fetches only AFTER the order read resolves — wait for that response
+   // too, otherwise the assertions below may read the pre-refresh DOM.
+   await page.waitForResponse((r) => r.url().includes('/commercial/purchase'), { timeout: 45000 });
    body = await page.locator('main').innerText();


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_04_active_face.mjs:27-28 ───
[test · medium] 这个 detached 等待不可能真正收敛：BillingPage 的 useEffect（BillingPage.tsx L71-91）每次加载只取一次
purchaseStatus，没有轮询，只有 Reload 按钮会重取；本腿脚本也没有任何 page.reload()。因此「已付款待激活」一旦渲染就永远不会自行
detach——在“结算已完成才开跑”的正常时序下该等待瞬时通过（等价于死代码，注释宣称的 deterministic convergence 并不存在）；而若本腿在投影仍为
paid_awaiting_activation 时启动，则只会空等 180s 后以 leg-error 失败。建议改为通过页面自身的 Reload
入口轮询直到文案翻转，或删去该等待直接等「已生效」。

-   await page.waitForSelector('text=已付款待激活', { state: 'detached', timeout: 180000 });
+   // BillingPage reads purchaseStatus once per load and never polls — drive
+   // the page's own Reload entry until the projection actually flips.
+   for (let i = 0; i < 36; i++) {
+     if ((await page.locator('main').innerText()).includes('已生效')) break;
+     await page.getByRole('button', { name: 'Reload' }).click();
+     await page.waitForTimeout(5000);
+   }
    await page.waitForSelector('text=已生效', { timeout: 45000 });


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:22-22 ───
[bug · medium] env_value 在键缺失时会让 set -euo pipefail 提前静默中止脚本，使下方精心编写的诊断分支不可达。机制：lab.env 缺某键时 grep
无匹配返回 1，pipefail 使管道以 1 退出；`_t9_base="$(env_value ...)"` 赋值语句的退出码即命令替换的退出码，set -e 直接触发中止——既不打印
"LAGO_API_URL missing/empty ... (run lab.sh init)"，也不打印 "LAGO_ORG_API_KEY
missing..."，操作者只看到一个无输出的失败（与脚本自身强调的 skip≠pass/可排查纪律相悖；source 场景下则表现为 export 半途而废）。请让 grep
的无匹配不进入退出码，把判定权交还给后面的 [ -n ... ] 检查。

- env_value() { grep -E "^$1=" "$ENV_FILE" | tail -n 1 | sed 's/^[^=]*=//; s/^"//; s/"$//'; }
+ env_value() { { grep -E "^$1=" "$ENV_FILE" || true; } | tail -n 1 | sed 's/^[^=]*=//; s/^"//; s/"$//'; }


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:79-81 ───
[maintainability · medium] 内嵌 Python 的 GraphQL/REST
响应直接链式索引：login["data"]["loginUser"]["token"]、["organization"]["lago_id"]、existing["data"]["paymentPr
oviders"]。GraphQL 失败（登录凭据失效、db 重建后 lab.env 过期、schema 漂移）时响应为 {"errors":[...]} 且无 "data" 键，将以裸
KeyError/TypeError 栈崩溃而非明确诊断。同文件下方 add 变异已经用 (add.get("data") or {}) 的防御风格（并通过 json.dumps(add)[:300]
打印失败体），登录/查询路径应保持同一模式，否则 harness 失败时难以区分是哪一环（login vs org 解析 vs 查询）。

  login = gql("mutation($e:String!,$p:String!){loginUser(input:{email:$e,password:$p}){token}}",
              {"e": email, "p": password})
+ if login.get("errors") or not ((login.get("data") or {}).get("loginUser") or {}).get("token"):
+     print("loginUser failed:", json.dumps(login)[:300]); sys.exit(1)
  token = login["data"]["loginUser"]["token"]


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:155-157 ───
[security · low] webhook secret 通过 `docker compose exec -e WSECRET=$SECRET` 的命令行参数传递：docker CLI
进程存活期间该值以明文出现在宿主机进程参数中（ps / /proc/PID/cmdline 对同用户可见），与脚本自身 "never written
anywhere"、凭据仅经环境流转的纪律不完全一致。t11 既有 harness（phases.py）没有 exec -e 传密先例，这里建议改为经 stdin 注入容器进程环境之外更窄的通道。

-   docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T \
-     -e WSECRET="$SECRET" api bin/rails runner \
-     "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: ENV['WSECRET'])" >/dev/null
+   printf '%s' "$SECRET" | docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T \
+     api bin/rails runner \
+     "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: STDIN.read)" >/dev/null


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:49-50 ───
[bug · low] HTTPError 分支假定错误响应体必为 JSON：网络中间层/网关返回非 JSON 错误页（或空体）时 json.loads 抛
JSONDecodeError，直接掩盖调用方精心准备的 `intent list HTTP {status}` 诊断，操作者看到的是解析栈而非真实状态码。读体后做一次容错解析，非 JSON
时保留状态码与原始片段。

          except urllib.error.HTTPError as e:
-             return e.code, json.loads(e.read().decode())
+             raw = e.read().decode(errors="replace")
+             try:
+                 return e.code, json.loads(raw)
+             except ValueError:
+                 return e.code, {"error": raw[:300]}


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:90-96 ───
[maintainability · low] org 兜底查询存在两个问题：(1) subprocess.run 未检查 returncode，docker 失败（容器未起、cwd
非仓库根导致相对路径 compose 文件不存在）时 stdout 为空，最终落入误导性的 "organization id unavailable" 而非真实原因；(2) 与
prepare_t9_env.sh 用 REPO_DIR=$(pwd) 拼绝对路径并显式非空校验的写法不一致，org 查询逻辑在两个脚本间重复维护。至少应检查 returncode 并把 stderr
带入诊断信息。

          out = subprocess.run(
              ["docker", "compose", "-f", "deploy/lago/compose.yaml",
               "--env-file", "deploy/lago-lab/payment-settle-trigger/lab.env",
               "-p", "weknora-lago-t11", "exec", "-T", "db", "psql", "-U", "lago",
               "-tAc", "select id from organizations order by created_at limit 1"],
              capture_output=True, text=True, timeout=30)
+         if out.returncode != 0:
+             raise SystemExit(f"org lookup failed (exit {out.returncode}): {(out.stderr or '').strip()[:200]}")
          args.org = (out.stdout or "").strip()


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:80-85 ───
[bug · low] intent 选取谓词是「最新一条携带任意 lago_invoice_id 的 succeeded PI」，未绑定目标发票。仓库自身证据已记录该风险面：重购复用同一
external customer（issue-72-ocr-issue-82-r2.md:1289 明确指出该启发式可能『选错』，lago_settlement.go
已按发票身份收敛），r4-flow/README 也实证同客户名下多笔 gating PI。当同客户存在更新、且属于另一张发票的 succeeded PI 而目标发票的 PI 尚未 settle
时，本替身会把 payment_intent.succeeded 投到错误发票上，产生假阳性的 settle 证据（虽有结尾 invoice 打印可事后察觉，但证据链已污染）。建议增加可选
--invoice 参数，把谓词收敛到 metadata.lago_invoice_id == 目标发票。

      intent = None
      for row in body.get("data", []):
-         if row.get("status") == "succeeded" and (row.get("metadata") or {}).get("lago_invoice_id"):
+         inv = (row.get("metadata") or {}).get("lago_invoice_id")
+         if row.get("status") != "succeeded" or not inv:
+             continue
+         if getattr(args, "invoice", None) and inv != args.invoice:
+             continue
-             cand = row
+         cand = row
-             if intent is None or cand.get("created", 0) > intent.get("created", 0):
+         if intent is None or cand.get("created", 0) > intent.get("created", 0):
-                 intent = cand
+             intent = cand


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:167-173 ───
[security · medium] (C-02) 声明"每个进入 SQL 的值先过白名单"，但本函数的 tenant 插值未被白名单覆盖：driveToActive(token, tenant,
log) 在 act2/act4a 中以登录响应的 u.tenant 直传（未经 tenantShape），随即拼入 docker psql
查询。被篡改/异常的登录响应（active_tenant.id）可注入 psql 命令，也违背"数据库查询禁止拼接 SQL"的安全约束与脚本自身不变量。对比：act4b 的 SQL 用了
tenantShape(u.tenant)、act3/act5 传的是已校验的 TEN。建议在本函数入口统一校验（覆盖全部调用点）。

- function lagoCustomer(tenant) {
+ function lagoCustomer(rawTenant) {
+   const tenant = tenantShape(rawTenant); // (C-02) 入口统一过白名单，覆盖 driveToActive 等全部调用点
    try {
      return execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
        `select ppc.provider_customer_id from payment_provider_customers ppc join customers c on c.id=ppc.customer_id where c.external_id='weknora-tenant-${tenant}'`],
        { encoding: 'utf8' }).trim();
    } catch { return ''; }
  }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:133-140 ───
[bug · medium] 失败路径丢失证据，违背 (C-04)"中止的 act 仍写出累积日志"的注释意图：process.exit(2) 直接终止进程，而 files
的唯一落盘循环在脚本末尾（行 371），因此 files['sql-shape-failure.txt'] 的赋值永远不落盘（死赋值），且此前各 act 已累积的
recovery-no-notify.txt 等日志一并丢失，只剩 console.error 的 stderr。同理，模块顶层的 LAGO_KEY = execFileSync('docker',
...) 与 readFileSync(seed-login-f.json) 位于 try/catch 之外，docker/文件不可用时脚本裸崩、无 RESULT 汇总。建议改为抛出异常交给已有顶层
catch 记 note 后走末尾统一写盘（sqlShape 的调用点均在主 try 内，直接 throw 即可），并给顶层 docker/文件读取加保护。

  function sqlShape(name, value, re) {
    if (!re.test(value)) {
+     // (C-04) 抛错而非 process.exit：顶层 catch 记 note，末尾写盘循环仍落盘
+     // files 中已累积的各 act 日志 + 本次失败原因。
      files['sql-shape-failure.txt'] = `refusing to interpolate ${name} into SQL: ${JSON.stringify(value)}\n`;
-     console.error(`bad ${name} for SQL interpolation: ${JSON.stringify(value)}`);
-     process.exit(2);
+     throw new Error(`bad ${name} for SQL interpolation: ${JSON.stringify(value)}`);
    }
    return value;
  }


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:111-113 ───
[maintainability · low] FLOW83_TOKEN 缺失的处理与 EMAIL/PASSWORD/TENANT 不一致：前三者缺失时快速 process.exit(2)（行
28/36 附近），而 TOKEN 缺失仅在此处静默降级为占位状态，后续 settle-webhook-active 断言将空转 24 次 × 10 秒轮询（约 4 分钟）后以 FAIL
告终，配置缺失被误读为结算失败。建议同样快速失败，保持四个必需环境变量一致的校验风格。

  const TOKEN = process.env.FLOW83_TOKEN ?? '';
+ if (!TOKEN) {
+   console.error('missing required env: FLOW83_TOKEN (purchase-state polling needs it)');
+   process.exit(2);
+ }
  async function purchaseState() {
-   if (!TOKEN) return { data: { state: 'env-missing-token' } };


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:194-198 ───
[maintainability · low] 与 api_recovery_83.mjs 存在大段可提取的重复逻辑：stubOrders/stubMark/stubNotify 封装、此处的
NOTPAY+9900 待付订单过滤（与 newestPendingOrder 完全同构）、lagoCustomer 查询、deliver_stripe_webhook.py 投递均各写一份；且
9900、容器名 weknora-lago-82flow-db-1、org UUID 305eddac-...、端口 48889/8296/8095
等环境标识在两脚本间多点重复，实验环境或定价调整需双处同步，漂移会直接造成误判 PASS/FAIL。82 轮已建立 _browser_lib.mjs
共享库先例，建议沿用该模式提取公共模块并把业务金额（9900）与余额（10900000）收敛为模块级常量。

-   const orders = await stubOrders();
-   const pendingKeys = Object.entries(orders)
-     .filter(([, v]) => v?.state === 'NOTPAY' && v?.total === 9900)
-     .map(([k]) => k);
-   const liveOrder = pendingKeys[pendingKeys.length - 1] ?? '';
+   // 提取共享模块（沿用 ../issue-72-flow-evidence-82/_browser_lib.mjs 的先例）：
+   // stubOrders/stubMark/stubNotify、newestPendingOrder(NOTPAY+AMOUNT_FEN 过滤)、
+   // lagoCustomer、deliverWebhook —— AMOUNT_FEN=9900 等常量一并收敛，两脚本共用。
+   const liveOrder = await newestPendingOrder();


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:222-224 ───
[maintainability · low] act3 中的 tokenF 声明后从未使用（act3 的通知重放走 stub、SQL 查询用 TEN，均不需要它），是死变量；act5
中的同名变量才有实际引用。建议删除，避免误导读者以为 act3 也走 API 鉴权路径。

      const act = 'act3-duplicate';
      const log = [];
-     const tokenF = TOKEN_MAIN;


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:352-353 ───
[test · low] walletsAny.includes('9.9') 以子串匹配断言到账金额：psql 输出为多行 "name|status|amount"，19.9、29.9 或任何一笔含
"9.9" 的交易都会误判通过，且依赖 psql 的浮点输出格式。建议按行解析并做精确数值比较。

      log.push(`purchase wallets: ${walletsAny.replace(/\n/g, ' ; ')}`);
-     note(act, 'purchase-wallet-granted', walletsAny.includes('9.9'), 'the purchase wallet granted the 9.90 first period (status 1 = settled)');
+     const granted = walletsAny.split('\n').filter(Boolean).some((row) => {
+       const amount = Number(row.split('|')[2]);
+       return Number.isFinite(amount) && Math.abs(amount - 9.9) < 0.001;
+     });
+     note(act, 'purchase-wallet-granted', granted, 'the purchase wallet granted the 9.90 first period (status 1 = settled)');


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_03_restart.sh:10-11 ───
[bug · medium] 前置 env 校验不完整：头注释声称 "Caller env identical to start_backend_8093.sh"，但该 launcher 实际必需三个
env——除了这里校验的两个凭据，还有 FLOW82_KEY_DIR（start_backend_8093.sh 内部 `KEYS="${FLOW82_KEY_DIR:?...}"`
会直接退出）。FLOW82_KEY_DIR 未导出时，重启的 launcher 在 nohup 下立即失败，错误只会写进 /tmp/issue82-r5b-backend-restart.log
而不出现在 tee 生成的证据文件中，随后 40×5s 健康轮询空转约 200 秒才超时，失败原因完全不可见于回放证据。建议与两个凭据并列补一条校验，让缺失在 kill 后端之前就清晰失败。

  : "${WEKNORA_COMMERCIAL_PLATFORM_API_KEY:?caller must export the Lago org key}"
  : "${WEKNORA_COMMERCIAL_STRIPE_API_KEY:?caller must source ~/.zcode/issue72-stripe.env}"
+ : "${FLOW82_KEY_DIR:?caller must export FLOW82_KEY_DIR (the local RSA key pair, required by start_backend_8093.sh)}"


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:82-83 ───
[security · medium] 两条 commercial_grants 语句都以字符串插值拼入 '$UID_B'。项目安全红线明确要求"数据库查询一律使用参数绑定,禁止拼接 SQL",且
OCR r2 轮(docs/plans/issue-72-ocr-issue-82-r2.md L746-747)已把同款 seed 语句的修复模式定为 sqlite3 `?1`
占位符绑定,issue-72-plan-82.md L31 也把该约束列为清偿面。本文件是新写入的脚本,却回退到插值模式(前置 uuid_shape 守卫只是缓解,不满足红线的字面要求);建议对
insert 与 select count 两条语句统一改用 `?1` 绑定传参。

- sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-83', 1);"
- say "granted: $(sqlite3 "$DB_PATH" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"
+ sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, ?1, 'plan_publish', 'flow-verifier-83', 1);" "$UID_B"
+ say "granted: $(sqlite3 "$DB_PATH" "select count(*) from commercial_grants where capability='plan_publish' and user_id=?1;" "$UID_B") row(s)"


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:86-90 ───
[bug · low] C-12 的 TRANSPORT 守卫没有覆盖全部出站点:plan draft、plan publish、Lago plans 拉取这三条 curl(以及 `jq -c ...
| say "... $(cat)"` 管道)仍是裸调用。后端/Lago 未起时 curl 以 exit 7 失败,在 set -euo pipefail
下脚本零输出直接死亡——这正是头注释声称已消灭的静默路径,与自身声明的 fail-loudly 纪律不一致。建议为这三处补上与 LOGIN/LAGO_KEY 相同的 `|| { say "FAIL:
... TRANSPORT failure (curl exit $?)"; exit 1; }` 守卫。



─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:26-28 ───
[security · low] $PW 以字符串插值直接嵌入 JSON 体:密码含 `"` 或 `\\` 时生成非法 JSON,注册/登录会以误导性的 "HTTP 400/401" FAIL
中断(而非指出 JSON 破坏)。同时 $PW 与 Bearer $TOKEN_B/$LAGO_KEY 都经 argv 暴露给同机 ps。本地单机 lab
降低了实际风险,但与该目录"凭据零字面量/不落易泄露面"的严格纪律有出入。建议用 `jq -n --arg pw "$PW"
'{username:$e,email:$e,password:$pw}'` 构造请求体,并用 `--data-binary @file` / `--header @file` 让凭据不进 argv。



─── docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py:285-289 ───
[other · medium] 模块级 ORDERS 字典在 ThreadingHTTPServer(每请求一线程)下被无锁并发读写:close 分支是典型的 check-then-act(读
state==SUCCESS → 400 / 否则写 CLOSED),与 /stub/mark 的读改写、native 的插入之间没有同步。交错序列"close 读到 NOTPAY → mark 写
SUCCESS 并推回调 → close 写 CLOSED"会把状态表终态改成 CLOSED,而渠道已推 SUCCESS——stub
自身引入的非确定性恰好会扭曲它要取证的关单竞态结果,产出不可信证据或随机失败。82 轮兄弟 stub(alipay_gateway_stub.py)用单线程 HTTPServer
且无状态表,不存在此问题。建议:给所有 ORDERS 访问加 threading.Lock,或直接改用单线程 HTTPServer 与既有惯例对齐。

-             if order["state"] == "SUCCESS":
-                 log("CLOSE %s -> 400 ORDER_PAID (race shape)" % order_id)
-                 self._reply(400, {"code": "ORDER_PAID", "message": "订单已支付，禁止关单"})
-                 return
-             order["state"] = "CLOSED"
+ # 建议(方案一):模块级锁保护 ORDERS 的复合操作
+ # LOCK = threading.Lock()
+ # with LOCK:
+ #     order = ORDERS.get(order_id)
+ #     if order is None: ...
+ #     if order["state"] == "SUCCESS": ... (reply 400)
+ #     order["state"] = "CLOSED"
+ # 或(方案二):__main__ 改用单线程 HTTPServer((HOST, PORT), Handler)


─── docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh:32-35 ───
[test · low] 两处 smoke 探测只回显 HTTP 状态码、不做任何断言:注释写明 expect 401,但 stub 起错(如缺 cryptography 依赖、行为异常返回
200/500)时脚本照常放行,复验轮会在更晚、更难定位的步骤才失败。建议捕获状态码并断言(wechat 未签名 native create 必须为 401;alipay 未签名 precreate
为 200 + isv.invalid-signature 信封或至少非常见 5xx),失败即 say FAIL + exit 1。另:pkill 后固定 sleep 1
在旧进程尚未释放端口时可加一次重试探测。

  echo '--- smoke: wechat stub rejects unsigned native create (expect 401) ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8296/v3/pay/transactions/native -d '{}'
- echo '--- smoke: alipay stub answers precreate-unauth ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8297/gateway.do -d 'service=alipay.trade.precreate'
+ SMOKE=$(curl -s -o /dev/null -w '%{http_code}' -X POST 127.0.0.1:8296/v3/pay/transactions/native -d '{}') || { echo "FAIL: wechat stub smoke TRANSPORT failure (exit $?)"; exit 1; }
+ [ "$SMOKE" = "401" ] || { echo "FAIL: wechat stub smoke expected 401, got $SMOKE"; exit 1; }
+ echo "wechat stub smoke 401 OK"


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:32-32 ───
[bug · medium] 与已确认的 env_value 问题同属一类但发生在不同代码点：`_t9_org` 是普通赋值语句，命令替换内管道在 pipefail 下以 docker/psql
的非零码退出时，set -e 会在赋值处直接中止脚本（本脚本被 source 时还会连带退出调用方 shell），下方 `[ -n "$_t9_org" ] || echo "organization
id unavailable (db container not ready?)"` 恰好在它自己预判的『db container not
ready』场景下不可达——诊断信息丢失，与脚本自身强调的可排查纪律相悖。建议对该管道做显式容错，让诊断分支真正可达。

- _t9_org="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T db psql -U lago -tAc 'select id from organizations order by created_at limit 1' | tr -d '[:space:]')"
+ _t9_org="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T db psql -U lago -tAc 'select id from organizations order by created_at limit 1' 2>&1 | tr -d '[:space:]' || true)"


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:112-114 ───
[bug · medium] 读取 stored webhook secret 的 docker exec 带 `2>/dev/null` 且无退出码保护：api 容器未就绪或 rails
runner 启动失败时，该命令以非零退出，set -e 在此处静默中止脚本——错误输出已被丢弃，`STORED` 提取、stored-vs-mint 分支、以及末尾的 "t9 env ready"
都不会执行，操作者只看到一个无任何输出的终止（被 source 时还会杀掉调用方 shell）。建议不要吞掉 stderr，并对失败显式降级或终止并给出原因。

- docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T api bin/rails runner \
+ if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T api bin/rails runner \
    "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__T9SECRET__' + s" \
-   > "$STORED_FILE" 2>/dev/null
+   > "$STORED_FILE"; then
+   echo "stored-secret lookup failed (api container ready?)" >&2
+   return 1 2>/dev/null || exit 1
+ fi


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:121-121 ───
[bug · low] `SECRET=$(python3 - <<'PY' ...)` 的失败路径会先于检查触发 set -e：mint python 的任何失败（网络异常、Stripe 非
200、响应缺 "secret" 键）都以非零码退出，普通赋值语句随即中止脚本，下方 `if [ -z "$SECRET" ]; then echo "secret mint failed";
return 1 ...` 实际不可达——该分支只在 python 以 0 退出且 stdout 为空时才成立，而正常路径必然打印非空 secret。请对赋值做显式 `|| true`/rc 捕获，让
"secret mint failed" 诊断真正可达（与文件其余 `return 1 2>/dev/null || exit 1` 的优雅失败风格一致）。

    SECRET=$(python3 - <<'PY'
+ ...
+ PY
+   ) || { echo "secret mint failed" >&2; return 1 2>/dev/null || exit 1; }


─── internal/handler/commercial.go:833-835 ───
[maintainability · low] 可观测性缺口：同分支的 rerr 降级路径（r2:823）刻意保留了"one diagnosable line"，但这里
qerr/eerr（QuoteSnapshotForTenant 失败——存储故障或快照 JSON 损坏）以及 plan 不匹配导致的裸 409
降级完全没有服务端日志。快照读取失败属于数据完整性事件，静默吞掉后裸 409 无任何痕迹，与紧邻 else 分支的日志约定不一致。建议在 qerr/eerr 非空时补一条日志（plan
不匹配是业务事实可不记）。

  			_, reqSnap, qerr := h.orders.QuoteSnapshotForTenant(c.Request.Context(), tenantID, req.QuoteID)
  			_, exSnap, eerr := h.orders.QuoteSnapshotForTenant(c.Request.Context(), tenantID, existing.QuoteID)
+ 			if qerr != nil || eerr != nil {
+ 				log.Printf("commercial: pending-purchase conflict snapshot read failed for tenant %d: req=%v existing=%v", tenantID, qerr, eerr)
+ 			}
  			if qerr == nil && eerr == nil && reqSnap.PlanKey == exSnap.PlanKey {


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:203-206 ───
[test · medium] act2 在未确认自身 purchase 落地的情况下无条件 stubMark：脚本自己注释了 Lago 忙窗（503）常态存在，且 act4a/4b 都为此使用
purchaseUntilLanded，唯独 act2 直接单次 purchase。若该次失败（HTTP 非 201，orderID=''），newestPendingOrder() 会按“最后一个
NOTPAY/9900”挑到残留的外来订单（如上一轮残留或浏览器 act1 主链尚未支付的订单）并将其标记 SUCCESS —— 既污染其他 act 的状态（act3 按 total=9900
匹配订单），后续 GET /orders/（空 id）也会级联 FAIL。建议在标记前校验 p1 已落地（失败则 throw，顶层 catch 会保留已完成证据），或直接改用
purchaseUntilLanded。

+     if (p1.status !== 201 || !orderID) {
+       throw new Error(`act2-recovery: wechat purchase never landed (HTTP ${p1.status}) — refusing to mark a possibly-foreign stub order SUCCESS`);
+     }
      const mo = merchantOrderShape(await newestPendingOrder());
      log.push(`stub NATIVE out_trade_no=${mo} (NOTPAY/9900-filtered)`);
      // mark SUCCESS but NEVER push the notify (the missed-notification case)
      await stubMark(mo, `wx_txn_83_rec_${u.tenant}`);


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:361-362 ───
[test · low] 与已确认的 walletsAny.includes('9.9') 同一弱点的另一处实例：credits.includes('10900000') 对
JSON.stringify 结果做子串匹配，例如 balance_micro 为 "110900000" 时同样包含 "10900000" 会误判 PASS；紧随其后的
features.includes('\"advanced_models\":true') 还依赖 stringify 的键序。建议直接对解析后的字段做严格相等比较。

-     note(act, 'product-credits-feature', credits.includes('10900000'), `credits face carries the 1.0 base + 9.9 purchase balance: ${credits.slice(0, 80)}`);
-     note(act, 'advanced-models', features.includes('"advanced_models":true'), `features face: ${features}`);
+     const creditsBalance = String(acct.json?.data?.benefits?.credits?.balance_micro ?? '');
+     note(act, 'product-credits-feature', creditsBalance === '10900000', `credits face carries the 1.0 base + 9.9 purchase balance: ${credits.slice(0, 80)}`);
+     note(act, 'advanced-models', acct.json?.data?.benefits?.features?.advanced_models === true, `features face: ${features}`);


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:214-217 ───
[test · low] C-04 头注释声称“中止的 act 仍写出其累积日志”，但每个 act 的 files['xxx.txt'] = log.join('\n') 只在 act
末行执行：act 中途 throw（registerAndLogin 5xx、sqlShape 之外的任何 fetch 异常等）时，顶层 catch 只记录 err.message，该 act 的
log 数组整体丢失，落盘证据缺该 act 一段。建议把日志落盘移入每个 act 的 finally，或抽一个 runAct(name, file, fn) 帮助函数统一包裹。

      const active = await driveToActive(u.token, u.tenant, log);
      note(act, 'recovered-active', active, 'purchase reached active after the missed-notify recovery');
      log.push(`active=${active}`);
+     // C-04: 把该赋值移入本 act 的 finally（或用 runAct(name, file, fn) 包裹），
+     // 保证 act 中途 abort 时 log 仍然落盘。
      files['recovery-no-notify.txt'] = log.join('\n');


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:124-127 ───
[test · low] chromium.launch() 位于 try/catch/finally 之外：Playwright 未安装或浏览器启动失败时脚本在进入守卫前裸崩，C-05 注释承诺的
purchase-state-progression.txt 与 RESULT 汇总都不会产生。建议把 launch 移入 try，finally 中改用 browser?.close()
防御空引用。

- const browser = await chromium.launch();
+ let browser;
  let page;
  try {
+   browser = await chromium.launch();
    page = await (await browser.newContext()).newPage();
+   // ... finally 中: await browser?.close();


─── internal/modules/commercial/commercialplatform/lago_purchase.go:627-631 ───
[performance · medium] readPurchaseInvoiceFees 的 N+1 明细读被嵌在 readPurchaseSnapshot 固定的
subscriptionRequestTimeout（15s，lago_purchase.go:538）预算内，且对已 active 的购买在每次快照读时无条件全量执行：(1)
无提前退出——即使已找到 bestSucceeded 也必须为 created_at 排名读完每一张发票；(2) 数量无界增长——Base 计划每月产生 0 金额 finalized 发票（F-5
注释自述）+ 购买续期发票，租户历史越长串行 GET 越多；(3) 任一明细 GET 的 429/5xx 使整个快照以 unreachable 失败。当串行读持续超出 15s
时产生活性悬崖：fulfiller 的 overBudget 探针（purchase_fulfillment.go:154）读快照失败→transient→pending，或探针成功显示
active→放行→步骤④ D6' 重校验再次超时→transient→pending——两种路径 attention 都永不落地，已付款订单永久
pending、不发放也不告警。建议：为扫描量设独立预算并在超出时以明确分类失败（而非借道 15s 超时）；或利用明细 created_at 对明显早于当前 bestSucceeded
的索引页做保守剪枝；或将仅需要 State 的调用方（observeActivation 轮询、settle 步骤(i) 的 active 短路）与终局行项读取拆分，避免每次快照都付出 O(发票总数)
的代价。



─── internal/modules/commercial/commercialplatform/lago.go:1115-1121 ───
[bug · low] readCustomerFeatures 漏掉了本次变更在文件内统一补上的 429→unreachable 分类：这里 429 落入 default 分支被归类为
ErrPlatformInvalidResponse（definitive）。同一次提交已给
readSubscriptionByIdentity、ensureProviderBinding、waitForPaymentMethodSync、customerProviderBound、发票读等
所有兄弟路径加上 status == http.StatusTooManyRequests || status >= 500，唯独这个新写的函数不一致——被限流的 entitlements 读会使
benefits 面以 invalid_response（不可重试姿态）而非 unreachable（可重试）呈现。

  		switch {
  		case status >= 200 && status < 300:
- 		case status >= 500:
+ 		case status == http.StatusTooManyRequests || status >= 500:
  			return nil, fmt.Errorf("%w: entitlement read unavailable", commercial.ErrPlatformUnreachable)
  		default:
  			return nil, fmt.Errorf("%w: entitlement read rejected", commercial.ErrPlatformInvalidResponse)
  		}


─── internal/modules/commercial/commercialplatform/lago_settlement.go:103-111 ───
[performance · low] settle 链对同一客户做两次串行 GET：customerProviderBound 已经 GET /api/v1/customers/{id} 并解析出
provider_customer_id（lago_purchase.go:495-521），却丢弃该值；紧接着 boundProviderCustomerID 对同一 URL 再读一次。每次
settle 驱动（含 outbox 重放）多付一个 RTT，且两读之间绑定状态可能变化造成判定与取值不一致。建议让 customerProviderBound 直接返回解析到的
provider_customer_id，一次读同时完成 bound 判定与取值。



─── internal/modules/commercial/commercialplatform/lago_settlement.go:130-134 ───
[bug · low] 该短路未将 settledInvoices 绑定到本次购买的门控发票身份：同一 provider customer 下任何携带 lago_invoice_id 的历史
succeeded intent（上一购买周期、同一 purchase 身份复用）都能满足 len(settledInvoices)>0。当本次门控 intent
恰好不在候选集（canceled/processing 等未纳入的状态）时，settle 每趟都回答成功回执，购买却停在 awaiting_payment；已核实 D7 总预算最终会以
attention 落地告警（非静默），但告警从"立即数据异常"退化为"10 分钟预算耗尽"。建议收紧：例如要求最新的 succeeded intent 金额落在 (0,
payload.AmountFen] 内，或以其 created 时间不早于本次门控发票可观测下界作为附加条件。



─── internal/modules/commercial/commercialplatform/fake.go:288-292 ───
[test · low] SetPurchaseInvoiceFees 无条件把 InvoicePaymentStatus 注入为 "succeeded"，且 fake
没有任何其他途径设置该字段——测试面永远无法表达"已 finalized 但 payment_status 非 succeeded"的形状。而真实适配器 readPurchaseInvoiceFees
的 bestAny 回退恰恰会返回 pending/failed 状态的行项，fulfiller 的 D6' 拒绝分支（purchase_fulfillment.go:237 的
InvoicePaymentStatus != "succeeded" → refused）依赖这个形状才能被驱动。建议补充一个可独立设置 InvoicePaymentStatus
的测试缝，使该收入保护分支可在 fake 下测试。

  		if s.InvoicePaymentStatus == "" {
  			// A fee injection models the finalized stage; the D6' review
  			// reads the invoice payment_status alongside the lines.
  			s.InvoicePaymentStatus = "succeeded"
+ 		}
+ 
+ // SetPurchaseInvoicePaymentStatus overrides the finalized-stage
+ // payment_status independently (the D6' non-succeeded refused shape).
+ func (f *FakeAdapter) SetPurchaseInvoicePaymentStatus(extPurchaseSubscriptionID, status string) {
+ 	f.mu.Lock()
+ 	defer f.mu.Unlock()
+ 	if s, ok := f.purchaseSubs[extPurchaseSubscriptionID]; ok {
+ 		s.InvoicePaymentStatus = status
+ 		f.purchaseSubs[extPurchaseSubscriptionID] = s
+ 	}
- 		}
+ }


─── internal/handler/commercial.go:523-524 ───
[maintainability · low] 日志设施不一致：本次新增的三处日志（本行、550 行 PurchaseStatus、844 行 CreateOrder 冲突降级）引入了标准库
log.Printf，而项目的统一日志设施是 internal/logger（带 context 关联，middleware 层已普遍使用 logger.Warnf(ctx, ...)，handler
包此前无任何日志调用）。标准 log 输出无请求关联、无级别元数据，且这里 500 响应是关闭式消息（"purchase failed"），客户端报障时无法把响应与日志行关联起来。建议改用
logger.Errorf(c.Request.Context(), ...)（三处一并统一）。

- 		log.Printf("commercial: purchase failed for tenant %d: %v", tenantID, err)
+ 		logger.Errorf(c.Request.Context(), "commercial: purchase failed for tenant %d: %v", tenantID, err)
  		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})


─── internal/handler/commercial.go:820-822 ───
[maintainability · low] 跨端点语义不一致（遗漏同步）：Purchase 处理器本次为 ErrQuoteNotFound 新增了 404 映射（OCR r4 /
R82-2：quote id 不存在/过期是客户端事实，应答 404 而非残余错误脸），但同一 PR 中同样被修改了 switch 的姊妹端点 CreateOrder（POST
/commercial/orders，legacy 下单路径同样经 quoteForTenant 读 quote）没有同步该分支 —— 不存在的 quote_id 在此仍落入 default 400
并透出原始 err.Error()。两个端点对同一客户端事实返回不同的状态码与错误脸，前端/SDK 需要分别适配。建议在 CreateOrder 的 switch 中补充同款
ErrQuoteNotFound → 404 分支，与 Purchase 对齐。

+ 	case errors.Is(err, repocommercial.ErrQuoteNotFound):
+ 		c.JSON(http.StatusNotFound, gin.H{"error": "quote not found"})
  	case errors.Is(err, repocommercial.ErrPurchasePendingExists):
- 		// (R3-25) The partial pending-purchase invariant rejected this
- 		// insert (the legacy POST /orders path has no PurchaseService-style


─── internal/modules/commercial/service/commercial/fulfillment.go:343-353 ───
[bug · high] 订阅购买的路由判定把“暂时无法分类”当成“确定非订阅购买”提交了。当 quote 读失败是非 NotFound 的瞬时 DB 错误（SQLite 锁、PG 抖动），或
SnapshotJSON 解析失败时，这里只留一条 Warn 就返回 false——一张已 paid 的订阅购买订单随即进入充值结算路径：TopUpOrderLines 按记账汇率铸造 credits
line → processRecord 经网关落 applied 记录 → MarkFulfilled。而订单一旦 fulfilled，fulfillEvent 和
PurchaseFulfiller.Fulfill 的 fulfilled 早退分支都会永久跳过 settle/observe/grant 链：款项永远不会在权威侧结算、订阅不会激活、钱包拿到的是按
1 Credit/CNY 折算的任意额度。一次瞬时读错误即可触发不可逆的错误结算，且无测试覆盖该形态（测试仅覆盖 NotFound）。这与同 PR 中 purchase_fulfillment.go
自己的分类纪律（r2:119：NotFound → attention、瞬时 DB 失败 → return nil 保持 pending）直接矛盾。建议把“无法分类”与“确定不是订阅购买”区分开：非
NotFound 的读错误与快照解析失败应让 fulfillEvent 将事件保持 pending（completeEvent(OutboxStatePending)），绝不落入 top-up
结算路径；只有 ErrRecordNotFound 才保留 frozen legacy 语义返回 false。

+ func (s *FulfillmentService) isSubscriptionPurchase(ctx context.Context, row repocommercial.OrderRow) (isSub, classified bool) {
+ 	var q repocommercial.QuoteRow
  	if err := s.db.WithContext(ctx).Where("id = ?", row.QuoteID).First(&q).Error; err != nil {
- 		// (r2:341) A paid order misrouting into the top-up settlement (book
- 		// rate credits instead of the settle/activation chain) is the most
- 		// expensive silent failure this dispatcher has: a NotFound is the
- 		// frozen legacy-seed semantics (silently false), but any OTHER read
- 		// failure leaves one Warn so the misroute stays diagnosable.
- 		if !errors.Is(err, gorm.ErrRecordNotFound) {
+ 		if errors.Is(err, gorm.ErrRecordNotFound) {
+ 			return false, true // frozen legacy-seed semantics
+ 		}
+ 		// transient DB failure: classification is INDETERMINATE — the caller
+ 		// must keep the event pending, never commit to the top-up settlement.
- 			logger.Warnf(ctx, "[CommercialFulfillment] subscription-purchase quote read failed for order %s: %v", row.ID, err)
+ 		logger.Warnf(ctx, "[CommercialFulfillment] subscription-purchase quote read failed for order %s: %v", row.ID, err)
+ 		return false, false
- 		}
+ 	}
- 		return false
+ 	var snap struct {
+ 		LineItems []struct {
+ 			Kind string `json:"kind"`
+ 		} `json:"line_items"`
+ 	}
+ 	if err := json.Unmarshal([]byte(q.SnapshotJSON), &snap); err != nil {
+ 		logger.Warnf(ctx, "[CommercialFulfillment] subscription-purchase quote snapshot unparsable for order %s: %v", row.ID, err)
+ 		return false, false
+ 	}
+ 	for _, line := range snap.LineItems {
+ 		if line.Kind == "subscription_fee" {
+ 			return true, true
+ 		}
+ 	}
+ 	return false, true
- 	}
+ }
+ 
+ // fulfillEvent 调用侧：
+ // 	if row.Kind == domain.OrderKindPurchase {
+ // 		isSub, classified := s.isSubscriptionPurchase(ctx, row)
+ // 		if !classified {
+ // 			return s.completeEvent(ctx, ev, repocommercial.OutboxStatePending)
+ // 		}
+ // 		if isSub { /* 原 PurchaseFulfiller 路由 */ }
+ // 	}


─── internal/modules/commercial/commercialplatform/lago_settlement.go:283-283 ───
[bug · high] limit=100 的 PaymentIntent 列表页几乎必然超过 maxProviderBodyBytes(64 KiB)上限。真实 Stripe 的 GET
/v1/payment_intents 返回每行的完整 PaymentIntent 对象(含
client_secret、payment_method_options、last_payment_error 等大量字段),单个对象通常约 0.8–1.5 KB;100 行合计约 80–150
KB,稳定超过 providerOutboundRequest 在 lago_settlement.go 中新增的 64 KiB body cap(超限返回
ErrPlatformInvalidResponse「provider response body exceeds 65536 bytes」,definitive)。后果:settle 命令对
intent 历史较多的客户(每次 checkout 尝试/3DS 失败都会累积一个 intent)会确定性地失败并 park 到 attention,而不是按 unreachable 重试——本地
stub 返回精简对象所以 lab 证据无法暴露此问题。建议把每页 limit 降到 10–20 并相应提高
stripeListPageBudget(保持总覆盖量),或对该列表路径使用显著更大的响应上限。

- 		path := "/v1/payment_intents?customer=" + url.QueryEscape(providerCustomerID) + "&limit=100"
+ 	// A full PaymentIntent object serializes to ~1KB on the real provider;
+ 	// a 100-row page overflows maxProviderBodyBytes (64 KiB) and the read
+ 	// fails closed as invalid_response. Keep the page small and compensate
+ 	// with the page budget so a page never trips the cap.
+ 	const listPerPage = 10
+ 	path := "/v1/payment_intents?customer=" + url.QueryEscape(providerCustomerID) + "&limit=" + strconv.Itoa(listPerPage)


─── internal/modules/commercial/commercialplatform/fake.go:761-765 ───
[test · medium] fake 的 create_purchase_subscription identity-replay 分支没有同步本次在 Lago 适配器上新加的 terminal
拒绝语义(lago_purchase.go:新增「canceled/terminated → purchase subscription terminal →
ErrPlatformInvalidResponse」)。fake 中被 CancelPurchase 置为 canceled 的购买,同 plan 重放 create 仍返回成功回执,而真实
Lago 返回 definitive invalid_response——两者的客户端可见行为分裂:经 fake 的服务层测试路径会走到 snapshot 读后以
ErrPurchaseNotAwaiting(409)收尾,真实栈则在 submit 处以 503 invalid_response 收尾。既然本次同时新增了 CancelPurchase 观察旋钮和
settle 分支的 terminal 拒绝,建议在 create 的 held-replay 分支同样拒绝 terminal 状态,保持 fake/real
契约一致,否则该新分支在服务层测试面不可驱动。

  		if sub.Status == "canceled" || sub.Status == "terminated" {
  			// A terminal purchase can never be settled (the authority's own
  			// truth refuses; the Lago rails would fail the same way).
  			return commercial.CommandReceipt{}, fmt.Errorf("%w: settle target purchase is terminal", commercial.ErrPlatformInvalidResponse)
  		}
+ 		// NOTE: the create_purchase_subscription replay branch above should
+ 		// mirror the Lago adapter's new terminal refusal (canceled/terminated
+ 		// held purchase → ErrPlatformInvalidResponse) so fake/real stay
+ 		// contract-identical for repurchase-after-cancel.


─── internal/modules/commercial/commercialplatform/config.go:159-163 ───
[documentation · low] 这条注释声称「the provider egress keeps its full runtime host policy either
way」,但与实现不符:providerOutboundRequest(lago_settlement.go)对 StripeAPIBase 同样调用
validateOutboundHostWithBypass(base, a.cfg.OutboundAllowLoopback)——bypass 开启时 provider 出站也放行
loopback,且本 guard 不对 StripeAPIBase 做 posture 校验。当前生产可达性受限于 configured() 要求 BaseURL 非空 + guard 要求
BaseURL 为 loopback,所以实际暴露面有限,但这是安全边界上的错误保障声明,会误导后续维护/审计(例如据此推断 provider 侧无需 posture
约束)。建议修正注释以描述真实行为(bypass 同时作用于 authority 与 provider egress,posture 仅由 authority BaseURL 保证)。

  // guardLoopbackBypassPosture enforces the dev-only posture of the loopback
  // bypass (A-27): with the bypass on, an explicitly configured authority
  // BaseURL must be a loopback/localhost host. An empty BaseURL stays legal
- // (the blocked-env posture — no authority egress exists to bypass; the
- // provider egress keeps its full runtime host policy either way).
+ // (the blocked-env posture — no authority egress exists to bypass). NOTE:
+ // the bypass ALSO relaxes the provider egress (StripeAPIBase) loopback
+ // refusal — see providerOutboundRequest; its posture is only indirectly
+ // guaranteed by this authority-BaseURL gate.


─── internal/modules/commercial/service/commercial/purchase.go:368-370 ───
[bug · low] ErrPurchasePendingExists 分支里，CurrentPayablePendingOrder 的非 NotFound 读失败（瞬时 DB
故障）被静默吞掉：`return PurchaseView{}, err` 只回传原始冲突哨兵，`gerr` 既未记录日志也未包装——而 10 行之下的清扫失败（r2:306 纪律）专门为此写了
Warnf。同文件自身声明的纪律是"每次吞掉的腿失败都要留一条可诊断日志"，这里基础设施故障被伪装成可重试 409 且无任何日志痕迹，事后无法从日志定位为何租户反复 409。建议至少补一条
Warnf（或用 %w 把 gerr 挂到返回错误上）。

  		if !errors.Is(gerr, repocommercial.ErrOrderNotFound) {
+ 			logger.Warnf(ctx, "[CommercialPurchase] conflict entry read failed for tenant %d: %v", tenantID, gerr)
  			return PurchaseView{}, err
  		}


─── internal/modules/commercial/service/commercial/order.go:617-622 ───
[bug · low] Close 与 Query 双腿都失败时（此处 return OrderView{}, qerr），原始的渠道 Close 失败原因 `cerr`（ORDER_PAID
语义、超时、具体渠道错误码）被完全丢弃，调用方（Purchase 换轨分支 → HTTP 层）只看到 Query 的错误。注释声称"surface
it"，但只上抛了第二条腿；排查换轨失败时最关键的失败原因（为什么 Close 失败）丢失。建议把两条腿都保留在一个错误里（例如 fmt.Errorf 包装两者），保持 qerr 作为主因以供
errors.Is。

  	res, qerr := provider.Query(ctx, att.MerchantOrderID)
  	if qerr != nil {
- 		// Both legs failed: the outcome stays unknown — surface it, mark
- 		// nothing, keep the payable entry for a retry.
- 		return OrderView{}, qerr
+ 		// Both legs failed: the outcome stays unknown — surface BOTH causes
+ 		// (the close failure is the primary one), mark nothing, keep the
+ 		// payable entry for a retry.
+ 		return OrderView{}, fmt.Errorf("close failed: %w; deciding query failed: %v", cerr, qerr)
  	}


─── internal/modules/commercial/service/commercial/purchase.go:222-227 ───
[bug · low] 同 quote 的 POST 重放会原样回放一张 channel-failed 的死订单：渠道 Create 失败后订单已落库且 quote 已消费，客户端若重试同一
quote，GetOrderByQuote 在所有可付性探测之前命中该死单，purchaseView 回答 201 形状的"成功"——orderViewFromRow 映射不出
checkout_url（空）也映射不出任何失败标记（checkout_error 不落库、channel_failed 未投影），客户端拿不到支付入口也拿不到重报价信号，只能永远停在一张
pending 死单上。R2-28 的目标是"channel-failed 单不是可付条目、不阻塞 checkout"，但这条重放路径没有消费该标记。建议：重放命中
channel_failed=true 的行时不要答干净的成功视图——要么携带失败/降级标记（复用 CheckoutLinkDegraded 类语义），要么直接答
ErrQuoteAlreadyUsed/重报价信号让客户端换新 quote（新 quote 路径已被 R2-28 解锁，可以正常开新渠道单）。

  	if existing, err := s.orders.orders.GetOrderByQuote(ctx, tenantID, quoteID); err == nil {
+ 		// (R2-28) a channel-failed row is a DEAD payment entry: replaying it
+ 		// as a clean success leaves the client on a link-less pending order
+ 		// for an already-consumed quote — answer the re-quote signal instead.
+ 		if existing.State == domain.OrderStatePending && existing.ChannelFailed {
+ 			return PurchaseView{}, repocommercial.ErrQuoteAlreadyUsed
+ 		}
  		ov := orderViewFromRow(existing)
  		return s.purchaseView(p, snap, pub, &ov), nil
  	} else if !errors.Is(err, repocommercial.ErrOrderNotFound) {
  		return PurchaseView{}, err
  	}


LLM retry report summary: 13 of 479 requests affected -- 5 requests failed, 1 request cancelled, 7 requests recovered after retry

Review planning (3 requests):
- internal/container/container.go,internal/handler/commercial.go,internal/handler/payment_callbacks.go,internal/middleware/auth.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/benefits.go,internal/modules/commercial/service/commercial/fulfillment.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go,internal/modules/commercial/service/commercial/purchase_fulfillment.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- docs/plans/issue-72-flow-evidence-83/seed_83.sh,docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh,docs/plans/issue-72-flow-evidence-83/v2_probe.sh,docs/plans/issue-72-flow-evidence-83/v2_up_backend.sh,docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh,docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Core review (9 requests):
- .gitignore,deploy/lago-lab/payment-activation/phases.py,deploy/lago-lab/payment-settle-trigger/.gitignore,deploy/lago-lab/payment-settle-trigger/fixtures.py,deploy/lago-lab/payment-settle-trigger/lab.sh,deploy/lago-lab/payment-settle-trigger/phases.py,deploy/lago-lab/payment-settle-trigger/run_lab.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- docs/plans/issue-72-flow-evidence-83/seed_83.sh,docs/plans/issue-72-flow-evidence-83/v2_gen_keys.sh,docs/plans/issue-72-flow-evidence-83/v2_probe.sh,docs/plans/issue-72-flow-evidence-83/v2_up_backend.sh,docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh,docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py: timed out -> failed
- internal/modules/commercial/purchase_command.go,internal/modules/commercial/purchase_settlement_command.go,internal/modules/commercial/subscription_command.go: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> failed
- .gitignore,deploy/lago-lab/payment-activation/phases.py,deploy/lago-lab/payment-settle-trigger/.gitignore,deploy/lago-lab/payment-settle-trigger/fixtures.py,deploy/lago-lab/payment-settle-trigger/lab.sh,deploy/lago-lab/payment-settle-trigger/phases.py,deploy/lago-lab/payment-settle-trigger/run_lab.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- .gitignore,deploy/lago-lab/payment-activation/phases.py,deploy/lago-lab/payment-settle-trigger/.gitignore,deploy/lago-lab/payment-settle-trigger/fixtures.py,deploy/lago-lab/payment-settle-trigger/lab.sh,deploy/lago-lab/payment-settle-trigger/phases.py,deploy/lago-lab/payment-settle-trigger/run_lab.py: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- ... and 4 more

Context compaction (1 request):
- internal/modules/commercial/repository/commercial/order.go,internal/modules/commercial/service/commercial/benefits.go,internal/modules/commercial/service/commercial/fulfillment.go,internal/modules/commercial/service/commercial/order.go,internal/modules/commercial/service/commercial/purchase.go,internal/modules/commercial/service/commercial/purchase_fulfillment.go: cancelled

Per-attempt detail: --format json (retry_report).
