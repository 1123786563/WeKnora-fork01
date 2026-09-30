# T10 Task 3 typed Web evaluation API report

- **Task:** #150 / T10, typed Web evaluation API/client
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-web/WeKnora-fork01`
- **Base:** `0370bebfecb661cecbf937d2663c6936caa65177`
- **Status:** implementation and targeted validation complete; independent frontend review/validation and integration remain with controller
- **Commit:** pending at report creation

## Files changed

- `packages/career-core/src/contracts.ts`, `contracts.test.ts`
- `packages/api-client/src/career.ts`, `career.test.ts`

## Implementation

Added strict `EvaluationReceipt` and `Evaluation` contracts with the backend `evaluation_created` discriminator, three-value status union, and exact immutable read fields. Decoders reject unknown enum values, unexpected fields (including aggregate score/probability-like fields), absent fact confirmation/revision provenance, mismatched profile revisions, mismatched snapshot references, malformed timestamps/hashes, and evidence spans whose UTF-8 byte offsets do not quote the exact immutable snapshot text. Raw JD and evidence strings are returned unchanged as inert data.

Added `evaluateOpportunity`, `evaluationReceipt`, and `evaluation` client methods for the three Career endpoints. Request identifiers are encoded in query/path segments; create accepts the backend request body with optional pinned `profileRevision` and validates identifiers/revision.

## RED/GREEN and verification evidence

The first attempted RED run could not load `tsx` because this isolated worktree had no dependencies installed. With a temporary symlink to the integration worktree dependency tree, API-client tests initially could not resolve workspace package aliases, and the new decoder rejected a test fixture whose soft-evidence byte span extended beyond the fixture JD. The workspace aliases were linked locally for testing, the fixture was corrected to reflect Go's UTF-8 byte offsets, and verification then passed. All temporary symlinks were removed before commit.

Focused verification command:

```text
pnpm exec tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts
PASS: 12 tests, 0 failures
```

Web typecheck:

```text
pnpm typecheck:web
PASS (exit 0)
```

Typecheck used temporary `node_modules` symlinks to the integration worktree because this worktree had no installed dependencies; symlinks were removed afterward. No production/test files outside the four owned paths were edited.

Whitespace check:

```text
git diff --check
PASS (exit 0)
```

## Remaining validation

Independent frontend review/validation and controller integration remain pending. No browser UI behavior is introduced by this contract/client-only task.
