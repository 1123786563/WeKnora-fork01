ALTER TABLE execution_cleanup
    ADD COLUMN backup_reconciled BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN retention_until TIMESTAMPTZ;
