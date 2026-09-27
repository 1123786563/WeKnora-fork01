-- Multi-action Action Plans (T21, #51) — sqlite track. Same shape as the
-- versioned migration. A plan binds an ORDERED set of already-prepared
-- actions under ONE plan digest (CONTEXT.md 操作计划): any content,
-- connection, target or set change forms a NEW plan with a NEW digest, so
-- an old approval can never authorize new content. Exclusions are an
-- approval-time decision recorded on the plan row; per-item results stay
-- on the authoritative app_actions rows and the plan only projects them.
CREATE TABLE app_action_plans (
    tenant_id INTEGER NOT NULL,
    id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    digest TEXT NOT NULL,
    state TEXT NOT NULL,
    excluded_json TEXT NOT NULL DEFAULT '',
    approved_by TEXT NOT NULL DEFAULT '',
    approved_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_app_action_plans_digest ON app_action_plans (tenant_id, digest);

CREATE TABLE app_action_plan_items (
    tenant_id INTEGER NOT NULL,
    plan_id TEXT NOT NULL,
    seq INTEGER NOT NULL,
    action_id TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (tenant_id, plan_id, seq)
);

CREATE INDEX idx_app_action_plan_items_action ON app_action_plan_items (tenant_id, action_id);
