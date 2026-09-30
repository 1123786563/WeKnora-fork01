# T10 Task 2 Web contract review fixes report

- **Task:** #150 / T10, evaluation decoder invariants
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-web/WeKnora-fork01`
- **Consumed base:** `b75595ecae54315b67d3e257e697a46da08db12a`
- **Implementation commit:** `5f37f68abd10733a39e02244e782a932c48e4864`
- **Status:** implementation and targeted validation complete; independent frontend review/validation and integration remain with controller

## Files changed

- `packages/career-core/src/contracts.ts`, `contracts.test.ts`
- `packages/api-client/src/career.test.ts`

## Implementation

The Evaluation decoder now requires a nonempty hard-rule list, computes aggregate status using `ineligible > unknown > eligible`, and rejects aggregate or receipt status that disagrees with those rules. Every conclusive (`eligible` or `ineligible`) hard rule must include both a valid JD citation and confirmed profile citation. Every hard or soft profile citation must match a complete evidence record in the pinned `facts` manifest, including source and confirmation metadata. `unknown` rules can remain uncited; the decoder does not invent provenance.

Replaced the API client test's empty hard-rule detail fixture with a valid ineligible graduation evaluation tied to a fixed JD span and pinned confirmed fact. Added malformed aggregate, missing citation, absent pinned fact, and valid unknown/no-citation fixtures.

## RED/GREEN and verification evidence

RED run after adding the malformed fixtures showed expected `Missing expected exception` failures for empty/aggregate-inconsistent rule cases and missing citation/fact-manifest cases. The initial fixture update also exposed that its old client assertion still expected the prior arbitrary raw text; it was aligned to the fixed fixture JD before GREEN verification.

Focused command:

```text
pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts
PASS: 14 tests, 0 failures
```

Web typecheck:

```text
pnpm typecheck:web
PASS (exit 0)
```

Whitespace check:

```text
git diff --check
PASS (exit 0)
```

This isolated worktree had no installed dependencies. Temporary symlinks to integration-worktree dependencies and workspace package aliases were used for verification, then removed. No tracked files beyond the assigned three paths changed.

## Remaining validation

Independent frontend review/validation and controller integration remain pending. No UI, router, Go, or API wire shape changes were made.
