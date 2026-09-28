# Issue #85 current readiness audit

> **Historical checkout scope:** This audit is a snapshot of checkout `cd6b0e521`. Its observation that `issue-72-dag.md` and `issue-72-execution-ledger.md` were absent applies only to that checkout and is not a statement about the current integration worktree. For current readiness, use [`issue-72-dag.md` §8](issue-72-dag.md#8-recovery-overlay--2026-09-28-supersedes-stale-readiness-snapshot-above) and [`issue-72-execution-ledger.md`](issue-72-execution-ledger.md). The `2026-09-20-lago-billing-waves.md` file cited below is the historical planning DAG, not the canonical current DAG.

Read time: 2026-09-29 (Asia/Shanghai), repository `1123786563/WeKnora-fork01`, worktree HEAD `cd6b0e521`.

## Sources and traversal

- GitHub API (authenticated `gh api`): Issue #72, formal `sub_issues` pagination; Issue #85 and its comments/formal children; dependency issues #75, #82, #83, and gate issues #84, #86, #87. No API errors observed. Issue #85 has zero comments and zero formal children.
- `docs/plans/2026-09-20-lago-billing-waves.md`: historical planning DAG and wave schedule. It places #85 in W7 with #84, after #82/#83 and #75; #86 consumes #85, then #87 consumes #86.
- At the audited checkout `cd6b0e521`, `docs/plans/issue-72-dag.md` and `docs/plans/issue-72-execution-ledger.md` were absent. This historical checkout-local absence is not evidence of an empty DAG/ledger; the current recovery overlay and ledger are linked in the scope note above.
- `docs/migrations/lago/t03-wallet-semantics/verdict.md` and evidence: #75 is closed but AC a2 (12-month top-up batch expiry under Lago's six-wallet cap) is explicitly BLOCKED; other wallet semantics are PASS-with-coordination.
- `docs/plans/ledgers/lago-80.md` and `docs/migrations/lago/t08-base-plan/README.md`: #80 evidence is complete for monthly included credits and identity replay, but is not #85 paid top-up evidence.
- Current code contains generic top-up fulfillment and refund primitives (`internal/modules/commercial/service/commercial/fulfillment.go`, `refund.go`) and #82/#86 related code, but no issue-#85-specific acceptance/evidence record was found.

Formal containment traversal from #72: children #73 through #105 (33 unique nodes, one page in the observed API response); #85 has no formal descendants. Text `Parent` links were not treated as child edges. The relevant dependency edges are:

```
#75 -> #85   (wallet/batch semantics; explicit Blocked by)
#82 -> #85   (payment activation; explicit Blocked by)
#83 -> #85   (WeChat activation; explicit Blocked by)
#85 -> #86 -> #87   (planned downstream consumption/admission chain)
```

No cycle or missing referenced Issue number was observed in these edges. The parent plan also records #74 as a transitive gate for #82/#83 and an unresolved Stripe/Premium runtime decision.

## Issue #85 status and uncovered acceptance

GitHub Issue #85 is **open**, label `ready-for-agent`, updated 2026-09-20. Its acceptance is:

1. unpaid Wallet transaction does not increase spendable Credits;
2. successful payment credits exactly once and displays source and expiry;
3. payment succeeded but fulfillment unconfirmed displays “权益处理中”;
4. response loss reuses original Invoice, Payment, and idempotency identity.

None of these four criteria has a direct #85 implementation ledger, real-stack evidence, or issue comment in the current repository/API snapshot. Existing #80 evidence covers monthly included grants, not paid top-up payment fulfillment. Existing #75 evidence covers wallet mechanics, but its twelve-month top-up expiry criterion is blocked and does not establish #85’s payment/recovery flow. Existing #82 work demonstrates purchase/activation infrastructure and code paths, but does not prove a paid Wallet top-up’s pending/confirmed/replay states.

Therefore #85 is not accepted-ready. The exact gaps are a dedicated paid-top-up flow against a real pinned Lago Community runtime, an unpaid control, exactly-once fulfillment proof, a distinct payment-success/fulfillment-pending product state, and response-loss replay proof binding Invoice + Payment + stable idempotency identity. The #75 a2 Option B design ruling is approved (ADR-0012 §“2026-09-23…75-a2 裁决”; [R-2](issue-72-user-rulings.md#r-2--2026-09-23--75-a2-充值批次并发模型--选项-b-协调层承载)); runtime proof of the twelve-month expiry behavior remains outstanding and gates the expiry evidence for #85/#86 ([T03 verdict](../migrations/lago/t03-wallet-semantics/verdict.md)).

## Frontier assessment

- In the audited `cd6b0e521` snapshot, #85 had the explicit blockers #75, #82, and #83. The current recovery overlay records later completion evidence for #74 and #81; #82 code is integrated but acceptance remains unverified; #83 remains unverified; and #75 a2 runtime expiry evidence remains blocked despite the approved design ruling. Sources: [DAG §8](issue-72-dag.md#8-recovery-overlay--2026-09-28-supersedes-stale-readiness-snapshot-above), [execution ledger #82 ancestry correction](issue-72-execution-ledger.md#82-ancestry-correction-and-87-owner-rule-options-2026-09-28), and [execution ledger #85 readiness refresh](issue-72-execution-ledger.md#85-readiness-refresh-2026-09-29). None of these statuses alone clears #85.
- #84 is also not an independent ready alternative while payment gates continue; its issue dependency chain is payment activation and quote/invoice behavior.
- In the audited `cd6b0e521` snapshot, #86 was downstream of #85. Currently, #86 checkpoint review/verification is the active gate, while #86 issue-level acceptance remains unverified and #87 remains gated ([DAG §8](issue-72-dag.md#8-recovery-overlay--2026-09-28-supersedes-stale-readiness-snapshot-above); [execution ledger #85 readiness refresh](issue-72-execution-ledger.md#85-readiness-refresh-2026-09-29)).
- This historical audit did not establish a ready independent task in the #85/#86/#87 slice. The current recovery overlay says no implementation task is verified-ready and identifies #86 checkpoint review/verification as the active gate ([DAG §8](issue-72-dag.md#8-recovery-overlay--2026-09-28-supersedes-stale-readiness-snapshot-above)). #75 a2 runtime expiry evidence remains blocked despite the approved design ruling, and #80 monthly-credit evidence does not satisfy #85 ([execution ledger #85 readiness refresh](issue-72-execution-ledger.md#85-readiness-refresh-2026-09-29)).

## Inference (clearly labeled)

The current code may provide reusable seams for implementing #85, but code presence is not acceptance evidence under Issue #72’s completion gate. A future #85 task could likely consume the existing fulfillment/adapter seams under the approved #75 a2 Option B model, after the required runtime expiry verification and #82/#83 activation contracts are resolved; this is an implementation hypothesis, not a graph decision ([ADR-0012 §“2026-09-23…75-a2 裁决”](../adr/0012-lago-as-commercial-billing-authority.md#2026-09-23澄清充值批次top-up的钱包写入时机与并发模型75-a2-裁决); [T03 verdict](../migrations/lago/t03-wallet-semantics/verdict.md); [DAG §8](issue-72-dag.md#8-recovery-overlay--2026-09-28-supersedes-stale-readiness-snapshot-above)).
