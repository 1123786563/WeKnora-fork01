# T08 Durable Run Actor Fix Plan

> **For Codex:** Execute with SDD RED → GREEN → REFACTOR, exact per-task checkpoints, independent Spec/quality Review and validators. No commit.

**Goal:** Close B1 High #126 finding: a Collaborator-triggered Craft Run is currently stored and executed as the Task Owner. Keep owner as the Task storage/foreign-key identity, but persist the authenticated initiating actor for authorization, tool attribution, replay and recovery.

**Sources:** Approved Spec #107/#126, `CONTEXT.md`, ADR-0004/0009, `t08-content-acl-review.md`, `t08-actor-principal-design.md`, T01 admission-claim review, current `agent_runs` schema. Reserve additive migration numbers PG `000194_craft_run_actor` and SQLite `000115_craft_run_actor`; verify unused before writing. Do not edit earlier migration numbers, which may have been applied. Integration HEAD `a5e9195...` plus active T19/RunView/T05 edits.

**Global Constraints:** `agent_run.go` is currently owned by T19 Task 2 and cannot be edited by another implementer until T19 checkpoint/review finishes. `craft_session.go` is released by B1 but wait for the actor contract to be reviewed. Each Task gets a backend_implementer, exact nonoverlapping files, no commits/subagents, full-content checkpoint and independent Review. Shared Worktree tests may temporarily compile RED while RunView/T19/T05 tasks edit adjacent packages; report exact blocker and rerun at a stable checkpoint. No change may grant Task Owner personal data to a Collaborator.

**Review Focus:** immutable actor from authenticated request, never payload/model/worker lease; owner remains storage key for Session/T01 claims/RunView. Different actor replay of same request ID conflicts even with identical body. Worker restart restores actor and rechecks current Task/source/connector authority. Legacy rows with no provable actor cannot silently execute personal capabilities. Generic non-Craft Run semantics remain compatible.

## Task 1 — additive actor row and admission contract

**Depends on:** T19 Task 2 finished/reviewed, RunView create-intent schema task integrated; no file conflicts. **Role:** backend_implementer. **Owned files:** new four 000194/000115 SQL; `internal/modules/agentruntime/agent/runtime/contracts.go` and focused tests; `internal/application/repository/agent_run.go` and focused tests. **Produces:** `Admission.ActorUserID`, `Run.ActorUserID`, `agent_runs.actor_user_id` and atomic immutable admission/replay rules.

1. RED: owner O/C Collaborator admission stores owner O but actor C; same request key/body from O or D conflicts; exact C retry replays; cross-tenant spoof fails; legacy Craft row missing actor fails closed for capability use. Ensure T01 claim token/owner scope still works and generic non-Craft admission retains documented behavior.
2. GREEN: migration adds nullable actor column for legacy rows, no false backfill; new Craft admission requires nonempty actor equal to trusted request scope, inserts actor in same transaction as Run/claim admission. Repository replay compares actor in addition to immutable request hash. Load exposes actor. Update tests/migration parity without weakening owner FK or admission fence.
3. Run focused repository/T01 tests, SQLite migration up/down/reapply and PG17 when available, `git diff --check`, full ignored SQL content/hash checkpoint; independent Review. If current Admission has no trusted actor signal, keep handler/service Task 2 as producer and fail closed until then; do not infer actor from owner.

## Task 2 — Craft admission supplies authenticated actor

**Depends on:** Task 1 reviewed/integrated. **Role:** backend_implementer. **Owned files:** `internal/application/service/craft_session.go` and focused tests; **ownership amendment 2026-09-23:** `internal/application/service/craft_inputs.go` and focused `craft_inputs_test.go` only for actor-bound claim digest while preserving raw durable Run request ID and owner/session claim key; `craft_session_test.go`, `craft_session_acl_test.go`, `craft_t01_test.go` only for valid active actor/user fixtures and targeted actor admission assertions; `internal/handler/session/craft_test.go` only to add active-member rows in existing owner happy-path fixtures, preserving HTTP expectations. **Produces:** owner storage scope + actor authorization scope in one admitted Run.

1. RED: real owner and Collaborator TaskWrite start Runs, actor C is persisted while owner O remains storage key; Viewer/Admin-without-grant denied; same K replay across actors conflicts; T01 continue claim cannot be stolen by another actor.
2. GREEN: derive actor only from authenticated caller context, validate tenant/user matches scope; pass `ActorUserID` to admission, bind same-actor replay, preserve owner-scoped workspace/version/input claims. No Owner fallback when collaborator starts.
3. Focused service/handler tests and checkpoint/review.

## Task 3 — worker and tool principal restoration

**Depends on:** Tasks 1–2 reviewed/integrated. **Role:** backend_implementer. **Owned files:** `internal/application/service/agent_run_graph.go`, focused worker/capability tests and narrow tool-metadata binder only by explicit ownership amendment. **Produces:** current authenticated actor for capabilities/tools after worker restart.

1. RED: Collaborator C's Run on O's Task executes on worker leases W1/W2; tool metadata/caller/principal is C, never O or W. C loses Task grant before retry → no new capability; O's private connector denied unless C has own grant. Same Run ID recovery preserves C.
2. GREEN: load durable actor from Run row before `prepareAgentCapabilities`; set Caller/Principal from actor, recheck current TaskWrite and source/connector grants, pass actor to tool registration. Keep owner separately for storage/artifact paths. No claim that a worker lease is a web principal.
3. Focused graph/tool tests, full joined Craft journey, checkpoint/review. Inspect every owner fallback in capability and MCP/connector paths; any unowned file requires a plan addendum.

## Task 4 — joined authority validation

**Depends on:** 1–3 reviewed/integrated and B1 stable. **Role:** backend_validator + independent reviewer. Exercise real authenticated Owner/Collaborator/Viewer/Admin flows through Run admission, worker restart, tool invocation and revoke. Verify same-K cross-actor replay denial, no Owner personal resource access by C, valid C-owned resource still works, and old rows without actor fail closed. Then mark B1 actor finding resolved and rerun T08 B1 final tests. Full T08 still requires B2/B3/B5 and final OCR.
