Review complete: 13 finding(s) across 14 selected item(s).

─── docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py:86-91 ───
[bug · low] billable_metrics/plans/subscriptions 三个 POST 均未检查返回状态码：创建失败（如 422 校验错误）时
body["billable_metric"]["lago_id"] 会以 KeyError 回溯崩溃，或后续事件失败被误报为 "events failed to
post"，排查成本高。consume_86.py 同位置均有状态检查，建议保持一致（plan/subscription 两处同理）。

-     _, body = call("POST", "/api/v1/billable_metrics", payload={
+     status, body = call("POST", "/api/v1/billable_metrics", payload={
          "billable_metric": {"code": metric_code,
                              "name": "WeKnora 86 concurrent " + tag,
                              "aggregation_type": "sum_agg",
                              "field_name": "units"}})
+     if status not in (200, 201):
+         print("metric create failed:", status, body); sys.exit(2)
      metric_id = body["billable_metric"]["lago_id"]


─── docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py:162-164 ───
[bug · low] 两个边界问题：1) `early, late = names[0], names[1]` 未先确认活跃钱包数 ≥ 2（不足时 IndexError），且
before[early]/before[late] 假设前后两次快照键集合一致，运行期间钱包发放或终止会触发 KeyError；2) 排序键 `expiry_of.get(n) or ""`
将无到期时间（None）的钱包排为"最早到期"，与"最早到期优先扣"的语义相反。建议排序时把 None 排最后并加前置校验。

      expiry_of = {w["name"]: w.get("expiration_at") for w in wallet_list()}
-     names = sorted(after, key=lambda n: expiry_of.get(n) or "")
+     names = sorted(after, key=lambda n: (expiry_of.get(n) is None,
+                                          expiry_of.get(n) or ""))
+     if len(names) < 2 or any(n not in before for n in names[:2]):
+         print("FAIL: need >=2 stable active wallets"); sys.exit(2)
      early, late = names[0], names[1]


─── docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py:156-157 ───
[test · low] 重放事件后仅固定 time.sleep(10) 即取余额，而主扣费路径使用最长 300s 的 wait_until 轮询：若重放被错误接受且结算延迟超过
10s，"重放不多扣"断言会在观察窗口外漏判，验证结论不稳定。建议改为在有限窗口内轮询并要求总额持续等于 total_after，与主路径保持一致。

-     time.sleep(10)
+     def replay_quiet(seconds=60, interval=3.0):
+         ended = time.monotonic() + seconds
+         while time.monotonic() < ended:
+             if sum(balances().values()) != total_after:
+                 return False
+             time.sleep(interval)
+         return True
+     ok_replay = replay_quiet()
      replay_after = balances()


─── docs/plans/issue-72-flow-evidence-86/concurrent_consumption_86.py:176-178 ───
[bug · medium] "draw order follows expiry rank" 断言只建模了两个钱包的分布（early 扣空、spill 全部落到 late）：当 draw_cents
超过 before[early] + before[late] 需要级联到第三个及以后的钱包时（例如 monthly 余额 0、次早钱包 400，而 draw=500，参考证据中 2026-09=0
的实际状态），after[late] == before[late] - spill 必然不成立，正确的级联扣减会被误判为 FAIL，验收结论不稳定；late 之后的钱包也完全未被覆盖。建议改为按
expiry 序遍历全部活跃钱包，逐个断言"前面的必须扣空、最多最后一个部分扣减"。

          ("draw order follows expiry rank (earliest drained first)",
-          after[early] == 0 and spill >= 0
-          and after[late] == before[late] - spill,
+          all(after[n] == 0 for n in names[:-1])
+          and after[names[-1]] == before[names[-1]] - remaining_spill,
+          ...)


