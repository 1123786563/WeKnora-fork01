---
status: accepted
date: 2026-09-20
---

# Native tool dispatch uses a server-owned preflight

The native Agent tool boundary must not treat a previously assembled scope,
approval, or budget snapshot as authorization to cause an external effect.
Immediately before an external call, `ToolDispatchPreflight` rechecks the
server-owned scope, required grants, and run fence; it obtains an atomic
budget reservation; and it consumes a durable decision reference supplied by
the pending-decision seam. A failed preflight performs zero external calls.

Recovery is capability-driven: a confirmed outcome is reused; queryable
unknown work is queried; still-valid idempotent/read-only work may receive a
new attempt; and non-queryable or non-idempotent unknown work is held for the
user. An outcome and its `CommitIntent` are written atomically, or recovery
must create the missing intent without dispatching the tool again.

This decision keeps P2.3 as an isolated governance/journal slice under the
P0 no-go: it neither constructs a native Runner nor wires MCP, Skills,
connectors, child agents, or production tool execution. P2.4 remains the
owner of durable pending-decision storage; P2.3 consumes its decision
reference rather than duplicating approval state.

## P2.3 seam

`ToolDispatchPreflight` receives only server-assembled scope, fence, plan,
attempt, and decision-reference values. It rechecks the live scope and every
required grant, then delegates one atomic `ReserveAndConsume` operation. That
operation is the future P2.4/P2.5 integration point: it must recheck the
fence, reserve budget, and CAS-consume the durable decision reference before
the preflight returns success. An absent preflight, reference, resolver, or
reservation fails closed.

The journal pins a tool attempt to the exact `(model attempt, CallID)` plan;
an ambiguous CallID cannot be copied from another model attempt. After a
confirmed delegate return, it writes the immutable outcome and its pending
`CommitIntent` in one transaction. If that transaction fails, P2.3 records
`EffectUnknown`; a subsequent call of that attempt returns `unknown_effect`
and never redispatches. P2.4/P2.5/P3 still need to supply the actual durable
decision/budget adapters, recovery query execution, and production dispatch.
