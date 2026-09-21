-- SP3 down: runs first (the fire ledger references the recipe), then the
-- recipe table.
DROP TABLE IF EXISTS craft_scheduled_task_runs;
DROP TABLE IF EXISTS craft_scheduled_tasks;
