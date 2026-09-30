# S2 Task 2 Fix1 independent Spec and quality re-review

## Scope and evidence

Reviewed the approved Craft web artifact Spec lines 64–66, `CONTEXT.md` Run/Workspace terms, ADR-0004/0008/0009, S1 Task 2 and S2 Task 1 contracts and reviews, S2 Task 2's initial FAIL review, Fix1 plan/report, incremental patch, checkpoint, and live admission/worker paths. This is the narrow Fix1 review, not complete S2 or S3 acceptance. No OCR or child agent was used.

The three live owned files match every `post_sha256` entry in the Fix1 checkpoint, and the archive members match those same hashes. Patch SHA-256: `33e92c927b3edf957357857a4452b45a94174318f14d45152e1ec82b170f3d03`; postimage archive SHA-256: `66463df4c778f987a6e68899cfa74013c60692059a76cff48f9b395174ae63d3`. The patch changes only `agent_run.go`, the repository seed tests, and the service legacy-worker fixture. `git diff --check` passed for those files.

Independently ran `go test ./internal/application/repository ./internal/application/service -run 'CraftSeed|DraftHeadAdmission|TestExecuteDurableCraftRunRejectsLegacySnapshotWithoutWorkspaceSeed|TestExecuteDurableCraftRunRestoresActorAcrossWorkerLeaseRecovery|Craft.*StartRun' -count=1`: both packages passed. `go test -race ./internal/application/repository -run 'CraftSeed|DraftHeadAdmission' -count=1` passed. A verbose targeted run confirmed that the PostgreSQL subtests skip because `TRPC_TEST_POSTGRES_DSN` is unset. These are SQLite and selected service results, not PostgreSQL parity or a green full suite.

## Findings

**Low — both admission/Advance transaction orders remain unforced.** `internal/application/repository/agent_run_craft_seed_test.go:412–456` starts the contenders behind one channel and accepts whichever scheduler result occurs. It cannot prove the specific pre-advance and post-advance ordering asserted by the Session/head lock comment in `agent_run.go:409–416`, particularly on PostgreSQL. **Impact:** a regression confined to the unobserved order could pass the focused race test and violate the exact seed linearization contract. **Smallest correction:** add controlled barriers or a narrow transaction seam that forces each order, then assert the exact revision/digest and rollback state; retain the scheduler race as a supplement. This is the preexisting Low finding, not a new Fix1 defect.

**Evidence limit — PostgreSQL behavior unverified.** `TRPC_TEST_POSTGRES_DSN` is unset, and both targeted PostgreSQL subtests explicitly skipped. The S2 plan calls for PostgreSQL admission, rollback and contention evidence before complete S2 acceptance. **Impact:** this review cannot certify dialect parity or the PostgreSQL lock-order claim. **Smallest correction:** run the same focused tests and controlled both-order checks against a disposable PostgreSQL database and record their exits. This is an unmet verification gate, not evidence of a production defect.

## Verified boundaries

- `AgentRunStore.Admit` checks the database's `(tenant_id, session_id)` `craft_sessions` registration under the Session writer transaction. At `agent_run.go:322–335`, missing, null, malformed or invalid `craft_input_manifest` is rejected for a registered Craft Session; `craftSeedAdmission` now derives from `isCraft`, so the old optional-marker branch cannot persist an unseeded fresh Craft Run. The validation precedes input-claim locking, replay lookup, head selection, slot reservation and Run insertion. The new denial test at `agent_run_craft_seed_test.go:247–299` checks no Run, slot, or claim transition for each invalid marker.
- `agent_run.go:326–327` still refuses caller-supplied `craft_workspace_seed`. Valid marked fresh Craft admissions use the owner-scoped S1 head inside the writer transaction; the selected seed and seed-inclusive request hash are stored with the Run. `agent_run.go:371–387` compares replay's canonical original intent and actor, then returns the stored Run without rereading a newer head. Existing seed replay tests passed. `TestAgentRunGenericAdmissionDoesNotRequireCraftSeed` passed for a non-Craft Session.
- `agent_run_graph_test.go:442–474` now inserts a historical unseeded Craft Run directly into `agent_runs`, claims it, and asserts worker execution fails with `craft.ErrInvalidInput` before model resolution. The test no longer obtains that row through fresh `Admit`.

## Verdict

**Spec compliance: PASS for Fix1's registered Craft admission fence.** The initial High marker-bypass finding is closed in the reviewed checkpoint. The approved current-Workspace continuation and single-writer contract remain dependent on the broader S2 and S3 work.

**Code quality: PASS for the narrow Fix1 change, with one Low race-test gap.** Focused SQLite/service tests, repository race selection, checkpoint integrity and diff check passed. **Complete S2 acceptance remains unverified** until PostgreSQL behavior and both forced transaction orders are demonstrated; this review does not mark those gates complete.
