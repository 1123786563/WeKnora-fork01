ALTER TABLE execution_cleanup ADD COLUMN backup_reconciled BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE execution_cleanup ADD COLUMN retention_until DATETIME;
