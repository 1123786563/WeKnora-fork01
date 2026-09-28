# Task8 interface audit R3 — 2026-09-29

Scope: read-only follow-up to `t64-task8-interface-audit-r2.md`; no code or tests were changed or run.

## R2 finding disposition

1. **Four-field Run pins — resolved in the plan.** The plan now requires one database constraint with either all four `security_*` fields NULL or all four populated and `security_pin_source` restricted to `admission` or `legacy_backfill` (shared interfaces; 8B storage contract and migration tests). Backfill precedes a trigger that forbids later changes to any of the four fields. New admission inserts all four fields in 8C. Exact cancellation uses tenant plus Agent, Version, and Release, including retired Variant targets. Ordinary non-Marketplace Runs retain all-NULL pins and are not selected.

2. **Human review evidence — resolved as an operational gate.** The release operator checks a named data owner's signed per-tenant disposition, database identity, schema version, preflight query/result hashes, and unchanged frozen result set before migration. Raw identifiers and signed reports remain in an access-controlled deployment evidence store; Git stores only query templates and opaque evidence references. The plan correctly says SQL cannot attest the human signature. An unresolved or missing report blocks rollout.

3. **Backfill-to-guard window — resolved at the design level.** Old Run and Variant writers are quiesced before final preflight and remain stopped through migration. SQLite uses the existing per-file transactional runner (`NoTxWrap=false` for this migration). PostgreSQL obtains `ACCESS EXCLUSIVE` locks on `agent_runs` and `agent_adoption_variants` before final recheck and holds them through backfill and both guards. The plan requires the backfill and guards to commit together and a concurrent-writer boundary check.

## Engine-specific implementation note

The PostgreSQL SQL file must explicitly delimit the transaction if its migration driver does not wrap a file automatically; the plan says one transaction but should make that executable in the SQL migration Brief. SQLite has explicit prior NoTxWrap exceptions for migrations 55 and 114 in `internal/database/migration.go`; the new migration is not one of them, so the normal transactional path applies. Triggers and partial unique indexes are supported by both engines, but syntax and down-migration behavior must be validated separately as the plan requires.

## Conclusion

No remaining architecture blocker from the R2 scope. The PostgreSQL transaction delimiter is an implementation detail to pin in 8B's Brief and verify against the actual migrator. This is design-contract readiness, not migration execution evidence or production data approval.
