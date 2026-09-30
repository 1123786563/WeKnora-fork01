# T10 Canonical Graduation Clause Independent Validation

- **Verdict:** DONE
- **Code SHA:** `df53504dfab705bbad7edf77015ff87d0b371814`
- **Worktree HEAD:** `df53504dfab705bbad7edf77015ff87d0b371814` at validation start; source tree clean.
- **Plan:** `docs/plans/2026-09-24-issue-140-t10-canonical-clause-fix.md` in the integration worktree.
- **Scope:** Reviewer counterexamples, three-valued graduation outcomes, raw citation offsets, replay and concurrent changed-intent behavior. Read-only; only this report was added to the ignored `.superpowers` directory.

## Independent checks

1. `go test -count=10 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` — **PASS**.
2. `go test -race -count=1 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` — **PASS**, no race reported.
3. `git diff --check df53504dfab705bbad7edf77015ff87d0b371814^ df53504dfab705bbad7edf77015ff87d0b371814` — **PASS**.

Same-SHA implementation evidence was reused for the broad and migration checks because it directly tested the same code commit and the repair changes only parser/test files: `go test ./internal/modules/career ./internal/router ./internal/database ./internal/handler/... ./internal/container` passed. The database package run includes `TestCareerEvaluationSQLiteMigrationUpDownUp`; the repair changes no schema or migration files, so no separate migration rerun was needed.

## Acceptance evidence

- Confirmed 2026 with canonical `仅限2027届` remains `ineligible`; confirmed 2027 with the canonical matching clause can be `eligible` for the narrow graduation rule; a missing confirmed graduation year stays `unknown`.
- Accepted canonical clauses retain citation offsets into the original raw JD. Tests verify `quotedText == rawText[spanStart:spanEnd]`, and terminal span ends exactly after `届`, excluding optional trailing `。` or `.`. Unknown mixed/question/alternative forms carry no fabricated job or profile citation.
- Both reviewer counterexamples remain unknown: `仅限2027届？` and `仅限2027届\n技能：Go，其他批次均可`. Matrix coverage also includes ASCII `?`, LF/CRLF skill/project text, later/embedded graduation alternatives, unrecognized continuation, and other punctuation.
- Sequential replay after a profile edit preserves the exact original receipt; a fresh request captures the later profile revision while the old evaluation remains immutable. A synchronized concurrent changed-intent race returns HTTP 409 `idempotency_conflict` to the losing request.
- No aggregate hiring score or probability is introduced in the matrix output checks.

## Limitations

- The hard rule now intentionally accepts only a whole-input canonical affirmative clause with optional surrounding whitespace and one allowed full stop. Any mixed or multi-line JD yields `unknown`, even if a human could interpret its intended scope. This is the conservative plan boundary.
- No acceptance gap was found in this parser repair. This report does not approve Web completion or the final T10/#150 and parent OCR gates.
