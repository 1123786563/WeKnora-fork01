# T63 Task 6 independent review

Reviewed commit `0d99b972eaedfaf6aa23a776e798d81d85206e57` against `plan-t63.md` Task 6, Issue #63 AC1–AC3, the approved Agent Marketplace spec §§9–10, ADR 0011, `CONTEXT.md`, and the accepted Task 3–5 interfaces. This was a static, read-only review of production and test code. I did not run tests or OCR.

## Verdict

- **Spec compliance: conditional.** The three tests use real SQLite migrations, marketplace services and HTTP handlers. They exercise all four exit routes, the catalog/adoption/proposal distinction, and a retired-agent HTTP 409 with no new request or Run. The admission test does not exercise the production coordinator factory, and the history test proves row counts rather than the continued presence of each seeded historical record.
- **Code quality: changes required in acceptance evidence.** No production source was changed in this commit. The two test gaps below leave AC3's production assembly and AC1's preservation claim insufficiently pinned.

## Findings

### F1 — Medium — Admission E2E bypasses production gate assembly

- **Evidence / affected symbols:** `TestLifecycleRetireBlocksNewWorkEndToEnd` constructs `workbenchservice.NewAdmissionCoordinator` and calls `SetAgentUseGate(newRealAgentUseGate(db))` at `internal/router/routes_agent_marketplace_lifecycle_test.go:329–331`. `newRealAgentUseGate` at lines 211–225 copies the closure from production `internal/container/workbench.go:63–79`. The production `NewWorkbenchAdmissionCoordinator` provider and router assembly are never called by this test. Existing Task 5 service tests also install a gate manually, and the inspected container tests do not cover this retirement wiring.
- **Impact:** The E2E test still passes if production DI stops installing the retired-agent gate, leaving real new work able to use retired variants. The test proves handler, coordinator and repository behavior under test-owned wiring, not that the deployed assembly enforces AC3.
- **Smallest defensible correction:** Build the admission handler with `container.NewWorkbenchAdmissionCoordinator` and its required real repository/target dependencies in this HTTP test (or add one narrow container-backed HTTP test). Remove the copied gate helper so the 409 and zero-write assertions depend on production wiring.

### F2 — Medium — Conservation counts do not establish preservation of the seeded history

- **Evidence / affected symbol:** `TestLifecycleExitDeletesNothingAcrossGovernanceRows` creates a Run and Artifact at `internal/router/routes_agent_marketplace_lifecycle_test.go:242–249`, then compares only table-wide counts at lines 251–277. It never queries `accepted.Key.RunID`, the artifact ID, or the review/license/version/lineage identities after the four exits. The sole identity check is for the local Agent at lines 278–283.
- **Impact:** Deleting a historical row while another row is inserted can leave every count equal and pass AC1; the test's stated conclusion that the historical Task, Artifact and governance records remain does not follow from its assertions. The direct local Agent check does cover that one row.
- **Smallest defensible correction:** Keep the count matrix and additionally snapshot the created record IDs (and relevant lineage links), then assert those exact Run, Artifact, Agent Version, Release, review and license records still exist after the exits. A focused check of the seeded Run and Artifact is the minimum needed to substantiate the nonempty history claim.

## Positive evidence and limits

- AC2 is exercised through real HTTP flows: unlisting removes the listing from catalog and rejects a new adoption while a previously created adoption receives a later upgrade proposal; a separate fixture keeps the deprecated listing visible, allows adoption of its current release, and rejects explicit use of the deprecated release with successor guidance (`routes_agent_marketplace_lifecycle_test.go:286–319`).
- AC3's HTTP response and durable-write assertions are otherwise concrete: pre-retirement start returns 202, retirement removes the Agent from available-agents, post-retirement start returns 409, and the test checks zero matching request and Run rows (`routes_agent_marketplace_lifecycle_test.go:321–349`).
- The lifecycle setup uses real migrations and repository-backed services; the release and variant helpers drive freeze, submission, review, mapping, test and publish over HTTP (`routes_agent_marketplace_test.go:349–372`; `routes_agent_upgrade_test.go:116–198`). It is therefore more than a mock-only test, subject to F1's assembly gap.
- The implementation report records passing targeted tests and build. I did not independently rerun them, per review scope.
