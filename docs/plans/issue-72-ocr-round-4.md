Review complete: 13 finding(s) across 63 selected item(s).

─── deploy/lago-lab/payment-activation/clients.py:400-404 ───
[maintainability · low] last_error 是死存储：赋值（含循环前的 last_error = None）后从未被读取，最终失败路径直接 raise
当前异常。死变量会误导读者以为存在错误聚合或日志用途，建议删除（如需在重试耗尽时保留最后错误上下文，应改为 raise 前附加使用）。

-             except (URLError, OSError) as error:
-                 last_error = error
+             except (URLError, OSError):
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/evidence/t02-duplicates.json:6-13 ───
[documentation · medium] baseline 缺少 phases.py:966 生成的 invoice_count 键，observed 亦缺少 phases.py:1073
生成的 deferred_recheck 键——本快照生成于 ocr-2（延迟复查）与 ocr-3（pre-probe 发票计数锚定）修复落地之前，与 ocr2/ocr3-replay
副本（148/150 行）的键集已漂移。最新 verify_ac_assertions.py:76-83 断言 deferred_recheck.checked/ok 为真且
evidence.baseline.invoice_count == deferred_recheck.invoice_count，对本文件会因键缺失直接判 fail。作为 AC2
的权威证据目录，应与最终 phases.py 输出对齐，建议重跑刷新本快照。



─── deploy/lago-lab/payment-activation/evidence/t02-retries.json:103-111 ───
[documentation · medium] 该权威证据快照与同分支最终版 phases.py 的输出键集不一致（OCR-2/OCR-3 修复前生成）：phase_retries 现在无条件写入
pending_gate_retry.subscription_status_after（phases.py:1214），且 gate 发票 API 不可见时跳过探针、记录
http_status=null 与 code=not_applicable（phases.py:1162-1179），仅当探针实际执行才追加
checks.no_succeeded_payment_for_gate（phases.py:1224-1229）。本快照仍为旧路径（http_status=404 +
invoice_not_found + 含 no_succeeded_payment_for_gate + 缺 subscription_status_after），与
docs/plans/issue-72-ocr3-replay/t02-retries.json 不一致；最新 verify_ac_assertions.py 的 AC3
断言（gr.get("http_status") is None、code=="not_applicable"、subscription_status_after in (incomplete,
canceled)）对本文件全部失配。建议用当前代码重跑并刷新 evidence/ 快照。



─── deploy/lago-lab/payment-activation/phases.py:1297-1300 ───
[bug · medium] 轮询耗尽（not settled）后的最终复查只探测 status=incomplete，存在竞态误报：若订阅在最后一次 canceled 探测与这次复查之间（中间还隔着
invoices/entitlements/payments 三次调用）恰好转入 canceled(payment_failed)——这正是异步 charge-failure 处理的合法终局，gate
阶段对客户 A 的同款 3DS 卡片就显式处理了该终局——复查会得到 404，still_incomplete=False，被误判 FAIL，且错误信息 "failed charge neither
canceled nor held incomplete" 在根本未探测 canceled 的情况下断言"未取消"，与事实不符。本 diff 新增的 _subscription_status_any
正是为"不得预设状态"的探测场景而写，这里应同时探测两种状态并对 canceled(payment_failed) 按 settled 分支的契约评估。

-             is_status, ib = _subscription_show(ctx, ext, "incomplete")
+             is_status, ib = _subscription_status_any(ctx, ext, ("incomplete", "canceled"))
+             sub_end = (ib.get("subscription") or {}) if isinstance(ib, dict) else {}
+             if sub_end.get("status") == "canceled":
+                 # 竞态：轮询耗尽后订阅恰好转入 canceled —— 按 settled 分支契约评估
+                 observed["cancellation_reason"] = sub_end.get("cancellation_reason")
+                 checks = {
+                     "payment_failed_reason": sub_end.get("cancellation_reason") == "payment_failed",
+                     "no_succeeded_payment": observed["payments_succeeded_count"] == 0,
+                     "entitlements_unusable": es == 404,
+                 }
+                 observed["checks"] = checks
+                 return _report(ctx, "decline_control", expected, observed,
+                                PASS if all(checks.values()) else FAIL)
              still_incomplete = _ok(is_status) and isinstance(ib, dict) and (
-                 (ib.get("subscription") or {}).get("status") == "incomplete"
+                 sub_end.get("status") == "incomplete"
              )


