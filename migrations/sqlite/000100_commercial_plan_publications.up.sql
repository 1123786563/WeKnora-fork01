-- Lago billing migration T07 (#79) — SQLite mirror of versioned
-- 000179: the append-only publish projection plus the published-row
-- immutability trigger (RAISE(ABORT, 'published_plan_immutable')).
CREATE TABLE IF NOT EXISTS commercial_plan_publications (
    command_key  TEXT NOT NULL,
    plan_key     TEXT NOT NULL,
    version      INTEGER NOT NULL,
    plan_code    TEXT NOT NULL UNIQUE,
    receipt_json TEXT NOT NULL,
    published_by TEXT NOT NULL DEFAULT '',
    published_at DATETIME NOT NULL,
    PRIMARY KEY (command_key),
    UNIQUE (plan_key, version)
);

CREATE TRIGGER IF NOT EXISTS trg_commercial_plan_catalog_published_immutable
    BEFORE UPDATE ON commercial_plan_catalog
    FOR EACH ROW
    WHEN OLD.state = 'published'
        AND (NEW.definition_json <> OLD.definition_json OR NEW.external_id <> OLD.external_id)
BEGIN
    SELECT RAISE(ABORT, 'published_plan_immutable');
END;
