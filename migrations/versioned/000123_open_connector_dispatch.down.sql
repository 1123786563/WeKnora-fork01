-- T10 down (PG): drop the dispatch idempotency, lease and retry tables.
DROP TABLE connector_provider_retry_state;
DROP TABLE connector_dispatch_leases;
DROP TABLE connector_dispatch_lease_scopes;
DROP TABLE connector_dispatch_records;
