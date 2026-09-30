# F08 T20 Server-Owned Build Execution Receipt Design

Status: receipt persistence integrated; production dispatch remains blocked on upstream identity and billing interfaces; F08 remains open.

## Evidence and scope

The approved Craft Web Artifact Spec requires a successful build, an accessible preview, and an actual page-load check before a version is promoted. OCR Round 1 finding F08 showed that `output/build-log.json` is writable by the delegated build and therefore cannot establish a real process result. Round 2 already changed the trust boundary to fail closed when no server receipt is supplied, but code tracing in the integrated workspace found no production caller that supplies one. The result is safe but unusable: a valid build remains `not_run` and web promotion cannot succeed.

This is a T20/#139 assembly task. T04 owns the fixed build command and policy gate, T03 owns the uploaded-material execution decision, and T19 owns durable Docker normal-exec semantics. This design does not weaken any of those gates or infer completion from the build log.

## Existing seams

- `CraftWebBuildCommandGate.Review` validates the exact pinned build command, rechecks toolchain/path constraints, and calls the T03 execution policy before dispatch.
- `service.CraftDockerNormalExecService.Execute` accepts a durable `CraftCallBinding`, a live Docker handle, and an exact staged `CraftDockerNormalInputRequest`; it returns a provider receipt, observed process state/exit code, output snapshot, and transport completeness.
- `craftWebBuildEvidenceSource` currently reads the writable build log and, after OCR remediation, must receive server-observed receipt data before it can project `BuildRan`.
- Both the Run-view collector and post-terminal capture collector assemble artifact evidence. The capture promotion seam later calls `PromoteWebVersion`, so a collector-time boolean alone is insufficient unless bound to immutable output identity and revalidated at promotion.

## Required receipt contract

Persist an immutable receipt for one build attempt containing:

1. tenant, Task/session, workspace, Run, delegation/activity, and the exact staged request identity;
2. canonical command and argument digest, pinned runtime/toolchain/template identities, timeout and output bound;
3. Docker provider/container/exec receipt returned by normal execution;
4. observed process state, observed exit code, start evidence, transport-complete flag, and output-complete flag;
5. a digest of the exact output generation or sealed candidate manifest inspected by the build.

Only a complete observation with process state succeeded, observed exit code zero, and matching current deployment pins can prove `BuildRan=true`. Unknown, timed out, failed transport, truncated output, missing observation, or an unbound log remains unverified and prevents promotion. The receipt must not store or trust the build log's claimed exit code.

## T20 implementation tasks and interface

1. Add a durable build-receipt repository and SQLite/PostgreSQL migration pair under T20 migration ownership. Uniqueness is scoped to `(tenant, task, workspace, run, activity, request_digest)`; a conflicting request for the same logical attempt is rejected. Receipt rows are append-only after terminal observation.
2. Add the T20 build dispatch orchestration at the production Run/delegation boundary. It obtains the exact handle and `CraftCallBinding`, stages the fixed T04 command request, calls `CraftWebBuildCommandGate.Review`, then invokes `CraftDockerNormalExecService.Execute`. It persists the returned execution receipt before making evidence available. No other command may enter this path.
3. Extend the artifact evidence reader interface to resolve a receipt by the same Task/Run/activity and request identity, rather than treating `build-log.json` as process evidence. The log may remain diagnostic metadata only.
4. Bind the receipt to the captured output generation or immutable candidate manifest. `CollectCandidate` seals that identity alongside evidence. `PromoteWebVersion` re-reads the receipt and compares the candidate's Run and manifest digest inside its fenced promotion decision; changed output or a receipt from another Run fails closed.
5. Add journey tests for real fake-provider execution success, nonzero exit, unknown state, truncated output, foreign Task/Run/activity, request mismatch, output mutation after build, restart/recovery observation, and successful promotion only with the same bound receipt.

## Ownership and execution constraints

The integration controller owns only T20 assembly, the new repository/migrations, and the production journey tests. Existing F08 worker-owned files remain owned by the reviewed build-gate lane. T03/T04/T19 APIs are consumed through their current public seams; if one cannot express the fixed request or identity, stop and write a bounded interface proposal before editing those owned files. No OCR finding is closed by a design document.

Verification must include SQLite migration up/down/up, PostgreSQL migration/runtime coverage when the configured test DSN is available, focused service/container/handler journeys, full affected Go packages, `git diff --check`, independent Spec and code-quality review, and a later complete OCR workspace review.

## Verified blocker and bounded interface proposal (2026-09-28)

Independent repository tracing and architecture review confirmed that the current server cannot truthfully supply all required inputs for a build activity:

- The durable `craft_budget_grants` row provides a grant by tenant and Run, but there is no Run-level model/funding default for a separate server-owned Docker build activity.
- `craft_budget_calls` records binding facts for individual authorized model calls. Selecting the latest row is race-sensitive and is not a sanctioned billing rule. Do not reuse it as the build's `CraftCallBinding`.
- `CraftBudgetService.AuthorizeSandbox` records a separate sandbox billing facet, but `CraftDockerNormalExecService.Execute` currently enters through `CraftCallBinding`; no approved adapter maps that sandbox charge to durable normal-exec authorization.
- `CraftRunViewMaterialHandle` is an opaque verified capability, but exposes no Docker `RemoteSandboxHandle`; no production caller currently joins the RunView lifecycle, `CraftWebBuildCommandGate`, and `CraftDockerNormalExecService`.
- A client-created/structural Docker handle from `CraftRunView.Runtime.ContainerID` would cross the server-owned lifecycle boundary. Do not implement that shortcut.

The smallest safe upstream seam must have a named server owner and persist its policy:

1. Define an explicit server-authorized billing binding for the fixed build activity, including whether it consumes `AuthorizeSandbox` budget or a named model/funding binding. Do not infer this from prior model calls.
2. Add a provider-issued live Docker handle resolver that verifies the persisted RunView and binds tenant, Task/session, Workspace, Run, generation, and exact observed Docker container ID.
3. Derive a deterministic server-only activity key from the admitted Run/generation/build attempt. Recovery must resolve the same activity and observe the same durable attempt.
4. Trigger the build after successful delegation and before evidence collection; preserve T03 and T04 checks; persist complete observed process/transport/output facts and exact request digest.
5. Bind the receipt to the captured candidate manifest and re-read it under promotion fences.

Until these contracts are supplied or approved by their budget/sandbox owners, the dispatch slice cannot be implemented safely. This is an interface decision blocker; the receipt repository slice remains useful and integrated but does not close F08.
