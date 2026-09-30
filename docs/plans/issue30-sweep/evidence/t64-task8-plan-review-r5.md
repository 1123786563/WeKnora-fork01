# T64 Task 8 independent plan review — round 5

**Scope:** Latest `plan-t64-task8-atomic-admission.md`, parent `plan-t64.md` Task 8/9, proposed ADR-0015, approved Agent Marketplace spec §§10–11, Issue #64 snapshot, and current repository code. No implementation or tests run. Requirements, plan, ADR, and source were not edited.

## Verdict

- **Spec compliance: FAIL for the documented rollback path.** The forward admission and cancellation design matches the approved security requirements, but a rollback to pre-Task8 application code can reopen the AgentQA admission path after a revocation.
- **Dispatch quality: FAIL until R5-1 is resolved.** R4-1, R4-2 and R4-3 are resolved in the plan; no new file ownership or forward-interface conflict was found.

## Finding

### R5-1 — High — Code-only rollback can reopen AgentQA security admission

**Evidence:** The Task 8 migration steps say production rollback reverts application code while retaining the forward schema (`plan-t64-task8-atomic-admission.md:171–172`). The Run insert trigger covers old unpinned Marketplace Run writers (`:62`), but AgentQA security depends on the new 8B claim store and 8D handler adoption (`:82,214–223`). The current pre-Task8 `AgentQA` handler proceeds from request parsing into `executeQA` without that claim or an atomic Release verdict (`internal/handler/session/qa.go:919–962`). The approved spec forbids a Security Revoked Release from starting new Task/Run work (`docs/specs/2026-09-20-agent-marketplace-domain-model.md:146–156`).

**Impact:** Reverting to a pre-8D binary while keeping the new schema protects old Workbench inserts by failure, but can allow a new adopted AgentQA turn on a revoked Release. A code-only rollback is therefore not a safe general production rollback.

**Smallest correction:** Define a minimum rollback binary that retains 8B/8D claim admission and the security gate, or gate off adopted AgentQA and Workbench ingress before deploying an older binary and keep them closed until a compatible build is restored. Record this as an explicit rollback procedure and verify the old-binary ingress behavior in a deployment drill or targeted integration check.

## R4 resolution audit

- **R4-1 (live downgrade): resolved in the plan.** Both down migrations refuse if any claim or revocation history, pending obligation, or Run pin exists; populated-state refusal is a stated test (`plan-t64-task8-atomic-admission.md:171–172,194`). This preserves the forward schema in normal production rollback. R5-1 concerns the compatible application code used with that schema.
- **R4-2 (signed preflight staleness): resolved in the plan.** Writers are quiesced before final preflight through migration commit. The signed per-tenant report binds database identity, schema and query/result hashes, the release operator compares the frozen set at go/no-go, and raw identifiers remain in controlled deployment evidence (`:62,167`). The PostgreSQL migration holds table locks through recheck, backfill and trigger creation; SQLite uses the transactional runner with writers quiesced (`:62`). SQL is not misrepresented as enforcing human signoff.
- **R4-3 (Task 9 GORM scan): resolved.** The fixture inserts exact sidecars and uses explicit `gorm:"column:security_*"` tags for the readback (`plan-t64.md:1747–1756`).

## Interface and task boundaries

The 8A predicate, 8B resolver/claim and reconciliation APIs, 8C trusted Version/Release fields, 8D claim consumer, and 8E binding propagation are consistent at the planned boundaries (`plan-t64-task8-atomic-admission.md:40–64,147–158,214–223`). 8B owns migrations, repository/service reconciliation, worker lifecycle, DI and evidence templates; 8C and 8D own disjoint production files after 8B integration, and 8E follows 8C (`:40–46,169–184,245`). The sidecar all-or-none check and post-backfill update trigger close the prior exact-pin mutation gap (`:62,171–172,194`).
