# T10 Whole-JD Grammar Independent Validation

- **Verdict:** DONE
- **Code SHA:** `85ee5cac251cfd93998a951f9e783baac0c02035`
- **Validation worktree HEAD:** `e235783461432f4a0540ba88c003652524196229`; the only change after the code SHA is the implementation report `task-10-whole-jd-grammar-report.md`. Parser and test source match the requested SHA.
- **Plan:** `/Users/wuyongjun/.codex/worktrees/issue-140-integration/WeKnora-fork01/docs/plans/2026-09-24-issue-140-t10-whole-jd-grammar-fix.md`
- **Scope:** #150 backend acceptance for whole-JD exclusive graduation grammar, citation and profile evidence, replay/concurrent intent conflict, and regression evidence. No source/test edits.

## Independent checks

1. `go test -count=20 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` — **PASS**.
2. `go test -race -count=1 ./internal/modules/career -run 'TestEvaluateOpportunityHardOutcomesArePinnedToEvidence|TestEvaluationReplayKeepsOriginalProfileVersionAndNewEvaluationIsImmutable|TestConcurrentChangedEvaluationIntentReturnsHTTPConflict'` — **PASS**, no race reported.
3. `git diff --check 85ee5cac251cfd93998a951f9e783baac0c02035^ 85ee5cac251cfd93998a951f9e783baac0c02035` — **PASS**.

Same-SHA implementation evidence was reused for the broader checks because those files and migrations are unchanged in the exact code commit: `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/` passed, including database migration coverage. The report also records the focused matrix at `-count=20` and Career race run as passing. No migration SQL/schema changed in `85ee5cac`, so no separate migration rerun was needed.

## Behavior checked

- The table-driven acceptance cases include confirmed 2026 versus isolated 2027-only JD (`ineligible`), matching 2027, and standalone skill/project fields before/after the graduation line. Mismatch citations are checked against the exact raw JD slice and byte offsets.
- Later-line and embedded field counterexamples return `unknown`: `仅限2027届\n技能：Go\n2026届亦可`, `仅限2027届\n技能：Go，2026届亦可`, and their CRLF variants. Cases for `备注：2026届可报`, `项目：2026届亦可`, `技能：不限届别`, alternative/negation wording, publication-year text, and unrecognized continuation also return `unknown` with no fabricated job/profile evidence.
- Sequential replay after profile edit preserves the original receipt/evaluation; the fresh request captures the new profile revision and the old result remains immutable. The synchronized changed-intent request test returns HTTP 409 `idempotency_conflict` to the competing intent.
- The evaluator emits no aggregate score/probability field in the acceptance matrix.

## Limitations

- The grammar is intentionally conservative: outside the single positive graduation line, only complete `技能：...` / `技能:...` and `项目：...` / `项目:...` fields without years or graduation/qualification language are accepted. Other JD structures produce `unknown` by design.
- No backend acceptance gap was found for the assigned plan and repair scope. This does not replace full T10 Web acceptance or final parent OCR.
