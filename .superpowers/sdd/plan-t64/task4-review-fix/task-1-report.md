# T64 Task 4 Review Fix — Task 1 Report

## Scope

Addressed reviewer finding T64-T4-R1 in the existing T64 worktree, based on `docs/plans/issue30-sweep/plans/plan-t64-task4-review-fix.md` and `.superpowers/sdd/plan-t64/task-4-review-report.md`.

Changed only:

- `internal/application/service/agent_security.go`
- `internal/application/service/agent_security_test.go`

The service now rejects non-object lock JSON, missing or null `dependencies`, malformed JSON, and dependency entries with missing/blank identity components (type, ID, version, digest). It continues to allow `{"dependencies":[]}`. Added behavioral coverage for those invalid and valid lock forms, exact case-sensitive mismatches for each tuple component, release revocation precedence over a matching dependency revocation, and unknown/foreign release admission errors.

## RED → GREEN evidence

- RED: `go test ./internal/application/service/ -run 'TestAgentSecurity(VerdictRejectsStructurallyInvalidLocks|VerdictAllowsValidEmptyLock|VerdictDependencyTupleMismatchBoundaries|VerdictReleaseRevocationPrecedesDependencyRevocation|ReleaseAdmissionUnknownAndForeignRelease)$' -count=1` failed on the existing permissive decoding: `null`, `{}`, missing/null dependency arrays, null entries, and missing tuple fields produced no error.
- GREEN focused: same command — PASS.
- Related regression filter: `go test ./internal/application/service/ -run 'TestAgent(Security|Upgrade|Adoption)' -count=1` — PASS.
- Formatting check: `git diff --check` — PASS.

## Review notes

Exact case-sensitive tuple comparison, release-first precedence, tenant-scoped variant lookup, introduced-release lookup, and existing unresolvable-release behavior are preserved. Optional `license_id` is not required by this validator; the adjudicator's identity check requires only the four tuple fields.

Commit will be recorded in the task handoff after committing the two owned source/test files and this report.
