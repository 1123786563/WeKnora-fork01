# T10 profile key integration fix report

## Scope

Implemented the assigned profile graduation key integration fix in `internal/modules/career/evaluation.go` and `evaluation_test.go`. The evaluator now recognizes the confirmed aliases `education.graduation_year`, `graduation_year`, `毕业时间`, and `education.graduation_date`; malformed recognized facts and any conflicting candidate make the hard result unknown. Equal candidates use deterministic evidence preference (`education.graduation_year`, `graduation_year`, `毕业时间`, then `education.graduation_date`) and retain the original fact key/value/revision in evidence.

## Verification evidence

- RED before implementation: `go test ./internal/modules/career -run '^TestEvaluateOpportunityRecognizesConfirmedCareerFormGraduationAliases$' -count=1` failed as expected. `毕业时间=2026` and `education.graduation_date` returned unknown; the third conflicting alias incorrectly returned eligible.
- GREEN focused: `go test ./internal/modules/career -run 'TestEvaluateOpportunityRecognizesConfirmedCareerFormGraduationAliases|TestEvaluationUnknownForUnconfirmedMalformedAndContradictoryGraduationFacts|TestEvaluateOpportunityHardOutcomesArePinnedToEvidence' -count=1` passed (`ok`, 1.022s).
- Race: `go test -race ./internal/modules/career -count=1` passed (`ok`, 4.041s).
- Broader: `go test ./internal/modules/career ./internal/handler/... ./internal/router ./internal/database ./internal/container -count=1` passed all listed packages. The linker emitted a duplicate `-lc++` warning for `internal/container.test`.
- Diff hygiene: `git diff --check` passed.

## Notes

No API shape, persistence, authentication, authorization, cancellation, or JD grammar code changed. Existing tests cover missing/unconfirmed facts and malformed confirmed values; the new tests cover the browser form key through `Office.Act`, the resume date alias, equality preference across three aliases, and a third contradictory candidate.
