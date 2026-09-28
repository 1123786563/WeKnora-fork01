# Issue #85 current readiness audit

Read time: 2026-09-29 (Asia/Shanghai), repository `1123786563/WeKnora-fork01`, worktree HEAD `cd6b0e521`.

## Sources and traversal

- GitHub API (authenticated `gh api`): Issue #72, formal `sub_issues` pagination; Issue #85 and its comments/formal children; dependency issues #75, #82, #83, and gate issues #84, #86, #87. No API errors observed. Issue #85 has zero comments and zero formal children.
- `docs/plans/2026-09-20-lago-billing-waves.md`: canonical planned DAG and wave schedule. It places #85 in W7 with #84, after #82/#83 and #75; #86 consumes #85, then #87 consumes #86.
- Requested `docs/plans/issue-72-dag.md` and `docs/plans/issue-72-execution-ledger.md` are absent at current HEAD (and not present in the worktree). This is an unresolved source gap, not evidence of an empty DAG/ledger.
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

Therefore #85 is not accepted-ready. The exact gaps are a dedicated paid-top-up flow against a real pinned Lago Community runtime, an unpaid control, exactly-once fulfillment proof, a distinct payment-success/fulfillment-pending product state, and response-loss replay proof binding Invoice + Payment + stable idempotency identity. The #75 a2 design decision remains a prerequisite for the expiry portion of #85/#86.

## Frontier assessment

- #85 is blocked by all three explicit blockers (#75, #82, #83); #82/#83 are themselves gated by #74/#81 and #74 currently has documented blocked runtime evidence in the parent plan.
- #84 is also not an independent ready alternative while payment gates continue; its issue dependency chain is payment activation and quote/invoice behavior.
- #86 is downstream of #85 and remains blocked; #87 is downstream of #86 and cannot be treated as ready.
- No independent non-live task in the #85/#86/#87 slice is shown ready by the current Issue graph. The only clearly completed prerequisite evidence is #75’s partial verdict and #80’s monthly-credit implementation, neither satisfies #85.

## Inference (clearly labeled)

The current code may provide reusable seams for implementing #85, but code presence is not acceptance evidence under Issue #72’s completion gate. A future #85 task could likely consume the existing fulfillment/adapter seams after the #75 a2 ruling and #82/#83 activation contracts are resolved; this is an implementation hypothesis, not a graph decision.

