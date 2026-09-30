# T10 whole-JD graduation grammar repair report

## Scope

Updated only `internal/modules/career/evaluation.go` and `evaluation_test.go`. The hard parser now classifies the entire JD before consulting confirmed profile facts. It accepts one standalone positive `仅限 YYYY 届` line plus zero or more complete `技能：...` / `项目：...` fields. It scans every line, and any other nonblank line, 4-digit year, graduation lexeme, alternative, exception, or negation in a soft field makes the result unknown. CRLF affects only the classification line view; evidence offsets remain offsets into the unchanged raw JD.

## TDD evidence

- RED: `go test ./internal/modules/career -run TestEvaluateOpportunityHardOutcomesArePinnedToEvidence -count=1` failed on the new whole-JD cases. In particular, `仅限2027届\n技能：Go\n2026届亦可` and the embedded field variant returned `ineligible` instead of `unknown`. The same failure occurred for CRLF, notes, graduation lexemes inside project fields, unrestricted batch wording, and publication years.
- GREEN: the matrix now covers both valid soft-field labels separately and together, fields before/after the graduation line, later and embedded alternatives, CRLF, notes, graduation words, unrestricted wording, publication year, unrecognized continuation, and soft-field alternative/negation terms.
- Ambiguous table cases assert no JobEvidence and no ProfileEvidence. Recognized mismatch cases verify the exact original-source span through the returned evidence offsets and quote.

## Verification

All commands ran from `/Users/wuyongjun/.codex/worktrees/issue-140-t10-evaluation/WeKnora-fork01`.

| Command | Result |
| --- | --- |
| `go test ./internal/modules/career -run TestEvaluateOpportunityHardOutcomesArePinnedToEvidence -count=1` (before fix) | Expected RED failures, including the later-line and embedded skill-field false ineligible cases. |
| `go test ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationUsesOnlyConfirmedSoftEvidenceAndKeepsItSeparate' -count=1` | PASS. |
| `go test ./internal/modules/career -run TestEvaluateOpportunityHardOutcomesArePinnedToEvidence -count=20` | PASS after final matrix additions. |
| `go test -race ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict' -count=1` | PASS after final matrix additions. |
| `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/` | PASS. |
| `git diff --check` | PASS. |

## Code commit

`85ee5cac251cfd93998a951f9e783baac0c02035` (`fix(career): scan whole JD graduation grammar`).

## Conservative limits

Any nonblank line outside the candidate clause must be a full `技能：` or `项目：` field. These fields are rejected if they contain any four-digit number or graduation/qualification language. Ordinary JDs that do not fit this small grammar produce unknown by design.
