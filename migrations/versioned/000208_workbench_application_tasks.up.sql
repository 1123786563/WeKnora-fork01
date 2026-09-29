CREATE TABLE workbench_application_tasks (
  tenant_id BIGINT NOT NULL,
  owner_id VARCHAR(512) NOT NULL,
  origin VARCHAR(64) NOT NULL CHECK (origin = 'career_application'),
  origin_request_id VARCHAR(64) NOT NULL,
  application_id VARCHAR(36) NOT NULL,
  task_id VARCHAR(36) NOT NULL,
  run_id VARCHAR(64) NOT NULL,
  title VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
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
