# Craft #107 T19 Normal Provider Empty Stdin Task 1 Report

Status: **DONE**

## Scope and behavior

Changed only `internal/modules/execution/sandbox/docker_normal_exec.go` and `internal/modules/execution/sandbox/docker_normal_exec_test.go` for this task. `DockerNormalExecRequest.StdinEnabled` now controls `ExecCreate.AttachStdin` independently of byte count and is copied into the immutable receipt. `StartAttachedExecOnce` now accepts the staged stdin enablement explicitly and compares it with the receipt, byte count, and SHA-256 before any preflight inspect or attach. Enabled-empty stdin therefore sends an attached empty stream and half-close; disabled-empty stdin creates without stdin attachment. Disabled with nonempty bytes is rejected before ExecCreate.

The service owner adapted its request builder and start call in its separately owned file to pass the staged flag. A compile-only service package check passed against that integration. No coordinator/output store/migration/routing, Docker, or paid egress change was made. No commit was created.

## TDD and verification

RED: Before provider API changes, the focused test compile failed because `DockerNormalExecRequest` had no `StdinEnabled` field and `StartAttachedExecOnce` had no explicit flag parameter.

GREEN evidence:

- `go test ./internal/modules/execution/sandbox -run 'TestDockerNormalExec(PreservesEnabledAndDisabledEmptyStdin|RejectsMismatchedEmptyStdinEnablementBeforeAttach|RejectsNonemptyStdinWhenDisabledBeforeCreate|CreatesInertAttachedNonTTYExec|AttachDemuxesStreamsAndHalfClosesStdinOnce)$' -count=1` — PASS.
- `go test ./internal/modules/execution/sandbox -count=1` — PASS (`11.962s`).
- `go test -race ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` — PASS (`6.674s`).
- `go test ./internal/modules/execution/sandbox -run '^$' -count=1` — PASS; package compiles.
- `go test ./internal/application/service -run '^$' -count=1` — PASS; service adapter compiles against the explicit flag API.
- `gofmt -d internal/modules/execution/sandbox/docker_normal_exec.go internal/modules/execution/sandbox/docker_normal_exec_test.go` — no output.
- The exact task-local patch passed `git apply --check --whitespace=error` against the reconstructed preimage and replayed to both recorded postimage hashes.

Focused tests cover distinct enabled-empty and disabled-empty receipts with the same zero-byte count and empty-input hash, `AttachStdin`, enabled-empty half-close, wrong enablement/bytes rejected before start inspect/attach, disabled-nonempty rejected before create, and existing nonempty behavior.

## Exact checkpoint

The exact preimage was reconstructed from the retained provider creation patch followed by reviewed Fix1, Fix2, and Fix3 deltas. Both reconstructed hashes matched the Fix3 checkpoint (`b85fb4e7…5b9c8b` provider and `c9afe943…c7ce87` test). The task-local patch is the exact two-file delta from those preimages; the checkpoint JSON records preimage/postimage hashes, patch hash, commands, and results.

## Remaining boundary

Independent review remains required before physical Task3 work. The focused provider path has no live Docker test because explicit zero-byte stream semantics are verified through the exact mocked Moby create/start seam; the earlier pinned live transport evidence remains unchanged.
