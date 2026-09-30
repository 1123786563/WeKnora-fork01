# Craft #107 R5 Effect Task3a Fix1 Task1 — independent re-review

Date: 2026-09-24. Scope: the exact two-file Fix1 checkpoint against the approved Craft Spec, `CONTEXT.md`, the Task3a plan, and independent findings R5E-3A-1/2. This review did not run a physical provider or duplicate the controller's Go test run.

## Verdict

- **Scoped Spec compliance: PASS.** The two original Medium findings are closed in the admitted coordinator: read-only Docker observations and identity checks happen before their one-shot claims, and the DB-only session marker commits and validates before the OpenCode create claim. No provider mutation is authorized by the marker's `maySend` value. Task fence/digest, generation, exact receipt, and unknown/no-resend behavior remain intact.
- **Code quality: PASS for this task-local patch.** The stateful tests exercise observation failure, wrong preclaim identity, state observation failure, marker failure, exact replay and one send, while retaining post-claim unknown/cancellation coverage. No new blocking finding was identified in the two owned files.
- **R5 physical admission: still blocked downstream.** The current combined provider is refused by the admitted path; a real split provider, safe network provisioning authority, DI wiring, runtime quiescence and physical evidence remain required. This review does not claim production release.

## Evidence

The preimage archive SHA-256 is `e43be1293a9dd2e69537a123ec9dac1a931a8b821a668bc96e417296ca3c3a0e`; postimage archive SHA-256 is `edac13c56c66b484d5a8cfd6a1c4f29dbbd3b178f32a9760ab9b02c409ea04e7`. The task-local patch SHA-256 is `9f9661aa1d68a9488be4eb6d53d4637a2ad10c7b22a9fe7941b63cfbac057aa2`. All match the checkpoint. Extracting both archives, applying the patch to the preimage, and hashing/comparing both outputs reproduced the postimage and current integration files byte for byte: `craft_runview_runtime.go` `806ad093...`; `craft_runview_admitted_runtime_test.go` `c4902428...`. The patch covers only the two owned files.

At `craft_runview_runtime.go:286-306`, `ObserveContainer` and `matchesAdmittedContainer` run before `beginAdmittedEffect(DockerCreate)`. At `:332-350`, `ObserveContainerState` and identity matching run before `beginAdmittedEffect(DockerStart)`. Each preclaim error returns unresolved with no effect intent or provider mutation, allowing an exact replay to claim once. The new stateful tests at `craft_runview_admitted_runtime_test.go:495-562` check these failures and successful replays. After a claim, the create/start methods still require `maySend=true`, and post-send observations determine exact success or durable unknown.

At `craft_runview_runtime.go:203-235`, `BeginSessionCreate` and its key/generation/intent validation precede `beginAdmittedEffect(OpenCodeCreate)`, followed by `CreateOpenCodeSession` only when the claim permits a send. The marker failure/replay test at `craft_runview_admitted_runtime_test.go:564-585` verifies no OpenCode claim or send on the initial failure, then one claim and one send after recovery. A canceled context is checked immediately before each claim; the existing claimed-call cancellation test still expects an unknown result and no fake side effect.

The report records focused, race, package compile and diff checks passing at the same postimage. I inspected their code and the exact postimage hashes but did not independently rerun the Go suite in this shared integration workspace. The provider's promised pure Observe methods are only an interface contract at this checkpoint; the real combined provider is rejected, so purity of a future split adapter must be checked in its own review.