─── deploy/lago-lab/payment-activation/phases.py:1215-1219 ───
[test · medium] AC3 的响应丢失模拟对客户 B 订阅执行的重复注册 POST（同 external_id，1134-1137 行）与 duplicates 探针 1
是完全相同的操作，而本 diff 刚在 duplicates 中为它补上了延迟复查（注释自己记录过：200 应答的重复注册曾在此后数分钟终止订阅并开具新续期发票）。retries 的 checks
只覆盖即时窗口（subscription_count==1、same_lago_id），没有任何延迟复查；且本阶段恰好运行在 decline_control（约 300s 轮询）与 cleanup
之前——若延迟漂移落在这段窗口内：要么表现为误归因的 cleanup 失败（sub-b 不再是 active，三个按状态的 DELETE 探针全部 404），要么被 cleanup
完全掩盖（对替代的新 active 订阅 DELETE ?status=active 返回 200）。一次真实的 exactly-once 违例可能出现在全绿的运行报告中，违背模块文档声明的
"pass only on full authoritative final state"。建议在 recovery 探针后镜像 duplicates 的延迟复查，不一致则 FAIL。

+         # Deferred re-check (mirror phase_duplicates): the re-POST above is
+         # the same duplicate-registration probe; a 200 answer has once
+         # terminated the subscription and issued a renewal invoice minutes
+         # later, which the immediate window cannot see.
+         time.sleep(ctx.stability_rounds * ctx.stability_delay)
+         _, dsub_body = _subscription_show(ctx, sub_ext, "active")
+         dsub = (dsub_body or {}).get("subscription", {}) if isinstance(dsub_body, dict) else {}
+         _, dinvoices = _invoices_for(ctx, customer_b["external_id"])
+         _, dpayments = _payments_for(ctx, customer_b["external_id"])
+         observed["deferred_recheck"] = {
+             "subscription_same_lago_id": dsub.get("lago_id") == known_lago_id,
+             "invoice_count": len(dinvoices),
+             "payments_succeeded_count": len(_succeeded(dpayments)),
+         }
          checks = {
              "same_identity_recovered": (
                  recovery["subscription_count"] == 1 and recovery["same_lago_id"]
              ),
              "gate_not_activated": _ok(as_),
+             "duplicate_no_deferred_drift": (
+                 dsub.get("lago_id") == known_lago_id
+                 and observed["deferred_recheck"]["payments_succeeded_count"] == 1
+             ),


─── deploy/lago-lab/payment-activation/phases.py:254-258 ───
[maintainability · low] 传输错误被重复记录：`_subscription_show` 在 OSError 时已经写入逐字相同的 note 并 re-raise（239-241
行），这里的再次 ctx.note 会让每次传输失败在 blocked 报告的 contract_notes 中出现两条完全相同的条目。建议去掉本层 note，直接让异常穿透（或仅 `except
OSError: raise`）。

          try:
              ss, sb = _subscription_show(ctx, external_id, status)
-         except OSError as error:
-             ctx.note(f"transport error reading subscription {status}: {error.__class__.__name__}")
-             raise
+         except OSError:
+             raise  # _subscription_show already notes the transport error


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:64-73 ───
[bug · low] max_succeeded() 与 rows_for() 的隔离策略不一致：rows_for 通过 run_prefix 过滤并在注释中明确防御多 run TSV
串扰，且只统计 midrun 边界前的样本；但 max_succeeded() 遍历全部采样行，既不按本 run 的前缀过滤，也不应用 midrun 边界。TSV 中 payments[...]
是全库状态聚合（无 run 键），一旦在归档缺失时回退到 runs/ 下混有其他 replay 样本的多 run TSV（回退分支是本脚本明确支持的路径），外来 run 的 succeeded
计数会污染 `max_succeeded() == 1` 断言，对本次 replay 产生误报 CHECK。虽然本目录归档的 verify-db-watch-samples.tsv 仅含单一
run（当前执行不受影响），仍建议补上与 rows_for 一致的过滤，保持两条断言的隔离语义对称。

  def max_succeeded():
      best = 0
      for ln in lines:
+         if not midrun(ln) or run_prefix not in ln:
+             continue
          m = re.search(r"payments\[([^\]]*)\]", ln)
          if not m:
              continue
          for part in m.group(1).split(";"):
              if part.startswith("succeeded|"):
                  best = max(best, int(part.split("|")[1]))
      return best


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:26-27 ───
[documentation · low] 该注释是 ocr-2 副本的复制残留：本文件位于 issue-72-ocr1-replay/ 目录，"(ocr-2)" 指代的是另一个 replay
目录，会误导后续复核者对归档优先策略归属与实际取数路径的理解。另外模块 docstring 声称本副本 "executed against ... and the runs/ observer TSV
of the replay"，而代码在本目录存在 verify-db-watch-samples.tsv 时会优先使用归档文件（且该文件实际存在于本目录），docstring
描述与代码行为不一致，建议一并修正措辞。

- # Prefer THIS evidence directory's archived copy (ocr-2): replayed observer
+ # Prefer THIS evidence directory's archived copy: replayed observer
  # TSVs land in the repo-level runs/ dir and db_watch.sh writes a fresh


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:111-111 ───
[maintainability · low] 摘要行把 `len(lines)`(TSV 全部行数)放在 "samples ... (mid-run before ...Z)"
的语境下打印,读者会将其理解为 mid-run 样本数,但所有断言实际只消费 `midrun(ln)`
过滤后的子集,两个数字并不相等。以本目录归档数据实测:verify-db-watch-samples.tsv 共 516 行、其中边界(2026-09-23T08:10:34)前 478
行,脚本会打印 `samples: 516`;而已归档的 verify-db-watch-output.txt 记录的是 `samples: 319`,与当前归档 TSV
对不上(疑似输出录制与归档样本来自不同文件)。建议改为打印真实的 mid-run 计数(如 `sum(midrun(ln) for ln in
lines)`),使摘要与断言基数一致,也便于审计者发现归档 TSV 与输出记录错配的问题。

- print(f"samples: {len(lines)} (mid-run before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")
+ midrun_count = sum(1 for ln in lines if midrun(ln))
+ print(f"samples: {len(lines)} total / {midrun_count} mid-run (before {boundary}Z)  max_succeeded_payments: {max_succeeded()}")


─── docs/plans/issue-72-ocr2-replay/verify_ac_assertions.py:10-11 ───
[documentation · low] docstring 声称 "unmodified assertion script"，但与 ocr1
副本（docs/plans/issue-72-ocr1-replay/verify_ac_assertions.py，87 行）对比，本副本（97 行）新增了两处带 "# ocr-2:"
标注的断言块：AC2 的 deferred_recheck 检查（dr.get("checked") is True and dr.get("ok") is True）与 AC3 的 gate
重试探针 not_applicable 检查。来源声明与文件实际内容自相矛盾，会误导后续审计者对该脚本各轮回放间的可比性判断（新增断言读取的 deferred_recheck 字段已确认存在于本目录
t02-duplicates.json，运行时不致 FAIL，问题仅在声明失实）。建议改为如实描述相对前轮的增量。

- ocr-2 replay copy: unmodified assertion script, executed against the
- docs/plans/issue-72-ocr2-replay/ evidence directory.
+ ocr-2 replay copy: extends the ocr-1 assertion script with the AC2
+ duplicates deferred re-check and the AC3 gate-retry not_applicable probe;
+ executed against the docs/plans/issue-72-ocr2-replay/ evidence directory.


─── docs/plans/issue-72-ocr2-replay/verify_db_watch.py:32-35 ───
[other · medium] 归档优先分支依赖脚本同目录的 verify-db-watch-samples.tsv，但该文件未出现在本次变更清单中：本目录其余 12 个文件（全部
t02-*.json 与两个校验脚本）均为本次 ADDED，而归档 TSV 与 verify-db-watch-output.txt 均不在列；回退目录 runs/ 又被 .gitignore
忽略（根 .gitignore 无任何 tsv 规则，并非被忽略所致，更像遗漏 git add）。若该 TSV 未被先前提交跟踪，全新检出的仓库中此优先分支不可达，脚本只能 exit
2（MISSING-EVIDENCE）或退回 runs/ 下可能来自外部 run 的 TSV，本次提交的证据链将无法在仓库内复现 DB-WATCH PASS 结果——这恰好抵消了 OCR2-01
修复（"归档优先 + 可溯源"）在仓库层面的价值。建议随本目录一并提交该归档 TSV（如确认已在先前提交中跟踪，请在文件头注明其 commit 来源以便溯源）。

+ # NOTE: the archived TSV this branch prefers must be committed alongside
+ # this script (git add verify-db-watch-samples.tsv); runs/ fallback is
+ # git-ignored, so without the archive a fresh clone cannot reproduce PASS.
  archived = EVID / "verify-db-watch-samples.tsv"
- matches = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))
- if archived.exists():
-     path = str(archived)


