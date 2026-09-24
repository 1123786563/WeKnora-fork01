# T10 graduation block parser repair report

## Scope

Fixed the high-severity cross-line graduation requirement classification defect in `internal/modules/career/evaluation.go` and `evaluation_test.go`. The change accepts a graduation clause only when its logical block is isolated or the adjacent block is explicitly labeled as skills/project content. Any other non-empty adjacent line remains unknown before profile evidence is consulted. CRLF classification removes the carriage return from the line view only; cited offsets and quoted evidence still refer to the unchanged original JD bytes.

## TDD and behavior evidence

- RED: `go test ./internal/modules/career -run TestEvaluateOpportunityHardOutcomesArePinnedToEvidence -count=1` failed as expected: wrapped alternative on following line, wrapped alternative before candidate, second graduation option on following line, negation prefix on prior line, and both CRLF wrapped alternatives all returned `ineligible` instead of `unknown`.
- GREEN: the same matrix passed after the repair. It now covers those cases, same-line negation/list alternatives, a standalone mismatch followed by `技能：Go`, and the existing `技能要求：Go` labeled-section case.
- Ambiguous table cases assert both absent JobEvidence and absent ProfileEvidence. Conclusive cases retain exact span quote verification against the raw JD slice.
- Updated the soft evidence integration test to use the explicit `技能要求：` section boundary required for a conclusive graduation classification.

## Verification

All commands were run from the isolated worktree `/Users/wuyongjun/.codex/worktrees/issue-140-t10-evaluation/WeKnora-fork01`.

| Command | Result |
| --- | --- |
| `go test ./internal/modules/career -run TestEvaluateOpportunityHardOutcomesArePinnedToEvidence -count=1` (before fix) | Expected RED failures listed above. |
| `go test ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationUsesOnlyConfirmedSoftEvidenceAndKeepsItSeparate' -count=1` | PASS. |
| `go test ./internal/modules/career -run TestEvaluateOpportunityHardOutcomesArePinnedToEvidence -count=20` | PASS. |
| `go test -race ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict' -count=1` | PASS. |
| `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/` | PASS. |
| `go test -count=1 ./internal/database -run 'TestCareerEvaluationSQLiteMigrationUpDownUp|TestCareerOpportunitySQLiteMigrationUpDownUp'` | PASS. |
| `git diff --check` | PASS. |

## Commit

- Code and tests: `d70792a3e1e5b5d29ca2386452489089322205af` (`fix(career): classify graduation requirement blocks`).

## Limits

The parser intentionally returns unknown when an unrecognized non-empty line appears directly before or after the candidate graduation line. Only the explicit prefixes `技能：`, `技能:`, `技能要求：`, `技能要求:`, `项目：`, `项目:`, `项目经验：`, and `项目经验:` establish a safe adjacent section boundary. This favors a missed hard result over a potentially false ineligible decision.
