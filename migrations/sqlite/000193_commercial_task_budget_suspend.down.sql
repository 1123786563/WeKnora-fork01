-- Reverse of the suspend catch-up migration. The bundled SQLite supports
-- ALTER TABLE DROP COLUMN (as do the other early down migrations, e.g.
-- 000002/000003), and these plain columns carry no index or constraint, so
-- dropping them keeps the migration replay-safe on down/up round trips.
ALTER TABLE commercial_task_budgets DROP COLUMN suspended_at;
ALTER TABLE commercial_task_budgets DROP COLUMN suspended_reason;
