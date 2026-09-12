-- O01 down: outbox first (it references fact revisions), then the ledger.
DROP TABLE IF EXISTS craft_usage_outbox;
DROP TABLE IF EXISTS craft_usage_facts;
