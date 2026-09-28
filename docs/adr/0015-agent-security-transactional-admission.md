---
status: proposed
date: 2026-09-29
---

# Serialize Agent security decisions with new work admission

## Context

Issue #64 and the approved Agent Marketplace specification require a revoked Release or an exactly locked revoked dependency to block new Task/Run work while retaining historical data. The current AgentRun admission transaction serializes on the session row, while security revocation serializes on the tenant row. A standalone verdict read before admission can race a revocation, and reversing the lock order can deadlock. Workbench also preserves idempotent replay of an existing request. AgentQA has quick-answer and agent modes, persists uploads before its current entry point returns, and has no durable turn admission record that revocation can enumerate.

## Decision

New use of an adopted Agent is admitted only by a short transaction guarded by the source tenant's security lock. The transaction resolves the published local Variant to its fixed Release, rejects ambiguous Variant mappings, evaluates that Release and its exact dependency lock using the revocation rows on the same transaction, and commits the durable Run or chat-turn admission before any external work. The tenant guard is acquired before session, request, Variant, or Run rows. For shared Agents, source-tenant and session-tenant guards are acquired in ascending tenant-ID order before session/share/Variant/claim rows. Existing admitted-request replay remains available and does not become a new admission. Variant admission binds both the local Agent ID and the server-derived immutable `LocalAgentVersionID`; stale, mismatched, unpublished, or ambiguous mappings fail closed, and the chosen Version is frozen in the Run or claim.

AgentQA receives a durable, tenant-scoped turn claim uniquely keyed by `(session_tenant_id, session_id, owner_id, request_id)`. The claim records a canonical request hash, server-generated assistant message ID, source tenant, session tenant, session, local Agent Version, pinned Release, lease owner/expiry, monotonic fencing generation, and lifecycle state. Claim creation and assistant placeholder insertion occur in one transaction, so Stop always has a concrete message identity. A blocked decision creates no claim, message, upload, live-run slot, or SSE response. Claim creation is the admission point; uploads, parsing, streaming, Redis, and model calls happen after the guard transaction commits. Matching request/hash retries return the existing claim; a changed hash conflicts, and a terminal claim cannot restart generation. Every continuation and final write is fenced by the active generation; lease expiry fences the old handler and terminalizes an orphan placeholder rather than restarting work. Post-claim failures and terminal turns transition the claim with compare-and-swap. Revocation with `allow` leaves earlier claims runnable. Revocation with `cancel` appends its audit and marks matching active claims cancelled in the same guarded transaction; the owning AgentQA handler observes the durable state, cancels the exact message's generation context, and persists its terminal message state. Polling the durable claim is the recovery mechanism if stop-event delivery is interrupted.

The existing `checkReleaseAdmissionTx` predicate remains the source of exact Release/dependency revocation semantics. A shared repository seam will resolve and check a local published Variant for the exact server-derived local Agent Version inside a caller-held tenant-guarded transaction so Run and chat admission cannot drift.

## Alternatives considered

- **Service precheck followed by existing admission:** rejected because revocation can commit between the check and Run/turn creation.
- **Hold the tenant guard across upload, extraction, budget/network work, or SSE:** rejected because external work would serialize unrelated requests for the tenant and could hold a database lock indefinitely.
- **Use only Redis live-run markers or message rows:** rejected because quick-answer turns do not necessarily set the marker and message rows have no atomic security admission state.
- **Append a stop event without durable claim cancellation:** rejected because a process or stream delivery failure can lose the notification while generation continues. Durable cancellation commits with the revocation ledger; watcher polling recovers notification loss.
- **Change the meaning of existing replay:** rejected because current workbench retries return their previously admitted result.

## Consequences

- New Run and AgentQA admission must use the same source-tenant guard and exact Release predicate as revocation.
- The AgentQA claim requires paired SQLite and PostgreSQL migrations, tenant-scoped repository methods, and SQLite schema coverage. The current migration heads are SQLite `000125` and versioned `000204`; the next candidates are `000126` and `000205` and must be rechecked before implementation.
- Chat claim completion/cancellation must be idempotent and observable by the handler that owns the live generation. Claim polling adds bounded database reads while a custom-Agent turn is active.
- A crash after claim commit but before work starts may leave a nonterminal claim. Recovery never starts a second generation automatically. Lease expiry fences the prior generation and terminalizes any orphan assistant placeholder; a new explicit turn uses a new request ID and message identity. The lease interval is a runtime constant selected from the bounded handler heartbeat interval and tested at expiry boundaries.
- For shared Agents, the source tenant is `effectiveTenantID`; for tenant-local Agents it is the owning session tenant. The session tenant remains the scope for message and upload persistence. The implementation must not accept a client-selected source tenant without successful `resolveAgent` authorization.

## Status

Proposed. Requires independent architecture review of exact Version binding, tenant lock ordering, claim identity/fencing, and atomic cancellation before Task8 implementation. This ADR records an implementation seam for the existing approved security behavior; it does not change the product's `cancel` / `allow` choices or authorize deletion of historical data.
