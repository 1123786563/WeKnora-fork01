# Independent Task Review — T63 Task 6 F2, round 1

**Scope:** `0d99b972eaedfaf6aa23a776e798d81d85206e57` → `707b0f803dfd3c7cf31271de1b1acdda5dab1fe7`; one test file, `internal/router/routes_agent_marketplace_lifecycle_test.go`. I read the supplied diff package first and compared it with the revised `plan-t63-task6-review-fix.md` Task 1, Issue #63 AC1, the approved marketplace spec §§8–10, ADR-0011, and `CONTEXT.md`. F1 production admission wiring is expressly assigned to `plan-t63-task6-production-wiring.md` and is outside this review.

## Finding

**F2-R1-1 — Medium — Historical Adoption and pinned Release lineage can change while AC1 passes.**

- **Evidence / affected symbols:** In `TestLifecycleExitDeletesNothingAcrossGovernanceRows`, `internal/router/routes_agent_marketplace_lifecycle_test.go:259-281` captures variant, release, submission, review, license, listing, agent, Run, and Artifact, but does not capture the seeded `AgentAdoptionEntity`. At lines 320–327 the post-exit variant check covers `AdoptionID`, `LocalAgentID`, and `LocalAgentVersionID`, but omits `variantAfter.ReleaseID`. No post-exit query fetches the exact `adoptionID` row or checks its `ListingID` and `AcceptedReleaseID`. The aggregate `agent_adoptions` count at lines 293–309 does not close that gap. `AgentAdoptionVariantEntity.ReleaseID` and `AgentAdoptionEntity.AcceptedReleaseID` are the persisted links to the fixed source Release (`internal/types/agent_adoption_persistence.go:20-39`).
- **Impact:** An exit could orphan the variant by removing its Adoption and adding another Adoption, or silently repoint the variant/adoption to another Release, while all new identity and count assertions still pass. That misses the fixed Release provenance required by approved spec §8 and ADR-0011, and the F2 brief's explicit lineage/source-identity assertion. The test therefore does not yet fully substantiate AC1's history-preservation claim.
- **Smallest defensible correction:** Capture the original Adoption row and the variant's `ReleaseID` before exits. After all four exits, fetch the Adoption by tenant and exact ID, assert its `ListingID` and `AcceptedReleaseID` equal the captured values, and assert the reloaded variant's `ReleaseID` equals its original value. Check that these pinned Release IDs resolve to the retained Release rows. Keep the existing count and identity assertions.

## Verdict

- **Spec compliance for scoped F2:** **Not yet met.** The change strengthens AC1 evidence for exact Run, Artifact, review, submission, listing, license, and local version rows, but leaves a material Adoption→Release / Variant→Release provenance gap.
- **Code quality:** **Changes requested.** The added queries are tenant-scoped where the schema supports tenancy, the assertions run after all four HTTP exits, and no production source changed. The missing lineage assertions are a behavioral test coverage defect, not a production code defect in this diff.
- **Verification evidence:** The implementer report records a passing targeted router test and `git diff --check`; I did not rerun tests or OCR for this read-only independent review. Passing tests do not establish the omitted lineage behavior.
- **Task acceptance:** **Fail pending F2-R1-1 correction and review.** F1 remains a separate follow-up and does not affect this scoped finding.
