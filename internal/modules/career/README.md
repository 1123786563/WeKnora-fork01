# Career backend module

Career owns one private member space and its confirmed facts, idempotency
receipts, and append-only evidence. Every repository call requires the
server-derived tenant and owner scope. HTTP handlers and process registration
are supplied by the integration layer; this module does not read identity
authority from request payloads.

The authenticated grant issuer derives tenant and owner from `Caller`. The
`career_artifact_bindings` catalog binds an owner resource to an exact ready
Workbench version while taking digest and storage metadata from the immutable
version row. Every redemption rechecks tenant, owner, resource, version,
digest, deletion, and revocation before opening bytes; the adapter is provided
to Workbench signing through an integration-layer authorization port so
Career persistence has no Workbench import.