─── docs/plans/issue-72-ocr3-replay/verify_db_watch.py:64-67 ───
[bug · medium] max_succeeded() 与 rows_for() 的隔离级别不一致：rows_for() 在本迭代专门加入了 run_prefix 过滤以防御"多运行 TSV /
旧回放残留泄漏外来状态"（脚本自身注释所述场景），但 max_succeeded() 既不受 midrun 边界限制，也没有任何运行隔离——而观察者对 payments
的采样是全局计数（db_watch.sh: SELECT status, count(*) FROM payments GROUP BY status，不按 external_id 键控）。因此当走
runs/ 回退路径（本目录无归档 TSV 时取字典序最新的 db-watch-verify-*.tsv，可能属于另一次运行）或共享 lab DB 中残留早前回放的 succeeded
付款时，外来运行的付款计数会泄漏进 `max_succeeded() == 1` 这一 exactly-once 不变量，导致误报 CHECK 失败。至少应给 max_succeeded 加上与
rows_for 相同的 only_midrun 过滤保持一致；对于无法按 run 前缀隔离的全局计数，回退到外来 runs/ TSV 时应对该不变量显式告警或降级（如退出码 2
MISSING-EVIDENCE），避免用不可隔离的数据源支撑结论。

- def max_succeeded():
+ def max_succeeded(only_midrun=True):
      best = 0
      for ln in lines:
+         if only_midrun and not midrun(ln):
+             continue
          m = re.search(r"payments\[([^\]]*)\]", ln)


─── docs/plans/issue-72-ocr3-replay/verify_ac_assertions.py:69-71 ───
[test · low] 脚本对证据文件的直接下标访问（如
env["run"]["overall"]、d["observed"]["final"]、d["evidence"]["baseline"]、oa["payments_succeeded_count"
]、r["evidence"]["responses"]["retry_payment"]）与 check() 收集具名 FAIL 行的设计不一致：一旦某次重放的证据文件出现键缺失或 schema
漂移，脚本会以 KeyError traceback 崩溃，跳过其后所有 AC 断言，且失败原因不表现为具名 FAIL 条目。已核对当前 ocr-3
证据文件中这些键路径全部存在，本次运行不受影响，故仅作低危健壮性建议：对跨文件取值统一改用 .get() 并将缺键纳入具名 check（或对 load+索引做异常包裹转 FAIL），确保每个 AC
的失败都能被点名输出。


