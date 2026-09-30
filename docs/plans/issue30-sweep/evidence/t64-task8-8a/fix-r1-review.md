# Task 8A fix round 1 independent review

Reviewed the exact `283022dd30ba9fa5f73a238d1d6c5e6a5d4fa1e9..8090adb3c46431f3c706dca3d557a928db6baa04` package and both changed source files against the Task 8A brief, approved Marketplace spec §§10–11, `CONTEXT.md`, ADR-0011, ADR-0015, Task 8 plan, prior independent review, and fix report. Scope was read-only source review; I did not run tests or OCR. The fix report records a passing focused `TestCheckLocalAgentReleaseAdmissionTx` run and `git diff --check` at the reviewed HEAD.

## Prior findings

- **T64-8A-R1-1 — resolved.** `checkLocalAgentReleaseAdmissionTx` now loads a tenant-scoped `CustomAgent` at `agent_security_guard.go:137–144` before either the ordinary return at `:145–151` or the adopted Version/Release path at `:153–172`. `CustomAgent.DeletedAt` is a GORM soft-delete field, so its normal query scope excludes deleted rows. The model's `(ID, TenantID)` primary key makes the live row unique. The missing and soft-deleted adopted cases at `agent_security_guard_test.go:38–39,45–53` preserve the Variant/Version/Release fixture and require `ErrAgentSecurityReleaseUnresolvable`. The ordinary path also traverses the same live-Agent check.
- **T64-8A-R1-2 — resolved.** The table specifies `ErrAgentSecurityReleaseUnresolvable` for missing/mismatched Version, retired/draft/duplicate Variant, and missing/deleted Agent; Release and exact dependency revocations specify `ErrAgentSecurityReleaseBlocked` (`agent_security_guard_test.go:29–39`). `require.ErrorIs` at `:72–74` checks each sentinel.

## New findings

None established in the reviewed range. The helper uses the supplied transaction, locks Variant rows by ID before the Agent and Version reads, checks the exact tenant/Agent/Version binding, and delegates Release/dependency revocation to the existing transactional predicate. No other files changed in the range.

## Verdict

- **Spec Compliance: PASS for Task 8A.** The previous stale-Agent admission gap is closed and the adopted/ordinary classifications now require a live tenant-owned Agent. The fixed Release and exact dependency checks remain in the guarded transaction.
- **Code Quality: PASS for Task 8A.** The new regressions and sentinel assertions cover the two prior findings. Evidence is the reviewed code and the fix report's focused SQLite run; PostgreSQL lock behavior and concurrent delete/admission were not independently exercised in this review.
