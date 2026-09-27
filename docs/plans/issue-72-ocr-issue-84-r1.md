Review complete: 140 finding(s) across 163 selected item(s).

─── .gitignore:102-103 ───
[bug · medium] `data/` 未加路径锚定(缺少前导 `/`),会匹配仓库任意层级下名为 data 的目录,而不仅是根目录的实验 sqlite 库(docs/plans 各
seed.sh 以 worktree root 为 cwd 写入 data/issue8*.db)。仓库中实际存在 internal/textconv/data/(被跟踪的简繁转换词典
README.md/TSCharacters.txt/TSPhrases.txt,且 .github/workflows/app.yml 的 paths-ignore 引用
internal/textconv/data/**):已跟踪文件不受影响,但该目录下未来新增的任何文件(如词典更新)会被 git 静默忽略,造成提交缺文件且难以察觉。建议锚定为
`/data/`,与实验脚本写入的根目录路径精确对应。

  docs/plans/**/seed-login-*.json
- data/
+ /data/


─── .gitignore:98-102 ───
[documentation · medium] 注释与当前脚本行为不符,存在两处失实:① 「seed scripts write the RAW login answer here on every
rerun」——现有全部 seed 脚本(r4-flow/2/3、r5-verify、r5b-flowcheck 的 seed.sh 及 seed_83.sh、seed_84.sh)已改为直接落盘
REDACTED 版本(`jq '.token = "REDACTED" | .refresh_token = "REDACTED"'`),JWT 仅存活于当轮 shell,RAW
落盘的威胁模型已不存在;② 「this rule keeps a future rerun's live capture OUT of git status」——gitignore
对已跟踪文件无效,已跟踪的 12 个 seed-login-*.json 在 rerun 覆写时仍会出现在 git status
中,该规则对它们不提供任何保护(docs/plans/issue-72-ocr-issue-83-r2.md 已指出同样的不准确,但注释未同步修正)。误导性注释会让维护者误以为 RAW
落盘是常态、gitignore 是主要防线;若未来某脚本回归写 RAW 且覆写已跟踪路径,凭据仍可能被 git add -A
误提交。建议按实际机制改写注释:脚本内联脱敏是第一防线,此规则仅是针对新增未跟踪捕获文件的纵深防御;已跟踪文件的回归防护建议依赖 CI 侧凭据检查(如
cli/scripts/check-secret-tokens.sh)兜底。

- # (OCR C-83 / C-01) Flow-evidence login captures: seed scripts write the RAW
- # login answer here on every rerun — with live access/refresh JWTs. The
- # tracked copies are redacted (token fields = "REDACTED"); this rule keeps a
- # future rerun's live capture OUT of git status until it is redacted.
+ # (OCR C-83 / C-01) Flow-evidence login captures. The seed scripts write
+ # the REDACTED form directly (token fields = "REDACTED"; JWTs live only in
+ # the run's shell). This rule is defense-in-depth for NEW untracked captures
+ # only: it does NOT protect the already-tracked seed-login-*.json files —
+ # rerun overwrites surface as modified entries and their redaction relies on
+ # the scripts themselves (CI secret scan as backstop).
  docs/plans/**/seed-login-*.json


─── docs/plans/issue-72-flow-evidence-82/alipay_gateway_stub.py:82-86 ───
[bug · medium] rsa_verify_sha256 将 openssl 的所有非零退出统一判为"签名无效"，与 verify_request 上方 (OCR r2)
注释声明的契约直接矛盾：注释明确说"密钥文件缺失/不可读或 openssl 调用失败属于配置错误、必须抛异常"，但这里没有 check=True 也没有区分
stderr。实际效果是：FLOW82_KEY_DIR 配错或 SHARED_PUB 不存在时，openssl 以非零退出（Could not read public key），本函数返回
False，verify_request 随之返回 False，stub 对合法签名也回答 isv.invalid-signature——取证轮中会把配置问题误判为签名链
bug（正是注释声称已修复的那类误导）。建议区分"验证失败"（stderr 含 Verification Failure）与"调用失败"，后者抛出异常。

          proc = subprocess.run(
              ["openssl", "dgst", "-sha256", "-verify", pub_pem,
               "-signature", sig_path],
              input=content, capture_output=True)
-         return proc.returncode == 0
+         if proc.returncode == 0:
+             return True
+         if b"Verification Failure" in proc.stderr:
+             return False
+         raise RuntimeError(
+             f"openssl dgst -verify failed (config error, not a signature "
+             f"mismatch): {proc.stderr.decode(errors='replace').strip()}")


─── docs/plans/issue-72-flow-evidence-82/browser_paid_face_82.mjs:16-25 ───
[maintainability · low] 两个问题：(1) envRequired 从 _browser_lib.mjs 导入但从未调用，属于未使用导入；(2) 这段 _D→基础名
的回退+缺失即退出块与 browser_sync_face_82.mjs 的 _B 版本近乎逐字重复，与 _browser_lib.mjs 收敛登录/记录逻辑为单一副本的目标（r2:38
注释）相悖。建议在 _browser_lib.mjs 增加候选回退 helper（如 envRequiredAny：任一候选存在即可，返回第一个命中值），两个脚本复用；至少应删除未使用的
envRequired 导入或改为真正调用它。

- if (!process.env.FLOW82_EMAIL_D && !process.env.FLOW82_EMAIL) {
-   console.error('missing required env: FLOW82_EMAIL(_D)');
-   process.exit(2);
- }
- if (!process.env.FLOW82_PASSWORD_D && !process.env.FLOW82_PASSWORD) {
-   console.error('missing required env: FLOW82_PASSWORD(_D)');
-   process.exit(2);
- }
  const EMAIL = process.env.FLOW82_EMAIL_D ?? process.env.FLOW82_EMAIL;
  const PASSWORD = process.env.FLOW82_PASSWORD_D ?? process.env.FLOW82_PASSWORD;
+ if (!EMAIL || !PASSWORD) {
+   console.error('missing required env: FLOW82_EMAIL(_D) / FLOW82_PASSWORD(_D)');
+   process.exit(2);
+ }
+ // 更优：在 _browser_lib.mjs 中提供 envRequiredAny 候选回退 helper，与 sync 腿共用一份实现。


─── docs/plans/issue-72-flow-evidence-82/browser_sync_face_82.mjs:15-24 ───
[maintainability · low] 两个问题：(1) envRequired 从 _browser_lib.mjs 导入但从未调用，属于未使用导入；(2) 这段 _B→基础名
的回退+缺失即退出块与 browser_paid_face_82.mjs 的 _D 版本近乎逐字重复，与 _browser_lib.mjs 收敛单一副本的既定目标相悖。建议在
_browser_lib.mjs 增加候选回退 helper（如 envRequiredAny）供两条腿共用，避免后续新增腿继续复制该模式并在变量名/退出码约定上漂移。

- if (!process.env.FLOW82_EMAIL_B && !process.env.FLOW82_EMAIL) {
-   console.error('missing required env: FLOW82_EMAIL(_B)');
-   process.exit(2);
- }
- if (!process.env.FLOW82_PASSWORD_B && !process.env.FLOW82_PASSWORD) {
-   console.error('missing required env: FLOW82_PASSWORD(_B)');
-   process.exit(2);
- }
  const EMAIL = process.env.FLOW82_EMAIL_B ?? process.env.FLOW82_EMAIL;
  const PASSWORD = process.env.FLOW82_PASSWORD_B ?? process.env.FLOW82_PASSWORD;
+ if (!EMAIL || !PASSWORD) {
+   console.error('missing required env: FLOW82_EMAIL(_B) / FLOW82_PASSWORD(_B)');
+   process.exit(2);
+ }
+ // 更优：在 _browser_lib.mjs 中提供 envRequiredAny 候选回退 helper，与 paid 腿共用一份实现。


─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:11-11 ───
[maintainability · low] 本次改动删除了 glob.glob 的唯一调用点（runs/ 回退探测），import glob 成为未使用的死导入，建议一并删除。



─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:18-18 ───
[maintainability · low] RUNS 仅被已删除的 runs/ 回退逻辑（glob.glob(str(RUNS /
"db-watch-verify-*.tsv"))）使用，本改动后文件中再无代码引用（仅剩注释文本提及 runs/），成为死变量，建议删除该定义。



─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:33-33 ───
[documentation · low] 紧邻上方保留的旧注释末句（"Fall back to the newest runs/ TSV only when no archive exists,
and always print the file actually used."）描述的是本次已删除的 runs/ 回退行为，与新逻辑（仅认归档文件、缺失即退出码
2）直接矛盾，会误导后续维护者，建议同步清理该句。

+ # (R3-17) 仅认脚本旁归档的 verify-db-watch-samples.tsv；缺失即 MISSING-EVIDENCE。
  if not archived.exists():


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:18-18 ───
[maintainability · low] 本次改动删除了 glob.glob 的唯一调用点（runs/ 回退探测），import glob 成为未使用的死导入，建议一并删除。



─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:25-25 ───
[maintainability · low] RUNS 仅被已删除的 runs/ 回退逻辑（glob.glob(str(RUNS /
"db-watch-verify-*.tsv"))）使用，本改动后文件中再无代码引用（仅剩注释/docstring 文本提及 runs/），成为死变量，建议删除该定义。



─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:40-40 ───
[documentation · low] 两处描述已删除回退行为的过期文本未同步更新：1) 顶部 docstring 中 "the runs/ observer TSV is only a
fallback when no archive exists"；2) archived 上方旧注释末句 "Fall back to the newest runs/ TSV only when no
archive exists, and always print the file actually used."。二者均与新逻辑（仅认归档文件、缺失即退出码 2）矛盾，建议一并清理。

+ # (R3-17) 仅认脚本旁归档的 verify-db-watch-samples.tsv；缺失即 MISSING-EVIDENCE。
  if not archived.exists():


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:15-15 ───
[maintainability · low] 本次改动删除了 glob.glob 的唯一调用点（runs/ 回退探测），import glob 成为未使用的死导入，建议一并删除。



─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:22-22 ───
[maintainability · low] RUNS 仅被已删除的 runs/ 回退逻辑（glob.glob(str(RUNS /
"db-watch-verify-*.tsv"))）使用，本改动后文件中再无代码引用（仅剩注释文本提及 runs/），成为死变量，建议删除该定义。



─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:37-37 ───
[documentation · low] 紧邻上方保留的旧注释末句（"Fall back to the newest runs/ TSV only when no archive exists,
and always print the file actually used."）描述的是本次已删除的 runs/ 回退行为，与新逻辑（仅认归档文件、缺失即退出码
2）直接矛盾，会误导后续维护者，建议同步清理该句。

+ # (R3-17) 仅认脚本旁归档的 verify-db-watch-samples.tsv；缺失即 MISSING-EVIDENCE。
  if not archived.exists():


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:15-15 ───
[maintainability · low] 本次改动删除了 glob.glob 的唯一调用点（runs/ 回退探测），import glob 成为未使用的死导入，建议一并删除。



─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:22-22 ───
[maintainability · low] RUNS 仅被已删除的 runs/ 回退逻辑（glob.glob(str(RUNS /
"db-watch-verify-*.tsv"))）使用，本改动后文件中再无代码引用（仅剩注释文本提及 runs/），成为死变量，建议删除该定义。



─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:37-37 ───
[documentation · low] 紧邻上方保留的旧注释末句（"Fall back to the newest runs/ TSV only when no archive exists,
and always print the file actually used."）描述的是本次已删除的 runs/ 回退行为，与新逻辑（仅认归档文件、缺失即退出码
2）直接矛盾，会误导后续维护者，建议同步清理该句。

+ # (R3-17) 仅认脚本旁归档的 verify-db-watch-samples.tsv；缺失即 MISSING-EVIDENCE。
  if not archived.exists():


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_04_active_face.mjs:44-45 ───
[test · medium] leg4 缺少与 leg1-3 相同的 (A-03) catch 兜底：try/finally 之间没有
catch，基础设施故障（登录被拒、waitForSelector 超时等）会直接抛出未捕获异常，RESULT 汇总行不会打印，违反本组脚本自行声明的"断言失败与脚本基础设施失败可区分"契约（对比
browser_01/02/03 的 note('script-error', ...)，以及被复制的源文件 r4-flow2/browser_04_active_face.mjs ——
该文件是有完整 catch 的，本次复制时被裁掉，属回归）。注意本文件的 page 是 try 内的 const，补 catch 时需同时改为 leg1-3 的 `let page;` 提升声明，否则
catch 中不可见。

+ } catch (err) {
+   // (A-03) same contract as legs 1-3: an infrastructure failure still emits
+   // an explicit FAIL note + RESULT line, distinguishable from an assertion
+   // failure. (Requires hoisting `const page` inside try to `let page;` above.)
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}07-billing-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
  } finally { await browser.close(); }
  console.log('RESULT ' + JSON.stringify(results));


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:53-53 ───
[test · low] checkoutHref 经 getAttribute 获取后未做非空断言即写入 order-info.json：orderId
有正则断言（order-id-visible），而 href 缺失时 getAttribute 返回 null，会静默落盘 "checkoutHref": null 且不产生任何 FAIL note
—— pay-link-present 只断言了文案存在，不覆盖 href 属性缺失的形态，后续依赖该 href 的取证/复放腿会拿到无效值。

    const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
