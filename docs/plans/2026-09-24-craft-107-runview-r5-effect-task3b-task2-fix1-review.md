# Craft #107 R5 Task3b Task 2 Fix1 — independent re-review

Date: 2026-09-24. Scope: Fix1 for prior Task 2 F1/F2 only. Reviewed the Fix1 plan, original independent review, exact incremental patch, four owned files and focused tests. No OCR, Docker run, source/test edit, commit or remote issue action.

## Exact checkpoint and verification

- Fix1 preimage archive SHA-256 `b34b32f9861effd14bd36006759ffbb9ecbec730862280b2e7fe2df827df2638` equals the original Task 2 postimage archive. Fix1 postimage and patch hashes match the checkpoint: `68d7fff5037e62970d7e27178d66ae64355740c534cb3e27d687d3560517a18c` and `82228a3817f0c59eddbc86fa08b6797c16dde993cd57c609f3499367f8af2e89`. The patch passed `git apply --check --whitespace=error`; applying it to the archived preimage reproduced all four archived postimage and current integration files byte for byte. Their SHA-256 values match the checkpoint.
- Independently ran `go test -race ./internal/container -run '^(TestCraftRunViewDockerEngine.*|TestCraftRunViewAdmittedProvider.*|TestCraftRunViewStartRechecksFreshContainerIsolationBeforeSend)$' -count=1`: PASS (`5.703s` test time). The macOS linker emitted the duplicate `-lc++` warning; exit was zero. The implementer report also records focused non-race and RED evidence. No live Docker test was needed for the two fake-seam regressions.

## F1/F2 disposition

- **F1 closed.** `ObserveNetwork` and post-create network receipt validation both call `verifyRunViewDockerNetworkAttachments` (`craft_runview_docker_engine.go:113-115,146-148`). The helper rejects more than one attachment, inspects every map-key Docker container ID, and requires the inspected ID, deterministic name and complete generation labels to match (`:163-183`). An expected-looking endpoint name alone no longer authorizes a foreign ID. Negative and positive focused tests exercise these cases (`craft_runview_docker_engine_test.go:163-207`). Attachment inspection is read-only and returns an error rather than an exact receipt on uncertainty.
- **F2 closed.** Before `StartContainer`, `StartGenerationContainer` refreshes through `ObserveContainerState` and requires the same exact Docker ID plus an eligible current state (`craft_runview_container_provider.go:497-511`). That observation reinspects container, pinned image, labels, mounts, security, exact network and generation filesystem marker (`:375-412,470-485,963-1017`). Privileged drift after the earlier observation produces zero start calls in the new regression (`craft_runview_container_provider_test.go:229-250`); the existing admitted flow test asserts exactly one start send on a valid resource (`:169-226`). The post-start exact observation remains.

## Verdicts and remaining boundary

**Spec compliance: PASS for scoped Task 2 after Fix1.** Both blocking findings are corrected, and the reviewed read-only observation, split single-send and unknown-no-resend boundaries remain intact. The Task 3 coordinator must still bind each physical send to its own durable generation claim; this provider cannot grant that authority by itself.

**Code quality: PASS.** No new blocking finding in this four-file increment. Docker cannot atomically combine the final inspection and `ContainerStart`; an external daemon-side mutation between them remains a residual physical race and is documented in the source (`craft_runview_container_provider.go:497-500`). Probe output remains unrecoverable by read-only Docker inspection, and the saved real Docker test uses a no-op OpenCode session API. This review does not establish full physical admission or production DI.
