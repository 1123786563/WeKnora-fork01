# Career backend module

Career owns one private member space and its confirmed facts, idempotency
receipts, and append-only evidence. Every repository call requires the
server-derived tenant and owner scope. HTTP handlers and process registration
are supplied by the integration layer; this module does not read identity
authority from request payloads.

The Workbench Artifact version grant authority is consumed through a narrow
port. Career owns resource authorization and must recheck tenant, owner,
resource, immutable version, digest, deletion, and revocation at download time.
