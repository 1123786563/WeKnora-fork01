DROP TABLE IF EXISTS semantic_attempt_counters;
DROP TABLE IF EXISTS semantic_task_operations;
ALTER TABLE semantic_completion_receipts DROP COLUMN IF EXISTS operation_id;
ALTER TABLE semantic_completion_receipts DROP COLUMN IF EXISTS attempt;
