Review complete: 5 finding(s) across 51 selected item(s).

─── deploy/lago-lab/payment-activation/phases.py:565-569 ───
[test · medium] phase_gate 发票不可见分支的 PASS 判定只要求 `_ok(cs2)`（存在 canceled 订阅即通过），未像发票可见为 failed 的分支那样校验
`reason == "payment_failed"`，也未将 `reason` 写入 observed。两个分支判定严格度不一致：若订阅因无关原因（如 other/revoked）被取消，AC1
会得到误导性的 PASS，且证据 JSON 中缺失 cancellation_reason 字段，与 expected 文本中 "ends canceled(payment_failed)"
的表述不符。

              if core_ok:
                  cs2, cb2 = _subscription_show(ctx, ext, "canceled")
-                 if _ok(cs2):
-                     reason = ((cb2 or {}).get("subscription") or {}).get("cancellation_reason")
+                 canceled = _ok(cs2)
+                 reason = ((cb2 or {}).get("subscription") or {}).get("cancellation_reason") \
+                     if canceled else None
+                 observed["cancellation_reason"] = reason
+                 if canceled and reason == "payment_failed":
                      ctx.note(


─── deploy/lago-lab/payment-activation/phases.py:1181-1183 ───
[maintainability · low] `asub` 解包后从未被使用：`checks` 只引用 `as_`（HTTP 状态），`observed`/`gate_retry`
均未记录重查后的订阅实际状态。既是死变量，也丢失了本可入证据的观测值——AC3 的 "gate never activates" 断言只看 `_ok(as_)`，事后从证据 JSON 无法得知
gate 最终停在 incomplete 还是 canceled。建议将状态记入 gate_retry。

          as_, asub = _subscription_status_any(
              ctx, gate["subscription_external_id"], ("incomplete", "canceled")
          )
+         gate_retry["subscription_status_after"] = asub.get("status")


─── deploy/lago-lab/payment-activation/clients.py:400-404 ───
[maintainability · low] `last_error` 是死存储：在 except 分支被赋值但从未被读取（最后一轮直接 `raise` 当前的
`error`），循环外也没有使用点。读者会误以为它在循环外参与错误报告，建议删除。

              except (URLError, OSError) as error:
-                 last_error = error
                  if attempt == transport_retries:
                      raise
                  time.sleep(1.5 + attempt)


─── deploy/lago-lab/payment-activation/phases.py:143-145 ───
[maintainability · low] 组织 id 解析失败被 `except ...: pass` 完全吞掉：既不缓存失败结果（下次 graphql() 会再次重发 GET
/api/v1/organizations），也不留任何 note。当解析失败而服务端又强制要求 x-lago-organization 头时，GraphQL 会以 400 "Missing
organization id" 被拒，阶段判为 FAIL 而非 blocked-env，且证据中无任何线索指向缺失的头，排障困难。建议至少在失败时记录一条 note（可一并缓存哨兵值避免重复解析）。

-             except (OSError, clients.OffOriginRedirect):
-                 pass
+             except (OSError, clients.OffOriginRedirect) as error:
+                 ctx.note(
+                     "x-lago-organization resolution failed "
+                     f"({error.__class__.__name__}); header omitted"
+                 )
          return self._graphql_organization_id


─── deploy/lago-lab/payment-activation/phases.py:1029-1030 ───
[test · low] deferred 复查的 "no new invoice" 断言锚点选错了基准：`invoice_count` 只与即时窗口的
`invoice_count_immediate`（探测后数秒内读取的 invoices2）比较，而探测前的 baseline 从未记录发票数量，即时窗口的
`final`/`state_intact` 也不校验发票数。若 200 应答的重复注册在即时探测之前就同步产生了新发票、且订阅身份未变（same lago_id、仍 active、仍恰好 1 笔
succeeded payment、原发票字段不变——例如运行时把重复注册按"更新"处理并补开一张 advance 发票），该重复商业对象会同时逃过即时断言与延迟断言，得到误导性 PASS，与
expected 文本及注释承诺的 "no new invoice" 契约不符。建议在 baseline 中记录 `"invoice_count": len(invoices)`，并让
`deferred["ok"]` 与 baseline 比较（即时计数可一并保留入证据）。

                  "invoice_count": len(invoices3),
+                 "invoice_count_baseline": baseline["invoice_count"],
                  "invoice_count_immediate": len(invoices2),
+             }
+             deferred["ok"] = (
+                 ...
+                 and deferred["invoice_count"] == baseline["invoice_count"]
+                 ...
+             )
+             # baseline 侧（探测前）：
+             # baseline["invoice_count"] = len(invoices)


LLM retry report summary: 1 of 22 requests affected -- 1 request recovered after retry

Core review (1 request):
- deploy/lago-lab/payment-activation/clients.py,deploy/lago-lab/payment-activation/fixtures.py,deploy/lago-lab/payment-activation/phases.py,deploy/lago-lab/payment-activation/run_lab.py: network error -> network error -> network error -> timed out -> network error -> succeeded

Per-attempt detail: --format json (retry_report).
