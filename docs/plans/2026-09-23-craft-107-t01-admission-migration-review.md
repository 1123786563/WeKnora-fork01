# T01 / #120 admission migration independent review

Date: 2026-09-23. Read-only review of four Git-ignored SQL files at integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34` against the full-content checkpoint `.superpowers/sdd/2026-09-23-craft-107-implementation/t01-admission-migration-checkpoint-01/` (manifest SHA-256 `a3f46c8c17cfbc47f9afa21dfce4cc10196a20ad8bf0a8e7ac152fe2f21b584f`). All live files byte-match their checkpoint copies and manifest hashes: PG up `7ded4b7d201936ab211dc934416efde9295967354df5ca4d0cc3c6601af3634f`, PG down `dff4ada13df499b2f6fcafbdf57f91b31f316c22b428c0d1e72f9061af556ec7`, SQLite up `d59894a3b9b318a9f182f0b23eb69d689547599c21ead5249d4cdcb1f961fc18`, SQLite down `dff4ada13df499b2f6fcafbdf57f91b31f316c22b428c0d1e72f9061af556ec7`. Sources: approved Craft Spec/#120, `CONTEXT.md`, ADR-0004, T01 fix-2 report, admission migration plan/report, and original `craft_session_requests` migrations PG 000129 / SQLite 000049. No OCR or SQL/source edit was made.

## Verdict

- **Scoped migration Spec compliance: PASS.** Both dialects add nullable `admission_run_id`, nullable `admission_token`, non-null `admission_state` defaulting to empty, and nullable `lease_expires_at`. Existing rows receive null/empty values rather than a fabricated admitted Run. The existing composite primary key remains available for tenant/user/purpose/request lookup. PG syntax is a straightforward `ALTER TABLE ... ADD/DROP COLUMN` sequence under existing conventions; no live PostgreSQL execution was available.
- **Scoped code quality: PASS, with delivery and integration gates.** I independently applied, rolled back, and reapplied SQLite 000111 in a disposable database with a preexisting `input_decision` row and foreign key enabled. Schema and legacy row survived each step; a populated `input_admission` claim inserted and looked up after reapply. This confirms basic SQLite reversibility on the local SQLite version, not all production schema versions.
- **Full #120/R3: FAIL / not yet implemented.** These columns are only the storage prerequisite. `RunStore.Admit`, token CAS, lease/recovery logic, and the preserved RED abandoned-claim test have not changed in this checkpoint. No claim that the migration alone fixes admission recovery is warranted.

## Finding

### 1. Medium — migrations are absent from ordinary Git delivery unless explicitly added

**Evidence / affected files:** `git check-ignore -v` identifies `.gitignore:96:migrations/` for both new versions; `git status --short` does not list the four SQL paths. The checkpoint captures full contents, but the live files are not in a normal tracked or untracked diff.

**Impact:** A branch/PR or copied diff can omit the required columns while application code assumes them, causing startup/migration or admission recovery failure. The checkpoint manifest alone does not make the schema deployable.

**Smallest defensible correction:** The integration owner must explicitly include the four exact-hash files in the delivery mechanism (for Git, force-add the approved migration paths), then verify the final branch diff and migration order before enabling the RunStore changes. Do not treat the ignored workspace copies as integrated evidence.

## Verification and limits

The disposable SQLite check ran `up → down → up` against the original seven-column table shape, preserving a legacy row through each step and confirming new-row claim lookup. PostgreSQL was inspected manually; `psql` was unavailable, so live PG apply/rollback and lock behavior remain unverified. The PG `VARCHAR(64)` Run ID column is wider than the T01 fix-2 proposal's `VARCHAR(36)` and does not truncate that proposal's IDs. No CHECK constraint was added for state/token combinations; application admission/recovery transactions must enforce allowed states and fail closed on legacy empty-state rows. The migration does not alter `agent_runs` or atomic admission behavior.
