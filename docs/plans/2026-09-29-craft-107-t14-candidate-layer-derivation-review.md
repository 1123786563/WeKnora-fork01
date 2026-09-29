# T14 candidate layer derivation — independent review

## Scope and evidence

Read-only review of `derive-volume-free-diagnostics-candidate.sh` (SHA-256 `de3cb6d8e06e388c43cb9e58d6f39593298e36d339131de81df9cb78298483f7`), its six static tests, implementation report, layer derivation plan, failed full-import report, pinned predecessor report, exact-source export report, approved Craft web artifact Spec, `CONTEXT.md`, and relevant ADRs. The predecessor report's on-disk SHA-256 is the script's pinned `181139de9decadb75542841419cc75bfa86db7aafd09cb999bd402b5d4160353`. No Docker, browser, network, OCR, or runtime test was invoked for this review.

## Findings

### High — Failed final cleanup can retain an unapproved candidate

**Evidence:** `derive-volume-free-diagnostics-candidate.sh:295-300` sets `SUCCEEDED=1` before the `EXIT` trap removes the verification and derivation containers. In `cleanup` at lines 99-119, a failed or timed-out `docker rm -v` sets the exit status to 1, but candidate removal is gated on `SUCCEEDED=0`. Thus a cleanup failure returns failure while leaving the new image in Docker. This contradicts the plan's Global Constraints and Failure handling, which require any cleanup discrepancy to fail closed and remove the newly created candidate.

**Impact:** The candidate ID is printed at line 246 and remains available even though required cleanup and the acceptance gate failed. A caller or later operator could consume an image from an incomplete derivation.

**Smallest correction:** Mark success only after both container removals are confirmed; on any cleanup failure, re-inspect and remove the candidate by its full ID, then return failure. Report a retained image explicitly if removal also fails. Add a behavioral test that injects failed verification-container and derivation-container removals and asserts no success or retained candidate.

### High — Timed-out create/commit can leave an untracked Docker object

**Evidence:** `docker_bounded` at lines 67-76 bounds the Docker CLI process. Container and candidate IDs are assigned only after command substitution returns at lines 153-158, 220-225, and 249-254. If Docker creates the object, but the CLI times out or fails before returning its ID, `CONTAINER_ID`, `VERIFY_CONTAINER_ID`, or `CANDIDATE_ID` remains empty; `cleanup` at lines 99-119 cannot inspect or remove that object. Killing a Docker CLI does not establish that the daemon cancelled a create or commit. The same loss can occur on an interrupted command substitution.

**Impact:** The stated guarantee to remove the never-started containers and newly created candidate on every failure path is not established. An orphaned image can outlive a failed bounded run, and an orphaned container can hold anonymous volumes. The 840-second bound limits the client wait, not completion of a daemon-side operation.

**Smallest correction:** Use uniquely named derivation and verification containers whose names are recorded before Docker calls, then reconcile and remove them by inspected full ID on timeout. For commit, persist/reconcile an identifiable result or explicitly make timeout an unknown-outcome state requiring a bounded Docker inventory and operator reconciliation before further use; never claim guaranteed deletion without a recoverable candidate identity. Test timeout-after-create and timeout-after-commit behavior with a Docker CLI fake that creates an object but suppresses its ID.

### Medium — Static tests do not exercise the failure contract

**Evidence:** `test_derive_volume_free_diagnostics_candidate.py:31-69` checks string presence and absence. The only subprocess execution is the extra-argument usage check at lines 70-73. It does not simulate Docker inspect output, tar metadata, `docker diff`, commit ancestry, timeout, cleanup failure, or the successful path. The implementation report describes these as static checks and correctly says the helper was not invoked.

**Impact:** The tests passed while the two failure paths above remained. They do not provide behavioral evidence for the plan's `no success without complete verification` or cleanup guarantees.

**Smallest correction:** Add a fake-Docker behavioral harness for the bounded helper, with at least a complete success fixture and injected cleanup and timeout failures. Keep live Docker validation as the separate parent gate.

### Low — Implementation report misstated cleanup timing

**Evidence:** `candidate-layer-derivation-report.md:16` says a 45-second *per-command* cleanup bound, while script lines 97-98 establish one `cleanup-start` timestamp and lines 81-85 decrement a shared 45-second deadline. The report itself states the shared deadline correctly at line 28.

**Impact:** A reader could overestimate the time available to reconcile multiple objects and misunderstand the cleanup behavior.

**Smallest correction:** Change line 16 to “one shared 45-second cleanup bound.” This is report wording only.

## Independent verdict

**Spec compliance: blocked for this helper's approved plan.** The pin to the exact predecessor ID and matching report digest, source ID, six before/after hashes, never-started state checks, exact `docker diff` path, one-layer ancestry, config/platform/tag/volume checks, and probe ownership/mode checks are present in source. The approved plan also requires fail-closed cleanup on every exit path. Findings 1 and 2 leave that requirement unsatisfied. This helper has not been run against Docker, so neither candidate construction nor T14's browser/egress acceptance is established.

**Code quality: changes required.** The identity and content checks are specific and auditable, and Docker copy tar headers are used to check actual numeric owner, group, and mode rather than inferring them from the runtime user. The untested lifecycle transitions and unknown daemon outcome on timeout are material correctness gaps. Resolve both high findings and add behavioral failure tests before the parent uses the helper for a live candidate.
