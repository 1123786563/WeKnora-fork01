-- Description: Lago billing migration T07 (#79) — append-only publish
-- projection for immutable Plan Versions. Each published (plan_key, version)
-- maps exactly once to its seam command identity
-- (publish_plan_version:<key>:<version>), its external plan code
-- (weknora-<slug>-v<n>) and the durable receipt. The plan code is
-- seam-internal: the admin API addresses versions by (plan_key, version)
-- and never surfaces provider vocabulary (ADR-0014).
-- The trigger enforces published-definition immutability in the DATABASE
-- (belt and braces with the repository/service guards): any UPDATE changing
-- definition_json/external_id of a row whose OLD state is 'published'
-- aborts with the closed token 'published_plan_immutable'; state-only
-- transitions (publishing→published, →archived) never trip it.
DO $$ BEGIN RAISE NOTICE '[Migration 000179] Creating commercial_plan_publications'; END $$;

CREATE TABLE commercial_plan_publications (
    command_key  VARCHAR(255)  NOT NULL,
    plan_key     VARCHAR(255)  NOT NULL,
    version      BIGINT        NOT NULL,
    plan_code    VARCHAR(255)  NOT NULL,
    receipt_json TEXT          NOT NULL,
    published_by VARCHAR(255)  NOT NULL DEFAULT '',
    published_at TIMESTAMPTZ   NOT NULL,
    PRIMARY KEY (command_key),
    CONSTRAINT uq_plan_publication_version UNIQUE (plan_key, version),
    CONSTRAINT uq_plan_publication_code UNIQUE (plan_code)
);

COMMENT ON TABLE commercial_plan_publications IS 'Append-only map from a published plan version to its seam command identity and receipt (Lago T07, #79)';
COMMENT ON COLUMN commercial_plan_publications.command_key IS 'Idempotency identity publish_plan_version:<plan_key>:<version> — coordinator-owned';
COMMENT ON COLUMN commercial_plan_publications.plan_code IS 'External plan code weknora-<slug>-v<n> — seam-internal, never crosses the admin API';
COMMENT ON COLUMN commercial_plan_publications.receipt_json IS 'Durable CommandReceipt JSON recorded at publish time';
COMMENT ON COLUMN commercial_plan_publications.published_by IS 'Publishing user id (empty means a system/API-key principal)';

CREATE OR REPLACE FUNCTION commercial_plan_catalog_published_immutable() RETURNS trigger AS $$
BEGIN
    IF OLD.state = 'published' AND (NEW.definition_json IS DISTINCT FROM OLD.definition_json OR NEW.external_id IS DISTINCT FROM OLD.external_id) THEN
        RAISE EXCEPTION 'published_plan_immutable';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_commercial_plan_catalog_published_immutable ON commercial_plan_catalog;

CREATE TRIGGER trg_commercial_plan_catalog_published_immutable
    BEFORE UPDATE ON commercial_plan_catalog
    FOR EACH ROW
    WHEN (OLD.state = 'published' AND (NEW.definition_json IS DISTINCT FROM OLD.definition_json OR NEW.external_id IS DISTINCT FROM OLD.external_id))
    EXECUTE FUNCTION commercial_plan_catalog_published_immutable();
