Review complete: 7 finding(s) across 39 selected item(s).

─── docs/plans/issue-72-flow-evidence-74/verify_db_watch.py:21-26 ───
[bug · medium] 两份 verify_db_watch.py（本文件与 issue-72-ocr1-replay 拷贝）解析同一个仓库级 runs/ 目录并取字典序最新的
db-watch-verify-*.tsv，且选中的 path 从不打印。两个证据目录对应两次不同 run（本目录 run 3dc51207…，decline 边界
2026-09-23T06:27:38；replay 目录 run dfcc76a8…，边界 08:10:34），而 db_watch.sh 每次运行生成独立时间戳 TSV、run_lab.py
不清理，多份必然共存。replay 之后在本目录重跑会用 replay 的 TSV（样本均 ≥08:0x）对照本目录 06:27:38 的边界：midrun() 全为 False，sub_a/b/c
全空 → ok_states_a/seen_active 为 False，DB-WATCH 输出 CHECK——这是文件选择错配而非业务回归，且无任何提示，反向场景（新 TSV
意外通过）同样不可辨。建议：优先使用本证据目录已归档的 verify-db-watch-samples.tsv，否则才回退 glob 并打印实际选中的 path；另可将 rows_for 的
`weknora-t02-` 前缀收紧为本目录 t02-decline.json 中的 `weknora-t02-<run_id>-`。

+ archived = EVID / "verify-db-watch-samples.tsv"
+ if archived.exists():
+     path = archived
+ else:
- matches = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))
+     matches = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))
- if not matches:
+     if not matches:
-     print(f"DB-WATCH: no observer TSV under {RUNS} "
+         print(f"DB-WATCH: no observer TSV under {RUNS} "
-           "(db-watch-verify-*.tsv; runs/ is git-ignored)")
+               "(db-watch-verify-*.tsv; runs/ is git-ignored)")
-     sys.exit(2)
+         sys.exit(2)
- path = matches[-1]
+     path = matches[-1]
+ print(f"DB-WATCH: using {path}")


─── docs/plans/issue-72-ocr1-replay/verify_db_watch.py:25-30 ───
[bug · medium] 与 issue-72-flow-evidence-74/verify_db_watch.py 相同的问题：两份拷贝解析同一个仓库级 runs/ 目录并取字典序最新的
db-watch-verify-*.tsv，且选中的 path 从不打印。本目录对应 replay run dfcc76a8…（边界 08:10:34），一旦 runs/ 里再出现更新的
TSV（后续任何重跑），本脚本会静默改用那次运行的数据对照本目录的 decline 边界——若新样本整体早于边界会被全部当作 mid-run、晚于则全部判为尾段，得出与本次 replay 无关的
PASS/CHECK 且无法溯源。建议：优先使用本目录已归档的 verify-db-watch-samples.tsv，否则才回退 glob 并打印实际选中的 path；rows_for 的
`weknora-t02-` 前缀可收紧为本目录 t02-decline.json 的 `weknora-t02-<run_id>-`。

+ archived = EVID / "verify-db-watch-samples.tsv"
+ if archived.exists():
+     path = archived
+ else:
- matches = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))
+     matches = sorted(glob.glob(str(RUNS / "db-watch-verify-*.tsv")))
- if not matches:
+     if not matches:
-     print(f"DB-WATCH: no observer TSV under {RUNS} "
+         print(f"DB-WATCH: no observer TSV under {RUNS} "
-           "(db-watch-verify-*.tsv; runs/ is git-ignored)")
+               "(db-watch-verify-*.tsv; runs/ is git-ignored)")
-     sys.exit(2)
+         sys.exit(2)
- path = matches[-1]
+     path = matches[-1]
+ print(f"DB-WATCH: using {path}")


─── deploy/lago-lab/payment-activation/clients.py:400-404 ───
[maintainability · low] 死变量：`last_error` 在整个重试循环中只写不读——重试耗尽路径直接 `raise` 重新抛出当前异常，不会用到先前保存的引用。建议删除
`last_error = None` 与此处的赋值，避免读者误以为存在未实现的错误上报路径。

-             except (URLError, OSError) as error:
-                 last_error = error
+             except (URLError, OSError):
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/phases.py:1109-1111 ───
[maintainability · low] 死变量：`asub` 解包后从未被使用（checks 只消费 `as_` 判定 `gate_not_activated`），建议用 `_`
占位，保持与代码库其他忽略值的写法一致。

