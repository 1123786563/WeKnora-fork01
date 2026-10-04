DO $$ BEGIN RAISE NOTICE '[Migration 000184] Dropping commercial projection convergence faces'; END $$;
DROP TABLE IF EXISTS commercial_reconciliation_state;
DROP TABLE IF EXISTS commercial_projection_audit;
DROP TABLE IF EXISTS commercial_webhook_inbox;
