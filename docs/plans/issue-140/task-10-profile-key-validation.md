# T10 Profile Graduation Key Independent Validation

- **Verdict:** DONE
- **Code SHA:** `b28d133fcd935730557eb67ad885948b2b7aac31`
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-profile-key/WeKnora-fork01`
- **Scope:** Public Career Office graduation profile aliases, evidence, ambiguity, replay/pinned facts and race behavior. No source/test edits; only this ignored `.superpowers` validation report was added.

## Independent checks

1. `go test -count=10 ./internal/modules/career -run 'TestEvaluateOpportunityRecognizesConfirmedCareerFormGraduationAliases|TestEvaluationUnknownForUnconfirmedMalformedAndContradictoryGraduationFacts|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` — **PASS**.
2. `go test -race -count=1 ./internal/modules/career -run 'TestEvaluateOpportunityRecognizesConfirmedCareerFormGraduationAliases|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` — **PASS**, no race reported.
3. `git diff --check b28d133fcd935730557eb67ad885948b2b7aac31^ b28d133fcd935730557eb67ad885948b2b7aac31` — **PASS**.

Same-SHA implementation evidence was reused for the broader suites because this change touches only `evaluation.go` and `evaluation_test.go`: `go test ./internal/modules/career ./internal/handler/... ./internal/router ./internal/database ./internal/container -count=1` passed all listed packages. The report records a duplicate `-lc++` linker warning for the container test, with the package passing. No route or migration changed.

## Behavior evidence

- The public Office test uses `Office.Act("confirm", ..., "毕业时间", "2026", ...)`, then evaluates canonical `仅限2027届`. Result is `ineligible`, and returned profile evidence preserves original key `毕业时间`, original value `2026`, and the fact revision returned by the confirmation action.
- Resume alias `education.graduation_date=2026-06-30` is `ineligible`; `2027-06-30` is eligible for the isolated canonical rule. A malformed recognized `毕业时间=预计2026` remains unknown.
- Unconfirmed proposal, missing graduation fact and contradictory/malformed facts remain unknown. All three aliases are compared; equal values choose `education.graduation_year` deterministically, and a third conflicting alias yields unknown.
- Existing replay/pinning coverage verifies that a replay returns the original receipt after profile edits, a new request captures the new revision, and the old evaluation remains immutable/readable. The synchronized concurrent changed-intent test returns HTTP 409 `idempotency_conflict`.
- No acceptance gap was found in the assigned scope.

## Limitations

- This is unit-level public Office/API seam evidence; the separate live-browser re-run after the repair remains the controller's integration acceptance responsibility.
- PostgreSQL execution is not part of this focused alias repair; the change does not touch SQL or persistence schema.
