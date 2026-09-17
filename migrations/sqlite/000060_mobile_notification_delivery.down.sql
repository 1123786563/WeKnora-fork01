DROP INDEX IF EXISTS idx_mobile_notification_next_attempt;
-- SQLite does not support DROP COLUMN on all supported versions; rollback is table-rebuild managed by migration tooling.
