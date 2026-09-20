-- Description: Lago billing migration T08 (#80) — per-tenant benefits
-- projection (the rebuildable Billing Projection of the spec's "Local
-- persistence and state projection" section) plus the coordinator's monthly
-- credit batch registry. External identities (plan_code,
-- external_subscription_id, wallet_ref) are seam-internal columns: the
-- Billing API maps plan codes through commercial_plan_publications and
-- never surfaces provider vocabulary (ADR-0014).
-- commercial_credit_batches anchors the three T03-verdict coordinator
-- obligations: pre-dispatch expiry rejection (expires_at is checked against
-- now on every read — the authority's lazy wallet termination never leaks
-- spendable-looking credits), grant idempotency (one row per
-- (tenant_id, period); the unique constraint is the FIRST idempotency
-- layer), and wallet-cap accounting (the registry records every batch so
-- future top-up tickets can budget the remaining ≥ 4 wallet slots under the
-- authority's 6-active-wallet cap; the monthly cadence occupies ≤ 2
-- transient slots).
DO $$ BEGIN RAISE NOTICE '[Migration 000180] Creating commercial_tenant_benefits and commercial_credit_batches'; END $$;

CREATE TABLE commercial_tenant_benefits (
    tenant_id                BIGINT        NOT NULL,
    external_customer_id     VARCHAR(255)  NOT NULL,
    external_subscription_id VARCHAR(255)  NOT NULL,
    subscription_state       VARCHAR(32)   NOT NULL,
    plan_code                VARCHAR(255)  NOT NULL DEFAULT '',
    plan_key                 VARCHAR(255)  NOT NULL DEFAULT '',
    plan_version             BIGINT        NOT NULL DEFAULT 0,
    features_json            TEXT          NOT NULL DEFAULT '{}',
    limits_json              TEXT          NOT NULL DEFAULT '{}',
    credits_balance_micro    BIGINT        NOT NULL DEFAULT 0,
    projected_at             TIMESTAMPTZ   NOT NULL,
    PRIMARY KEY (tenant_id)
);

COMMENT ON TABLE commercial_tenant_benefits IS 'Rebuildable per-tenant billing projection: plan identity, entitlements, quotas and credit balance at last refresh (Lago T08, #80)';
COMMENT ON COLUMN commercial_tenant_benefits.external_customer_id IS 'Deterministic customer identity weknora-tenant-<id> — seam-internal, never crosses the Billing API';
COMMENT ON COLUMN commercial_tenant_benefits.external_subscription_id IS 'Deterministic subscription identity weknora-tenant-<id>-sub — seam-internal (subscription continuity)';
COMMENT ON COLUMN commercial_tenant_benefits.subscription_state IS 'Closed authority truth: active|pending';
COMMENT ON COLUMN commercial_tenant_benefits.plan_code IS 'External plan code of the subscribed plan version — mapped to (plan_key, version) via commercial_plan_publications at write time';
COMMENT ON COLUMN commercial_tenant_benefits.credits_balance_micro IS 'Included-credit balance in micro-credits at last refresh, BEFORE the registry expiry overlay';

CREATE TABLE commercial_credit_batches (
    id            BIGSERIAL     NOT NULL,
    tenant_id     BIGINT        NOT NULL,
    period        VARCHAR(7)    NOT NULL,
    command_key   VARCHAR(255)  NOT NULL,
    wallet_ref    VARCHAR(255)  NOT NULL DEFAULT '',
    granted_micro BIGINT        NOT NULL,
    expires_at    TIMESTAMPTZ   NOT NULL,
    state         VARCHAR(32)   NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL,
    PRIMARY KEY (id),
    CONSTRAINT uq_credit_batch_tenant_period UNIQUE (tenant_id, period)
);

COMMENT ON TABLE commercial_credit_batches IS 'Coordinator-owned monthly credit batch registry: one short-TTL wallet batch per (tenant, period) — grant idempotency anchor, pre-dispatch expiry gate and wallet-cap ledger (Lago T08, #80; T03 verdict obligations)';
COMMENT ON COLUMN commercial_credit_batches.period IS 'UTC calendar month YYYY-MM of the batch';
COMMENT ON COLUMN commercial_credit_batches.command_key IS 'Idempotency identity grant_included_credits:<ext-customer>:<period> — coordinator-owned (E3: the authority wallet API has none)';
COMMENT ON COLUMN commercial_credit_batches.wallet_ref IS 'Seam-internal wallet lago_id once known — immutable once set';
COMMENT ON COLUMN commercial_credit_batches.expires_at IS 'Exclusive period end; a batch at expires_at <= now is unavailable even while the authority still reports the wallet active (lazy termination window)';
COMMENT ON COLUMN commercial_credit_batches.state IS 'Advisory state granted|expired — expiry is always decided by expires_at, never by this column';
