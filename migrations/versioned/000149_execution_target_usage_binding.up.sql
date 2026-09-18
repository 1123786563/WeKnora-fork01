ALTER TABLE execution_targets
    ADD COLUMN usage_binding_json JSONB NOT NULL DEFAULT '{}'::jsonb;
