DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM craft_run_view_effect_intents
        WHERE effect_kind IN ('docker_network_create', 'docker_probe')
    ) THEN
        RAISE EXCEPTION 'cannot downgrade RunView effect request digests while new effect rows exist';
    END IF;
END
$$;

ALTER TABLE craft_run_view_effect_intents
    DROP CONSTRAINT ck_craft_run_view_effect_request_required,
    DROP CONSTRAINT ck_craft_run_view_effect_request_digest,
    DROP CONSTRAINT ck_craft_run_view_effect_kind,
    DROP COLUMN request_digest,
    ADD CONSTRAINT ck_craft_run_view_effect_kind CHECK (effect_kind IN (
        'allocate', 'docker_create', 'docker_start', 'opencode_create'
    ));