─── docs/plans/issue-72-flow-evidence-86/consume_86.py:155-157 ───
[bug · medium] 硬编码 "consume_cents": 150 与实际扣减不符：脚本发送 units=2（单价 1.00 CNY = 200 分），wait_until
断言总额恰好减少 200，且两份证据文件 deltas 合计均为 200。150 是遗留常量，导致 #86 验收证据中的扣费金额元数据与真实扣减不一致，削弱证据可信度。建议改为 200
或与断言共用同一常量。

      result = {
          "metric": metric_code, "plan": plan_code, "subscription": sub_ext,
-         "event": txn_id, "consume_cents": 150,
+         "event": txn_id, "consume_cents": 200,  # units=2 × 1.00 CNY，与 wait_until 断言一致


─── docs/plans/issue-72-flow-evidence-86/consume_86.py:22-23 ───
[security · medium] BASE 直接取自环境变量 LAGO_BASE，未经 scheme 校验与出网白名单校验就在 request() 中拼接 URL 并携带从 .env 读取的
Bearer API Key。同目录 concurrent_consumption_86.py 与 reconcile.py 均实现了 base_url() 的 scheme +
ALLOWED_TARGETS(127.0.0.1) 校验；LAGO_BASE 被误设或被污染时，Lago 凭据会被发往任意主机，脚本族的出网策略在此处失守。建议补齐同款校验并在 request()
中使用 base_url()。

- BASE = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
+ ALLOWED_TARGETS = {"127.0.0.1"}  # the local Lago test stack, nothing else
  CUSTOMER = os.environ.get("LAGO_CUSTOMER", "weknora-tenant-10000")
+ 
+ 
+ def base_url():
+     raw = os.environ.get("LAGO_BASE", "http://127.0.0.1:48889")
+     parts = urlsplit(raw)
+     if parts.scheme not in ("http", "https"):
+         raise SystemExit("scheme not allowed: " + parts.scheme)
+     if parts.hostname not in ALLOWED_TARGETS:
+         raise SystemExit("target host not in test-stack allow-list: "
+                          + parts.hostname)
+     return raw
+ # request() 中改为: url = f"{base_url()}{path}"


─── docs/plans/issue-72-flow-evidence-86/consume_86.py:55-57 ───
[bug · low] 钱包查询固定 per_page=20 且未跟随 next_cursor 分页（concurrent_consumption_86.py 的 wallet_list() 与
reconcile.py 的 lago_wallets() 同样如此，三处均无游标处理）：租户活跃钱包数超过 20 时余额快照与对账会被静默截断、漏计钱包，可能得出错误的对账结论。建议循环读取
next_cursor 直到取完。

  def wallets():
-     _, body = request("GET", f"/api/v1/customers/{CUSTOMER}/wallets?per_page=20")
-     return {w["name"]: w for w in body.get("wallets", [])}
+     out, cursor = [], None
+     while True:
+         path = f"/api/v1/customers/{CUSTOMER}/wallets?per_page=20"
+         if cursor:
+             path += "&cursor=" + cursor
+         _, body = request("GET", path)
+         out += body.get("wallets", [])
+         cursor = body.get("next_cursor")
+         if not cursor:
+             break
+     return {w["name"]: w for w in out}


─── docs/plans/issue-72-flow-evidence-86/consume_86.py:168-170 ───
[bug · medium] 对过滤结果直接取 [0] 未判空，且把判定绑死在硬编码的钱包名上：当租户钱包集合不含 "topup-c"/"topup-d"（如同目录证据
consume-leg1.json 的真实运行只有 topup-a/topup-b/2026-09，尚无 topup-c/d）时，此处会 IndexError 崩溃，且崩溃发生在
consume-cny.json 已写盘之后，操作者拿到的是"证据已产出但脚本以 traceback 退出"的中间态。建议先判空并以明确信息退出，或改为按 expiration_at
排序动态推导最早/次早钱包（与 concurrent_consumption_86.py 的做法对齐），避免脚本只能跑在单一预置布局上。

-     m = [k for k in deltas if k.endswith("-2026-09")][0]
-     c = [k for k in deltas if "topup-c" in k][0]
-     d = [k for k in deltas if "topup-d" in k][0]
+     def pick(pred, what):
+         hits = [k for k in deltas if pred(k)]
+         if not hits:
+             print("FAIL: no wallet matches " + what + ":", sorted(deltas)); sys.exit(1)
+         return hits[0]
+ 
+     m = pick(lambda k: k.endswith("-2026-09"), "monthly wallet")
+     c = pick(lambda k: "topup-c" in k, "topup-c")
+     d = pick(lambda k: "topup-d" in k, "topup-d")


─── docs/plans/issue-72-flow-evidence-86/consume_86.py:60-61 ───
[bug · low] 余额快照未过滤钱包状态：同目录 concurrent_consumption_86.py 的 balances() 只统计 status == "active"
的钱包，reconcile.py 也把 "terminated 钱包绝不计入余额" 作为待验证不变量；此处 wallets() 返回全部钱包并对全量求和，若租户存在仍带 balance_cents 的
terminated 钱包，total_before/after 与 wait_until 的扣减断言都会被污染，甚至 ORDER VERDICT
的名字过滤会命中已终止的钱包。建议与兄弟脚本保持一致，过滤 active。

  def balance_snapshot():
-     return {name: w.get("balance_cents") for name, w in wallets().items()}
+     return {name: w.get("balance_cents")
+             for name, w in wallets().items()
+             if w.get("status") == "active"}


─── docs/plans/issue-72-flow-evidence-86/reconcile.py:80-81 ───
[bug · medium] 以 expires_at/expiration_at 作为字典键：当两个活跃钱包（或页面两行批次）共享同一到期时间时，后写条目会静默覆盖先写条目，"each active
wallet reconciles 1:1（无多余、无缺失钱包）"断言会漏检被覆盖的条目，对账结论失真。建议构建字典前显式检测键冲突并直接失败，而非静默丢弃。

-     page_batches = {b["expires_at"]: b for b in credits["batches"]}
+     page_batches = {}
+     for b in credits["batches"]:
+         if b["expires_at"] in page_batches:
+             raise SystemExit("duplicate page batch expiry: " + b["expires_at"])
+         page_batches[b["expires_at"]] = b
+     expiries = [w["expiration_at"] for w in active]
+     if len(expiries) != len(set(expiries)):
+         raise SystemExit("active wallets share an expiration_at")
      lago_active = {w["expiration_at"]: w for w in active}


─── docs/plans/issue-72-flow-evidence-86/reconcile.py:27-27 ───
[maintainability · low] HTTPError 导入后全文无任何引用，属死代码导入；若不打算在 lago_wallets() 中捕获 HTTP 错误，请删除该导入。

- from urllib.error import HTTPError
+ # 删除该行，仅保留实际使用的导入：
+ from urllib.request import Request, build_opener, HTTPRedirectHandler


─── docs/plans/issue-72-flow-evidence-86/weknora-account-api-final.json:50-51 ───
[documentation · low] 格式化版相对 weknora-account-api-final-raw.json 删除了
data.state、data.ensured_at、data.reason 与顶层 success 四个键，仅保留 data.benefits 并新增 source 注记。若为有意只呈现
benefits 子树，建议在 source 说明中注明 envelope 键已省略，或补齐这四个键，以免该文件被误当作接口完整响应形状的参照（raw 版保留了完整键集，证据链本身未断）。



─── docs/plans/issue-72-flow-evidence-86/weknora-account-api.json:1-1 ───
[documentation · low] 该初始快照的键集与序列化器实际输出不符：internal/handler/commercial.go 的 benefitsWire 在 plan
存在时恒定输出 benefits.features 与 benefits.limits，AccountStatus 恒定输出 data.reason 与顶层 success，且账户自 02:25:18
已 linked（data.ensured_at 必在）——而本文件仅有 benefits.credits/benefits.plan/data.state，缺少
benefits.features、benefits.limits、data.reason、data.ensured_at、success
五个键。可判定这是采集时手工裁剪的结果而非该时点的真实响应形状。作为 #86 验收对照证据，建议像 weknora-account-api-final-raw.json
一样另存未经修改的原始响应，避免后续按键集 diff 核对契约漂移时出现虚假的『新增键』。


