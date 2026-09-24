# T10 Task 1 backend review-fix report

- **Task:** #150 / T10, first independent backend review fixes
- **Worktree:** `/Users/wuyongjun/.codex/worktrees/issue-140-t10-evaluation/WeKnora-fork01`
- **Base implementation:** `d170cc02ea478f51a21804f6193d67a046424e0a`
- **Fix code commit:** `ec8f0f2f60e0041b47ea1113f12d5729f3c8c299`
- **Files changed:** `internal/modules/career/evaluation.go`, `evaluation_test.go`, `office.go`

## Review findings and changes

1. **High — ambiguous graduation text could reject a candidate.** `evaluateGraduationRule` now only interprets a single explicit `仅限 <year> 届` when that phrase is isolated on its own line (surrounded only by punctuation/whitespace), or when it is the entire JD. Alternatives (`仅限2027届或2026届`), negation (`并非仅限2027届`), same-line additional qualifiers and absent graduation wording return `unknown`. Unsupported/ambiguous rules have no `jobEvidence`; a recognized rule cites only the exact UTF-8 byte span. A separate skill line remains visible as unparsed content, so a matching graduation year stays unknown; a mismatching year remains ineligible because its hard conflict has a clearly isolated condition.
2. **Medium — missing graduation condition cited the whole JD.** Generic JDs and unrecognized/ambiguous graduation clauses no longer fabricate a graduation source span. The immutable snapshot and profile revision remain present at evaluation level.
3. **Medium — short Latin soft values matched inside larger words.** ASCII alphabetic fact values of up to four characters require ASCII token boundaries. The search continues after embedded occurrences, so `Go` does not match `Google` but can cite a later standalone `Go` or `Go语言` occurrence. Quotes and byte offsets come from the exact cited bytes; profile source/revision evidence remains attached.
4. **Medium — concurrent changed-intent race surfaced as 504.** When a transaction fails after an initial receipt miss and bounded reconciliation finds the same scoped request ID with a different fingerprint, Office now returns `ErrIdempotencyConflict`. Existing handler mapping returns HTTP 409 `idempotency_conflict`. A test-only internal barrier makes the insert race deterministic.

## RED evidence

Before the production behavior changes, the focused regression command failed as expected:

```text
go test -count=1 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationShortLatinSoftMatchRequiresASCIIWordBoundary|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'
```

Observed failures included: alternative/negated/same-line clauses returned `ineligible` instead of `unknown`; generic and bare-year JDs carried fabricated whole-text `jobEvidence`; `Go` in `Google` produced a soft match; the committed conflicting-intent race returned HTTP 504 `outcome_unknown` instead of 409. This established the regressions before GREEN.

## Verification

Focused regression tests passed:

```text
go test -count=1 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationShortLatinSoftMatchRequiresASCIIWordBoundary|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'
ok github.com/Tencent/WeKnora/internal/modules/career
```

The deterministic HTTP race passed 20 consecutive runs:

```text
go test -count=20 ./internal/modules/career -run '^TestConcurrentChangedEvaluationIntentReturnsHTTPConflict$'
ok github.com/Tencent/WeKnora/internal/modules/career
```

Career package race detector passed:

```text
go test -race -count=1 ./internal/modules/career/...
ok github.com/Tencent/WeKnora/internal/modules/career
```

The requested broader verification passed:

```text
go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/
ok github.com/Tencent/WeKnora/internal/modules/career
ok github.com/Tencent/WeKnora/internal/router
ok github.com/Tencent/WeKnora/internal/database
ok github.com/Tencent/WeKnora/internal/handler
ok github.com/Tencent/WeKnora/internal/handler/dto
ok github.com/Tencent/WeKnora/internal/handler/session
ok github.com/Tencent/WeKnora/internal/container
```

The container test link emitted the existing warning `ignoring duplicate libraries: '-lc++'`; its package passed.

```text
git diff --check
PASS (exit 0)
```

## Remaining limitation

Graduation parsing is intentionally limited to a single explicit and isolated clause. Other JD syntax remains unknown pending a separately reviewed parser. Independent re-review and backend validation remain with the controller.
