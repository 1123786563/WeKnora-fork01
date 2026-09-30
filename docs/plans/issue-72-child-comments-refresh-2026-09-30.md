# Issue #72 child issue comment refresh (2026-09-30)

## Source and coverage

Authenticated GitHub REST API responses were captured at `2026-09-29T18:57:20Z` for repository `1123786563/WeKnora-fork01`. The capture contains one paginated comments response per current direct child, #73–#105. The 33 `.json` files in `/tmp/issue72-refresh-20260930/comments-current/` each parse as a JSON array; together they contain eight comment objects on #73–#80 and empty arrays for #81–#105. The unrelated `96.stderr` capture is not a JSON response and is excluded.

The manifest orders files by ascending numeric issue number, #73–#105. Each line is `<filename> SHA256(file bytes)`; lines are joined with `\n` and there is no trailing newline. Its SHA-256 is `1010e2d6490d406e71e86fbc95f85ad62cbb29aea2498beeccb49e52eb64ee21`.

## Non-empty comments

| Issue | Comment (date, ID) | Summary |
|---|---|---|
| #73 | [2026-09-20, 5751302341](https://github.com/1123786563/WeKnora-fork01/issues/73#issuecomment-5751302341) | Reports the pinned Lago Community environment, health classification, contract probe and OpenMeter isolation complete; records the cleanup follow-up later handled before #77. |
| #74 | [2026-09-20, 5752347974](https://github.com/1123786563/WeKnora-fork01/issues/74#issuecomment-5752347974) | Reports offline/setup evidence and a then-blocked Stripe TEST environment for AC1–AC3, with manual payment Premium gating and provider allowlist constraints. This is historical: later #74 evidence records the payment activation run as passing, and R-1 records the supported-provider path. GitHub #74 remains **OPEN** in the 2026-09-30 tree refresh; this comment does not change remote status or readiness. See [#74 Ledger](issue-72-ledger-74.md) and [user rulings](issue-72-user-rulings.md#r-1--2026-09-23--t02-付款激活通道--选项-受支持-provider). |
| #75 | [2026-09-20, 5752347053](https://github.com/1123786563/WeKnora-fork01/issues/75#issuecomment-5752347053) | Reports wallet tests and an apparent six-active-wallet limit, leaving batch-expiry choices unresolved. This is superseded by the Sep 23 user-confirmed R-2 option B: the coordination layer owns batch state and Lago wallet transactions are created idempotently on confirmed payment, without a product concurrency cap. GitHub #75 remains **CLOSED** in the 2026-09-30 tree refresh. See [R-2](issue-72-user-rulings.md#r-2--2026-09-23--75-a2-充值批次并发模型--选项-b-协调层承载) and [ADR-0012](../adr/0012-lago-as-commercial-billing-authority.md). |
| #76 | [2026-09-20, 5752347322](https://github.com/1123786563/WeKnora-fork01/issues/76#issuecomment-5752347322) | Reports pricing-group acceptance and measured throughput; notes contract findings handed to #87/#88. |
| #77 | [2026-09-20, 5752347657](https://github.com/1123786563/WeKnora-fork01/issues/77#issuecomment-5752347657) | Reports the provider-neutral seam and contract checks complete, including the #73 cleanup follow-up; #78/#79 are noted as follow-on work. |
| #78 | [2026-09-20, 5752848525](https://github.com/1123786563/WeKnora-fork01/issues/78#issuecomment-5752848525) | Reports customer identity, tenant isolation, idempotency and recovery evidence; records a PostgreSQL twin environment limitation. |
| #79 | [2026-09-20, 5752849001](https://github.com/1123786563/WeKnora-fork01/issues/79#issuecomment-5752849001) | Reports plan publication validation, idempotency and immutability evidence, with follow-up minor items. |
| #80 | [2026-09-20, 5753437195](https://github.com/1123786563/WeKnora-fork01/issues/80#issuecomment-5753437195) | Reports Base Plan initialization, entitlement/limit enforcement and tenant-isolation evidence; notes `concurrent_tasks` and manual/passage guard follow-ups. |

## Containment, dependencies, and current gates

The current tree refresh records #73–#105 as the 33 direct children of #72, with no grandchildren. These eight comments report evidence, findings and handoffs; none asserts a new native child or establishes a prerequisite absent from the current DAG. References such as #87/#88 in #76 and #78/#79 in #77 remain ordinary cross-references/handoffs, not dependency evidence by themselves. The existing dependency candidate is unchanged.

This archive does not change the current DAG readiness, #82/#86/#87 acceptance gates, or any GitHub status. The refreshed remote state is #74 OPEN and #75 CLOSED; local acceptance/readiness judgments remain those recorded in the current DAG and execution records. See the [live tree refresh](issue-72-live-tree-refresh-2026-09-30.md), [execution ledger](issue-72-execution-ledger.md), [#74 Ledger](issue-72-ledger-74.md), and [#82 Ledger](issue-72-ledger-82.md).
