-- O01 down: drop the delivery outbox first (it references fact revisions),
-- then the usage fact ledger itself. No other table references these.
DROP TABLE IF EXISTS craft_usage_outbox;
DROP TABLE IF EXISTS craft_usage_facts;