-         as_, asub = _subscription_status_any(
+         as_, _ = _subscription_status_any(
              ctx, gate["subscription_external_id"], ("incomplete", "canceled")
          )


─── deploy/lago-lab/payment-activation/phases.py:630-632 ───
[maintainability · low] 死变量：稳定窗口循环中 `sub_now`（连带 `sb`）赋值后从未被读取，循环只依赖 `ss` 判断
404。重排缩进时沿袭了旧代码的无用赋值，建议清理。

-             ss, sb = _subscription_show(ctx, ext, "incomplete")
-             sub_now = (sb or {}).get("subscription", {}) if isinstance(sb, dict) else {}
+             ss, _sb = _subscription_show(ctx, ext, "incomplete")
              if ss == 404:


─── deploy/lago-lab/payment-activation/phases.py:1009-1010 ───
[test · medium] 假 PASS 风险：PASS 判定只依赖即时采集的 final 探测（同 lago_id、发票未变、仍一笔成功支付），而紧随其后的 note 自述"后续运行观察到
200 之后数分钟订阅被终止并开出续期发票"。即时探测窗口无法捕获该延迟破坏，AC2 的 exactly-once 门槛可能被虚假满足，而 expected 文本（"final
authoritative state byte-identical"）未向证据消费者披露此局限。建议在给出 PASS 前复用现有 stability_rounds/stability_delay
旋钮做一轮有界延迟复查（订阅仍 active、成功支付仍为 1 笔、无新增发票），或至少在 expected 文本中注明即时窗口的局限。

+         # Deferred-update guard: the 200-answered duplicate can terminate
+         # the subscription minutes later; re-check after a bounded delay
+         # before granting PASS.
+         if probes_harmless and state_intact:
+             time.sleep(ctx.stability_rounds * ctx.stability_delay)
+             _, sub_body3 = _subscription_show(ctx, sub_ext, "active")
+             sub3 = (sub_body3 or {}).get("subscription", {}) \
+                 if isinstance(sub_body3, dict) else {}
+             _, invoices3 = _invoices_for(ctx, customer["external_id"])
+             _, payments3 = _payments_for(ctx, customer["external_id"])
+             state_intact = state_intact and (
+                 sub3.get("lago_id") == base_lago_id
+                 and sub3.get("status") == "active"
+                 and len(_succeeded(payments3)) == 1
+                 and len(_gating_invoice(invoices3) and invoices3 or []) == len(invoices2)
+             )
          status = PASS if probes_harmless and state_intact else FAIL
          if _ok(rs):


─── deploy/lago-lab/payment-activation/phases.py:1105-1108 ───
[bug · medium] 重试探测空转风险：gate 阶段新增的"发票不可见仍 PASS"分支会发布 invoice_lago_id 为 None 的 gate 状态，而
phase_retries 无条件执行 `ctx.lago.post(f"/api/v1/invoices/{invoice_lago_id}/retry_payment",
{})`，于是探测打到字面量 URL
`/api/v1/invoices/None/retry_payment`。真实运行证据（docs/plans/issue-72-flow-evidence-74/t02-retries.json，r
un 3dc51207，对应 gate 阶段 invoice_api_visible=false）显示响应为 404 `invoice_not_found`——这是 id "None" 不存在导致的
404，并非本注释（及 expected 文本）所称"发票 API 不可见所以重试返回 404"。该路径下探测完全空转（invoice_payment_status 前后均为 null、支付计数
0==0 恒真），却仍以 `pending_gate_retry.http_status: 404` 记为 AC3 的重试证据并判 pass，证据是误导性的。建议在 phase_retries 中当
`gate["invoice_lago_id"]` 为 None 时跳过 retry POST，显式记录 not_applicable 原因（gate 发票 id 未知/从未 API 可见），仅保留
same-identity / 无新支付行 / gate 未激活三项检查。

-         # v1.53.0 endgame for the 3DS gate: the invoice turns closed (an
-         # INVISIBLE_STATUS), so the retry probe answers 404 and a "pending"
-         # gate state is not observable via the API. AC3's bar is that the
-         # retry creates no new payment row and the gate never activates.
+         invoice_lago_id = gate["invoice_lago_id"]
+         _, invoices = _invoices_for(ctx, customer_a["external_id"])
+         invoice = next(
+             (inv for inv in invoices if inv.get("lago_id") == invoice_lago_id),
+             None,
+         )
+         _, payments_before = _payments_for(ctx, customer_a["external_id"])
+         if invoice_lago_id is None:
+             # gate invoice never became API-visible: its lago_id is unknown,
+             # so the retry probe cannot address it. Record not_applicable
+             # instead of POSTing to "/api/v1/invoices/None/retry_payment"
+             # (that 404 is id-nonexistence, not invisibility).
+             rrs, rrb = None, {"code": "not_applicable",
+                               "detail": "gate invoice id unknown (API-invisible)"}
+         else:
+             rrs, rrb = ctx.lago.post(f"/api/v1/invoices/{invoice_lago_id}/retry_payment", {})

