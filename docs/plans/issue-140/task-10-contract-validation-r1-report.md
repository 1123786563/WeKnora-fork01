# T10 Task 2 contract repair validation

- **Status:** DONE
- **Validated code SHA:** `5f37f68abd10733a39e02244e782a932c48e4864`
- **Scope:** malformed Evaluation detail rejection, valid backend-shaped Evaluation response, focused tests, Web typecheck, and whitespace check. No tracked source or test files were changed.

## Commands and results

- `node_modules/.bin/tsx --test packages/career-core/src/contracts.test.ts packages/api-client/src/career.test.ts` — **PASS, 14/14** at the exact SHA in a detached worktree with temporary workspace dependency links. This covers invalid empty/inconsistent aggregate rule sets, missing conclusive citations, citations absent from the pinned facts manifest, plus unknown/no-citation acceptance and a complete ineligible response fixture.
- `pnpm typecheck:web` — **PASS, exit 0** at the exact SHA after temporarily linking `apps/web/node_modules` and `packages/views/node_modules` from the integration checkout (in addition to root dependencies and API workspace aliases). Links were removed with the temporary worktree.
- `git diff --check 5f37f68abd10733a39e02244e782a932c48e4864^ 5f37f68abd10733a39e02244e782a932c48e4864` — **PASS, exit 0**.

## Evidence and scope

The detail fixture carries the backend wire fields (`evaluation_created` receipt fields, fixed snapshot, hard/soft results, fact manifest, timestamp and source/confirmation provenance) and passes the strict decoder. I compared its shape against the Go `Evaluation` and nested structs at this SHA. The Go evaluation tests marshal actual results and assert score/probability fields are absent. The repair now rejects empty rules, mismatched aggregate status, conclusive rules lacking either citation, and rule/soft citations that do not match a complete pinned fact; an unknown rule without citations remains valid.

No browser, responsive, accessibility, or UI loading/error state applies to this contract-only repair. Backend/browser end-to-end verification remains outside this validation scope.
