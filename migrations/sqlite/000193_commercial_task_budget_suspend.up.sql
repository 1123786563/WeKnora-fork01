-- #90 / Lago 18 (fcb4f0c68) added the durable task-suspend columns to
-- TaskBudgetRow without a matching migration; the sqlite chain catches up.
ALTER TABLE commercial_task_budgets ADD COLUMN suspended_reason TEXT NOT NULL DEFAULT '';
ALTER TABLE commercial_task_budgets ADD COLUMN suspended_at DATETIME;
