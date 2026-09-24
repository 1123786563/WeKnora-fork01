ALTER TABLE craft_run_view_effect_intents
    ADD COLUMN request_digest VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE craft_run_view_effect_intents
    DROP CONSTRAINT ck_craft_run_view_effect_kind,
    ADD CONSTRAINT ck_craft_run_view_effect_kind CHECK (effect_kind IN (
        'allocate', 'docker_network_create', 'docker_create', 'docker_start', 'docker_probe', 'opencode_create'
    )),
    ADD CONSTRAINT ck_craft_run_view_effect_request_digest CHECK (
        request_digest = '' OR request_digest ~ '^[0-9a-f]{64}$'
    ),
    ADD CONSTRAINT ck_craft_run_view_effect_request_required CHECK (
        effect_kind NOT IN ('docker_network_create', 'docker_probe') OR request_digest ~ '^[0-9a-f]{64}$'
    );
