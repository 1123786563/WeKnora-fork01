# Task 1 independent review — concurrent PaymentIntent guard

**Reviewed range:** `1ad04526b769b88980dbb4405ce1ff8e4ec819c1..26215a3bd7b7baf568628c4c509d7b409140f5e3` using the supplied diff package once, plus the task brief, implementer report, approved billing spec, Lago ADR, `CONTEXT.md`, prior fixture review F1, and the changed source. This was a read-only code review; no test or live service was run.

## Verdict

- **Spec Compliance: PASS for the approved Task 1 scope.** The tagged fixture records all nonempty IDs from the exact pre-settle `data` slice used for candidate resolution (`lago_settlement_integration_test.go:444-448`), scans every returned post-settle row before selection (`:241-268`), and rejects a newly appeared succeeded invoice-linked ID before webhook construction (`:489-501`). Exact expected ID, succeeded status, and nonempty invoice metadata remain required. The code change is confined to the integration-tagged test file; live T9 and AC4 remain explicitly open in the durable records.
- **Task Quality: PASS with two Low findings.** The core race regression and both requested controls are present. The report records focused tagged, package, tagged compile-only, formatting, and diff checks at the implementation checkpoint. The missing pre-change RED run is disclosed accurately; it is a process deviation, not behavioral proof.

## Strengths

- The guard scans all rows and delays returning the expected PI until after any newly appeared invoice-linked success has been rejected (`lago_settlement_integration_test.go:245-268`). This closes the old-target masking path described by prior final-review F1.
- The regression checks an interleaved success, pre-observed historical success, and newly observed unlinked success (`:97-114`), while the earlier exact-target/no-match assertions remain (`:79-95`).
- The changes leave the Lago authority and production settlement implementation untouched, consistent with the approved spec and ADR boundary.

## Findings

### Low — L1: The regression does not exercise capture of the complete pre-settle ID set

**Evidence / affected symbol:** `TestPaymentIntentCandidateSelection` constructs `preObservedIDs` by hand at `lago_settlement_integration_test.go:83`; `paymentIntentIDSet` at `:273-282` is called only by the live fixture at `:446`. The unit regression therefore still passes if capture later omits succeeded historical or unlinked rows.

**Impact:** A future regression in the capture helper could make the fixture falsely fail on known historical successes or weaken identity evidence without being caught by the focused test.

**Smallest correction:** Build `preObservedIDs` from the same test pre-settle rows passed to `preSettlePaymentIntentCandidates`, then assert that the set includes all four IDs, including historical success and unlinked PI. This is a coverage improvement; the current runtime wiring does capture the complete observed set.

### Low — L2: The durable checkpoint is not recorded as an exact commit

**Evidence / affected file:** `docs/plans/issue-72-execution-ledger.md:371` says “this task commit at completion (HEAD recorded by Git)” and `task-1-report.md` points to that ledger. Neither records `26215a3bd7b7baf568628c4c509d7b409140f5e3`, although Step 6 of the assigned brief requires the exact commit/checkpoint.

**Impact:** After the branch advances, the durable task record cannot identify the reviewed implementation checkpoint without reconstructing Git history or retaining this review package.

**Smallest correction:** Add the exact implementation SHA to the execution ledger in a follow-up evidence record, and cite this review and validation checkpoint when available.

## Cannot verify here

- The implementer's listed commands were not rerun. Their same-checkpoint results are reported in `task-1-report.md`; no particular contrary test evidence appeared in this review.
- There is no captured pre-change RED run; the implementation and regression were added together, as the report states.
- The guard is scoped to IDs present in the bounded post-settle list. It does not prove the production command's selected PI identity for every possible provider interleaving, particularly an unseen new PI that has not succeeded. The approved Task 1 contract specifically addresses newly appeared **succeeded invoice-linked** rows in that list.
- Dedicated live T9 and real Alipay sandbox AC4 remain incomplete and are not established by the local selector regression.
