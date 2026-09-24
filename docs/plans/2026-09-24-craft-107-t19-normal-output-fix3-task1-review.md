# T19 normal-output provider Fix3 — independent Task 1 review

Date: 2026-09-24. Read-only source review against the Fix3 brief, Fix2 independent review, approved Craft web Artifact Spec (stories 34–36, interruption/recovery and false-success decisions), `CONTEXT.md`, ADR-0004, and the normal-output architecture brief. No OCR, source/test edit, commit, or remote issue action was performed.

## Exact checkpoint and verification

- Integration Worktree HEAD is `a5e9195acd6500c085c85d60c852148e7bbbbf34`; both owned provider files are untracked at HEAD. Current SHA-256/size match the Fix3 checkpoint: `docker_normal_exec.go` = `b85fb4e798a04dbdbe340774a1ad92789bc108fc5b797db46369be5b985b9c8b` / 20,300 bytes; `docker_normal_exec_test.go` = `c9afe94329a947fbadd5225307a825a867a481af123a2c86f512568c13c7ce87` / 30,011 bytes. Hashes were rechecked after verification and remained unchanged.
- Independently ran `go test -race ./internal/modules/execution/sandbox -run '^TestDockerNormalExec' -count=1`: PASS (`6.792s`). This covers the typed and legacy rejection cases plus the existing attach, demux, quota, sink, EOF and cancellation tests. The saved implementer output reports a passing full package and focused race run. The live Docker transport probe was not repeated because Fix3 does not change the physical attach or framing path.
- `CreateAttachedExec` rejects non-nil `DockerNormalExecRequest.OutputCallback` and legacy `Request.OnOutput` with `ErrDockerNormalExecCallbackUnsupported` at lines 151–153, before the only `ExecCreate` call at line 178. The adversarial typed tests use blocking and ineffective `Seal` implementations, assert neither `Deliver` nor `Seal` runs, and assert zero create/attach calls. The legacy test likewise asserts zero Docker side effects. Accepted stream output goes only to `DockerNormalExecChunkSink.Append`; the provider has one `ExecAttach` call at line 245 and no direct callback invocation.
- The focused race run retained tests for blocked conforming sink cancellation, cancellation during final inspect, clean EOF while the exact exec is running, output quota and persistence failure, and the one-attach path. These tests support no observed regression in those paths at this hash. They do not validate a real durable store or live projection.

## Finding

### F1 — Low — Saved Fix3 patch is a final file snapshot, not the stated Fix2-to-Fix3 increment

**Evidence / affected artifact:** `2026-09-24-craft-107-t19-normal-output-fix3-task1-task-local.patch` begins both owned files with `--- /dev/null` and `+++ b/...` (lines 1–2 and 589–590). The Fix3 report calls it a task-local incremental patch against the Fix2 final hashes, while the checkpoint lists those hashes as preimages. The patch therefore presents the full untracked files rather than the changes made in Fix3.

**Impact:** An independent reviewer cannot use this patch alone to distinguish Fix3 changes from earlier provider work, increasing the chance of missing a regression and weakening the task increment audit trail. The current final-file hashes and tests remain valid; this is a review-package traceability defect, not evidence of a source behavior defect.

**Smallest correction:** Save a true two-file diff between the recorded Fix2 preimages and the matching Fix3 final files, and label the existing patch as a full snapshot if retained. Verify the preimage hashes before generating that delta.

## Verdicts and downstream gate

**Spec compliance: pass for this narrow Fix3 Task 1, conditional on the stated integration boundary.** The prior High callback-lifetime finding is closed for direct caller callbacks: both typed and legacy requests are explicitly rejected before Docker create, so their arbitrary implementations cannot block provider return or run late. The accepted output path remains the separately reviewed durable sink interface; its actual store and live projection are not implemented here. Approved recovery and no-false-success behavior still require the application Tasks 2–3 to bind a verified durable store, read live output by cursor under its own cancellation contract, and gate result/version promotion on complete output and authoritative terminal evidence.

**Code quality: pass for the owned provider change, with F1 documentation repair recommended.** The exact-hash focused race suite passed, no new callback execution path is visible, and the existing one-attach/EOF/cancellation behavior remained covered. `DockerNormalExecChunkSink.Seal` is still an external synchronous call and `Append` still relies on the sink's stated cancellation/linearization contract; production routing must use a separately reviewed concrete implementation. This is an explicit dependency in the Fix3 plan, not a claim that an arbitrary sink is safe.
