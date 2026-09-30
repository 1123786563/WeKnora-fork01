# Craft #107 R5 Effect Task3b Task 2 — independent review

Date: 2026-09-24. Scope: split physical Docker/OpenCode provider only. Reviewed the Task 2 brief, Task 1 Fix1 review, approved Craft spec, ADR/CONTEXT boundaries, provider/engine sources and focused tests, and saved live Docker evidence. No OCR, Docker run, production/test source edit, commit, or remote issue action.

## Checkpoint and checks

- Integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Preimage archive, postimage archive and incremental patch SHA-256 match the checkpoint: `9fca5a4cedb787571217a3d24d89049ebe79e20a3e53bac8b3d833e2aebf9238`, `b34b32f9861effd14bd36006759ffbb9ecbec730862280b2e7fe2df827df2638`, and `6d855cdf884b045bb5f035dee5eaea26359eac715509c246824cee3d2e28d33c`. Applying the patch with whitespace checking to the archived preimage reproduces all four archived postimage files and current integration bytes, including the recorded file hashes.
- Independently ran `go test ./internal/container -run '^(TestCraftRunViewDockerEngine.*|TestCraftRunViewAdmittedProvider.*)$' -count=1`: PASS (2.539s; linker warned about duplicate `-lc++`). The report records passing focused race and disposable real Docker tests. The live test verifies pinned image/runtime hashes, network/container/probe happy path and the corrected in-container read-only knowledge mount challenge, but uses a no-op OpenCode session API; it does not prove live session GET/POST.
- The admitted Observe call graph uses `NetworkInspect`, `ContainerInspect`/`ImageInspect`, generation filesystem reads, and OpenCode inventory GETs. `ObserveRuntimeProbe` returns unresolved without Docker exec. These methods do not call the legacy combined create/start/probe APIs or mutate the generation filesystem. Split network create sends one `NetworkCreate`, then inspects the returned ID; container create sends one `ContainerCreate`, then inspects; start sends one `ContainerStart`, then observes; probe send performs one `ExecCreate` and one `ExecAttach`, then inspects the exact exec. Errors do not trigger in-method resend. OpenCode create sends one POST and confirms the returned ID by GET. These are adapter guarantees; the durable claim remains the Task 3 coordinator's responsibility.

## Findings

### F1 — Medium — Network attachment validation accepts a foreign container ID with the expected endpoint name

**Evidence / affected symbol:** `verifyRunViewDockerNetwork` in `internal/container/craft_runview_docker_engine.go:147-163` accepts each entry of Docker's `actual.Containers` when either its map key equals the deterministic container name **or** `endpoint.Name` equals that name. A map key containing a different Docker container ID passes whenever its endpoint name matches. `ObserveNetwork` and `CreateGenerationNetwork` return this as exact network evidence. The focused network tests cover empty attachments, not this conflicting ID/name case.

**Impact:** A network with an independently attached foreign container can be treated as a clean generation network. This weakens the explicit foreign-attachment and exact Docker-source receipt requirement before subsequent physical operations. Later container inspection may reject some combinations, but the network receipt itself is falsely accepted.

**Smallest correction:** Treat Docker attachment IDs as container IDs: for a nonempty attachment set, inspect the attached container by the map key and require its Docker ID, deterministic name and generation labels to match the expected generation. Reject all other IDs, including an expected-looking endpoint name on a foreign ID. Add a fake test for that exact mismatch.

### F2 — Medium — Start trusts a prior observation without checking current Docker identity before the send

**Evidence / affected symbol:** `StartGenerationContainer` in `internal/container/craft_runview_container_provider.go:490-512` validates fields of the caller-supplied `CraftRunViewContainerObservation` and immediately calls `p.engine.StartContainer(ctx, expected.DockerID)`. It calls `ObserveContainerState` only after the start. Network attachment can change between the earlier observation and this method, while the supplied struct stays valid.

**Impact:** A now foreign-attached or identity-drifted generation container may be started before the provider discovers the drift. A post-start error is fail-closed for returning a receipt, but does not undo the physical start or its exposure. This conflicts with the brief's foreign attachment/security fail-closed condition at the start boundary.

**Smallest correction:** Immediately before `StartContainer`, read the current container/network state and require the same exact Docker ID and eligible state with all isolation checks passing. Use that fresh ID for the one start send, then retain the existing post-start inspection. Add a fake test that changes attachment or labels after the earlier observation and asserts zero start calls. Docker cannot make this check and start atomic; document that residual daemon-side race at the physical admission gate.

## Verdicts and boundary

**Spec compliance: FAIL for Task 2 pending F1–F2.** The read/send split, one API call per send invocation, exact-ID post-send inspection, unknown-no-resend behavior, image/security inspection, and read-only probe limitation are substantially implemented. The two findings leave foreign network ownership and pre-start isolation insufficiently enforced.

**Code quality: CHANGES REQUIRED.** Focused tests and patch replay pass, but the tests omit the two adversarial identity transitions above. Probe replay remains parked because read-only Docker inspection cannot recover measured stdout hashes. The live session API is a no-op in the Docker test, so this review does not authorize full physical admission or production DI.
