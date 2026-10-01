# Task 3 review repair plan — round 1

Source: independent Task 3 review of `.superpowers/sdd/plan-t63/review-package-task-3-0fa8c3d..a51b354.patch`, SHA-256 `7af8d4d1457bce77b98808bff8931f47c457300b6e704c7c14c1cc41977d925a`. The review verdict was Spec FAIL / Quality FAIL. This repair plan addresses only its two valid concurrency findings; it does not expand #63 scope.

## Finding F1 — High: stale marketplace eligibility

Adopt, CreateVariant, AcceptUpgradeProposal, and reconciliation can read an eligible listing/release, then write after a concurrent Unlist/Deprecate commits. Guard use at the repository write boundary, synchronized with lifecycle transitions. Rejections must map to existing conflict semantics. Reconciliation must not materialize stale proposals. Add deterministic interleaving tests proving no adoption, variant, or accepted proposal references an unlisted/deprecated source. Preserve existing service signatures unless the repository seam requires a narrow interface extension.

## Finding F2 — Medium: concurrent successor cycle

Two concurrent DeprecateRelease calls can each validate an active successor and then deprecate both releases into a cycle. Validate successor and transition target atomically while serializing both release rows in stable order. Add deterministic concurrent A→B/B→A coverage proving at most one succeeds and no cycle is persisted.

## Task

- **Owner role:** backend_implementer. **Validator:** backend_validator. Independent reviewer: reviewer.
- **Owned files:** only the Task 3 lifecycle service/repository interfaces and implementations plus their focused tests; any new repository lifecycle file must be explicitly listed in the implementer brief before edits. Do not touch Task 4/5/6 files or unrelated untracked `paseo.json`.
- **Consumes:** integrated T63 Task 2 repository primitives at `codex/issue30-b6-t63-cont` HEAD `8327d1d33141e9f67c939bbf30960852900fe0ec`; original Task 3 brief and plan; this repair plan; the two review findings above.
- **Produces:** atomic lifecycle eligibility guards, successor transition serialization, deterministic behavior tests, a repair report, exact diff package and SHA-256.
- **Verification:** focused lifecycle and adoption/upgrade service tests, race run for deterministic concurrent cases, gofmt, `git diff --check`; report exact commands and outcomes. No downstream Task 4/5/6 dispatch before independent review and integration.
- **Failure handling:** if existing DB/backend semantics cannot provide an atomic guard, report the precise seam and evidence; do not replace deterministic concurrency tests with sleeps or claim sequential tests prove race safety.

## Review focus

Spec: no new use after unlist/deprecation; deprecation successor remains active and same listing under concurrent reciprocal transitions; conflict response preserved. Quality: transaction/locking correctness under supported databases, tenant scoping, deadlock order, no partial writes, deterministic interleaving coverage.

## Status

Pending. Task 3 remains blocked from integration and downstream dispatch until verification, independent review, and integration succeed.
