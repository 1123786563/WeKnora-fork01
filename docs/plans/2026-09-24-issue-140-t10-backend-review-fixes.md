# T10 Career Evaluation backend review fixes

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove false eligibility evidence and recover an actual changed-intent conflict in T10's first backend review.

**Architecture:** Restrict the graduation rule to an unambiguous explicit clause before any hard rejection. A rule without a recognized source condition keeps the fixed snapshot reference but has no fabricated condition span. Soft literal matches require an appropriate boundary for short Latin values. Scoped receipt reconciliation propagates a found fingerprint conflict as 409.

**Tech Stack:** Go Career Office/HTTP tests, GORM concurrency tests.

**Spec:** #150 snapshot, approved Job Search Spec §§3–4/8, ADR-0017; initial backend commit `d170cc02ea478f51a21804f6193d67a046424e0a`; first independent review with one high and three medium findings.

## Global Constraints

- Existing isolated T10 backend worktree only. Own `internal/modules/career/evaluation.go` and `evaluation_test.go`, and handler test only if needed. No migration, route, Web or guard changes.
- Preserve historical pinned evaluation JSON and scoped receipt semantics. Local code commit authorized; no push/merge/deploy.
- An uncertain/ambiguous source rule must be `unknown`, not `ineligible` or `eligible`; soft text occurrence is not authority to alter hard status.

## Review Focus

- Confirmed 2026 against `仅限2027届或2026届` and `并非仅限2027届` must be unknown, while a standalone `仅限2027届` remains ineligible with a valid span. A standalone condition line plus unrelated skill line may stay ineligible only if the graduation clause is demonstrably isolated.
- A generic JD with no graduation wording has no graduation `jobEvidence` span; it still carries fixed snapshot/profile revision and an explicit missing/unsupported reason.
- Skill `Go` must not match `Google`; a standalone `Go` token or `Go语言` can match if the cited bytes are exact. Keep project/intent evidence and source offsets valid.
- Concurrent same request ID with different intent returns 409 `idempotency_conflict` when a committed conflicting receipt is found after the insert race, never 504 in that known-conflict case.

---

### Task 1: Constrain the hard graduation rule

**Depends:** initial T10 backend. **Owner/validator:** backend_implementer / backend_validator. **Files:** `internal/modules/career/evaluation.go`, `evaluation_test.go`. **Consumes:** exact immutable JD and confirmed year. **Produces:** no false hard rejection.

- [ ] RED: Add table-driven Office tests for alternative (`仅限2027届或2026届`), negation (`并非仅限2027届`), isolated mismatch, missing graduation wording, and a condition line with separate skill text. Verify current false rejection and fabricated whole-JD citation.
- [ ] GREEN: Recognize only an explicit, unambiguous clause with safe surrounding boundary; classify alternatives/negation or other qualifier as unknown before comparing years. Create `JobEvidence` span only for the recognized cited clause; leave it absent for no requirement. Keep source-span byte offsets consistent with exact JD.

### Task 2: Bound soft literal evidence

**Depends:** initial T10 backend; same owner/worktree. **Owner/validator:** backend_implementer / backend_validator. **Files:** `evaluation.go`, `evaluation_test.go`. **Consumes:** confirmed skill/project/preference fact and raw JD. **Produces:** exact cited soft match or unknown/absence.

- [ ] RED: Add `Go` in `Google` non-match and standalone `Go` match tests; assert JD offsets/quote and confirmed fact revision/source on any match.
- [ ] GREEN: For short Latin fact values, require ASCII word boundaries around the occurrence; scan later occurrences if an earlier embedded occurrence fails. Keep case-sensitive matching and avoid inventing a match when the boundary is ambiguous.

### Task 3: Propagate changed-intent reconciliation

**Depends:** initial T10 backend; same owner/worktree. **Owner/validator:** backend_implementer / backend_validator. **Files:** `evaluation.go`, `evaluation_test.go`, handler test if needed. **Consumes:** scoped receipt lookup after transaction error. **Produces:** original receipt, 409 conflict, or typed unknown.

- [ ] RED: Reproduce a concurrent same-ID changed-intent race where initial lookup sees no receipt, one transaction commits, the other insert conflicts; assert 409 rather than 504. Use a deterministic barrier/hook if the package already exposes one, with bounded test time.
- [ ] GREEN: Propagate `ErrIdempotencyConflict` returned by `reconcileEvaluation`; keep actual not-found/lookup uncertainty as `OutcomeUnknownError` with request ID.
- [ ] VERIFY: Focused Office/HTTP tests, race-sensitive test if feasible, `go test -count=1 ./internal/modules/career/... ./internal/router/... ./internal/database/... ./internal/handler/... ./internal/container/...`, `git diff --check`; independent Spec/quality re-review and validator before integration.

## Shared-file preflight

This single backend implementer owns all repair files in one isolated worktree. No T10 guard or Web implementation starts before review/integration. The reviewed initial code commit stays intact; report repair commit and test evidence separately. If the narrow clause parser cannot distinguish common qualifiers with evidence, return unknown and report the limitation instead of relaxing safety.