+   note('checkout-href-present', typeof checkoutHref === 'string' && checkoutHref.length > 0,
+     `channel checkout href: ${checkoutHref ?? '(href attribute missing)'}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/browser_01_checkout.mjs:30-33 ───
[maintainability · low] 四个 leg 各自内联重复约 30 行登录序列/note 助手/RESULT 打印/env 校验脚手架，而本次更新已在父目录新增共享库
_browser_lib.mjs（login/note/runLeg/evidencePath/envRequired），同批次的 r5-verify、r5b-flowcheck 各 leg
均已导入复用。继续逐轮复制会扩大漂移面 —— 本组 leg4 恰好丢了 (A-03) catch，正是这种复制漂移的实证（runLeg 已把该契约固化为"throw 也必须输出
RESULT"）。建议新轮次脚本改用共享库（四个文件同理）。

-   await page.goto(`${WEB}/login`);
-   await page.fill('#auth-email', EMAIL);
-   await page.fill('#auth-password', PASSWORD);
-   await page.locator('form[aria-label="Login form"] button[type="submit"]').click();
+ // reuse the shared leg library (as r5-verify/browser_01_checkout.mjs does):
+ // import { login, note, runLeg, evidencePath, envRequired } from '../_browser_lib.mjs';
+ // ...
+ await login(page, WEB, EMAIL, PASSWORD);


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:140-140 ───
[security · low] LAGO_KEY 经 curl argv（-H "Authorization: Bearer $LAGO_KEY"）传递，本地 ps 可观测，与脚本自身对密码采用
jq+STDIN 以规避 ps 观测的安全姿态不一致（branch redline 注释明言"curl argv password is observable in local ps"——API
key 同理）。两处 plans 读取（baseline 与 post-publish）均涉及；TOKEN_B 的两处亦同。虽为 localhost 实验环境，建议与密码同样收敛为 stdin
传递，统一敏感凭据处理标准。

- curl -s "http://127.0.0.1:48889/api/v1/plans" -H "Authorization: Bearer $LAGO_KEY" > "$EV/api-03-lago-plans-baseline.json"
+ # keep the bearer credential off the argv (same ps-observability posture as
+ # the password riding STDIN): curl -H @- reads the header line from stdin.
+ printf 'Authorization: Bearer %s\n' "$LAGO_KEY" \
+   | curl -s --max-time 30 "http://127.0.0.1:48889/api/v1/plans" -H @- > "$EV/api-03-lago-plans-baseline.json"


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:156-156 ───
[maintainability · low] 诊断退化问题（两处）：1) 所有 curl 均未设置 --max-time，后端/Lago 挂起时脚本会无限期阻塞（browser 各腿有 45s
超时，seed 没有）；2) draft/publish/Lago 响应为非 JSON（如代理返回 HTML 错误页）时，本行的 jq 解析在 set -e 下直接以原始报错中止，精心编写的
"FAIL: plan draft rejected" 友好断言永远执行不到，削弱脚本自述的快速失败可诊断性。BASE_N/N 的 jq 提取同理。建议为 curl 统一加
--max-time，并在提取字段前先做 jq -e . 可解析性前置校验。

+ # fail fast with the friendly diagnosis instead of a raw jq parse error
+ jq -e . "$EV/api-01-draft.json" >/dev/null \
+   || { say "FAIL: plan draft response is not JSON (see api-01-draft.json)"; exit 1; }
  DRAFT_V=$(jq -r '.data.version // 0' "$EV/api-01-draft.json")


─── docs/plans/issue-72-flow-evidence-82/r4-flow3/seed.sh:128-128 ───
[test · low] 授权回读只打印未断言：若 insert 失败（DB_PATH 指向了存在但 schema 不符的文件、表缺失等），count 查询同样失败，输出退化为 "granted: 
row(s)" 而脚本继续执行，直到后续 publish 因 b 缺少 plan_publish 能力才间接失败 —— 归因被推迟且模糊。这与脚本自身 (A-13)/(A-14) 确立的
"asserted, never merely printed" 姿态不一致，且 say 管道退出状态取决于最后一个命令（tee），命令替换内的 sqlite3 失败不会被 set -e 捕获。

- say "granted: $(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"
+ GRANT_N=$(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';")
+ [ "$GRANT_N" -ge 1 ] 2>/dev/null \
+   || { say "FAIL: plan_publish grant not visible for B (count='$GRANT_N') — wrong DB_PATH/schema?"; exit 1; }
+ say "granted: $GRANT_N row(s)"


─── deploy/lago-lab/payment-activation/phases.py:575-577 ───
[test · medium] 新增的 exactly-once 断言忽略了 `_payments_for` 返回的 HTTP 状态 `ps`。`LagoRestClient.request` 对非
2xx 不抛错（仅传输失败抛 OSError），body 非 dict 或缺少 `payments` 键时 `_payments_for` 返回空列表——payments API 一次 4xx/5xx
会让 `non_succeeded` 计数为 0，"至多一笔非成功支付"的 AC1 断言空转通过。两处新增调用点（invoice 不可见分支，及其共享结果的 canceled 分支与下方
invoice visible-as-failed 分支）同样受影响。建议先校验 `_ok(ps)`，读取失败时显式 FAIL 而非静默放行。

              ps, payments = _payments_for(ctx, customer["external_id"])
+             if not _ok(ps):
+                 observed["error"] = f"payments API read failed (HTTP {ps})"
+                 return _report(ctx, "gate", expected, observed, FAIL)
              non_succeeded = [p for p in payments if p.get("status") != "succeeded"]
              observed["payments_non_succeeded_count"] = len(non_succeeded)


─── deploy/lago-lab/payment-settle-trigger/phases.py:753-755 ───
[test · medium] P-D 的 inbound_webhooks 行落地校验存在静默跳过路径：`inbound_before` 为 None（投递前 psql 读取失败）而
`inbound_after` 读取成功时，`(inbound_before is not None and ...)` 短路使整个条件不成立——即使 after 计数为
0（行未落地）也判定通过，F9 接收半程的验证形同虚设。建议 baseline 读取失败时显式归类 blocked-env（与 webhook_secret/org_id
读取失败的处理一致），而非放宽校验。

-         if inbound_after is None or (inbound_before is not None
-                                      and int(inbound_after) < int(inbound_before) + 1):
+         if inbound_before is None or inbound_after is None:
+             return _blocked(ctx, "settle_trigger",
+                             "inbound_webhooks count not readable from the lab DB",
+                             expected)
+         if int(inbound_after) < int(inbound_before) + 1:
              observed["error"] = "inbound_webhooks row did not land"


─── deploy/lago-lab/payment-settle-trigger/phases.py:807-809 ───
[maintainability · low] 同一变更集中 payment-activation 的 cleanup 刚因 R3-39 漂移教训把 "terminated"
加入删除候选（该漂移会把订阅留在 terminated，原三态候选无法删除），而本实验室面向同一 pinned v1.53.0 运行时的订阅删除候选仍缺 "terminated"；若本实验室订阅也落入
terminated 终态，cleanup 将记为删除失败。建议与 payment-activation 对齐补齐候选集。

          deleted = False
          last_status = None
-         for sub_status in ("active", "incomplete", "canceled"):
+         for sub_status in ("active", "incomplete", "canceled", "terminated"):


─── deploy/lago-lab/payment-settle-trigger/phases.py:413-415 ───
[security · low] `deliver_stripe_webhook` 用裸 `build_opener(ProxyHandler({}))` 投递带有效签名的 webhook 载荷：默认
HTTPRedirectHandler 会跟随任意 30x，签名载荷可能被重定向送往非预期主机；而同文件 `RunContext.graphql` 已特意改用
`clients._proxyless_opener` 的源站绑定 opener（OffOriginRedirect 纪律，见其 OCR r2
注释）。同一模块内出网/重定向纪律不一致，建议统一复用源站绑定 opener。

-         opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
+         from urllib.parse import urlsplit
+         opener = clients._proxyless_opener(urlsplit(self.lago_url)[:2])
          import hashlib
          payload_sha = hashlib.sha256(payload).hexdigest()


─── deploy/lago-lab/payment-settle-trigger/run_lab.py:257-257 ───
[maintainability · low] `PHASE_FILES[name]` 的字典访问位于 run_one 的 try/except 之外：未来向 phases.PHASE_ORDER
新增阶段而漏配文件名时会抛 KeyError，逃过分类报告契约——environment/timeline 不再落盘，进程带原始堆栈退出。现有
test_phase_order_contract_matches_runner_order 只锁定阶段顺序，不覆盖文件名映射。建议模块加载时 fail-fast 校验映射完整性。

          write_json(output_dir / PHASE_FILES[name], report)
+ 
+ # 模块加载时校验（置于 PHASE_SEQUENCE 定义之后）：
+ # _missing = [name for name, _fn in PHASE_SEQUENCE if name not in PHASE_FILES]
+ # if _missing:
+ #     raise RuntimeError(f"PHASE_FILES missing entries for phases: {_missing}")


─── deploy/lago-lab/payment-settle-trigger/run_lab.py:314-315 ───
[bug · low] 密钥扫描发现命中时 `overall` 被改为 "fail"，但该值只影响退出码：environment 报告中 `run.overall`
仍保留阶段级结论，第二次写盘虽附上了 secrets_scan，evidence JSON 却与退出码/timeline（"overall verdict:
fail"）矛盾。建议同步更新嵌套字段，保证报告口径一致。

      if not scan["clean"]:
          overall = "fail"
+         environment["run"]["overall"] = overall


─── deploy/lago-lab/payment-settle-trigger/lab.sh:88-89 ───
[bug · low] `${url##*:}` 提取端口无法处理带路径或尾斜杠的 URL：`http://127.0.0.1:48895/` 会得到 `48895/`，被误判为不一致并 die（下游
clients 均对 URL rstrip("/")，该写法本可正常工作）；无端口的主机名也会得到 `//host` 误判。手工编辑 lab.env 的合法变体被拒绝且报错未提示真实原因。建议先截掉
scheme 与路径再取端口。

-     url_port="${url##*:}"
+     url_host="${url#*://}"
+     url_port="${url_host%%/*}"
+     url_port="${url_port##*:}"
      if [[ "$url_port" != "$port" ]]; then


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:76-76 ───
[bug · medium] 死代码 + 虚假同步：全仓库检索确认 `window.__r4PurchasesSeen` 没有任何赋值点（前端 CheckoutPage.tsx
不注入此全局，本脚本也无 addInitScript/evaluate 设置它），该 waitForFunction 条件恒为假、1 秒必超时且被 `.catch(() => {})`
静默吞掉。它伪装出"已等待抓包完成"的同步并不存在——所幸待付款面只在 POST /purchases 完成后才渲染，实际竞态窗口很小，但这条死等待（此前 OCR r1/r2
已两轮指出仍未移除）持续误导读者以为存在同步保障。建议直接删除；若确需显式同步，请轮询 Node 侧已捕获的 purchases 数组。

-   await page.waitForFunction(() => window.__r4PurchasesSeen > 0, null, { timeout: 1000 }).catch(() => {});
+   // 无需额外等待：待付款面只在 POST /purchases 完成后才渲染，此时
+   // page.on('request') 监听器必然已捕获到请求体。
+   // 若要显式同步，请轮询 Node 侧已捕获的数组而非一个从未被赋值的页面全局：
+   // for (let i = 0; i < 50 && purchases.length === 0; i++) await page.waitForTimeout(100);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:34-37 ───
[maintainability · medium] 缺失 (A-03) 基础设施失败兜底 + 重复脚手架：本脚本只有 try/finally 没有 catch——登录被拒、选择器超时等任何
throw 会直接以裸栈跟踪退出，RESULT 汇总行不会打印，违背取证脚本"可区分断言失败与基础设施失败"的自身纪律（browser_02/browser_04 均已实现 catch +
note('script-error')）。同时本次更新同批新增了共享库 ../_browser_lib.mjs（导出
runLeg/login/note/evidencePath/envRequired，runLeg 内置 A-03 catch 且保证 RESULT
行与非零退出码），r5-verify、r5b-flowcheck 的同名四腿均已改为导入复用，唯独 r4-flow 四个脚本各自内联复制了约 30 行登录/记录脚手架，且质量已经漂移（仅 02/04
有 catch、01 残留 __r4PurchasesSeen 死等待）。建议与 r5 系列对齐改用共享库。

- const browser = await chromium.launch();
- try {
+ import { login, note, runLeg, evidencePath, envRequired } from '../_browser_lib.mjs';
+ 
+ await runLeg(async (results, browser) => {
    const page = await (await browser.newContext()).newPage();
    page.setDefaultTimeout(45000);
+   // ...断言体（runLeg 自动兜底 A-03：任何 throw 仍输出 RESULT 并以非零退出）
+ });


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_01_checkout.mjs:73-73 ───
[bug · low] checkoutHref 缺少 null 检查：`getAttribute('href')` 可能返回 null，此处既未断言也未兜底，null 会被原样写入
order-info.json 供后续腿与读者误读（链接缺失本应是一条 FAIL 证据而非静默的 null 字段）。建议落盘前断言其为 http/https 非空字符串。

    const checkoutHref = await page.locator('main a:has-text("前往支付")').first().getAttribute('href');
+   note('checkout-href-present', typeof checkoutHref === 'string' && /^https?:\/\//.test(checkoutHref),
+     `checkout link href: ${checkoutHref ?? '(null)'}`);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/browser_03_paid_face.mjs:56-60 ───
[maintainability · medium] 缺失 (A-03) 基础设施失败兜底：与 browser_02/browser_04 不同，本脚本只有 try/finally 没有
catch——登录被拒、`text=已付款，权益处理中` 或 `text=已付款待激活` 选择器超时（复核时最需要捕捉的失败形态）会以裸栈跟踪退出，RESULT
汇总行不打印，下游无法区分"断言失败"与"基础设施失败"。建议补齐与 browser_02/04 相同的 catch（note('script-error') + 错误截图），或直接改用
../_browser_lib.mjs 的 runLeg 包装（r5 系列同名脚本已如此）。

+ } catch (err) {
+   // (A-03) 基础设施失败也要落为显式 FAIL：RESULT 仍打印、退出码非零。
+   note('script-error', false, String(err));
+   if (page) {
+     try { await page.screenshot({ path: `${EV}05-paid-face-script-error.png`, fullPage: true }); } catch { /* best effort */ }
+   }
  } finally {
    await browser.close();
  }
  console.log('RESULT ' + JSON.stringify(results));
  process.exit(results.every((r) => r.ok) ? 0 : 1);


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:33-39 ───
[bug · medium] prune 波及面过大，会误删其他实验的在用端点：清理条件是 description 含 "weknora" 即删（limit 100），但 t11
实验（deploy/lago-lab/payment-settle-trigger/phases.py:346）铸造的端点 description 为 "weknora t11 local
settle probe <run_id> …"，t9 实验为 "weknora t9 integration secret-mint stand-in …"——只要共用同一 Stripe
测试账号，运行 t11 中途或其清理残留的端点都会被本脚本删掉，破坏 t11 记录在 state 里的 endpoint_id 清理链与其 harness
投递。本脚本自己铸造的端点带有唯一标记（description 含 "r4"、URL 以 /webhooks/stripe/r4 结尾），建议把 prune 条件收窄到本轮自己的标记。

  pruned = 0
  for row in rows:
-     if "weknora" in str(row.get("description", "")):
+     desc, url = str(row.get("description", "")), str(row.get("url", ""))
+     # 只清理本轮自己的标记，避免误删 t11/t9 等其他实验的在用端点
+     if "weknora r4" in desc or "/webhooks/stripe/r4" in url:
          opener.open(urllib.request.Request("https://api.stripe.com/v1/webhook_endpoints/" + row["id"],
                                             headers={"Authorization": auth}, method="DELETE"),
                      timeout=30).read()
          pruned += 1


─── docs/plans/issue-72-flow-evidence-82/r4-flow/prepare_82flow_webhook_secret.sh:16-17 ───
[maintainability · low] stderr 重定向到 /dev/null 吞掉唯一诊断信息：读取存量 secret 的 docker exec 把 stderr 全部丢弃，而脚本
`set -euo pipefail` 下该管道失败会直接中止——结果是容器不可达 / rails runner 报错时脚本"无声退出"，没有任何错误文本可供定位（本目录其它脚本的 OCR
注释反复强调"让诊断可达"，此处却相反）。sed 的 sentinel 前缀提取本就隔离了 stdout，无需静音 stderr。

  STORED=$(docker compose -f "$COMPOSE_FILE" -p "$PROJECT" exec -T api bin/rails runner \
-   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s" 2>/dev/null | sed -n 's/.*__R4SECRET__//p')
+   "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__R4SECRET__' + s" | sed -n 's/.*__R4SECRET__//p')


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:131-132 ───
[security · medium] SQL 字符串拼接与项目安全红线冲突：INSERT 与随后的 SELECT 都把来自登录 HTTP 响应的 $UID_B 以字符串拼接方式内插进 sqlite3
语句。虽已设 uuid_shape 严格 UUID 白名单兜底（且经核对 Grant 模型列名与 hasPlatformPlanPublisher 的 tenant_id=0
平台范围语义完全匹配，行结构正确），但验收条件为绝对条款——"数据库查询一律使用参数绑定，禁止拼接 SQL"。建议改用 python3 的 sqlite3 模块做参数绑定执行（本实验族已依赖
python3，同目录 prepare 脚本即用）。

- sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
- say "granted: $(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"
+ say "== grant plan_publish to B at platform scope (seed row) =="
+ # 参数绑定执行（红线：禁止拼接 SQL）；uuid_shape 白名单保留为快速失败的前置防线
+ GRANTS=$(python3 - "$DB" "$UID_B" <<'PY'
+ import sqlite3, sys
+ db, uid = sys.argv[1], sys.argv[2]
+ con = sqlite3.connect(db)
+ con.execute("insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) "
+             "values (0, ?, 'plan_publish', 'flow-verifier-r4', 1)", (uid,))
+ con.commit()
+ print(con.execute("select count(*) from commercial_grants where capability='plan_publish' and user_id=?", (uid,)).fetchone()[0])
+ PY
+ )
+ say "granted: ${GRANTS} row(s)"


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:15-15 ───
[maintainability · low] 实验端点硬编码且无环境变量出口：BACKEND、Lago 地址 :48889（LAGO_KEY 读取处）、容器名
weknora-lago-82flow-db-1 均为字面量，而同目录浏览器腿的 WEB 已有 FLOW82_WEB
覆盖出口——换端口/换容器名复跑时只有此处会以难懂的方式失败。建议与浏览器腿保持同一参数化约定。

- BACKEND=http://127.0.0.1:8093
+ BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"
+ LAGO="${FLOW82_LAGO:-http://127.0.0.1:48889}"
+ LAGO_DB="${FLOW82_LAGO_DB:-weknora-lago-82flow-db-1}"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:11-13 ───
[bug · medium] EV 在 `cd` 到 worktree 根之后才用 `$0` 计算：当在 round 目录内以 `bash seed.sh` / `./seed.sh`
相对调用时，第二次 `dirname "$0"` 解析为 `.`，而 cwd 已是仓库根，EV
会静默变成仓库根——seed-run.txt、seed-login-a/b.json、api-0*.json 等证据文件全部写错位置，破坏"证据落在各轮目录"的不变量（且无任何报错）。应先解析 EV
再 cd（此模式同样存在于 r4-flow/r4-flow3/r5 系列 seed.sh，建议一并修正）

  set -euo pipefail
- cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
- EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
+ EV="$(cd "$(dirname "$0")" && pwd)"   # resolve the evidence dir BEFORE any cd
+ # worktree root (docs/plans/<dir>/<round> -> root)
+ cd "$EV/../../../.."


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:130-130 ───
[security · medium] grant 插入与随后的 count 查询均以字符串拼接向 sqlite3 CLI 插值 `$UID_B`，仅靠 uuid_shape
白名单防护，与分支安全红线"数据库查询一律使用参数绑定"不符——当前两处被 UUID 白名单覆盖，但后续任何人新增一个插值点即失去保护。同时 CLI 直写运行中后端持有的 SQLite
库，绕过后端校验且无锁协调，可能与后端并发写触发 database is locked。建议改用 python3 的 qmark 占位符绑定（仓库已依赖
python），白名单退化为纵深防御而非唯一闸门

- sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r4', 1);"
+ python3 - "$DB" "$UID_B" <<'PY'
+ import sqlite3, sys
+ db = sqlite3.connect(sys.argv[1])
+ db.execute(
+     "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) "
+     "values (0, ?, 'plan_publish', 'flow-verifier-r4', 1)",
+     (sys.argv[2],),
+ )
+ db.commit()
+ PY
+ say "granted: $(python3 -c "import sqlite3,sys; print(sqlite3.connect(sys.argv[1]).execute(\"select count(*) from commercial_grants where capability='plan_publish' and user_id=?\", (sys.argv[2],)).fetchone()[0])" "$DB" "$UID_B") row(s)"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:34-36 ───
[security · low] 注释声称密码走 STDIN 是为了避免 curl argv 被 ps 观测，但 `jq --arg p "$PW"` 同样把密码放入 jq 子进程的 argv，本地
ps 一样可见——安全姿态与实现自相矛盾。应改用环境变量注入 jq（env.PW），使密码不出现在任何子进程 argv（reg() 与 login()
两处同改；r4-flow/r4-flow3/r5-verify/r5b-flowcheck 同模式，可一并修）

-   jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}' \
+   PW="$PW" jq -nc --arg e "$1" '{username:$e,email:$e,password:env.PW}' \
      | curl -s -o /dev/null -w "%{http_code}" -X POST "$BACKEND/api/v1/auth/register" \
          -H 'Content-Type: application/json' --data @-


─── docs/plans/issue-72-flow-evidence-82/_browser_lib.mjs:64-76 ───
[bug · low] runLeg 将 requirePlaywright() 与 chromium.launch() 放在 try/catch 之外：当浏览器启动失败（playwright
浏览器二进制缺失、沙箱/资源限制等常见场景）时，runLeg 以未处理的 rejection 直接抛栈退出，既不会输出 RESULT 行，也不会记录 'leg-error' FAIL
note——恰好违反本函数头注释冻结的 A-03 契约（"a browser leg that THROWS must still emit a RESULT line ... never a
bare stack trace"）。依赖解析 RESULT 行的验证流水线（seed.sh 等）会在该场景下漏判。建议把启动也纳入 try，并在 finally 中判空后再关闭。

  export async function runLeg(body) {
-   const { chromium } = requirePlaywright();
    const results = [];
-   const browser = await chromium.launch();
+   let browser;
    try {
+     const { chromium } = requirePlaywright();
+     browser = await chromium.launch();
      await body(results, browser);
    } catch (err) {
      // (A-03) A thrown browser leg must not die with a bare stack trace and
      // a zero exit code: record the failure face, keep the RESULT contract.
      note(results, 'leg-error', false, String(err?.message ?? err));
    } finally {
-     await browser.close().catch(() => { /* best-effort close */ });
+     if (browser) await browser.close().catch(() => { /* best-effort close */ });
    }


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs:23-28 ───
[maintainability · low] 本轮同一 PR 新增了共享库 _browser_lib.mjs（导出
login/note/runLeg/evidencePath/envRequired），r5-verify 与 r5b-flowcheck 的全部腿均已复用，而同为新增的 r4-flow2
四个腿却各自内联了 ~25 行登录、results/note 记录、try/catch/finally 与错误截图样板（内容基本逐字相同）。登录选择器集一旦变化（如 #auth-email
改名），需要同步修 4 处。建议像 r5 系列一样 import 共享库（注意 lib 的 LOGIN.timeoutMs 为 30000，与本组内联的 45000 略有差异，复用时统一取舍）

- const results = [];
- const purchases = [];
- const note = (step, ok, detail) => {
-   results.push({ step, ok, detail });
-   console.log(`${ok ? 'PASS' : 'FAIL'} | ${step} | ${detail}`);
- };
+ import { login, note, runLeg, evidencePath, envRequired } from '../_browser_lib.mjs';
+ // ...then drive the leg body via runLeg(async (results, browser) => { ... }),
+ // reusing login(page, WEB, EMAIL, PASSWORD) and note(results, step, ok, detail)


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:14-14 ───
[maintainability · low] BACKEND 端点、Lago 端口 :48889（两处 curl 直写）与容器名 weknora-lago-82flow-db-1 全部硬编码且无
env 覆盖，与同组 mjs 侧 FLOW82_WEB 可覆盖的设计不一致；此前 OCR r1 已给出 `BACKEND="${FLOW82_BACKEND:-...}"`
系列建议但本新增脚本未采纳，跨环境重跑需改源码

- BACKEND=http://127.0.0.1:8093
+ BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"
+ LAGO="${FLOW82_LAGO:-http://127.0.0.1:48889}"
+ LAGO_DB_CONTAINER="${FLOW82_LAGO_DB_CONTAINER:-weknora-lago-82flow-db-1}"


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:178-179 ───
[test · low] A-09 的"严格递增"断言存在并发归属窗口：82flow Lago 栈跨轮共享，BASE_N 读取与发布后读取之间若另一轮次并发发布 9900 套餐，同样满足 N >
BASE_N，产生跨轮假阳性（脚本对注册隔离做了并发断言，对 Lago 计数却没有）。且确定性 plan code（DeterministicPlanCode("pro",1) →
weknora-pro-v1）跨轮相同，纯 amount_cents 计数本质上无法归属到本轮。建议改用本轮专属 plan_key（如 pro-r4f2）或断言目标 plan 的 created_at
晚于本轮 baseline 读取时刻，把"本轮发布到达"做成可归属的证明

- if [ "$N" -gt "$BASE_N" ]; then
-   say "lago 9900 plan count increased ($BASE_N -> $N): THIS round's publish arrived"
+ # attribute THIS round's arrival by creation time, not by cross-round count
+ PUBLISHED_AT=$(jq -r '[.plans[] | select(.amount_cents==9900 and .created_at > "'"$BASELINE_TS"'")] | length' "$EV/api-03-lago-plans.json")
+ if [ "$PUBLISHED_AT" -ge 1 ]; then
+   say "lago 9900 plan created after this round's baseline read: THIS round's publish arrived"
+ else
+   say "FAIL: no 9900 plan created after baseline — this round's publish not attributable"; exit 1
+ fi


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:159-159 ───
[test · low] DRAFT_V 取自后端 JSON 后被内插进 publish URL 与 jq
程序字符串（"publish_plan_version:pro:$DRAFT_V"）；而前置断言 `.data.version >= 1` 在 jq 的全序中 string > number
恒真，字符串形状的 version（当前契约恒为数值，理论不可达）也能通过。作为 A-14"断言驱动"的一部分，宜补一个数值形状校验再进入插值，成本一行

  DRAFT_V=$(jq -r '.data.version // 0' "$EV/api-01-draft.json")
+ case "$DRAFT_V" in ''|*[!0-9]*) say "FAIL: draft version is not a positive integer ('$DRAFT_V') — refusing URL/jq interpolation"; exit 1 ;; esac


─── apps/web/src/commercial/CheckoutPage.tsx:263-264 ───
[bug · high] 迟到覆盖竞态:restartCheckout 只递增 retryToken,不推进 scope generation 也不 abort 信号,因此
live()(signal.aborted + isCurrent)对重启前已发出的 getOrder/purchaseStatus 在途请求始终放行。轮询每 3
秒一次(POLL_INTERVAL_MS=3000),且后端 GetOrder 含渠道恢复查询、解析可能慢于「新报价+新购买」两次往返:旧请求迟到解析时 prev.status 已是
'ready',{...prev, order} 会把新订单覆盖回被放弃的旧 pending 订单及其 checkout_url,轮询 effect 随 order?.id
变化转而持续轮询旧订单——渠道切换场景下旧链接仍可付款,用户可能对新旧两单重复付款。建议 live() 增加 orderIdRef 一致性校验(或改为在 setState updater 中比对
prev.order.id === id),嵌套的 purchaseStatus 更新同理。

      const live = (): boolean =>
-       !currentScope.signal.aborted && scopeController.isCurrent(currentScope.scope);
+       !currentScope.signal.aborted && scopeController.isCurrent(currentScope.scope)
+       && orderIdRef.current === id;


─── apps/web/src/commercial/CheckoutPage.tsx:28-32 ───
[maintainability · medium] purchaseStateMessage 仍持有私有状态文案,与 order-state.ts 的 PURCHASE_STATE_LABEL
对同一闭合状态措辞不一致:paid_awaiting_activation('已付款,权益处理中' vs '已付款待激活')、active('权益已生效' vs
'已生效')、awaiting_payment('待付款(权益未开通)' vs '待付款(权益未开放)')。BillingPage 注释声称
D15-f「两页不再各持一套」,此处构成用户可见文案漂移并违背该声明。建议 purchase 驱动的分支直接消费共享词表(order 渠道面兜底分支保持 orderMessage 措辞不变)。

  export function purchaseStateMessage(purchase: PurchaseView | undefined, order: OrderView): string {
    switch (purchase?.state) {
-     case 'paid_awaiting_activation': return '已付款，权益处理中';
-     case 'active': return '权益已生效';
-     case 'awaiting_payment': return '待付款（权益未开通）';
+     case 'paid_awaiting_activation': return PURCHASE_STATE_LABEL.paid_awaiting_activation;
+     case 'active': return PURCHASE_STATE_LABEL.active;
+     case 'awaiting_payment': return PURCHASE_STATE_LABEL.awaiting_payment;


─── apps/web/src/commercial/CheckoutPage.tsx:101-102 ───
[maintainability · low] PURCHASE_CONFLICT_MESSAGES 遗漏了后端 Purchase handler 实际会返回的 409 令牌 'purchase
pending exists'(internal/handler/commercial.go:526,后端注释明确这是可重试冲突)。该令牌当前落入 purchaseErrorText
的统一兜底「购买未能创建,请稍后重试」——动作指引正确但文案未区分「已有一笔购买正在处理」这一情形。建议补齐该令牌的中文映射,并考虑在后端分派表与前端映射之间建立同步清单,避免英文字符串精确匹配这一隐
式契约随措辞变动静默失配。

+   'purchase pending exists': '已有一笔购买正在处理，请稍后重试',
    'purchase failed': '购买未能创建，请稍后重试',
  };


─── apps/web/src/commercial/CheckoutPage.tsx:334-334 ───
[style · low] 支付渠道 fieldset 使用静态内联 style(border/borderRadius/margin/padding
均为常量),违反内联样式规范(仅动态样式允许内联);本页其余元素均用 tailwind className(如 'border-b
border-[#e7e7ea]')。建议改为样式类,与页面既有做法一致。

-               <fieldset style={{ border: '1px solid #e7e7ea', borderRadius: 8, margin: '12px 0', padding: '8px 12px' }}>
+               <fieldset className="my-3 rounded-lg border border-[#e7e7ea] px-3 py-2">


─── apps/web/src/commercial/CheckoutPage.tsx:378-379 ───
[maintainability · low] checkout_url && isSafeCheckoutUrl(...) 在上方 IIFE 与此处共求值两次:注释宣称「checkoutHref
是唯一事实源,两处判断共用」,但此处是独立的一份条件表达式,后端若调整 checkout_url 语义(如改为空串/新增字段)两处会漂移。建议在组件体(return 之前)提取 const
checkoutHref = ...,IIFE 与本条件共同消费。

-               {state.order.payment === 'pending'
-                 && !(state.order.checkout_url && isSafeCheckoutUrl(state.order.checkout_url)) ? (
+               {state.order.payment === 'pending' && !checkoutHref ? (


─── apps/web/src/commercial/BillingPage.tsx:222-226 ───
[maintainability · low] 余额卡错误分支三处问题:① effect 处注释写「失败静默降级(卡片隐藏,不阻塞账单页)」,实际失败时渲染了错误卡片——注释与行为矛盾;② 直出
loadCommercialAccount 捕获的原始 error.message(可能是 'Commercial request failed'/'Internal Server Error'
等英文机器文案),与本 MR CheckoutPage 刚建立的闭合中文文案口径(spec L210)不一致;③ 以魔法字符串 'Loading…' 作哨兵区分加载/失败态,脆弱易碎。另外本分支
success→error→null 是两层嵌套三元,违反检查表禁令(本 MR D15-e 刚在 CheckoutPage 移除同类写法)。建议:文案映射为固定中文或真正静默隐藏,用显式
status: 'loading' 态替代字符串哨兵,渲染分支提取为函数。



─── apps/web/src/commercial/BillingPage.tsx:124-126 ───
[bug · low] loadCommercialAccount 在 signal.aborted 时 rethrow,而此处 void ...then(...) 无
.catch():切换租户(switchScope 会 abort 旧信号)时,在途 account 请求以 AbortError reject,产生 unhandled promise
rejection(控制台噪音,可能触发全局错误上报)。summary/usage 两个既有 loader 是同款问题,此处新增了第三处——建议调用点补 .catch(() => {})(或
loader 不 rethrow,交由 active 守卫吞掉),并顺带修复前两处。

      void loadCommercialAccount(client.commercial, scope.signal).then((next) => {
        if (active && scopeController.isCurrent(scope.scope)) setAccountState(next);
-     });
+     }).catch(() => { /* aborted on scope switch — dropped by the active guard */ });


─── apps/web/src/commercial/CheckoutPage.tsx:237-240 ───
[bug · high] 深链错误分流误伤购买创建失败:此处以 orderIdRef.current 是否非空区分「深链读单失败」与「购买创建失败」,但下单成功后 orderIdRef.current
= order.id 已被写入。此后 purchaseStatus 后台补充 setState 虽被 prev.status === 'ready' 守卫,但后续重试(retryToken+1)会在
orderIdRef.current 非空的情况下重新走深链分支——这本身合理;然而当用户点击「重试」后 getOrder 抛出非 quote
级错误(如网络抖动/500)时,展示的是「订单加载失败,请稍后重试」,这是正确的。真正的问题在于:重启后(restartCheckout 已清空 orderIdRef)首轮 purchase()
若因网络层错误失败(非 ApiError,如 TypeError from fetch),orderIdRef.current 仍为 null,会落入 purchaseErrorText
的「网络异常,请稍后重试」——正确;但若 purchase() 成功返回且 active 为 true 而 scopeController.isCurrent 为
false(租户切换的精确竞态窗口),orderIdRef.current 未被写入,用户切回原租户后重试会重新发起
purchase——服务端幂等保证不产生第二单,可接受。综上该分流逻辑成立,但存在一个真实缺陷:isQuoteLevelConflict(error) 清空 quoteRef.current 后,若
error 同时使 orderIdRef.current 非空(重启前的在途错误迟到命中 catch),旧 quoteRef 被清空会导致下一次重试走新报价,与服务端幂等语义(同 quote
重试同单)冲突,可能创建第二张订单。

-         if (orderIdRef.current) {
+         // 仅当本次 catch 的错误确实来自 getOrder(深链分支)时才按「订单加载失败」分流;
+         // 购买创建失败必须继续走 purchaseErrorText,且 quoteRef 清空只对 purchase 阶段的
+         // 报价级冲突生效。
+         if (orderIdRef.current && thisErrorCameFromGetOrder) {
            setState({ status: 'error', message: '订单加载失败，请稍后重试' });
            return;
          }


─── apps/web/src/commercial/BillingPage.tsx:205-206 ───
[maintainability · low] 批次行 key 使用 `${batch.source}-${batch.period}-${index}`:同 source+period
的两行(如两次月度发放落在同一 period 字符串)仅靠 index 区分,而 React 的 index-key 在列表重排时会错位。当前批次表无重排交互,但 key 未包含任何唯一标识(如
granted_at),若后端同一 period 补发两批,行身份不稳定。建议纳入 granted_at 或后端提供的稳定 id。

-                 {accountState.credits.batches.map((batch, index) => (
-                   <tr key={`${batch.source}-${batch.period}-${index}`}
+                 {accountState.credits.batches.map((batch) => (
+                   <tr key={`${batch.source}-${batch.period}-${batch.granted_at}`}


─── apps/web/src/commercial/BillingPage.tsx:73-76 ───
[bug · medium] batchExpiringWithin 把已过期批次也标记为「近到期」:exp - now <= 30 天对 exp < now(负值)恒真。余额分解中已过期批次理论上
balance_micro 应为 0 或被过滤,但若后端投影短暂保留过期行(投影收敛窗口),这些行会带上近到期标记而非显示已过期。建议加下界:exp >= now && exp - now <=
30d,或对已过期行给独立标识。

  export function batchExpiringWithin(expiresAt: string, now: Date = new Date()): boolean {
    const exp = new Date(expiresAt).getTime();
-   return Number.isFinite(exp) && exp - now.getTime() <= 30 * 24 * 3600 * 1000;
+   const delta = exp - now.getTime();
+   return Number.isFinite(exp) && delta >= 0 && delta <= 30 * 24 * 3600 * 1000;
  }


─── docs/plans/issue-72-flow-evidence-74/verify_ac_assertions.py:68-71 ───
[maintainability · low] 检查名称 "AC1 invoice open/pending/numberless" 未随本次断言放宽同步更新：该断言现在还接受
failed/closed 终局形态，但标签仍只描述 open/pending 形态。同一次改动中 if 分支的标签已从 "still incomplete across window" 更名为
"gate held across window" 以匹配放宽后的语义，此处属同类遗漏。该名称是校验脚本 PASS/FAIL 输出行与失败汇总列表中的锚点（ocr-3 副本 docstring
也明确要求后续维护以 check 名称而非行号为锚点），失配会在证据 FAIL 时误导审计者只去检查 open/pending 形态。建议更名以覆盖两种形态，例如 "AC1 invoice
shape: open/pending/numberless or unpaid-terminal endgame"。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice shape: open/pending/numberless or unpaid-terminal endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py:71-74 ───
[maintainability · low] 检查名称 "AC1 invoice open/pending/numberless" 未随本次断言放宽同步更新：该断言现在还接受
failed/closed 终局形态，但标签仍只描述 open/pending 形态。同一次改动中 if 分支的标签已从 "still incomplete across window" 更名为
"gate held across window" 以匹配放宽后的语义，此处属同类遗漏。该名称是校验脚本 PASS/FAIL 输出行与失败汇总列表中的锚点（ocr-3 副本 docstring
也明确要求后续维护以 check 名称而非行号为锚点），失配会在证据 FAIL 时误导审计者只去检查 open/pending 形态。建议更名以覆盖两种形态。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice shape: open/pending/numberless or unpaid-terminal endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:73-76 ───
[maintainability · low] 检查名称 "AC1 invoice open/pending/numberless" 未随本次断言放宽同步更新：该断言现在还接受
failed/closed 终局形态，但标签仍只描述 open/pending 形态。同一次改动中 if 分支的标签已从 "still incomplete across window" 更名为
"gate held across window" 以匹配放宽后的语义，此处属同类遗漏。该名称是校验脚本 PASS/FAIL 输出行与失败汇总列表中的锚点（ocr-3 副本 docstring
也明确要求后续维护以 check 名称而非行号为锚点），失配会在证据 FAIL 时误导审计者只去检查 open/pending 形态。建议更名以覆盖两种形态。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice shape: open/pending/numberless or unpaid-terminal endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:82-85 ───
[maintainability · low] 检查名称 "AC1 invoice open/pending/numberless" 未随本次断言放宽同步更新：该断言现在还接受
failed/closed 终局形态，但标签仍只描述 open/pending 形态。同一次改动中 if 分支的标签已从 "still incomplete across window" 更名为
"gate held across window" 以匹配放宽后的语义，此处属同类遗漏。该名称是校验脚本 PASS/FAIL 输出行与失败汇总列表中的锚点（本文件 docstring
也明确要求后续副本以 check 名称而非行号为锚点），失配会在证据 FAIL 时误导审计者只去检查 open/pending 形态。建议更名以覆盖两种形态。

-     check("AC1 invoice open/pending/numberless",
+     check("AC1 invoice shape: open/pending/numberless or unpaid-terminal endgame",
            (o.get("invoice_status") == "open" and o.get("invoice_payment_status") == "pending"
             and not o.get("invoice_number"))
            or o.get("invoice_status") in ("failed", "closed"))


─── docs/plans/issue-72-flow-evidence-82/r4-flow/seed.sh:13-14 ───
[bug · medium] EV 在已经 `cd` 到 worktree 根之后才基于 `$0` 重新解析，而 `$0` 可能是相对路径：按 README「脚本（本目录，可重放）」的指引从
r4-flow 目录内以 `./seed.sh` 复跑时，第一次 `cd "$(dirname "$0")/../../../.."` 在原 cwd 下正确落到仓库根，但随后 `$(cd
"$(dirname "$0")" && pwd)` 中的 `.` 已相对新 cwd（仓库根）解析，EV 会错误地等于 worktree 根——所有证据落盘（`: >
"$EV/seed-run.txt"`、seed-login-a/b.json、api-01/02/03-*.json）会静默写到仓库根而非本轮目录；若从其它目录以相对路径调用，第二次 cd
则直接报错中止。建议在离开原始 cwd 之前先解析 EV，再用 `$EV` 拼接定位 worktree 根。

- cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
- EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script (per-round dir)
+ EV="$(cd "$(dirname "$0")" && pwd)"   # resolve the evidence dir BEFORE leaving the invocation cwd
+ cd "$EV/../../../.."                  # worktree root (docs/plans/<dir>/<round> -> root)


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:5-6 ───
[security · medium] 头注释宣称本脚本沿用 r4-flow3 的 "sqlite3 ?1 parameter binding" 加固约定，但事实不符：r4-flow3 及本脚本（第
103-104 行）均是把 '$UID_B' 字符串内插进 SQL 语句，实际防护是前置的 uuid_shape 白名单（r4-flow3 在 sqlite3 行内注释明确说明 "The
shell's sqlite3 CLI has NO usable parameter binding for statement
values"）。注释断言了一个代码并不具备的安全属性，违反分支红线"数据库查询一律使用参数绑定"的口径，后续复制该片段时容易误以为已有绑定保护而丢弃白名单。建议：要么把注释改为如实描述"UUID
白名单门控的内插（CLI 无值级参数绑定）"，要么用 heredoc + `.parameter set @uid ...` 实现真正的参数绑定。

- # payloads with the password on STDIN, REDACTED login captures, sqlite3
- # ?1 parameter binding, asserted registrations and tenant serials.
+ # payloads with the password kept off curl argv, REDACTED login captures,
+ # UUID-whitelist-gated sqlite3 interpolation (the CLI has no value binding),
+ # asserted registrations and tenant serials.


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_04_active_face.mjs:12-12 ───
[bug · medium] 缺少与腿 02/03 一致的 usage 快速失败守卫。argv[2] 缺失时 `?? ''` 使 URL 变为 `checkout?order=`，前端
router（router.tsx:690 `get('order')?.trim() ?? ''`）会把空串视同无参，CheckoutPage 仅在 orderIdRef
非空时走深链读单，空串则落入自动建单/自建 quote 路径（GeneralPreferencesPanel.tsx:253 注释："orderId 空串 = checkout 页自建
quote+order"）——恰好是脚本自述 "an active purchase must never auto-submits a new one" 所要排除的面；同时 `if (ORDER)`
会静默跳过 A-12 订单身份锚点断言，漏判意外重复下单。

- const ORDER = process.argv[2] ?? '';
+ const ORDER = process.argv[2];
+ if (!ORDER) { console.error('usage: node browser_04_active_face.mjs <orderId>'); process.exit(2); }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:8-9 ───
[documentation · low] 头注释称 "fresh db: pad 0 is enough for an isolated db"，但脚本内 `[ "$PAD" -ge 1 ]`
守卫直接拒绝 PAD=0，注释与行为矛盾；按注释指引以 FLOW82_PAD_COUNT=0 启动会在预检处失败。若 0 确为合法值（隔离库），应放宽守卫为 `>= 0`；否则修正注释为 "PAD
>= 1 强制（占位符至少 1 个）"。

- # Registers FLOW82_PAD_COUNT placeholders (fresh db: pad 0 is enough for an
- # isolated db — the default keeps the r4 shape) then the two protagonists
+ # Registers FLOW82_PAD_COUNT (>= 1, enforced below) placeholders then the
+ # two protagonists


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:18-18 ───
[maintainability · low] BACKEND 地址、Lago 端口 48889 及容器名 weknora-lago-82r5-db-1 均硬编码且无环境变量覆盖入口；同组的 mjs
腿尚有 `FLOW82_WEB` 兜底，而 seed.sh 换端口/容器复跑必须改源码。建议至少为 BACKEND、Lago API 地址与容器名提供 `${VAR:-默认值}` 形式的覆盖入口。

- BACKEND=http://127.0.0.1:8093
+ BACKEND="${FLOW82_BACKEND:-http://127.0.0.1:8093}"


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:34-34 ───
[security · low] 口令经 `jq --arg p "$PW"` 进入 jq 子进程的命令行参数，运行窗口内可被本机 ps 观测，与头注释 "password on STDIN"
的加固口径不符（STDIN 仅覆盖了 curl 的 --data @- 一侧）。可改用 jq 内建 `env` 对象从环境读取，使口令完全不经过任何子进程 argv。

-   jq -nc --arg e "$1" --arg p "$PW" '{username:$e,email:$e,password:$p}' \
+   jq -nc --arg e "$1" '{username:$e,email:$e,password:env.FLOW82_R5_PW}' \


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:16-17 ───
[bug · low] `EV` 在第一处 `cd` 到仓库根之后才对相对形态的 `$0` 取 dirname：若操作者从非仓库根目录以相对路径调用本脚本，cd 后原相对 `$0`
已失效，命令替换失败在 set -e 下直接中断脚本。应先求值 EV（或用 `$PWD` 缓存原始目录）再执行 cd。

- cd "$(dirname "$0")/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)
  EV="$(cd "$(dirname "$0")" && pwd)"   # evidence lands beside this script
+ SCRIPT_DIR="$(dirname "$0")"
+ cd "$SCRIPT_DIR/../../../.."   # worktree root (docs/plans/<dir>/<round> -> root)


─── docs/plans/issue-72-flow-evidence-82/r4-flow2/api-03-lago-plans.json:1-1 ───
[other · high] 证据完整性问题：本文件与 r4-flow（round 1）的同名文件逐字相同（Pro 的
lago_id=06c3afdd-…、created_at=2026-09-25T03:07:32Z、meta.total_count=2），而本目录 api-02-publish.json 的
published_at=2026-09-26T16:50:10Z 晚于 round 1 的 15:46:12Z（两次独立运行）。Pro plan 的 created_at 早于本轮 publish
一天多，说明本轮 publish 命中的是确定性 code weknora-pro-v1 的既有 plan（更新而非新建），Lago 中 9900 计数不会增长——按此证据，seed.sh 的
A-09 断言 `[ "$N" -gt "$BASE_N" ]` 必然落入 FAIL 分支，本轮 seed 不可能输出 SEED OK。同时 seed 写出的
api-03-lago-plans-baseline.json（BASE_N 的唯一稽核依据，.gitignore 并不忽略它，r5b-flowcheck 就提交了自己的
baseline）未随目录提交。这份证据链无法自洽支撑"本轮 publish 到达 Lago"，建议在重置后的 Lago 栈上重跑并提交本轮真实产物 + baseline，或改用本轮专属
plan_key（如 pro-r4f2）让计数归属可判定。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:144-145 ───
[bug · medium] BASE_N/N 缺少数值形状校验，两种失败形状都会破坏脚本自己立的 fail-honestly 姿态：(1) baseline curl
失败（连接拒绝等）落盘空文件时，jq 对空输入输出空且 exit 0，BASE_N=""，随后 `[ "$N" -gt "$BASE_N" ]` 报 "integer expression
expected" 后落入 elif 分支，产生误导性 FAIL 文案（"count unchanged at <空>"——实际是 baseline 读取失败而非计数未变）；(2) 落盘的是 Lago
错误 JSON（如 LAGO_KEY 失效的 401 体）时，`jq '[.plans[]…]'` 以 "Cannot iterate over null" exit 5，命令替换使 set -e
直接中止且没有任何 FAIL note。建议沿用脚本对 PAD 已有的模式：`case "$BASE_N" in ''|*[!0-9]*) say "FAIL: baseline read not a
count: '$BASE_N'"; exit 1;; esac`（N 同理）。



─── docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh:131-131 ───
[test · low] grant 落库结果只打印不断言，且 sqlite3 失败会被吞掉：count 查询位于 say 的 `$(…)` 命令替换内，而 say 管道（echo |
tee）的退出状态由 tee 决定，set -e 看不到 sqlite3 的失败——发生 database is locked 等错误时输出 "granted:  row(s)" 后脚本继续，后续
browser 腿将以 403 失败且难以归因。这与 (A-13) 自己立的原则（"responses are ASSERTED, never merely
printed"）不一致。建议显式断言：`GRANT_N=$(sqlite3 "$DB" "select count(*) …")` 后 `[ "$GRANT_N" -ge 1 ] || { say
"FAIL: …"; exit 1; }`。



─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_04_active_face.mjs:13-16 ───
[bug · low] 该腿的 env 守卫要求 FLOW82_EXPECT_CNY，但脚本体内从未读取该变量（报价断言只存在于 leg 01）。当 operator 单独重跑 leg 04（例如
settle 延迟后补截图）而 shell 中未导出 FLOW82_EXPECT_CNY 时，脚本会以 exit 2 直接失败，且该变量本不为本腿所需。疑为从
browser_01_checkout.mjs 复制后未裁剪，建议从校验中移除该变量。

- if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW || !process.env.FLOW82_EXPECT_CNY) {
-   console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW / FLOW82_EXPECT_CNY');
+ if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW) {
+   console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW');
    process.exit(2);
  }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_01_checkout.mjs:28-28 ───
[maintainability · low] catch 块为空且注释 '/* captured below */' 与实际代码不符——下方并不存在任何重新捕获或兜底逻辑。若 purchases
请求体 JSON.parse 失败（或 provider 字段缺席），purchaseProvider 会静默保持空串，checkout-submit-provider 断言只会显示
provider=""，取证时无法区分「请求未发出」与「请求体解析失败」。建议在 catch 中至少输出 warn 便于排查。

-       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; } catch { /* captured below */ }
+       try { purchaseProvider = JSON.parse(req.postData() ?? '{}').provider ?? ''; } catch (err) { console.warn('purchases request body parse failed:', String(err)); }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_02_sync_face.mjs:13-16 ───
[maintainability · low] 四个腿逐字重复同一段样板：手工 env 校验 + WEB/EV 常量 + newContext/setDefaultTimeout/login
前奏。共享库 ../_browser_lib.mjs 已为此导出 envRequired（browser_flow_82.mjs 等顶层脚本已在用，且本 issue 早期 OCR r1 记录
docs/plans/issue-72-ocr-issue-82-r1.md L779 也建议过同样收口）；WEB 回退地址 'http://localhost:5194' 与 45000
等魔法超时同样分散在各腿。建议改用 envRequired（需同步在 import 中加入），并考虑把 WEB 回退与常用超时收口进共享库，避免后续调整端口/节奏时多点漂移。

- if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW) {
-   console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW');
-   process.exit(2);
- }
+ import { login, note, runLeg, evidencePath, assertOrderIdentity, envRequired } from '../_browser_lib.mjs';
+ // ...
+ const { FLOW82_EMAIL_A: EMAIL, FLOW82_R5_PW: PASSWORD } =
+   envRequired('FLOW82_EMAIL_A', 'FLOW82_R5_PW');


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/seed.sh:89-90 ───
[security · medium] seed.sh 向 commercial_grants 的 insert 及后续 select count(*) 均用字符串拼接组装 SQL（'$UID_B'
直接内插）。虽然 L84-85 的 uuid_shape 白名单先于拼接执行、当前封住了注入路径，但这直接违反本迁移分支的安全验收条件第 2 条（数据库查询一律参数绑定，禁止拼接组装
SQL），且注释自称 "injection gate" 表明该写法是明知的拼接——后续维护者一旦放宽白名单（例如接受其他 id 形态）即构成注入入口。建议改用 python3 sqlite3（或 Go
辅助）做参数绑定执行：

- sqlite3 "$DB" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-r5b', 1);"
- say "granted: $(sqlite3 "$DB" "select count(*) from commercial_grants where capability='plan_publish' and user_id='$UID_B';") row(s)"
+ python3 - "$DB" "$UID_B" <<'PY'
+ import sqlite3, sys
+ db, uid = sys.argv[1], sys.argv[2]
+ with sqlite3.connect(db) as c:
+     c.execute("insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, ?, 'plan_publish', 'flow-verifier-r5b', 1)", (uid,))
+     n = c.execute("select count(*) from commercial_grants where capability='plan_publish' and user_id=?", (uid,)).fetchone()[0]
+ say() { echo "$@" | tee -a "$EV/seed-run.txt"; }
+ say "granted: ${n} row(s)"
+ PY


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_03_restart.sh:29-34 ───
[bug · medium] 重启链路存在假阳性风险：健康轮询循环结束后没有对 code=200 做任何断言。本脚本头部只校验了 WEKNORA_COMMERCIAL_PLATFORM_API_KEY
与 WEKNORA_COMMERCIAL_STRIPE_API_KEY，而被 nohup 调用的 start_backend_8093.sh 还以 :? 必需
FLOW82_KEY_DIR（L18）——缺失时启动器立即静默失败，40 次轮询（约 200 秒）全部落空后脚本仍继续输出 "after" 计数；此时读取的是未被触动过的旧 DB（订单仍
fulfilled、activations=1），输出形似 "重启后无二次发放" 的 PASS 证据，实为后端根本没起来。建议轮询后强制断言健康状态，并在头部补齐 FLOW82_KEY_DIR
的存在性校验（与 start_backend_8093.sh 的必需集合对齐）：

- for i in $(seq 1 40); do
-   code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8093/health 2>/dev/null)
-   [ "$code" = "200" ] && break
-   sleep 5
- done
+ : "${FLOW82_KEY_DIR:?caller must export FLOW82_KEY_DIR=/tmp/issue82-r5-keys}"
+ # ... 轮询循环后：
  echo "health=$code after restart" | tee -a "$EV/replay-03-restart.txt"
+ [ "$code" = "200" ] || { echo "FAIL: backend did not come back (health=$code) — after-counts NOT trustworthy" | tee -a "$EV/replay-03-restart.txt"; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_03_restart.sh:20-22 ───
[bug · low] kill 段有两个边界缺陷：(1) 端口 8093 无监听时 lsof -t 返回空且退出码非零，PID=$(...) 赋值在 set -euo pipefail（含管道子
shell）下直接中止脚本，预设的 FAIL 提示不会打印，replay-03-restart.txt 只留半截记录；(2) DB=data/issue82-r5b-verify.db 在此硬编码，而
seed.sh 通过 DB_PATH 环境变量注入同名路径，两处独立维护存在漂移风险——且 Q_ORD 绑定的订单 ID（ord_86c00340e158d726）是本轮专属身份，重新跑
seed（fresh sqlite）后该查询会落空，sqlite 返回空/0 会被误读为"重启后无变化"。建议对空 PID 显式报错，并让 DB 路径复用与 seed.sh 相同的 DB_PATH
注入（带本轮默认值）：

-   PID=$(lsof -nP -iTCP:8093 -sTCP:LISTEN -t)
+   PID=$(lsof -nP -iTCP:8093 -sTCP:LISTEN -t) || true
+   if [ -z "$PID" ]; then
+     echo "FAIL: no listener on 8093 before kill — restart leg invalid" | tee -a /dev/stderr
+     exit 1
+   fi
    echo "killing backend pid=$PID"
    kill "$PID"
+ # 头部：DB="${DB_PATH:-data/issue82-r5b-verify.db}"（与 seed.sh 的注入方式对齐）


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_02_webhook.sh:13-13 ───
[bug · low] LAGO_KEY 从 docker exec 读取后未做非空校验（seed.sh L93-94 同款取值后有 [ -n "$LAGO_KEY" ] 门禁）。若 api_keys
表为空或容器异常，后续三个 curl 携带空 Bearer 得到 401 错误体，jq 提取 .subscriptions[0].status 等字段输出 null/null，被原样写入
replay-02-webhook.txt 证据文件——表面上"四对象重读"完成，实则一个权威对象都没读到，可能被误读为投影为空/幂等通过。建议补齐与 seed.sh 一致的非空门禁：

  LAGO_KEY=$(docker exec weknora-lago-82r5-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]')
+ [ -n "$LAGO_KEY" ] || { echo "FAIL: no lago api key readable from weknora-lago-82r5-db-1" >&2; exit 1; }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/diag_checkout.mjs:11-11 ───
[maintainability · low] 诊断脚本缺少同目录兄弟腿的既定健壮性惯例：browser_01_checkout.mjs L12-15 对 FLOW82_EMAIL_A /
FLOW82_R5_PW 做了显式存在性校验（缺失时 exit 2 并给出明确指引），公共库 _browser_lib.mjs 也提供 runLeg 把任何抛错收敛为结构化失败输出；本脚本在未校验
env 的情况下直接把 process.env.FLOW82_EMAIL_A / FLOW82_R5_PW 传入 login，缺失时会在 waitForURL 处以不相关的 30
秒超时失败，整段流程无 try/catch，导航异常产生未处理的 Promise 拒绝。waitForTimeout(8000) 固定等待也使诊断结果随机器负载抖动。建议至少补 env 校验并用
try/finally 包裹（与兄弟脚本纪律一致）：

+ if (!process.env.FLOW82_EMAIL_A || !process.env.FLOW82_R5_PW) {
+   console.error('missing required env: FLOW82_EMAIL_A / FLOW82_R5_PW');
+   process.exit(2);
+ }
+ try {
- await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);
+   await login(page, WEB, process.env.FLOW82_EMAIL_A, process.env.FLOW82_R5_PW);
+   await page.goto(`${WEB}/platform/billing/checkout`);
+   // ... 诊断主体
+ } catch (err) {
+   console.error('[diag] failed:', err?.message ?? err);
+   process.exitCode = 1;
+ } finally {
+   await browser.close().catch(() => {});
+ }


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/diag_checkout.mjs:16-16 ───
[maintainability · low] 截图路径用 new URL('.', import.meta.url).pathname 拼接：Windows 下 pathname 会产生形如
/C:/Users/... 的前导斜杠路径，且目录名中的非 ASCII/空格会被百分号编码导致写入失败。公共库 _browser_lib.mjs 已提供
evidencePath(subdir)（内部走 fileURLToPath，跨平台安全），同目录四个 browser_0x 腿全部在用它——本脚本应复用同一辅助而非自行拼接：

- await page.screenshot({ path: new URL('.', import.meta.url).pathname + 'diag-checkout.png', fullPage: true });
+ import { login, requirePlaywright, evidencePath } from '../_browser_lib.mjs';
+ // ...
+ await page.screenshot({ path: evidencePath('r5b-flowcheck') + 'diag-checkout.png', fullPage: true });


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:79-81 ───
[bug · medium] 内嵌 Python 的 gql() 只保证 HTTP 200,未检查 GraphQL 层的 errors / data 为 None:凭据失效、Lago 未就绪或
schema 漂移时,`login["data"]["loginUser"]["token"]` 与 `(existing["data"]["paymentProviders"] or {})`
会直接抛 KeyError/TypeError,真实的 GraphQL 错误信息被吞掉,只剩不可读的 traceback;且此时 provider 可能已注册而 LAGO_INTEGRATION_*
尚未导出,环境停留在半初始化状态。同一段 heredoc 里 add 变异已用 `(add.get("data") or {})` + 失败打印的防御写法,t10/t11 lab 的 clients
层也有 LoginError 分类先例,建议对 login 与 existing 两处读数补同样的 errors/data 守卫,失败时打印 errors 摘要并 sys.exit(1)。

  login = gql("mutation($e:String!,$p:String!){loginUser(input:{email:$e,password:$p}){token}}",
              {"e": email, "p": password})
+ if login.get("errors") or not ((login.get("data") or {}).get("loginUser") or {}).get("token"):
+     print("graphql login failed:", json.dumps(login.get("errors"))[:300]); sys.exit(1)
  token = login["data"]["loginUser"]["token"]


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/deliver_stripe_webhook.py:90-96 ───
[bug · low] --org 兜底分支的 subprocess.run 未检查 returncode,且 out.stderr 已捕获却从未输出:db 容器未就绪、compose
项目不匹配或守护进程故障时,psql 失败导致 stdout 为空,最终报错固定为 "organization id unavailable",掩盖真实根因。建议失败时把 returncode 与
stderr 尾部并入错误信息,便于取证环境定位。

          out = subprocess.run(
              ["docker", "compose", "-f", "deploy/lago/compose.yaml",
               "--env-file", "deploy/lago-lab/payment-settle-trigger/lab.env",
               "-p", "weknora-lago-t11", "exec", "-T", "db", "psql", "-U", "lago",
               "-tAc", "select id from organizations order by created_at limit 1"],
              capture_output=True, text=True, timeout=30)
+         if out.returncode != 0:
+             raise SystemExit(f"org id psql failed ({out.returncode}): {(out.stderr or '').strip()[:200]}")
          args.org = (out.stdout or "").strip()


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:155-157 ───
[security · low] 以 `-e WSECRET="$SECRET"` 形式传递 webhook secret,该值会出现在 docker CLI 进程的 argv 中,对本机 ps
可见(执行窗口内),与脚本自身声明的凭据最小暴露纪律(不落盘、不外显)不完全一致。改用无值透传形式 `-e WSECRET` 并把变量放进 docker CLI 自身环境,值经 environ 而非
argv 传递,即可消除 ps 暴露面;仅实验室环境,风险轻微。

-   docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T \
-     -e WSECRET="$SECRET" api bin/rails runner \
+   env WSECRET="$SECRET" docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T \
+     -e WSECRET api bin/rails runner \
      "PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').update!(webhook_secret: ENV['WSECRET'])" >/dev/null


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_04_active_face.mjs:12-12 ───
[test · medium] ORDER 参数静默降级：legs 02/03 缺参时以 usage 提示 exit 2，本腿却用 `?? ''` 兜底。当 operator 忘传 <orderId>
时，后续 `goto(...checkout?order=${ORDER})` 的空参数对 CheckoutPage 是 falsy orderId，会走自动发起新购买的分支（CheckoutPage
run() 中 orderIdRef 为空即 quote+purchase）——恰好绕过本腿要验证的『active purchase 深链绝不自动开新单』守卫，且可能在取证环境里产生一张多余订单；同时
`if (ORDER)` 使 A-12 订单身份断言被静默跳过，腿仍可退出 0，取证完整性受损。建议与 legs 02/03 对齐：缺参即报 usage 并 exit 2，并去掉 `if
(ORDER)` 条件使身份断言恒执行。

- const ORDER = process.argv[2] ?? '';
+ const ORDER = process.argv[2];
+ if (!ORDER) { console.error('usage: node browser_04_active_face.mjs <orderId>'); process.exit(2); }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/browser_04_active_face.mjs:27-27 ───
[bug · medium] 该 `detached` 等待不具备注释声称的 "Deterministic convergence" 能力:BillingPage.tsx
只在挂载/reloadToken 变化时拉取一次数据(仅 useEffect + 手动 Reload 按钮,无 setInterval/轮询,与 CheckoutPage 的 3 秒轮询不同)。若腿
04 启动时投影尚未翻转(settle/webhook 竞态),首屏渲染「已付款待激活」后 DOM 永远不会自行更新,此等待必然烧满 180 秒后以 leg-error
超时失败,而不是等待收敛。建议在等待期间周期性驱动页面自身的 Reload 按钮(或 page.reload()),使投影翻转后等待真正能收敛。

-   await page.waitForSelector('text=已付款待激活', { state: 'detached', timeout: 180000 });
+   // BillingPage 无自动轮询(仅挂载/手动 Reload 拉取):等待期间需驱动
+   // 页面自身 Reload,投影翻转后 detached 才能真正收敛。
+   const deadline = Date.now() + 180_000;
+   while ((await page.locator('text=已付款待激活').count()) > 0) {
+     if (Date.now() > deadline) throw new Error('billing paid copy never detached within 180s');
+     await page.getByRole('button', { name: 'Reload' }).click();
+     await page.waitForTimeout(3000);
+   }


─── docs/plans/issue-72-flow-evidence-82/r5-verify/seed.sh:124-124 ───
[security · low] publish 断言把 $PLAN_KEY/$DRAFT_V 直接内插进 jq 程序字符串:值中含引号/反斜杠时会破坏表达式甚至注入 jq 代码(与同脚本上方
draft 断言 `jq -e --arg k "$PLAN_KEY"` 的参数化风格不一致)。PLAN_KEY 来自环境可被任意覆盖,建议统一改用 --arg 传参,消除程序字符串拼接。

- jq -e ".data.receipt.command_key == \"publish_plan_version:$PLAN_KEY:$DRAFT_V\"" "$EV/api-02-publish.json" >/dev/null \
+ jq -e --arg ck "publish_plan_version:$PLAN_KEY:$DRAFT_V" '.data.receipt.command_key == $ck' "$EV/api-02-publish.json" >/dev/null \


─── deploy/lago-lab/payment-activation/phases.py:1286-1289 ───
[bug · medium] 新增的 deferred 漂移复查把三处读取的 HTTP 状态全部丢弃(`_`):`LagoRestClient.request` 对非 2xx 不抛错(仅传输失败抛
OSError),`_invoices_for`/`_payments_for` 在 body 非 dict 或缺少键时返回空列表、`_subscription_show` 的非 2xx body
解析为空 dict。于是一次瞬时 5xx/4xx 会让
`subscription_status=None`、`invoice_count=0`、`payments_succeeded_count=0`,`deferred_no_drift=False`,
把一次 API 读取故障误判为『minutes-late drift』并记为 AC3 FAIL(fail = pinned runtime 违反契约的证据口径被污染)。这与已确认的 gate
阶段忽略状态问题方向相反:那边是断言空转通过,这里是误告失败。建议:active 状态查询的 404 仍算真实漂移,其余非 2xx 应归类 blocked-env 或至少不计入漂移证据。

-             _, sub_body3 = _subscription_show(ctx, sub_ext, "active")
+             ss3, sub_body3 = _subscription_show(ctx, sub_ext, "active")
+             is3, invoices3 = _invoices_for(ctx, customer_b["external_id"])
+             ps3, payments3 = _payments_for(ctx, customer_b["external_id"])
+             if ss3 not in (200, 404) or not _ok(is3) or not _ok(ps3):
+                 observed["deferred_read_status"] = {
+                     "subscription": ss3, "invoices": is3, "payments": ps3}
+                 return _blocked(ctx, "retries",
+                                 "deferred re-check reads failed (non-2xx)",
+                                 expected)
              sub3 = (sub_body3 or {}).get("subscription", {}) if isinstance(sub_body3, dict) else {}
-             _, invoices3 = _invoices_for(ctx, customer_b["external_id"])
-             _, payments3 = _payments_for(ctx, customer_b["external_id"])


─── deploy/lago-lab/payment-settle-trigger/run_lab.py:30-31 ───
[maintainability · low] 死代码:`import subprocess` 在 run_lab.py 中没有任何使用;同文件顶部的 `COMPOSE_FILE = REPO_DIR
/ "deploy" / "lago" / "compose.yaml"` 同样无引用(phases.py 自带 `_COMPOSE_FILE`);`scan_and_scrub` 里的
`hits_total` 只累加、从不被读取(返回字典只含 hits_after_scrub)。建议一并删除。

- import subprocess
  import sys


─── deploy/lago-lab/payment-settle-trigger/phases.py:728-732 ───
[maintainability · low] P-D 环境缺口(webhook_secret/org_id 从 lab DB 读取失败)时直接 `return _blocked(...)`,而
`_blocked` 会用 `{"blocked_reason": ...}` 整体替换 observed——此前已采集的 P-B/P-C
观测(settle_pm_attached、pi_update_status、pi_confirm_status、pi_status_after_confirm)从落盘的 phase
报告中丢失。P-C 的 PI update+confirm→succeeded 正是本探针存在理由(The UNVERIFIED LINK of D2' step iv),在 P-D
被环境阻塞时应保留其证据而不是抹掉。建议把已观测内容并入 blocked 报告。

          if not secret or not org_id:
-             return _blocked(ctx, "settle_trigger",
-                             "webhook_secret/organization id not readable from the "
-                             "lab DB (docker compose exec db psql unavailable)",
-                             expected)
+             observed["blocked_reason"] = (
+                 "webhook_secret/organization id not readable from the lab DB "
+                 "(docker compose exec db psql unavailable)")
+             return _report(ctx, "settle_trigger", expected, observed, BLOCKED)


─── docs/plans/issue-72-flow-evidence-84/seed_84.sh:67-67 ───
[security · medium] 外部输入 $UID_B 仍以字符串拼接方式进入三条 sqlite3 语句(两条 insert or replace + 一条 select count
回读),前置 uuid_shape 白名单只是缓解,不满足项目「数据库查询一律参数绑定」的红线。该模式已在 82 轮(security·medium)与 83 轮(security)OCR
中两次报告,本新文件第三次复制且插值点从 2 处增至 3 处——校验与拼接在代码上分离,后续新增使用未校验变量(如 TENANT_*)的 sqlite3 语句时保护即失效。sqlite3 CLI
不支持绑定参数,建议改用 python3 的 sqlite3 模块原生 ? 绑定(uuid_shape 保留为纵深防御)。

- sqlite3 "$DB_PATH" "insert or replace into commercial_grants (tenant_id, user_id, capability, granted_by, version) values (0, '$UID_B', 'plan_publish', 'flow-verifier-84', 1);"
+ python3 - "$DB_PATH" "$UID_B" <<'PY'
+ import sqlite3, sys
+ conn = sqlite3.connect(sys.argv[1])
+ for cap in ("plan_publish", "refund_review"):
+     conn.execute(
+         "insert or replace into commercial_grants "
+         "(tenant_id, user_id, capability, granted_by, version) "
+         "values (0, ?, ?, 'flow-verifier-84', 1)", (sys.argv[2], cap))
+ n = conn.execute("select count(*) from commercial_grants where user_id=?",
+                  (sys.argv[2],)).fetchone()[0]
+ print(f"granted: {n} row(s) for B")
+ conn.commit()
+ PY


─── docs/plans/issue-72-flow-evidence-84/seed_84.sh:88-88 ───
[bug · low] 读取 Lago API key 的 `select value from api_keys limit 1` 无 ORDER BY:82r5 栈 DB 存在多把 key
时命中哪一行不确定,若取到已失效/权限不足的 key,后续 Lago 断言将以难以复现的方式间歇性失败。82 轮 r2 OCR 已确认此问题并修复为 `order by created_at desc
limit 1`(r4-flow/r5-verify 的 seed.sh 均带排序;同一 weknora-lago-82r5-db-1 栈的 r5b/replay_02_webhook.sh
也带排序),83/84 轮回归为无排序版本,同栈两种读取口径并存。

- LAGO_KEY=$(docker exec weknora-lago-82r5-db-1 psql -U lago -tAc "select value from api_keys limit 1" | tr -d '[:space:]') \
+ LAGO_KEY=$(docker exec weknora-lago-82r5-db-1 psql -U lago -tAc "select value from api_keys order by created_at desc limit 1" | tr -d '[:space:]') \


─── docs/plans/issue-72-flow-evidence-84/run_84.sh:15-16 ───
[bug · low] 在 set -u 下,默认值内裸用 ${TMPDIR}:TMPDIR 未导出的常见 Linux 环境(cron/CI/容器)脚本直接以 unbound variable
中止;TMPDIR=/tmp(不以 / 结尾)时兜底路径变成 /tmpissue84-keys,落在根目录且通常无写权限。83 轮 r1 OCR 已对同一形态给出修复建议,本新脚本未吸收。本文件的
backend.log/frontend.log 与 up_stubs_84.sh 的两处 stub 日志路径同模式,建议统一改用 ${TMPDIR:-/tmp}/ 前缀。

- export FLOW84_KEYS="${FLOW84_KEYS:-${TMPDIR}issue84-keys}"
- export FLOW84_SECRETS_ENV="${FLOW84_SECRETS_ENV:-${TMPDIR}issue84-secrets.env}"
+ export FLOW84_KEYS="${FLOW84_KEYS:-${TMPDIR:-/tmp}/issue84-keys}"
+ export FLOW84_SECRETS_ENV="${FLOW84_SECRETS_ENV:-${TMPDIR:-/tmp}/issue84-secrets.env}"


─── docs/plans/issue-72-flow-evidence-84/run_84.sh:21-23 ───
[bug · low] 端口预检在 lsof 缺失时静默放行:lsof 未安装的环境下 `lsof -iTCP:...` 以 command-not-found(127)退出,`&&`
短路,预检对已占用端口不再报错,违背「端口预检」步骤的承诺,失败面后移到 stub/backend 启动时才暴露。建议前置 command -v lsof 显式失败,或改用无需外部依赖的探测方式。

+ command -v lsof >/dev/null 2>&1 || { echo "lsof missing — port precheck cannot run"; exit 1; }
  for p in 8096 5197 8298 8299; do
    lsof -iTCP:$p -sTCP:LISTEN >/dev/null 2>&1 && { echo "port $p occupied"; exit 1; }
  done


─── docs/plans/issue-72-flow-evidence-84/up_stubs_84.sh:34-35 ───
[test · low] 两个 smoke 检查只 curl 打印状态码而未断言:curl 对 HTTP 4xx/5xx 默认 exit 0,set -e 不会拦截——stub
进程起来了但鉴权行为回退(如验签逻辑被误改)时本步骤照常通过,失败面后移到四幕执行时才暴露,排障成本变高。run_84.sh Step 2 的 --selftest 仅覆盖 anomaly
副本自身的回调构造,不覆盖 alipay stub,这里是 alipay 面唯一的起栈把关点。建议对状态码做显式比较。

  echo '--- smoke: alipay stub answers precreate-unauth ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8299/gateway.do -d 'service=alipay.trade.precreate'
+ code=$(curl -s -o /dev/null -w '%{http_code}' -X POST 127.0.0.1:8299/gateway.do -d 'service=alipay.trade.precreate')
+ [ "$code" != "000" ] || { echo "FAIL: alipay stub not answering (curl transport failure)"; exit 1; }
+ echo "alipay stub answered $code"


─── docs/plans/issue-72-flow-evidence-84/wechat_native_stub_anomaly.py:226-228 ───
[bug · low] /stub/mark 与 /stub/notify 对请求体直接 json.loads 并以 data["out_trade_no"] 取键:畸形 JSON
或缺键时抛未捕获异常,BaseHTTPRequestHandler 打印 traceback 后直接断开连接,编排侧 curl 收到空应答而非明确的
4xx——这两个是四幕手动编排的高频入口,一次漏字段就表现为「空回复+断连」,与证据链要求的可定位 FAIL 面不符。建议捕获 ValueError/KeyError 回 400。

          if path == "/stub/mark":
+             try:
-             data = json.loads(body.decode("utf-8"))
+                 data = json.loads(body.decode("utf-8"))
-             order = ORDERS.get(data["out_trade_no"])
+                 order = ORDERS.get(data["out_trade_no"])
+             except (ValueError, KeyError):
+                 self._reply(400, {"error": "malformed mark request"})
+                 return


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:30-32 ───
[bug · medium] 在 `set -euo pipefail` 下,这三行纯赋值语句会把命令替换的失败状态作为自身退出状态并直接触发 errexit:`env_value` 里 grep
未命中键时返回 1(pipefail 传播),docker/psql 失败同样如此。结果是脚本在走到下面 `[ -n ... ]` 守卫之前就被杀死——"LAGO_API_URL
missing/empty…"、"organization id unavailable…" 这些提示在它们本来针对的故障场景下永远不可达,且缺键场景连 stderr
都没有任何输出,完全静默退出,违背脚本自己声明的 skip≠pass 诊断纪律。同类的还有后面 `SECRET=$(python3 …)` 之后的 `[ -z "$SECRET" ]`
守卫。建议给每处命令替换追加 `|| true`,让显式守卫真正接管失败路径并输出诊断。

- _t9_base="$(env_value LAGO_API_URL)"
- _t9_orgcred="$(env_value LAGO_ORG_API_KEY)"
- _t9_org="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T db psql -U lago -tAc 'select id from organizations order by created_at limit 1' | tr -d '[:space:]')"
+ _t9_base="$(env_value LAGO_API_URL)" || true
+ _t9_orgcred="$(env_value LAGO_ORG_API_KEY)" || true
+ _t9_org="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T db psql -U lago -tAc 'select id from organizations order by created_at limit 1' | tr -d '[:space:]')" || true


─── docs/plans/issue-72-flow-evidence-82/settle-evidence/prepare_t9_env.sh:112-114 ───
[bug · medium] 读取存量 webhook secret 的这一步用 `2>/dev/null` 丢弃了全部 stderr,又处于 `set -e` 之下:api
容器未就绪、compose 项目不匹配或 rails runner 报错时,命令以非零状态退出,脚本静默死亡,操作者得不到任何线索(连 docker/rails
的原始错误都被吞掉),无法区分"读取失败"与"确实没有存量 secret"。建议保留 stderr 到临时文件,在退出状态非零时回显其尾部再失败,与脚本其余部分的取证诊断风格保持一致。

- docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T api bin/rails runner \
+ _t9_err="$STORED_FILE.err"
+ if ! docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" -p "$PROJECT" exec -T api bin/rails runner \
    "s = PaymentProviders::StripeProvider.where(deleted_at: nil).find_by(code: 'weknora-stripe').try(:webhook_secret).to_s; print '__T9SECRET__' + s" \
-   > "$STORED_FILE" 2>/dev/null
+   > "$STORED_FILE" 2>"$_t9_err"; then
+   { echo "stored webhook-secret read failed:"; tail -n 20 "$_t9_err"; } >&2
+   rm -f "$STORED_FILE" "$_t9_err"
+   return 1 2>/dev/null || exit 1
+ fi


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:133-138 ───
[bug · medium] sqlShape 校验失败路径调用 process.exit(2)，会立即终止进程，绕过脚本末尾的 `for (const [name, content] of
Object.entries(files)) writeFileSync(...)` 循环与 RESULT 汇总——与 (C-04) 注释声称的"中止的 act 仍写出累积日志并打印 RESULT
摘要"不变量直接矛盾：中途形状校验一旦触发（如 newestPendingOrder 返回 '' 传入 merchantOrderShape），此前 act2/act3 累积的全部证据文件丢失，且
`files['sql-shape-failure.txt']` 本身也永远不会落盘（赋值后未写即退出）。应改为抛错交给顶层 catch 记录，让末尾的写盘循环自然执行。

  function sqlShape(name, value, re) {
    if (!re.test(value)) {
      files['sql-shape-failure.txt'] = `refusing to interpolate ${name} into SQL: ${JSON.stringify(value)}\n`;
-     console.error(`bad ${name} for SQL interpolation: ${JSON.stringify(value)}`);
-     process.exit(2);
+     // 抛错交给顶层 catch：note 记录后仍走到末尾的 writeFileSync 循环与 RESULT 汇总
+     throw new Error(`bad ${name} for SQL interpolation: ${JSON.stringify(value)}`);
    }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:226-226 ───
[bug · medium] act3 用 `.find()` 按 Object.keys 插入序取第一个 `wx_txn_83_main_` 匹配，而脚本自身 (C-03) 注释已声明 stub
跨轮存活（证据目录里 tenant=39/muja7bfh 与 tenant=46/mujdp5nu 两轮残留可证）：重跑 browser_flow 后旧轮订单插入在先，find
会命中旧租户的订单——notify 重放作用于错误订单，而后续 SQL 计数断言却针对本轮 TEN，可能产生"重放未测到本轮订单但仍 PASS"的伪证据或误导性 FAIL。应与
newestPendingOrder 同形（过滤后取最后一个），更稳的做法是 browser_flow 把本轮 out_trade_no 落成 order-info.json（82 轮已有先例）由
act3 精确读取。

-     const mo = Object.keys(orders).find((k) => orders[k].total === 9900 && (orders[k].transaction_id ?? '').startsWith('wx_txn_83_main_')) ?? '';
+     // 与 newestPendingOrder 同形：过滤后取最后一个（本轮的），不取插入序第一个
+     const mains = Object.entries(orders)
+       .filter(([, v]) => v?.total === 9900 && (v?.transaction_id ?? '').startsWith('wx_txn_83_main_'))
+       .map(([k]) => k);
+     const mo = mains[mains.length - 1] ?? '';


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:167-173 ───
[security · medium] lagoCustomer 把 tenant 未经白名单校验直接内插进 docker psql 查询。act2/act4a 的 driveToActive 传入
u.tenant（registerAndLogin 仅校验字段存在，未过 tenantShape），违反脚本自身 (C-02) "每个进入 SQL
字符串的值都先过白名单"的声明不变量——形状异常的租户值会原样进入 SQL（TEN 入口路径与 browser_flow 的 TENANT 均已校验，唯此路径遗漏）。在函数入口补
tenantShape 即可闭合该不变量。

- function lagoCustomer(tenant) {
+ function lagoCustomer(rawTenant) {
+   const tenant = tenantShape(rawTenant); // (C-02) 进入 SQL 前一律过白名单（u.tenant 来自登录响应，未经入口校验）
    try {
      return execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
        `select ppc.provider_customer_id from payment_provider_customers ppc join customers c on c.id=ppc.customer_id where c.external_id='weknora-tenant-${tenant}'`],
        { encoding: 'utf8' }).trim();
    } catch { return ''; }
  }


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:222-224 ───
[maintainability · low] act3 中 `const tokenF = TOKEN_MAIN;` 声明后在整个 act 作用域内从未被引用（act5
的同名变量才有使用），属于死代码，易让读者误以为 act3 需要 API 鉴权。建议删除。

      const act = 'act3-duplicate';
      const log = [];
-     const tokenF = TOKEN_MAIN;


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:24-25 ───
[bug · low] 模块顶层的 LAGO_KEY（docker exec psql）与 MAIN_TENANT_JSON（readFileSync
seed-login-f.json）均无守卫：容器未启动或种子文件缺失时以未包装的原始堆栈崩溃退出，不输出任何友好提示、不产出 RESULT，与脚本其余部分的 (C-04) 证据纪律不一致。另外
MAIN_TENANT_JSON 在 FLOW83_TENANT 已提供时纯属多余——却仍会因文件缺失而崩溃。建议两处都给出可诊断的友好退出，且 seed-login-f.json 仅在 env
缺失时读取。

- const LAGO_KEY = execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
+ function readLagoKey() {
+   try {
+     const key = execFileSync('docker', ['exec', 'weknora-lago-82flow-db-1', 'psql', '-U', 'lago', '-tAc',
-   'select value from api_keys limit 1'], { encoding: 'utf8' }).trim();
+       'select value from api_keys limit 1'], { encoding: 'utf8' }).trim();
+     if (key) return key;
+     throw new Error('empty api key');
+   } catch (err) {
+     console.error(`cannot read Lago api key via docker exec (82flow db container up?): ${String(err).split('\n')[0]}`);
+     process.exit(2);
+   }
+ }
+ const LAGO_KEY = readLagoKey();
+ // MAIN_TENANT_JSON 同理改为：FLOW83_TENANT 未提供时才 try/catch 读取 seed-login-f.json


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:111-113 ───
[bug · medium] FLOW83_TOKEN 未像 EMAIL/PASSWORD/TENANT 那样在入口硬校验（api_recovery_83.mjs 对同名 env
是入口即退）。TOKEN 缺失时 purchaseState 永远返回 'env-missing-token'，settle-active 轮询需空转 24×10 秒（约 4 分钟）后才以
"purchase reached active ... FAIL" 收场——失败证据指向"后端未达 active"，真实原因却是环境变量缺失；且 webhook 投递仍在进行，末端
checkout-active-face 可能反而 PASS，留下自相矛盾的证据。应在入口与其它 env 一致地硬校验（或在轮询中遇到 'env-missing-token' 立即终止并记明原因）。

  const TOKEN = process.env.FLOW83_TOKEN ?? '';
+ if (!TOKEN) {
+   console.error('missing required env: FLOW83_TOKEN (settle-active 轮询依赖 purchase 状态查询；缺失会空转约 4 分钟后留下误导性 FAIL)');
+   process.exit(2);
+ }
  async function purchaseState() {
-   if (!TOKEN) return { data: { state: 'env-missing-token' } };


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:186-187 ───
[bug · low] 订单号从页面文本正则提取失败时（orderId=''）仅记 FAIL 但继续执行，随后以 `checkout?order=`（空订单号）重访。已核实 CheckoutPage
的 `orderIdRef = useRef(orderId)` 对空字符串按 falsy 处理：跳过 getOrder 分支、走新报价提交，已被 active 的购买会被
purchase_not_awaiting_payment 拒绝（"当前购买状态不支持重复支付"），'权益已生效' 永不渲染——最终在 60
秒选择器超时上掩盖真实失败点，恰是本脚本注释自警的路径。orderId 为空时应在该处立即抛错终止此阶段（外层 catch 会记 note，finally 仍写 progression）。

    const orderId = (orderLine.match(/ord_[0-9a-f]+/) ?? [''])[0];
    note('wechat-order-id', orderId !== '', orderId);
+   if (!orderId) {
+     // 空 order 重访会走新报价 → purchase_not_awaiting 拒绝 → 60s 的「权益已生效」超时掩盖真实失败点
+     throw new Error('checkout: awaiting-payment face missing the ord_ id — cannot revisit the active face');
+   }


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:80-87 ───
[maintainability · low] browser_flow_83.mjs 与 api_recovery_83.mjs 重复实现
stubOrders/stubMark/stubNotify/lagoCustomer、NOTPAY/9900 过滤逻辑及硬编码 ORG UUID、docker 容器名等常量（82 目录已有
_browser_lib.mjs 共享库先例）。双份实现存在漂移风险——本轮 (C-03) 的过滤修复只落在两边的内联副本里，后续一处再修另一处极易遗漏。建议抽出共享模块（stub 封装 +
newestPending9900 + lagoCustomer + deliverWebhook + ORG/容器名常量），两脚本统一 import。

- async function stubOrders() {
-   try {
-     const res = await fetch(`${STUB}/stub/orders`);
-     return await res.json();
-   } catch (err) {
-     throw new Error(`stubOrders: stub at ${STUB} unreachable (${String(err).split('\n')[0]})`);
-   }
- }
+ // 新建 docs/plans/issue-72-flow-evidence-83/_stub_lib.mjs（或扩展 82 轮 _browser_lib.mjs），统一导出：
+ // export const LAGO_ORG = '305eddac-1bbd-47a3-af15-219f1d39a27d';
+ // export const LAGO_DB = 'weknora-lago-82flow-db-1';
+ // export async function stubOrders(STUB) { /* ... */ }
+ // export async function stubMark(STUB, id, txn) { /* ... */ }
+ // export async function newestPending9900(STUB) { /* NOTPAY/9900 过滤 + 取最后 */ }
+ // export function lagoCustomer(tenant) { /* tenantShape 守卫 */ }
+ // 两份 mjs 统一 import，消除双份实现漂移。


─── docs/plans/issue-72-flow-evidence-83/v2_up_stubs.sh:31-35 ───
[bug · low] v2_up_stubs.sh 的两个冒烟检查只打印 HTTP 状态码而不做断言（注释写明 expect 401）：stub 以异常姿态启动（如签名校验失效返回
200）时脚本仍判成功退出，冒烟失去把关作用；且启动后固定 sleep 2 与 stub 初始化存在竞态——stub 未就绪时 curl 连接拒绝在 set -e
下以裸退出码结束。建议捕获状态码并比较（wechat 未签名应为 401；alipay 按真实网关形态对未签名请求答 HTTP 200 + isv.invalid-signature 信封，可断言
200 或检查信封），并用 --retry-connrefused/就绪轮询替代固定 sleep。

- sleep 2
- echo '--- smoke: wechat stub rejects unsigned native create (expect 401) ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8296/v3/pay/transactions/native -d '{}'
- echo '--- smoke: alipay stub answers precreate-unauth ---'
- curl -s -o /dev/null -w '%{http_code}\n' -X POST 127.0.0.1:8297/gateway.do -d 'service=alipay.trade.precreate'
+ smoke() { # smoke <name> <url> <data> <expect>
+   local code
+   code=$(curl -s -o /dev/null -w '%{http_code}' --retry 5 --retry-connrefused -X POST "$2" -d "$3") \
+     || { echo "FAIL: $1 TRANSPORT failure (stub up?)"; exit 1; }
+   [ "$code" = "$4" ] || { echo "FAIL: $1 expected HTTP $4 got $code"; exit 1; }
+   echo "$1: HTTP $code OK"
+ }
+ smoke wechat-unsigned-native 'http://127.0.0.1:8296/v3/pay/transactions/native' '{}' 401
+ # alipay stub 按真实网关形态对未签名请求答 200 + isv.invalid-signature 信封
+ smoke alipay-precreate-unauth 'http://127.0.0.1:8297/gateway.do' 'service=alipay.trade.precreate' 200


─── docs/plans/issue-72-flow-evidence-83/wechat_native_stub.py:285-289 ───
[bug · low] wechat_native_stub.py 使用 ThreadingHTTPServer（每请求一线程），但模块级 ORDERS 字典的读-改-写序列（/stub/mark
的状态写入、close 的 get→检查 state→置 CLOSED、/native 的注册）无任何锁保护。act4a 场景本身就是"mark SUCCESS 与后端 close
并发"的编排，另有后端重试/查询并发到达——交错时可能出现 mark 的 SUCCESS 被 close 的延迟写入覆盖为 CLOSED（或反之），把 stub
自身的内部竞态误归因到被测代码，产生误导性证据。建议加 threading.Lock 将 check-then-act 原子化（mark 写与 native 注册同样纳入）。

+ # 模块级：import threading; ORDERS_LOCK = threading.Lock()
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
+ # /stub/mark 的状态写入与 /native 的 ORDERS[...] = {...} 注册同样用 with ORDERS_LOCK 包裹


─── internal/modules/commercial/subscription_command.go:148-152 ───
[maintainability · medium] WalletRank 返回的 1..n 排名无上界，而同一 wallet priority 字段在
GrantIncludedCreditsPayload.Validate 中被约束为 [1,50]（"Required [1,50]"）。rebalance 适配器（lago.go
rebalanceCreditsOrder）把该排名直接 PUT 回 wallet priority：top-up 批次带 12 个月 TTL（#86），活跃钱包数随高频充值累积，超过 50 后
rebalance 会写出文档域外的优先级——若权威侧拒绝则 rebalance 反复失败落入 attention，若接受则同一字段的本地契约静默失真。建议以共享常量（如
MaxWalletPriority=50）统一两处：或在 WalletRank/rebalance 写回前显式守卫（超限 fail closed 交人工处置），或将文档域放宽至与排名语义一致。另外
WalletRank 以 WalletRef 为 map 键，若权威列表出现同名钱包（E3 恢复异常路径），后写排名会静默覆盖前写——建议对重复 WalletRef 显式报错而非静默折叠。

+ const MaxWalletPriority = 50
+ 
+ func WalletRank(batches []WalletRankInput) (map[string]int, error) {
+ 	if len(batches) > MaxWalletPriority {
+ 		return nil, fmt.Errorf("active wallet set %d exceeds the priority domain [1,%d]", len(batches), MaxWalletPriority)
+ 	}
+ 	// ... 排序后：
  	ranks := make(map[string]int, len(ordered))
  	for i, b := range ordered {
+ 		if _, dup := ranks[b.WalletRef]; dup {
+ 			return nil, fmt.Errorf("duplicate wallet ref %q in rank input", b.WalletRef)
+ 		}
  		ranks[b.WalletRef] = i + 1
  	}
- 	return ranks
+ 	return ranks, nil
+ }


─── internal/modules/commercial/payment/alipay.go:490-494 ───
[bug · medium] Alipay Query 只上报 AmountFen 而将 AmountCurrency 留空，与 provider.go 契约及 wechat
腿不一致：collectedAmountMismatch（service/commercial/order.go:533）在 reportedCurrency 为空时直接跳过货币比对，因此
Alipay 腿的恢复路径完全没有 wrong-currency 防护。恢复路径构造 ConfirmPayment 事实时使用的是 `Currency:
att.Currency`（order.go:619）——这正是 provider.go 注释明确警示的"把渠道报告的矛盾货币洗白为 attempt 自身货币"的场景。而 Alipay Create
不校验 req.Currency、total_amount 按定义即为 CNY，所以金额解析成功时货币是确定已知的，应当一并上报；不上报使 spec L127
的货币异常分类（ClassifyPaymentAnomaly 的 currency 变体）在 Alipay 腿永远不可能触发。建议解析成功时同时填充 AmountCurrency: "CNY"，与
wechat 腿（wechat.go:456）对齐。

  	collected := int64(0)
+ 	collectedCurrency := ""
  	if fen, err := ParseCNYAmount(out.TotalAmount); err == nil {
  		collected = fen
+ 		// total_amount 由渠道定义即为 CNY：与实收金额一并上报，
+ 		// 避免恢复路径因空货币跳过比对（与 wechat 腿对齐）。
+ 		collectedCurrency = "CNY"
  	}
- 	return AttemptResult{State: mapAlipayTradeStatus(out.TradeStatus), ProviderID: id, AmountFen: collected}, nil
+ 	return AttemptResult{State: mapAlipayTradeStatus(out.TradeStatus), ProviderID: id,
+ 		AmountFen: collected, AmountCurrency: collectedCurrency}, nil


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_03_restart.sh:29-33 ───
[bug · high] 健康轮询探测未做 errexit 防护：脚本已 `set -euo pipefail`，而 nohup 启动的 `go run ./cmd/server`
在编译完成前端口必未监听，第一次探测即 connection refused（curl 退出码 7）。`code=$(curl ...)` 这类赋值语句的退出码就是命令替换的退出码，会直接触发
errexit 中止脚本——40×5s 等待循环根本无法度过停机窗口，`health=` 行与其后的 "after" 断言块永远执行不到，replay-03-restart.txt 静默截断在
"restarted (launcher N)" 之后且无 FAIL 标记。这与既定发现（缺少 200
断言导致假阳性）是同一循环内的另一个独立缺陷：实际行为不是"轮询落空后继续输出"，而是"第一次失败即中止"。建议给赋值加 `|| code=000` 防护。

  for i in $(seq 1 40); do
-   code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8093/health 2>/dev/null)
+   code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8093/health 2>/dev/null) || code=000
    [ "$code" = "200" ] && break
    sleep 5
  done


─── docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/replay_02_webhook.sh:23-25 ───
[bug · low] 订阅 index 读取缺少显式 `status[]`
过滤。本库的约束性事实（docs/migrations/lago/t02-payment-activation/DECISION.md §7，plan-81 F6、plan-82
§393、adapter 的 identity read 与 lago-after-purchase.txt 均遵守）：`GET /api/v1/subscriptions` 默认只返回
`status=active`。本轮重放预期恰好是 active，默认过滤碰巧能命中；但一旦预期状态不是 active（或重复 webhook 意外改变了状态），该请求会静默返回空数组，jq 提取
`.subscriptions[0].status` 输出 null 被原样写入 replay-02-webhook.txt——与既定发现（空 LAGO_KEY → null
证据）同一误导面，且违反本目录证据脚本自身沿用的显式 status 纪律。建议与 adapter/既有证据读取保持一致的显式 status 列表。

-   curl -s "http://127.0.0.1:48889/api/v1/subscriptions?external_id=weknora-tenant-7-purchase" \
+   curl -s "http://127.0.0.1:48889/api/v1/subscriptions?external_id=weknora-tenant-7-purchase&status[]=active&status[]=incomplete&status[]=canceled&status[]=terminated" \
      -H "Authorization: Bearer $LAGO_KEY" \
      | jq -c '{status: .subscriptions[0].status, plan: .subscriptions[0].plan_code}'


─── internal/handler/commercial.go:680-689 ───
[maintainability · low] benefitsWire 与同 PR 的契约解析器
parseCommercialAccountCredits（packages/contracts/src/commercial.ts）在 Credits==nil 的降级形态上不一致：当
status.Plan != nil 而 status.Credits == nil 时，此处输出的 credits 对象为 batches:null（nil 切片序列化为 JSON null）且无
projected_at 键；而解析器要求 batches 必须是数组、projected_at 必须为非空字符串——api-client 的 account() 会整体抛错，而非其注释宣称的 "a
benefits.credits section absent ... answers null — the card hides, never errors"（该 null 分支实际永不命中，因为
handler 在 benefits 存在时总是输出 credits 对象）。当前该状态经 EnsureBenefits 不可达（月度授予在第 5 步投影前无条件创建 registry 批次行，故
Plan 非空蕴含 Credits 非空），属潜在契约分歧而非现行故障；但这个防御分支一旦被触发（例如未来引入无积分计划版本或调整授予时序），产生的恰是配对解析器拒绝的形态。建议在
Credits==nil 时省略 credits 键（或输出 nil），让 api-client 的 null 降级分支真正生效，与解析器契约对齐。

+ 	if status.Credits != nil {
- 	credits := gin.H{
+ 		credits := gin.H{
- 		"balance_micro":       strconv.FormatInt(balance, 10),
+ 			"balance_micro":       strconv.FormatInt(balance, 10),
- 		"held_micro":          strconv.FormatInt(held, 10),
+ 			"held_micro":          strconv.FormatInt(held, 10),
- 		"refund_locked_micro": strconv.FormatInt(refundLocked, 10),
+ 			"refund_locked_micro": strconv.FormatInt(refundLocked, 10),
- 		"available_micro":     strconv.FormatInt(balance-held-refundLocked, 10),
+ 			"available_micro":     strconv.FormatInt(balance-held-refundLocked, 10),
- 		"batches":             batches,
+ 			"batches":             batches,
- 	}
+ 		}
- 	if projectedAt != "" {
+ 		if projectedAt != "" {
- 		credits["projected_at"] = projectedAt
+ 			credits["projected_at"] = projectedAt
+ 		}
+ 		// Credits==nil：省略 credits 键，让 api-client 的 null 降级分支
+ 		// （"the card hides, never errors"）真正生效——解析器要求
+ 		// batches 为数组且 projected_at 非空。
+ 		return gin.H{
+ 			"plan":     gin.H{ /* ... */ },
+ 			"features": status.Plan.Features,
+ 			"limits":   status.Plan.Limits,
+ 			"credits":  credits,
+ 		}
+ 	}
+ 	return gin.H{
+ 		"plan":     gin.H{ /* ... */ },
+ 		"features": status.Plan.Features,
+ 		"limits":   status.Plan.Limits,
  	}


─── docs/plans/issue-72-flow-evidence-84/wechat_native_stub_anomaly.py:41-46 ───
[bug · medium] 复制 83 原版(wechat_native_stub.py)时丢失了模块级的必需 env 校验块(原版第 63-73 行:REQUIRED 字典 + missing
检查 + sys.exit(2)),与本文件 docstring 声称的"凭据纪律等其余部分逐字复制 83 原版"不符。后果:任一 WECHAT_STUB_* env
缺失时不再快速失败——KEY_DIR 未设时 os.path.join("", "mch_public.pem") 退化为相对 cwd 的裸文件名,轻则抛出难定位的 FileNotFoundError
traceback,重则静默加载 cwd 中恰好同名的无关密钥文件;NOTIFY_URL 为空时推迟到 /stub/notify 才以 urllib ValueError
崩溃;APP_ID/MCH_ID/SERIAL 为空则把空串直接写进签名回调体,失败面远离根因。建议按 83 原版恢复 fail-fast 守卫。

  HOST, PORT = "127.0.0.1", int(os.environ.get("WECHAT_STUB_PORT", "8298"))
  APP_ID = os.environ.get("WECHAT_STUB_APP_ID", "")
  MCH_ID = os.environ.get("WECHAT_STUB_MCH_ID", "")
  PLATFORM_SERIAL = os.environ.get("WECHAT_STUB_PLATFORM_SERIAL", "")
  NOTIFY_URL = os.environ.get("WECHAT_STUB_NOTIFY_URL", "")
  KEY_DIR = os.environ.get("WECHAT_STUB_KEY_DIR", "")
+ 
+ REQUIRED = {
+     "WECHAT_STUB_APP_ID": APP_ID,
+     "WECHAT_STUB_MCH_ID": MCH_ID,
+     "WECHAT_STUB_PLATFORM_SERIAL": PLATFORM_SERIAL,
+     "WECHAT_STUB_NOTIFY_URL": NOTIFY_URL,
+     "WECHAT_STUB_KEY_DIR": KEY_DIR,
+ }
+ missing = [k for k, v in REQUIRED.items() if not v]
+ if missing:
+     print("missing required env: %s (no source-code fallback)" % ",".join(missing), file=sys.stderr)
+     sys.exit(2)


─── docs/plans/issue-72-flow-evidence-84/run_84.sh:47-49 ───
[bug · low] 后端就绪用固定 sleep 25 兜底,而 Step 4 以 go run ./cmd/server 冷启动——首次编译常见超过 25s(冷缓存/低配机)。届时
seed_84.sh 在首个 register 即以 "TRANSPORT failure … backend reachable?" 误报失败,提示面指向"后端不可达"而非"仍在编译",且重跑
run_84.sh 会 pkill 重起全套 stub。注释里自己给出的判据是"应答 4xx 即活",应改为按该判据轮询(带超时上限)再执行种子。

  echo "== 6. 种子（pad + 主角 + pro 9900 发布 + Lago 断言）=="
- sleep 25
+ # 轮询就绪（应答任意 HTTP 状态即活，上限 120s）——go run 冷编译常超 25s。
+ ready=0
+ for _ in $(seq 1 60); do
+   code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8096/api/v1/auth/login || true)
+   [ "$code" != "000" ] && { ready=1; break; }
+   sleep 2
+ done
+ [ "$ready" = 1 ] || { echo "FAIL: backend :8096 not ready within 120s (see ${TMPDIR}issue84-backend.log)"; exit 1; }
  bash docs/plans/issue-72-flow-evidence-84/seed_84.sh


─── docs/plans/issue-72-flow-evidence-84/up_stubs_84.sh:13-15 ───
[bug · low] pkill -f 'alipay_gateway_stub.py' 按脚本名匹配、不分端口与轮次:82/83 轮复用的正是同名脚本(本脚本自己也在 :8299
拉起它)。而本文件头部声明端口纪律是"避开 8291-8297(已被 82/83 轮使用)"——即承认其他轮次 stub
可能仍在运行,此处的按名清理会把它们一并杀掉,破坏并发/续跑中的其他验证轮环境,且失败面后移到那轮的编排里。建议只清理本轮端口实例(按 PID 文件记录,或 pkill -f 匹配含 8299 端口
env 的完整命令行),而不是全局按名杀。

- pkill -f 'wechat_native_stub_anomaly.py' 2>/dev/null || true
- pkill -f 'alipay_gateway_stub.py' 2>/dev/null || true
+ # 只清理本轮遗留（记 PID 文件），避免按名误杀其他轮次仍在跑的同名 stub
+ EV_="$EV"
+ if [ -f "${TMPDIR}issue84-stub-pids" ]; then
+   xargs -r kill 2>/dev/null < "${TMPDIR}issue84-stub-pids" || true
- sleep 1
+   sleep 1
+ fi
+ : > "${TMPDIR}issue84-stub-pids"
+ # …启动处：echo $! >> "${TMPDIR}issue84-stub-pids"


─── internal/modules/commercial/commercialplatform/lago_settlement.go:130-132 ───
[bug · high] 空 unsettled 候选分支的幂等回执未绑定到本次订单的 gating 发票：settledInvoices 是"该客户任意 succeeded intent 携带的发票
id"集合，注释宣称 "A succeeded intent carrying the invoice identity means exactly that"，但代码只检查
len(settledInvoices) > 0，未校验发票归属。当本次购买的手续费 intent 尚未创建或已被取消，而客户存在历史 succeeded
intent（前次购买/续费的残留）时，会铸造假结算回执——渠道已收款但 intent 永不 confirm，激活链路不触发；fulfiller（purchase_fulfillment.go
步骤②→③）每轮都拿到假成功、observeActivation 失败、事件回 pending，直到 D7 预算耗尽才转 attention，且把本应立即 fail closed 的数据异常（no
unsettled gating intent）掩盖成静默空转。建议：空候选分支不做任何发票归属推断，直接按 invalid_response fail closed；幂等窗口只保留下方 F-1
那个按 intent.LagoInvoiceID 绑定的检查。

- 		if len(settledInvoices) > 0 {
- 			return receipt, nil
- 		}
+ 		// No candidate to bind THIS drive's invoice: any settled invoice is
+ 		// unprovable — fail closed (F-1's invoice-bound check below is the
+ 		// only idempotent window).
+ 		return commercial.CommandReceipt{}, fmt.Errorf(
+ 			"%w: no unsettled gating intent carries the invoice identity", commercial.ErrPlatformInvalidResponse)


─── internal/modules/commercial/commercialplatform/lago_settlement.go:317-321 ───
[bug · high] 卡死门定位谓词漏掉 requires_confirmation：settle 驱动在 POST /payment_intents/{pi}（挂 pm，intent 从
requires_payment_method 变为 requires_confirmation）之后、confirm 之前失败——进程崩溃、预算到期、响应丢失，正是 A-18 注释自述要防护的
"re-driving a half-completed leg (pm attached)" 场景——重放时该 intent 被 default 分支当作"非卡死门"跳过：候选为空 →
走上面的空候选分支（假回执或 invalid_response），精心设计的 "<cmd.Key>:update"/":confirm" 确定性幂等键永远没有机会执行，已付款订单无法自动激活。建议将
requires_confirmation 纳入候选集合（对已挂 pm 的 intent 重放 update 是幂等无害的，confirm 才是缺的那一步）。

  			switch row.Status {
- 			case "requires_payment_method", "requires_action":
+ 			case "requires_payment_method", "requires_confirmation", "requires_action":
  			default:
  				continue // canceled/processing… are not the stuck gate
  			}


─── internal/modules/commercial/commercialplatform/lago_settlement.go:330-332 ───
[bug · low] has_more=true 但本页 data 为空时（畸形分页形态）会静默终止分页并返回已收集的结果，违背本函数 A-17/F85 注释的自我声明（"never
silently truncated … beyond is an explicit fail-closed error, never a quiet cut"）——这是扣款正确性关键的读，此时应显式
fail closed 而非返回可能被截断的候选/结算集合。

- 		if !parsed.HasMore || len(parsed.Data) == 0 {
+ 		if !parsed.HasMore {
  			return unsettled, settled, nil
+ 		}
+ 		if len(parsed.Data) == 0 {
+ 			return nil, nil, fmt.Errorf("%w: payment intent list answered an empty page with has_more", commercial.ErrPlatformInvalidResponse)
  		}


─── internal/modules/commercial/commercialplatform/config.go:165-166 ───
[security · medium] 启动姿态守卫只校验 authority BaseURL，但同一个 OutboundAllowLoopback 开关也作用于 provider
腿（lago_settlement.go providerOutboundRequest → validateOutboundHostWithBypass(StripeAPIBase,
bypass)）：BaseURL 为空时守卫放行（"An empty BaseURL stays legal … the provider egress keeps its full runtime
host policy either way" 这句注释与实际行为矛盾——bypass=true 时 provider 出网确实会放行环回），此时 StripeAPIBase
指向环回即被准入，A-27 "绕过只存在于环回 authority 旁"的不变量对 provider 腿未闭合；另外导出的 NewLagoAdapter 可绕过 NewPlatform
的守卫直接构造（当前仅测试使用）。既然守卫的目的是"dev 姿势泄漏到生产时高声失败"，建议把 StripeAPIBase 一并纳入校验。

  	if cfg.BaseURL != "" && !outboundHostIsLoopback(cfg.BaseURL) {
  		return fmt.Errorf("commercial loopback bypass (%s=true) requires a loopback %s, got %q — "+
+ 			EnvOutboundAllowLoopback, EnvBaseURL, cfg.BaseURL)
+ 	}
+ 	if cfg.StripeAPIBase != "" && !outboundHostIsLoopback(cfg.StripeAPIBase) {
+ 		return fmt.Errorf("commercial loopback bypass (%s=true) requires a loopback %s, got %q",
+ 			EnvOutboundAllowLoopback, EnvStripeAPIBase, cfg.StripeAPIBase)
+ 	}


─── internal/modules/commercial/commercialplatform/config.go:176-177 ───
[maintainability · low] 环回主机判定现在有两份实现：本函数与 lago_purchase.go validateOutboundHostWithBypass 内联的
EqualFold("localhost")/.localhost 后缀/ParseIP.IsLoopback 逻辑。当前语义一致，但任一份单独漂移都会让启动姿态守卫与运行时出网策略对同一 URL
给出不同答案（守卫认为非环回、运行时放行，或反之）。建议 validateOutboundHostWithBypass 的 bypass 分支复用本函数，保持单一事实源。

- func outboundHostIsLoopback(rawURL string) bool {
- 	parsed, err := url.Parse(rawURL)
+ // validateOutboundHostWithBypass 的 bypass 分支应改为：
+ //   if outboundHostIsLoopback(rawURL) { return nil }
+ //   return validateOutboundHost(rawURL)


─── internal/modules/commercial/commercialplatform/lago.go:1224-1225 ───
[bug · low] 本函数的状态分支未纳入 429：http.StatusTooManyRequests 落入 default 被归为
ErrPlatformInvalidResponse（终态），与本 diff 在
readSubscriptionByIdentity、ensureProviderBinding、waitForPaymentMethodSync、customerProviderBound、boun
dProviderCustomerID、readPurchaseInvoiceFees、finalizedInvoiceIDs、classifyStripeStatus 等处统一建立的
429/5xx→unreachable 约定不一致。当前唯一调用方吞错（readBenefitsSnapshot 里 err==nil
才采用），暂无实际消费方受害，但一旦复用就会把限流误判为终态，建议按同文件约定对齐。

- 		case status >= 500:
+ 		case status == http.StatusTooManyRequests || status >= 500:
  			return nil, fmt.Errorf("%w: entitlement read unavailable", commercial.ErrPlatformUnreachable)


─── internal/modules/commercial/commercialplatform/lago_purchase.go:621-623 ───
[performance · medium] active 态的 readPurchaseSnapshot 每次都要为定位 gating 发票做 N+1 detail 读（≤10 页索引 × 100
张逐张 GET，注释自述索引含 Base 月度 0 元 finalized 发票、无 subscription 过滤），全部跑在 subscriptionRequestTimeout=15s
内。消费方都是重试热路径：settle 步骤 的幂等重放、observeActivation 轮询、D6' 复核、overBudget 探测——长历史租户一旦超过 15s 即被归为
ErrPlatformUnreachable，fulfiller 视为瞬态反复重试直至 D7 预算耗尽转 attention，已激活的订单可能被误伤。R-23 虽已披露 N+1 形态，但它实际落在
fulfiller 重试路径上而非纯冷读。建议：解析一次成功后将 gating invoice lago_id 与 (tenant, purchase identity)
持久化（outbox/缓存）供后续读直取单张；或利用索引行可用的字段（金额/日期）先做粗筛再 detail，削减逐张 GET 数量。



─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:198-200 ───
[bug · medium] act2 是五幕中唯一没有落地保障的一手 purchase：脚本自身在 purchaseUntilLanded 的注释里记录了「settle/webhook 突发后共享
Lago 栈有短 503 busy window」，而 act2 恰好紧跟 act1 的 settle+webhook 完成，正是最易触发窗口的位置；acts 4a/4b 都已改用
purchaseUntilLanded，唯独 act2 仍是一次性 purchase。失败时 orderID='' 只记一条 FAIL 就继续：后续 newestPendingOrder() 按
NOTPAY/9900 过滤仍可能命中跨轮残留订单（C-03 注释自认 stub 跨轮存活、存在残留），把错误订单标成 SUCCESS 污染 stub 状态；而 GET /orders/'' 404
之后的连锁 FAIL 全部指向「恢复查询未支付」，掩盖了「purchase 根本没落地」的真实根因。建议改用 purchaseUntilLanded 并对落地订单 id 过 orderShape。

-     const q1 = await quote(u.token);
-     const p1 = await purchase(u.token, q1, 'wechat');
-     const orderID = p1.json?.data?.order?.id ?? '';
+     const { res: p1, order } = await purchaseUntilLanded(u.token, 'wechat', log);
+     const orderID = orderShape(order?.id ?? '');
+     note(act, 'wechat-order', p1.status === 201 && (order?.checkout_url ?? '').startsWith('weixin://'), `order=${orderID}`);


─── docs/plans/issue-72-flow-evidence-83/api_recovery_83.mjs:366-368 ───
[bug · medium] 与已确认的 sqlShape process.exit(2) 问题（#1）不同机制但同样违反 (C-04)：act 是裸块而非函数，块内任何
throw——purchaseUntilLanded 耗尽预算后的终止抛错、quote() 里 json.data 为 undefined 时 `json.data.id` 的
TypeError、driveToActive 中的意外异常——都会跳过该 act 末尾的 files['xxx.txt'] 赋值，被中止 act 的累积日志（含 purchase try0..3
的诊断行）全部丢失，且后续所有 act 被整体跳过。这恰好是 (C-04) 注释承诺「aborted act 仍写出累积日志」却不成立的路径。建议每个 act 用 try/finally（或
catch 中补写）保证日志落盘后再重抛。

+     try {
+       // ... act body ...
- } catch (err) {
+     } catch (err) {
-   note('script', 'uncaught-flow-error', false, String(err));
+       files['close-race-paid.txt'] = `${log.join('\n')}\nABORTED: ${String(err)}`;
+       throw err;
+     } finally {
+       if (!files['close-race-paid.txt']) files['close-race-paid.txt'] = log.join('\n');
- }
+     }


─── docs/plans/issue-72-flow-evidence-83/seed_83.sh:74-75 ───
[test · medium] PAD 占位的前提（新 WeKnora 库租户号从 1 起、Lago 仅被占到 12）既无守卫也无事后断言，且默认值已相对现实过期：同目录 README 记录 v2
复验轮时 82flow 栈已被外部轮占到 tenant-34，不得不手工把 FLOW83_PAD_COUNT 提到 38。今天按默认 14 重跑，主角会落在 tenant 15/16，与外部轮的
weknora-customer-15/16（可能是 active 的 purchase 订阅）在 Lago 侧身份碰撞，act3/act5 会对错误租户的 Lago 四对象做断言，产生误导性
PASS/FAIL——脚本自身 C-11「断言轮次特定事实」的纪律应覆盖这一点：注册后断言主角租户号确实越过了被占上限。

- say "tenant A (browser protagonist) = $TENANT_A"
- say "tenant B (publisher) = $TENANT_B, user id = $UID_B"
+ OCCUPIED="${FLOW83_LAGO_OCCUPIED_MAX:-12}"
+ [ "$TENANT_A" -gt "$OCCUPIED" ] && [ "$TENANT_B" -gt "$OCCUPIED" ] \
+   || { say "FAIL: protagonists ($TENANT_A/$TENANT_B) overlap the Lago-occupied id range (<= $OCCUPIED); raise FLOW83_PAD_COUNT"; exit 1; }


─── docs/plans/issue-72-flow-evidence-83/browser_flow_83.mjs:175-180 ───
[test · low] 初始 checkout 有 4 次循环容忍 Lago 503 busy
window（点击页面自带的重试入口），但切换微信的「改用微信支付重新发起支付」这条腿没有同等容错：已核实 CheckoutPage 在切换购买失败时渲染的就是同一个通用错误面 +
同一枚「重试（同一报价服务端幂等…）」按钮（CheckoutPage.tsx:318-322，restartCheckout 会带着新渠道重跑同一 effect）。此刻一次 503 会直接落在 60s
的 weixin:// 选择器超时上，真实失败点（渠道切换被平台短暂拒绝）被掩盖——正是本脚本注释自警的场景。建议 switch 腿复用同一 retry 检测循环。

-   await wechatRadio.check();
-   const switchBtn = page.locator('button', { hasText: '改用微信支付重新发起支付' });
-   await switchBtn.waitFor({ timeout: 10000 });
    await switchBtn.click();
-   // The new order opens on the wechat channel: awaiting payment + weixin:// link.
-   await page.waitForSelector('a[href^="weixin://wxpay/"]', { timeout: 60000 });
+   for (let attempt = 0; attempt < 4; attempt += 1) {
+     const shown = await Promise.race([
+       page.waitForSelector('a[href^="weixin://wxpay/"]', { timeout: 20000 }).then(() => 'landed'),
+       page.locator('button', { hasText: '重试（同一报价服务端幂等' }).waitFor({ timeout: 20000 }).then(() => 'retry'),
+     ]).catch(() => 'timeout');
+     if (shown === 'landed') break;
+     if (shown === 'retry') {
+       await dismissGuides(page);
+       await page.locator('button', { hasText: '重试（同一报价服务端幂等' }).click();
+       continue;
+     }
+     if (attempt === 3) throw new Error('wechat switch: the weixin:// face never landed after 4 attempts');
+   }


─── docs/plans/issue-72-flow-evidence-83/v2_up_backend.sh:20-21 ───
[test · low] v2 复验轮的配套 env 没有任何提示或断言：本脚本把后端指向新库 data/issue83-flow-v2.db，但同轮要复用的 seed_83.sh 与
api_recovery_83.mjs 的 FLOW83_DB 默认仍是 data/issue83-flow.db。忘设 FLOW83_DB 时 seed 会把 commercial_grants
写进 v1 旧库（后端在新库读不到授权 → 发布 403「receipt mismatch」，根因难辨），api 腿则对旧库跑全部 SQL 断言（形状校验以「bad order id:
''」硬退，指向错误）。README 只在文档层记录了参数化，脚本侧无护栏。建议启动时打印/校验配套 env，或提供统一注入
FLOW83_DB/FLOW83_BACKEND/FLOW83_STUB/FLOW83_TENANT/FLOW83_PAD_COUNT 的 v2 runner。

  export SERVER_PORT=8095 SERVER_HOST=127.0.0.1
  export DB_DRIVER=sqlite DB_PATH=data/issue83-flow-v2.db
+ echo 'companion env for seed_83.sh / api_recovery_83.mjs (required this round):'
+ echo "  FLOW83_DB=${DB_PATH} FLOW83_BACKEND=http://127.0.0.1:8095 FLOW83_STUB=http://127.0.0.1:8296 FLOW83_PAD_COUNT=<past occupied Lago tenants>"


─── internal/modules/commercial/repository/commercial/order.go:356-358 ───
[bug · high] CurrentPurchaseOrder 的 pending 偏好谓词只检查 `State == pending && !ChannelFailed`，遗漏了
`CheckoutURL != ""`，与注释声明的「与 CurrentPendingPurchaseOrder 相同的 payable 谓词」（其 SQL 含 `checkout_url <>
''`）不一致。R2-27 持久化降级行（渠道 Create 成功但 SetCheckoutURL 失败——service/commercial/order.go:488-491
明确保留该形态且不标记 channel_failed）会赢得偏好：调用方 purchase.go:488 用该行投影 out.Order（orderViewFromRow 携带空
CheckoutURL），客户端得到一个无支付入口的永久 pending 投影；同时它遮蔽同价位已支付订单，导致 paid_awaiting_activation 合成窗口（A-29/F109
注释声称要防护的场景）被错过，且该状态可持续到 15 分钟时窗的 SweepStaleLinklessPending 清扫为止。建议补齐与 CurrentPendingPurchaseOrder
一致的 payable 判定。

- 		if row.State == domain.OrderStatePending && !row.ChannelFailed {
+ 		if row.State == domain.OrderStatePending && !row.ChannelFailed && row.CheckoutURL != "" {
  			return row, nil
  		}


─── internal/modules/commercial/repository/commercial/order.go:441-442 ───
[documentation · low] 此注释声称「PAYABLE is the SAME predicate as the index: channel_failed = false AND a
persisted checkout_url」，但实际索引 uq_purchase_pending_per_tenant 的谓词仅为 `channel_failed =
false`（service/commercial/order.go:155-157），且该处注释（141-147 行）明确说明这是刻意设计（含 checkout_url 会把冲突从原子 INSERT
挪到 SetCheckoutURL UPDATE）。本注释误述了数据库不变量：link-less 行在未被清扫前确实占据 pending 槽位——这与同文件
SweepStaleLinklessPending 的注释相互矛盾，会误导后续维护者依赖一个不存在的索引保证。建议更正为「读取谓词严于索引」。



─── internal/modules/commercial/repository/commercial/payment_anomaly.go:193-194 ───
[maintainability · low] RowsAffected==0 且行存在时一律返回
ErrPaymentAnomalyVersionConflict，把两种不同情况混叠为一个哨兵：版本确实过期，以及行已处于 resolved 状态（操作员重放一次已成功的
resolve，或以回读的新版本重放）。后者在语义上是幂等成功，却报「版本冲突」，误导操作面排查。建议先判 state：已 resolved
的行直接返回该行（幂等重放），仅版本不匹配才报冲突。另建议为 ListPaymentAnomalies 预留分页参数，避免异常堆积时管理接口返回超量行。

+ 		if row.State == PaymentAnomalyStateResolved {
+ 			return row, nil // idempotent replay of an already-resolved anomaly
+ 		}
  		return PaymentAnomalyRow{}, ErrPaymentAnomalyVersionConflict
  	}


─── packages/contracts/src/commercial.ts:176-180 ───
[bug · high] 跨文件契约不匹配（高）：解析器把 projected_at 定为必填（nonEmptyString 抛错）、batches 必须是数组，但后端
benefitsWire（internal/handler/commercial.go:680-689）在 status.Credits == nil 时仍会输出 credits 对象——省略
projected_at 键且 batches 序列化为 null；而服务层 status.Plan（benefits.go:308，row.PlanKey != ""）与
status.Credits（benefits.go:319，len(batches) > 0）是两个独立条件，「套餐已投影但批次为空」是合法形态（跨月唯一批次到期、授权快照读滞后、grant
注册表幂等跳过等）。此时 api-client 的 account() 会因 credits 对象存在而走 parseCommercialAccountCredits → 抛 'invalid
account credits (projected_at)'，BillingPage
渲染原始英文解析消息的错误卡片（BillingPage.tsx:222-225），直接违背注释宣称的「benefits.credits 缺席 → null，卡片隐藏、永不报错」。建议：将
projected_at 与 granted_at 同等对待为展示性字段（缺省降级为 ''），并把 null/缺省的 batches 视同空数组；或由后端在 Credits == nil 时整个省略
credits 键，使客户端 null 降级路径真正可达。

    available_micro: signedDigitString(v.available_micro,'available_micro','account credits'),
-   projected_at: nonEmptyString(v.projected_at,'projected_at','account credits'),
+   // projected_at 与 granted_at 同为展示性字段：后端 Credits==nil（套餐已投影
+   // 但批次为空）时省略该键，降级为 '' 而非整卡解析失败
+   projected_at: typeof v.projected_at === 'string' && v.projected_at !== '' ? v.projected_at : '',
    batches: [],
   };
+  // 后端 Credits==nil 时 batches 序列化为 null：视同空批次数组
+  if(v.batches === undefined || v.batches === null) return out;
   if(!Array.isArray(v.batches)) throw new Error('invalid account credits (batches)');


─── packages/contracts/src/commercial.ts:192-192 ───
[bug · low] granted_at 缺省降级为空串后，消费方 BillingPage（BillingPage.tsx:211）无守卫地渲染 ` ·
{batch.granted_at.slice(0, 10)}`，CR-86-1 跨月滞留批次形态会显示悬挂的「 · 」分隔符（空日期）。契约类型上更安全的做法是声明为可选 `granted_at?:
string`，让类型系统强制消费方处理缺省（如 `{batch.granted_at ? ` · ${batch.granted_at.slice(0,10)}` :
''}`），而非用空串静默吞掉缺失。

-    granted_at: typeof b.granted_at==='string' ? b.granted_at : '',
+    granted_at: typeof b.granted_at==='string' ? b.granted_at : undefined,


─── internal/handler/commercial.go:870-873 ───
[bug · low] 409 附带的 pending 订单投影缺少 payment_attention 装饰，与同一订单的其它读面不一致：GET /orders/:id（service 层
withAttention，order.go:582/590/633）和 GET /commercial/purchase（purchase.go:496-498）都会为带未决异常的订单置
PaymentAttention=true，而 CurrentPayablePendingOrderView 不做该装饰。mismatch/部分支付留存后订单仍处
pending，此场景完全可达：客户端在此 409 面上收到 fulfillment:"pending" 且无 payment_attention，下一次轮询同一订单又变为 attention ——
同一订单出现两个互相矛盾的 wire 形态。建议让 CurrentPayablePendingOrderView 返回前同样过 withAttention 装饰（服务层一行修复），保持该 add-on
"rides along on every state" 的契约。

- 			if qerr == nil && eerr == nil && reqSnap.PlanKey == exSnap.PlanKey {
- 				c.JSON(http.StatusConflict, gin.H{"error": "purchase pending exists",
- 					"order": orderWire(existing)})
- 				return
+ // 服务层修复（service/commercial/order.go CurrentPayablePendingOrderView）：
+ // return s.withAttention(ctx, OrderView{ID: row.ID, QuoteID: row.QuoteID, State: row.State,
+ // 	AmountFen: row.AmountFen, Currency: row.Currency, CheckoutURL: row.CheckoutURL,
+ // 	Version: row.Version}), nil


─── internal/handler/commercial.go:536-537 ───
[style · low] 本文件引入的 5 处 log.Printf 使用标准库 log，而整个 handler 包（以及本 PR 其它服务层代码）统一使用 internal/logger 的带
ctx 结构化日志（logger.Errorf 等）。这些日志行存在的目的恰是"可诊断的一行"（R1-09 / r2:823 的降级可诊断性），走 stderr
裸输出会脱离项目的日志管道（request id / 级别 / 采集），降低其可达性。建议改用 logger.Errorf(c.Request.Context(), ...) 保持一致。

- 		log.Printf("commercial: purchase failed for tenant %d: %v", tenantID, err)
+ logger.Errorf(c.Request.Context(), "[Commercial] purchase failed for tenant %d: %v", tenantID, err)
- 		c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})
+ c.JSON(http.StatusInternalServerError, gin.H{"error": "purchase failed"})


─── internal/modules/commercial/service/commercial/fulfillment.go:380-381 ───
[bug · medium] disposeOverPayment 将 outbox 载荷的 attempt_id 直接写入 PaymentAnomalyRow.AttemptID，但该列的文档语义是
merchant_order_id（payment_anomaly.go 注释），且同表其余两个写入方（ConfirmPayment 的 buildMismatchAnomaly、本包
recoverMismatchedCollection）写入的都是 att.MerchantOrderID（"mo_" 前缀）。而 ConfirmPayment 构造 over_payment 载荷时
AttemptID: attempt.ID 是 PaymentAttemptRow 的内部主键（"att_" 前缀，openOrder 中两者是不同随机值）。结果：over_payment 类异常行的
attempt_id 与金额/币种错配异常行的口径分裂，运营处置面按 merchant_order_id 关联渠道单据时 over_payment 行全部对不上。幂等不受影响（唯一键是
provider+merchant+transaction），但列语义被破坏。建议：在 ConfirmPayment 写 over_payment 载荷时携带 MerchantOrderID，或此处按
payload.OrderID 反查 attempt 行取其 MerchantOrderID 后再落库。

+ 	// 对齐 PaymentAnomalyRow.AttemptID = merchant_order_id 口径（与
+ 	// buildMismatchAnomaly / recoverMismatchedCollection 一致）。
+ 	attemptID := payload.AttemptID
+ 	if att, aerr := s.orders.FirstPendingAttempt(ctx, payload.OrderID); aerr == nil {
+ 		attemptID = att.MerchantOrderID
+ 	}
  	if err := s.orders.RecordPaymentAnomaly(ctx, repocommercial.PaymentAnomalyRow{
- 		TenantID: payload.TenantID, OrderID: payload.OrderID, AttemptID: payload.AttemptID,
+ 		TenantID: payload.TenantID, OrderID: payload.OrderID, AttemptID: attemptID,


─── internal/modules/commercial/service/commercial/purchase_fulfillment.go:169-173 ───
[bug · medium] succeededAttempt 的错误（含"无 succeeded attempt"/"attempt 无渠道流水"这类确定性形状，以及瞬时 DB 故障）直接
return err，而 FulfillmentService.Recover 对 fulfill 事件的错误是整批中止（return fmt.Errorf("fulfill event %s:
%w", ...)），与本文件自己声明的 r2:119/A-32 纪律（瞬时 DB 失败 return nil 保持 pending、确定性数据缺口落 attention 后 return
nil，绝不中止共享 drain）不一致。瞬时故障会饿死同批后续事件（含普通充值履约与 over_payment 处置）一个 drain 周期；确定性形状则每一趟都永久阻断整批。Fulfill 顶部的
GetOrder 返回 err 也有同样问题。建议与 settleSnapshotFailure 相同的分流：确定性形状落 markActivationState(attention) 后返回
nil，瞬时 DB 故障 Warn 后返回 nil。

  	// The verified channel transaction is the settle idempotency anchor.
  	attempt, err := p.succeededAttempt(ctx, order.ID)
  	if err != nil {
- 		return err
+ 		// (A-32 同款分流)：确定性形状（哨兵）落 attention；瞬时 DB 故障
+ 		// 保持 pending——都不中止共享 drain 批。
+ 		if errors.Is(err, ErrNoSucceededAttempt) {
+ 			return p.markActivationState(ctx, order.ID, order.TenantID, domain.FulfillmentStateAttention)
+ 		}
+ 		logger.Warnf(ctx, "[CommercialFulfillment] purchase %s succeeded-attempt read failed (stays pending): %v", order.ID, err)
+ 		return nil
  	}


─── packages/api-client/src/commercial.ts:106-107 ───
[test · low] 测试缺口（低）：新增的 account() 是本文件中唯一没有单元测试的端点方法——packages/api-client/src/commercial.test.ts
按既有惯例覆盖了每个方法的路径映射与信封解包（'maps each commercial endpoint to method, path, and body'），但 account()
的四个分支（benefits 缺席→null 的 pending 语义、benefits 非对象→INVALID_RESPONSE、credits
缺席→null、正常解析链）均未被执行；apps/web 的 BillingPage.test.ts 只 mock 了 client.account，真实分支逻辑零覆盖。鉴于确认发现 #1
所示的契约/后端漂移正是这类真实信封回放测试本可拦截的问题，建议按文件惯例补充 account 的路径映射 + pending-null + 正常解析用例。

-     async account(signal?: AbortSignal): Promise<CommercialAccountCredits | null> {
-       const data = unwrap(await request({ method: 'GET', path: '/api/v1/commercial/account', signal }));
+ // packages/api-client/src/commercial.test.ts 补充（示意）：
+ test('account maps GET /commercial/account; pending benefits answer null', async () => {
+   const requests: Array<{ method: string; path: string }> = [];
+   const pending = fakeApi(() => ({ success: true, data: { state: 'pending', reason: 'unconfigured' } }), requests);
+   assert.equal(await pending.account(), null);
+   const api = fakeApi(() => ({ success: true, data: { state: 'linked', benefits: { credits: creditsBreakdown } } }), requests);
+   const credits = await api.account();
+   assert.equal(credits?.balance_micro, creditsBreakdown.balance_micro);
+   assert.deepEqual(requests.map((r) => r.path), ['/api/v1/commercial/account', '/api/v1/commercial/account']);
+ });


─── internal/modules/commercial/repository/commercial/order.go:732-734 ───
[bug · low] ConfirmPayment 第二个 mismatch 分支（ValidatePayment 失败）的异常快照 expected 取自 attempt，但该分支恰是
attempt 与 fact 完全一致、而订单行（row）与 fact 不一致时才触发：此时 ClassifyPaymentAnomaly(attempt.AmountFen,
fact.Amount, attempt.Currency, fact.Currency) 的两侧相等，落库的异常行会呈现 kind=amount_mismatch 且
ExpectedAmountFen == ActualAmountFen、ExpectedCurrency == ActualCurrency
的自相矛盾记录，真正的期望面（row.AmountFen/row.Currency，作用域内可得）被丢弃，操作员处置面拿不到实际差异。buildMismatchAnomaly 的注释声称
"expected come from the REGISTERED attempt (the order's frozen face)"，在本分支恰是该假设被违反的时刻。建议此分支用订单行作为
expected（或为该分支单独构造快照）。

+ 		if err := domain.ValidatePayment(row.Domain(), fact); err != nil {
  			snap := buildMismatchAnomaly(attempt, fact)
+ 			// 该分支 attempt==fact，真正不一致的是订单行的冻结价面
+ 			snap.ExpectedAmountFen = row.AmountFen
+ 			snap.ExpectedCurrency = row.Currency
+ 			snap.Kind = ClassifyPaymentAnomaly(row.AmountFen, int64(fact.Amount), row.Currency, fact.Currency)
  			anomaly = &snap
  			return err
+ 		}


─── internal/modules/commercial/commercialplatform/lago.go:244-247 ───
[bug · low] 新加的 rebalanceCreditsOrder 钱包 PUT 状态分支漏掉了 429：本 diff 在
readSubscriptionByIdentity、ensureProviderBinding、waitForPaymentMethodSync、customerProviderBound、read
PurchaseInvoiceFees、finalizedInvoiceIDs、classifyStripeStatus、boundProviderCustomerID
等所有（含新增）状态分流处统一建立了「429/5xx → ErrPlatformUnreachable（可重试）」的约定（lago_settlement.go 文件头亦将此声明为 seam
的封闭错误分类契约），但这里 http.StatusTooManyRequests 落入 default 被归为 ErrPlatformInvalidResponse（终态"权威拒绝"）。Lago
限流时一次 priority 校准会被误标为终态；当前唯一调用方（benefits 刷新尾的 Warn-and-retry）不受影响，但该 sentinel 是 seam 的公共契约，未来经
outbox/协调器重试的消费者会据此把 429 停进 attention 而非重试。

  		switch {
  		case status >= 200 && status < 300:
- 		case status >= 500:
+ 		case status == http.StatusTooManyRequests || status >= 500:
  			return commercial.CommandReceipt{}, fmt.Errorf("%w: wallet priority update unavailable", commercial.ErrPlatformUnreachable)


─── internal/modules/commercial/service/commercial/fulfillment.go:404-408 ───
[bug · high] isSubscriptionPurchase 把「无法证明是订阅购买」当成「证明不是订阅购买」处理：除 NotFound（legacy 语义）外，任何瞬时 DB
读失败（连接池耗尽、failover、重启）以及 SnapshotJSON 解析失败都返回 false。调用方 fulfillEvent 随即让这笔已支付的订阅订单走
s.lines(order)=TopUpOrderLines 的按簿记汇率结算（order.Amount×10000 额度），经
ensureRecord/processRecord/orderComplete 后 MarkFulfilled 并把事件标记 Sent——此后没有任何路径会再把该订单重新路由到
PurchaseFulfiller 的 settle/observe/grant 链。这正是注释自己承认的"本调度器最昂贵的静默失败"，但代码只用一条 Warn
兜底，误路由是一次性、不可恢复的（客户付了订阅费却只拿到按汇率折算的一次性额度，订阅永不激活）。这与本文件 r2:119/A-32 已确立的纪律矛盾：瞬时故障应保持事件 pending
等下一趟重驱（return nil + completeEvent(Pending)），确定性数据缺口应落 attention。建议将 isSubscriptionPurchase 改为 (bool,
error) 或三态：ErrRecordNotFound/无 subscription_fee 行保持 false（legacy/top-up 语义）；其他读错误与解析失败向上传递为"不确定"，由
fulfillEvent 以 completeEvent(Pending) 保留事件，绝不落入 top-up 结算。

  		if !errors.Is(err, gorm.ErrRecordNotFound) {
- 			logger.Warnf(ctx, "[CommercialFulfillment] subscription-purchase quote read failed for order %s: %v", row.ID, err)
- 		}
- 		return false
+ 			// Transient read failure: the discriminator is UNPROVEN, not false —
+ 			// return an indeterminate signal so the event stays pending instead
+ 			// of misrouting a paid subscription into the terminal top-up path.
+ 			return false, fmt.Errorf("subscription-purchase quote read for order %s: %w", row.ID, err)
- 	}
+ 		}
+ 		return false, nil


─── internal/modules/commercial/service/commercial/order.go:698-703 ───
[bug · low] CloseChannelOrder 的 close 与 query 双失败分支只返回 qerr，原始 close 错误 cerr
被完全丢弃。这个分支的语义是"两端都无法定论、结果未知"，而 close 失败的原因（ORDER_PAID 竞态 vs 传输超时 vs
already-closed）恰恰是运维判断该重试还是人工介入的关键线索；只剩 query 错误（往往只是又一次网络故障）会让日志/告警丢失真正的原因链。建议用 %w 保留 qerr 身份的同时把
cerr 并入错误链。

  	res, qerr := provider.Query(ctx, att.MerchantOrderID)
  	if qerr != nil {
- 		// Both legs failed: the outcome stays unknown — surface it, mark
- 		// nothing, keep the payable entry for a retry.
- 		return OrderView{}, qerr
+ 		// Both legs failed: the outcome stays unknown — surface BOTH errors
+ 		// (the close cause is the operationally decisive one), mark nothing,
+ 		// keep the payable entry for a retry.
+ 		return OrderView{}, fmt.Errorf("channel close failed (%v) and status query failed: %w", cerr, qerr)
  	}


LLM retry report summary: 2 of 673 requests affected -- 2 requests recovered after retry

Core review (2 requests):
- docs/plans/issue-72-flow-evidence-82/r4-flow2/api-01-draft.json,docs/plans/issue-72-flow-evidence-82/r4-flow2/api-02-publish.json,docs/plans/issue-72-flow-evidence-82/r4-flow2/api-03-lago-plans.json,docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_01_checkout.mjs,docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_02_sync_face.mjs,docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_03_paid_face.mjs,docs/plans/issue-72-flow-evidence-82/r4-flow2/browser_04_active_face.mjs,docs/plans/issue-72-flow-evidence-82/r4-flow2/order-info.json,docs/plans/issue-72-flow-evidence-82/r4-flow2/seed.sh: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded
- docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/account-after-active.json,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/api-01-draft.json,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/api-02-publish.json,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/api-03-lago-plans-baseline.json,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/api-03-lago-plans.json,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/api-04-lago-plan-readback.json,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_01_checkout.mjs,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_02_sync_face.mjs,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_03_paid_face.mjs,docs/plans/issue-72-flow-evidence-82/r5b-flowcheck/browser_04_active_face.mjs: rate limited (HTTP 429) -> rate limited (HTTP 429) -> rate limited (HTTP 429) -> succeeded

Per-attempt detail: --format json (retry_report).
