---
status: accepted
date: 2026-09-20
---

# Native pending decisions use typed authority and fail-closed composition

P2.4 owns pending-decision identity, one-shot compare-and-swap resolution,
OAuth-state validation, and cancellation finalization. P2.5 owns budget
reservation and idempotent accounting. A dispatch coordinator consumes a
typed resolved-pending identity, live scope/fence, and reservation in one
transactional composition; a failed reservation leaves the decision reusable
and makes zero external calls.

OAuth is provider-bound. Its authority adapter owns state/attempt storage,
tenant/principal/service binding, redirect allowlist verification, expiry,
callback receipt verification, and token re-query. The adapter may be absent;
absence remains a durable fail-closed error, never an implicit approval.

Cancellation is owned by P2.4 while a run waits for approval or OAuth. It
uses the same pending revision CAS as resolution and produces a terminal
cancelled decision; P2.6 consumes that terminal state for broader control
commands. All seams must validate SQLite and PostgreSQL transaction behavior.

This decision authorizes only fail-closed adapters, deterministic fakes, and
storage/coordinator tests until P0 product-execution gates are separately
closed. It does not authorize OAuth callbacks, provider tokens, Runner
execution, or external dispatch in production.
