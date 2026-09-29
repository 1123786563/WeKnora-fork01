# Final scoped re-review — T9 concurrent PI guard F1

**Reviewed fix:** `84cc3625156b89d2e8dacbd754137c5113d1f224..88c1396e3636a37614b385dd8ca9399ef26f3858`, using the supplied diff package once, the refreshed Task 1 brief and report, and the prior final review. This review is limited to final-review finding F1. No Git command, test, live service, database, Docker, or network call was run.

## F1 — ADDRESSED for the specified interleaving

The pre-settle response's single `data` slice feeds both `paymentIntentIDSet` and `preSettlePaymentIntentCandidates` (`internal/modules/commercial/commercialplatform/lago_settlement_integration_test.go:444-448`). `paymentIntentIDSet` retains every nonempty ID without filtering by status or invoice metadata (`:273-282`). The selected latest candidate is resolved from that same response before the command (`:453-466`).

After the command, `succeededPaymentIntent` scans all rows in the returned post-settle slice, collects every succeeded invoice-linked ID absent from the pre-settle set, and returns an error before returning the expected PI (`lago_settlement_integration_test.go:241-270`). The call site invokes `t.Fatal` on that error before constructing or delivering the synthetic event (`:483-503`). Thus, when the previously predicted PI and a newly appeared invoice-linked PI both succeed and are present in the post-settle response, the old success cannot mask the new identity.

The regression models that interleaving and checks that the error names `pi_interleaved` (`lago_settlement_integration_test.go:97-102`). It also keeps the requested controls: a pre-observed historical success remains allowed, while a newly listed success without invoice metadata is ignored by this invoice-identity guard (`:104-115`). The exact expected-ID and nonempty invoice metadata checks remain in the selector (`:259-270`).

## New breakage

None found in the supplied repair diff for F1. The changed behavior is confined to the `lago_integration` test harness (`lago_settlement_integration_test.go:1,241-282,430-503`); the task's durable records retain the separate live T9 and AC4 residuals (`docs/plans/issue-72-ledger-82.md:257-261`).

## Out-of-scope observations and evidence limits

- The guard examines all **returned** post-settle rows, not every PI that might exist outside the fixture's bounded `limit=10` list (`lago_settlement_integration_test.go:483-489`). It also detects newly appeared invoice-linked PIs only after they have succeeded (`:245-265`). This re-review establishes the approved F1 repair case, not a universal proof of command target identity under every provider interleaving.
- The task review's Low coverage suggestion remains deferred: the pure test constructs `preObservedIDs` by hand, while the live fixture uses `paymentIntentIDSet` on the response (`lago_settlement_integration_test.go:83,273-282,446`; `docs/plans/issue-72-execution-ledger.md:373`). This is non-blocking for the current F1 behavior.
- The prior Low checkpoint-record issue is resolved by the exact implementation SHA `26215a3bd7b7baf568628c4c509d7b409140f5e3` in the execution ledger (`docs/plans/issue-72-execution-ledger.md:371`). The implementer disclosed that the prescribed pre-change RED run was not captured, while reporting focused tagged, package, tagged compile-only, formatting, and diff checks as passing (`.superpowers/sdd/issue-72-plan-82-t9-concurrent-pi-fail-closed-r1/task-1-report.md:11-24`); I did not rerun them.
- Dedicated live T9 and real Alipay sandbox AC4 acceptance remain incomplete (`docs/plans/issue-72-ledger-82.md:255,260-261`). They are separate from this F1 code verdict.

## Scoped verdict

**F1 ADDRESSED. Spec compliance: PASS for the approved concurrent-success guard. Code quality: PASS for this scoped fix, with the deferred Low coverage item. Ready to proceed with the outer review gate: yes.** The result applies to newly appeared succeeded invoice-linked IDs present in the post-settle response (`lago_settlement_integration_test.go:241-270,483-503`); it does not certify live T9 or AC4 acceptance (`docs/plans/issue-72-ledger-82.md:257-261`).
