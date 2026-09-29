-- Multi-action Action Plans (T21, #51): an ORDERED set of already-prepared
-- external actions approved as ONE decision under ONE plan digest
-- (CONTEXT.md 操作计划). Any content, connection, target or set change
-- forms a NEW plan with a NEW digest, so an old approval can never
-- authorize new content; exclusions are an approval-time decision
-- recorded on the plan row; per-item results stay on the authoritative
-- app_actions rows and the plan only projects them.
CREATE TABLE app_action_plans (
    tenant_id BIGINT NOT NULL,
    id VARCHAR(64) NOT NULL,
    actor_id VARCHAR(255) NOT NULL,
    digest VARCHAR(64) NOT NULL,
    state VARCHAR(16) NOT NULL,
    excluded_json TEXT NOT NULL DEFAULT '',
    approved_by VARCHAR(255) NOT NULL DEFAULT '',
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, id)
);

CREATE INDEX idx_app_action_plans_digest ON app_action_plans (tenant_id, digest);

CREATE TABLE app_action_plan_items (
    tenant_id BIGINT NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    seq INTEGER NOT NULL,
    action_id VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (tenant_id, plan_id, seq)
);

CREATE INDEX idx_app_action_plan_items_action ON app_action_plan_items (tenant_id, action_id);
