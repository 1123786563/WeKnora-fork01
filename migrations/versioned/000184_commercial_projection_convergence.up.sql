-- Description: Lago billing migration (#98 / Lago 26) — commercial
-- projection convergence faces: the webhook inbox (unique-key dedupe),
-- the projection audit trail (divergence/repair/watermark, US43-45 gate 19)
-- and the per-stream reconciliation watermark. External identities are
-- seam-internal columns and never cross the Billing API (ADR-0014).
DO $$ BEGIN RAISE NOTICE '[Migration 000184] Creating commercial_webhook_inbox, commercial_projection_audit, commercial_reconciliation_state'; END $$;

CREATE TABLE commercial_webhook_inbox (
    id            BIGSERIAL       PRIMARY KEY,
    provider      VARCHAR(64)     NOT NULL,
    event_id      VARCHAR(255)    NOT NULL,
    kind          VARCHAR(64)     NOT NULL DEFAULT '',
    external_id   VARCHAR(255)    NOT NULL DEFAULT '',
    tenant_id     BIGINT          NOT NULL DEFAULT 0,
    received_at   TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

-- The unique key the spec's "unique key 验证" requires: one receipt per
-- (provider, event_id) — a redelivery is a no-op, never a second event.
CREATE UNIQUE INDEX uniq_webhook_event
    ON commercial_webhook_inbox (provider, event_id);

CREATE TABLE commercial_projection_audit (
    id            BIGSERIAL       PRIMARY KEY,
    source        VARCHAR(16)     NOT NULL,
    action        VARCHAR(16)     NOT NULL,
    kind          VARCHAR(64)     NOT NULL DEFAULT '',
    external_id   VARCHAR(255)    NOT NULL DEFAULT '',
    tenant_id     BIGINT          NOT NULL DEFAULT 0,
    detail        TEXT            NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_projection_audit_object
    ON commercial_projection_audit (kind, external_id, created_at DESC);

CREATE TABLE commercial_reconciliation_state (
    stream        VARCHAR(64)     PRIMARY KEY,
    cursor_value  TEXT            NOT NULL DEFAULT '',
    updated_at    TIMESTAMPTZ     NOT NULL DEFAULT NOW()
);
