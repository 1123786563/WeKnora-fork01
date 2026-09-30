# T14 candidate layer derivation Fix 1 — independent review

## Scope and frozen evidence

Read-only review against the approved Craft web artifact Spec, `CONTEXT.md`, the relevant ADR inventory (no Craft-specific ADR), the original and Fix 1 plans, the prior review findings T14-LAYER-R1–R4, the task report, SDD ledger, shell helper, and test source. No Docker, browser, network, OCR, or test command was run.

The on-disk SHA-256 values match the Fix 1 ledger: helper `efa0207e4fb6ed585e586bf39f1fd478fb91890904cc5aa3d7e14a9ce0f592b5`, test `ee7bb91957d6d53046a95fec22fd75eda7987b5c7d154ffd6acf298730a895e1`, report `1af83aba336cc2901c9031f962b7416e235b56653d469156742751f4b75b9a00`. The pinned predecessor report matches `181139de9decadb75542841419cc75bfa86db7aafd09cb999bd402b5d4160353`; the six local overlay files match the helper's pinned digests. This review is bound to those bytes.

## Findings

### High — Commit timeout can adopt and delete another newly dangling image (T14-LAYER-R2)

**Evidence:** `derive-volume-free-diagnostics-candidate.sh:247-287` subtracts a pre-commit dangling-image snapshot, but the candidate filter at lines 262-276 never uses its fifth argument, `EXPECTED_PROBE_SHA256`. It checks layers/config/platform/tags only. Any other actor's newly dangling image with those inspect fields can become `CANDIDATE_ID`; if its embedded files fail the later verification, the EXIT trap at lines 111-123 removes that image. If its files happen to match, the helper may retain and announce an image it did not create. The four fake-Docker scenarios supply only the helper's one new candidate and never add a concurrent newly dangling image.

**Impact:** Concurrent Docker activity can cause deletion or adoption of an unrelated image, violating the exact provenance and safe-cleanup contract. The inventory checks do exclude an image present in the pre-commit snapshot, but do not establish ownership of a newly appearing image.

**Smallest correction:** Do not assign `CANDIDATE_ID` or remove an inventory image until its exact probe content and metadata, all six embedded hashes, and a reliable operation identity have been established. If Docker cannot provide a reliable commit identity, treat a matching inventory entry as an uncertain outcome requiring operator reconciliation; do not delete it automatically. Add pre-existing and concurrent dangling-image fixtures, including a same-ancestry image with a wrong probe.

### High — A real subprocess timeout leaves no budget for create reconciliation (T14-LAYER-R2/R3)

**Evidence:** `docker_bounded` at lines 71-79 sets the subprocess timeout to the entire remaining 840-second budget. On a true `TimeoutExpired`, the following create-failure branches at lines 164-179 and 321-335 call `docker_bounded container inspect`; its calculated remaining time is then zero or negative, so inspection cannot execute. The fake's `create_timeout` at test lines 127-129 creates an object and exits 124 immediately; the fake's `commit_timeout` at lines 154-155 does the same. Neither case waits until `subprocess.run` raises `TimeoutExpired`.

**Impact:** The tests pass for a quick nonzero CLI exit while the actual timeout-after-daemon-create path remains `unknown_outcome` and cannot perform the promised bounded name/full-ID reconciliation. The commit path has the same exhausted-budget limitation for inventory. An orphaned temporary container can retain anonymous volumes.

**Smallest correction:** Reserve a separate, explicit reconciliation budget before starting a mutating Docker call, or use a shorter operation budget within the total bound; make the recovery operations use that reserve. Fake a process that remains alive until the Python wrapper's own timeout expires and assert the resulting inspected/cleaned state or explicit unresolved state.

### High — Failed verification-container removal need not permit candidate removal (T14-LAYER-R1/R3)

**Evidence:** In `cleanup` at lines 103-123, a failed `docker rm -v` of the verification container sets failure, then the helper runs `docker image rm "$CANDIDATE_ID"` without first clearing the container reference. A Docker image still referenced by an existing stopped container normally conflicts with unforced image removal. The fake at test lines 119-120 deletes any image unconditionally, even when its `containers` map still contains the verification container. The cleanup-failure test at lines 270-275 therefore proves the intended shell branch runs, not that the candidate is actually removable in the failure state it injects.

**Impact:** A failed verification-container cleanup can leave both that container and the unapproved candidate. The helper exits nonzero and avoids a success announcement, but the prior finding's removal requirement is still unproven and likely fails in Docker.

**Smallest correction:** Reconcile/retry the container removal within the shared cleanup deadline; if necessary, use an explicit safe image-removal method that handles stopped references, and report the retained identities if removal still fails. Make fake `image rm` reject an image referenced by a container and assert final state under each cleanup failure.

### Medium — Temporary-directory removal can fail after candidate-removal decision (T14-LAYER-R1)

**Evidence:** `cleanup` decides whether to remove `CANDIDATE_ID` at lines 111-123. It then runs `rm -rf "$TEMP_DIR"` at lines 124-127. If that removal fails, it changes `STATUS` to 1, but never returns to candidate removal. `VALIDATION_COMPLETE=1` then emits `candidate_retained=false cleanup_unverified=true`, while the image remains. No fake scenario injects local temporary-directory cleanup failure.

**Impact:** An unsuccessful final cleanup can leave a candidate after a failing run, contradicting the Fix 1 plan's candidate-retention gate.

**Smallest correction:** Complete local cleanup before making the final candidate-retention decision, or route any later cleanup error back through verified candidate removal. Add a focused failure fixture if this guarantee is required for local cleanup as well as Docker objects.

## Finding resolution and test assessment

- **R1:** Partly addressed. Success announcement is deferred until the EXIT trap, and failed container removal sets a failing exit. Candidate removal under an extant verification-container reference and local cleanup failure remain gaps.
- **R2:** Partly addressed. Unique names and a pre-commit dangling snapshot improve recovery. A true total-budget timeout cannot reconcile, and a concurrent newly dangling image is not safely attributable to this commit.
- **R3:** Partly addressed. Four behavioral cases invoke a copied version of the real shell helper through `sh` with a stateful fake executable named `docker`; they exercise a complete happy path and three fast error branches. The test rewrites fixed pins/report path and clamps timeout expressions in that copy. Its two “timeout” cases are immediate exit-124 failures, and its image-removal model omits container-reference conflicts. Thus the tests are genuine shell execution but do not prove the central real-timeout and failed-cleanup contracts.
- **R4:** Resolved. The report now consistently says one shared 45-second cleanup window.

## Independent verdict

**Spec compliance: blocked for the Fix 1 helper and its approved plan.** Exact predecessor/source/report pins, six local overlay hashes, never-started checks, single-path diff, layer/config/platform checks, and delayed success output are present. The lifecycle and provenance gaps above prevent a trustworthy derivation/cleanup guarantee. The approved product Spec's browser, egress and version-promotion acceptance remain outside this helper and were not claimed or tested here.

**Code quality: changes required.** The fake-Docker harness is useful evidence that the shell logic runs, but the untested timeout semantics, image ownership ambiguity, and unrealistic cleanup model leave material correctness and data-consistency risks. No live candidate should be derived from this checkpoint until the high findings are corrected and independently re-reviewed.
