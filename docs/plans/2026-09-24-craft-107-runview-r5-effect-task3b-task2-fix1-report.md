# Craft #107 R5 Effect Task3b Task2 Fix1 Report

## Status

Fix1 implementation and focused verification complete; independent re-review pending. No commit was created.

## Corrections

- Network observation and post-create receipt validation now inspect every attached container using the network `Containers` map key as the Docker ID. They require the inspected Docker ID to equal that key, the inspected name and endpoint name to equal the deterministic generation container name, and the complete generation labels to match. More than one attachment is rejected. An endpoint name alone cannot authorize a Docker ID.
- `StartGenerationContainer` now performs a fresh read-only `ObserveContainerState` immediately before the single `StartContainer` call. It verifies the current inspection still resolves to the caller's exact Docker ID and an eligible state; the observation rechecks generation labels, image, security profile, and exact private network attachment. The fresh exact ID is used for the one send. The existing post-start observation remains.
- The inspect and start calls cannot be atomic at the Docker daemon. An external daemon-side mutation between the fresh inspect and `ContainerStart` remains a residual race and is documented beside the code and here. This Fix1 does not claim an atomic check/start guarantee.

## RED / GREEN evidence

Before production edits, these focused regressions failed for the reviewed behavior:

`go test ./internal/container -run '^(TestCraftRunViewDockerEngineObserveNetworkRejectsForeignAttachmentWithExpectedName|TestCraftRunViewStartRechecksFreshContainerIsolationBeforeSend)$' -count=1`

RED observations:

- A foreign Docker attachment carrying the expected endpoint name was accepted without inspecting its Docker ID.
- A container whose isolation had drifted to privileged received a Start send before the post-start inspection rejected it.

After the fixes, the foreign attachment test rejects the inspected generation-label mismatch and the positive counterpart accepts an attached container whose inspected ID/name/generation labels all match. The start regression detects current privileged drift and asserts zero start calls.

## Verification

- `go test ./internal/container -run '^(TestCraftRunViewDockerEngineObserveNetwork(RejectsForeignAttachmentWithExpectedName|AcceptsInspectedGenerationAttachment)|TestCraftRunViewStartRechecksFreshContainerIsolationBeforeSend)$' -count=1` — PASS.
- `go test ./internal/container -run '^(TestCraftRunViewDockerEngine.*|TestCraftRunViewAdmittedProvider.*|TestCraftRunViewStartRechecksFreshContainerIsolationBeforeSend)$' -count=1` — PASS.
- `go test -race ./internal/container -run '^(TestCraftRunViewDockerEngine.*|TestCraftRunViewAdmittedProvider.*|TestCraftRunViewStartRechecksFreshContainerIsolationBeforeSend)$' -count=1` — PASS.
- `gofmt -d` over all four owned Go files — clean.
- Whitespace check across the Fix1 delta — clean.
- No Docker operation was needed: both findings are proven at the fake Docker/provider boundaries. The Docker slot was released for T14 work.

Go linking emitted a duplicate `-lc++` library warning; the tests exited successfully.

## Checkpoint

- Base checkpoint: Task2 postimage archive.
- Preimage archive SHA-256: `b34b32f9861effd14bd36006759ffbb9ecbec730862280b2e7fe2df827df2638`.
- Fix1 postimage archive SHA-256: `68d7fff5037e62970d7e27178d66ae64355740c534cb3e27d687d3560517a18c`.
- Fix1 task-local patch SHA-256: `82228a3817f0c59eddbc86fa08b6797c16dde993cd57c609f3499367f8af2e89`.
- Detailed file hashes and commands: `2026-09-24-craft-107-runview-r5-effect-task3b-task2-fix1-checkpoint.json`.
