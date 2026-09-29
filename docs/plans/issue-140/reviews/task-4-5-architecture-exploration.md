# Task 4/5 architecture exploration

Read-only exploration at integration HEAD `320df1db2`. No production files were modified.

## Task 3 dependency status

Task 3 is not an approved dependency at this checkpoint. The integration worktree does not contain its profile, opportunity, source, or evaluation implementation. The latest candidate exists in a separate worktree at `7174a98e`, but its independent validator/reviewer status must be checked and its changes must be integrated before Task 4 can consume it. Therefore this report does not treat Task 3 interfaces as stable.

## Task 4 ownership and seams

Task 4 owns `internal/modules/career/search/**`, `rule/**`, `admission/**`, local Career handlers, and tests. It is required to consume reviewed Task 3 seams for confirmed profile facts/snapshots, opportunity snapshots (ID, revision, digest, source, observed time, completeness), and evaluation result/tri-state/evidence.

Task 4 must call the existing Workbench `AdmissionCoordinator.Start(StartInput)` and `TaskBudgetPort.Ensure/ReleaseUnstarted`; it must not modify Workbench admission internals. Existing admission owns `(tenant, owner, request_id)` replay/hash/recovery and budget reservation. Existing owned read seams are `AgentRunStore.GetOwnedRun`/`GetRunForGrantedReader` and `WorkbenchListStore.ReadTaskFactsForRun`; granted reads remain read-only.

Required behavior is: merge only with strong job/employer/location/batch evidence; keep uncertain duplicates separate; preserve every source URL and check time; annotate JD changes, expiry, and removal without mutating historical application snapshots; retain last success and stale time after source failure; leave reads available after budget denial; map one application to one Workbench Task; reconcile same-key retries without duplicates.

The DAG currently places `source/**` under Task 3 while Task 4 requires source reconciliation. This is an ownership conflict that must be resolved after Task 3 review: Task 4 should use a separate reconciliation package/adapter, or the controller should explicitly transfer ownership. Two implementation agents must not edit `source/source.go` concurrently.

## Task 5 ownership and seams

Task 5 owns `internal/modules/career/application/**`, `material/**`, `submission/**`, export/Artifact adapters, and tests. It owns no `preparation/**`; Task 6 is the sole owner of that area. Task 5 consumes an admitted application, frozen opportunity snapshot, confirmed facts, Workbench Task ID, and the Artifact permission port. It produces a versioned structured body, immutable published PDF/DOCX Artifact references and digest, and an explicit user submission with actual channel/time/version or explicit unknown.

Task 5 must invoke trusted `ArtifactCatalogStore.BindVersion` after a ready immutable version is created. Clients must not supply artifact metadata or URLs. The current binding validates authenticated scope and ready Workbench version, locks the owned session, and `WithResolved` rechecks authorization/revocation while downloading. The #142 publisher gap remains open until a real Career material publisher invokes this seam.

Required invariants are: edits create a new version while old body/digest remain immutable; only confirmed facts can become claims; PDF and DOCX derive from the same structured-body digest and are inspected before publication; corrupt or missing exports remain unpublished; submission never invokes an external platform; unknown actual version is retained without guessing the latest; duplicate confirmation creates one submission event.

## Serial and shared-file constraints

Task 4 and Task 5 cannot be implemented concurrently as backend domain tasks: Task 5 consumes Task 4's admitted application and both need the application/receipt contract boundary. Task 4 may only split into search/reconciliation, rule, and admission substreams after Task 3 read interfaces and Task 4 internal DTOs are frozen. Central router/container/migration registration is integration-owner-only.

Shared files requiring serial coordination are `packages/contracts/src/career/*` if additions are needed, paired migration files under `migrations/versioned` and `migrations/sqlite` with a unique version, and `internal/container/container.go` plus `internal/router/router.go`. Existing Career contracts already include application, snapshot references, material version/reference, submission, and search request/receipt types.

## Current evidence limits

At this integration HEAD the Career repository remains generic `Record`/`Receipt`/`Evidence`; no application/material/submission persistence or handlers exist, and the Career handler is a placeholder. Consequently any later Task 4/5 brief must specify concrete domain records, repository/service seams, migrations, and route wiring. Do not finalize an executable brief until Task 3 has passed independent review and its read interfaces are frozen.

Planned verification: Task 4 Go tests over Career, Workbench, router, and container; Task 5 Career tests plus real PDF/DOCX parser/inspection fixtures and API-client response tests, race tests for same-key replay and immutable publication/submission, and migration up/down tests.
