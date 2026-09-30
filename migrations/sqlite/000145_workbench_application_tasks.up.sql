CREATE TABLE workbench_application_tasks (
  tenant_id INTEGER NOT NULL,
  owner_id TEXT NOT NULL,
  origin TEXT NOT NULL CHECK (origin = 'career_application'),
  origin_request_id TEXT NOT NULL,
  application_id TEXT NOT NULL,
  task_id TEXT NOT NULL,
  run_id TEXT NOT NULL,
  title TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL,
  FOREIGN KEY (tenant_id, task_id)
    REFERENCES sessions (tenant_id, id) ON DELETE CASCADE,
  FOREIGN KEY (tenant_id, run_id)
    REFERENCES agent_runs (tenant_id, run_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_workbench_application_tasks_request
  ON workbench_application_tasks (tenant_id, owner_id, origin, origin_request_id);
CREATE UNIQUE INDEX uq_workbench_application_tasks_application
  ON workbench_application_tasks (tenant_id, owner_id, origin, application_id);
CREATE INDEX idx_workbench_application_tasks_task
  ON workbench_application_tasks (tenant_id, task_id);
CREATE INDEX idx_workbench_application_tasks_run
  ON workbench_application_tasks (tenant_id, run_id);
