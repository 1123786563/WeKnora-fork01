# T08 Task 1 — Durable Run actor admission report

Date: 2026-09-23. Scoped to Task 1 of `2026-09-23-craft-107-t08-actor-principal-plan.md`. No commit was created. Craft service/worker callers, full B1 acceptance, and T08 completion remain open.

## Result

- Added `ActorUserID` to runtime `Admission` and `Run`, preserving `UserID` as the Task storage owner.
- `agentRunRow` persists and loads `actor_user_id`; new Craft admissions require an explicit actor and reject actors that are inactive, deleted, or not in the Run tenant. The repository does not infer the owner as actor for Craft sessions.
- Owner-scoped idempotent replays now compare the actor. A different actor using the same tenant/owner/request/body conflicts; an exact actor retry returns the existing Run.
- A legacy Craft row with NULL `actor_user_id` remains explicitly actor-unknown on `Get` and cannot be replayed as an actor-bearing Run. This allows the later worker task to fail closed.
- Generic admissions with no separate actor continue to persist the owner as actor. Existing owner storage/session-lock/input-claim scope remains unchanged.
- Updated a focused T01 input-claim repository test to provide the explicit actor so it continues reaching the intended claim-fence assertion.

## Changed source files

- `internal/modules/agentruntime/agent/runtime/contracts.go`
- `internal/application/repository/agent_run.go` (also contains concurrent T01 and T19 changes; they were preserved)
- `internal/application/repository/agent_run_test.go` (two actor inputs added to preserve the existing T01 assertion under Craft fail-closed validation)
- `internal/application/repository/agent_run_actor_test.go` (new)

The four reviewed SQL migration files were not modified. Their current SHA-256 values exactly match the independent PASS review:

```text
9ebc9b93902ba0e893d302b2371fffae4b074cec7e5c61102005bf5a9bbd0b28  migrations/versioned/000194_craft_run_actor.up.sql
cb429432a13c89f4de482d340f13a45ca09d6c5b1abd4db117805091d204e115  migrations/versioned/000194_craft_run_actor.down.sql
9ebc9b93902ba0e893d302b2371fffae4b074cec7e5c61102005bf5a9bbd0b28  migrations/sqlite/000115_craft_run_actor.up.sql
9d9a2350e3d176b4237c62c5a188177c12077cf40596485ee29d4589fc69a589  migrations/sqlite/000115_craft_run_actor.down.sql
```

Final owned source/test SHA-256 values are:

```text
5b1e82e65c1b7e2f9635d3e296d21fb2a2163f7d0d7d2d9a4ad91d8efb93ad2a  internal/application/repository/agent_run.go
06e0956577b38d034da21f556d1c5aed5ea36827f4c3a61e8f0629c3b50f75ae  internal/application/repository/agent_run_test.go
61826e519d67971f01281800d269f696e53d68c6167771bccc4166ff73d166b6  internal/application/repository/agent_run_actor_test.go
345c0126722f5c88c1841a5990368fa5714426f8ad4f95c21d06588f18c830f7  internal/modules/agentruntime/agent/runtime/contracts.go
```

The independent migration report records SQLite up/down/reapply and PostgreSQL 17.9 transactional up/down/reapply passing; the actor migration review is PASS. This Task reused those results because its SQL bytes are unchanged. The four complete SQL contents and checksums are also copied into this Task checkpoint.

## TDD and verification

- RED: before adding the contract fields, `go test ./internal/application/repository -run '^TestCraftRunAdmissionStoresOwnerAndAuthenticatedActorSeparately$' -count=1` failed to compile because `Admission.ActorUserID` and `Run.ActorUserID` were undefined.
- GREEN: `go test ./internal/application/repository -run 'TestAgentRunAdmission|TestCraftRunAdmission|TestGenericRunAdmission' -count=1` passed. This includes existing admission/T01 tests and new cases for owner-versus-actor storage, load/replay, cross-actor replay conflict, cross-tenant actor rejection, missing actor, legacy NULL actor, and generic owner fallback.
- GREEN: `go test ./internal/modules/agentruntime/agent/runtime -count=1` passed.
- `git diff --check -- internal/modules/agentruntime/agent/runtime/contracts.go internal/application/repository/agent_run.go internal/application/repository/agent_run_test.go internal/application/repository/agent_run_actor_test.go` passed.
- PostgreSQL repository subtests were skipped because `TRPC_TEST_POSTGRES_DSN` is unset. PostgreSQL evidence for the unchanged migration only is from the prior migration report, not this repository behavior.

## Exact checkpoint and limits

Checkpoint: `.superpowers/sdd/2026-09-23-craft-107-implementation/t08-actor-admission-checkpoint-01/`. It contains full pre/post sources for owned existing files, the new test source, task delta patch, migration file bytes and hashes, HEAD, complete `git status --short`, and before/after SHA-256 manifests. `agent_run.go` before bytes are from the immediately preceding T19 pending-cancel checkpoint; the reconstructed before copies of contracts and `agent_run_test.go` remove only this Task's actor additions while retaining shared T01 edits.

Craft service callers still do not supply `ActorUserID` and therefore fail closed for Craft admission until Task 2 passes the authenticated caller through. This is intentional under the plan's instruction not to infer actor from owner. This Task does not authorize Collaborator execution: Task 2 must verify authenticated TaskWrite authority, and Task 3 must restore actor for worker/capability use and fail closed on legacy rows. Independent Task 1 review is pending.
