# T19 normal provider enabled-empty stdin — independent Task 1 review

Date: 2026-09-24. Read-only review against the Task 1 plan, approved Craft web Artifact Spec, `CONTEXT.md`, ADR-0004, prior normal provider Fix3 review and patch chain. No OCR, Docker, source/test edit, commit, or remote issue action.

## Exact review range and verification

- Integration HEAD: `a5e9195acd6500c085c85d60c852148e7bbbbf34`. I replayed the initial provider creation patch and Fix1–Fix3 patches in a temporary directory. The resulting preimage SHA-256 values were `b85fb4e798a04dbdbe340774a1ad92789bc108fc5b797db46369be5b985b9c8b` (provider) and `c9afe94329a947fbadd5225307a825a867a481af123a2c86f512568c13c7ce87` (test). Applying the Task 1 patch yielded `2e88b72728d7362c069e76fc004373c3ecb7089980b08eb46987790691aa57c1` and `e42629a1a93b1e37a92347084afa7fc1fa1c72f532496e31798f0b5ae378886f`, matching the checkpoint and current files. The patch hash also matches the checkpoint.
- Independently ran `go test ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1` (PASS, 5.715s) and `go test ./internal/application/service -run '^$' -count=1` (PASS, compile). The implementer checkpoint reports a passing full sandbox package and focused race run; I did not repeat those or use Docker.
- `CreateAttachedExec` rejects disabled stdin carrying bytes before `ExecCreate`, uses the explicit flag for Docker `AttachStdin`, and records that flag, byte count, SHA-256, and timeout in the receipt (`docker_normal_exec.go:168-198`). `StartAttachedExecOnce` compares the supplied flag, count and SHA-256 before its first start inspect and the sole `ExecAttach` (`:216-252`). Enabled-empty executes the stdin writer and half-close; disabled-empty skips both (`:286-308`). `LoadOrStore` remains before attach, so competing starts have one winner (`:233-252`).
- The new tests check both empty states, their equal byte counts and SHA-256 but distinct receipts, Docker create attachment, half-close behavior, wrong flag and changed bytes rejected before start inspect/attach, and disabled-nonempty rejected before create (`docker_normal_exec_test.go:181-259`). Existing nonempty receipt, write, demux, one-send, and callback rejection tests still run. Both callback forms are rejected before Docker create (`docker_normal_exec.go:150-156`). The service callsite passes staged `StdinEnabled` to create and start and compiles (`craft_docker_normal_exec.go:177,330-334`).
- The patch also makes durable sink `Seal` return an error and marks seal failure partial (`docker_normal_exec.go:100-102,356-375,527-531`), with a focused regression test (`docker_normal_exec_test.go:335-349`). This is a provider behavior change beyond the stdin flag, but it is fail-closed, confined to the two owned files, and compatible with the current service callsite. No observed regression follows from it.

## Finding

### F1 — Low — Equal-length stdin substitution lacks a focused SHA assertion

**Evidence / affected test:** `TestDockerNormalExecRejectsMismatchedEmptyStdinEnablementBeforeAttach` (`docker_normal_exec_test.go:227-249`) exercises wrong flags and changing zero bytes to nonzero bytes. The existing nonempty test checks the expected receipt hash but does not replay a different same-length input. The SHA comparison itself is present in `StartAttachedExecOnce` (`docker_normal_exec.go:230`).

**Impact:** The implementation enforces byte identity, but the focused suite would not catch a future change that retained only the count and flag checks. This is a regression coverage gap, not an observed incorrect start.

**Smallest correction:** Add one start test that creates with nonempty stdin and supplies different bytes of equal length; assert `ErrDockerNormalExecInputMismatch`, one create inspect only, and zero attach calls.

## Verdicts

**Spec compliance: PASS for this narrow provider task.** Explicit enablement survives create, receipt, and start independently of zero-byte/hash identity; mismatches fail before Docker start, with nonempty and one-send behavior preserved. This does not attest to the later physical Task 3 or live Docker behavior.

**Code quality: PASS with low-priority F1.** The exact patch is reconstructible from the reviewed Fix3 preimage, focused tests and service compilation pass, callback rejection remains before Docker side effects, and the incidental seal-error change fails closed. F1 is recommended regression coverage before the final T19 gate, not a blocker to continuing the dependent service work.
