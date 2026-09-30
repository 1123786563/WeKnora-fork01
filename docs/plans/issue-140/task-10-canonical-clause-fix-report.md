# T10 canonical graduation clause repair — implementer report

Plan: `docs/plans/2026-09-24-issue-140-t10-canonical-clause-fix.md` (integration worktree copy)

Code commit: `df53504dfab705bbad7edf77015ff87d0b371814`

Scope: `internal/modules/career/evaluation.go` and `evaluation_test.go` only. No push, merge, or deploy.

## Root cause

The prior parser allowed labeled skill/project lines outside the graduation clause and treated any punctuation as harmless. It also returned a hard mismatch before the separate unparsed-text check. This made a question and an exception hidden in a skill field look like an affirmative exclusive requirement.

## RED → GREEN → REFACTOR

- RED: Updated the outcome matrix for extra JD text, including `仅限2027届\n技能：Go，其他批次均可`, `仅限2027届？`, ASCII `?`, LF/CRLF soft fields, and other punctuation. The focused run failed on the expected `unknown` versus actual `ineligible` outcomes.
- GREEN: A hard graduation comparison now requires the trimmed entire JD to consist of `仅限`, optional horizontal spacing, a four-digit year, `届`, and at most one `。` or `.`. Any other content leaves hard outcome `unknown` with no JD or profile citation. Matched citations use byte offsets into the unmodified JD.
- REFACTOR: Removed the previous line/soft-field allowlist, qualification denylist, and broad punctuation helper. Soft evidence logic remains separate and unchanged.

## Verification at code commit

- `go test ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationUsesOnlyConfirmedSoftEvidenceAndKeepsItSeparate' -count=10` — pass.
- `go test -race ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationUsesOnlyConfirmedSoftEvidenceAndKeepsItSeparate|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict' -count=1` — pass.
- `go test ./internal/modules/career ./internal/router ./internal/database ./internal/handler/... ./internal/container` — pass. Container linker emitted nonfatal duplicate `-lc++` warning.
- `git diff --check` — pass before commit; working tree clean after commit.

## Limit

Any multi-line or otherwise noncanonical JD yields `unknown` for this hard rule, even when a reader might infer a definite graduation restriction. This is the conservative boundary specified by the repair plan; soft literal matches can still be displayed separately.

Independent validation and review remain assigned to the controller.
