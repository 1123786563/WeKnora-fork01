# Task 8B fix round 1 independent review

- Reviewed range: `cbdb08e627b3466cc419815e3108fe18bb0b3bfc..ebb439454823f746734b0ce14184cc413e21877b`.
- Inputs: Task8B R1 brief, approved Marketplace spec, ADR-0011/0015, `CONTEXT.md`, implementation report, and scoped review package.
- Read-only review; no tests or source modifications.

| Finding | Verdict | Evidence |
|---|---|---|
| F1 HIGH — retired Variant identity can be rewritten | ADDRESSED | Repository locks and rereads the Variant, rejecting identity changes when `PublishedAt != nil` (`internal/application/repository/agent_adoption.go:332`). SQLite and PostgreSQL triggers use `OLD.published_at IS NOT NULL`; the SQLite regression mutates a retired Variant directly (`internal/database/migration_sqlite_versioned_schema_test.go:204`). |
| F2 HIGH — missing assistant placeholders do not roll back cancellation | ADDRESSED | Revocation, owner cancellation, and expiry cleanup share a tenant/session/owner/request/assistant-role scoped placeholder transition and require exactly one affected row (`internal/application/repository/agent_chat_turn_claim.go:209`). Its error aborts the enclosing transaction. Regression coverage includes all three missing-placeholder paths. |
| F3 MEDIUM — claim tenant-scoped key/index contract missing | ADDRESSED | SQLite and versioned PostgreSQL DDL declare `PRIMARY KEY(id, source_tenant_id)`, both session-tenant message uniqueness constraints, and `(source_tenant_id,state,release_id)` index. GORM tags match; SQLite schema assertions and PostgreSQL SQL source-contract assertion cover them. |
| F4 MEDIUM — committed revocation returned as an error after immediate reconciliation failure | ADDRESSED | Both service paths now return the committed revocation view and ID with `run_cancellation_state=pending`; a later retry test confirms completion (`internal/application/service/agent_security.go:235`). |

**New critical/high/medium breakage in fix diff:** None identified.

**Scoped spec verdict:** compliant. **Quality verdict:** pass for F1–F4.

The reviewer did not rerun tests because the diff presented no specific unresolved behavior doubt. The implementer report contains the exact focused test/build evidence. PostgreSQL runtime execution remains unverified; earlier broad-suite failures from unpinned 8C+ Run fixtures remain outside this fix round.
