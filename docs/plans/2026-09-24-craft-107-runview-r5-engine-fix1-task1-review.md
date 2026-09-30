# Craft #107 T01 RunView R5 engine Fix1 Task 1 — independent review

Date: 2026-09-24. Scope: the Fix1 delta in `craft_runview_docker_engine.go` and its test, at integration HEAD `a5e9195acd6500c085c85d60c852148e7bbbbf34`. Read the approved Craft web Artifact Spec, `CONTEXT.md`, ADR-0003, R5 engine plan, prior Task 1 review, Fix1 plan, report, patch and checkpoint. Source and tests were read only; no OCR was run.

## Findings

No critical, high, medium or low finding in this Fix1 scope.

## Spec compliance verdict: PASS for Fix1 scope

- The previous high finding is closed. `hasUnsafeRunViewContainerSettings` (`internal/container/craft_runview_docker_engine.go:513-531`) accepts only empty inspected PID mode; empty or explicit `private` IPC mode; empty UTS and user namespace modes; and empty or explicit `private` cgroup namespace mode. `container:<id>`, `host`, shareable IPC and unknown modes fail before the adapter returns facts for provider acceptance. The fake-inspect cases at `craft_runview_docker_engine_test.go:194-242` exercise each rejected category and allowed private values.
- The previous medium finding is closed. The live test takes the actual knowledge destination from inspected mounts (`craft_runview_docker_engine_test.go:371-382`), reads the host-created challenge through that exact container path and checks successful exit and original bytes (`:383-402`), then attempts a root append at the same path. It requires nonzero exit, the specific `read-only file system` error and unchanged host bytes (`:403-420`). The existing test cleanup removes and reinspects its container and network (`:327-342`).
- This verdict covers the R5 Engine Task 1 Fix1 acceptance only. Production DI and a deployable registry digest remain separate R5 Task 2 gates; the local image digest in the live proof does not satisfy those gates.

## Code quality verdict: PASS for Fix1 scope

The allowlist is small and fail closed for unrecognized inspected values. The live assertion distinguishes a missing path from an actual read-only mount denial and confirms host content integrity. No concurrency, data consistency, security or regression issue was found in the changed paths. The prior R5 Task 1 review covers the unchanged adapter and provider paths.

## Verification and limits

- All final engine and test hashes match the Fix1 checkpoint: `33965067fb21158b5e9ba6d748c18787a210b2743c914527c71f53f8afe01649` and `ae300a303ea43ad9f801e2ab9aa91de97944b7cced255437e93d2b3e24ba9a50`. The unchanged provider, provider test and H2 fixture match the recorded preimage hashes. Report, patch and preimage hash manifest also match their checkpoint hashes. The two final hashes remained identical after the independent focused test run.
- Independent `go test ./internal/container -run '^TestCraftRunViewDockerEngine|^TestCraftRunView(ContainerProvider|MaterialHandle|MaterialLayoutIdentity)' -count=1` passed (exit 0). The linker warned about duplicate `-lc++` libraries without failing the test. `git diff --check` for the two owned files passed.
- The implementer report records passing focused race tests and a disposable Linux/arm64 Docker run with successful destination read, EROFS write denial, unchanged host bytes and verified container/network absence. This review inspected the exact matching test and checkpoint but did not duplicate the resource-creating Docker run.
